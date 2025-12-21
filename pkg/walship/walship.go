package walship

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/bft-labs/walship/internal/adapters/fs"
	httpAdapter "github.com/bft-labs/walship/internal/adapters/http"
	logAdapter "github.com/bft-labs/walship/internal/adapters/log"
	"github.com/bft-labs/walship/internal/app"
	"github.com/bft-labs/walship/internal/domain"
	"github.com/bft-labs/walship/internal/ports"
	"github.com/bft-labs/walship/pkg/log"
	"github.com/bft-labs/walship/pkg/sender"
	"github.com/bft-labs/walship/pkg/state"
	"github.com/bft-labs/walship/pkg/wal"
)

// Walship is a WAL streaming agent that can be embedded in other applications.
// Use New() to create an instance, then Start() to begin streaming.
type Walship struct {
	config    Config
	opts      options
	lifecycle *app.Lifecycle
	agent     *app.Agent
	reader    ports.FrameReader
	sender    ports.FrameSender
	stateRepo ports.StateRepository
	logger    ports.Logger

	// Plugin support
	plugins []Plugin

	// Cleanup runner (config-based, not a plugin)
	cleanup *cleanupRunner

	// Resource gate (config-based, not a plugin)
	resourceGate *resourceGate

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new Walship instance with the given configuration.
// The instance is created in StateStopped; call Start() to begin streaming.
// Returns an error if configuration is invalid.
func New(cfg Config, opts ...Option) (*Walship, error) {
	// Set defaults
	cfg.SetDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Validate module version compatibility
	if err := validateModuleVersions(); err != nil {
		return nil, err
	}

	// Apply options
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	o := defaultOptions(httpClient)
	for _, opt := range opts {
		opt(&o)
	}

	// Create logger
	var logger ports.Logger
	if o.logger != nil {
		logger = o.logger
	} else {
		logger = logAdapter.NewNoopLogger()
	}

	// Create event emitter wrapper
	var emitter eventEmitterWrapper
	if o.eventHandler != nil {
		emitter = eventEmitterWrapper{handler: o.eventHandler}
	}

	// Create lifecycle manager
	lifecycle := app.NewLifecycle(logger, &emitter)

	// Create adapters
	reader := fs.NewIndexReader(cfg.WALDir, logger)
	stateRepo := fs.NewStateFileRepository(cfg.StateDir)
	sender := httpAdapter.NewFrameSender(o.httpClient, logger)

	// Create agent config
	agentCfg := app.AgentConfig{
		PollInterval:  cfg.PollInterval,
		SendInterval:  cfg.SendInterval,
		HardInterval:  cfg.HardInterval,
		MaxBatchBytes: cfg.MaxBatchBytes,
		Once:          cfg.Once,
		Verify:        cfg.Verify,
		Meta:          cfg.Meta,
		ChainID:       cfg.ChainID,
		NodeID:        cfg.NodeID,
		Hostname:      hostname(),
		OSArch:        runtime.GOOS + "/" + runtime.GOARCH,
		AuthKey:       cfg.AuthKey,
		ServiceURL:    cfg.ServiceURL,
	}

	// Create resource gate if configured
	var resGate *resourceGate
	if o.resourceGatingConfig != nil && o.resourceGatingConfig.Enabled {
		resGate = newResourceGate(*o.resourceGatingConfig, logger)
	}

	// Create agent (pass resource gate for backpressure)
	agent := app.NewAgent(agentCfg, reader, sender, stateRepo, logger, &emitter, resGate)

	// Create cleanup runner if configured
	var cleanup *cleanupRunner
	if o.cleanupConfig != nil && o.cleanupConfig.Enabled {
		cleanup = newCleanupRunner(*o.cleanupConfig, cfg.WALDir, cfg.StateDir, logger)
	}

	return &Walship{
		config:       cfg,
		opts:         o,
		lifecycle:    lifecycle,
		agent:        agent,
		reader:       reader,
		sender:       sender,
		stateRepo:    stateRepo,
		logger:       logger,
		plugins:      o.plugins,
		cleanup:      cleanup,
		resourceGate: resGate,
	}, nil
}

// Start begins WAL streaming in the background.
// Returns immediately after starting the streaming goroutine.
// Returns an error if already running or if startup fails.
// The provided context is used for the lifetime of the streaming operation.
func (w *Walship) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.lifecycle.CanStart() {
		return domain.ErrAlreadyRunning
	}

	// Transition to starting
	if err := w.lifecycle.TransitionTo(app.StateStarting, "Start() called"); err != nil {
		return err
	}

	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)
	w.ctx = runCtx
	w.cancel = cancel
	w.lifecycle.SetCancel(cancel)

	// Initialize plugins
	pluginCfg := PluginConfig{
		WALDir:     w.config.WALDir,
		StateDir:   w.config.StateDir,
		ServiceURL: w.config.ServiceURL,
		ChainID:    w.config.ChainID,
		NodeID:     w.config.NodeID,
		AuthKey:    w.config.AuthKey,
		NodeHome:   w.config.NodeHome,
		Logger:     w.logger,
	}
	for _, p := range w.plugins {
		if err := p.Initialize(runCtx, pluginCfg); err != nil {
			w.logger.Error("plugin initialization failed",
				ports.String("plugin", p.Name()),
				ports.Err(err))
			cancel()
			_ = w.lifecycle.TransitionTo(app.StateCrashed, "plugin init failed: "+p.Name())
			return err
		}
		w.logger.Info("plugin initialized", ports.String("plugin", p.Name()))
	}

	// Start cleanup runner if configured
	if w.cleanup != nil {
		w.cleanup.start(runCtx)
	}

	// Log resource gating status
	if w.resourceGate != nil {
		w.logger.Info("resource gating enabled")
	}

	// Start the agent in a goroutine
	w.lifecycle.AddWorker()
	go func() {
		defer w.lifecycle.WorkerDone()

		// Transition to running
		if err := w.lifecycle.TransitionTo(app.StateRunning, "agent starting"); err != nil {
			w.logger.Error("failed to transition to running", ports.Err(err))
			return
		}

		// Run the agent loop
		err := w.agent.Run(runCtx)

		// Handle completion
		if err != nil && err != context.Canceled {
			w.logger.Error("agent error", ports.Err(err))
			_ = w.lifecycle.TransitionTo(app.StateCrashed, err.Error())
		}
	}()

	return nil
}

// Stop gracefully shuts down the agent.
// Flushes pending batches and persists state.
// Waits up to 30 seconds before forcing shutdown.
// Returns nil on graceful shutdown, ErrShutdownTimeout if forced.
func (w *Walship) Stop() error {
	w.mu.Lock()

	if !w.lifecycle.CanStop() {
		w.mu.Unlock()
		return domain.ErrNotRunning
	}

	// Transition to stopping
	if err := w.lifecycle.TransitionTo(app.StateStopping, "Stop() called"); err != nil {
		w.mu.Unlock()
		return err
	}

	// Cancel the context
	if w.cancel != nil {
		w.cancel()
	}

	w.mu.Unlock()

	// Wait for workers with timeout
	err := w.lifecycle.WaitWithTimeout(app.ShutdownTimeout)

	// Stop cleanup runner
	if w.cleanup != nil {
		w.cleanup.stop()
	}

	// Shutdown plugins (in reverse order)
	shutdownCtx := context.Background()
	for i := len(w.plugins) - 1; i >= 0; i-- {
		p := w.plugins[i]
		if shutdownErr := p.Shutdown(shutdownCtx); shutdownErr != nil {
			w.logger.Error("plugin shutdown failed",
				ports.String("plugin", p.Name()),
				ports.Err(shutdownErr))
		} else {
			w.logger.Info("plugin shutdown complete", ports.String("plugin", p.Name()))
		}
	}

	// Transition to stopped
	if err != nil {
		_ = w.lifecycle.TransitionTo(app.StateCrashed, "shutdown timeout")
	} else {
		_ = w.lifecycle.TransitionTo(app.StateStopped, "graceful shutdown")
	}

	return err
}

// Status returns the current lifecycle state.
// Safe to call concurrently from any goroutine.
func (w *Walship) Status() State {
	return convertState(w.lifecycle.State())
}

// hostname returns the current hostname.
func hostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

// eventEmitterWrapper adapts EventHandler to the internal emitter interfaces.
type eventEmitterWrapper struct {
	handler EventHandler
}

func (e *eventEmitterWrapper) OnStateChange(previous, current app.State, reason string) {
	if e.handler == nil {
		return
	}
	e.handler.OnStateChange(StateChangeEvent{
		Previous: convertState(previous),
		Current:  convertState(current),
		Reason:   reason,
	})
}

func (e *eventEmitterWrapper) OnSendSuccess(frameCount, bytesSent int, duration time.Duration) {
	if e.handler == nil {
		return
	}
	e.handler.OnSendSuccess(SendSuccessEvent{
		FrameCount: frameCount,
		BytesSent:  bytesSent,
		Duration:   duration,
	})
}

func (e *eventEmitterWrapper) OnSendError(err error, frameCount int, retryable bool) {
	if e.handler == nil {
		return
	}
	e.handler.OnSendError(SendErrorEvent{
		Error:      err,
		FrameCount: frameCount,
		Retryable:  retryable,
	})
}

func convertState(s app.State) State {
	switch s {
	case app.StateStopped:
		return StateStopped
	case app.StateStarting:
		return StateStarting
	case app.StateRunning:
		return StateRunning
	case app.StateStopping:
		return StateStopping
	case app.StateCrashed:
		return StateCrashed
	default:
		return StateStopped
	}
}

// validateModuleVersions checks that all module versions are compatible.
// Returns an error if any module version is below its minimum compatible version.
func validateModuleVersions() error {
	modules := map[string]struct {
		version    string
		minVersion string
	}{
		"wal":    {wal.Version, wal.MinCompatibleVersion},
		"sender": {sender.Version, sender.MinCompatibleVersion},
		"state":  {state.Version, state.MinCompatibleVersion},
		"log":    {log.Version, log.MinCompatibleVersion},
	}

	for name, m := range modules {
		if !isVersionCompatible(m.version, m.minVersion) {
			return fmt.Errorf("module %s version %s is below minimum compatible version %s",
				name, m.version, m.minVersion)
		}
	}

	return nil
}

// isVersionCompatible checks if version >= minVersion using semantic versioning.
// Assumes versions are in format "major.minor.patch".
func isVersionCompatible(version, minVersion string) bool {
	// Parse versions (simplified semver comparison)
	var vMajor, vMinor, vPatch int
	var mMajor, mMinor, mPatch int

	_, _ = fmt.Sscanf(version, "%d.%d.%d", &vMajor, &vMinor, &vPatch)
	_, _ = fmt.Sscanf(minVersion, "%d.%d.%d", &mMajor, &mMinor, &mPatch)

	if vMajor != mMajor {
		return vMajor > mMajor
	}
	if vMinor != mMinor {
		return vMinor > mMinor
	}
	return vPatch >= mPatch
}

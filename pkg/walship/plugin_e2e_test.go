package walship_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bft-labs/walship/pkg/walship"
	"github.com/bft-labs/walship/plugins/configwatcher"
)

// =============================================================================
// Test Utilities
// =============================================================================

// testLogger implements walship.Logger for capturing log output in tests.
type testLogger struct {
	mu       sync.Mutex
	messages []string
}

func newTestLogger() *testLogger {
	return &testLogger{messages: make([]string, 0)}
}

func (l *testLogger) Debug(msg string, fields ...walship.LogField) {
	l.log("DEBUG", msg)
}

func (l *testLogger) Info(msg string, fields ...walship.LogField) {
	l.log("INFO", msg)
}

func (l *testLogger) Warn(msg string, fields ...walship.LogField) {
	l.log("WARN", msg)
}

func (l *testLogger) Error(msg string, fields ...walship.LogField) {
	l.log("ERROR", msg)
}

func (l *testLogger) log(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf("[%s] %s", level, msg))
}

func (l *testLogger) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.messages))
	copy(cp, l.messages)
	return cp
}

// trackingPlugin tracks initialization and shutdown calls for testing.
type trackingPlugin struct {
	name          string
	initOrder     *[]string
	shutdownOrder *[]string
	initDelay     time.Duration
	shutdownDelay time.Duration
	initError     error
	shutdownError error
	mu            sync.Mutex
	initialized   bool
	shutdown      bool
}

func newTrackingPlugin(name string, initOrder, shutdownOrder *[]string) *trackingPlugin {
	return &trackingPlugin{
		name:          name,
		initOrder:     initOrder,
		shutdownOrder: shutdownOrder,
	}
}

func (p *trackingPlugin) Name() string { return p.name }

func (p *trackingPlugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	if p.initDelay > 0 {
		select {
		case <-time.After(p.initDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initError != nil {
		return p.initError
	}

	*p.initOrder = append(*p.initOrder, p.name)
	p.initialized = true
	return nil
}

func (p *trackingPlugin) Shutdown(ctx context.Context) error {
	if p.shutdownDelay > 0 {
		time.Sleep(p.shutdownDelay)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	*p.shutdownOrder = append(*p.shutdownOrder, p.name)
	p.shutdown = true

	if p.shutdownError != nil {
		return p.shutdownError
	}
	return nil
}

func (p *trackingPlugin) IsInitialized() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initialized
}

func (p *trackingPlugin) IsShutdown() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shutdown
}

// panicPlugin panics during initialization or shutdown for testing.
type panicPlugin struct {
	walship.BasePlugin
	panicOnInit     bool
	panicOnShutdown bool
}

func (p *panicPlugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	if p.panicOnInit {
		panic("intentional panic during initialization")
	}
	return nil
}

func (p *panicPlugin) Shutdown(ctx context.Context) error {
	if p.panicOnShutdown {
		panic("intentional panic during shutdown")
	}
	return nil
}

// slowPlugin simulates a slow plugin that respects context cancellation.
type slowPlugin struct {
	walship.BasePlugin
	initDuration time.Duration
	initStarted  chan struct{}
}

func (p *slowPlugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	if p.initStarted != nil {
		close(p.initStarted)
	}
	select {
	case <-time.After(p.initDuration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// eventTracker tracks state change events.
type eventTracker struct {
	walship.BaseEventHandler
	mu           sync.Mutex
	stateChanges []walship.StateChangeEvent
	sendSuccess  []walship.SendSuccessEvent
	sendErrors   []walship.SendErrorEvent
}

func newEventTracker() *eventTracker {
	return &eventTracker{
		stateChanges: make([]walship.StateChangeEvent, 0),
		sendSuccess:  make([]walship.SendSuccessEvent, 0),
		sendErrors:   make([]walship.SendErrorEvent, 0),
	}
}

func (e *eventTracker) OnStateChange(event walship.StateChangeEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stateChanges = append(e.stateChanges, event)
}

func (e *eventTracker) OnSendSuccess(event walship.SendSuccessEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sendSuccess = append(e.sendSuccess, event)
}

func (e *eventTracker) OnSendError(event walship.SendErrorEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sendErrors = append(e.sendErrors, event)
}

func (e *eventTracker) StateChanges() []walship.StateChangeEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]walship.StateChangeEvent, len(e.stateChanges))
	copy(cp, e.stateChanges)
	return cp
}

// createTestConfig creates a minimal valid config for testing.
func createTestConfig(t *testing.T) walship.Config {
	t.Helper()
	tmpDir := t.TempDir()
	walDir := filepath.Join(tmpDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatalf("Failed to create WAL dir: %v", err)
	}
	// Create minimal index file
	if err := os.WriteFile(filepath.Join(walDir, "0000000000000000.idx"), []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	return walship.Config{
		WALDir:       walDir,
		StateDir:     walDir,
		AuthKey:      "test-key",
		ChainID:      "test-chain",
		NodeID:       "test-node",
		ServiceURL:   "http://localhost:9999",
		PollInterval: 100 * time.Millisecond,
		SendInterval: 1 * time.Second,
		HardInterval: 2 * time.Second,
		HTTPTimeout:  5 * time.Second,
		Once:         true, // Exit after processing to make tests faster
	}
}

// =============================================================================
// Plugin Lifecycle Tests
// =============================================================================

func TestPlugin_InitializationOrder(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	var initOrder []string
	var shutdownOrder []string

	plugin1 := newTrackingPlugin("plugin1", &initOrder, &shutdownOrder)
	plugin2 := newTrackingPlugin("plugin2", &initOrder, &shutdownOrder)
	plugin3 := newTrackingPlugin("plugin3", &initOrder, &shutdownOrder)

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithPlugin(plugin1),
		walship.WithPlugin(plugin2),
		walship.WithPlugin(plugin3),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for initialization
	time.Sleep(100 * time.Millisecond)

	// Verify initialization order
	if len(initOrder) != 3 {
		t.Errorf("Expected 3 plugins initialized, got %d", len(initOrder))
	}
	if initOrder[0] != "plugin1" || initOrder[1] != "plugin2" || initOrder[2] != "plugin3" {
		t.Errorf("Unexpected init order: %v", initOrder)
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify shutdown order (should be reverse of init)
	if len(shutdownOrder) != 3 {
		t.Errorf("Expected 3 plugins shutdown, got %d", len(shutdownOrder))
	}
	if shutdownOrder[0] != "plugin3" || shutdownOrder[1] != "plugin2" || shutdownOrder[2] != "plugin1" {
		t.Errorf("Unexpected shutdown order: %v (expected reverse of init)", shutdownOrder)
	}
}

func TestPlugin_InitializationFailure_PreventsStart(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	var initOrder []string
	var shutdownOrder []string

	plugin1 := newTrackingPlugin("plugin1", &initOrder, &shutdownOrder)
	plugin2 := newTrackingPlugin("plugin2", &initOrder, &shutdownOrder)
	plugin2.initError = errors.New("intentional init failure")
	plugin3 := newTrackingPlugin("plugin3", &initOrder, &shutdownOrder)

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithPlugin(plugin1),
		walship.WithPlugin(plugin2),
		walship.WithPlugin(plugin3),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	err = w.Start(ctx)

	// Start should fail due to plugin2 init failure
	if err == nil {
		t.Fatal("Start() should have failed due to plugin init error")
	}

	// plugin1 should have been initialized before plugin2 failed
	if len(initOrder) != 1 || initOrder[0] != "plugin1" {
		t.Errorf("Expected only plugin1 to init before failure, got: %v", initOrder)
	}

	// plugin3 should NOT have been initialized
	if plugin3.IsInitialized() {
		t.Error("plugin3 should not have been initialized after plugin2 failed")
	}

	// State should be crashed
	if w.Status() != walship.StateCrashed {
		t.Errorf("Status = %v, want Crashed", w.Status())
	}
}

func TestPlugin_ShutdownFailure_ContinuesOtherPlugins(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	var initOrder []string
	var shutdownOrder []string

	plugin1 := newTrackingPlugin("plugin1", &initOrder, &shutdownOrder)
	plugin2 := newTrackingPlugin("plugin2", &initOrder, &shutdownOrder)
	plugin2.shutdownError = errors.New("intentional shutdown failure")
	plugin3 := newTrackingPlugin("plugin3", &initOrder, &shutdownOrder)

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithPlugin(plugin1),
		walship.WithPlugin(plugin2),
		walship.WithPlugin(plugin3),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Stop should complete even though plugin2 fails
	_ = w.Stop()

	// All plugins should have attempted shutdown (reverse order)
	if len(shutdownOrder) != 3 {
		t.Errorf("Expected all 3 plugins to attempt shutdown, got: %v", shutdownOrder)
	}

	// plugin1 and plugin3 should have shutdown despite plugin2's failure
	if !plugin1.IsShutdown() {
		t.Error("plugin1 should have been shutdown")
	}
	if !plugin3.IsShutdown() {
		t.Error("plugin3 should have been shutdown")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestPlugin_EmptyPluginList(t *testing.T) {
	cfg := createTestConfig(t)

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	if w.Status() != walship.StateStopped {
		t.Errorf("Status = %v, want Stopped", w.Status())
	}
}

func TestPlugin_NilLogger(t *testing.T) {
	cfg := createTestConfig(t)

	var initOrder []string
	var shutdownOrder []string
	plugin := newTrackingPlugin("test-plugin", &initOrder, &shutdownOrder)

	// Create without logger - should use noop logger internally
	w, err := walship.New(cfg,
		walship.WithPlugin(plugin),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !plugin.IsInitialized() {
		t.Error("Plugin should have been initialized even without logger")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_StartAlreadyRunning(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false // Keep running

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Second Start should fail
	err = w.Start(ctx)
	if err == nil {
		t.Error("Second Start() should have failed")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_StopAlreadyStopped(t *testing.T) {
	cfg := createTestConfig(t)

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Stop without starting should fail
	err = w.Stop()
	if err == nil {
		t.Error("Stop() without Start() should have failed")
	}
}

func TestPlugin_RapidStartStop(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	logger := newTestLogger()
	var initOrder []string
	var shutdownOrder []string
	plugin := newTrackingPlugin("rapid-test", &initOrder, &shutdownOrder)

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithPlugin(plugin),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Rapid start/stop cycles
	for i := 0; i < 5; i++ {
		ctx := context.Background()
		if err := w.Start(ctx); err != nil {
			t.Fatalf("Start() iteration %d failed: %v", i, err)
		}

		// Very short run time
		time.Sleep(50 * time.Millisecond)

		if err := w.Stop(); err != nil {
			t.Errorf("Stop() iteration %d failed: %v", i, err)
		}

		// Reset tracking for next iteration
		initOrder = initOrder[:0]
		shutdownOrder = shutdownOrder[:0]
	}

	// Should end in stopped state
	if w.Status() != walship.StateStopped {
		t.Errorf("Final status = %v, want Stopped", w.Status())
	}
}

func TestPlugin_ContextCancellationDuringInit(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	initStarted := make(chan struct{})
	slowPlugin := &slowPlugin{
		BasePlugin:   walship.NewBasePlugin("slow-plugin"),
		initDuration: 5 * time.Second,
		initStarted:  initStarted,
	}

	w, err := walship.New(cfg,
		walship.WithPlugin(slowPlugin),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	startErr := make(chan error, 1)
	go func() {
		startErr <- w.Start(ctx)
	}()

	// Wait for init to start
	<-initStarted

	// Cancel context during init
	cancel()

	// Start should fail due to context cancellation
	select {
	case err := <-startErr:
		if err == nil {
			t.Error("Start() should have failed due to context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}
}

// =============================================================================
// Built-in Plugin Integration Tests
// =============================================================================

func TestPlugin_ResourceGatingIntegration(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	rgConfig := walship.ResourceGatingConfig{
		Enabled:      true,
		CPUThreshold: 0.90,
		NetThreshold: 0.70,
	}

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithResourceGatingConfig(rgConfig),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Check that resource gating logged initialization
	messages := logger.Messages()
	found := false
	for _, msg := range messages {
		if msg == "[INFO] resource gating enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Resource gating should have logged initialization")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_ConfigWatcherIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create node home with config files
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte("test = true"), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test = true"), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	walDir := filepath.Join(tmpDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatalf("Failed to create WAL dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(walDir, "0000000000000000.idx"), []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	cfg := walship.Config{
		NodeHome:     tmpDir,
		WALDir:       walDir,
		StateDir:     walDir,
		AuthKey:      "test-key",
		ChainID:      "test-chain",
		NodeID:       "test-node",
		ServiceURL:   "http://localhost:9999",
		PollInterval: 100 * time.Millisecond,
		SendInterval: 1 * time.Second,
		HardInterval: 2 * time.Second,
		HTTPTimeout:  5 * time.Second,
		Once:         true,
	}

	logger := newTestLogger()

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		configwatcher.WithConfigWatcher(configwatcher.DefaultConfig()),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Check that plugin logged initialization
	messages := logger.Messages()
	found := false
	for _, msg := range messages {
		if msg == "[INFO] Config watcher plugin initialized" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Config watcher plugin should have logged initialization")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_MultipleBuiltinPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup directories
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte("test = true"), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("test = true"), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	walDir := filepath.Join(tmpDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatalf("Failed to create WAL dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(walDir, "0000000000000000.idx"), []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	cfg := walship.Config{
		NodeHome:     tmpDir,
		WALDir:       walDir,
		StateDir:     walDir,
		AuthKey:      "test-key",
		ChainID:      "test-chain",
		NodeID:       "test-node",
		ServiceURL:   "http://localhost:9999",
		PollInterval: 100 * time.Millisecond,
		SendInterval: 1 * time.Second,
		HardInterval: 2 * time.Second,
		HTTPTimeout:  5 * time.Second,
		Once:         true,
	}

	logger := newTestLogger()

	// Use all built-in features together
	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithResourceGatingConfig(walship.DefaultResourceGatingConfig()),
		configwatcher.WithConfigWatcher(configwatcher.DefaultConfig()),
		walship.WithCleanupConfig(walship.DefaultCleanupConfig()),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Verify final state
	if w.Status() != walship.StateStopped {
		t.Errorf("Status = %v, want Stopped", w.Status())
	}
}

// =============================================================================
// Cleanup Config Tests
// =============================================================================

func TestCleanupConfig_Enabled(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	cleanupCfg := walship.CleanupConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour,
		HighWatermark: 2 << 30,
		LowWatermark:  1 << 30,
	}

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithCleanupConfig(cleanupCfg),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Check that cleanup was enabled
	messages := logger.Messages()
	found := false
	for _, msg := range messages {
		if msg == "[INFO] WAL cleanup enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Error("WAL cleanup should have logged enablement")
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestCleanupConfig_Disabled(t *testing.T) {
	cfg := createTestConfig(t)
	logger := newTestLogger()

	cleanupCfg := walship.CleanupConfig{
		Enabled: false,
	}

	w, err := walship.New(cfg,
		walship.WithLogger(logger),
		walship.WithCleanupConfig(cleanupCfg),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Cleanup should NOT be enabled
	messages := logger.Messages()
	for _, msg := range messages {
		if msg == "[INFO] WAL cleanup enabled" {
			t.Error("WAL cleanup should not be enabled when disabled")
		}
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestCleanupConfig_DefaultValues(t *testing.T) {
	defaultCfg := walship.DefaultCleanupConfig()

	if !defaultCfg.Enabled {
		t.Error("Default cleanup config should be enabled")
	}
	if defaultCfg.CheckInterval != 72*time.Hour {
		t.Errorf("Default CheckInterval = %v, want 72h", defaultCfg.CheckInterval)
	}
	if defaultCfg.HighWatermark != 2<<30 {
		t.Errorf("Default HighWatermark = %d, want %d", defaultCfg.HighWatermark, 2<<30)
	}
	if defaultCfg.LowWatermark != 3<<29 {
		t.Errorf("Default LowWatermark = %d, want %d", defaultCfg.LowWatermark, 3<<29)
	}
}

// =============================================================================
// Event Handler Tests with Plugins
// =============================================================================

func TestPlugin_EventHandlerReceivesStateChanges(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	tracker := newEventTracker()

	var initOrder []string
	var shutdownOrder []string
	plugin := newTrackingPlugin("test-plugin", &initOrder, &shutdownOrder)

	w, err := walship.New(cfg,
		walship.WithEventHandler(tracker),
		walship.WithPlugin(plugin),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}

	// Check state transitions
	changes := tracker.StateChanges()
	if len(changes) < 2 {
		t.Fatalf("Expected at least 2 state changes, got %d", len(changes))
	}

	// First transition should be Stopped -> Starting
	if changes[0].Previous != walship.StateStopped || changes[0].Current != walship.StateStarting {
		t.Errorf("First transition = %v -> %v, want Stopped -> Starting",
			changes[0].Previous, changes[0].Current)
	}

	// Should eventually reach Running
	foundRunning := false
	for _, change := range changes {
		if change.Current == walship.StateRunning {
			foundRunning = true
			break
		}
	}
	if !foundRunning {
		t.Error("Should have transitioned to Running state")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestPlugin_ConcurrentStatusCalls(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Concurrent status calls
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Status()
		}()
	}

	wg.Wait()

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_ConcurrentStartAttempts(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()

	// Try to start concurrently - only one should succeed
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Start(ctx); err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&successCount) != 1 {
		t.Errorf("Expected exactly 1 successful Start(), got %d", successCount)
	}

	if err := w.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}

func TestPlugin_StartStopRace(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Once = false

	w, err := walship.New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	ctx := context.Background()

	// Start
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Race: try to stop while checking status repeatedly
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = w.Stop()
	}()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Status()
		}()
	}

	wg.Wait()

	// Should end in a stable state
	status := w.Status()
	if status != walship.StateStopped && status != walship.StateCrashed {
		t.Errorf("Final status = %v, want Stopped or Crashed", status)
	}
}

// =============================================================================
// BasePlugin Tests
// =============================================================================

func TestBasePlugin_DefaultBehavior(t *testing.T) {
	bp := walship.NewBasePlugin("test-base")

	if bp.Name() != "test-base" {
		t.Errorf("Name() = %v, want test-base", bp.Name())
	}

	ctx := context.Background()
	cfg := walship.PluginConfig{}

	// Initialize should be no-op
	if err := bp.Initialize(ctx, cfg); err != nil {
		t.Errorf("Initialize() = %v, want nil", err)
	}

	// Shutdown should be no-op
	if err := bp.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() = %v, want nil", err)
	}
}

func TestBaseEventHandler_DefaultBehavior(t *testing.T) {
	beh := walship.BaseEventHandler{}

	// All methods should be no-ops (not panic)
	beh.OnStateChange(walship.StateChangeEvent{})
	beh.OnSendSuccess(walship.SendSuccessEvent{})
	beh.OnSendError(walship.SendErrorEvent{})
}

// =============================================================================
// State Transition Tests
// =============================================================================

func TestState_StringRepresentation(t *testing.T) {
	tests := []struct {
		state    walship.State
		expected string
	}{
		{walship.StateStopped, "Stopped"},
		{walship.StateStarting, "Starting"},
		{walship.StateRunning, "Running"},
		{walship.StateStopping, "Stopping"},
		{walship.StateCrashed, "Crashed"},
		{walship.State(99), "Unknown"},
	}

	for _, tc := range tests {
		if got := tc.state.String(); got != tc.expected {
			t.Errorf("State(%d).String() = %q, want %q", tc.state, got, tc.expected)
		}
	}
}

func TestState_CanStart(t *testing.T) {
	if !walship.StateStopped.CanStart() {
		t.Error("StateStopped.CanStart() should be true")
	}
	if !walship.StateCrashed.CanStart() {
		t.Error("StateCrashed.CanStart() should be true")
	}
	if walship.StateRunning.CanStart() {
		t.Error("StateRunning.CanStart() should be false")
	}
	if walship.StateStarting.CanStart() {
		t.Error("StateStarting.CanStart() should be false")
	}
	if walship.StateStopping.CanStart() {
		t.Error("StateStopping.CanStart() should be false")
	}
}

func TestState_CanStop(t *testing.T) {
	if !walship.StateRunning.CanStop() {
		t.Error("StateRunning.CanStop() should be true")
	}
	if !walship.StateStarting.CanStop() {
		t.Error("StateStarting.CanStop() should be true")
	}
	if walship.StateStopped.CanStop() {
		t.Error("StateStopped.CanStop() should be false")
	}
	if walship.StateCrashed.CanStop() {
		t.Error("StateCrashed.CanStop() should be false")
	}
	if walship.StateStopping.CanStop() {
		t.Error("StateStopping.CanStop() should be false")
	}
}

func TestState_IsRunning(t *testing.T) {
	if !walship.StateRunning.IsRunning() {
		t.Error("StateRunning.IsRunning() should be true")
	}
	if walship.StateStopped.IsRunning() {
		t.Error("StateStopped.IsRunning() should be false")
	}
	if walship.StateStarting.IsRunning() {
		t.Error("StateStarting.IsRunning() should be false")
	}
}

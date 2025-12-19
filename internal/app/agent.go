package app

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/bft-labs/walship/internal/domain"
	"github.com/bft-labs/walship/internal/ports"
)

// AgentConfig contains configuration for the agent loop.
type AgentConfig struct {
	PollInterval  time.Duration
	SendInterval  time.Duration
	HardInterval  time.Duration
	MaxBatchBytes int
	Once          bool

	// Metadata for send operations
	ChainID    string
	NodeID     string
	Hostname   string
	OSArch     string
	AuthKey    string
	ServiceURL string
}

// Agent orchestrates the WAL streaming loop.
type Agent struct {
	config    AgentConfig
	reader    ports.FrameReader
	sender    ports.FrameSender
	stateRepo ports.StateRepository
	logger    ports.Logger
	batcher   *Batcher
	emitter   SendEventEmitter
}

// SendEventEmitter is called on send success or failure.
type SendEventEmitter interface {
	OnSendSuccess(frameCount, bytesSent int, duration time.Duration)
	OnSendError(err error, frameCount int, retryable bool)
}

// NewAgent creates a new agent with the given dependencies.
func NewAgent(
	config AgentConfig,
	reader ports.FrameReader,
	sender ports.FrameSender,
	stateRepo ports.StateRepository,
	logger ports.Logger,
	emitter SendEventEmitter,
) *Agent {
	return &Agent{
		config:    config,
		reader:    reader,
		sender:    sender,
		stateRepo: stateRepo,
		logger:    logger,
		batcher:   NewBatcher(config.MaxBatchBytes, config.SendInterval, config.HardInterval),
		emitter:   emitter,
	}
}

// Run executes the main streaming loop.
// It reads frames, batches them, and sends to the remote service.
// Returns when the context is canceled or an unrecoverable error occurs.
func (a *Agent) Run(ctx context.Context) error {
	// Load initial state
	state, err := a.stateRepo.Load(ctx)
	if err != nil {
		a.logger.Error("failed to load state", ports.Err(err))
		// Continue with empty state
	}

	// Open reader
	if err := a.reader.Open(ctx, &state); err != nil {
		return err
	}
	defer a.reader.Close()

	backoff := newBackoff(500*time.Millisecond, 10*time.Second)

	for {
		select {
		case <-ctx.Done():
			// Flush pending batch before exit
			if a.batcher.HasPending() {
				a.trySend(ctx, &state, backoff)
			}
			return ctx.Err()
		default:
		}

		// Read next frame
		frame, compressed, idxLineLen, err := a.reader.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// No more frames available
				// Flush pending batch
				if a.batcher.HasPending() {
					a.trySend(ctx, &state, backoff)
				}

				if a.config.Once {
					return nil
				}

				// Poll for new data
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(a.config.PollInterval):
					continue
				}
			}

			// Other error, log and retry
			a.logger.Error("read error", ports.Err(err))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.config.PollInterval):
				continue
			}
		}

		// Add frame to batch
		shouldSend := a.batcher.Add(frame, compressed, idxLineLen)

		// Check if we should send
		if shouldSend || a.batcher.ShouldSend() {
			a.trySend(ctx, &state, backoff)
		}
	}
}

// trySend attempts to send the current batch.
func (a *Agent) trySend(ctx context.Context, state *domain.State, backoff *backoff) {
	batch := a.batcher.Batch()
	if batch.Empty() {
		return
	}

	metadata := ports.SendMetadata{
		ChainID:    a.config.ChainID,
		NodeID:     a.config.NodeID,
		Hostname:   a.config.Hostname,
		OSArch:     a.config.OSArch,
		AuthKey:    a.config.AuthKey,
		ServiceURL: a.config.ServiceURL,
	}

	start := time.Now()
	err := a.sender.Send(ctx, batch, metadata)
	duration := time.Since(start)

	if err != nil {
		a.logger.Error("send failed",
			ports.Err(err),
			ports.Int("frames", batch.Size()),
			ports.Int("bytes", batch.TotalBytes),
		)

		if a.emitter != nil {
			a.emitter.OnSendError(err, batch.Size(), true)
		}

		backoff.Sleep()
		return
	}

	// Success
	a.logger.Info("sent batch",
		ports.Int("frames", batch.Size()),
		ports.Int("bytes", batch.TotalBytes),
		ports.Duration("duration", duration),
	)

	if a.emitter != nil {
		a.emitter.OnSendSuccess(batch.Size(), batch.TotalBytes, duration)
	}

	// Update state
	lastFrame := batch.LastFrame()
	if lastFrame != nil {
		state.UpdateAfterSend(batch.TotalIdxAdvance(), lastFrame.File, lastFrame.FrameNumber)
	}

	// Update position from reader
	idxPath, idxOffset, curGz := a.reader.CurrentPosition()
	state.IdxPath = idxPath
	state.IdxOffset = idxOffset
	state.CurGz = curGz

	// Persist state
	if err := a.stateRepo.Save(ctx, *state); err != nil {
		a.logger.Error("failed to save state", ports.Err(err))
	}

	// Reset batch and backoff
	a.batcher.Reset()
	backoff.Reset()
}

// Flush sends any pending frames immediately.
func (a *Agent) Flush(ctx context.Context, state *domain.State) error {
	if !a.batcher.HasPending() {
		return nil
	}

	batch := a.batcher.Batch()
	metadata := ports.SendMetadata{
		ChainID:    a.config.ChainID,
		NodeID:     a.config.NodeID,
		Hostname:   a.config.Hostname,
		OSArch:     a.config.OSArch,
		AuthKey:    a.config.AuthKey,
		ServiceURL: a.config.ServiceURL,
	}

	if err := a.sender.Send(ctx, batch, metadata); err != nil {
		return err
	}

	// Update state
	lastFrame := batch.LastFrame()
	if lastFrame != nil {
		state.UpdateAfterSend(batch.TotalIdxAdvance(), lastFrame.File, lastFrame.FrameNumber)
	}

	// Persist state
	if err := a.stateRepo.Save(ctx, *state); err != nil {
		a.logger.Error("failed to save state on flush", ports.Err(err))
	}

	a.batcher.Reset()
	return nil
}

package lifecycle

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/bft-labs/walship/pkg/batch"
	"github.com/bft-labs/walship/pkg/log"
	"github.com/bft-labs/walship/pkg/sender"
	"github.com/bft-labs/walship/pkg/state"
	"github.com/bft-labs/walship/pkg/wal"
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

// SendEventEmitter is called on send success or failure.
type SendEventEmitter interface {
	OnSendSuccess(frameCount, bytesSent int, duration time.Duration)
	OnSendError(err error, frameCount int, retryable bool)
}

// Agent orchestrates the WAL streaming loop.
type Agent struct {
	config    AgentConfig
	reader    wal.Reader
	sender    sender.Sender
	stateRepo state.Repository
	logger    log.Logger
	batcher   *batch.DefaultBatcher
	emitter   SendEventEmitter
}

// NewAgent creates a new agent with the given dependencies.
func NewAgent(
	config AgentConfig,
	reader wal.Reader,
	snd sender.Sender,
	stateRepo state.Repository,
	logger log.Logger,
	emitter SendEventEmitter,
) *Agent {
	return &Agent{
		config:    config,
		reader:    reader,
		sender:    snd,
		stateRepo: stateRepo,
		logger:    logger,
		batcher:   batch.NewDefaultBatcher(config.MaxBatchBytes, config.SendInterval, config.HardInterval),
		emitter:   emitter,
	}
}

// Run executes the main streaming loop.
// It reads frames, batches them, and sends to the remote service.
// Returns when the context is canceled or an unrecoverable error occurs.
func (a *Agent) Run(ctx context.Context) error {
	// Load initial state
	st, err := a.stateRepo.Load(ctx)
	if err != nil {
		a.logger.Error("failed to load state", log.Err(err))
		// Continue with empty state
	}

	// Open reader
	if err := a.reader.Open(ctx, st.IdxPath, st.IdxOffset, st.CurGz); err != nil {
		return err
	}
	defer a.reader.Close()

	backoff := NewBackoff(500*time.Millisecond, 10*time.Second)

	for {
		select {
		case <-ctx.Done():
			// Flush pending batch before exit
			if a.batcher.HasPending() {
				a.trySend(ctx, &st, backoff)
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
					a.trySend(ctx, &st, backoff)
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
			a.logger.Error("read error", log.Err(err))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.config.PollInterval):
				continue
			}
		}

		// Add frame to batch
		shouldSend := a.batcher.AddWithSizeCheck(frame, compressed, idxLineLen)

		// Check if we should send
		if shouldSend || a.batcher.ShouldSend() {
			a.trySend(ctx, &st, backoff)
		}
	}
}

// trySend attempts to send the current batch.
func (a *Agent) trySend(ctx context.Context, st *state.State, backoff *Backoff) {
	b := a.batcher.Batch()
	if b.Empty() {
		return
	}

	metadata := sender.Metadata{
		ChainID:    a.config.ChainID,
		NodeID:     a.config.NodeID,
		Hostname:   a.config.Hostname,
		OSArch:     a.config.OSArch,
		AuthKey:    a.config.AuthKey,
		ServiceURL: a.config.ServiceURL,
	}

	start := time.Now()
	err := a.sender.Send(ctx, b, metadata)
	duration := time.Since(start)

	if err != nil {
		a.logger.Error("send failed",
			log.Err(err),
			log.Int("frames", b.Size()),
			log.Int("bytes", b.TotalBytes),
		)

		if a.emitter != nil {
			a.emitter.OnSendError(err, b.Size(), true)
		}

		backoff.Sleep()
		return
	}

	// Success
	a.logger.Info("sent batch",
		log.Int("frames", b.Size()),
		log.Int("bytes", b.TotalBytes),
		log.Duration("duration", duration),
	)

	if a.emitter != nil {
		a.emitter.OnSendSuccess(b.Size(), b.TotalBytes, duration)
	}

	// Update state
	lastFrame := b.LastFrame()
	if lastFrame != nil {
		st.UpdateAfterSend(b.TotalIdxAdvance(), lastFrame.File, lastFrame.FrameNumber)
	}

	// Update position from reader
	idxPath, idxOffset, curGz := a.reader.CurrentPosition()
	st.IdxPath = idxPath
	st.IdxOffset = idxOffset
	st.CurGz = curGz

	// Persist state
	if err := a.stateRepo.Save(ctx, *st); err != nil {
		a.logger.Error("failed to save state", log.Err(err))
	}

	// Reset batch and backoff
	a.batcher.Reset()
	backoff.Reset()
}

// Flush sends any pending frames immediately.
func (a *Agent) Flush(ctx context.Context, st *state.State) error {
	if !a.batcher.HasPending() {
		return nil
	}

	b := a.batcher.Batch()
	metadata := sender.Metadata{
		ChainID:    a.config.ChainID,
		NodeID:     a.config.NodeID,
		Hostname:   a.config.Hostname,
		OSArch:     a.config.OSArch,
		AuthKey:    a.config.AuthKey,
		ServiceURL: a.config.ServiceURL,
	}

	if err := a.sender.Send(ctx, b, metadata); err != nil {
		return err
	}

	// Update state
	lastFrame := b.LastFrame()
	if lastFrame != nil {
		st.UpdateAfterSend(b.TotalIdxAdvance(), lastFrame.File, lastFrame.FrameNumber)
	}

	// Persist state
	if err := a.stateRepo.Save(ctx, *st); err != nil {
		a.logger.Error("failed to save state on flush", log.Err(err))
	}

	a.batcher.Reset()
	return nil
}

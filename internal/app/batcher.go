package app

import (
	"time"

	"github.com/bft-labs/walship/internal/domain"
)

// Batcher manages the batching of frames for sending.
type Batcher struct {
	batch         *domain.Batch
	maxBatchBytes int
	sendInterval  time.Duration
	hardInterval  time.Duration
	lastSend      time.Time
}

// NewBatcher creates a new batcher with the given configuration.
func NewBatcher(maxBatchBytes int, sendInterval, hardInterval time.Duration) *Batcher {
	return &Batcher{
		batch:         domain.NewBatch(),
		maxBatchBytes: maxBatchBytes,
		sendInterval:  sendInterval,
		hardInterval:  hardInterval,
		lastSend:      time.Now(),
	}
}

// Add adds a frame to the batch.
// Returns true if the batch should be sent after this add (size trigger).
func (b *Batcher) Add(frame domain.Frame, compressed []byte, idxLineLen int) bool {
	// Check if this single frame exceeds max batch size
	if b.maxBatchBytes > 0 && len(compressed) > b.maxBatchBytes {
		// Large frame: will send alone
		b.batch.Add(frame, compressed, idxLineLen)
		return true
	}

	// Check if adding this frame would exceed max batch size
	if b.maxBatchBytes > 0 && b.batch.TotalBytes+len(compressed) > b.maxBatchBytes {
		// Don't add yet, signal to send current batch first
		return true
	}

	// Add to batch
	b.batch.Add(frame, compressed, idxLineLen)
	return false
}

// ShouldSend returns true if the batch should be sent based on time triggers.
func (b *Batcher) ShouldSend() bool {
	if b.batch.Empty() {
		return false
	}

	elapsed := time.Since(b.lastSend)
	return elapsed >= b.sendInterval || elapsed >= b.hardInterval
}

// ShouldForceSend returns true if the hard interval has been exceeded.
func (b *Batcher) ShouldForceSend() bool {
	if b.batch.Empty() {
		return false
	}
	return time.Since(b.lastSend) >= b.hardInterval
}

// Batch returns the current batch.
func (b *Batcher) Batch() *domain.Batch {
	return b.batch
}

// Reset clears the batch and updates the last send time.
func (b *Batcher) Reset() {
	b.batch.Reset()
	b.lastSend = time.Now()
}

// HasPending returns true if there are frames waiting to be sent.
func (b *Batcher) HasPending() bool {
	return !b.batch.Empty()
}

// TimeSinceLastSend returns the duration since the last send.
func (b *Batcher) TimeSinceLastSend() time.Duration {
	return time.Since(b.lastSend)
}

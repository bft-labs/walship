package batch

import (
	"time"

	"github.com/bft-labs/walship/pkg/wal"
)

// DefaultBatcher manages the batching of frames for sending.
type DefaultBatcher struct {
	batch         *Batch
	maxBatchBytes int
	sendInterval  time.Duration
	hardInterval  time.Duration
	lastSend      time.Time
}

// NewDefaultBatcher creates a new batcher with the given configuration.
func NewDefaultBatcher(maxBatchBytes int, sendInterval, hardInterval time.Duration) *DefaultBatcher {
	return &DefaultBatcher{
		batch:         NewBatch(),
		maxBatchBytes: maxBatchBytes,
		sendInterval:  sendInterval,
		hardInterval:  hardInterval,
		lastSend:      time.Now(),
	}
}

// Add adds a frame to the batch.
func (b *DefaultBatcher) Add(frame wal.Frame, compressed []byte, idxLineLen int) {
	b.batch.Add(frame, compressed, idxLineLen)
}

// AddWithSizeCheck adds a frame and returns true if batch should be sent after this add.
// This method is useful when you need to check size triggers.
func (b *DefaultBatcher) AddWithSizeCheck(frame wal.Frame, compressed []byte, idxLineLen int) bool {
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
func (b *DefaultBatcher) ShouldSend() bool {
	if b.batch.Empty() {
		return false
	}

	elapsed := time.Since(b.lastSend)
	return elapsed >= b.sendInterval || elapsed >= b.hardInterval
}

// ShouldForceSend returns true if the hard interval has been exceeded.
func (b *DefaultBatcher) ShouldForceSend() bool {
	if b.batch.Empty() {
		return false
	}
	return time.Since(b.lastSend) >= b.hardInterval
}

// Batch returns the current batch.
func (b *DefaultBatcher) Batch() *Batch {
	return b.batch
}

// Reset clears the batch and updates the last send time.
func (b *DefaultBatcher) Reset() {
	b.batch.Reset()
	b.lastSend = time.Now()
}

// HasPending returns true if there are frames waiting to be sent.
func (b *DefaultBatcher) HasPending() bool {
	return !b.batch.Empty()
}

// TimeSinceLastSend returns the duration since the last send.
func (b *DefaultBatcher) TimeSinceLastSend() time.Duration {
	return time.Since(b.lastSend)
}

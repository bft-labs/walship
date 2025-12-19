package batch

import "github.com/bft-labs/walship/pkg/wal"

// Batcher accumulates frames until a batch is ready to be sent.
// It tracks size limits and decides when to flush.
type Batcher interface {
	// Add adds a frame to the current batch.
	Add(frame wal.Frame, compressed []byte, idxLineLen int)

	// ShouldSend returns true if the batch should be sent.
	// This is typically based on size or frame count thresholds.
	ShouldSend() bool

	// Batch returns the current batch ready for sending.
	Batch() *Batch

	// Reset clears the batcher for a new batch.
	Reset()

	// HasPending returns true if there are frames waiting to be sent.
	HasPending() bool
}

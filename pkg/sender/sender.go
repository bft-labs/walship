package sender

import (
	"context"

	"github.com/bft-labs/walship/pkg/batch"
)

// Sender transmits frame batches to an ingestion service.
// Implementations handle serialization, communication, and authentication.
type Sender interface {
	// Send transmits a batch of frames to the remote service.
	// Returns nil on success, error on failure.
	// The implementation should handle retries with backoff internally
	// or return an error for the caller to handle.
	Send(ctx context.Context, b *batch.Batch, metadata Metadata) error
}

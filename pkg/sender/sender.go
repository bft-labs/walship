package sender

import (
	"context"

	"github.com/bft-labs/walship/pkg/wal"
)

// FrameData represents a frame with its compressed data for sending.
type FrameData struct {
	// Frame contains the frame metadata
	Frame wal.Frame

	// CompressedData is the raw compressed bytes
	CompressedData []byte
}

// Sender transmits frames to an ingestion service.
// Implementations handle serialization, communication, and authentication.
// Batching is handled internally by the caller; Sender receives ready-to-send frames.
type Sender interface {
	// Send transmits frames to the remote service.
	// Returns nil on success, error on failure.
	// The implementation should handle retries with backoff internally
	// or return an error for the caller to handle.
	Send(ctx context.Context, frames []FrameData, metadata Metadata) error
}

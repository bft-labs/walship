package ports

import (
	"context"

	"github.com/bft-labs/walship/internal/domain"
)

// FrameSender transmits frame batches to the ingestion service.
// Implementations handle serialization, HTTP communication, and authentication.
type FrameSender interface {
	// Send transmits a batch of frames to the remote service.
	// Returns nil on success, error on failure.
	// The implementation should handle retries with backoff internally
	// or return an error for the caller to handle.
	Send(ctx context.Context, batch *domain.Batch, metadata SendMetadata) error
}

// SendMetadata provides context for the send operation.
// This information is included in HTTP headers for server-side tracking.
type SendMetadata struct {
	// ChainID is the blockchain chain identifier
	ChainID string

	// NodeID is the node identifier
	NodeID string

	// Hostname is the agent's hostname
	Hostname string

	// OSArch is the operating system and architecture (e.g., "linux/amd64")
	OSArch string

	// AuthKey is the API authentication key
	AuthKey string

	// ServiceURL is the base URL of the ingestion service
	ServiceURL string
}

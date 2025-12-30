package ports

import (
	"github.com/bft-labs/walship/pkg/sender"
)

// FrameSender transmits frame data to the ingestion service.
// Implementations handle serialization, HTTP communication, and authentication.
// This is an alias to the public sender.Sender interface.
type FrameSender = sender.Sender

// SendMetadata provides context for the send operation.
// This is an alias to the public sender.Metadata type.
type SendMetadata = sender.Metadata

// FrameData represents a frame with its compressed data for sending.
// This is an alias to the public sender.FrameData type.
type FrameData = sender.FrameData

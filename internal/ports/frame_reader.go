package ports

import (
	"context"
	"io"

	"github.com/bft-labs/walship/internal/domain"
)

// FrameReader provides access to WAL frame data from index files.
// Implementations read from the WAL index (.wal.idx) and corresponding
// compressed data files (.wal.gz).
type FrameReader interface {
	// Open prepares the reader starting from the given state.
	// If state is nil or empty, starts from the oldest available index.
	Open(ctx context.Context, state *domain.State) error

	// Next returns the next frame and its compressed data.
	// Returns io.EOF when no more frames are available (should poll and retry).
	// Returns other errors for unrecoverable issues.
	Next(ctx context.Context) (domain.Frame, []byte, int, error)

	// CurrentPosition returns the current reading position for state persistence.
	// Returns (idxPath, idxOffset, curGz).
	CurrentPosition() (string, int64, string)

	// Close releases all resources held by the reader.
	Close() error
}

// ErrNoMoreFrames indicates that there are no more frames to read.
// The caller should poll and retry after a delay.
var ErrNoMoreFrames = io.EOF

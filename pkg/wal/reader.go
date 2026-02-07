package wal

import (
	"context"
	"io"
)

// ErrNoMoreFrames indicates that there are no more frames to read.
// The caller should poll and retry after a delay.
var ErrNoMoreFrames = io.EOF

// Reader provides access to WAL frame data from index files.
// Implementations read from the WAL index (.wal.idx) and corresponding
// compressed data files (.wal.gz).
type Reader interface {
	// Open prepares the reader starting from the given state.
	// If state is nil or empty, starts from the oldest available index.
	Open(ctx context.Context, idxPath string, idxOffset int64, curGz string) error

	// Next returns the next frame and its compressed data.
	// Returns io.EOF when no more frames are available (should poll and retry).
	// Returns other errors for unrecoverable issues.
	// The third return value is the index line length for offset tracking.
	Next(ctx context.Context) (Frame, []byte, int, error)

	// CurrentPosition returns the current reading position for state persistence.
	// Returns (idxPath, idxOffset, curGz).
	CurrentPosition() (string, int64, string)

	// Close releases all resources held by the reader.
	Close() error
}

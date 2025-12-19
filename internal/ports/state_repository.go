package ports

import (
	"context"

	"github.com/bft-labs/walship/internal/domain"
)

// StateRepository handles state persistence for crash recovery.
// Implementations persist state to disk (or other storage) atomically.
type StateRepository interface {
	// Load retrieves the last saved state.
	// Returns an empty state and nil error if no state exists.
	// Returns an error only for actual read failures.
	Load(ctx context.Context) (domain.State, error)

	// Save persists the current state atomically.
	// The implementation should use atomic writes (e.g., write to temp file, then rename)
	// to prevent corruption on crash.
	Save(ctx context.Context, state domain.State) error
}

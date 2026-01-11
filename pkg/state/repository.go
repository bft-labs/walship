package state

import "context"

// Repository handles state persistence for crash recovery.
// Implementations persist state to disk (or other storage) atomically.
type Repository interface {
	// Load retrieves the last saved state.
	// Returns an empty state and nil error if no state exists.
	// Returns an error only for actual read failures.
	Load(ctx context.Context) (State, error)

	// Save persists the current state atomically.
	// The implementation should use atomic writes (e.g., write to temp file, then rename)
	// to prevent corruption on crash.
	Save(ctx context.Context, state State) error
}

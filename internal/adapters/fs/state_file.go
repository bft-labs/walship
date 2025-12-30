package fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/bft-labs/walship/internal/domain"
)

const stateFileName = "status.json"

// StateFileRepository implements ports.StateRepository using a JSON file.
type StateFileRepository struct {
	dir string
}

// NewStateFileRepository creates a new StateFileRepository for the given directory.
func NewStateFileRepository(dir string) *StateFileRepository {
	return &StateFileRepository{dir: dir}
}

// Load retrieves the last saved state from disk.
// Returns an empty state and nil error if no state file exists.
func (r *StateFileRepository) Load(ctx context.Context) (domain.State, error) {
	path := filepath.Join(r.dir, stateFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.State{}, nil
		}
		return domain.State{}, err
	}

	var state domain.State
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.State{}, err
	}

	return state, nil
}

// Save persists the current state atomically.
// Uses atomic write (write to temp file, then rename) to prevent corruption.
func (r *StateFileRepository) Save(ctx context.Context, state domain.State) error {
	// Ensure directory exists
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(r.dir, stateFileName)
	tmp := path + ".tmp"

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tmp, path)
}

// Path returns the full path to the state file.
func (r *StateFileRepository) Path() string {
	return filepath.Join(r.dir, stateFileName)
}

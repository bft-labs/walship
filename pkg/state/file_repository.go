package state

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

const stateFileName = "status.json"

// FileRepository implements Repository using a JSON file.
type FileRepository struct {
	dir string
}

// NewFileRepository creates a new FileRepository for the given directory.
func NewFileRepository(dir string) *FileRepository {
	return &FileRepository{dir: dir}
}

// Load retrieves the last saved state from disk.
// Returns an empty state and nil error if no state file exists.
func (r *FileRepository) Load(ctx context.Context) (State, error) {
	path := filepath.Join(r.dir, stateFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}

	return state, nil
}

// Save persists the current state atomically.
// Uses atomic write (write to temp file, then rename) to prevent corruption.
func (r *FileRepository) Save(ctx context.Context, state State) error {
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
func (r *FileRepository) Path() string {
	return filepath.Join(r.dir, stateFileName)
}

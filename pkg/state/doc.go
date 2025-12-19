// Package state provides state persistence functionality for resumable streaming.
//
// This package manages loading and saving of streaming state, enabling
// resumption after restarts. The state tracks the current position in
// WAL files and the last successfully sent frame.
//
// # Usage
//
// Create a file-based repository:
//
//	repo := state.NewFileRepository("/path/to/state/dir")
//
//	// Load existing state
//	s, err := repo.Load(ctx)
//	if err != nil {
//	    return err
//	}
//
//	// ... do work ...
//
//	// Save updated state
//	if err := repo.Save(ctx, s); err != nil {
//	    return err
//	}
//
// # Backward Compatibility
//
// State JSON uses snake_case field names for compatibility with existing
// state files from previous walship versions.
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package state

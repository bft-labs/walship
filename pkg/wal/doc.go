// Package wal provides WAL (Write-Ahead Log) reading functionality.
//
// This package enables reading WAL index files and extracting compressed frames.
// It is designed as an independent module that can be imported without pulling
// in HTTP sending or other unrelated dependencies.
//
// # Usage
//
// Create a Reader to read frames from a WAL directory:
//
//	reader := wal.NewIndexReader("/path/to/wal", logger)
//	if err := reader.Open(ctx, nil); err != nil {
//	    return err
//	}
//	defer reader.Close()
//
//	for {
//	    frame, data, lineLen, err := reader.Next(ctx)
//	    if err == io.EOF {
//	        break
//	    }
//	    // Process frame...
//	}
//
// # Interfaces
//
// The Reader interface allows custom implementations for different WAL formats.
// The default IndexReader reads the standard walship index format.
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package wal

// Package batch provides frame batching functionality for efficient transmission.
//
// This package manages the aggregation of WAL frames into batches based on
// size limits and time intervals. It can be used independently of the sender
// module to build custom pipelines.
//
// # Usage
//
// Create a Batcher to accumulate frames:
//
//	batcher := batch.NewBatcher(4<<20, 5*time.Second, 10*time.Second)
//
//	for frame := range frames {
//	    if batcher.Add(frame, data, lineLen) || batcher.ShouldSend() {
//	        b := batcher.Batch()
//	        // Send batch...
//	        batcher.Reset()
//	    }
//	}
//
// # Configuration
//
// - MaxBatchBytes: Maximum compressed bytes per batch
// - SendInterval: Soft interval for time-based sends
// - HardInterval: Hard interval that overrides gating
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package batch

// Package lifecycle provides orchestration and state machine functionality.
//
// This package manages the lifecycle of the streaming agent, including
// state transitions (Stopped, Starting, Running, Stopping, Crashed),
// graceful shutdown with timeout, and worker coordination.
//
// # Usage
//
// Create a lifecycle manager:
//
//	manager := lifecycle.NewManager(logger, eventEmitter)
//
//	if !manager.CanStart() {
//	    return ErrAlreadyRunning
//	}
//
//	if err := manager.TransitionTo(lifecycle.StateStarting, "starting"); err != nil {
//	    return err
//	}
//
//	// ... do work in goroutines ...
//
//	// Graceful shutdown
//	if err := manager.WaitWithTimeout(30 * time.Second); err != nil {
//	    return ErrShutdownTimeout
//	}
//
// # State Machine
//
// Valid state transitions:
//   - Stopped -> Starting
//   - Starting -> Running, Crashed
//   - Running -> Stopping, Crashed
//   - Stopping -> Stopped, Crashed
//   - Crashed -> Starting
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package lifecycle

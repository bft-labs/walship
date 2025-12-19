package lifecycle

import "time"

// State represents the lifecycle state of an agent or component.
type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
	StateCrashed
)

// String returns a human-readable representation of the state.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "Stopped"
	case StateStarting:
		return "Starting"
	case StateRunning:
		return "Running"
	case StateStopping:
		return "Stopping"
	case StateCrashed:
		return "Crashed"
	default:
		return "Unknown"
	}
}

// EventEmitter is called when lifecycle state changes.
type EventEmitter interface {
	OnStateChange(previous, current State, reason string)
}

// Manager manages the lifecycle state machine for an agent or component.
type Manager interface {
	// State returns the current lifecycle state.
	State() State

	// CanStart returns true if Start() can be called.
	CanStart() bool

	// CanStop returns true if Stop() can be called.
	CanStop() bool

	// TransitionTo attempts to transition to a new state.
	// Returns an error if the transition is not valid.
	TransitionTo(newState State, reason string) error

	// WaitWithTimeout waits for all workers to finish with a timeout.
	// Returns ErrShutdownTimeout if the timeout expires.
	WaitWithTimeout(timeout time.Duration) error

	// AddWorker increments the worker count.
	AddWorker()

	// WorkerDone decrements the worker count.
	WorkerDone()
}

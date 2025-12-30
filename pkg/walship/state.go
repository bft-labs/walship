package walship

// State represents the lifecycle state of a Walship instance.
type State int

const (
	// StateStopped indicates the agent is not running.
	StateStopped State = iota

	// StateStarting indicates initialization is in progress.
	StateStarting

	// StateRunning indicates the agent is actively streaming.
	StateRunning

	// StateStopping indicates graceful shutdown is in progress.
	StateStopping

	// StateCrashed indicates the agent terminated due to an error.
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

// IsRunning returns true if the agent is in a running state.
func (s State) IsRunning() bool {
	return s == StateRunning
}

// CanStart returns true if the agent can be started from this state.
func (s State) CanStart() bool {
	return s == StateStopped || s == StateCrashed
}

// CanStop returns true if the agent can be stopped from this state.
func (s State) CanStop() bool {
	return s == StateRunning || s == StateStarting
}

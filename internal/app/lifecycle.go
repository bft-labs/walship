package app

import (
	"context"
	"sync"
	"time"

	"github.com/bft-labs/walship/internal/domain"
	"github.com/bft-labs/walship/internal/ports"
)

// ShutdownTimeout is the maximum time to wait for graceful shutdown.
const ShutdownTimeout = 30 * time.Second

// State represents the lifecycle state of the agent.
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

// Lifecycle manages the state machine for the agent.
type Lifecycle struct {
	mu           sync.RWMutex
	state        State
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logger       ports.Logger
	eventEmitter EventEmitter
}

// EventEmitter is called when lifecycle state changes.
type EventEmitter interface {
	OnStateChange(previous, current State, reason string)
}

// NewLifecycle creates a new lifecycle manager.
func NewLifecycle(logger ports.Logger, emitter EventEmitter) *Lifecycle {
	return &Lifecycle{
		state:        StateStopped,
		logger:       logger,
		eventEmitter: emitter,
	}
}

// State returns the current lifecycle state.
func (l *Lifecycle) State() State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// TransitionTo attempts to transition to a new state.
// Returns an error if the transition is not valid.
func (l *Lifecycle) TransitionTo(newState State, reason string) error {
	l.mu.Lock()
	oldState := l.state

	// Validate transition
	switch oldState {
	case StateStopped:
		if newState != StateStarting {
			l.mu.Unlock()
			return domain.ErrNotRunning
		}
	case StateStarting:
		if newState != StateRunning && newState != StateCrashed {
			l.mu.Unlock()
			return domain.ErrAlreadyRunning
		}
	case StateRunning:
		if newState != StateStopping && newState != StateCrashed {
			l.mu.Unlock()
			return domain.ErrAlreadyRunning
		}
	case StateStopping:
		if newState != StateStopped && newState != StateCrashed {
			l.mu.Unlock()
			return domain.ErrAlreadyRunning
		}
	case StateCrashed:
		if newState != StateStarting {
			l.mu.Unlock()
			return domain.ErrNotRunning
		}
	}

	l.state = newState
	l.mu.Unlock()

	// Emit event outside of lock
	if l.eventEmitter != nil {
		l.eventEmitter.OnStateChange(oldState, newState, reason)
	}

	l.logger.Info("state transition",
		ports.String("from", oldState.String()),
		ports.String("to", newState.String()),
		ports.String("reason", reason),
	)

	return nil
}

// CanStart returns true if Start() can be called.
func (l *Lifecycle) CanStart() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state == StateStopped || l.state == StateCrashed
}

// CanStop returns true if Stop() can be called.
func (l *Lifecycle) CanStop() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state == StateRunning || l.state == StateStarting
}

// SetCancel stores the cancel function for graceful shutdown.
func (l *Lifecycle) SetCancel(cancel context.CancelFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancel = cancel
}

// Cancel triggers graceful shutdown.
func (l *Lifecycle) Cancel() {
	l.mu.Lock()
	cancel := l.cancel
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// AddWorker increments the worker count.
func (l *Lifecycle) AddWorker() {
	l.wg.Add(1)
}

// WorkerDone decrements the worker count.
func (l *Lifecycle) WorkerDone() {
	l.wg.Done()
}

// WaitWithTimeout waits for all workers to finish with a timeout.
// Returns ErrShutdownTimeout if the timeout expires.
func (l *Lifecycle) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		l.logger.Warn("shutdown timeout, forcing exit",
			ports.Duration("timeout", timeout),
		)
		return domain.ErrShutdownTimeout
	}
}

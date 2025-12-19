package lifecycle

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bft-labs/walship/pkg/log"
)

// Common lifecycle errors.
var (
	ErrNotRunning      = errors.New("not running")
	ErrAlreadyRunning  = errors.New("already running")
	ErrShutdownTimeout = errors.New("shutdown timeout")
)

// ShutdownTimeout is the default maximum time to wait for graceful shutdown.
const ShutdownTimeout = 30 * time.Second

// DefaultManager implements Manager with a state machine for lifecycle management.
type DefaultManager struct {
	mu           sync.RWMutex
	state        State
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logger       log.Logger
	eventEmitter EventEmitter
}

// NewManager creates a new lifecycle manager.
func NewManager(logger log.Logger, emitter EventEmitter) *DefaultManager {
	return &DefaultManager{
		state:        StateStopped,
		logger:       logger,
		eventEmitter: emitter,
	}
}

// State returns the current lifecycle state.
func (l *DefaultManager) State() State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// TransitionTo attempts to transition to a new state.
// Returns an error if the transition is not valid.
func (l *DefaultManager) TransitionTo(newState State, reason string) error {
	l.mu.Lock()
	oldState := l.state

	// Validate transition
	switch oldState {
	case StateStopped:
		if newState != StateStarting {
			l.mu.Unlock()
			return ErrNotRunning
		}
	case StateStarting:
		if newState != StateRunning && newState != StateCrashed {
			l.mu.Unlock()
			return ErrAlreadyRunning
		}
	case StateRunning:
		if newState != StateStopping && newState != StateCrashed {
			l.mu.Unlock()
			return ErrAlreadyRunning
		}
	case StateStopping:
		if newState != StateStopped && newState != StateCrashed {
			l.mu.Unlock()
			return ErrAlreadyRunning
		}
	case StateCrashed:
		if newState != StateStarting {
			l.mu.Unlock()
			return ErrNotRunning
		}
	}

	l.state = newState
	l.mu.Unlock()

	// Emit event outside of lock
	if l.eventEmitter != nil {
		l.eventEmitter.OnStateChange(oldState, newState, reason)
	}

	l.logger.Info("state transition",
		log.String("from", oldState.String()),
		log.String("to", newState.String()),
		log.String("reason", reason),
	)

	return nil
}

// CanStart returns true if Start() can be called.
func (l *DefaultManager) CanStart() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state == StateStopped || l.state == StateCrashed
}

// CanStop returns true if Stop() can be called.
func (l *DefaultManager) CanStop() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state == StateRunning || l.state == StateStarting
}

// SetCancel stores the cancel function for graceful shutdown.
func (l *DefaultManager) SetCancel(cancel context.CancelFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cancel = cancel
}

// Cancel triggers graceful shutdown.
func (l *DefaultManager) Cancel() {
	l.mu.Lock()
	cancel := l.cancel
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// AddWorker increments the worker count.
func (l *DefaultManager) AddWorker() {
	l.wg.Add(1)
}

// WorkerDone decrements the worker count.
func (l *DefaultManager) WorkerDone() {
	l.wg.Done()
}

// WaitWithTimeout waits for all workers to finish with a timeout.
// Returns ErrShutdownTimeout if the timeout expires.
func (l *DefaultManager) WaitWithTimeout(timeout time.Duration) error {
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
			log.Duration("timeout", timeout),
		)
		return ErrShutdownTimeout
	}
}

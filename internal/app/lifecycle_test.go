package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bft-labs/walship/internal/domain"
	"github.com/bft-labs/walship/internal/ports"
)

// mockLogger implements ports.Logger for testing.
type mockLogger struct{}

func (mockLogger) Debug(msg string, fields ...ports.Field) {}
func (mockLogger) Info(msg string, fields ...ports.Field)  {}
func (mockLogger) Warn(msg string, fields ...ports.Field)  {}
func (mockLogger) Error(msg string, fields ...ports.Field) {}

// mockEmitter tracks state change events for testing.
type mockEmitter struct {
	mu     sync.Mutex
	events []stateChangeEvent
}

type stateChangeEvent struct {
	previous State
	current  State
	reason   string
}

func (m *mockEmitter) OnStateChange(previous, current State, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, stateChangeEvent{previous, current, reason})
}

func (m *mockEmitter) Events() []stateChangeEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]stateChangeEvent{}, m.events...)
}

func TestNewLifecycle(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	if l == nil {
		t.Fatal("NewLifecycle returned nil")
	}
	if l.State() != StateStopped {
		t.Errorf("initial state = %v, want StateStopped", l.State())
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateStopped, "Stopped"},
		{StateStarting, "Starting"},
		{StateRunning, "Running"},
		{StateStopping, "Stopping"},
		{StateCrashed, "Crashed"},
		{State(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("State(%d).String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestLifecycle_TransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     State
		to       State
		wantErr  bool
	}{
		{"stopped to starting", StateStopped, StateStarting, false},
		{"starting to running", StateStarting, StateRunning, false},
		{"starting to stopping", StateStarting, StateStopping, false}, // Added: early stop during startup
		{"starting to crashed", StateStarting, StateCrashed, false},
		{"running to stopping", StateRunning, StateStopping, false},
		{"running to crashed", StateRunning, StateCrashed, false},
		{"stopping to stopped", StateStopping, StateStopped, false},
		{"stopping to crashed", StateStopping, StateCrashed, false},
		{"crashed to starting", StateCrashed, StateStarting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLifecycle(&mockLogger{}, nil)
			l.state = tt.from

			err := l.TransitionTo(tt.to, "test")

			if (err != nil) != tt.wantErr {
				t.Errorf("TransitionTo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && l.State() != tt.to {
				t.Errorf("state = %v after transition, want %v", l.State(), tt.to)
			}
		})
	}
}

func TestLifecycle_TransitionTo_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name    string
		from    State
		to      State
		wantErr error
	}{
		{"stopped to running", StateStopped, StateRunning, domain.ErrNotRunning},
		{"stopped to stopping", StateStopped, StateStopping, domain.ErrNotRunning},
		{"starting to stopped", StateStarting, StateStopped, domain.ErrAlreadyRunning},
		// "starting to stopping" removed - this is now a valid transition for early stop
		{"running to starting", StateRunning, StateStarting, domain.ErrAlreadyRunning},
		{"running to stopped", StateRunning, StateStopped, domain.ErrAlreadyRunning},
		{"stopping to running", StateStopping, StateRunning, domain.ErrAlreadyRunning},
		{"stopping to starting", StateStopping, StateStarting, domain.ErrAlreadyRunning},
		{"crashed to running", StateCrashed, StateRunning, domain.ErrNotRunning},
		{"crashed to stopped", StateCrashed, StateStopped, domain.ErrNotRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLifecycle(&mockLogger{}, nil)
			l.state = tt.from

			err := l.TransitionTo(tt.to, "test")

			if err != tt.wantErr {
				t.Errorf("TransitionTo() error = %v, want %v", err, tt.wantErr)
			}
			// State should not change on invalid transition
			if l.State() != tt.from {
				t.Errorf("state changed to %v on invalid transition, want %v", l.State(), tt.from)
			}
		})
	}
}

func TestLifecycle_TransitionTo_EmitsEvents(t *testing.T) {
	emitter := &mockEmitter{}
	l := NewLifecycle(&mockLogger{}, emitter)

	_ = l.TransitionTo(StateStarting, "start test")
	_ = l.TransitionTo(StateRunning, "running test")

	events := emitter.Events()
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if events[0].previous != StateStopped || events[0].current != StateStarting {
		t.Errorf("event 0: got %v->%v, want Stopped->Starting", events[0].previous, events[0].current)
	}
	if events[1].previous != StateStarting || events[1].current != StateRunning {
		t.Errorf("event 1: got %v->%v, want Starting->Running", events[1].previous, events[1].current)
	}
}

func TestLifecycle_CanStart(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StateStopped, true},
		{StateStarting, false},
		{StateRunning, false},
		{StateStopping, false},
		{StateCrashed, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			l := NewLifecycle(&mockLogger{}, nil)
			l.state = tt.state

			got := l.CanStart()
			if got != tt.want {
				t.Errorf("CanStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLifecycle_CanStop(t *testing.T) {
	tests := []struct {
		state State
		want  bool
	}{
		{StateStopped, false},
		{StateStarting, true},
		{StateRunning, true},
		{StateStopping, false},
		{StateCrashed, false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			l := NewLifecycle(&mockLogger{}, nil)
			l.state = tt.state

			got := l.CanStop()
			if got != tt.want {
				t.Errorf("CanStop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLifecycle_SetCancel_And_Cancel(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	l.SetCancel(cancel)

	// Context should not be canceled yet
	select {
	case <-ctx.Done():
		t.Error("context should not be canceled before Cancel()")
	default:
	}

	l.Cancel()

	// Context should be canceled now
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("context should be canceled after Cancel()")
	}
}

func TestLifecycle_Cancel_NilSafe(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	// Should not panic when cancel is nil
	l.Cancel()
}

func TestLifecycle_WorkerTracking(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	// Start some workers
	for i := 0; i < 3; i++ {
		l.AddWorker()
	}

	// Workers complete
	done := make(chan struct{})
	go func() {
		for i := 0; i < 3; i++ {
			l.WorkerDone()
		}
	}()

	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(time.Second):
		t.Error("workers did not complete in time")
	}
}

func TestLifecycle_WaitWithTimeout_Success(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	l.AddWorker()

	go func() {
		time.Sleep(10 * time.Millisecond)
		l.WorkerDone()
	}()

	err := l.WaitWithTimeout(time.Second)
	if err != nil {
		t.Errorf("WaitWithTimeout() = %v, want nil", err)
	}
}

func TestLifecycle_WaitWithTimeout_Timeout(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	l.AddWorker()
	// Never call WorkerDone

	err := l.WaitWithTimeout(10 * time.Millisecond)
	if err != domain.ErrShutdownTimeout {
		t.Errorf("WaitWithTimeout() = %v, want ErrShutdownTimeout", err)
	}

	// Clean up
	l.WorkerDone()
}

func TestLifecycle_Concurrency(t *testing.T) {
	l := NewLifecycle(&mockLogger{}, nil)

	var wg sync.WaitGroup

	// Concurrent state reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = l.State()
				_ = l.CanStart()
				_ = l.CanStop()
			}
		}()
	}

	// Concurrent transitions (some will fail, which is expected)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = l.TransitionTo(StateStarting, "test")
			_ = l.TransitionTo(StateRunning, "test")
		}()
	}

	wg.Wait()
}

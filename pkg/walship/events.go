package walship

import "time"

// EventHandler receives lifecycle and operational events from the Walship agent.
// Implement this interface to receive notifications about state changes,
// successful sends, and errors. All methods are called synchronously,
// so handlers should return quickly to avoid blocking the agent.
type EventHandler interface {
	// OnStateChange is called when the agent transitions between lifecycle states.
	OnStateChange(event StateChangeEvent)

	// OnSendSuccess is called after frames are successfully transmitted.
	OnSendSuccess(event SendSuccessEvent)

	// OnSendError is called when a send operation fails.
	OnSendError(event SendErrorEvent)
}

// StateChangeEvent contains information about a lifecycle state transition.
type StateChangeEvent struct {
	// Previous is the state before the transition.
	Previous State

	// Current is the new state after the transition.
	Current State

	// Reason describes why the transition occurred.
	Reason string
}

// SendSuccessEvent contains information about a successful frame transmission.
type SendSuccessEvent struct {
	// FrameCount is the number of frames sent in this batch.
	FrameCount int

	// BytesSent is the total bytes transmitted (compressed).
	BytesSent int

	// Duration is how long the send operation took.
	Duration time.Duration
}

// SendErrorEvent contains information about a failed send operation.
type SendErrorEvent struct {
	// Error is the underlying error that caused the failure.
	Error error

	// FrameCount is the number of frames that failed to send.
	FrameCount int

	// Retryable indicates whether the operation will be retried.
	Retryable bool
}

// BaseEventHandler provides a no-op implementation of EventHandler.
// Embed this in custom handlers to only implement methods you care about.
type BaseEventHandler struct{}

// OnStateChange implements EventHandler with a no-op.
func (BaseEventHandler) OnStateChange(event StateChangeEvent) {}

// OnSendSuccess implements EventHandler with a no-op.
func (BaseEventHandler) OnSendSuccess(event SendSuccessEvent) {}

// OnSendError implements EventHandler with a no-op.
func (BaseEventHandler) OnSendError(event SendErrorEvent) {}

// Ensure BaseEventHandler implements EventHandler.
var _ EventHandler = BaseEventHandler{}

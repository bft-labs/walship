package domain

import "errors"

// Domain errors represent error conditions in the walship domain.
// These errors are returned by the public API and can be checked with errors.Is.
var (
	// ErrAlreadyRunning is returned when Start() is called on a running instance.
	ErrAlreadyRunning = errors.New("walship: already running")

	// ErrNotRunning is returned when Stop() is called on a stopped instance.
	ErrNotRunning = errors.New("walship: not running")

	// ErrShutdownTimeout is returned when graceful shutdown times out.
	ErrShutdownTimeout = errors.New("walship: shutdown timeout")

	// ErrInvalidConfig is returned when configuration validation fails.
	ErrInvalidConfig = errors.New("walship: invalid configuration")

	// ErrContextCanceled is returned when the operation context is canceled.
	ErrContextCanceled = errors.New("walship: context canceled")
)

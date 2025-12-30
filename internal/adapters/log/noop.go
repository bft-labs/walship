package log

import "github.com/bft-labs/walship/internal/ports"

// NoopLogger implements ports.Logger by discarding all log messages.
type NoopLogger struct{}

// NewNoopLogger creates a new no-op logger.
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

// Debug discards the message.
func (NoopLogger) Debug(msg string, fields ...ports.Field) {}

// Info discards the message.
func (NoopLogger) Info(msg string, fields ...ports.Field) {}

// Warn discards the message.
func (NoopLogger) Warn(msg string, fields ...ports.Field) {}

// Error discards the message.
func (NoopLogger) Error(msg string, fields ...ports.Field) {}

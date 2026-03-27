package log

// NoopLogger implements Logger by discarding all log messages.
type NoopLogger struct{}

// NewNoopLogger creates a new no-op logger.
func NewNoopLogger() *NoopLogger {
	return &NoopLogger{}
}

// Debug discards the message.
func (NoopLogger) Debug(msg string, fields ...Field) {}

// Info discards the message.
func (NoopLogger) Info(msg string, fields ...Field) {}

// Warn discards the message.
func (NoopLogger) Warn(msg string, fields ...Field) {}

// Error discards the message.
func (NoopLogger) Error(msg string, fields ...Field) {}

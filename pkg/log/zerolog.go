package log

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// ZerologAdapter implements Logger using zerolog.
type ZerologAdapter struct {
	logger zerolog.Logger
}

// NewZerologAdapter creates a new zerolog adapter with console output.
func NewZerologAdapter() *ZerologAdapter {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}
	logger := zerolog.New(output).With().Timestamp().Logger()
	return &ZerologAdapter{logger: logger}
}

// NewZerologAdapterWithLogger creates an adapter wrapping an existing zerolog.Logger.
func NewZerologAdapterWithLogger(logger zerolog.Logger) *ZerologAdapter {
	return &ZerologAdapter{logger: logger}
}

// Debug logs a debug-level message.
func (z *ZerologAdapter) Debug(msg string, fields ...Field) {
	event := z.logger.Debug()
	for _, f := range fields {
		event = addField(event, f)
	}
	event.Msg(msg)
}

// Info logs an info-level message.
func (z *ZerologAdapter) Info(msg string, fields ...Field) {
	event := z.logger.Info()
	for _, f := range fields {
		event = addField(event, f)
	}
	event.Msg(msg)
}

// Warn logs a warning-level message.
func (z *ZerologAdapter) Warn(msg string, fields ...Field) {
	event := z.logger.Warn()
	for _, f := range fields {
		event = addField(event, f)
	}
	event.Msg(msg)
}

// Error logs an error-level message.
func (z *ZerologAdapter) Error(msg string, fields ...Field) {
	event := z.logger.Error()
	for _, f := range fields {
		event = addField(event, f)
	}
	event.Msg(msg)
}

// addField adds a Field to a zerolog.Event.
func addField(event *zerolog.Event, f Field) *zerolog.Event {
	switch v := f.Value.(type) {
	case string:
		return event.Str(f.Key, v)
	case int:
		return event.Int(f.Key, v)
	case int64:
		return event.Int64(f.Key, v)
	case uint64:
		return event.Uint64(f.Key, v)
	case float64:
		return event.Float64(f.Key, v)
	case bool:
		return event.Bool(f.Key, v)
	case time.Duration:
		return event.Dur(f.Key, v)
	case error:
		return event.Err(v)
	default:
		return event.Interface(f.Key, v)
	}
}

// Logger returns the underlying zerolog.Logger.
func (z *ZerologAdapter) Logger() zerolog.Logger {
	return z.logger
}

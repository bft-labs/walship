package log

import "time"

// Logger provides structured logging capabilities.
// Implementations can wrap zerolog, zap, logrus, or any other logging library.
type Logger interface {
	// Debug logs a debug-level message with fields.
	Debug(msg string, fields ...Field)

	// Info logs an info-level message with fields.
	Info(msg string, fields ...Field)

	// Warn logs a warning-level message with fields.
	Warn(msg string, fields ...Field)

	// Error logs an error-level message with fields.
	Error(msg string, fields ...Field)
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Uint64 creates a uint64 field.
func Uint64(key string, value uint64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a duration field.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field with key "error".
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value.
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

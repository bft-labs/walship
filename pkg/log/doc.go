// Package log provides a logging abstraction for walship components.
//
// This package defines a Logger interface that can be implemented by
// any logging library. Default implementations are provided for zerolog
// and a no-op logger for testing.
//
// # Usage
//
// Use the provided zerolog adapter:
//
//	logger := log.NewZerologLogger(zerolog.New(os.Stderr))
//
// Or use the no-op logger for testing:
//
//	logger := log.NewNoopLogger()
//
// # Custom Loggers
//
// Implement the Logger interface to integrate with your existing
// logging infrastructure:
//
//	type MyLogger struct { ... }
//
//	func (l *MyLogger) Debug(msg string, fields ...log.Field) { ... }
//	func (l *MyLogger) Info(msg string, fields ...log.Field) { ... }
//	func (l *MyLogger) Warn(msg string, fields ...log.Field) { ... }
//	func (l *MyLogger) Error(msg string, fields ...log.Field) { ... }
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package log

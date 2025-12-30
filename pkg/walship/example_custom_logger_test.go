package walship_test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/bft-labs/walship/pkg/log"
)

// BufferedLogger demonstrates a custom logger implementation that writes to a buffer.
// This shows that the Logger interface is implementation-agnostic.
type BufferedLogger struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	level  string
}

// NewBufferedLogger creates a new buffered logger for testing.
func NewBufferedLogger(minLevel string) *BufferedLogger {
	return &BufferedLogger{level: minLevel}
}

// Debug implements log.Logger.
func (l *BufferedLogger) Debug(msg string, fields ...log.Field) {
	if l.level == "debug" {
		l.write("DEBUG", msg, fields)
	}
}

// Info implements log.Logger.
func (l *BufferedLogger) Info(msg string, fields ...log.Field) {
	if l.level == "debug" || l.level == "info" {
		l.write("INFO", msg, fields)
	}
}

// Warn implements log.Logger.
func (l *BufferedLogger) Warn(msg string, fields ...log.Field) {
	if l.level != "error" {
		l.write("WARN", msg, fields)
	}
}

// Error implements log.Logger.
func (l *BufferedLogger) Error(msg string, fields ...log.Field) {
	l.write("ERROR", msg, fields)
}

func (l *BufferedLogger) write(level, msg string, fields []log.Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buffer.WriteString(fmt.Sprintf("[%s] %s", level, msg))
	for _, f := range fields {
		l.buffer.WriteString(fmt.Sprintf(" %s=%v", f.Key, f.Value))
	}
	l.buffer.WriteString("\n")
}

// String returns all logged messages.
func (l *BufferedLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

// Reset clears the buffer.
func (l *BufferedLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buffer.Reset()
}

// TestBufferedLoggerImplementsLogger verifies the BufferedLogger implements the Logger interface.
func TestBufferedLoggerImplementsLogger(t *testing.T) {
	var _ log.Logger = (*BufferedLogger)(nil)
}

// TestBufferedLoggerUsage demonstrates using the custom logger.
func TestBufferedLoggerUsage(t *testing.T) {
	logger := NewBufferedLogger("info")

	logger.Info("starting operation", log.String("component", "test"))
	logger.Debug("this should not appear") // level is info, not debug
	logger.Error("something failed", log.Err(fmt.Errorf("test error")))

	output := logger.String()
	if output == "" {
		t.Error("expected log output")
	}

	// Verify info message appears
	if !bytes.Contains([]byte(output), []byte("starting operation")) {
		t.Error("expected info message in output")
	}

	// Verify debug message does not appear
	if bytes.Contains([]byte(output), []byte("this should not appear")) {
		t.Error("debug message should not appear at info level")
	}
}

// ExampleBufferedLogger demonstrates creating a custom logger for testing.
func ExampleBufferedLogger() {
	// Create a custom logger for capturing logs in tests
	testLogger := NewBufferedLogger("debug")

	// Use the logger
	testLogger.Info("application started", log.String("version", "1.0.0"))
	testLogger.Debug("loading config", log.String("path", "/etc/app.toml"))
	testLogger.Error("connection failed", log.Err(fmt.Errorf("timeout")))

	// Inspect captured logs
	fmt.Println(testLogger.String())
}

package walship_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bft-labs/walship/pkg/walship"
)

// ExampleNew demonstrates how to embed walship in your application.
func ExampleNew() {
	// Create configuration
	cfg := walship.Config{
		WALDir:     "/path/to/wal/directory",
		AuthKey:    "your-api-key",
		ChainID:    "my-chain",
		NodeID:     "my-node",
		ServiceURL: "https://api.apphash.io",
	}

	// Create walship instance
	w, err := walship.New(cfg)
	if err != nil {
		fmt.Printf("failed to create walship: %v\n", err)
		return
	}

	// Start streaming (non-blocking)
	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		fmt.Printf("failed to start: %v\n", err)
		return
	}

	// Check status (may be Starting or Running depending on timing)
	status := w.Status()
	fmt.Printf("Status is valid: %v\n", status == walship.StateStarting || status == walship.StateRunning)

	// Stop gracefully (flushes pending data)
	_ = w.Stop()

	// Output: Status is valid: true
}

// Example_withEventHandler demonstrates how to receive walship events.
func Example_withEventHandler() {
	// Custom event handler
	handler := &myEventHandler{}

	cfg := walship.Config{
		WALDir:  "/path/to/wal",
		AuthKey: "api-key",
		ChainID: "chain-id",
		NodeID:  "node-id",
	}

	// Create with event handler
	w, err := walship.New(cfg, walship.WithEventHandler(handler))
	if err != nil {
		fmt.Printf("failed to create walship: %v\n", err)
		return
	}

	_ = w // Use walship instance...
}

// myEventHandler implements walship.EventHandler for event notifications.
type myEventHandler struct {
	walship.BaseEventHandler // Embed for no-op defaults
}

func (h *myEventHandler) OnStateChange(event walship.StateChangeEvent) {
	fmt.Printf("State changed: %s -> %s (reason: %s)\n",
		event.Previous, event.Current, event.Reason)
}

func (h *myEventHandler) OnSendSuccess(event walship.SendSuccessEvent) {
	fmt.Printf("Sent %d frames (%d bytes) in %v\n",
		event.FrameCount, event.BytesSent, event.Duration)
}

func (h *myEventHandler) OnSendError(event walship.SendErrorEvent) {
	fmt.Printf("Send error: %v (frames: %d, retryable: %v)\n",
		event.Error, event.FrameCount, event.Retryable)
}

// Example_withMockHTTPClient demonstrates dependency injection for testing.
func Example_withMockHTTPClient() {
	// Create a mock HTTP client for testing
	mockClient := &mockHTTPClient{
		responses: make(chan *http.Response, 10),
	}

	cfg := walship.Config{
		WALDir:  "/path/to/wal",
		AuthKey: "test-key",
		ChainID: "test-chain",
		NodeID:  "test-node",
	}

	// Inject mock HTTP client
	w, err := walship.New(cfg, walship.WithHTTPClient(mockClient))
	if err != nil {
		fmt.Printf("failed to create walship: %v\n", err)
		return
	}

	_ = w // Use in tests...
}

// mockHTTPClient implements walship.HTTPClient for testing.
type mockHTTPClient struct {
	responses chan *http.Response
	requests  []*http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	select {
	case resp := <-m.responses:
		return resp, nil
	default:
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
		}, nil
	}
}

// Example_withCustomLogger demonstrates injecting a custom logger.
func Example_withCustomLogger() {
	logger := &customLogger{}

	cfg := walship.Config{
		WALDir:  "/path/to/wal",
		AuthKey: "api-key",
		ChainID: "chain-id",
		NodeID:  "node-id",
	}

	// Inject custom logger
	w, err := walship.New(cfg, walship.WithLogger(logger))
	if err != nil {
		fmt.Printf("failed to create walship: %v\n", err)
		return
	}

	_ = w // Use walship instance...
}

// customLogger implements walship.Logger.
type customLogger struct{}

func (l *customLogger) Debug(msg string, fields ...walship.LogField) {
	fmt.Printf("[DEBUG] %s\n", msg)
}

func (l *customLogger) Info(msg string, fields ...walship.LogField) {
	fmt.Printf("[INFO] %s\n", msg)
}

func (l *customLogger) Warn(msg string, fields ...walship.LogField) {
	fmt.Printf("[WARN] %s\n", msg)
}

func (l *customLogger) Error(msg string, fields ...walship.LogField) {
	fmt.Printf("[ERROR] %s\n", msg)
}

// Example_withPlugins demonstrates using optional plugins and cleanup config.
func Example_withPlugins() {
	cfg := walship.Config{
		WALDir:  "/path/to/wal",
		AuthKey: "api-key",
		ChainID: "chain-id",
		NodeID:  "node-id",
	}

	// Import plugins from:
	//   "github.com/bft-labs/walship/plugins/resourcegating"
	//   "github.com/bft-labs/walship/plugins/configwatcher"
	//
	// Then create with plugins and cleanup config:
	//
	//   w, err := walship.New(cfg,
	//       resourcegating.WithResourceGating(resourcegating.DefaultConfig()),
	//       configwatcher.WithConfigWatcher(configwatcher.DefaultConfig()),
	//       walship.WithCleanupConfig(walship.DefaultCleanupConfig()),
	//   )
	//
	// Plugins are initialized on Start() and shutdown on Stop().
	// Cleanup is config-based and runs automatically when enabled.

	w, err := walship.New(cfg)
	if err != nil {
		fmt.Printf("failed to create walship: %v\n", err)
		return
	}

	_ = w // Use walship instance...
}

// Example_moduleVersions demonstrates version checking.
func Example_moduleVersions() {
	// Check walship version
	fmt.Printf("Walship version: %s\n", walship.Version)

	// Get all module versions
	versions := walship.ModuleVersions()
	for module, version := range versions {
		fmt.Printf("%s: %s\n", module, version)
	}
}

// ExampleWalship_Status demonstrates controlling walship lifecycle.
func ExampleWalship_Status() {
	cfg := walship.Config{
		WALDir:  "/path/to/wal",
		AuthKey: "api-key",
		ChainID: "chain-id",
		NodeID:  "node-id",
	}

	w, _ := walship.New(cfg)

	// Initial state is Stopped
	fmt.Printf("Initial state is Stopped: %v\n", w.Status() == walship.StateStopped)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start streaming
	_ = w.Start(ctx)

	// After Start, state is either Starting or Running
	status := w.Status()
	validStartState := status == walship.StateStarting || status == walship.StateRunning
	fmt.Printf("After Start is Starting/Running: %v\n", validStartState)

	// Stop explicitly
	_ = w.Stop()
	time.Sleep(50 * time.Millisecond) // Brief wait for state transition

	// Output:
	// Initial state is Stopped: true
	// After Start is Starting/Running: true
}

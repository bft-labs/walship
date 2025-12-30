package walship

import (
	"net/http"

	"github.com/bft-labs/walship/internal/ports"
	"github.com/bft-labs/walship/pkg/log"
	"github.com/bft-labs/walship/pkg/sender"
)

// HTTPClient is the interface for making HTTP requests.
// *http.Client satisfies this interface.
// Deprecated: Use github.com/bft-labs/walship/pkg/sender.HTTPClient instead.
type HTTPClient = ports.HTTPClient

// Logger is the interface for structured logging.
// Deprecated: Use github.com/bft-labs/walship/pkg/log.Logger instead.
type Logger = ports.Logger

// LogField represents a structured log field.
// Deprecated: Use github.com/bft-labs/walship/pkg/log.Field instead.
type LogField = ports.Field

// Re-export types from sub-packages for convenient access.
// Users can also import sub-packages directly for selective import.
type (
	// ModularLogger is the Logger interface from pkg/log.
	ModularLogger = log.Logger

	// ModularField is the Field type from pkg/log.
	ModularField = log.Field

	// ModularHTTPClient is the HTTPClient interface from pkg/sender.
	ModularHTTPClient = sender.HTTPClient

	// ModularMetadata is the Metadata type from pkg/sender.
	ModularMetadata = sender.Metadata
)

// Option configures optional behavior of Walship.
type Option func(*options)

// options holds the optional configuration for a Walship instance.
type options struct {
	httpClient            ports.HTTPClient
	logger                ports.Logger
	eventHandler          EventHandler
	plugins               []Plugin
	cleanupConfig         *CleanupConfig
	resourceGatingConfig  *ResourceGatingConfig
}

// defaultOptions returns options with sensible defaults.
func defaultOptions(client *http.Client) options {
	return options{
		httpClient:   client,
		logger:       &noopLogger{},
		eventHandler: nil,
		plugins:      nil,
	}
}

// WithHTTPClient sets a custom HTTP client for API communication.
// If not provided, a default client with the configured timeout is used.
func WithHTTPClient(client HTTPClient) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

// WithLogger sets a custom logger for structured logging.
// If not provided, a no-op logger is used (no output).
func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithEventHandler sets a handler for walship events.
// Events are called synchronously from the streaming goroutine.
// If not provided, no events are emitted.
func WithEventHandler(handler EventHandler) Option {
	return func(o *options) {
		o.eventHandler = handler
	}
}

// WithPlugin registers a plugin to be initialized when Walship starts.
// Plugins are initialized in registration order and shutdown in reverse order.
// Use this for custom plugins. For built-in plugins, use specific options
// like WithResourceGating(), WithWALCleanup(), or WithConfigWatcher().
func WithPlugin(plugin Plugin) Option {
	return func(o *options) {
		o.plugins = append(o.plugins, plugin)
	}
}

// noopLogger discards all log messages.
type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...ports.Field) {}
func (noopLogger) Info(msg string, fields ...ports.Field)  {}
func (noopLogger) Warn(msg string, fields ...ports.Field)  {}
func (noopLogger) Error(msg string, fields ...ports.Field) {}

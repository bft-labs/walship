package walship

import "context"

// Plugin defines the interface for optional feature plugins.
// Plugins are initialized when Walship starts and shutdown when it stops.
// Use functional options like WithResourceGating() or WithWALCleanup()
// to register plugins when creating a Walship instance.
type Plugin interface {
	// Name returns the plugin identifier for logging and debugging.
	Name() string

	// Initialize is called when Walship.Start() is invoked.
	// The context is the same context passed to Start() and will be
	// canceled when Stop() is called.
	// Returns an error if initialization fails; this will prevent Walship from starting.
	Initialize(ctx context.Context, cfg PluginConfig) error

	// Shutdown is called when Walship.Stop() is invoked.
	// Plugins should release resources and stop background goroutines.
	// The context may already be canceled at this point.
	Shutdown(ctx context.Context) error
}

// PluginConfig provides configuration and dependencies to plugins during initialization.
type PluginConfig struct {
	// WALDir is the directory containing WAL files.
	WALDir string

	// StateDir is the directory for state persistence.
	StateDir string

	// ServiceURL is the base URL of the ingestion service.
	ServiceURL string

	// ChainID is the blockchain chain identifier.
	ChainID string

	// NodeID is the node identifier.
	NodeID string

	// AuthKey is the API authentication key.
	AuthKey string

	// NodeHome is the node's home directory (for config files).
	NodeHome string

	// Logger provides logging capabilities to plugins.
	Logger Logger
}

// PluginHook defines lifecycle hooks that plugins can implement
// to intercept operations at specific points.
type PluginHook interface {
	// BeforeSend is called before each batch send operation.
	// Return false to skip the send (e.g., for resource gating).
	// Return an error to abort with an error.
	BeforeSend(ctx context.Context) (proceed bool, err error)

	// AfterSend is called after each successful batch send.
	AfterSend(ctx context.Context, frameCount int, bytesSent int) error
}

// BasePlugin provides a default implementation of Plugin that does nothing.
// Embed this in custom plugins to only implement methods you need.
type BasePlugin struct {
	name string
}

// NewBasePlugin creates a new base plugin with the given name.
func NewBasePlugin(name string) BasePlugin {
	return BasePlugin{name: name}
}

// Name returns the plugin name.
func (p BasePlugin) Name() string { return p.name }

// Initialize does nothing by default.
func (p BasePlugin) Initialize(ctx context.Context, cfg PluginConfig) error { return nil }

// Shutdown does nothing by default.
func (p BasePlugin) Shutdown(ctx context.Context) error { return nil }

// Ensure BasePlugin implements Plugin.
var _ Plugin = BasePlugin{}

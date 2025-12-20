// Package walship provides an embeddable WAL streaming agent for Cosmos nodes.
//
// Walship streams Write-Ahead Log (WAL) data to apphash.io for consensus
// monitoring and diff detection. It can be used as a standalone CLI application
// or embedded as a library in other Go programs.
//
// # Basic Usage
//
// To embed walship in your application:
//
//	cfg := walship.Config{
//	    WALDir:   "/path/to/wal/directory",
//	    AuthKey:  "your-api-key",
//	    ChainID:  "my-chain",
//	    NodeID:   "my-node",
//	}
//
//	agent, err := walship.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	if err := agent.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// ... run until shutdown signal ...
//
//	if err := agent.Stop(); err != nil {
//	    log.Printf("shutdown error: %v", err)
//	}
//
// # Configuration
//
// Create a [Config] with at minimum WALDir (or NodeHome to auto-derive WALDir).
// All other fields have sensible defaults set via [Config.SetDefaults].
//
// # Event Handling
//
// To receive notifications about walship operations, implement [EventHandler]
// and pass it via [WithEventHandler]:
//
//	handler := &myEventHandler{}
//	agent, err := walship.New(cfg, walship.WithEventHandler(handler))
//
// Events are called synchronously from the streaming goroutine. Implementations
// should return quickly to avoid blocking streaming.
//
// # Dependency Injection
//
// For testing, you can inject custom implementations of external dependencies:
//
//	agent, err := walship.New(cfg,
//	    walship.WithHTTPClient(mockClient),
//	    walship.WithLogger(customLogger),
//	)
//
// # Lifecycle States
//
// A Walship instance can be in one of five states: [StateStopped], [StateStarting],
// [StateRunning], [StateStopping], or [StateCrashed]. Use [Walship.Status] to
// query the current state.
//
// # Plugins and Cleanup
//
// Walship supports optional plugins for extended functionality:
//
//	import "github.com/bft-labs/walship/plugins/resourcegating"
//	import "github.com/bft-labs/walship/plugins/configwatcher"
//
//	agent, err := walship.New(cfg,
//	    resourcegating.WithResourceGating(resourcegating.DefaultConfig()),
//	    configwatcher.WithConfigWatcher(configwatcher.DefaultConfig()),
//	    walship.WithCleanupConfig(walship.DefaultCleanupConfig()),
//	)
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// Use [ModuleVersions] to get versions of all sub-modules and [CompatibilityMatrix]
// to check minimum compatible versions. See version.go for details.
package walship

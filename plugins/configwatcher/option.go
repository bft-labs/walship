package configwatcher

import "github.com/bft-labs/walship/pkg/walship"

// WithConfigWatcher returns a walship Option that enables config file watching.
// When enabled, the plugin monitors app.toml and config.toml for changes and
// sends updates to the service.
//
// Usage:
//
//	w, err := walship.New(cfg,
//	    configwatcher.WithConfigWatcher(configwatcher.Config{
//	        RetryInterval: 5 * time.Second,
//	        DebounceDelay: 100 * time.Millisecond,
//	    }),
//	)
func WithConfigWatcher(cfg Config) walship.Option {
	plugin := New(cfg)
	return walship.WithPlugin(plugin)
}

// WithDefaultConfigWatcher returns a walship Option that enables config
// watching with default settings (retry every 5s, debounce 100ms).
//
// Usage:
//
//	w, err := walship.New(cfg, configwatcher.WithDefaultConfigWatcher())
func WithDefaultConfigWatcher() walship.Option {
	return WithConfigWatcher(DefaultConfig())
}

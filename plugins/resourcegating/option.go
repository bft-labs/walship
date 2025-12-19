package resourcegating

import "github.com/bft-labs/walship/pkg/walship"

// WithResourceGating returns a walship Option that enables resource gating.
// When enabled, the plugin monitors CPU and network utilization and can
// delay batch sends when the system is under heavy load.
//
// Usage:
//
//	w, err := walship.New(cfg,
//	    resourcegating.WithResourceGating(resourcegating.Config{
//	        CPUThreshold: 0.85,
//	        NetThreshold: 0.70,
//	    }),
//	)
func WithResourceGating(cfg Config) walship.Option {
	plugin := New(cfg)
	return walship.WithPlugin(plugin)
}

// WithDefaultResourceGating returns a walship Option that enables resource
// gating with default settings (CPU threshold 0.85, network threshold 0.70).
//
// Usage:
//
//	w, err := walship.New(cfg, resourcegating.WithDefaultResourceGating())
func WithDefaultResourceGating() walship.Option {
	return WithResourceGating(DefaultConfig())
}

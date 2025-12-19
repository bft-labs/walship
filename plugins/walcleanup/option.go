package walcleanup

import "github.com/bft-labs/walship/pkg/walship"

// WithWALCleanup returns a walship Option that enables automatic WAL cleanup.
// When enabled, the plugin periodically checks the WAL directory size and
// removes old segments when it exceeds the configured high watermark.
//
// Usage:
//
//	w, err := walship.New(cfg,
//	    walcleanup.WithWALCleanup(walcleanup.Config{
//	        CheckInterval:  72 * time.Hour,
//	        HighWatermark:  2 << 30,  // 2 GiB
//	        LowWatermark:   3 << 29,  // 1.5 GiB
//	    }),
//	)
func WithWALCleanup(cfg Config) walship.Option {
	plugin := New(cfg)
	return walship.WithPlugin(plugin)
}

// WithDefaultWALCleanup returns a walship Option that enables WAL cleanup
// with default settings (check every 72h, high watermark 2GiB, low watermark 1.5GiB).
//
// Usage:
//
//	w, err := walship.New(cfg, walcleanup.WithDefaultWALCleanup())
func WithDefaultWALCleanup() walship.Option {
	return WithWALCleanup(DefaultConfig())
}

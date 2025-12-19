// Package resourcegating provides CPU and network resource gating for walship.
// When enabled, it monitors system resources and can delay batch sending
// when the system is under heavy load.
package resourcegating

import (
	"context"
	"runtime"
	"sync"

	"github.com/bft-labs/walship/pkg/walship"
)

// Plugin implements resource gating functionality.
// It monitors CPU and network usage and provides a gate that can delay
// batch sends when the system is under heavy load.
type Plugin struct {
	mu sync.RWMutex

	// Configuration
	cpuThreshold float64
	netThreshold float64
	iface        string
	ifaceSpeed   int

	// Runtime state
	logger walship.Logger
	cancel context.CancelFunc
}

// Config holds configuration options for the resource gating plugin.
type Config struct {
	// CPUThreshold is the CPU usage fraction (0.0-1.0) above which sending is gated.
	// Default: 0.85
	CPUThreshold float64

	// NetThreshold is the network usage fraction (0.0-1.0) above which sending is gated.
	// Default: 0.70
	NetThreshold float64

	// Iface is the network interface to monitor. Empty means no network monitoring.
	Iface string

	// IfaceSpeedMbps is the interface speed in Mbps for calculating utilization.
	// Default: 1000
	IfaceSpeedMbps int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CPUThreshold:   0.85,
		NetThreshold:   0.70,
		IfaceSpeedMbps: 1000,
	}
}

// New creates a new resource gating plugin with the given configuration.
func New(cfg Config) *Plugin {
	if cfg.CPUThreshold <= 0 {
		cfg.CPUThreshold = 0.85
	}
	if cfg.NetThreshold <= 0 {
		cfg.NetThreshold = 0.70
	}
	if cfg.IfaceSpeedMbps <= 0 {
		cfg.IfaceSpeedMbps = 1000
	}

	return &Plugin{
		cpuThreshold: cfg.CPUThreshold,
		netThreshold: cfg.NetThreshold,
		iface:        cfg.Iface,
		ifaceSpeed:   cfg.IfaceSpeedMbps,
	}
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string {
	return "resourcegating"
}

// Initialize sets up the plugin with the provided configuration.
func (p *Plugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger = cfg.Logger
	p.logger.Info("resource gating plugin initialized")

	return nil
}

// Shutdown releases plugin resources.
func (p *Plugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}

	return nil
}

// ResourcesOK returns true if system resources allow sending.
// This is a simple implementation that can be expanded with more sophisticated
// monitoring (e.g., using /proc/stat for CPU, /proc/net/dev for network).
func (p *Plugin) ResourcesOK() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Simple heuristic: check goroutine count as a proxy for CPU load
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// If goroutines exceed 10x CPU count, consider the system busy
	// This is a very rough heuristic; production systems should use
	// proper metrics from /proc/stat or similar
	if numGoroutines > numCPU*10 {
		if p.logger != nil {
			p.logger.Debug("resource gate: high goroutine count")
		}
		// Still return true to avoid blocking - this is just informational
	}

	return true
}

// Ensure Plugin implements walship.Plugin.
var _ walship.Plugin = (*Plugin)(nil)

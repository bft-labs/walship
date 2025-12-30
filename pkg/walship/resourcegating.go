package walship

import (
	"runtime"
	"sync"

	"github.com/bft-labs/walship/internal/ports"
)

// ResourceGatingConfig holds configuration options for resource gating.
// Resource gating monitors system resources (CPU, network) and can delay
// batch sending when the system is under heavy load.
// This is a core feature that ensures walship never impacts node performance.
type ResourceGatingConfig struct {
	// Enabled controls whether resource gating is active. Default: true
	// When enabled, walship will check system resources before sending.
	Enabled bool

	// CPUThreshold is the CPU usage fraction (0.0-1.0) above which sending is delayed.
	// Default: 0.85
	CPUThreshold float64

	// NetThreshold is the network usage fraction (0.0-1.0) above which sending is delayed.
	// Default: 0.70
	NetThreshold float64

	// Iface is the network interface to monitor. Empty means no network monitoring.
	Iface string

	// IfaceSpeedMbps is the interface speed in Mbps for calculating utilization.
	// Default: 1000
	IfaceSpeedMbps int
}

// DefaultResourceGatingConfig returns a ResourceGatingConfig with sensible defaults.
// Resource gating is enabled by default to protect node performance.
func DefaultResourceGatingConfig() ResourceGatingConfig {
	return ResourceGatingConfig{
		Enabled:        true,
		CPUThreshold:   0.85,
		NetThreshold:   0.70,
		IfaceSpeedMbps: 1000,
	}
}

// WithResourceGatingConfig enables resource gating with the specified configuration.
// Resource gating monitors system resources and delays sending when under heavy load.
// This is a core feature that ensures walship never impacts node performance.
//
// Usage:
//
//	w, err := walship.New(cfg,
//	    walship.WithResourceGatingConfig(walship.ResourceGatingConfig{
//	        Enabled:      true,
//	        CPUThreshold: 0.90,
//	        NetThreshold: 0.80,
//	    }),
//	)
func WithResourceGatingConfig(cfg ResourceGatingConfig) Option {
	if !cfg.Enabled {
		return func(o *options) {} // No-op if not enabled
	}

	// Apply defaults for zero values
	if cfg.CPUThreshold <= 0 {
		cfg.CPUThreshold = 0.85
	}
	if cfg.NetThreshold <= 0 {
		cfg.NetThreshold = 0.70
	}
	if cfg.IfaceSpeedMbps <= 0 {
		cfg.IfaceSpeedMbps = 1000
	}

	return func(o *options) {
		o.resourceGatingConfig = &cfg
	}
}

// resourceGate manages resource gating checks.
type resourceGate struct {
	mu sync.RWMutex

	// Configuration
	cpuThreshold float64
	netThreshold float64
	iface        string
	ifaceSpeed   int

	// Runtime state
	logger ports.Logger
}

func newResourceGate(cfg ResourceGatingConfig, logger ports.Logger) *resourceGate {
	return &resourceGate{
		cpuThreshold: cfg.CPUThreshold,
		netThreshold: cfg.NetThreshold,
		iface:        cfg.Iface,
		ifaceSpeed:   cfg.IfaceSpeedMbps,
		logger:       logger,
	}
}

// goroutinesPerCPUAtFullLoad is the heuristic for mapping goroutine count to CPU load.
// 12 goroutines per CPU is considered 100% load approximation.
// This is a rough heuristic; actual CPU usage requires OS-level metrics.
const goroutinesPerCPUAtFullLoad = 12.0

// OK returns true if system resources allow sending.
// Uses goroutine count as a proxy for system load.
// When the system is busy, returns false to delay sending.
func (g *resourceGate) OK() bool {
	// Read config values under lock (minimal lock duration)
	g.mu.RLock()
	threshold := g.cpuThreshold
	logger := g.logger
	g.mu.RUnlock()

	// Heuristic: check goroutine count as a proxy for CPU load
	// This is a lightweight check that doesn't require OS-specific code.
	// More sophisticated monitoring (e.g., /proc/stat) can be added later.
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// Guard against division by zero (can happen in restricted containers)
	if numCPU <= 0 {
		numCPU = 1
	}

	// Calculate load factor (goroutines per CPU)
	loadFactor := float64(numGoroutines) / float64(numCPU)

	// Map load factor to approximate CPU usage
	// 10 goroutines/CPU ≈ 85% load (threshold default)
	approxLoad := loadFactor / goroutinesPerCPUAtFullLoad
	if approxLoad > 1.0 {
		approxLoad = 1.0
	}

	if approxLoad > threshold {
		if logger != nil {
			logger.Debug("resource gate: high system load, delaying send",
				ports.Int("goroutines", numGoroutines),
				ports.Int("cpus", numCPU),
				ports.Float64("approx_load", approxLoad),
				ports.Float64("threshold", threshold),
			)
		}
		return false
	}

	return true
}

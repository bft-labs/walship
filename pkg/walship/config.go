package walship

import (
	"fmt"
	"time"

	"github.com/bft-labs/walship/internal/domain"
)

// DefaultServiceURL is the default endpoint for shipping WAL data.
const DefaultServiceURL = "https://api.apphash.io"

// Config contains all parameters for walship operation.
// Required fields must be set; optional fields have sensible defaults.
type Config struct {
	// NodeHome is the application home directory (required).
	NodeHome string

	// WALDir is the WAL directory. If empty, derived from NodeHome.
	WALDir string

	// AuthKey is the API authentication key (required).
	AuthKey string

	// ServiceURL is the API endpoint. Defaults to https://api.apphash.io
	ServiceURL string

	// ChainID is the blockchain chain ID. Auto-detected from genesis.json if empty.
	ChainID string

	// NodeID is the node identifier. Auto-detected from node_key.json if empty.
	NodeID string

	// PollInterval is the poll interval when idle. Defaults to 500ms.
	PollInterval time.Duration

	// SendInterval is the soft send interval. Defaults to 5s.
	SendInterval time.Duration

	// HardInterval is the hard send interval that overrides gating. Defaults to 10s.
	HardInterval time.Duration

	// HTTPTimeout is the HTTP request timeout. Defaults to 15s.
	HTTPTimeout time.Duration

	// MaxBatchBytes is the max compressed bytes per batch. Defaults to 4MB.
	MaxBatchBytes int

	// StateDir is the state file directory. Defaults to WALDir.
	StateDir string

	// CPUThreshold is the max CPU usage fraction before delaying sends. Defaults to 0.85.
	CPUThreshold float64

	// NetThreshold is the max network usage fraction before delaying sends. Defaults to 0.70.
	NetThreshold float64

	// Iface is the network interface to monitor (optional).
	Iface string

	// IfaceSpeedMbps is the interface speed in Mbps. Defaults to 1000.
	IfaceSpeedMbps int

	// Verify enables CRC/line count verification while reading (debug).
	Verify bool

	// Meta enables printing frame metadata to stderr (debug).
	Meta bool

	// Once processes available frames and exits.
	Once bool
}

// DefaultConfig returns a Config with default values set.
// NodeHome and AuthKey must still be provided.
func DefaultConfig() Config {
	return Config{
		ServiceURL:     DefaultServiceURL,
		PollInterval:   500 * time.Millisecond,
		SendInterval:   5 * time.Second,
		HardInterval:   10 * time.Second,
		HTTPTimeout:    15 * time.Second,
		MaxBatchBytes:  4 << 20, // 4MB
		CPUThreshold:   0.85,
		NetThreshold:   0.70,
		IfaceSpeedMbps: 1000,
	}
}

// Validate checks the configuration and returns an error if invalid.
// This is called automatically by New() but can be called explicitly.
func (c *Config) Validate() error {
	// WALDir is required (can be set directly or derived from NodeHome)
	if c.WALDir == "" && c.NodeHome == "" {
		return fmt.Errorf("%w: wal-dir or node-home is required", domain.ErrInvalidConfig)
	}

	// Derive WALDir if not set
	if c.WALDir == "" {
		if c.NodeID != "" {
			c.WALDir = fmt.Sprintf("%s/data/log.wal/node-%s", c.NodeHome, c.NodeID)
		} else {
			c.WALDir = fmt.Sprintf("%s/data/log.wal/node-default", c.NodeHome)
		}
	}

	// Derive StateDir if not set
	if c.StateDir == "" && c.WALDir != "" {
		c.StateDir = c.WALDir
	}

	// Set default ServiceURL
	if c.ServiceURL == "" {
		c.ServiceURL = DefaultServiceURL
	}

	// Strip trailing slash from ServiceURL
	if len(c.ServiceURL) > 0 && c.ServiceURL[len(c.ServiceURL)-1] == '/' {
		c.ServiceURL = c.ServiceURL[:len(c.ServiceURL)-1]
	}

	// Validate intervals
	if c.PollInterval <= 0 {
		return fmt.Errorf("%w: poll interval must be positive", domain.ErrInvalidConfig)
	}
	if c.SendInterval <= 0 {
		return fmt.Errorf("%w: send interval must be positive", domain.ErrInvalidConfig)
	}

	return nil
}

// SetDefaults sets default values for zero-valued fields.
// Called automatically during Validate().
func (c *Config) SetDefaults() {
	if c.ServiceURL == "" {
		c.ServiceURL = DefaultServiceURL
	}
	if c.PollInterval == 0 {
		c.PollInterval = 500 * time.Millisecond
	}
	if c.SendInterval == 0 {
		c.SendInterval = 5 * time.Second
	}
	if c.HardInterval == 0 {
		c.HardInterval = 10 * time.Second
	}
	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = 15 * time.Second
	}
	if c.MaxBatchBytes == 0 {
		c.MaxBatchBytes = 4 << 20 // 4MB
	}
	if c.CPUThreshold == 0 {
		c.CPUThreshold = 0.85
	}
	if c.NetThreshold == 0 {
		c.NetThreshold = 0.70
	}
	if c.IfaceSpeedMbps == 0 {
		c.IfaceSpeedMbps = 1000
	}
}

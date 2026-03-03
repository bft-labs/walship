// Package walship provides a lightweight agent for streaming Cosmos node WAL data.
//
// Example usage:
//
//	cfg := walship.DefaultConfig()
//	cfg.NodeHome = "/path/to/node"
//	cfg.AuthKey = "your-api-key"
//	cfg.ChainID = "mychain-1"
//	cfg.ChainBinaryPath = "/usr/local/bin/mychaind"
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//	if err := walship.LoadNodeInfo(&cfg); err != nil {
//	    log.Fatal(err)
//	}
//	if err := walship.Run(context.Background(), cfg); err != nil {
//	    log.Fatal(err)
//	}
package walship

import (
	"context"

	"github.com/bft-labs/walship/internal/agent"
	"github.com/rs/zerolog"
)

// Config holds the configuration for the WAL shipping agent.
// Use DefaultConfig() to get a Config with sensible defaults.
type Config = agent.Config

// FrameMeta contains metadata about a single WAL frame.
// Fields are used to locate and read gzip members from the .gz file.
type FrameMeta = agent.FrameMeta

// Run starts the WAL shipping agent with the given configuration.
// It blocks until the context is cancelled or an unrecoverable error occurs.
// Use cfg.Once = true to process available frames and exit immediately.
func Run(ctx context.Context, cfg Config) error {
	return agent.Run(ctx, cfg)
}

// DefaultConfig returns a Config with sensible default values.
// At minimum, you must set NodeHome and AuthKey before calling Run.
func DefaultConfig() Config {
	return agent.DefaultConfig()
}

// LoadNodeInfo resolves ChainID and NodeID from explicit config values or by
// querying the chain binary. It never reads private key material from disk.
// This should be called after setting cfg.ChainID (required) and optionally
// cfg.ChainBinaryPath, then before Run.
func LoadNodeInfo(cfg *Config) error {
	return agent.LoadNodeInfo(cfg)
}

// Logger returns the package-level zerolog logger used by the agent.
func Logger() zerolog.Logger {
	return agent.Logger()
}

// DefaultServiceURL is the default endpoint for shipping WAL data.
const DefaultServiceURL = agent.DefaultServiceURL

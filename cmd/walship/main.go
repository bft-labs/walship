package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"

	agent "github.com/bft-labs/cosmos-analyzer-shipper/internal/agent"
)

func main() {
	cfg := agent.DefaultConfig()
	var cfgPath string

	root := &cobra.Command{
		Use:   "walship",
		Short: "Ship MemLogger/WAL gzip frames using index metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config file first (default $HOME/.walship/config.toml), then apply flag overrides
			// Determine config path
			cfgFile := cfgPath
			if cfgFile == "" {
				cfgFile = agent.DefaultConfigPath()
			}

			// Build set of changed flags
			changed := map[string]bool{}
			cmd.Flags().Visit(func(f *pflag.Flag) { changed[f.Name] = true })

			if cfgFile != "" && agent.FileExists(cfgFile) {
				fc, err := agent.LoadFileConfig(cfgFile)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if err := agent.ApplyFileConfig(&cfg, fc, changed); err != nil {
					return err
				}
			}

			// Apply environment variables (WALSHIP_*)
			// These override file config but are overridden by flags (checked via changed map)
			agent.ApplyEnvConfig(&cfg, changed)

			// Load node info (ChainID, NodeID) from files if needed
			if err := agent.LoadNodeInfo(&cfg); err != nil {
				return err
			}

			// Validate and set derived defaults
			if err := cfg.Validate(); err != nil {
				return err
			}

			// Log configuration (masking API key)
			logCfg := cfg
			if len(logCfg.AuthKey) > 0 {
				logCfg.AuthKey = "*****"
			}
			fmt.Fprintf(os.Stderr, "Configuration: %+v\n", logCfg)

			if err := agent.Run(context.Background(), cfg); err != nil {
				return err
			}
			return nil
		},
	}

	// Flags
	root.Flags().StringVar(&cfgPath, "config", "", "path to config file (default: $HOME/.walship/config.toml)")
	root.Flags().StringVar(&cfg.NodeHome, "node-home", "", "application home directory")
	root.Flags().StringVar(&cfg.ChainID, "chain-id", cfg.ChainID, "chain id (override genesis.json)")
	root.Flags().StringVar(&cfg.NodeID, "node-id", cfg.NodeID, "node id (directory suffix)")
	root.Flags().StringVar(&cfg.WALDir, "wal-dir", cfg.WALDir, "WAL directory containing .idx/.gz pairs")

	root.Flags().StringVar(&cfg.ServiceURL, "service-url", cfg.ServiceURL, "webhook URL (e.g., https://api.apphash.io/v1/ingest)")
	root.Flags().StringVar(&cfg.AuthKey, "auth-key", cfg.AuthKey, "API key for authentication")

	root.Flags().DurationVar(&cfg.PollInterval, "poll", cfg.PollInterval, "poll interval when idle")
	root.Flags().DurationVar(&cfg.SendInterval, "send-interval", cfg.SendInterval, "soft send interval")
	root.Flags().DurationVar(&cfg.HardInterval, "hard-interval", cfg.HardInterval, "hard send interval (override gating)")
	root.Flags().IntVar(&cfg.MaxBatchBytes, "max-batch-bytes", cfg.MaxBatchBytes, "maximum compressed bytes per batch")

	root.Flags().Float64Var(&cfg.CPUThreshold, "cpu-threshold", cfg.CPUThreshold, "max CPU usage fraction before delaying send")
	root.Flags().Float64Var(&cfg.NetThreshold, "net-threshold", cfg.NetThreshold, "max network usage fraction before delaying send")
	root.Flags().StringVar(&cfg.Iface, "iface", cfg.Iface, "network interface to monitor (optional)")
	root.Flags().IntVar(&cfg.IfaceSpeedMbps, "iface-speed", cfg.IfaceSpeedMbps, "interface speed in Mbps (used for utilization)")

	root.Flags().StringVar(&cfg.StateDir, "state-dir", cfg.StateDir, "state directory for agent-status.json")
	root.Flags().DurationVar(&cfg.HTTPTimeout, "timeout", cfg.HTTPTimeout, "HTTP timeout")
	root.Flags().BoolVar(&cfg.Verify, "verify", cfg.Verify, "verify CRC/line counts while reading (debug)")
	root.Flags().BoolVar(&cfg.Meta, "meta", cfg.Meta, "print frame metadata to stderr (debug)")
	root.Flags().BoolVar(&cfg.Once, "once", cfg.Once, "process available frames and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "walship: %v\n", err)
		os.Exit(1)
	}
}

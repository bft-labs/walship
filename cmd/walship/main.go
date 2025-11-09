package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"time"

	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"

	agent "github.com/bft-labs/cometbft-analyzer-shipper/internal/agent"
)

func main() {
	var cfg agent.Config
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
			if cfgFile != "" && agent.FileExists(cfgFile) {
				fc, err := agent.LoadFileConfig(cfgFile)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				// Build set of changed flags
				changed := map[string]bool{}
				cmd.Flags().Visit(func(f *pflag.Flag) { changed[f.Name] = true })
				if err := agent.ApplyFileConfig(&cfg, fc, changed); err != nil {
					return err
				}
			}
			if cfg.WALDir == "" {
				if cfg.Root != "" && cfg.NodeID != "" {
					// fallback derived layout
					cfg.WALDir = fmt.Sprintf("%s/data/log.wal/node-%s", cfg.Root, cfg.NodeID)
				} else if cfg.Root != "" {
					cfg.WALDir = fmt.Sprintf("%s/data/log.wal", cfg.Root)
				}
			}
			if cfg.RemoteURL == "" && cfg.RemoteBase != "" && cfg.Network != "" {
				node := cfg.RemoteNode
				if node == "" {
					node = cfg.NodeID
				}
				base := cfg.RemoteBase
				// ensure no trailing slash
				if len(base) > 0 && base[len(base)-1] == '/' {
					base = base[:len(base)-1]
				}
				cfg.RemoteURL = base + pathpkg.Join("", "/v1/ingest/", url.PathEscape(cfg.Network), "/", url.PathEscape(node), "/wal-frames")
			}
			if cfg.StateDir == "" {
				if home, err := os.UserHomeDir(); err == nil {
					cfg.StateDir = home + "/.cometbft-analyzer"
				} else {
					cfg.StateDir = "."
				}
			}
			if err := agent.Run(context.Background(), cfg); err != nil {
				return err
			}
			return nil
		},
	}

	// Flags
	root.Flags().StringVar(&cfgPath, "config", "", "path to config file (default: $HOME/.memagent/config.toml)")
	root.Flags().StringVar(&cfg.Root, "root", "", "application root (contains data/) [fallback for WAL dir]")
	root.Flags().StringVar(&cfg.NodeID, "node", "default", "node id (directory suffix)")
	root.Flags().StringVar(&cfg.WALDir, "wal-dir", "", "WAL directory containing .idx/.gz pairs")

	root.Flags().StringVar(&cfg.RemoteURL, "remote-url", "", "remote HTTP(S) endpoint to POST frames (overrides base/network/node)")
	root.Flags().StringVar(&cfg.RemoteBase, "remote-base", "", "remote base URL (e.g., http://host:8080)")
	root.Flags().StringVar(&cfg.Network, "network", "", "network identifier for ingest route")
	root.Flags().StringVar(&cfg.RemoteNode, "remote-node", "", "node identifier for ingest route (defaults to --node)")
	root.Flags().StringVar(&cfg.AuthKey, "auth-key", os.Getenv("MEMAGENT_AUTH_KEY"), "authorization key (or MEMAGENT_AUTH_KEY)")

	root.Flags().DurationVar(&cfg.PollInterval, "poll", 500*time.Millisecond, "poll interval when idle")
	root.Flags().DurationVar(&cfg.SendInterval, "send-interval", 5*time.Second, "soft send interval")
	root.Flags().DurationVar(&cfg.HardInterval, "hard-interval", 10*time.Second, "hard send interval (override gating)")
	root.Flags().IntVar(&cfg.MaxBatchBytes, "max-batch-bytes", 4<<20, "maximum compressed bytes per batch")

	root.Flags().Float64Var(&cfg.CPUThreshold, "cpu-threshold", 0.85, "max CPU usage fraction before delaying send")
	root.Flags().Float64Var(&cfg.NetThreshold, "net-threshold", 0.70, "max network usage fraction before delaying send")
	root.Flags().StringVar(&cfg.Iface, "iface", "", "network interface to monitor (optional)")
	root.Flags().IntVar(&cfg.IfaceSpeedMbps, "iface-speed", 1000, "interface speed in Mbps (used for utilization)")

	root.Flags().StringVar(&cfg.StateDir, "state-dir", "", "state directory for agent-status.json")
	root.Flags().DurationVar(&cfg.HTTPTimeout, "timeout", 15*time.Second, "HTTP timeout")
	root.Flags().BoolVar(&cfg.Verify, "verify", false, "verify CRC/line counts while reading (debug)")
	root.Flags().BoolVar(&cfg.Meta, "meta", false, "print frame metadata to stderr (debug)")
	root.Flags().BoolVar(&cfg.Once, "once", false, "process available frames and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "memagent: %v\n", err)
		os.Exit(1)
	}
}

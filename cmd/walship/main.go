package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"

	logAdapter "github.com/bft-labs/walship/internal/adapters/log"
	"github.com/bft-labs/walship/internal/cliconfig"
	"github.com/bft-labs/walship/pkg/walship"
	"github.com/bft-labs/walship/plugins/configwatcher"
)

const helpBanner = `
 █████   ███   █████   █████████   █████        █████████  █████   █████ █████ ███████████ 
░░███   ░███  ░░███   ███░░░░░███ ░░███        ███░░░░░███░░███   ░░███ ░░███ ░░███░░░░░███
 ░███   ░███   ░███  ░███    ░███  ░███       ░███    ░░░  ░███    ░███  ░███  ░███    ░███
 ░███   ░███   ░███  ░███████████  ░███       ░░█████████  ░███████████  ░███  ░██████████ 
 ░░███  █████  ███   ░███░░░░░███  ░███        ░░░░░░░░███ ░███░░░░░███  ░███  ░███░░░░░░  
  ░░░█████░█████░    ░███    ░███  ░███      █ ███    ░███ ░███    ░███  ░███  ░███        
    ░░███ ░░███      █████   █████ ███████████░░█████████  █████   █████ █████ █████       
     ░░░   ░░░      ░░░░░   ░░░░░ ░░░░░░░░░░░  ░░░░░░░░░  ░░░░░   ░░░░░ ░░░░░ ░░░░░        
`

const helpDescription = `
Stream your node's consensus feed to apphash.io without slowing your validator.

Highlights:
  - Batches and backpressures automatically so performance stays intact.
  - Discovers chain/node IDs from your node home; configure via file, env, or flags.
  - Safe defaults with tunable thresholds for CPU/network utilization.
  - Requires apphash SDK integration and an API key—read the docs or email us.

Docs: https://docs.apphash.io/getting-started
Contact: actor93kor@gmail.com
`

var longHelp = strings.TrimSpace(helpBanner) + "\n\n" + strings.TrimSpace(helpDescription)

var exampleUsage = strings.TrimSpace(`
  walship --node-home ~/.mychain --auth-key <api-key>
  walship --config $HOME/.walship/config.toml --once
`)

func getVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}

func main() {
	cfg := cliconfig.DefaultConfig()
	var cfgPath string

	log := cliconfig.Logger()

	root := &cobra.Command{
		Use:     "walship",
		Short:   "Stream your node's consensus feed to apphash.io without slowing your validator",
		Long:    longHelp,
		Example: exampleUsage,
		Version: fmt.Sprintf("%s %s/%s", getVersion(), runtime.GOOS, runtime.GOARCH),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config file first (default $HOME/.walship/config.toml), then apply flag overrides
			// Determine config path
			cfgFile := cfgPath
			if cfgFile == "" {
				cfgFile = cliconfig.DefaultConfigPath()
			}

			// Build set of changed flags
			changed := map[string]bool{}
			cmd.Flags().Visit(func(f *pflag.Flag) { changed[f.Name] = true })

			if cfgFile != "" && cliconfig.FileExists(cfgFile) {
				fc, err := cliconfig.LoadFileConfig(cfgFile)
				if err != nil {
					return fmt.Errorf("load config: %w", err)
				}
				if err := cliconfig.ApplyFileConfig(&cfg, fc, changed); err != nil {
					return err
				}
			}

			// Apply environment variables (WALSHIP_*)
			// These override file config but are overridden by flags (checked via changed map)
			cliconfig.ApplyEnvConfig(&cfg, changed)

			// Load node info (ChainID, NodeID) from files if needed
			if err := cliconfig.LoadNodeInfo(&cfg); err != nil {
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
			log.Info().Interface("config", logCfg).Msg("configuration")

			// Convert agent.Config to walship.Config
			libCfg := walship.Config{
				NodeHome:       cfg.NodeHome,
				WALDir:         cfg.WALDir,
				StateDir:       cfg.StateDir,
				ServiceURL:     cfg.ServiceURL,
				AuthKey:        cfg.AuthKey,
				ChainID:        cfg.ChainID,
				NodeID:         cfg.NodeID,
				PollInterval:   cfg.PollInterval,
				SendInterval:   cfg.SendInterval,
				HardInterval:   cfg.HardInterval,
				MaxBatchBytes:  cfg.MaxBatchBytes,
				HTTPTimeout:    cfg.HTTPTimeout,
				CPUThreshold:   cfg.CPUThreshold,
				NetThreshold:   cfg.NetThreshold,
				Iface:          cfg.Iface,
				IfaceSpeedMbps: cfg.IfaceSpeedMbps,
				Verify:         cfg.Verify,
				Meta:           cfg.Meta,
				Once:           cfg.Once,
			}

			// Create zerolog adapter for the library
			zerologAdapter := logAdapter.NewZerologAdapterWithLogger(log)

			// Create walship instance with features enabled by default
			// This maintains backward compatibility with main branch behavior
			w, err := walship.New(libCfg,
				walship.WithLogger(zerologAdapter),
				// Enable config watcher plugin
				configwatcher.WithConfigWatcher(configwatcher.DefaultConfig()),
				// Enable WAL cleanup (config-based, not a plugin)
				walship.WithCleanupConfig(walship.DefaultCleanupConfig()),
				// Enable resource gating (core feature, protects node performance)
				walship.WithResourceGatingConfig(walship.ResourceGatingConfig{
					Enabled:        true,
					CPUThreshold:   cfg.CPUThreshold,
					NetThreshold:   cfg.NetThreshold,
					Iface:          cfg.Iface,
					IfaceSpeedMbps: cfg.IfaceSpeedMbps,
				}),
			)
			if err != nil {
				return fmt.Errorf("create walship: %w", err)
			}

			// Setup signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			// Start walship
			if err := w.Start(ctx); err != nil {
				return fmt.Errorf("start walship: %w", err)
			}

			// Create done channel to detect completion
			doneCh := make(chan struct{})
			go func() {
				// Poll for completion (for once mode)
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						status := w.Status()
						if status == walship.StateStopped || status == walship.StateCrashed {
							close(doneCh)
							return
						}
					}
				}
			}()

			// Wait for signal or completion
			select {
			case <-sigCh:
				log.Info().Msg("received signal, stopping...")
			case <-doneCh:
				// Completed (once mode or crash)
				if w.Status() == walship.StateCrashed {
					log.Error().Msg("walship crashed")
				}
			}

			// Graceful shutdown
			if err := w.Stop(); err != nil {
				return fmt.Errorf("stop walship: %w", err)
			}
			return nil
		},
	}

	// Flags
	root.Flags().StringVar(&cfgPath, "config", "", "path to config file (default: $HOME/.walship/config.toml)")
	root.Flags().StringVar(&cfg.NodeHome, "node-home", "", "application home directory")
	root.Flags().StringVar(&cfg.WALDir, "wal-dir", cfg.WALDir, "WAL directory containing .idx/.gz pairs")

	root.Flags().StringVar(&cfg.ServiceURL, "service-url", cfg.ServiceURL, fmt.Sprintf("base service URL (defaults to %s; override only for internal testing)", cliconfig.DefaultServiceURL))
	if err := root.Flags().MarkHidden("service-url"); err != nil {
		log.Info().Err(err).Msg("failed to hide service-url flag")
	}
	root.Flags().StringVar(&cfg.AuthKey, "auth-key", cfg.AuthKey, "API key for authentication")

	root.Flags().DurationVar(&cfg.PollInterval, "poll", cfg.PollInterval, "poll interval when idle")
	root.Flags().DurationVar(&cfg.SendInterval, "send-interval", cfg.SendInterval, "soft send interval")
	root.Flags().DurationVar(&cfg.HardInterval, "hard-interval", cfg.HardInterval, "hard send interval (override gating)")
	root.Flags().IntVar(&cfg.MaxBatchBytes, "max-batch-bytes", cfg.MaxBatchBytes, "maximum compressed bytes per batch")

	root.Flags().Float64Var(&cfg.CPUThreshold, "cpu-threshold", cfg.CPUThreshold, "max CPU usage fraction before delaying send")
	root.Flags().Float64Var(&cfg.NetThreshold, "net-threshold", cfg.NetThreshold, "max network usage fraction before delaying send")
	root.Flags().StringVar(&cfg.Iface, "iface", cfg.Iface, "network interface to monitor (optional)")
	root.Flags().IntVar(&cfg.IfaceSpeedMbps, "iface-speed", cfg.IfaceSpeedMbps, "interface speed in Mbps (used for utilization)")

	root.Flags().StringVar(&cfg.StateDir, "state-dir", cfg.StateDir, "state directory for status.json (defaults to wal-dir)")
	if err := root.Flags().MarkHidden("state-dir"); err != nil {
		log.Info().Err(err).Msg("failed to hide state-dir flag")
	}
	root.Flags().DurationVar(&cfg.HTTPTimeout, "timeout", cfg.HTTPTimeout, "HTTP timeout")
	root.Flags().BoolVar(&cfg.Verify, "verify", cfg.Verify, "verify CRC/line counts while reading (debug)")
	root.Flags().BoolVar(&cfg.Meta, "meta", cfg.Meta, "print frame metadata to stderr (debug)")
	root.Flags().BoolVar(&cfg.Once, "once", cfg.Once, "process available frames and exit")

	if err := root.Execute(); err != nil {
		log.Error().Err(err).Msg("walship")
		os.Exit(1)
	}
}

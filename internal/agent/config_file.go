package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// fileConfig mirrors Config but uses strings for durations to make TOML friendly.
type fileConfig struct {
	Root           string  `toml:"root"`
	NodeID         string  `toml:"node"`
	WALDir         string  `toml:"wal_dir"`
	RemoteURL      string  `toml:"remote_url"`
	RemoteBase     string  `toml:"remote_base"`
	Network        string  `toml:"network"`
	RemoteNode     string  `toml:"remote_node"`
	AuthKey        string  `toml:"auth_key"`
	PollInterval   string  `toml:"poll_interval"`
	SendInterval   string  `toml:"send_interval"`
	HardInterval   string  `toml:"hard_interval"`
	HTTPTimeout    string  `toml:"http_timeout"`
	CPUThreshold   float64 `toml:"cpu_threshold"`
	NetThreshold   float64 `toml:"net_threshold"`
	Iface          string  `toml:"iface"`
	IfaceSpeedMbps int     `toml:"iface_speed_mbps"`
	MaxBatchBytes  int     `toml:"max_batch_bytes"`
	StateDir       string  `toml:"state_dir"`
	Verify         *bool   `toml:"verify"`
	Meta           *bool   `toml:"meta"`
	Once           *bool   `toml:"once"`
}

func loadFileConfig(path string) (fileConfig, error) {
	var fc fileConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return fc, err
	}
	if err := toml.Unmarshal(b, &fc); err != nil {
		return fc, err
	}
	return fc, nil
}

func defaultConfigPath() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".walship", "config.toml")
	}
	return ""
}

func applyFileConfig(cfg *Config, fc fileConfig, changed map[string]bool) error {
	// helper to set string if not changed and non-empty
	setS := func(flag string, v string, dst *string) {
		if v == "" {
			return
		}
		if !changed[flag] {
			*dst = v
		}
	}
	// helper to set int if not changed and >0
	setI := func(flag string, v int, dst *int) {
		if v <= 0 {
			return
		}
		if !changed[flag] {
			*dst = v
		}
	}
	// helper to set float if not changed and >0
	setF := func(flag string, v float64, dst *float64) {
		if v <= 0 {
			return
		}
		if !changed[flag] {
			*dst = v
		}
	}
	// helper to parse duration string
	setD := func(flag string, v string, dst *time.Duration) error {
		if v == "" {
			return nil
		}
		if changed[flag] {
			return nil
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("parse %s: %w", flag, err)
		}
		*dst = d
		return nil
	}

	setS("root", fc.Root, &cfg.Root)
	setS("node", fc.NodeID, &cfg.NodeID)
	setS("wal-dir", fc.WALDir, &cfg.WALDir)
	setS("remote-url", fc.RemoteURL, &cfg.RemoteURL)
	setS("remote-base", fc.RemoteBase, &cfg.RemoteBase)
	setS("network", fc.Network, &cfg.Network)
	setS("remote-node", fc.RemoteNode, &cfg.RemoteNode)
	setS("auth-key", fc.AuthKey, &cfg.AuthKey)
	if err := setD("poll", fc.PollInterval, &cfg.PollInterval); err != nil {
		return err
	}
	if err := setD("send-interval", fc.SendInterval, &cfg.SendInterval); err != nil {
		return err
	}
	if err := setD("hard-interval", fc.HardInterval, &cfg.HardInterval); err != nil {
		return err
	}
	if err := setD("timeout", fc.HTTPTimeout, &cfg.HTTPTimeout); err != nil {
		return err
	}
	setF("cpu-threshold", fc.CPUThreshold, &cfg.CPUThreshold)
	setF("net-threshold", fc.NetThreshold, &cfg.NetThreshold)
	setS("iface", fc.Iface, &cfg.Iface)
	setI("iface-speed", fc.IfaceSpeedMbps, &cfg.IfaceSpeedMbps)
	setI("max-batch-bytes", fc.MaxBatchBytes, &cfg.MaxBatchBytes)
	setS("state-dir", fc.StateDir, &cfg.StateDir)
	if fc.Verify != nil && !changed["verify"] {
		cfg.Verify = *fc.Verify
	}
	if fc.Meta != nil && !changed["meta"] {
		cfg.Meta = *fc.Meta
	}
	if fc.Once != nil && !changed["once"] {
		cfg.Once = *fc.Once
	}
	return nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// Exported shims for use from main without making helpers public globally.
func LoadFileConfig(path string) (fileConfig, error) { return loadFileConfig(path) }
func DefaultConfigPath() string                      { return defaultConfigPath() }
func ApplyFileConfig(cfg *Config, fc fileConfig, changed map[string]bool) error {
	return applyFileConfig(cfg, fc, changed)
}
func FileExists(p string) bool { return fileExists(p) }

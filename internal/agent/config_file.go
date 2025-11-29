package agent

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// fileConfig mirrors Config but uses strings for durations to make TOML friendly.
type fileConfig struct {
	NodeHome       string  `toml:"node_home"`
	NodeID         string  `toml:"node_id"`
	WALDir         string  `toml:"wal_dir"`
	ServiceURL     string  `toml:"service_url"`
	AuthKey         string  `toml:"api_key"`
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

// loadFileConfig reads and parses a TOML config file.
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

// defaultConfigPath returns the default configuration file path.
// Returns ~/.walship/config.toml if user home directory is accessible.
func defaultConfigPath() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".walship", "config.toml")
	}
	return ""
}

// applyFileConfig applies configuration from a file to the Config struct.
// It respects flags that have been explicitly set (changed map).
func applyFileConfig(cfg *Config, fc fileConfig, changed map[string]bool) error {
	s := newConfigSetter(changed)

	s.setString("node-home", fc.NodeHome, &cfg.NodeHome)
	s.setString("node-id", fc.NodeID, &cfg.NodeID)
	s.setString("wal-dir", fc.WALDir, &cfg.WALDir)
	s.setString("service-url", fc.ServiceURL, &cfg.ServiceURL)
	s.setString("auth-key", fc.AuthKey, &cfg.AuthKey)
	s.setString("iface", fc.Iface, &cfg.Iface)
	s.setString("state-dir", fc.StateDir, &cfg.StateDir)

	if err := s.setDuration("poll", fc.PollInterval, &cfg.PollInterval); err != nil {
		return err
	}
	if err := s.setDuration("send-interval", fc.SendInterval, &cfg.SendInterval); err != nil {
		return err
	}
	if err := s.setDuration("hard-interval", fc.HardInterval, &cfg.HardInterval); err != nil {
		return err
	}
	if err := s.setDuration("timeout", fc.HTTPTimeout, &cfg.HTTPTimeout); err != nil {
		return err
	}

	s.setFloat("cpu-threshold", fc.CPUThreshold, &cfg.CPUThreshold)
	s.setFloat("net-threshold", fc.NetThreshold, &cfg.NetThreshold)

	s.setInt("iface-speed", fc.IfaceSpeedMbps, &cfg.IfaceSpeedMbps)
	s.setInt("max-batch-bytes", fc.MaxBatchBytes, &cfg.MaxBatchBytes)

	s.setBool("verify", fc.Verify, &cfg.Verify)
	s.setBool("meta", fc.Meta, &cfg.Meta)
	s.setBool("once", fc.Once, &cfg.Once)

	return nil
}

// fileExists checks if a file exists at the given path.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Exported functions for use from main package without exposing internal helpers.

// LoadFileConfig reads and parses a TOML config file from the given path.
func LoadFileConfig(path string) (fileConfig, error) {
	return loadFileConfig(path)
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	return defaultConfigPath()
}

// ApplyFileConfig applies configuration from a file to the Config struct.
// It respects flags that have been explicitly set (changed map).
func ApplyFileConfig(cfg *Config, fc fileConfig, changed map[string]bool) error {
	return applyFileConfig(cfg, fc, changed)
}

// FileExists checks if a file exists at the given path.
func FileExists(p string) bool {
	return fileExists(p)
}

package agent

import (
	"fmt"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"time"
)

// FrameMeta matches tools/memlogger/writer.go schema for index lines.
// Fields are used to locate and read gzip members from the .gz file.
type FrameMeta struct {
	File    string `json:"file"`
	Frame   uint64 `json:"frame"`
	Off     uint64 `json:"off"`
	Len     uint64 `json:"len"`
	Recs    uint32 `json:"recs"`
	FirstTS int64  `json:"first_ts"`
	LastTS  int64  `json:"last_ts"`
	CRC32   uint32 `json:"crc32"`
}

type Config struct {
	Root   string
	NodeID string
	WALDir string

	RemoteURL  string
	RemoteBase string
	Network    string
	RemoteNode string
	AuthKey    string

	PollInterval time.Duration
	SendInterval time.Duration
	HardInterval time.Duration
	HTTPTimeout  time.Duration

	CPUThreshold   float64
	NetThreshold   float64
	Iface          string
	IfaceSpeedMbps int
	MaxBatchBytes  int
	StateDir       string
	Verify         bool
	Meta           bool
	Once           bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		NodeID:         "default",
		PollInterval:   500 * time.Millisecond,
		SendInterval:   5 * time.Second,
		HardInterval:   10 * time.Second,
		HTTPTimeout:    15 * time.Second,
		CPUThreshold:   0.85,
		NetThreshold:   0.70,
		IfaceSpeedMbps: 1000,
		MaxBatchBytes:  4 << 20, // 4MB
		StateDir:       defaultStateDir(),
		AuthKey:        os.Getenv("MEMAGENT_AUTH_KEY"),
	}
}

func defaultStateDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".walship")
	}
	return "."
}

// Validate checks the configuration for errors and sets derived defaults.
func (c *Config) Validate() error {
	if c.WALDir == "" {
		if c.Root != "" && c.NodeID != "" {
			// fallback derived layout
			c.WALDir = fmt.Sprintf("%s/data/log.wal/node-%s", c.Root, c.NodeID)
		} else if c.Root != "" {
			c.WALDir = fmt.Sprintf("%s/data/log.wal", c.Root)
		} else {
			return fmt.Errorf("wal-dir is required (or root)")
		}
	}

	if c.RemoteURL == "" {
		if c.RemoteBase != "" && c.Network != "" {
			node := c.RemoteNode
			if node == "" {
				node = c.NodeID
			}
			base := c.RemoteBase
			// ensure no trailing slash
			if len(base) > 0 && base[len(base)-1] == '/' {
				base = base[:len(base)-1]
			}
			c.RemoteURL = base + pathpkg.Join("", "/v1/ingest/", url.PathEscape(c.Network), "/", url.PathEscape(node), "/wal-frames")
		} else {
			return fmt.Errorf("remote-url is required (or remote-base + network)")
		}
	}

	if c.PollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	if c.SendInterval <= 0 {
		return fmt.Errorf("send interval must be positive")
	}

	return nil
}

// configSetter helps apply configuration values while respecting flag precedence.
// It only applies values if the corresponding flag hasn't been explicitly set.
type configSetter struct {
	changed map[string]bool
}

// newConfigSetter creates a new setter with the given changed flags map.
func newConfigSetter(changed map[string]bool) *configSetter {
	return &configSetter{changed: changed}
}

// setString sets a string value if not empty and flag not changed.
func (s *configSetter) setString(flag, value string, dst *string) {
	if value == "" || s.changed[flag] {
		return
	}
	*dst = value
}

// setInt sets an int value if positive and flag not changed.
func (s *configSetter) setInt(flag string, value int, dst *int) {
	if value <= 0 || s.changed[flag] {
		return
	}
	*dst = value
}

// setFloat sets a float64 value if positive and flag not changed.
func (s *configSetter) setFloat(flag string, value float64, dst *float64) {
	if value <= 0 || s.changed[flag] {
		return
	}
	*dst = value
}

// setDuration parses and sets a duration from string if valid and flag not changed.
func (s *configSetter) setDuration(flag, value string, dst *time.Duration) error {
	if value == "" || s.changed[flag] {
		return nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", flag, err)
	}
	*dst = d
	return nil
}

// setBool sets a bool value from a pointer if not nil and flag not changed.
func (s *configSetter) setBool(flag string, value *bool, dst *bool) {
	if value == nil || s.changed[flag] {
		return
	}
	*dst = *value
}

// setIntFromString parses a string to int and sets the destination if valid.
// Used for environment variables that come as strings.
func (s *configSetter) setIntFromString(flag, value string, dst *int) error {
	if value == "" || s.changed[flag] {
		return nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", flag, err)
	}
	if i <= 0 {
		return nil
	}
	*dst = i
	return nil
}

// setFloatFromString parses a string to float64 and sets the destination if valid.
// Used for environment variables that come as strings.
func (s *configSetter) setFloatFromString(flag, value string, dst *float64) error {
	if value == "" || s.changed[flag] {
		return nil
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("parse %s: %w", flag, err)
	}
	if f <= 0 {
		return nil
	}
	*dst = f
	return nil
}

// setBoolFromString parses a string to bool and sets the destination.
// Accepts "true", "1" as true, anything else as false.
// Used for environment variables that come as strings.
func (s *configSetter) setBoolFromString(flag, value string, dst *bool) {
	if value == "" || s.changed[flag] {
		return
	}
	*dst = value == "true" || value == "1"
}

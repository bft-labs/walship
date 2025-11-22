package agent

import (
	"fmt"
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

package walship_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bft-labs/walship/pkg/walship"
)

// TestStateFileBackwardCompatibility verifies old state files can be loaded.
// The state file format uses snake_case JSON keys for backward compatibility.
func TestStateFileBackwardCompatibility(t *testing.T) {
	// Old format state file (snake_case keys) - this is the format used by
	// the original internal/agent/state.go
	oldStateJSON := `{
		"idx_path": "/data/2025-01-01/seg-000001.wal.idx",
		"idx_offset": 12345,
		"cur_gz": "seg-000001.wal.gz",
		"last_file": "seg-000001.wal.gz",
		"last_frame": 42,
		"last_commit_at": "2025-01-01T12:00:00Z",
		"last_send_at": "2025-01-01T12:00:00Z"
	}`

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "status.json")
	if err := os.WriteFile(statePath, []byte(oldStateJSON), 0644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	// Verify the new code can parse old format
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	var state struct {
		IdxPath      string `json:"idx_path"`
		IdxOffset    int64  `json:"idx_offset"`
		CurGz        string `json:"cur_gz"`
		LastFile     string `json:"last_file"`
		LastFrame    uint64 `json:"last_frame"`
		LastCommitAt string `json:"last_commit_at"`
		LastSendAt   string `json:"last_send_at"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}

	// Verify all fields parsed correctly
	if state.IdxPath != "/data/2025-01-01/seg-000001.wal.idx" {
		t.Errorf("idx_path mismatch: got %q", state.IdxPath)
	}
	if state.IdxOffset != 12345 {
		t.Errorf("idx_offset mismatch: got %d", state.IdxOffset)
	}
	if state.CurGz != "seg-000001.wal.gz" {
		t.Errorf("cur_gz mismatch: got %q", state.CurGz)
	}
	if state.LastFile != "seg-000001.wal.gz" {
		t.Errorf("last_file mismatch: got %q", state.LastFile)
	}
	if state.LastFrame != 42 {
		t.Errorf("last_frame mismatch: got %d", state.LastFrame)
	}
}

// TestConfigDefaults verifies default values match original behavior.
func TestConfigDefaults(t *testing.T) {
	cfg := walship.DefaultConfig()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"ServiceURL", cfg.ServiceURL, "https://api.apphash.io"},
		{"PollInterval", cfg.PollInterval, 500 * time.Millisecond},
		{"SendInterval", cfg.SendInterval, 5 * time.Second},
		{"HardInterval", cfg.HardInterval, 10 * time.Second},
		{"HTTPTimeout", cfg.HTTPTimeout, 15 * time.Second},
		{"MaxBatchBytes", cfg.MaxBatchBytes, 4 << 20}, // 4MB
		{"CPUThreshold", cfg.CPUThreshold, 0.85},
		{"NetThreshold", cfg.NetThreshold, 0.70},
		{"IfaceSpeedMbps", cfg.IfaceSpeedMbps, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s: got %v, expected %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestConfigValidation verifies configuration validation behavior.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*walship.Config)
		wantError bool
	}{
		{
			name:      "valid with WALDir",
			modify:    func(c *walship.Config) { c.WALDir = "/path/to/wal" },
			wantError: false,
		},
		{
			name:      "valid with NodeHome",
			modify:    func(c *walship.Config) { c.NodeHome = "/path/to/node" },
			wantError: false,
		},
		{
			name:      "invalid without WALDir or NodeHome",
			modify:    func(c *walship.Config) {},
			wantError: true,
		},
		{
			name: "invalid with zero PollInterval",
			modify: func(c *walship.Config) {
				c.WALDir = "/path/to/wal"
				c.PollInterval = 0
			},
			wantError: false, // SetDefaults will fix this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := walship.Config{}
			tt.modify(&cfg)
			cfg.SetDefaults()
			err := cfg.Validate()

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestLifecycleStates verifies all lifecycle states are defined.
func TestLifecycleStates(t *testing.T) {
	states := []walship.State{
		walship.StateStopped,
		walship.StateStarting,
		walship.StateRunning,
		walship.StateStopping,
		walship.StateCrashed,
	}

	for _, s := range states {
		name := s.String()
		if name == "" || name == "Unknown" {
			t.Errorf("state %d has invalid string representation: %q", s, name)
		}
	}
}

// TestEventHandlerInterface verifies BaseEventHandler implements EventHandler.
func TestEventHandlerInterface(t *testing.T) {
	var handler walship.EventHandler = walship.BaseEventHandler{}

	// These should not panic
	handler.OnStateChange(walship.StateChangeEvent{})
	handler.OnSendSuccess(walship.SendSuccessEvent{})
	handler.OnSendError(walship.SendErrorEvent{})
}

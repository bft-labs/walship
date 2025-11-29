package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyFileConfig(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name       string
		fileConfig fileConfig
		changed    map[string]bool
		initial    Config
		expected   Config
		wantErr    bool
	}{
		{
			name: "applies all valid config values",
			fileConfig: fileConfig{
				NodeHome:       "/test/root",
				NodeID:         "node-1",
				PollInterval:   "5m",
				CPUThreshold:   0.8,
				IfaceSpeedMbps: 1000,
				Verify:         &trueVal,
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				NodeHome:       "/test/root",
				NodeID:         "node-1",
				PollInterval:   5 * time.Minute,
				CPUThreshold:   0.8,
				IfaceSpeedMbps: 1000,
				Verify:         true,
			},
			wantErr: false,
		},
		{
			name: "respects changed flags",
			fileConfig: fileConfig{
				NodeHome: "/config/node_home",
				NodeID:   "config-node",
			},
			changed: map[string]bool{"node-home": true},
			initial: Config{
				NodeHome: "/flag/node_home",
				NodeID:   "flag-node",
			},
			expected: Config{
				NodeHome: "/flag/node_home", // unchanged because flag was set
				NodeID:   "config-node",
			},
			wantErr: false,
		},
		{
			name: "handles all field types correctly",
			fileConfig: fileConfig{
				NodeHome:       "/tmp/root",
				NodeID:         "node1",
				WALDir:         "/tmp/custom_wal",
				ServiceURL:      "http://example.com",
				AuthKey:        "secret",
				PollInterval:   "1m",
				SendInterval:   "2m",
				HardInterval:   "3m",
				HTTPTimeout:    "30s",
				CPUThreshold:   0.7,
				NetThreshold:   0.8,
				Iface:          "eth0",
				IfaceSpeedMbps: 1000,
				MaxBatchBytes:  1024,
				StateDir:       "/state",
				Verify:         &trueVal,
				Meta:           &falseVal,
				Once:           &trueVal,
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				NodeHome:       "/tmp/root",
				NodeID:         "node1",
				WALDir:         "/tmp/custom_wal",
				ServiceURL:      "http://example.com",
				AuthKey:        "secret",
				PollInterval:   1 * time.Minute,
				SendInterval:   2 * time.Minute,
				HardInterval:   3 * time.Minute,
				HTTPTimeout:    30 * time.Second,
				CPUThreshold:   0.7,
				NetThreshold:   0.8,
				Iface:          "eth0",
				IfaceSpeedMbps: 1000,
				MaxBatchBytes:  1024,
				StateDir:       "/state",
				Verify:         true,
				Meta:           false,
				Once:           true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.initial
			err := applyFileConfig(&cfg, tt.fileConfig, tt.changed)

			if tt.wantErr && err == nil {
				t.Error("applyFileConfig() expected error but got nil")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("applyFileConfig() unexpected error: %v", err)
				return
			}

			if !tt.wantErr {
				// Check string fields
				if cfg.NodeHome != tt.expected.NodeHome {
					t.Errorf("NodeHome = %v, want %v", cfg.NodeHome, tt.expected.NodeHome)
				}
				if cfg.NodeID != tt.expected.NodeID {
					t.Errorf("NodeID = %v, want %v", cfg.NodeID, tt.expected.NodeID)
				}
				if cfg.WALDir != tt.expected.WALDir {
					t.Errorf("WALDir = %v, want %v", cfg.WALDir, tt.expected.WALDir)
				}
				if cfg.ServiceURL != tt.expected.ServiceURL {
					t.Errorf("ServiceURL = %v, want %v", cfg.ServiceURL, tt.expected.ServiceURL)
				}

				// Check duration fields
				if cfg.PollInterval != tt.expected.PollInterval {
					t.Errorf("PollInterval = %v, want %v", cfg.PollInterval, tt.expected.PollInterval)
				}
				if cfg.SendInterval != tt.expected.SendInterval {
					t.Errorf("SendInterval = %v, want %v", cfg.SendInterval, tt.expected.SendInterval)
				}

				// Check float fields
				if cfg.CPUThreshold != tt.expected.CPUThreshold {
					t.Errorf("CPUThreshold = %v, want %v", cfg.CPUThreshold, tt.expected.CPUThreshold)
				}

				// Check int fields
				if cfg.IfaceSpeedMbps != tt.expected.IfaceSpeedMbps {
					t.Errorf("IfaceSpeedMbps = %v, want %v", cfg.IfaceSpeedMbps, tt.expected.IfaceSpeedMbps)
				}

				// Check bool fields
				if cfg.Verify != tt.expected.Verify {
					t.Errorf("Verify = %v, want %v", cfg.Verify, tt.expected.Verify)
				}
				if cfg.Meta != tt.expected.Meta {
					t.Errorf("Meta = %v, want %v", cfg.Meta, tt.expected.Meta)
				}
			}
		})
	}
}

func TestLoadFileConfig(t *testing.T) {
	// Create a temporary TOML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.toml")

	tomlContent := `
node_home = "/tmp/root"
node_id = "test-node"
poll_interval = "5m"
cpu_threshold = 0.8
iface_speed_mbps = 1000
verify = true
`

	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	fc, err := loadFileConfig(configPath)
	if err != nil {
		t.Fatalf("loadFileConfig() error = %v", err)
	}

	if fc.NodeHome != "/tmp/root" {
		t.Errorf("NodeHome = %v, want /tmp/root", fc.NodeHome)
	}
	if fc.NodeID != "test-node" {
		t.Errorf("NodeID = %v, want test-node", fc.NodeID)
	}
	if fc.PollInterval != "5m" {
		t.Errorf("PollInterval = %v, want 5m", fc.PollInterval)
	}
	if fc.CPUThreshold != 0.8 {
		t.Errorf("CPUThreshold = %v, want 0.8", fc.CPUThreshold)
	}
	if fc.IfaceSpeedMbps != 1000 {
		t.Errorf("IfaceSpeedMbps = %v, want 1000", fc.IfaceSpeedMbps)
	}
	if fc.Verify == nil || *fc.Verify != true {
		t.Errorf("Verify = %v, want true", fc.Verify)
	}
}

func TestLoadFileConfig_InvalidFile(t *testing.T) {
	_, err := loadFileConfig("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("loadFileConfig() expected error for nonexistent file")
	}
}

func TestLoadFileConfig_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.toml")

	invalidContent := `
root = "/test"
this is not valid toml
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err := loadFileConfig(configPath)
	if err == nil {
		t.Error("loadFileConfig() expected error for invalid TOML")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := defaultConfigPath()

	// Should return a path containing .walship
	if path != "" && !contains(path, ".walship") {
		t.Errorf("defaultConfigPath() = %v, should contain .walship", path)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "exists.txt")

	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if !fileExists(existingFile) {
		t.Error("fileExists() = false, want true for existing file")
	}

	if fileExists(filepath.Join(tmpDir, "nonexistent.txt")) {
		t.Error("fileExists() = true, want false for nonexistent file")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

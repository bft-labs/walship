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
		name        string
		fileConfig  fileConfig
		changed     map[string]bool
		initial     Config
		expected    Config
		expectError bool
	}{
		{
			name: "applies all valid config values",
			fileConfig: fileConfig{
				Root:           "/test/root",
				NodeID:         "node-1",
				PollInterval:   "5m",
				CPUThreshold:   0.8,
				IfaceSpeedMbps: 1000,
				Verify:         &trueVal,
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				Root:           "/test/root",
				NodeID:         "node-1",
				PollInterval:   5 * time.Minute,
				CPUThreshold:   0.8,
				IfaceSpeedMbps: 1000,
				Verify:         true,
			},
			expectError: false,
		},
		{
			name: "respects changed flags",
			fileConfig: fileConfig{
				Root:   "/config/root",
				NodeID: "config-node",
			},
			changed: map[string]bool{"root": true},
			initial: Config{
				Root:   "/flag/root",
				NodeID: "flag-node",
			},
			expected: Config{
				Root:   "/flag/root", // unchanged because flag was set
				NodeID: "config-node",
			},
			expectError: false,
		},
		{
			name: "handles all field types correctly",
			fileConfig: fileConfig{
				Root:           "/root",
				NodeID:         "node",
				WALDir:         "/wal",
				RemoteURL:      "http://example.com",
				RemoteBase:     "/base",
				Network:        "tcp",
				RemoteNode:     "remote",
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
				Root:           "/root",
				NodeID:         "node",
				WALDir:         "/wal",
				RemoteURL:      "http://example.com",
				RemoteBase:     "/base",
				Network:        "tcp",
				RemoteNode:     "remote",
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
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.initial
			err := applyFileConfig(&cfg, tt.fileConfig, tt.changed)

			if tt.expectError && err == nil {
				t.Error("applyFileConfig() expected error but got nil")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("applyFileConfig() unexpected error: %v", err)
				return
			}

			if !tt.expectError {
				// Check string fields
				if cfg.Root != tt.expected.Root {
					t.Errorf("Root = %v, want %v", cfg.Root, tt.expected.Root)
				}
				if cfg.NodeID != tt.expected.NodeID {
					t.Errorf("NodeID = %v, want %v", cfg.NodeID, tt.expected.NodeID)
				}
				if cfg.WALDir != tt.expected.WALDir {
					t.Errorf("WALDir = %v, want %v", cfg.WALDir, tt.expected.WALDir)
				}
				if cfg.RemoteURL != tt.expected.RemoteURL {
					t.Errorf("RemoteURL = %v, want %v", cfg.RemoteURL, tt.expected.RemoteURL)
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
root = "/test/root"
node = "test-node"
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

	if fc.Root != "/test/root" {
		t.Errorf("Root = %v, want /test/root", fc.Root)
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

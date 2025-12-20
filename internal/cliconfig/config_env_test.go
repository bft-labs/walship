package cliconfig

import (
	"os"
	"testing"
	"time"
)

func TestApplyEnvConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		changed  map[string]bool
		initial  Config
		expected Config
		wantErr  bool
	}{
		{
			name: "applies all valid env vars",
			envVars: map[string]string{
				"WALSHIP_NODE_HOME":        "/env/root",
				"WALSHIP_NODE_ID":          "env-node",
				"WALSHIP_POLL_INTERVAL":    "10m",
				"WALSHIP_CPU_THRESHOLD":    "0.9",
				"WALSHIP_IFACE_SPEED_MBPS": "100",
				"WALSHIP_VERIFY":           "true",
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				NodeHome:       "/env/root",
				NodeID:         "env-node",
				PollInterval:   10 * time.Minute,
				CPUThreshold:   0.9,
				IfaceSpeedMbps: 100,
				Verify:         true,
			},
			wantErr: false,
		},
		{
			name: "respects changed flags",
			envVars: map[string]string{
				"WALSHIP_NODE_HOME": "/env/root",
				"WALSHIP_NODE_ID":   "env-node",
			},
			changed: map[string]bool{"node-home": true},
			initial: Config{
				NodeID: "env-node",
			},
			expected: Config{
				NodeID: "env-node",
			},
			wantErr: false,
		},
		{
			name: "returns error for invalid duration",
			envVars: map[string]string{
				"WALSHIP_POLL_INTERVAL": "not-a-duration",
			},
			changed:  map[string]bool{},
			initial:  Config{},
			expected: Config{},
			wantErr:  true,
		},
		{
			name: "returns error for invalid int",
			envVars: map[string]string{
				"WALSHIP_IFACE_SPEED_MBPS": "not-a-number",
			},
			changed:  map[string]bool{},
			initial:  Config{},
			expected: Config{},
			wantErr:  true,
		},
		{
			name: "returns error for invalid float",
			envVars: map[string]string{
				"WALSHIP_CPU_THRESHOLD": "not-a-float",
			},
			changed:  map[string]bool{},
			initial:  Config{},
			expected: Config{},
			wantErr:  true,
		},
		{
			name: "handles bool '1' as true",
			envVars: map[string]string{
				"WALSHIP_VERIFY": "1",
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				Verify: true,
			},
			wantErr: false,
		},
		{
			name: "handles bool 'false' as false",
			envVars: map[string]string{
				"WALSHIP_VERIFY": "false",
			},
			changed: map[string]bool{},
			initial: Config{Verify: true},
			expected: Config{
				Verify: false,
			},
			wantErr: false,
		},
		{
			name: "handles all field types correctly",
			envVars: map[string]string{
				"WALSHIP_NODE_HOME":        "/root",
				"WALSHIP_NODE_ID":          "node",
				"WALSHIP_WAL_DIR":          "/wal",
				"WALSHIP_SERVICE_URL":      "http://example.com",
				"WALSHIP_AUTH_KEY":         "secret",
				"WALSHIP_POLL_INTERVAL":    "1m",
				"WALSHIP_SEND_INTERVAL":    "2m",
				"WALSHIP_HARD_INTERVAL":    "3m",
				"WALSHIP_HTTP_TIMEOUT":     "30s",
				"WALSHIP_CPU_THRESHOLD":    "0.7",
				"WALSHIP_NET_THRESHOLD":    "0.8",
				"WALSHIP_IFACE":            "eth0",
				"WALSHIP_IFACE_SPEED_MBPS": "1000",
				"WALSHIP_MAX_BATCH_BYTES":  "1024",
				"WALSHIP_STATE_DIR":        "/state",
				"WALSHIP_VERIFY":           "true",
				"WALSHIP_META":             "false",
				"WALSHIP_ONCE":             "1",
			},
			changed: map[string]bool{},
			initial: Config{},
			expected: Config{
				NodeHome:       "/root",
				NodeID:         "node",
				WALDir:         "/wal",
				ServiceURL:     "http://example.com",
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
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			// Clean up after test
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			cfg := tt.initial
			err := ApplyEnvConfig(&cfg, tt.changed)

			if tt.wantErr && err == nil {
				t.Error("ApplyEnvConfig() expected error but got nil")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ApplyEnvConfig() unexpected error: %v", err)
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
				if cfg.Once != tt.expected.Once {
					t.Errorf("Once = %v, want %v", cfg.Once, tt.expected.Once)
				}
			}
		})
	}
}

// Integration test: precedence order (CLI > Env > File)
func TestConfigPrecedence(t *testing.T) {
	trueVal := true

	// Setup file config
	fileConf := FileConfig{
		NodeHome: "/file/root",
		NodeID:   "file-node",
		Verify:   &trueVal,
	}

	// Setup env vars
	os.Setenv("WALSHIP_NODE_HOME", "/env/root")
	os.Setenv("WALSHIP_NODE_ID", "env-node")
	os.Setenv("WALSHIP_WAL_DIR", "/env/wal")
	defer func() {
		os.Unsetenv("WALSHIP_NODE_HOME")
		os.Unsetenv("WALSHIP_NODE_ID")
		os.Unsetenv("WALSHIP_WAL_DIR")
	}()

	// Simulate CLI flags
	changed := map[string]bool{
		"node-home": true, // CLI flag was set for root
	}

	cfg := Config{
		NodeHome: "/cli/root", // This should remain (CLI wins)
	}

	// Apply file config
	if err := ApplyFileConfig(&cfg, fileConf, changed); err != nil {
		t.Fatalf("ApplyFileConfig failed: %v", err)
	}

	// Apply env config
	if err := ApplyEnvConfig(&cfg, changed); err != nil {
		t.Fatalf("ApplyEnvConfig failed: %v", err)
	}

	// Verify precedence: CLI > Env > File
	if cfg.NodeHome != "/cli/root" {
		t.Errorf("NodeHome = %v, want /cli/root (CLI should win)", cfg.NodeHome)
	}
	if cfg.NodeID != "env-node" {
		t.Errorf("NodeID = %v, want env-node (env should override file)", cfg.NodeID)
	}
	if cfg.WALDir != "/env/wal" {
		t.Errorf("WALDir = %v, want /env/wal (env should set)", cfg.WALDir)
	}
	if cfg.Verify != true {
		t.Errorf("Verify = %v, want true (file should set)", cfg.Verify)
	}
}

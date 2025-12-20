package cliconfig

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.NodeID != "default" {
		t.Errorf("NodeID = %v, want default", cfg.NodeID)
	}
	if cfg.PollInterval != 500*time.Millisecond {
		t.Errorf("PollInterval = %v, want 500ms", cfg.PollInterval)
	}
	if cfg.ServiceURL != DefaultServiceURL {
		t.Errorf("ServiceURL = %v, want %v", cfg.ServiceURL, DefaultServiceURL)
	}
	if cfg.MaxBatchBytes != 4<<20 {
		t.Errorf("MaxBatchBytes = %v, want 4MB", cfg.MaxBatchBytes)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		wantErr        bool
		wantServiceURL string
	}{
		{
			name: "valid minimal config",
			config: Config{
				NodeHome:     "/tmp/root",
				WALDir:       "/tmp/wal",
				ServiceURL:   "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing wal dir and node-home",
			config: Config{
				ServiceURL:   "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
		{
			name: "derived wal dir from node-home",
			config: Config{
				NodeHome:     "/tmp/root",
				NodeID:       "default",
				ServiceURL:   "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing remote url and base",
			config: Config{
				WALDir:       "/tmp/wal",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
		{
			name: "service url defaults when omitted",
			config: Config{
				NodeHome:     "/tmp/root",
				WALDir:       "/tmp/wal",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr:        false,
			wantServiceURL: DefaultServiceURL,
		},
		{
			name: "valid with webhook url",
			config: Config{
				NodeHome:     "/tmp/root",
				WALDir:       "/tmp/wal",
				ServiceURL:   "http://localhost:8080/v1/ingest",
				NodeID:       "node1",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid poll interval",
			config: Config{
				NodeHome:     "/tmp/root",
				WALDir:       "/tmp/wal",
				ServiceURL:   "http://localhost:8080",
				PollInterval: -1,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
		{
			name: "missing node-home is always error",
			config: Config{
				ChainID:      "test-chain",
				NodeID:       "test-node",
				WALDir:       "/tmp/wal",
				ServiceURL:   "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.wantServiceURL != "" && tt.config.ServiceURL != tt.wantServiceURL {
				t.Errorf("ServiceURL = %v, want %v", tt.config.ServiceURL, tt.wantServiceURL)
			}
		})
	}
}

func TestConfig_Validate_Derivations(t *testing.T) {
	// Test WALDir derivation
	c1 := Config{
		NodeHome:     "/app",
		NodeID:       "node1",
		ServiceURL:   "http://example.com",
		PollInterval: time.Second,
		SendInterval: time.Second,
	}
	if err := c1.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	expectedWAL := "/app/data/log.wal/node-node1"
	if c1.WALDir != expectedWAL {
		t.Errorf("WALDir = %v, want %v", c1.WALDir, expectedWAL)
	}
	if c1.StateDir != expectedWAL {
		t.Errorf("StateDir = %v, want %v", c1.StateDir, expectedWAL)
	}

	// Test RemoteURL derivation
	c2 := Config{
		NodeHome:     "/tmp/root",
		WALDir:       "/wal",
		ServiceURL:   "http://api.com/v1/ingest/",
		NodeID:       "validator-1",
		PollInterval: time.Second,
		SendInterval: time.Second,
	}
	if err := c2.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	expectedURL := "http://api.com/v1/ingest"
	if c2.ServiceURL != expectedURL {
		t.Errorf("ServiceURL = %v, want %v", c2.ServiceURL, expectedURL)
	}

	// StateDir respects explicit override
	c3 := Config{
		NodeHome:     "/tmp/root",
		NodeID:       "validator-2",
		WALDir:       "/custom/wal",
		StateDir:     "/state",
		ServiceURL:   "http://api.com/v1/ingest",
		PollInterval: time.Second,
		SendInterval: time.Second,
	}
	if err := c3.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if c3.StateDir != "/state" {
		t.Errorf("StateDir = %v, want /state", c3.StateDir)
	}
}

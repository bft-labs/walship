package agent

import (
	"os"
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
	if cfg.MaxBatchBytes != 4<<20 {
		t.Errorf("MaxBatchBytes = %v, want 4MB", cfg.MaxBatchBytes)
	}
	// Check if AuthKey default is respected (depends on env, but logic is there)
	if val := os.Getenv("MEMAGENT_AUTH_KEY"); val != "" {
		if cfg.AuthKey != val {
			t.Errorf("AuthKey = %v, want %v (from env)", cfg.AuthKey, val)
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid minimal config",
			config: Config{
				WALDir:       "/tmp/wal",
				RemoteURL:    "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing wal dir and root",
			config: Config{
				RemoteURL:    "http://localhost:8080",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
		{
			name: "derived wal dir from root",
			config: Config{
				Root:         "/tmp/root",
				RemoteURL:    "http://localhost:8080",
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
			name: "derived remote url",
			config: Config{
				WALDir:       "/tmp/wal",
				RemoteBase:   "http://localhost:8080",
				Network:      "testnet",
				NodeID:       "node1",
				PollInterval: time.Second,
				SendInterval: time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid poll interval",
			config: Config{
				WALDir:       "/tmp/wal",
				RemoteURL:    "http://localhost:8080",
				PollInterval: -1,
				SendInterval: time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Validate_Derivations(t *testing.T) {
	// Test WALDir derivation
	c1 := Config{
		Root:         "/app",
		NodeID:       "node1",
		RemoteURL:    "http://example.com",
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

	// Test RemoteURL derivation
	c2 := Config{
		WALDir:       "/wal",
		RemoteBase:   "http://api.com/", // trailing slash should be handled
		Network:      "cosmos",
		NodeID:       "validator-1",
		PollInterval: time.Second,
		SendInterval: time.Second,
	}
	if err := c2.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	expectedURL := "http://api.com/v1/ingest/cosmos/validator-1/wal-frames"
	if c2.RemoteURL != expectedURL {
		t.Errorf("RemoteURL = %v, want %v", c2.RemoteURL, expectedURL)
	}
}

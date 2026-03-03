package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadNodeInfo_ExplicitValues(t *testing.T) {
	cfg := Config{
		ChainID: "manual-chain",
		NodeID:  "manual-node",
	}
	if err := LoadNodeInfo(&cfg); err != nil {
		t.Fatalf("LoadNodeInfo() unexpected error: %v", err)
	}
	if cfg.ChainID != "manual-chain" {
		t.Errorf("ChainID = %v, want manual-chain", cfg.ChainID)
	}
	if cfg.NodeID != "manual-node" {
		t.Errorf("NodeID = %v, want manual-node", cfg.NodeID)
	}
}

func TestLoadNodeInfo_MissingChainID(t *testing.T) {
	cfg := Config{
		NodeID: "some-node",
	}
	err := LoadNodeInfo(&cfg)
	if err == nil {
		t.Fatal("LoadNodeInfo() expected error for missing chain-id")
	}
	if got := err.Error(); got != "chain-id is required (set --chain-id, WALSHIP_CHAIN_ID, or chain_id in config file)" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLoadNodeInfo_MissingNodeID_NoBinary(t *testing.T) {
	cfg := Config{
		ChainID: "test-chain",
	}
	err := LoadNodeInfo(&cfg)
	if err == nil {
		t.Fatal("LoadNodeInfo() expected error for missing node-id without binary")
	}
	if got := err.Error(); got != "node-id is required (set --node-id, WALSHIP_NODE_ID, or use --chain-binary-path)" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLoadNodeInfo_DefaultNodeID_NoBinary(t *testing.T) {
	cfg := Config{
		ChainID: "test-chain",
		NodeID:  "default",
	}
	err := LoadNodeInfo(&cfg)
	if err == nil {
		t.Fatal("LoadNodeInfo() expected error for default node-id without binary")
	}
}

func TestLoadNodeInfo_WithBinary(t *testing.T) {
	// Create a mock binary that outputs a fake node ID.
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "mock-binary")

	var script string
	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell script")
	}
	// Shell script that echoes a valid 40-char hex node ID.
	script = "#!/bin/sh\necho 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2'\n"

	if err := os.WriteFile(mockBinary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		ChainID:         "test-chain",
		NodeID:          "default",
		ChainBinaryPath: mockBinary,
		NodeHome:        "/dummy",
	}
	if err := LoadNodeInfo(&cfg); err != nil {
		t.Fatalf("LoadNodeInfo() unexpected error: %v", err)
	}
	if cfg.NodeID != "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2" {
		t.Errorf("NodeID = %v, want a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", cfg.NodeID)
	}
}

func TestQueryNodeID_InvalidBinaryPath(t *testing.T) {
	_, err := queryNodeID("/nonexistent/binary", "/tmp")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestQueryNodeID_BadOutput(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "bad-output")

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell script")
	}
	// Script that outputs something that doesn't look like a node ID.
	script := "#!/bin/sh\necho 'NOT-A-VALID-NODE-ID'\n"
	if err := os.WriteFile(mockBinary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := queryNodeID(mockBinary, "/tmp")
	if err == nil {
		t.Fatal("expected error for invalid node ID output")
	}
}

func TestQueryNodeID_UppercaseNormalized(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinary := filepath.Join(tmpDir, "upper-binary")

	if runtime.GOOS == "windows" {
		t.Skip("test requires unix shell script")
	}
	// Output uppercase hex — should be normalized to lowercase.
	script := "#!/bin/sh\necho 'A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2'\n"
	if err := os.WriteFile(mockBinary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	nodeID, err := queryNodeID(mockBinary, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nodeID != "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2" {
		t.Errorf("nodeID = %v, want lowercase", nodeID)
	}
}

func TestResolveBinary_Empty(t *testing.T) {
	_, err := resolveBinary("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestResolveBinary_NullByte(t *testing.T) {
	_, err := resolveBinary("/bin/\x00evil")
	if err == nil {
		t.Fatal("expected error for null byte in path")
	}
}

func TestResolveBinary_NotRegularFile(t *testing.T) {
	// A directory is not a regular file.
	tmpDir := t.TempDir()
	_, err := resolveBinary(tmpDir)
	if err == nil {
		t.Fatal("expected error for directory as binary path")
	}
}

func TestSanitizeOutput(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello\nworld", "hello\nworld"},
		{string(make([]byte, 200)), ""},
	}
	for _, tt := range tests {
		got := sanitizeOutput(tt.input)
		if len(tt.input) > 120 && len(got) > 124 { // 120 + "..."
			t.Errorf("sanitizeOutput did not truncate long input")
		}
		_ = got
	}
}

package cliconfig

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNodeInfo(t *testing.T) {
	// Create temp dir for file-based tests
	tmpDir, err := os.MkdirTemp("", "cliconfig-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup dummy files
	configDir := filepath.Join(tmpDir, "config")
	if err := os.Mkdir(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create genesis.json
	genesis := genesisDoc{ChainID: "test-chain-1"}
	genesisBytes, _ := json.Marshal(genesis)
	if err := os.WriteFile(filepath.Join(configDir, "genesis.json"), genesisBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Create node_key.json with valid private key
	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	privKeyBase64 := base64.StdEncoding.EncodeToString(privKey)

	// Calculate expected ID
	sha := sha256.Sum256(pubKey)
	expectedID := hex.EncodeToString(sha[:20])

	nodeKeyStruct := struct {
		PrivKey struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"priv_key"`
	}{}
	nodeKeyStruct.PrivKey.Type = "tendermint/PrivKeyEd25519"
	nodeKeyStruct.PrivKey.Value = privKeyBase64

	nodeKeyBytes, _ := json.Marshal(nodeKeyStruct)
	if err := os.WriteFile(filepath.Join(configDir, "node_key.json"), nodeKeyBytes, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		wantChainID string
		wantNodeID  string
	}{
		{
			name: "load from files (missing IDs)",
			cfg: Config{
				NodeHome: tmpDir,
			},
			wantChainID: "test-chain-1",
			wantNodeID:  expectedID,
			wantErr:     false,
		},
		{
			name: "ids already set",
			cfg: Config{
				ChainID: "manual-chain",
				NodeID:  "manual-node",
			},
			wantChainID: "manual-chain",
			wantNodeID:  "manual-node",
			wantErr:     false,
		},
		{
			name:    "missing root and missing chain-id",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "invalid node_key.json (bad json)",
			cfg: Config{
				NodeHome: filepath.Join(tmpDir, "bad_json"),
			},
			wantErr: true,
		},
		{
			name: "invalid node_key.json (bad base64)",
			cfg: Config{
				NodeHome: filepath.Join(tmpDir, "bad_base64"),
			},
			wantErr: true,
		},
		{
			name: "invalid node_key.json (bad key length)",
			cfg: Config{
				NodeHome: filepath.Join(tmpDir, "bad_length"),
			},
			wantErr: true,
		},
	}

	// Setup bad files
	badJSONDir := filepath.Join(tmpDir, "bad_json", "config")
	os.MkdirAll(badJSONDir, 0755)
	os.WriteFile(filepath.Join(badJSONDir, "genesis.json"), genesisBytes, 0644)
	os.WriteFile(filepath.Join(badJSONDir, "node_key.json"), []byte("{invalid-json"), 0644)

	badBase64Dir := filepath.Join(tmpDir, "bad_base64", "config")
	os.MkdirAll(badBase64Dir, 0755)
	os.WriteFile(filepath.Join(badBase64Dir, "genesis.json"), genesisBytes, 0644)
	badBase64Key := nodeKeyStruct
	badBase64Key.PrivKey.Value = "not-base64!"
	badBase64Bytes, _ := json.Marshal(badBase64Key)
	os.WriteFile(filepath.Join(badBase64Dir, "node_key.json"), badBase64Bytes, 0644)

	badLengthDir := filepath.Join(tmpDir, "bad_length", "config")
	os.MkdirAll(badLengthDir, 0755)
	os.WriteFile(filepath.Join(badLengthDir, "genesis.json"), genesisBytes, 0644)
	badLengthKey := nodeKeyStruct
	badLengthKey.PrivKey.Value = base64.StdEncoding.EncodeToString([]byte("short-key"))
	badLengthBytes, _ := json.Marshal(badLengthKey)
	os.WriteFile(filepath.Join(badLengthDir, "node_key.json"), badLengthBytes, 0644)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of config to avoid modifying the original in the slice
			cfg := tt.cfg
			err := LoadNodeInfo(&cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadNodeInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cfg.ChainID != tt.wantChainID {
					t.Errorf("ChainID = %v, want %v", cfg.ChainID, tt.wantChainID)
				}
				if cfg.NodeID != tt.wantNodeID {
					t.Errorf("NodeID = %v, want %v", cfg.NodeID, tt.wantNodeID)
				}
			}
		})
	}
}

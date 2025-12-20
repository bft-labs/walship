package cliconfig

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir       = "config"
	DefaultGenesisJSONName = "genesis.json"
	DefaultNodeKeyName     = "node_key.json"
)

// LoadNodeInfo loads ChainID and NodeID from files if they are not already set in the config.
// It respects the NodeHome directory in the config.
func LoadNodeInfo(cfg *Config) error {
	// Read ChainID from genesis.json if not set
	if cfg.ChainID == "" {
		if cfg.NodeHome != "" {
			chainID, err := readChainID(cfg.NodeHome)
			if err != nil {
				return fmt.Errorf("read chain id: %w", err)
			}
			cfg.ChainID = chainID
		} else {
			return fmt.Errorf("chain-id is required (or node-home)")
		}
	}

	// Read NodeID from node_key.json if not set (or default)
	if cfg.NodeID == "" || cfg.NodeID == "default" {
		if cfg.NodeHome != "" {
			nodeID, err := readNodeID(cfg.NodeHome)
			if err != nil {
				return fmt.Errorf("read node id: %w", err)
			}
			cfg.NodeID = nodeID
		} else {
			return fmt.Errorf("node-id is required (or node-home)")
		}
	}
	return nil
}

func readChainID(nodeHome string) (string, error) {
	path := rootify(filepath.Join(DefaultConfigDir, DefaultGenesisJSONName), nodeHome)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc genesisDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", err
	}
	return doc.ChainID, nil
}

func readNodeID(nodeHome string) (string, error) {
	path := rootify(filepath.Join(DefaultConfigDir, DefaultNodeKeyName), nodeHome)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var nk nodeKey
	if err := json.Unmarshal(b, &nk); err != nil {
		return "", err
	}

	// Decode base64 private key
	privKeyBytes, err := base64.StdEncoding.DecodeString(nk.PrivKey.Value)
	if err != nil {
		return "", fmt.Errorf("decode priv key: %w", err)
	}

	// Ed25519 private key is 64 bytes. Public key is the last 32 bytes.
	// Or we can regenerate it from the seed (first 32 bytes).
	// CometBFT uses standard Ed25519.
	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid priv key length: %d", len(privKeyBytes))
	}

	privKey := ed25519.PrivateKey(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Address is the first 20 bytes of SHA256(PubKey)
	sha := sha256.Sum256(pubKey)
	address := sha[:20]

	return hex.EncodeToString(address), nil
}

// rootify returns the absolute path if path is absolute,
// otherwise it joins nodeHome and path.
func rootify(path, nodeHome string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(nodeHome, path)
}

type genesisDoc struct {
	ChainID string `json:"chain_id"`
}

type nodeKey struct {
	PrivKey struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"priv_key"`
}

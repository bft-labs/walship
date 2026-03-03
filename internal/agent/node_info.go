package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	// queryTimeout is the maximum time allowed for binary execution.
	queryTimeout = 10 * time.Second
)

// validNodeID matches a 40-character lowercase hex string (CometBFT node ID format).
var validNodeID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// LoadNodeInfo resolves ChainID and NodeID from explicit config or by querying the
// chain binary. It never reads private key material from disk.
//
// Resolution order for each field:
//  1. Already set via flag / env / config file — keep as-is.
//  2. ChainBinaryPath is set — query the binary.
//  3. Neither — return an actionable error.
func LoadNodeInfo(cfg *Config) error {
	if cfg.ChainID == "" {
		return fmt.Errorf("chain-id is required (set --chain-id, WALSHIP_CHAIN_ID, or chain_id in config file)")
	}

	if cfg.NodeID == "" || cfg.NodeID == "default" {
		if cfg.ChainBinaryPath != "" {
			nodeID, err := queryNodeID(cfg.ChainBinaryPath, cfg.NodeHome)
			if err != nil {
				return fmt.Errorf("query node id: %w", err)
			}
			cfg.NodeID = nodeID
		} else {
			return fmt.Errorf("node-id is required (set --node-id, WALSHIP_NODE_ID, or use --chain-binary-path)")
		}
	}
	return nil
}

// queryNodeID executes "$binary comet show-node-id --home $nodeHome" and returns
// the validated node ID. The binary path is resolved and verified before execution.
func queryNodeID(binaryPath, nodeHome string) (string, error) {
	resolved, err := resolveBinary(binaryPath)
	if err != nil {
		return "", err
	}

	args := []string{"comet", "show-node-id"}
	if nodeHome != "" {
		args = append(args, "--home", nodeHome)
	}

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, resolved, args...)
	// Isolate child process: discard stdin, capture only stdout.
	cmd.Stdin = nil

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("binary timed out after %s", queryTimeout)
		}
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			stderr = strings.TrimSpace(string(ee.Stderr))
			if len(stderr) > 256 {
				stderr = stderr[:256]
			}
		}
		if stderr != "" {
			return "", fmt.Errorf("exec %s: %w: %s", resolved, err, stderr)
		}
		return "", fmt.Errorf("exec %s: %w", resolved, err)
	}

	nodeID := strings.ToLower(strings.TrimSpace(string(out)))

	if !validNodeID.MatchString(nodeID) {
		return "", fmt.Errorf("unexpected node-id format from binary (expected 40 hex chars): %q", sanitizeOutput(nodeID))
	}

	return nodeID, nil
}

// resolveBinary validates and resolves the chain binary path.
// It ensures:
//   - The path is non-empty and contains no null bytes.
//   - The path resolves to an existing, regular, executable file.
func resolveBinary(binaryPath string) (string, error) {
	if binaryPath == "" {
		return "", fmt.Errorf("chain-binary-path is empty")
	}

	// Reject null bytes (path injection vector).
	if strings.ContainsRune(binaryPath, 0) {
		return "", fmt.Errorf("chain-binary-path contains invalid characters")
	}

	// exec.LookPath resolves both absolute paths and $PATH lookups.
	resolved, err := exec.LookPath(binaryPath)
	if err != nil {
		return "", fmt.Errorf("chain binary not found: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat chain binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("chain-binary-path is not a regular file: %s", resolved)
	}

	return resolved, nil
}

// sanitizeOutput truncates and cleans output for safe inclusion in error messages.
func sanitizeOutput(s string) string {
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' {
			return -1
		}
		return r
	}, s)
}

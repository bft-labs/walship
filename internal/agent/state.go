package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type state struct {
	IdxPath      string    `json:"idx_path"`
	IdxOffset    int64     `json:"idx_offset"`
	CurGz        string    `json:"cur_gz"`
	LastFile     string    `json:"last_file"`
	LastFrame    uint64    `json:"last_frame"`
	LastCommitAt time.Time `json:"last_commit_at"`
	LastSendAt   time.Time `json:"last_send_at"`
}

func stateFile(dir string) string { return filepath.Join(dir, "agent-status.json") }

func loadState(dir string) (state, error) {
	b, err := os.ReadFile(stateFile(dir))
	if err != nil {
		return state{}, err
	}
	var st state
	if err := json.Unmarshal(b, &st); err != nil {
		return state{}, err
	}
	return st, nil
}

func saveState(dir string, st state) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := stateFile(dir) + ".tmp"
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, stateFile(dir))
}

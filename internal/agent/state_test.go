package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()

	expected := state{IdxPath: "/tmp/new.idx"}

	writeJSON := func(path string, st state) {
		b, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	path := stateFile(dir)
	writeJSON(path, expected)

	st, err := loadState(dir)
	if err != nil {
		t.Fatalf("loadState returned error: %v", err)
	}
	if filepath.Clean(path) != filepath.Join(dir, "status.json") {
		t.Fatalf("expected state file %s/status.json, got %s", dir, path)
	}
	if st.IdxPath != expected.IdxPath {
		t.Fatalf("expected idx path %s, got %s", expected.IdxPath, st.IdxPath)
	}
}

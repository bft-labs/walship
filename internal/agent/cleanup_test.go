package agent

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWalCleanup_RemovesOldestUntilLowWatermark(t *testing.T) {
	tmp := t.TempDir()
	walDir := filepath.Join(tmp, "wal")
	if err := os.MkdirAll(walDir, 0o755); err != nil {
		t.Fatal(err)
	}

	restore := patchCleanupThresholds(300, 150)
	t.Cleanup(restore)

	dayA := filepath.Join(walDir, "2025-12-05")
	dayB := filepath.Join(walDir, "2025-12-06")
	if err := os.MkdirAll(dayA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dayB, 0o755); err != nil {
		t.Fatal(err)
	}

	// Sizes chosen so that removing two oldest segments crosses the low watermark.
	createSegment(t, dayA, "seg-000001", 120, 10)
	createSegment(t, dayA, "seg-000002", 120, 10)
	createSegment(t, dayB, "seg-000001", 120, 10)

	walCleanupOnce(context.Background(), walDir, walDir)

	if pathExists(filepath.Join(dayA, "seg-000001.wal.gz")) || pathExists(filepath.Join(dayA, "seg-000001.wal.idx")) {
		t.Fatalf("expected oldest segment in %s to be removed", dayA)
	}
	if pathExists(filepath.Join(dayA, "seg-000002.wal.gz")) || pathExists(filepath.Join(dayA, "seg-000002.wal.idx")) {
		t.Fatalf("expected second-oldest segment in %s to be removed", dayA)
	}
	if !pathExists(filepath.Join(dayB, "seg-000001.wal.gz")) || !pathExists(filepath.Join(dayB, "seg-000001.wal.idx")) {
		t.Fatalf("expected newest segment in %s to remain", dayB)
	}

	total, err := walDirSize(walDir)
	if err != nil {
		t.Fatal(err)
	}
	if total > walCleanupLowWatermark {
		t.Fatalf("expected wal dir size <= %d, got %d", walCleanupLowWatermark, total)
	}
}

func TestWalCleanup_RespectsSegmentOrderWithinDir(t *testing.T) {
	tmp := t.TempDir()

	restore := patchCleanupThresholds(150, 90)
	t.Cleanup(restore)

	createSegment(t, tmp, "seg-000001", 120, 0)
	createSegment(t, tmp, "seg-000002", 40, 10)

	walCleanupOnce(context.Background(), tmp, tmp)

	if pathExists(filepath.Join(tmp, "seg-000001.wal.gz")) || pathExists(filepath.Join(tmp, "seg-000001.wal.idx")) {
		t.Fatalf("expected seg-000001 to be removed first")
	}
	if !pathExists(filepath.Join(tmp, "seg-000002.wal.gz")) {
		t.Fatalf("expected seg-000002 to remain")
	}
}

func TestWalCleanup_SkipsActiveDay(t *testing.T) {
	tmp := t.TempDir()
	walDir := filepath.Join(tmp, "wal")
	if err := os.MkdirAll(walDir, 0o755); err != nil {
		t.Fatal(err)
	}

	restore := patchCleanupThresholds(200, 100)
	t.Cleanup(restore)

	dayA := filepath.Join(walDir, "2025-12-15")
	dayB := filepath.Join(walDir, "2025-12-16")
	dayC := filepath.Join(walDir, "2025-12-17")

	createSegment(t, dayA, "seg-000001", 80, 10)
	createSegment(t, dayA, "seg-000002", 80, 10)
	createSegment(t, dayB, "seg-000001", 120, 10)
	createSegment(t, dayC, "seg-000001", 50, 10)

	// Protect dayB
	st := state{IdxPath: filepath.Join(dayB, "seg-000001.wal.idx")}
	if err := saveState(walDir, st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	walCleanupOnce(context.Background(), walDir, walDir)

	// Oldest day should be pruned
	if pathExists(filepath.Join(dayA, "seg-000001.wal.gz")) || pathExists(filepath.Join(dayA, "seg-000002.wal.gz")) {
		t.Fatalf("expected dayA segments to be removed")
	}

	// Active day should remain untouched
	if !pathExists(filepath.Join(dayB, "seg-000001.wal.gz")) {
		t.Fatalf("expected active day segment to remain")
	}

	// Newer days should also remain untouched
	if !pathExists(filepath.Join(dayC, "seg-000001.wal.gz")) {
		t.Fatalf("expected newer day segment to remain")
	}
}

func createSegment(t *testing.T, dir, base string, gzSize, idxSize int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	gzPath := filepath.Join(dir, base+".wal.gz")
	if err := os.WriteFile(gzPath, bytes.Repeat([]byte{0}, gzSize), 0o644); err != nil {
		t.Fatalf("write gz: %v", err)
	}
	idxPath := filepath.Join(dir, base+".wal.idx")
	if err := os.WriteFile(idxPath, bytes.Repeat([]byte{1}, idxSize), 0o644); err != nil {
		t.Fatalf("write idx: %v", err)
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func patchCleanupThresholds(high, low int64) func() {
	prevHigh := walCleanupHighWatermark
	prevLow := walCleanupLowWatermark
	prevInterval := walCleanupCheckInterval
	prevNow := walCleanupTickerNow
	walCleanupHighWatermark = high
	walCleanupLowWatermark = low
	walCleanupCheckInterval = time.Millisecond
	walCleanupTickerNow = true
	return func() {
		walCleanupHighWatermark = prevHigh
		walCleanupLowWatermark = prevLow
		walCleanupCheckInterval = prevInterval
		walCleanupTickerNow = prevNow
	}
}

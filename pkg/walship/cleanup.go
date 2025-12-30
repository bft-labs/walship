package walship

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bft-labs/walship/internal/ports"
)

// CleanupConfig holds configuration options for automatic WAL cleanup.
// When enabled, walship periodically checks the WAL directory size and
// removes old segments when it exceeds the high watermark.
type CleanupConfig struct {
	// Enabled controls whether cleanup is active. Default: false
	Enabled bool

	// CheckInterval is how often to check the WAL directory size.
	// Default: 72 hours
	CheckInterval time.Duration

	// HighWatermark is the size in bytes above which cleanup begins.
	// Default: 2 GiB (2147483648 bytes)
	HighWatermark int64

	// LowWatermark is the target size in bytes after cleanup.
	// Default: 1.5 GiB (1610612736 bytes)
	LowWatermark int64
}

// DefaultCleanupConfig returns a CleanupConfig with sensible defaults.
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		Enabled:       true,
		CheckInterval: 72 * time.Hour,
		HighWatermark: 2 << 30,  // 2 GiB
		LowWatermark:  3 << 29,  // 1.5 GiB
	}
}

// WithCleanupConfig enables automatic WAL cleanup with the specified configuration.
// When enabled, walship periodically checks the WAL directory size and removes
// old segments to prevent unbounded disk usage.
//
// Usage:
//
//	w, err := walship.New(cfg,
//	    walship.WithCleanupConfig(walship.CleanupConfig{
//	        Enabled:       true,
//	        HighWatermark: 10 << 30, // 10GB
//	        LowWatermark:  5 << 30,  // 5GB
//	        CheckInterval: 1 * time.Hour,
//	    }),
//	)
func WithCleanupConfig(cfg CleanupConfig) Option {
	if !cfg.Enabled {
		return func(o *options) {} // No-op if not enabled
	}

	// Apply defaults for zero values
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 72 * time.Hour
	}
	if cfg.HighWatermark <= 0 {
		cfg.HighWatermark = 2 << 30
	}
	if cfg.LowWatermark <= 0 {
		cfg.LowWatermark = 3 << 29
	}

	return func(o *options) {
		o.cleanupConfig = &cfg
	}
}

// cleanupRunner manages the WAL cleanup goroutine.
type cleanupRunner struct {
	mu sync.RWMutex

	// Configuration
	checkInterval time.Duration
	highWatermark int64
	lowWatermark  int64

	// Runtime state
	walDir   string
	stateDir string
	logger   ports.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newCleanupRunner(cfg CleanupConfig, walDir, stateDir string, logger ports.Logger) *cleanupRunner {
	return &cleanupRunner{
		checkInterval: cfg.CheckInterval,
		highWatermark: cfg.HighWatermark,
		lowWatermark:  cfg.LowWatermark,
		walDir:        walDir,
		stateDir:      stateDir,
		logger:        logger,
	}
}

func (c *cleanupRunner) start(ctx context.Context) {
	if c.walDir == "" {
		c.logger.Warn("WAL cleanup disabled: no WAL directory configured")
		return
	}

	cleanupCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.logger.Info("WAL cleanup enabled")

	c.wg.Add(1)
	go c.cleanupLoop(cleanupCtx)
}

func (c *cleanupRunner) stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
}

func (c *cleanupRunner) cleanupLoop(ctx context.Context) {
	defer c.wg.Done()

	// Run immediately on startup
	c.cleanupOnce(ctx)

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanupOnce(ctx)
		}
	}
}

func (c *cleanupRunner) cleanupOnce(ctx context.Context) {
	c.mu.RLock()
	walDir := c.walDir
	stateDir := c.stateDir
	c.mu.RUnlock()

	curSize, err := walDirSize(walDir)
	if err != nil {
		c.logger.Error("WAL cleanup: size check failed", ports.Err(err))
		return
	}

	if curSize <= c.highWatermark {
		return
	}

	protectedDay := c.currentActiveDay(stateDir)

	segs, err := orderedSegments(walDir, protectedDay)
	if err != nil {
		c.logger.Error("WAL cleanup: list segments failed", ports.Err(err))
		return
	}
	if len(segs) == 0 {
		return
	}

	removed := int64(0)
	for _, seg := range segs {
		if ctx.Err() != nil {
			return
		}
		if curSize <= c.lowWatermark {
			break
		}

		bytesFreed, rmErr := removeSegment(seg)
		if rmErr != nil {
			c.logger.Error("WAL cleanup: remove failed", ports.Err(rmErr))
			continue
		}
		curSize -= bytesFreed
		removed += bytesFreed
	}

	if removed > 0 {
		c.logger.Info("WAL cleanup completed", ports.Int64("bytes_freed", removed))
	}
}

func (c *cleanupRunner) currentActiveDay(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	st, err := c.loadState(stateDir)
	if err != nil || st.IdxPath == "" {
		return ""
	}
	day := filepath.Base(filepath.Dir(st.IdxPath))
	if isDayDir(day) {
		return day
	}
	return ""
}

type cleanupStateFile struct {
	IdxPath string `json:"idx_path"`
}

func (c *cleanupRunner) loadState(stateDir string) (cleanupStateFile, error) {
	path := filepath.Join(stateDir, "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cleanupStateFile{}, err
	}
	var st cleanupStateFile
	if err := json.Unmarshal(data, &st); err != nil {
		return cleanupStateFile{}, err
	}
	return st, nil
}

// walSegment represents a WAL segment pair (gz + idx).
type walSegment struct {
	day     string
	gzPath  string
	idxPath string
	gzSize  int64
	idxSize int64
}

func walDirSize(walDir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(walDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func orderedSegments(walDir, skipFromDay string) ([]walSegment, error) {
	dayDirs, err := dayDirectories(walDir)
	if err != nil {
		return nil, err
	}

	var segs []walSegment

	// Include any top-level segments first
	top, err := scanSegmentDir(walDir, "")
	if err != nil {
		return nil, err
	}
	segs = append(segs, top...)

	for _, day := range dayDirs {
		if skipFromDay != "" && day >= skipFromDay {
			continue
		}
		dayPath := filepath.Join(walDir, day)
		daySegs, err := scanSegmentDir(dayPath, day)
		if err != nil {
			return nil, err
		}
		segs = append(segs, daySegs...)
	}

	return segs, nil
}

func dayDirectories(walDir string) ([]string, error) {
	ents, err := os.ReadDir(walDir)
	if err != nil {
		return nil, err
	}
	var days []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if isDayDir(e.Name()) {
			days = append(days, e.Name())
		}
	}
	sort.Strings(days)
	return days, nil
}

func isDayDir(name string) bool {
	return len(name) == len("2006-01-02") && strings.Count(name, "-") == 2
}

func scanSegmentDir(dir, day string) ([]walSegment, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	byNum := map[int]*walSegment{}
	for _, e := range ents {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".wal.gz"):
			num, ok := segmentNumber(name, ".wal.gz")
			if !ok {
				continue
			}
			info, err := e.Info()
			if err != nil {
				return nil, err
			}
			seg := getSegment(byNum, num)
			seg.day = day
			seg.gzPath = filepath.Join(dir, name)
			seg.gzSize = info.Size()
		case strings.HasSuffix(name, ".wal.idx"):
			num, ok := segmentNumber(name, ".wal.idx")
			if !ok {
				continue
			}
			info, err := e.Info()
			if err != nil {
				return nil, err
			}
			seg := getSegment(byNum, num)
			seg.day = day
			seg.idxPath = filepath.Join(dir, name)
			seg.idxSize = info.Size()
		}
	}

	var numbers []int
	for n, seg := range byNum {
		if seg.gzPath != "" {
			numbers = append(numbers, n)
		}
	}
	sort.Ints(numbers)

	out := make([]walSegment, 0, len(numbers))
	for _, n := range numbers {
		seg := byNum[n]
		if seg.gzPath != "" {
			out = append(out, *seg)
		}
	}
	return out, nil
}

func getSegment(m map[int]*walSegment, num int) *walSegment {
	if seg, ok := m[num]; ok {
		return seg
	}
	seg := &walSegment{}
	m[num] = seg
	return seg
}

func segmentNumber(name, suffix string) (int, bool) {
	if !strings.HasPrefix(name, "seg-") || !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	numStr := strings.TrimPrefix(name, "seg-")
	numStr = strings.TrimSuffix(numStr, suffix)
	if len(numStr) != 6 {
		return 0, false
	}
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, false
	}
	return n, true
}

func removeSegment(seg walSegment) (int64, error) {
	if seg.gzPath == "" {
		return 0, errors.New("missing gzip path for segment")
	}
	if err := os.Remove(seg.gzPath); err != nil {
		return 0, err
	}

	bytesFreed := seg.gzSize
	if seg.idxPath != "" {
		if err := os.Remove(seg.idxPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return bytesFreed, err
		}
		bytesFreed += seg.idxSize
	}
	return bytesFreed, nil
}

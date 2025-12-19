// Package walcleanup provides automatic WAL file cleanup for walship.
// When enabled, it periodically removes old WAL segments to prevent
// unbounded disk usage.
package walcleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bft-labs/walship/pkg/walship"
)

// Plugin implements WAL cleanup functionality.
// It periodically checks the WAL directory size and removes old segments
// when it exceeds the high watermark.
type Plugin struct {
	mu sync.RWMutex

	// Configuration
	checkInterval time.Duration
	highWatermark int64
	lowWatermark  int64

	// Runtime state
	walDir   string
	stateDir string
	logger   walship.Logger
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// Config holds configuration options for the WAL cleanup plugin.
type Config struct {
	// CheckInterval is how often to check the WAL directory size.
	// Default: 72 hours
	CheckInterval time.Duration

	// HighWatermark is the size in bytes above which cleanup begins.
	// Default: 2 GiB
	HighWatermark int64

	// LowWatermark is the target size in bytes after cleanup.
	// Default: 1.5 GiB
	LowWatermark int64

	// RunImmediately if true, runs a cleanup check on startup.
	// Default: true
	RunImmediately bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CheckInterval:  72 * time.Hour,
		HighWatermark:  2 << 30,  // 2 GiB
		LowWatermark:   3 << 29,  // 1.5 GiB
		RunImmediately: true,
	}
}

// New creates a new WAL cleanup plugin with the given configuration.
func New(cfg Config) *Plugin {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 72 * time.Hour
	}
	if cfg.HighWatermark <= 0 {
		cfg.HighWatermark = 2 << 30
	}
	if cfg.LowWatermark <= 0 {
		cfg.LowWatermark = 3 << 29
	}

	return &Plugin{
		checkInterval: cfg.CheckInterval,
		highWatermark: cfg.HighWatermark,
		lowWatermark:  cfg.LowWatermark,
	}
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string {
	return "walcleanup"
}

// Initialize sets up the plugin and starts the cleanup loop.
func (p *Plugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	p.mu.Lock()
	p.walDir = cfg.WALDir
	p.stateDir = cfg.StateDir
	p.logger = cfg.Logger
	p.mu.Unlock()

	if p.walDir == "" {
		p.logger.Warn("WAL cleanup disabled: no WAL directory configured")
		return nil
	}

	// Create cancellable context for the cleanup loop
	cleanupCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	p.logger.Info("WAL cleanup plugin initialized")

	// Start cleanup loop
	p.wg.Add(1)
	go p.cleanupLoop(cleanupCtx)

	return nil
}

// Shutdown stops the cleanup loop.
func (p *Plugin) Shutdown(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}

// cleanupLoop runs periodic cleanup checks.
func (p *Plugin) cleanupLoop(ctx context.Context) {
	defer p.wg.Done()

	// Run immediately on startup
	p.cleanupOnce(ctx)

	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.cleanupOnce(ctx)
		}
	}
}

// cleanupOnce performs a single cleanup check.
func (p *Plugin) cleanupOnce(ctx context.Context) {
	p.mu.RLock()
	walDir := p.walDir
	stateDir := p.stateDir
	p.mu.RUnlock()

	curSize, err := walDirSize(walDir)
	if err != nil {
		p.logger.Error("WAL cleanup: size check failed")
		return
	}

	if curSize <= p.highWatermark {
		return
	}

	protectedDay := p.currentActiveDay(stateDir)

	segs, err := orderedSegments(walDir, protectedDay)
	if err != nil {
		p.logger.Error("WAL cleanup: list segments failed")
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
		if curSize <= p.lowWatermark {
			break
		}

		bytesFreed, rmErr := removeSegment(seg)
		if rmErr != nil {
			p.logger.Error("WAL cleanup: remove failed")
			continue
		}
		curSize -= bytesFreed
		removed += bytesFreed
	}

	if removed > 0 {
		p.logger.Info("WAL cleanup completed")
	}
}

// currentActiveDay returns the day directory that should not be cleaned.
func (p *Plugin) currentActiveDay(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	st, err := p.loadState(stateDir)
	if err != nil || st.IdxPath == "" {
		return ""
	}
	day := filepath.Base(filepath.Dir(st.IdxPath))
	if isDayDir(day) {
		return day
	}
	return ""
}

// stateFile represents the persisted state structure.
type stateFile struct {
	IdxPath string `json:"idx_path"`
}

func (p *Plugin) loadState(stateDir string) (stateFile, error) {
	path := filepath.Join(stateDir, "status.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return stateFile{}, err
	}
	var st stateFile
	if err := json.Unmarshal(data, &st); err != nil {
		return stateFile{}, err
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

func formatBytes(b int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
	)

	fb := float64(b)
	switch {
	case fb >= GB:
		return fmt.Sprintf("%.2fGiB", fb/GB)
	case fb >= MB:
		return fmt.Sprintf("%.2fMiB", fb/MB)
	case fb >= KB:
		return fmt.Sprintf("%.2fKiB", fb/KB)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// Ensure Plugin implements walship.Plugin.
var _ walship.Plugin = (*Plugin)(nil)

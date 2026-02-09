package agent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	walCleanupCheckInterval   = 72 * time.Hour
	walCleanupHighWatermark   = int64(2 << 30) // 2GiB
	walCleanupLowWatermark    = int64(3 << 29) // 1.5GiB
	walCleanupTickerNow       = true           // run once immediately; used for tests
	traceCleanupHighWatermark = int64(1 << 30) // 1GiB
	traceCleanupLowWatermark  = int64(1 << 29) // 512MiB
)

type walSegment struct {
	day     string
	gzPath  string
	idxPath string
	gzSize  int64
	idxSize int64
}

// cleanupLoop runs a periodic cleanup that trims old WAL segments and trace files
// when they grow beyond their respective high watermarks.
func cleanupLoop(ctx context.Context, walDir, stateDir, traceDir string) {
	if walDir == "" {
		return
	}

	if walCleanupTickerNow {
		walCleanupOnce(ctx, walDir, stateDir)
		traceCleanupOnce(traceDir)
	}

	t := time.NewTicker(walCleanupCheckInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			walCleanupOnce(ctx, walDir, stateDir)
			traceCleanupOnce(traceDir)
		}
	}
}

func walCleanupOnce(ctx context.Context, walDir, stateDir string) {
	curSize, err := walDirSize(walDir)
	if err != nil {
		logger.Error().Err(err).Msg("wal cleanup: size check failed")
		return
	}
	if curSize <= walCleanupHighWatermark {
		return
	}

	protectedDay := currentActiveDay(stateDir)

	segs, err := orderedSegments(walDir, protectedDay)
	if err != nil {
		logger.Error().Err(err).Msg("wal cleanup: list segments failed")
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
		if curSize <= walCleanupLowWatermark {
			break
		}

		bytesFreed, rmErr := removeSegment(seg)
		if rmErr != nil {
			logger.Error().Err(rmErr).Str("segment", seg.gzPath).Msg("wal cleanup: remove failed")
			continue
		}
		curSize -= bytesFreed
		removed += bytesFreed
	}

	if removed > 0 {
		logger.Info().
			Str("freed", formatBytes(removed)).
			Str("remaining", formatBytes(curSize)).
			Msg("wal cleanup completed")
	}
}

func traceCleanupOnce(traceDir string) {
	if traceDir == "" {
		return
	}

	store := NewTraceStore(traceDir)

	curSize, err := store.Size()
	if err != nil {
		logger.Error().Err(err).Msg("trace cleanup: size check failed")
		return
	}

	if curSize <= traceCleanupHighWatermark {
		return
	}

	entries, err := os.ReadDir(traceDir)
	if err != nil {
		logger.Error().Err(err).Msg("trace cleanup: list files failed")
		return
	}

	type rangeFile struct {
		name string
		minH int64
	}
	var files []rangeFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		minH, _, ok := parseRangeFileName(entry.Name())
		if !ok {
			continue
		}
		files = append(files, rangeFile{name: entry.Name(), minH: minH})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].minH < files[j].minH })

	removed := int64(0)
	for _, f := range files {
		if curSize <= traceCleanupLowWatermark {
			break
		}

		path := filepath.Join(traceDir, f.name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if err := os.Remove(path); err != nil {
			logger.Error().Err(err).Str("file", f.name).Msg("trace cleanup: remove failed")
			continue
		}

		bytesFreed := info.Size()
		curSize -= bytesFreed
		removed += bytesFreed
	}

	if removed > 0 {
		logger.Info().
			Str("freed", formatBytes(removed)).
			Str("remaining", formatBytes(curSize)).
			Msg("trace cleanup completed")
	}
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
	if err != nil {
		return 0, err
	}
	return total, nil
}

func orderedSegments(walDir, skipFromDay string) ([]walSegment, error) {
	dayDirs, err := dayDirectories(walDir)
	if err != nil {
		return nil, err
	}

	var segs []walSegment

	// Include any top-level segments first.
	top, err := scanSegmentDir(walDir, "")
	if err != nil {
		return nil, err
	}
	segs = append(segs, top...)

	for _, day := range dayDirs {
		if skipFromDay != "" && day >= skipFromDay {
			logger.Info().Str("protectedDayFrom", skipFromDay).Str("targetDay", day).Msg("wal cleanup: skipping active day and newer")
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

func currentActiveDay(stateDir string) string {
	if stateDir == "" {
		return ""
	}
	st, err := loadState(stateDir)
	if err != nil || st.IdxPath == "" {
		return ""
	}
	day := filepath.Base(filepath.Dir(st.IdxPath))
	if isDayDir(day) {
		return day
	}
	return ""
}

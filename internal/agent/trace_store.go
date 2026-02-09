package agent

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// traceRange represents a range file's height bounds.
type traceRange struct {
	filename string
	minH     int64
	maxH     int64
}

// TraceStore reads range-based trace log files written by cosmos-sdk.
// Files are named trace-{minHeight}-{maxHeight}.gz.
type TraceStore struct {
	dir string
	mu  sync.RWMutex
}

// NewTraceStore creates a new TraceStore for the given directory.
func NewTraceStore(dir string) *TraceStore {
	return &TraceStore{dir: dir}
}

// parseRangeFileName extracts min and max height from a range filename.
// Format: trace-{minHeight}-{maxHeight}.gz
func parseRangeFileName(name string) (minH, maxH int64, ok bool) {
	if !strings.HasPrefix(name, "trace-") || !strings.HasSuffix(name, ".gz") {
		return 0, 0, false
	}
	numStr := strings.TrimPrefix(name, "trace-")
	numStr = strings.TrimSuffix(numStr, ".gz")

	parts := strings.Split(numStr, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}

	minH, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	maxH, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return minH, maxH, true
}

// listRanges returns all range files in the directory.
func (s *TraceStore) listRanges() ([]traceRange, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ranges []traceRange
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		minH, maxH, ok := parseRangeFileName(entry.Name())
		if !ok {
			continue
		}
		ranges = append(ranges, traceRange{
			filename: entry.Name(),
			minH:     minH,
			maxH:     maxH,
		})
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].minH < ranges[j].minH })
	return ranges, nil
}

// findRangeForHeight finds the range file that contains the given height.
func (s *TraceStore) findRangeForHeight(height int64) (*traceRange, error) {
	ranges, err := s.listRanges()
	if err != nil {
		return nil, err
	}

	for _, r := range ranges {
		if height >= r.minH && height <= r.maxH {
			return &r, nil
		}
	}
	return nil, nil
}

var heightKey = []byte(`"height":`)

// extractHeight extracts height from a StoreTraceSet line without full JSON parsing.
// Format: {"_msg":"store trace set","height":150,"count":10,"traces":[...]}
func extractHeight(line []byte) int64 {
	idx := bytes.Index(line, heightKey)
	if idx == -1 {
		return 0
	}

	start := idx + len(heightKey)
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}

	end := start
	for end < len(line) && line[end] >= '0' && line[end] <= '9' {
		end++
	}

	if start == end {
		return 0
	}

	height, _ := strconv.ParseInt(string(line[start:end]), 10, 64)
	return height
}

// Load reads trace data for a specific height, filtering from the range file.
func (s *TraceStore) Load(height int64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, err := s.findRangeForHeight(height)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}

	path := filepath.Join(s.dir, r.filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines, err := s.decompressLines(data)
	if err != nil {
		return nil, err
	}

	var filtered [][]byte
	for _, line := range lines {
		if extractHeight(line) == height {
			filtered = append(filtered, line)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	return CompressLines(filtered)
}

// LoadMultiple loads trace data for multiple heights and returns combined compressed data.
func (s *TraceStore) LoadMultiple(heights []int64) ([]byte, []FrameMeta, error) {
	if len(heights) == 0 {
		return nil, nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	heightSet := make(map[int64]bool)
	for _, h := range heights {
		heightSet[h] = true
	}

	ranges, err := s.listRanges()
	if err != nil {
		return nil, nil, err
	}

	var allLines [][]byte
	var metas []FrameMeta
	heightLines := make(map[int64][][]byte)

	for _, r := range ranges {
		hasRelevant := false
		for _, h := range heights {
			if h >= r.minH && h <= r.maxH {
				hasRelevant = true
				break
			}
		}
		if !hasRelevant {
			continue
		}

		path := filepath.Join(s.dir, r.filename)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}

		lines, err := s.decompressLines(data)
		if err != nil {
			logger.Warn().Err(err).Str("file", r.filename).Msg("failed to decompress trace")
			continue
		}

		for _, line := range lines {
			h := extractHeight(line)
			if heightSet[h] {
				heightLines[h] = append(heightLines[h], line)
			}
		}
	}

	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	for _, h := range heights {
		lines := heightLines[h]
		if len(lines) == 0 {
			continue
		}
		allLines = append(allLines, lines...)
		metas = append(metas, FrameMeta{
			File:  fmt.Sprintf("trace-%d.gz", h),
			Frame: uint64(h),
			Recs:  uint32(len(lines)),
		})
	}

	if len(allLines) == 0 {
		return nil, nil, nil
	}

	compressed, err := CompressLines(allLines)
	if err != nil {
		return nil, nil, err
	}

	if len(metas) > 0 {
		metas[0].Len = uint64(len(compressed))
	}

	return compressed, metas, nil
}

// DeleteBefore removes all trace files where maxHeight < given height.
func (s *TraceStore) DeleteBefore(height int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ranges, err := s.listRanges()
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, r := range ranges {
		if r.maxH < height {
			path := filepath.Join(s.dir, r.filename)
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// ListHeights returns all heights covered by stored trace files.
func (s *TraceStore) ListHeights() ([]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ranges, err := s.listRanges()
	if err != nil {
		return nil, err
	}

	var heights []int64
	for _, r := range ranges {
		for h := r.minH; h <= r.maxH; h++ {
			heights = append(heights, h)
		}
	}

	return heights, nil
}

// Size returns the total size of all trace files in bytes.
func (s *TraceStore) Size() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int64
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, _, ok := parseRangeFileName(entry.Name()); ok {
			info, err := entry.Info()
			if err == nil {
				total += info.Size()
			}
		}
	}

	return total, nil
}

// decompressLines reads gzip data and returns individual lines.
func (s *TraceStore) decompressLines(data []byte) ([][]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	content, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	var lines [][]byte
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// Exists checks if trace data exists for a specific height.
func (s *TraceStore) Exists(height int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, err := s.findRangeForHeight(height)
	return err == nil && r != nil
}

// Count returns the number of stored trace files.
func (s *TraceStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ranges, err := s.listRanges()
	if err != nil {
		return 0, err
	}
	return len(ranges), nil
}

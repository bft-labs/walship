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

// TraceStore manages height-indexed trace log files.
type TraceStore struct {
	dir string
	mu  sync.RWMutex
}

// NewTraceStore creates a new TraceStore with the given directory.
func NewTraceStore(dir string) (*TraceStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create trace dir: %w", err)
	}
	return &TraceStore{dir: dir}, nil
}

// traceFileName returns the filename for a given height.
func traceFileName(height int64) string {
	return fmt.Sprintf("trace-%09d.wal.gz", height)
}

// parseTraceFileName extracts height from a trace filename.
func parseTraceFileName(name string) (int64, bool) {
	if !strings.HasPrefix(name, "trace-") || !strings.HasSuffix(name, ".wal.gz") {
		return 0, false
	}
	numStr := strings.TrimPrefix(name, "trace-")
	numStr = strings.TrimSuffix(numStr, ".wal.gz")
	height, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, false
	}
	return height, true
}

// Save stores trace lines for a specific height.
// If trace data already exists for this height, it appends to it.
func (s *TraceStore) Save(height int64, lines [][]byte) error {
	if len(lines) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, traceFileName(height))

	var existingLines [][]byte
	if data, err := os.ReadFile(path); err == nil {
		existingLines, _ = s.decompressLines(data)
	}

	allLines := append(existingLines, lines...)

	compressed, err := CompressLines(allLines)
	if err != nil {
		return fmt.Errorf("compress trace: %w", err)
	}

	if err := os.WriteFile(path, compressed, 0o644); err != nil {
		return fmt.Errorf("write trace file: %w", err)
	}

	return nil
}

// Load reads trace data for a specific height.
func (s *TraceStore) Load(height int64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, traceFileName(height))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// LoadMultiple loads trace data for multiple heights and returns combined compressed data.
func (s *TraceStore) LoadMultiple(heights []int64) ([]byte, []FrameMeta, error) {
	if len(heights) == 0 {
		return nil, nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var allLines [][]byte
	var metas []FrameMeta

	for _, height := range heights {
		path := filepath.Join(s.dir, traceFileName(height))
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}

		lines, err := s.decompressLines(data)
		if err != nil {
			logger.Warn().Err(err).Int64("height", height).Msg("failed to decompress trace")
			continue
		}

		allLines = append(allLines, lines...)
		metas = append(metas, FrameMeta{
			File:  traceFileName(height),
			Frame: uint64(height),
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

// Delete removes trace files for the given heights.
func (s *TraceStore) Delete(heights []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error
	for _, height := range heights {
		path := filepath.Join(s.dir, traceFileName(height))
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to delete %d trace files", len(errs))
	}
	return nil
}

// DeleteBefore removes all trace files for heights less than the given height.
func (s *TraceStore) DeleteBefore(height int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		h, ok := parseTraceFileName(entry.Name())
		if !ok {
			continue
		}
		if h < height {
			path := filepath.Join(s.dir, entry.Name())
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// ListHeights returns all heights that have stored trace data.
func (s *TraceStore) ListHeights() ([]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var heights []int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if h, ok := parseTraceFileName(entry.Name()); ok {
			heights = append(heights, h)
		}
	}

	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
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
		if _, ok := parseTraceFileName(entry.Name()); ok {
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

	path := filepath.Join(s.dir, traceFileName(height))
	_, err := os.Stat(path)
	return err == nil
}

// Count returns the number of stored trace files.
func (s *TraceStore) Count() (int, error) {
	heights, err := s.ListHeights()
	if err != nil {
		return 0, err
	}
	return len(heights), nil
}

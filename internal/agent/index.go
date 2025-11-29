package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// openIdx opens the index file and returns the file and a buffered reader.
func openIdx(idxPath string) (*os.File, *bufio.Reader, error) {
	f, err := os.Open(idxPath)
	if err != nil {
		return nil, nil, err
	}
	return f, bufio.NewReaderSize(f, 64*1024), nil
}

// openGz opens the given gzip file path (not a gzip.Reader; we range-read compressed bytes).
func openGz(path string) (*os.File, error) { return os.Open(path) }

// nextFrame reads next complete JSON line and returns FrameMeta and raw line bytes.
func nextFrame(r *bufio.Reader) (FrameMeta, []byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return FrameMeta{}, nil, err
	}
	var fm FrameMeta
	if err := json.Unmarshal(line, &fm); err != nil {
		return FrameMeta{}, line, fmt.Errorf("bad index line: %w", err)
	}
	return fm, line, nil
}

// preadSection reads [off, off+len) bytes from file.
func preadSection(f *os.File, off int64, length int64) ([]byte, error) {
	if f == nil {
		return nil, errors.New("nil file")
	}
	sr := io.NewSectionReader(f, off, length)
	buf := make([]byte, length)
	_, err := io.ReadFull(sr, buf)
	return buf, err
}

// latestIndex discovers the newest day directory (YYYY-MM-DD) under dir, then
// returns the lexicographically newest .wal.idx inside that day. If no day
// directories exist, it falls back to picking the newest .idx directly under dir.
func latestIndex(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	// First pass: look for day directories
	latestDay := ""
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// quick check: format YYYY-MM-DD (length 10 with two dashes)
		if len(name) == len("2006-01-02") && strings.Count(name, "-") == 2 {
			if name > latestDay {
				latestDay = name
			}
		}
	}
	if latestDay != "" {
		// Scan inside latest day directory for newest .wal.idx
		dayDir := filepath.Join(dir, latestDay)
		dayEnts, err := os.ReadDir(dayDir)
		if err != nil {
			return "", err
		}
		latest := ""
		for _, de := range dayEnts {
			n := de.Name()
			if strings.HasSuffix(n, ".wal.idx") && n > latest {
				latest = n
			}
		}
		if latest == "" {
			return "", fmt.Errorf("no index files in %s", dayDir)
		}
		return filepath.Join(dayDir, latest), nil
	}
	// Fallback: no day dirs; look directly under dir
	var latest string
	for _, e := range ents {
		n := e.Name()
		if (strings.HasSuffix(n, ".wal.idx") || strings.HasSuffix(n, ".idx")) && n > latest {
			latest = n
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no index files in %s", dir)
	}
	return filepath.Join(dir, latest), nil
}

// oldestIndex mirrors latestIndex but picks the earliest day and the
// lexicographically smallest index within that day. Falls back to dir
// directly if no day dirs are present.
func oldestIndex(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("%w\n\nPlease verify:\n  - The --wal-dir flag points to the correct directory\n  - The directory exists\n  - You have permission to read the directory", err)
	}
	// Find earliest day
	earliestDay := "~" // larger than any valid day; we will pick smaller
	hasDay := false
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) == len("2006-01-02") && strings.Count(name, "-") == 2 {
			hasDay = true
			if name < earliestDay {
				earliestDay = name
			}
		}
	}
	if hasDay {
		dayDir := filepath.Join(dir, earliestDay)
		dayEnts, err := os.ReadDir(dayDir)
		if err != nil {
			return "", err
		}
		// pick smallest seg index by lexicographic name
		oldest := "~"
		for _, de := range dayEnts {
			n := de.Name()
			if strings.HasSuffix(n, ".wal.idx") && n < oldest {
				oldest = n
			}
		}
		if oldest == "~" {
			return "", fmt.Errorf("no index files in %s", dayDir)
		}
		return filepath.Join(dayDir, oldest), nil
	}
	// No day dirs; pick smallest idx directly under dir
	oldest := "~"
	for _, e := range ents {
		n := e.Name()
		if (strings.HasSuffix(n, ".wal.idx") || strings.HasSuffix(n, ".idx")) && n < oldest {
			oldest = n
		}
	}
	if oldest == "~" {
		return "", fmt.Errorf("no index files found in %q\n\nPlease verify:\n  - The --wal-dir directory contains .idx files", dir)
	}
	return filepath.Join(dir, oldest), nil
}

// nextIndexAfter returns the next index path after the given current index.
// It looks for the next segment within the same day; if not present, advances
// to the next day directory and selects the first segment there. If nothing
// newer exists yet, returns ("", false, nil).
func nextIndexAfter(curIdxPath string) (string, bool, error) {
	dayDir := filepath.Dir(curIdxPath)
	base := filepath.Base(curIdxPath)
	// Extract current segment number
	var cur int
	if _, err := fmt.Sscanf(base, "seg-%06d.wal.idx", &cur); err != nil {
		return "", false, fmt.Errorf("unrecognized index name: %s", base)
	}
	// Candidate in same day
	cand := filepath.Join(dayDir, fmt.Sprintf("seg-%06d.wal.idx", cur+1))
	if _, err := os.Stat(cand); err == nil {
		return cand, true, nil
	}
	// Advance to next day directory
	parent := filepath.Dir(dayDir)
	ents, err := os.ReadDir(parent)
	if err != nil {
		return "", false, err
	}
	curDay := filepath.Base(dayDir)
	nextDay := ""
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) == len("2006-01-02") && strings.Count(name, "-") == 2 {
			if name > curDay && (nextDay == "" || name < nextDay) {
				nextDay = name
			}
		}
	}
	if nextDay == "" {
		return "", false, nil
	}
	nd := filepath.Join(parent, nextDay)
	first := filepath.Join(nd, "seg-000001.wal.idx")
	if _, err := os.Stat(first); err == nil {
		return first, true, nil
	}
	// No first segment yet in the new day
	return "", false, nil
}

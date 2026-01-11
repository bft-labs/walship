package wal

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bft-labs/walship/pkg/log"
)

// IndexReader implements Reader by reading WAL index files.
type IndexReader struct {
	walDir  string
	idxFile *os.File
	reader  *bufio.Reader
	gzFile  *os.File
	idxPath string
	idxOff  int64
	curGz   string
	logger  log.Logger
}

// NewIndexReader creates a new IndexReader for the given WAL directory.
func NewIndexReader(walDir string, logger log.Logger) *IndexReader {
	return &IndexReader{
		walDir: walDir,
		logger: logger,
	}
}

// Open prepares the reader starting from the given state.
func (r *IndexReader) Open(ctx context.Context, idxPath string, idxOffset int64, curGz string) error {
	if idxPath == "" {
		// Start from oldest index
		var err error
		idxPath, err = oldestIndex(r.walDir)
		if err != nil {
			return err
		}
		idxOffset = 0
	}

	f, bufReader, err := openIdx(idxPath)
	if err != nil {
		return err
	}

	r.idxFile = f
	r.reader = bufReader
	r.idxPath = idxPath
	r.idxOff = idxOffset

	// Seek to offset if needed
	if idxOffset > 0 {
		if _, err := r.idxFile.Seek(idxOffset, io.SeekStart); err == nil {
			r.reader.Reset(r.idxFile)
		}
	}

	// Open gz file if specified
	if curGz != "" {
		gzPath := filepath.Join(filepath.Dir(idxPath), curGz)
		if gzf, err := os.Open(gzPath); err == nil {
			r.gzFile = gzf
			r.curGz = curGz
		}
	}

	return nil
}

// Next returns the next frame and its compressed data.
func (r *IndexReader) Next(ctx context.Context) (Frame, []byte, int, error) {
	select {
	case <-ctx.Done():
		return Frame{}, nil, 0, ctx.Err()
	default:
	}

	// Read next frame metadata from index
	line, err := r.reader.ReadBytes('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			// Try to advance to next index file
			if next, ok, _ := nextIndexAfter(r.idxPath); ok {
				if err := r.advanceToIndex(next); err != nil {
					return Frame{}, nil, 0, io.EOF
				}
				// Retry read
				return r.Next(ctx)
			}
			return Frame{}, nil, 0, io.EOF
		}
		return Frame{}, nil, 0, err
	}

	var meta FrameMeta
	if err := json.Unmarshal(line, &meta); err != nil {
		return Frame{}, nil, len(line), fmt.Errorf("bad index line: %w", err)
	}

	frame := meta.ToFrame()

	// Ensure gz file is open for this frame
	if r.gzFile == nil || r.curGz != frame.File {
		if r.gzFile != nil {
			r.gzFile.Close()
		}
		gzPath := filepath.Join(filepath.Dir(r.idxPath), frame.File)
		gzf, err := os.Open(gzPath)
		if err != nil {
			return Frame{}, nil, len(line), err
		}
		r.gzFile = gzf
		r.curGz = frame.File
	}

	// Read compressed data
	compressed, err := preadSection(r.gzFile, int64(frame.Offset), int64(frame.Length))
	if err != nil {
		return Frame{}, nil, len(line), err
	}

	// Update offset
	r.idxOff += int64(len(line))

	return frame, compressed, len(line), nil
}

// CurrentPosition returns the current reading position.
func (r *IndexReader) CurrentPosition() (string, int64, string) {
	return r.idxPath, r.idxOff, r.curGz
}

// Close releases all resources.
func (r *IndexReader) Close() error {
	var errs []error
	if r.idxFile != nil {
		if err := r.idxFile.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.gzFile != nil {
		if err := r.gzFile.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// advanceToIndex closes the current index and opens the next one.
func (r *IndexReader) advanceToIndex(nextPath string) error {
	if r.idxFile != nil {
		r.idxFile.Close()
	}
	if r.gzFile != nil {
		r.gzFile.Close()
		r.gzFile = nil
		r.curGz = ""
	}

	f, bufReader, err := openIdx(nextPath)
	if err != nil {
		return err
	}

	r.idxFile = f
	r.reader = bufReader
	r.idxPath = nextPath
	r.idxOff = 0

	return nil
}

// openIdx opens the index file and returns a buffered reader.
func openIdx(idxPath string) (*os.File, *bufio.Reader, error) {
	f, err := os.Open(idxPath)
	if err != nil {
		return nil, nil, err
	}
	return f, bufio.NewReaderSize(f, 64*1024), nil
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

// oldestIndex finds the oldest index file in the WAL directory.
func oldestIndex(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("%w\n\nPlease verify:\n  - The --wal-dir flag points to the correct directory\n  - The directory exists\n  - You have permission to read the directory", err)
	}

	// Find earliest day
	earliestDay := "~"
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
func nextIndexAfter(curIdxPath string) (string, bool, error) {
	dayDir := filepath.Dir(curIdxPath)
	base := filepath.Base(curIdxPath)

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

	return "", false, nil
}

package agent

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
)

const MsgStoreTraceSet = "store trace set"

// traceMarker is the byte pattern to identify trace log lines.
var traceMarker = []byte(`"_msg":"` + MsgStoreTraceSet + `"`)

// heightPrefix is used to extract height value from log lines.
var heightPrefix = []byte(`"height":`)

// extractHeight extracts the height value from a JSON log line.
// Returns 0 if height is not found or invalid.
func extractHeight(line []byte) int64 {
	idx := bytes.Index(line, heightPrefix)
	if idx == -1 {
		return 0
	}

	start := idx + len(heightPrefix)
	if start >= len(line) {
		return 0
	}

	// Find the end of the number (comma, brace, or end of line)
	end := start
	for end < len(line) {
		c := line[end]
		if c < '0' || c > '9' {
			break
		}
		end++
	}

	if start == end {
		return 0
	}

	height, err := strconv.ParseInt(string(line[start:end]), 10, 64)
	if err != nil {
		return 0
	}
	return height
}

// isTraceLine checks if a log line is a trace log using byte matching.
func isTraceLine(line []byte) bool {
	return bytes.Contains(line, traceMarker)
}

// SeparatedLogs contains trace and non-trace logs separated from a compressed frame.
type SeparatedLogs struct {
	TraceByHeight map[int64][][]byte // Trace lines grouped by height
	NonTraceLines [][]byte           // Lines that are not trace logs
}

// separateLogs decompresses a gzip frame and separates trace logs from other logs.
func separateLogs(compressed []byte) (*SeparatedLogs, error) {
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	result := &SeparatedLogs{
		TraceByHeight: make(map[int64][][]byte),
	}

	scanner := bufio.NewScanner(gr)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 64<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)

		if isTraceLine(line) {
			height := extractHeight(line)
			if height > 0 {
				result.TraceByHeight[height] = append(result.TraceByHeight[height], lineCopy)
			}
		} else {
			result.NonTraceLines = append(result.NonTraceLines, lineCopy)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// CompressLines takes log lines and compresses them into gzip format.
func CompressLines(lines [][]byte) ([]byte, error) {
	if len(lines) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	for i, line := range lines {
		if _, err := gw.Write(line); err != nil {
			gw.Close()
			return nil, err
		}
		if i < len(lines)-1 {
			if _, err := gw.Write([]byte("\n")); err != nil {
				gw.Close()
				return nil, err
			}
		}
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// ContainsTrace checks if compressed data contains any trace logs.
func ContainsTrace(compressed []byte) (bool, error) {
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return false, err
	}
	defer gr.Close()

	scanner := bufio.NewScanner(gr)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 64<<20)

	for scanner.Scan() {
		if isTraceLine(scanner.Bytes()) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// ExtractTraceHeights extracts all unique heights from compressed trace data.
func ExtractTraceHeights(compressed []byte) ([]int64, error) {
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	heightSet := make(map[int64]struct{})
	var heights []int64

	scanner := bufio.NewScanner(gr)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 64<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if isTraceLine(line) {
			height := extractHeight(line)
			if height > 0 {
				if _, exists := heightSet[height]; !exists {
					heightSet[height] = struct{}{}
					heights = append(heights, height)
				}
			}
		}
	}

	return heights, scanner.Err()
}

// FilterTraceLinesByHeight reads compressed data and returns only trace lines matching given heights.
func FilterTraceLinesByHeight(r io.Reader, targetHeights []int64) ([][]byte, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	heightSet := make(map[int64]struct{}, len(targetHeights))
	for _, h := range targetHeights {
		heightSet[h] = struct{}{}
	}

	var result [][]byte
	scanner := bufio.NewScanner(gr)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 64<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if isTraceLine(line) {
			height := extractHeight(line)
			if _, match := heightSet[height]; match {
				lineCopy := make([]byte, len(line))
				copy(lineCopy, line)
				result = append(result, lineCopy)
			}
		}
	}

	return result, scanner.Err()
}

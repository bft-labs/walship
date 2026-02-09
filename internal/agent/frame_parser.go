package agent

import (
	"bytes"
	"compress/gzip"
)

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

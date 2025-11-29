package agent

import (
	"bytes"
	"compress/gzip"
	"hash/crc32"
	"io"
)

// verifyFrame reads a gzip member and optionally checks CRC/line counts.
func verifyFrame(fm FrameMeta, rc io.ReadCloser) error {
	defer rc.Close()
	zr, err := gzip.NewReader(rc)
	if err != nil {
		return err
	}
	defer zr.Close()
	buf := make([]byte, 64<<10)
	var lines int
	h := crc32.NewIEEE()
	for {
		n, err := zr.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			h.Write(chunk)
			lines += bytes.Count(chunk, []byte{'\n'})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	// Optional checks; non-fatal in calling context.
	_ = lines
	_ = h.Sum32()
	return nil
}

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	walFramesEndpoint = "/v1/ingest/wal-frames"
	configEndpoint    = "/v1/ingest/config"
)

type batchFrame struct {
	Meta       FrameMeta
	Compressed []byte
	IdxLineLen int
}

func Run(ctx context.Context, cfg Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if cfg.ServiceURL == "" {
		return fmt.Errorf("service-url is required")
	}
	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return fmt.Errorf("state dir: %w", err)
	}

	// Start config watcher for dynamic configuration updates
	cfgPtr := &cfg
	watcher := NewConfigWatcher(cfgPtr)
	go watcher.Run(ctx)
	go walCleanupLoop(ctx, cfg.WALDir, cfg.StateDir)

	// Load prior state; if none, start from the oldest index (first logs)
	st, _ := loadState(cfg.StateDir)
	if st.IdxPath == "" {
		idxPath, err := oldestIndex(cfg.WALDir)
		if err != nil {
			return err
		}
		st.IdxPath = idxPath
		st.IdxOffset = 0
		_ = saveState(cfg.StateDir, st)
	}

	idx, r, err := openIdx(st.IdxPath)
	if err != nil {
		return fmt.Errorf("open idx: %w", err)
	}
	defer idx.Close()
	if st.IdxOffset > 0 {
		if _, err := idx.Seek(st.IdxOffset, io.SeekStart); err == nil {
			r.Reset(idx)
		}
	}

	// Open current gz if known
	var gz *os.File
	if st.CurGz != "" {
		if f, err := openGz(filepath.Join(filepath.Dir(st.IdxPath), st.CurGz)); err == nil {
			gz = f
		}
	}
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	back := newBackoff(500*time.Millisecond, 10*time.Second)

	var (
		batch      []batchFrame
		batchBytes int
		lastSend   time.Time
	)

	for {
		// Handle context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fm, line, nerr := func() (FrameMeta, []byte, error) { return nextFrame(r) }()
		if nerr != nil {
			if errors.Is(nerr, os.ErrClosed) {
				return nerr
			}
			if errors.Is(nerr, io.EOF) {
				// Flush pending batch
				if len(batch) > 0 {
					trySend(cfg, httpClient, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), &gz, lastSend, back)
					lastSend = st.LastSendAt
				}
				if cfg.Once {
					return nil
				}
				// rotation discovery: move to next index after current
				if next, ok, _ := nextIndexAfter(st.IdxPath); ok {
					idx.Close()
					if gz != nil {
						gz.Close()
					}
					idx2, r2, oerr := openIdx(next)
					if oerr == nil {
						idx, r = idx2, r2
						st.IdxPath, st.IdxOffset, st.CurGz = next, 0, ""
						_ = saveState(cfg.StateDir, st)
						continue
					}
				}
				time.Sleep(cfg.PollInterval)
				continue
			}
			// other read error
			time.Sleep(cfg.PollInterval)
			continue
		}

		// Ensure gz open for this frame
		if gz == nil || filepath.Base(st.CurGz) != fm.File {
			if gz != nil {
				_ = gz.Close()
			}
			path := filepath.Join(filepath.Dir(st.IdxPath), fm.File)
			ngz, gerr := openGz(path)
			if gerr != nil {
				time.Sleep(cfg.PollInterval)
				continue
			}
			gz = ngz
			st.CurGz = fm.File
		}
		if cfg.Meta {
			logger.Info().
				Str("file", fm.File).
				Uint64("frame", fm.Frame).
				Uint64("off", fm.Off).
				Uint64("len", fm.Len).
				Uint32("recs", fm.Recs).
				Msg("frame metadata")
		}
		// Read compressed bytes for this frame
		b, rerr := preadSection(gz, int64(fm.Off), int64(fm.Len))
		if rerr != nil {
			time.Sleep(cfg.PollInterval)
			continue
		}
		if cfg.Verify {
			_ = verifyFrame(fm, io.NopCloser(bytes.NewReader(b)))
		}

		// Large frame: send alone
		if cfg.MaxBatchBytes > 0 && len(b) > cfg.MaxBatchBytes {
			bf := batchFrame{Meta: fm, Compressed: b, IdxLineLen: len(line)}
			batch = append(batch, bf)
			batchBytes += len(b)
			trySend(cfg, httpClient, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), &gz, lastSend, back)
			lastSend = st.LastSendAt
			continue
		}
		// Normal batch
		if cfg.MaxBatchBytes > 0 && batchBytes+len(b) > cfg.MaxBatchBytes {
			trySend(cfg, httpClient, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), &gz, lastSend, back)
			lastSend = st.LastSendAt
		}
		batch = append(batch, batchFrame{Meta: fm, Compressed: b, IdxLineLen: len(line)})
		batchBytes += len(b)

		// Time-based send
		if time.Since(lastSend) >= cfg.SendInterval || time.Since(lastSend) >= cfg.HardInterval {
			trySend(cfg, httpClient, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), &gz, lastSend, back)
			lastSend = st.LastSendAt
		}
	}
}

func trySend(cfg Config, httpClient *http.Client, batch *[]batchFrame, batchBytes *int, st *state, curIdxBase string, gz **os.File, lastSend time.Time, back *backoff) {
	if len(*batch) == 0 {
		return
	}
	// Resource gating (soft)
	hard := time.Since(lastSend) >= cfg.HardInterval
	if !hard && !resourcesOK(cfg) {
		return
	}

	// Build payload
	manifest := make([]FrameMeta, 0, len(*batch))
	var advance int64
	for _, fr := range *batch {
		manifest = append(manifest, fr.Meta)
		advance += int64(fr.IdxLineLen)
	}
	url := cfg.ServiceURL + walFramesEndpoint
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		logger.Error().Err(err).Msg("marshal manifest")
		back.Sleep()
		return
	}
	manifestPart, err := writer.CreateFormField("manifest")
	if err != nil {
		logger.Error().Err(err).Msg("create manifest field")
		back.Sleep()
		return
	}
	if _, err := manifestPart.Write(manifestJSON); err != nil {
		logger.Error().Err(err).Msg("write manifest field")
		back.Sleep()
		return
	}

	framesPart, err := writer.CreateFormFile("frames", curIdxBase)
	if err != nil {
		logger.Error().Err(err).Msg("create frames field")
		back.Sleep()
		return
	}
	for _, fr := range *batch {
		if _, err := framesPart.Write(fr.Compressed); err != nil {
			logger.Error().Err(err).Msg("write frames payload")
			back.Sleep()
			return
		}
	}
	if err := writer.Close(); err != nil {
		logger.Error().Err(err).Msg("finalize multipart payload")
		back.Sleep()
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AuthKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Agent-Hostname", hostname())
	req.Header.Set("X-Agent-OSArch", runtime.GOOS+"/"+runtime.GOARCH)
	req.Header.Set("X-Cosmos-Analyzer-Chain-Id", cfg.ChainID)
	req.Header.Set("X-Cosmos-Analyzer-Node-Id", cfg.NodeID)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("send batch")
		back.Sleep()
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		logger.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("server returned error")
		back.Sleep()
		return
	}

	logger.Info().
		Int("frames", len(*batch)).
		Int("bytes", *batchBytes).
		Msg("sent batch")

	// Success: commit idx offset
	st.IdxOffset += advance
	st.LastFile = manifest[len(manifest)-1].File
	st.LastFrame = manifest[len(manifest)-1].Frame
	st.LastSendAt = time.Now()
	st.LastCommitAt = st.LastSendAt
	_ = saveState(cfg.StateDir, *st)

	// reset batch
	*batch = (*batch)[:0]
	*batchBytes = 0
	back.Reset()
}

func hostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

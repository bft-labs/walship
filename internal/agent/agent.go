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
	walFramesEndpoint   = "/v1/ingest/wal-frames"
	traceFramesEndpoint = "/v1/ingest/trace-frames"
	configEndpoint      = "/v1/ingest/config"
)

// IngestResponse represents the response from WAL frames endpoint.
type IngestResponse struct {
	Status         string  `json:"status"`
	Frames         int     `json:"frames"`
	EventsProc     int     `json:"events_processed"`
	EventsStored   int     `json:"events_stored"`
	FailureHeights []int64 `json:"failure_heights"`
}

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

	// Initialize trace store for height-indexed trace logs
	traceDir := filepath.Join(cfg.StateDir, "traces")
	traceStore, err := NewTraceStore(traceDir)
	if err != nil {
		return fmt.Errorf("init trace store: %w", err)
	}
	go traceCleanupLoop(ctx, cfg.StateDir)

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
					trySend(cfg, httpClient, traceStore, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), lastSend, back)
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
		// Warn if frame is extremely large (>50MB), as it may cause issues
		const warnThresholdBytes = 50 << 20
		if len(b) > warnThresholdBytes {
			logger.Warn().
				Str("file", fm.File).
				Uint64("frame", fm.Frame).
				Int("size_mb", len(b)/(1<<20)).
				Msg("extremely large frame detected - consider investigating data size")
		}
		if cfg.MaxBatchBytes > 0 && len(b) > cfg.MaxBatchBytes {
			logger.Debug().
				Str("file", fm.File).
				Uint64("frame", fm.Frame).
				Int("size_mb", len(b)/(1<<20)).
				Msg("large frame sent alone")
			bf := batchFrame{Meta: fm, Compressed: b, IdxLineLen: len(line)}
			batch = append(batch, bf)
			batchBytes += len(b)
			trySend(cfg, httpClient, traceStore, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), lastSend, back)
			lastSend = st.LastSendAt
			continue
		}
		// Normal batch
		if cfg.MaxBatchBytes > 0 && batchBytes+len(b) > cfg.MaxBatchBytes {
			trySend(cfg, httpClient, traceStore, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), lastSend, back)
			lastSend = st.LastSendAt
		}
		batch = append(batch, batchFrame{Meta: fm, Compressed: b, IdxLineLen: len(line)})
		batchBytes += len(b)

		// Time-based send
		if time.Since(lastSend) >= cfg.SendInterval || time.Since(lastSend) >= cfg.HardInterval {
			trySend(cfg, httpClient, traceStore, &batch, &batchBytes, &st, filepath.Base(st.IdxPath), lastSend, back)
			lastSend = st.LastSendAt
		}
	}
}

func trySend(cfg Config, httpClient *http.Client, traceStore *TraceStore, batch *[]batchFrame, batchBytes *int, st *state, curIdxBase string, lastSend time.Time, back *backoff) {
	if len(*batch) == 0 {
		return
	}
	hard := time.Since(lastSend) >= cfg.HardInterval
	if !hard && !resourcesOK(cfg) {
		return
	}

	// Parse frames and separate trace from non-trace
	var nonTraceFrames []batchFrame
	var advance int64
	for _, fr := range *batch {
		advance += int64(fr.IdxLineLen)

		separated, err := separateLogs(fr.Compressed)
		if err != nil {
			logger.Warn().Err(err).Str("file", fr.Meta.File).Msg("failed to parse frame, sending as-is")
			nonTraceFrames = append(nonTraceFrames, fr)
			continue
		}

		// Save trace lines by height
		for height, lines := range separated.TraceByHeight {
			if err := traceStore.Save(height, lines); err != nil {
				logger.Warn().Err(err).Int64("height", height).Msg("failed to save trace")
			}
		}

		// Compress non-trace lines
		if len(separated.NonTraceLines) > 0 {
			compressed, err := CompressLines(separated.NonTraceLines)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to compress non-trace lines")
				continue
			}
			newMeta := fr.Meta
			newMeta.Len = uint64(len(compressed))
			newMeta.Recs = uint32(len(separated.NonTraceLines))
			nonTraceFrames = append(nonTraceFrames, batchFrame{
				Meta:       newMeta,
				Compressed: compressed,
				IdxLineLen: fr.IdxLineLen,
			})
		}
	}

	// Send non-trace frames
	if len(nonTraceFrames) == 0 {
		logger.Debug().Msg("no non-trace frames to send")
		st.IdxOffset += advance
		st.LastSendAt = time.Now()
		st.LastCommitAt = st.LastSendAt
		_ = saveState(cfg.StateDir, *st)
		*batch = (*batch)[:0]
		*batchBytes = 0
		back.Reset()
		return
	}

	ingestResp, ok := sendFrames(cfg, httpClient, nonTraceFrames, curIdxBase, walFramesEndpoint, back)
	if !ok {
		return
	}

	logger.Info().
		Int("frames", len(nonTraceFrames)).
		Int("failure_heights", len(ingestResp.FailureHeights)).
		Msg("sent non-trace batch")

	// Send trace for failure heights
	if len(ingestResp.FailureHeights) > 0 {
		sendTraceForHeights(cfg, httpClient, traceStore, ingestResp.FailureHeights, back)
	}

	// Commit state
	if len(*batch) > 0 {
		lastFrame := (*batch)[len(*batch)-1]
		st.LastFile = lastFrame.Meta.File
		st.LastFrame = lastFrame.Meta.Frame
	}
	st.IdxOffset += advance
	st.LastSendAt = time.Now()
	st.LastCommitAt = st.LastSendAt
	_ = saveState(cfg.StateDir, *st)

	*batch = (*batch)[:0]
	*batchBytes = 0
	back.Reset()
}

// sendFrames sends frames to the specified endpoint and returns the response.
func sendFrames(cfg Config, httpClient *http.Client, frames []batchFrame, curIdxBase string, endpoint string, back *backoff) (*IngestResponse, bool) {
	manifest := make([]FrameMeta, 0, len(frames))
	for _, fr := range frames {
		manifest = append(manifest, fr.Meta)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		logger.Error().Err(err).Msg("marshal manifest")
		back.Sleep()
		return nil, false
	}
	manifestPart, err := writer.CreateFormField("manifest")
	if err != nil {
		logger.Error().Err(err).Msg("create manifest field")
		back.Sleep()
		return nil, false
	}
	if _, err := manifestPart.Write(manifestJSON); err != nil {
		logger.Error().Err(err).Msg("write manifest field")
		back.Sleep()
		return nil, false
	}

	framesPart, err := writer.CreateFormFile("frames", curIdxBase)
	if err != nil {
		logger.Error().Err(err).Msg("create frames field")
		back.Sleep()
		return nil, false
	}
	for _, fr := range frames {
		if _, err := framesPart.Write(fr.Compressed); err != nil {
			logger.Error().Err(err).Msg("write frames payload")
			back.Sleep()
			return nil, false
		}
	}
	if err := writer.Close(); err != nil {
		logger.Error().Err(err).Msg("finalize multipart payload")
		back.Sleep()
		return nil, false
	}

	url := cfg.ServiceURL + endpoint
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		back.Sleep()
		return nil, false
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AuthKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Agent-Hostname", hostname())
	req.Header.Set("X-Agent-OSArch", runtime.GOOS+"/"+runtime.GOARCH)
	req.Header.Set("X-Cosmos-Analyzer-Chain-Id", cfg.ChainID)
	req.Header.Set("X-Cosmos-Analyzer-Node-Id", cfg.NodeID)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("send request")
		back.Sleep()
		return nil, false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		logger.Error().
			Int("status", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("server returned error")
		back.Sleep()
		return nil, false
	}

	var ingestResp IngestResponse
	if err := json.Unmarshal(respBody, &ingestResp); err != nil {
		logger.Warn().Err(err).Msg("failed to parse ingest response")
		return &IngestResponse{Status: "ok"}, true
	}

	return &ingestResp, true
}

// sendTraceForHeights sends trace data for the given failure heights.
func sendTraceForHeights(cfg Config, httpClient *http.Client, traceStore *TraceStore, heights []int64, back *backoff) {
	compressed, metas, err := traceStore.LoadMultiple(heights)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to load traces for failure heights")
		return
	}
	if compressed == nil || len(metas) == 0 {
		logger.Debug().Ints64("heights", heights).Msg("no trace data found for failure heights")
		return
	}

	frames := []batchFrame{{
		Meta:       metas[0],
		Compressed: compressed,
	}}

	_, ok := sendFrames(cfg, httpClient, frames, "trace.wal.gz", traceFramesEndpoint, back)
	if ok {
		logger.Info().
			Ints64("heights", heights).
			Int("bytes", len(compressed)).
			Msg("sent trace for failure heights")
	}
}

func hostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/bft-labs/walship/internal/domain"
	"github.com/bft-labs/walship/internal/ports"
	"github.com/bft-labs/walship/pkg/sender"
)

const walFramesEndpoint = "/v1/ingest/wal-frames"

// FrameSender implements ports.FrameSender using HTTP.
type FrameSender struct {
	client ports.HTTPClient
	logger ports.Logger
}

// NewFrameSender creates a new HTTP frame sender.
func NewFrameSender(client ports.HTTPClient, logger ports.Logger) *FrameSender {
	return &FrameSender{
		client: client,
		logger: logger,
	}
}

// Send transmits frames to the remote service.
func (s *FrameSender) Send(ctx context.Context, frames []sender.FrameData, metadata sender.Metadata) error {
	if len(frames) == 0 {
		return nil
	}

	// Build manifest
	manifest := make([]domain.FrameMeta, len(frames))
	for i, fd := range frames {
		manifest[i] = domain.FrameMeta{
			File:    fd.Frame.File,
			Frame:   fd.Frame.FrameNumber,
			Off:     fd.Frame.Offset,
			Len:     fd.Frame.Length,
			Recs:    fd.Frame.RecordCount,
			FirstTS: fd.Frame.FirstTimestamp,
			LastTS:  fd.Frame.LastTimestamp,
			CRC32:   fd.Frame.CRC32,
		}
	}

	// Build multipart request body
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add manifest
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPart, err := writer.CreateFormField("manifest")
	if err != nil {
		return fmt.Errorf("create manifest field: %w", err)
	}
	if _, err := manifestPart.Write(manifestJSON); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Add frames data
	// Use the first frame's file as the filename hint (frames is guaranteed non-empty here)
	filename := filepath.Base(frames[0].Frame.File)

	framesPart, err := writer.CreateFormFile("frames", filename)
	if err != nil {
		return fmt.Errorf("create frames field: %w", err)
	}

	for _, fd := range frames {
		if _, err := framesPart.Write(fd.CompressedData); err != nil {
			return fmt.Errorf("write frames data: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize multipart: %w", err)
	}

	// Build request
	url := metadata.ServiceURL + walFramesEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+metadata.AuthKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Agent-Hostname", metadata.Hostname)
	req.Header.Set("X-Agent-OSArch", runtime.GOOS+"/"+runtime.GOARCH)
	req.Header.Set("X-Cosmos-Analyzer-Chain-Id", metadata.ChainID)
	req.Header.Set("X-Cosmos-Analyzer-Node-Id", metadata.NodeID)

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode/100 != 2 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("server returned %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

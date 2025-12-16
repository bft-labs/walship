package agent

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrySend(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("X-Cosmos-Analyzer-Chain-Id") != "test-chain" {
			t.Errorf("X-Cosmos-ChainId = %v, want test-chain", r.Header.Get("X-Cosmos-Analyzer-Chain-Id"))
		}
		if r.Header.Get("X-Cosmos-Analyzer-Node-Id") != "test-node" {
			t.Errorf("X-Cosmos-NodeId = %v, want test-node", r.Header.Get("X-Cosmos-Analyzer-Node-Id"))
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("Authorization = %v, want Bearer secret", r.Header.Get("Authorization"))
		}

		// Verify body
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			t.Fatalf("expected multipart content type, got %s", mediaType)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		var framesPayload []byte
		var hasManifest bool
		for {
			part, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("multipart read: %v", err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part: %v", err)
			}
			switch part.FormName() {
			case "manifest":
				hasManifest = len(data) > 0
			case "frames":
				framesPayload = data
			}
		}
		if len(framesPayload) == 0 {
			t.Fatalf("frames payload missing")
		}
		if string(framesPayload) != "compressed-data" {
			t.Errorf("Body = %v, want compressed-data", string(framesPayload))
		}
		if !hasManifest {
			t.Fatalf("manifest payload missing")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		AuthKey:    "secret",
	}

	batch := []batchFrame{
		{
			Meta:       FrameMeta{File: "000.gz", Frame: 1},
			Compressed: []byte("compressed-data"),
			IdxLineLen: 10,
		},
	}
	batchBytes := 15
	st := state{IdxOffset: 0}
	back := newBackoff(time.Millisecond, time.Second)

	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "000.idx", nil, time.Now(), back)

	if len(batch) != 0 {
		t.Errorf("batch length = %d, want 0", len(batch))
	}
	if batchBytes != 0 {
		t.Errorf("batchBytes = %d, want 0", batchBytes)
	}
	if st.IdxOffset != 10 {
		t.Errorf("st.IdxOffset = %d, want 10", st.IdxOffset)
	}
}

func TestRun_Startup(t *testing.T) {
	tmpDir := t.TempDir()
	walDir := filepath.Join(tmpDir, "data", "log.wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create genesis.json and node_key.json
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	genesis := genesisDoc{ChainID: "test-chain"}
	genesisBytes, err := json.Marshal(genesis)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "genesis.json"), genesisBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Create node_key.json with valid private key
	_, privKey, _ := ed25519.GenerateKey(nil)
	privKeyBase64 := base64.StdEncoding.EncodeToString(privKey)

	nodeKeyStruct := struct {
		PrivKey struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"priv_key"`
	}{}
	nodeKeyStruct.PrivKey.Type = "tendermint/PrivKeyEd25519"
	nodeKeyStruct.PrivKey.Value = privKeyBase64

	nodeKeyBytes, err := json.Marshal(nodeKeyStruct)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "node_key.json"), nodeKeyBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Create dummy WAL files
	if err := os.WriteFile(filepath.Join(walDir, "0000000000000000.idx"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		NodeHome:     tmpDir,
		WALDir:       walDir,
		ServiceURL:   "http://localhost:8080",
		PollInterval: time.Millisecond,
		StateDir:     filepath.Join(tmpDir, ".walship"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// It should run and exit on context cancellation without error (other than context deadline)
	err = Run(ctx, cfg)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v", err)
	}
}

func TestTrySend_EmptyBatch(t *testing.T) {
	cfg := Config{}
	batch := []batchFrame{}
	batchBytes := 0
	st := state{}
	back := newBackoff(time.Millisecond, time.Second)

	// Should return immediately without error or panic
	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "000.idx", nil, time.Now(), back)
}

func TestTrySend_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := Config{ServiceURL: ts.URL}
	batch := []batchFrame{{Meta: FrameMeta{File: "f", Frame: 1}}}
	batchBytes := 10
	st := state{IdxOffset: 0}
	back := newBackoff(time.Millisecond, time.Second)

	// Should handle 500 error gracefully (backoff and return, no state update)
	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "000.idx", nil, time.Now(), back)

	if len(batch) == 0 {
		t.Error("batch should not be cleared on server error")
	}
	if st.IdxOffset != 0 {
		t.Error("state should not be updated on server error")
	}
}

func TestTrySend_Timeout(t *testing.T) {
	// Server that sleeps longer than client timeout
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		ServiceURL:  ts.URL,
		HTTPTimeout: 10 * time.Millisecond,
	}
	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}

	batch := []batchFrame{{Meta: FrameMeta{File: "f", Frame: 1}}}
	batchBytes := 10
	st := state{IdxOffset: 0}
	back := newBackoff(time.Millisecond, time.Second)

	trySend(cfg, httpClient, &batch, &batchBytes, &st, "000.idx", nil, time.Now(), back)

	if len(batch) == 0 {
		t.Error("batch should not be cleared on timeout")
	}

	if st.IdxOffset != 0 {
		t.Error("state should not be updated on timeout")
	}
}

func TestRun_MissingWALDir(t *testing.T) {
	// Test that Run returns error when WALDir is empty/invalid
	cfg := Config{
		ServiceURL: "http://test",
		StateDir:   "/tmp",
		// WALDir is empty - should fail in oldestIndex
	}
	err := Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run() expected error for missing/invalid WALDir")
	}
	t.Logf("Run() error = %v", err)
}

func TestTrySend_StateVerification(t *testing.T) {
	// Verify that all state fields are properly updated after successful send
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cfg := Config{
		ServiceURL: ts.URL,
		StateDir:   tmpDir,
	}

	batch := []batchFrame{
		{
			Meta:       FrameMeta{File: "seg-000001.wal.gz", Frame: 5},
			Compressed: []byte("data"),
			IdxLineLen: 20,
		},
		{
			Meta:       FrameMeta{File: "seg-000001.wal.gz", Frame: 6},
			Compressed: []byte("more"),
			IdxLineLen: 15,
		},
	}
	batchBytes := 8
	st := state{IdxOffset: 100}
	back := newBackoff(time.Millisecond, time.Second)

	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "seg-000001.wal.idx", nil, time.Now(), back)

	// Verify state updates
	if st.IdxOffset != 135 { // 100 + 20 + 15
		t.Errorf("st.IdxOffset = %d, want 135", st.IdxOffset)
	}
	if st.LastFile != "seg-000001.wal.gz" {
		t.Errorf("st.LastFile = %s, want seg-000001.wal.gz", st.LastFile)
	}
	if st.LastFrame != 6 {
		t.Errorf("st.LastFrame = %d, want 6", st.LastFrame)
	}
	if st.LastSendAt.IsZero() {
		t.Error("st.LastSendAt should be set")
	}
	if st.LastCommitAt.IsZero() {
		t.Error("st.LastCommitAt should be set")
	}

	// Verify batch and batchBytes are cleared
	if len(batch) != 0 {
		t.Errorf("batch should be cleared, got %d items", len(batch))
	}
	if batchBytes != 0 {
		t.Errorf("batchBytes should be reset to 0, got %d", batchBytes)
	}
}

func TestRun_OnceMode(t *testing.T) {
	// Test that Once mode exits cleanly on EOF without error
	tmpDir := t.TempDir()
	walDir := filepath.Join(tmpDir, "wal")
	if err := os.MkdirAll(walDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a minimal index file with no frames (immediate EOF)
	idxPath := filepath.Join(walDir, "0000000000000000.idx")
	if err := os.WriteFile(idxPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		ServiceURL:   "http://localhost:9999",
		StateDir:     filepath.Join(tmpDir, ".state"),
		WALDir:       walDir,
		Once:         true,
		PollInterval: time.Millisecond,
	}

	ctx := context.Background()
	err := Run(ctx, cfg)

	// Once mode should return nil on EOF, not an error
	if err != nil {
		t.Errorf("Once mode should return nil on EOF, got %v", err)
	}
}

func TestTrySend_LargeFrame(t *testing.T) {
	// Test that frames exceeding MaxBatchBytes are sent alone
	var sentBatches int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentBatches++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		ServiceURL:    ts.URL,
		MaxBatchBytes: 100, // Small limit
	}

	// This frame is larger than MaxBatchBytes
	largeData := make([]byte, 200)
	batch := []batchFrame{
		{
			Meta:       FrameMeta{File: "test.gz", Frame: 1},
			Compressed: largeData,
			IdxLineLen: 10,
		},
	}
	batchBytes := len(largeData)
	st := state{}
	back := newBackoff(time.Millisecond, time.Second)

	// In actual Run(), large frames are added to batch then immediately sent
	// Here we verify trySend processes it correctly
	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "test.idx", nil, time.Now(), back)

	if sentBatches != 1 {
		t.Errorf("Expected 1 batch sent, got %d", sentBatches)
	}
	if len(batch) != 0 {
		t.Error("Batch should be cleared after send")
	}
}

func TestTrySend_BatchOverflow(t *testing.T) {
	// Test that batch is sent when adding a frame would exceed MaxBatchBytes
	// This simulates the logic in Run() at line 151-154
	var sendCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		ServiceURL:    ts.URL,
		MaxBatchBytes: 100,
	}

	// First batch with 80 bytes
	batch := []batchFrame{
		{
			Meta:       FrameMeta{File: "test.gz", Frame: 1},
			Compressed: make([]byte, 80),
			IdxLineLen: 10,
		},
	}
	batchBytes := 80
	st := state{}
	back := newBackoff(time.Millisecond, time.Second)

	// Try to send - should succeed
	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "test.idx", nil, time.Now(), back)

	if sendCount != 1 {
		t.Errorf("Expected 1 send, got %d", sendCount)
	}
	if len(batch) != 0 || batchBytes != 0 {
		t.Error("Batch should be cleared after successful send")
	}
}

func TestTrySend_URLConstruction(t *testing.T) {
	// Test that base URL is correctly constructed to full path for WAL frames
	var requestPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		ServiceURL: ts.URL, // Base URL only, no /v1/ingest/wal-frames
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	batch := []batchFrame{
		{
			Meta:       FrameMeta{File: "000.gz", Frame: 1},
			Compressed: []byte("data"),
			IdxLineLen: 10,
		},
	}
	batchBytes := 4
	st := state{IdxOffset: 0}
	back := newBackoff(time.Millisecond, time.Second)

	trySend(cfg, http.DefaultClient, &batch, &batchBytes, &st, "000.idx", nil, time.Now(), back)

	expectedPath := "/v1/ingest/wal-frames"
	if requestPath != expectedPath {
		t.Errorf("Request path = %v, want %v", requestPath, expectedPath)
	}
}

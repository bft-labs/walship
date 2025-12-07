package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigWatcher_SendConfig(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create app.toml
	appToml := `[api]
enable = true
address = "tcp://0.0.0.0:1317"
`
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appToml), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}

	// Create config.toml
	configToml := `[p2p]
laddr = "tcp://0.0.0.0:26656"
seeds = ""
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	// Track received multipart data
	var receivedAppConfig string
	var receivedCometConfig string
	var receivedAppError string
	var receivedCometError string
	var receivedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ingest/config" {
			t.Errorf("Path = %v, want /v1/ingest/config", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Method = %v, want POST", r.Method)
		}

		receivedHeaders = r.Header.Clone()

		// Verify Content-Type is multipart/form-data
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			t.Errorf("Content-Type = %v, want multipart/form-data", contentType)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Get file: app_config
		if file, _, err := r.FormFile("app_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedAppConfig = string(data)
			file.Close()
		}

		// Get file: comet_config
		if file, _, err := r.FormFile("comet_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedCometConfig = string(data)
			file.Close()
		}

		// Get fields: app_error, comet_error
		receivedAppError = r.FormValue("app_error")
		receivedCometError = r.FormValue("comet_error")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		AuthKey:    "secret",
	}

	watcher := NewConfigWatcher(cfg)

	// Send config
	watcher.sendConfig(context.Background())

	// Verify headers
	if receivedHeaders.Get("X-Cosmos-Analyzer-Chain-Id") != "test-chain" {
		t.Errorf("Chain-Id header = %v, want test-chain", receivedHeaders.Get("X-Cosmos-Analyzer-Chain-Id"))
	}
	if receivedHeaders.Get("X-Cosmos-Analyzer-Node-Id") != "test-node" {
		t.Errorf("Node-Id header = %v, want test-node", receivedHeaders.Get("X-Cosmos-Analyzer-Node-Id"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer secret" {
		t.Errorf("Authorization header = %v, want Bearer secret", receivedHeaders.Get("Authorization"))
	}

	// Verify app config was received as file
	if receivedAppConfig == "" {
		t.Error("AppConfig should not be empty")
	}
	if receivedAppError != "" {
		t.Errorf("AppError should be empty, got %v", receivedAppError)
	}

	// Verify comet config was received as file
	if receivedCometConfig == "" {
		t.Error("CometConfig should not be empty")
	}
	if receivedCometError != "" {
		t.Errorf("CometError should be empty, got %v", receivedCometError)
	}
}

func TestConfigWatcher_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	// Don't create any config files

	var receivedAppConfig string
	var receivedCometConfig string
	var receivedAppError string
	var receivedCometError string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Get files (should not exist)
		if file, _, err := r.FormFile("app_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedAppConfig = string(data)
			file.Close()
		}
		if file, _, err := r.FormFile("comet_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedCometConfig = string(data)
			file.Close()
		}

		// Get error fields
		receivedAppError = r.FormValue("app_error")
		receivedCometError = r.FormValue("comet_error")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)
	watcher.sendConfig(context.Background())

	// Should have error codes for missing files
	if receivedAppError != ErrCodeFileNotFound {
		t.Errorf("AppError = %v, want %v", receivedAppError, ErrCodeFileNotFound)
	}
	if receivedCometError != ErrCodeFileNotFound {
		t.Errorf("CometError = %v, want %v", receivedCometError, ErrCodeFileNotFound)
	}
	if receivedAppConfig != "" {
		t.Errorf("AppConfig should be empty when file is missing")
	}
	if receivedCometConfig != "" {
		t.Errorf("CometConfig should be empty when file is missing")
	}
}

func TestConfigWatcher_FsnotifyDetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	appTomlPath := filepath.Join(configDir, "app.toml")
	if err := os.WriteFile(appTomlPath, []byte(`enable = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`laddr = "tcp://0.0.0.0:26656"`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	sendCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sendCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in background
	go watcher.Run(ctx)

	// Wait for initial send
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	initialCount := sendCount
	mu.Unlock()

	if initialCount < 1 {
		t.Errorf("sendCount = %d, want >= 1 (initial send)", initialCount)
	}

	// Modify app.toml
	if err := os.WriteFile(appTomlPath, []byte(`enable = false`), 0644); err != nil {
		t.Fatalf("Failed to modify app.toml: %v", err)
	}

	// Wait for fsnotify to detect change and debounce to fire
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	afterChangeCount := sendCount
	mu.Unlock()

	if afterChangeCount <= initialCount {
		t.Errorf("sendCount after change = %d, want > %d", afterChangeCount, initialCount)
	}
}

func TestConfigWatcher_URLConstruction(t *testing.T) {
	// Test that base URL is correctly constructed to full path for config endpoint
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create app.toml
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}

	// Create config.toml
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var requestPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL, // Base URL only, no /v1/ingest/config
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)
	watcher.sendConfig(context.Background())

	expectedPath := "/v1/ingest/config"
	if requestPath != expectedPath {
		t.Errorf("Request path = %v, want %v", requestPath, expectedPath)
	}
}

// TestConfigWatcher_RetryOnFailure verifies that sendConfig retries when the server fails.
func TestConfigWatcher_RetryOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config files
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	attemptCount := 0

	// Server fails first 2 times, succeeds on 3rd attempt
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		currentAttempt := attemptCount
		mu.Unlock()

		if currentAttempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	// Send config with retry - should succeed after retries
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	watcher.sendConfigWithRetry(ctx)

	mu.Lock()
	finalCount := attemptCount
	mu.Unlock()

	// Should have retried at least 3 times
	if finalCount < 3 {
		t.Errorf("attemptCount = %d, want >= 3", finalCount)
	}
}

// TestConfigWatcher_RetryStopsOnContextCancel verifies that retry stops when context is cancelled.
func TestConfigWatcher_RetryStopsOnContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config files
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	attemptCount := 0

	// Server always fails
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start send in background
	done := make(chan struct{})
	go func() {
		watcher.sendConfigWithRetry(ctx)
		close(done)
	}()

	// Wait for a few attempts
	time.Sleep(2 * time.Second)

	// Cancel context
	cancel()

	// Wait for sendConfigWithRetry to return
	select {
	case <-done:
		// Good, it returned
	case <-time.After(5 * time.Second):
		t.Fatal("sendConfigWithRetry did not stop after context cancel")
	}

	mu.Lock()
	finalCount := attemptCount
	mu.Unlock()

	// Should have attempted at least once but stopped after cancel
	if finalCount < 1 {
		t.Errorf("attemptCount = %d, want >= 1", finalCount)
	}
}

// TestConfigWatcher_RetryPreservesSnapshot verifies that when config changes during retry,
// the original snapshot is preserved and sent (not the latest state).
// This is important for history: each change should be recorded separately.
func TestConfigWatcher_RetryPreservesSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	appTomlPath := filepath.Join(configDir, "app.toml")
	configTomlPath := filepath.Join(configDir, "config.toml")

	// Create initial config files
	if err := os.WriteFile(appTomlPath, []byte(`version = 1`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(configTomlPath, []byte(`version = 1`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	attemptCount := 0
	var lastReceivedAppConfig string

	// Server fails first 3 times, succeeds on 4th attempt
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		currentAttempt := attemptCount
		mu.Unlock()

		// Read the app config content
		if err := r.ParseMultipartForm(10 << 20); err == nil {
			if file, _, err := r.FormFile("app_config"); err == nil {
				data, _ := io.ReadAll(file)
				mu.Lock()
				lastReceivedAppConfig = string(data)
				mu.Unlock()
				file.Close()
			}
		}

		if currentAttempt < 4 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start send in background
	done := make(chan struct{})
	go func() {
		watcher.sendConfigWithRetry(ctx)
		close(done)
	}()

	// Wait for first failure attempt
	time.Sleep(1 * time.Second)

	// Modify app.toml during retry - this should NOT affect the current retry loop
	if err := os.WriteFile(appTomlPath, []byte(`version = 2`), 0644); err != nil {
		t.Fatalf("Failed to modify app.toml: %v", err)
	}

	// Wait for completion
	select {
	case <-done:
		// Good
	case <-time.After(25 * time.Second):
		t.Fatal("sendConfigWithRetry did not complete")
	}

	mu.Lock()
	finalContent := lastReceivedAppConfig
	mu.Unlock()

	// Should have received the ORIGINAL version (snapshot preserved)
	// The modified version = 2 should be sent by a separate retry loop triggered by fsnotify
	if !strings.Contains(finalContent, "version = 1") {
		t.Errorf("lastReceivedAppConfig = %q, want to contain 'version = 1' (snapshot should be preserved)", finalContent)
	}
}

// TestConfigWatcher_NoRetryOnSuccess verifies that successful send doesn't retry.
func TestConfigWatcher_NoRetryOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config files
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	attemptCount := 0

	// Server always succeeds
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	watcher.sendConfigWithRetry(ctx)

	mu.Lock()
	finalCount := attemptCount
	mu.Unlock()

	// Should have attempted exactly once (no retry on success)
	if finalCount != 1 {
		t.Errorf("attemptCount = %d, want 1", finalCount)
	}
}

// TestConfigWatcher_SendsCapturedAtTimestamp verifies that captured_at timestamp is included.
func TestConfigWatcher_SendsCapturedAtTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config files
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var capturedAt string
	beforeSend := time.Now().UTC()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err == nil {
			capturedAt = r.FormValue("captured_at")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)
	ctx := context.Background()
	watcher.sendConfigWithRetry(ctx)

	afterSend := time.Now().UTC()

	// Verify captured_at is present and valid
	if capturedAt == "" {
		t.Fatal("captured_at field is missing")
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		t.Fatalf("captured_at is not valid RFC3339Nano: %v", err)
	}

	// Verify timestamp is within expected range
	if parsedTime.Before(beforeSend) || parsedTime.After(afterSend) {
		t.Errorf("captured_at = %v, want between %v and %v", parsedTime, beforeSend, afterSend)
	}
}


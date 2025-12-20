package configwatcher

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

	"github.com/bft-labs/walship/pkg/walship"
)

// TestPlugin_EndpointPath verifies that the plugin uses the correct config endpoint.
// This test exists to prevent regressions like using /v1/node/config instead of /v1/ingest/config.
func TestPlugin_EndpointPath(t *testing.T) {
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

	var requestPath string
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestPath = r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	plugin := New(Config{
		RetryInterval: 100 * time.Millisecond,
		DebounceDelay: 10 * time.Millisecond,
		HTTPTimeout:   5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := plugin.Initialize(ctx, walship.PluginConfig{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		AuthKey:    "test-key",
		Logger:     &noopLogger{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Wait for initial config send
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	path := requestPath
	mu.Unlock()

	// CRITICAL: Verify the endpoint matches the backend API
	// The correct endpoint is /v1/ingest/config (same as internal/agent/agent.go)
	expectedPath := "/v1/ingest/config"
	if path != expectedPath {
		t.Errorf("Request path = %q, want %q", path, expectedPath)
	}

	if err := plugin.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestPlugin_EndpointConsistency ensures the plugin uses the same endpoint as internal/agent.
// This is a compile-time constant check.
func TestPlugin_EndpointConsistency(t *testing.T) {
	// The constant should match what's defined in internal/agent/agent.go
	// We verify this by checking the constant value directly
	expected := "/v1/ingest/config"
	if configEndpoint != expected {
		t.Errorf("configEndpoint = %q, want %q (must match internal/agent/agent.go)", configEndpoint, expected)
	}
}

func TestPlugin_SendsConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	appToml := `[api]
enable = true
`
	configToml := `[p2p]
laddr = "tcp://0.0.0.0:26656"
`
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appToml), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	var receivedAppConfig, receivedCometConfig string
	var receivedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		receivedHeaders = r.Header.Clone()

		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %v, want multipart/form-data", r.Header.Get("Content-Type"))
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

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

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	plugin := New(DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := plugin.Initialize(ctx, walship.PluginConfig{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		AuthKey:    "secret",
		Logger:     &noopLogger{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	appConfig := receivedAppConfig
	cometConfig := receivedCometConfig
	headers := receivedHeaders
	mu.Unlock()

	// Verify headers
	if headers.Get("X-Cosmos-Analyzer-Chain-Id") != "test-chain" {
		t.Errorf("Chain-Id header = %v, want test-chain", headers.Get("X-Cosmos-Analyzer-Chain-Id"))
	}
	if headers.Get("X-Cosmos-Analyzer-Node-Id") != "test-node" {
		t.Errorf("Node-Id header = %v, want test-node", headers.Get("X-Cosmos-Analyzer-Node-Id"))
	}
	if headers.Get("Authorization") != "Bearer secret" {
		t.Errorf("Authorization header = %v, want Bearer secret", headers.Get("Authorization"))
	}

	// Verify config files were received
	if appConfig == "" {
		t.Error("AppConfig should not be empty")
	}
	if cometConfig == "" {
		t.Error("CometConfig should not be empty")
	}

	if err := plugin.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestPlugin_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	// Don't create any config files

	var mu sync.Mutex
	var receivedAppError, receivedCometError string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return
		}

		receivedAppError = r.FormValue("app_error")
		receivedCometError = r.FormValue("comet_error")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	plugin := New(DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := plugin.Initialize(ctx, walship.PluginConfig{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		Logger:     &noopLogger{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	appErr := receivedAppError
	cometErr := receivedCometError
	mu.Unlock()

	if appErr != ErrCodeFileNotFound {
		t.Errorf("AppError = %v, want %v", appErr, ErrCodeFileNotFound)
	}
	if cometErr != ErrCodeFileNotFound {
		t.Errorf("CometError = %v, want %v", cometErr, ErrCodeFileNotFound)
	}

	if err := plugin.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestPlugin_Name(t *testing.T) {
	plugin := New(DefaultConfig())
	if plugin.Name() != "configwatcher" {
		t.Errorf("Name() = %v, want configwatcher", plugin.Name())
	}
}

func TestPlugin_DisabledWhenNodeHomeEmpty(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	plugin := New(DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize with empty NodeHome - should be disabled
	err := plugin.Initialize(ctx, walship.PluginConfig{
		NodeHome:   "", // Empty
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		Logger:     &noopLogger{},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := requestCount
	mu.Unlock()

	if count != 0 {
		t.Errorf("Expected 0 requests when disabled, got %d", count)
	}

	if err := plugin.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// noopLogger implements walship.Logger for testing
type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...walship.LogField) {}
func (noopLogger) Info(msg string, fields ...walship.LogField)  {}
func (noopLogger) Warn(msg string, fields ...walship.LogField)  {}
func (noopLogger) Error(msg string, fields ...walship.LogField) {}

// Package configwatcher provides config file monitoring for walship.
// When enabled, it watches app.toml and config.toml for changes and
// sends updates to the service.
package configwatcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/bft-labs/walship/pkg/walship"
)

const configEndpoint = "/v1/ingest/config"

// Error codes for config file issues.
const (
	ErrCodeFileNotFound     = "FILE_NOT_FOUND"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeReadError        = "READ_ERROR"
)

// Plugin implements config watching functionality.
// It monitors app.toml and config.toml in the node's config directory
// and sends updates to the service when they change.
type Plugin struct {
	mu sync.RWMutex

	// Configuration
	retryInterval time.Duration
	debounceDelay time.Duration

	// Runtime state
	nodeHome   string
	serviceURL string
	chainID    string
	nodeID     string
	authKey    string
	logger     walship.Logger
	httpClient *http.Client
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	debounce   *time.Timer
}

// Config holds configuration options for the config watcher plugin.
type Config struct {
	// RetryInterval is the delay between retries on failure.
	// Default: 5 seconds
	RetryInterval time.Duration

	// DebounceDelay is the delay to wait after a file change before sending.
	// Default: 100 milliseconds
	DebounceDelay time.Duration

	// HTTPTimeout is the timeout for HTTP requests.
	// Default: 30 seconds
	HTTPTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		RetryInterval: 5 * time.Second,
		DebounceDelay: 100 * time.Millisecond,
		HTTPTimeout:   30 * time.Second,
	}
}

// New creates a new config watcher plugin with the given configuration.
func New(cfg Config) *Plugin {
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 5 * time.Second
	}
	if cfg.DebounceDelay <= 0 {
		cfg.DebounceDelay = 100 * time.Millisecond
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}

	return &Plugin{
		retryInterval: cfg.RetryInterval,
		debounceDelay: cfg.DebounceDelay,
		httpClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

// Name returns the plugin identifier.
func (p *Plugin) Name() string {
	return "configwatcher"
}

// Initialize sets up the plugin and starts the config watcher.
func (p *Plugin) Initialize(ctx context.Context, cfg walship.PluginConfig) error {
	p.mu.Lock()
	p.nodeHome = cfg.NodeHome
	p.serviceURL = cfg.ServiceURL
	p.chainID = cfg.ChainID
	p.nodeID = cfg.NodeID
	p.authKey = cfg.AuthKey
	p.logger = cfg.Logger
	p.mu.Unlock()

	if p.nodeHome == "" || p.serviceURL == "" {
		p.logger.Warn("Config watcher disabled: nodeHome or serviceURL not configured")
		return nil
	}

	// Create cancellable context for the watcher loop
	watchCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	p.logger.Info("Config watcher plugin initialized")

	// Start watcher loop
	p.wg.Add(1)
	go p.watchLoop(watchCtx)

	return nil
}

// Shutdown stops the config watcher.
func (p *Plugin) Shutdown(ctx context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}

// watchLoop watches for config file changes.
func (p *Plugin) watchLoop(ctx context.Context) {
	defer p.wg.Done()

	configDir := p.configDir()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		p.logger.Error("Config watcher: failed to create watcher: " + err.Error())
		return
	}
	defer watcher.Close()

	if err := watcher.Add(configDir); err != nil {
		p.logger.Error("Config watcher: failed to watch directory: " + err.Error())
		// Still try to send initial config
		p.sendConfigWithRetry(ctx)
		return
	}

	// Send initial config
	p.sendConfigWithRetry(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			filename := filepath.Base(event.Name)
			if filename != "app.toml" && filename != "config.toml" {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			p.debounceSend(ctx, p.debounceDelay)

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return
			}
			p.logger.Error("Config watcher: watcher error: " + watchErr.Error())
		}
	}
}

func (p *Plugin) debounceSend(ctx context.Context, delay time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.debounce != nil {
		p.debounce.Stop()
	}

	p.debounce = time.AfterFunc(delay, func() {
		// Check if context was canceled before the timer fired
		select {
		case <-ctx.Done():
			return
		default:
			p.sendConfigWithRetry(ctx)
		}
	})
}

func (p *Plugin) configDir() string       { return filepath.Join(p.nodeHome, "config") }
func (p *Plugin) appConfigPath() string   { return filepath.Join(p.configDir(), "app.toml") }
func (p *Plugin) cometConfigPath() string { return filepath.Join(p.configDir(), "config.toml") }
func (p *Plugin) configURL() string       { return p.serviceURL + configEndpoint }

// buildMultipartPayload builds multipart form-data with config files.
// Returns the payload buffer, content type, and any error encountered.
func (p *Plugin) buildMultipartPayload() (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("captured_at", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return nil, "", fmt.Errorf("write captured_at: %w", err)
	}

	appContent, appErr := p.readFile(p.appConfigPath())
	if appErr != nil {
		if err := writer.WriteField("app_error", p.errorToCode(appErr)); err != nil {
			return nil, "", fmt.Errorf("write app_error: %w", err)
		}
	} else {
		part, err := writer.CreateFormFile("app_config", "app.toml")
		if err != nil {
			return nil, "", fmt.Errorf("create app_config field: %w", err)
		}
		if _, err := part.Write([]byte(appContent)); err != nil {
			return nil, "", fmt.Errorf("write app_config: %w", err)
		}
	}

	cometContent, cometErr := p.readFile(p.cometConfigPath())
	if cometErr != nil {
		if err := writer.WriteField("comet_error", p.errorToCode(cometErr)); err != nil {
			return nil, "", fmt.Errorf("write comet_error: %w", err)
		}
	} else {
		part, err := writer.CreateFormFile("comet_config", "config.toml")
		if err != nil {
			return nil, "", fmt.Errorf("create comet_config field: %w", err)
		}
		if _, err := part.Write([]byte(cometContent)); err != nil {
			return nil, "", fmt.Errorf("write comet_config: %w", err)
		}
	}

	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	return &buf, contentType, nil
}

// sendConfigWithRetry retries until success or context cancellation.
func (p *Plugin) sendConfigWithRetry(ctx context.Context) {
	retryCount := 0

	snapshot, contentType, err := p.buildMultipartPayload()
	if err != nil {
		p.logger.Error("Config watcher: failed to build payload: " + err.Error())
		return
	}
	snapshotBytes := snapshot.Bytes()

	for {
		reader := bytes.NewReader(snapshotBytes)

		if err := p.send(ctx, reader, contentType); err == nil {
			if retryCount > 0 {
				p.logger.Info("Config watcher: sent configuration update after retries")
			} else {
				p.logger.Info("Config watcher: sent configuration update")
			}
			return
		}

		// Failure - log and retry
		retryCount++
		p.logger.Error("Config watcher: send failed")

		select {
		case <-ctx.Done():
			p.logger.Info("Config watcher: stopping retry due to context cancellation")
			return
		case <-time.After(p.retryInterval):
			// Continue to next retry
		}
	}
}

func (p *Plugin) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *Plugin) errorToCode(err error) string {
	if os.IsNotExist(err) {
		return ErrCodeFileNotFound
	}
	if os.IsPermission(err) {
		return ErrCodePermissionDenied
	}
	if strings.Contains(err.Error(), "permission denied") {
		return ErrCodePermissionDenied
	}
	return ErrCodeReadError
}

func (p *Plugin) send(ctx context.Context, body io.Reader, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.configURL(), body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Cosmos-Analyzer-Chain-Id", p.chainID)
	req.Header.Set("X-Cosmos-Analyzer-Node-Id", p.nodeID)
	if p.authKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.authKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("unexpected status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Ensure Plugin implements walship.Plugin.
var _ walship.Plugin = (*Plugin)(nil)

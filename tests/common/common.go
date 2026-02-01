// Package common provides shared test utilities for iter-service tests.
// All test setup, teardown, and result collection is centralized here.
package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestEnv represents an isolated test environment with its own iter-service instance.
type TestEnv struct {
	T          *testing.T
	Name       string
	Type       string // "api", "service", or "ui"
	DataDir    string
	ConfigPath string
	ResultsDir string
	Port       int
	BaseURL    string
	Cmd        *exec.Cmd
	LogFile    *os.File
	mu         sync.Mutex
	started    bool
	external   bool // true if using external service via ITER_BASE_URL
}

// portCounter is used to allocate unique ports for each test.
var (
	portCounter = 19000
	portMu      sync.Mutex
	projectRoot string
)

// allocatePort returns a unique port for the test.
func allocatePort() int {
	portMu.Lock()
	defer portMu.Unlock()
	portCounter++
	return portCounter
}

// getProjectRoot finds the project root directory by looking for go.mod.
func getProjectRoot() string {
	if projectRoot != "" {
		return projectRoot
	}

	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, use current directory
			projectRoot, _ = os.Getwd()
			return projectRoot
		}
		dir = parent
	}
}

// NewTestEnv creates a new isolated test environment.
// The environment includes its own data directory, config file, and port.
// testType should be "api", "service", or "ui".
// testName is the specific test name (e.g., "project-crud").
//
// If ITER_BASE_URL environment variable is set, the test will use an external
// service instead of starting its own. This is useful for Docker-based testing.
func NewTestEnv(t *testing.T, testType, testName string) *TestEnv {
	t.Helper()

	// Create results directory: ./tests/results/{type}/{datetime}-{testname}/
	root := getProjectRoot()
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	resultsDir := filepath.Join(root, "tests", "results", testType, fmt.Sprintf("%s-%s", timestamp, testName))
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	// Create isolated data directory within results
	dataDir := filepath.Join(resultsDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Check for external service URL
	externalURL := os.Getenv("ITER_BASE_URL")
	if externalURL != "" {
		env := &TestEnv{
			T:          t,
			Name:       testName,
			Type:       testType,
			DataDir:    dataDir,
			ResultsDir: resultsDir,
			Port:       0,
			BaseURL:    externalURL,
			external:   true,
		}
		t.Logf("Using external service at %s", externalURL)
		return env
	}

	port := allocatePort()

	env := &TestEnv{
		T:          t,
		Name:       testName,
		Type:       testType,
		DataDir:    dataDir,
		ConfigPath: filepath.Join(dataDir, "config.toml"),
		ResultsDir: resultsDir,
		Port:       port,
		BaseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		external:   false,
	}

	// Write test config file
	if err := env.writeConfig(); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	return env
}

// writeConfig writes the test configuration file.
func (e *TestEnv) writeConfig() error {
	config := fmt.Sprintf(`[service]
host = "127.0.0.1"
port = %d
data_dir = "%s"
pid_file = "%s/iter-service.pid"
shutdown_timeout_seconds = 5

[api]
enabled = true
api_key = ""

[mcp]
enabled = true

[logging]
level = "debug"
format = "text"
output = "file"

[index]
debounce_ms = 100
watch_enabled = true
`, e.Port, e.DataDir, e.DataDir)

	return os.WriteFile(e.ConfigPath, []byte(config), 0644)
}

// Start starts the iter-service for this test environment.
// If using an external service (ITER_BASE_URL), this just verifies connectivity.
func (e *TestEnv) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return fmt.Errorf("service already started")
	}

	// If using external service, just verify it's reachable
	if e.external {
		if err := e.waitForReady(10 * time.Second); err != nil {
			return fmt.Errorf("external service not ready at %s: %w", e.BaseURL, err)
		}
		e.started = true
		e.Log("Using external service at %s", e.BaseURL)
		return nil
	}

	// Find the binary
	binaryPath := findBinary()
	if binaryPath == "" {
		return fmt.Errorf("iter-service binary not found")
	}

	// Open log file
	logPath := filepath.Join(e.ResultsDir, "service.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	e.LogFile = logFile

	// Start service
	e.Cmd = exec.Command(binaryPath, "serve", "--config", e.ConfigPath)
	e.Cmd.Stdout = logFile
	e.Cmd.Stderr = logFile
	e.Cmd.Env = append(os.Environ(),
		fmt.Sprintf("ITER_CONFIG=%s", e.ConfigPath),
		fmt.Sprintf("ITER_DATA_DIR=%s", e.DataDir),
	)

	if err := e.Cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start service: %w", err)
	}

	// Wait for service to be ready
	if err := e.waitForReady(30 * time.Second); err != nil {
		e.Stop()
		return fmt.Errorf("service not ready: %w", err)
	}

	e.started = true
	e.Log("Service started on port %d", e.Port)
	return nil
}

// waitForReady waits for the service to respond to health checks.
func (e *TestEnv) waitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(e.BaseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for service to be ready")
}

// Stop stops the iter-service.
// If using an external service, this is a no-op.
func (e *TestEnv) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Don't stop external services
	if e.external {
		e.started = false
		return
	}

	if e.Cmd != nil && e.Cmd.Process != nil {
		e.Cmd.Process.Signal(os.Interrupt)

		// Wait for graceful shutdown with timeout
		done := make(chan error, 1)
		go func() {
			done <- e.Cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			e.Cmd.Process.Kill()
			<-done // Wait for process to actually exit
		}

		// Wait for port to be released
		e.waitForPortRelease(5 * time.Second)
	}

	if e.LogFile != nil {
		e.LogFile.Close()
	}

	e.started = false
}

// waitForPortRelease waits for the service port to be released.
func (e *TestEnv) waitForPortRelease(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}

	for time.Now().Before(deadline) {
		resp, err := client.Get(e.BaseURL + "/health")
		if err != nil {
			// Port is released
			return
		}
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

// Log writes a message to the test log file.
func (e *TestEnv) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))

	// Write to results log
	logPath := filepath.Join(e.ResultsDir, "test.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(msg)
		f.Close()
	}

	// Also log to test
	e.T.Log(strings.TrimSpace(msg))
}

// SaveResult saves a test result to the results directory.
func (e *TestEnv) SaveResult(name string, data []byte) error {
	path := filepath.Join(e.ResultsDir, name)
	return os.WriteFile(path, data, 0644)
}

// SaveJSON saves a JSON result to the results directory.
func (e *TestEnv) SaveJSON(name string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return e.SaveResult(name, data)
}

// SaveScreenshot saves HTML content as a screenshot file.
func (e *TestEnv) SaveScreenshot(name string, html []byte) error {
	return e.SaveResult(name+".html", html)
}

// WriteSummary writes a test summary to the results directory.
func (e *TestEnv) WriteSummary(passed bool, duration time.Duration, details string) error {
	summary := map[string]interface{}{
		"test_name": e.Name,
		"passed":    passed,
		"duration":  duration.String(),
		"timestamp": time.Now().Format(time.RFC3339),
		"port":      e.Port,
		"data_dir":  e.DataDir,
		"details":   details,
	}
	return e.SaveJSON("summary.json", summary)
}

// findBinary locates the iter-service binary.
func findBinary() string {
	root := getProjectRoot()

	// Try bin/ directory first (preferred location)
	paths := []string{
		filepath.Join(root, "bin", "iter-service"),
		"./bin/iter-service",
		"../bin/iter-service",
		"../../bin/iter-service",
	}

	// Also try to find in PATH
	if path, err := exec.LookPath("iter-service"); err == nil {
		paths = append([]string{path}, paths...)
	}

	for _, p := range paths {
		if absPath, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	return ""
}

// HTTPClient returns an HTTP client for making API requests.
type HTTPClient struct {
	env    *TestEnv
	client *http.Client
}

// NewHTTPClient creates an HTTP client for the test environment.
func (e *TestEnv) NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		env: e,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get performs a GET request.
func (c *HTTPClient) Get(path string) (*http.Response, []byte, error) {
	return c.Do("GET", path, nil)
}

// Post performs a POST request with JSON body.
func (c *HTTPClient) Post(path string, body interface{}) (*http.Response, []byte, error) {
	return c.Do("POST", path, body)
}

// Delete performs a DELETE request.
func (c *HTTPClient) Delete(path string) (*http.Response, []byte, error) {
	return c.Do("DELETE", path, nil)
}

// Do performs an HTTP request.
func (c *HTTPClient) Do(method, path string, body interface{}) (*http.Response, []byte, error) {
	url := c.env.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.env.Log("%s %s", method, path)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("read response: %w", err)
	}

	c.env.Log("Response: %d %s", resp.StatusCode, string(respBody))

	return resp, respBody, nil
}

// GetHTML fetches an HTML page.
func (c *HTTPClient) GetHTML(path string) ([]byte, error) {
	resp, body, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return body, nil
}

// CreateTestProject creates a temporary project directory for testing.
func (e *TestEnv) CreateTestProject(name string) (string, error) {
	projectDir := filepath.Join(e.DataDir, "test-projects", name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("create project dir: %w", err)
	}

	// Create a sample Go file
	goFile := filepath.Join(projectDir, "main.go")
	goContent := `package main

import "fmt"

// HelloWorld prints a greeting message.
func HelloWorld() {
	fmt.Println("Hello, World!")
}

// Add adds two numbers together.
func Add(a, b int) int {
	return a + b
}

func main() {
	HelloWorld()
	fmt.Println(Add(1, 2))
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		return "", fmt.Errorf("write go file: %w", err)
	}

	// Create go.mod
	goMod := filepath.Join(projectDir, "go.mod")
	modContent := fmt.Sprintf("module %s\n\ngo 1.21\n", name)
	if err := os.WriteFile(goMod, []byte(modContent), 0644); err != nil {
		return "", fmt.Errorf("write go.mod: %w", err)
	}

	return projectDir, nil
}

// Cleanup removes the test environment data (optional - results are kept).
func (e *TestEnv) Cleanup() {
	e.Stop()
	// Note: We don't remove resultsDir so test results are preserved
}

// RunWithRetry runs a function with retries.
func RunWithRetry(t *testing.T, attempts int, delay time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
			t.Logf("Attempt %d failed: %v", i+1, err)
			time.Sleep(delay)
		}
	}
	return lastErr
}

// WaitFor waits for a condition to become true.
func WaitFor(timeout time.Duration, check func() bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if check() {
				return true
			}
		}
	}
}

// AssertJSON parses JSON response and returns the parsed map.
func AssertJSON(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nData: %s", err, string(data))
	}
	return result
}

// AssertJSONArray parses JSON array response.
func AssertJSONArray(t *testing.T, data []byte) []map[string]interface{} {
	t.Helper()
	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JSON array: %v\nData: %s", err, string(data))
	}
	return result
}

// AssertContains checks if a string contains a substring.
func AssertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain %q, got: %s", substr, s)
	}
}

// AssertStatusCode checks the HTTP response status code.
func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp == nil {
		t.Errorf("Expected status %d, but response was nil", expected)
		return
	}
	if resp.StatusCode != expected {
		t.Errorf("Expected status %d, got %d", expected, resp.StatusCode)
	}
}

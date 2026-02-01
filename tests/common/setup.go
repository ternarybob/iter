// Package common provides shared test utilities for iter-service tests.
// This file provides test environment setup that builds and runs the service.
package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMode determines how the service is run.
type TestMode int

const (
	// ModeLocal runs the service as a local process.
	ModeLocal TestMode = iota
	// ModeDocker runs the service in a Docker container.
	ModeDocker
)

// TestSetup manages the test environment lifecycle for a test file.
// Use in TestMain to build, start, run tests, and cleanup.
type TestSetup struct {
	Mode        TestMode
	BinaryPath  string
	ServiceEnv  *TestEnv
	projectRoot string
	mu          sync.Mutex

	// Docker-specific
	ctx             context.Context
	cancel          context.CancelFunc
	network         *testcontainers.DockerNetwork
	iterContainer   testcontainers.Container
	claudeContainer testcontainers.Container
}

// NewTestSetup creates a new test setup for local service testing.
// Call this in TestMain before m.Run().
func NewTestSetup() *TestSetup {
	return &TestSetup{
		Mode:        ModeLocal,
		projectRoot: getProjectRoot(),
	}
}

// NewDockerTestSetup creates a new test setup for Docker-based testing.
// This builds the binary, creates Docker containers, and runs tests.
func NewDockerTestSetup() *TestSetup {
	return &TestSetup{
		Mode:        ModeDocker,
		projectRoot: getProjectRoot(),
	}
}

// Run executes the full test lifecycle:
// 1. Build binary
// 2. Start service (local or Docker)
// 3. Run tests
// 4. Cleanup
func (s *TestSetup) Run(m *testing.M, testType, testName string) int {
	// Build binary
	if err := s.Build(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
		return 1
	}

	// Start service based on mode
	var err error
	if s.Mode == ModeDocker {
		err = s.startDocker(testType, testName)
	} else {
		err = s.startLocal(testType, testName)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start service: %v\n", err)
		return 1
	}

	// Run tests
	code := m.Run()

	// Cleanup
	s.Cleanup()

	return code
}

// Build compiles the iter-service binary to tests/bin/.
func (s *TestSetup) Build() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	binDir := filepath.Join(s.projectRoot, "tests", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	s.BinaryPath = filepath.Join(binDir, "iter-service")

	// Get version
	version := "dev"
	if data, err := os.ReadFile(filepath.Join(s.projectRoot, ".version")); err == nil {
		version = string(data)
	}

	fmt.Printf("Building iter-service (version: %s)...\n", version)

	// Build
	cmd := exec.Command("go", "build",
		"-ldflags", fmt.Sprintf("-X main.version=%s", version),
		"-o", s.BinaryPath,
		"./cmd/iter-service",
	)
	cmd.Dir = s.projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("Built: %s\n", s.BinaryPath)
	return nil
}

// startLocal starts the service as a local process.
func (s *TestSetup) startLocal(testType, testName string) error {
	s.ServiceEnv = s.createEnv(testType, testName)

	if err := s.ServiceEnv.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	fmt.Printf("Service started at %s\n", s.ServiceEnv.BaseURL)
	return nil
}

// startDocker starts the service in a Docker container.
func (s *TestSetup) startDocker(testType, testName string) error {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	// Create results directory
	resultsDir := filepath.Join(s.projectRoot, "tests", "results", testType, testName)
	os.RemoveAll(resultsDir)
	os.MkdirAll(resultsDir, 0755)

	// Create data directory
	dataDir := filepath.Join(resultsDir, "data")
	os.MkdirAll(dataDir, 0755)

	// Build Docker image
	fmt.Println("Building Docker image...")
	dockerDir := filepath.Join(s.projectRoot, "tests", "docker")
	cmd := exec.Command("docker", "compose", "-f", filepath.Join(dockerDir, "docker-compose.yml"), "build", "iter")
	cmd.Dir = s.projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build docker image: %w", err)
	}

	// Create network
	var err error
	s.network, err = network.New(s.ctx, network.WithDriver("bridge"))
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}

	// Start iter container
	fmt.Println("Starting iter container...")
	iterReq := testcontainers.ContainerRequest{
		Image:        "docker-iter:latest",
		ExposedPorts: []string{"19000/tcp"},
		Networks:     []string{s.network.Name},
		NetworkAliases: map[string][]string{
			s.network.Name: {"iter"},
		},
		Env: map[string]string{
			"GOOGLE_GEMINI_API_KEY": os.Getenv("GOOGLE_GEMINI_API_KEY"),
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("19000/tcp").WithStartupTimeout(60 * time.Second),
	}

	s.iterContainer, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: iterReq,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start iter container: %w", err)
	}

	// Get mapped port
	mappedPort, err := s.iterContainer.MappedPort(s.ctx, "19000")
	if err != nil {
		return fmt.Errorf("get mapped port: %w", err)
	}

	host, err := s.iterContainer.Host(s.ctx)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// Create TestEnv pointing to Docker service
	s.ServiceEnv = &TestEnv{
		Name:       testName,
		Type:       testType,
		DataDir:    dataDir,
		ResultsDir: resultsDir,
		Port:       0, // Not used for Docker
		BaseURL:    baseURL,
		external:   true, // Don't try to stop process
	}

	fmt.Printf("Docker service started at %s (internal: http://iter:19000)\n", baseURL)
	return nil
}

// StartClaudeContainer starts the Claude test runner container (for Docker mode).
func (s *TestSetup) StartClaudeContainer() error {
	if s.Mode != ModeDocker {
		return fmt.Errorf("Claude container only available in Docker mode")
	}

	fmt.Println("Starting Claude container...")
	claudeReq := testcontainers.ContainerRequest{
		Image:    "docker-claude:latest",
		Networks: []string{s.network.Name},
		NetworkAliases: map[string][]string{
			s.network.Name: {"claude"},
		},
		Env: map[string]string{
			"ITER_BASE_URL":       "http://iter:19000",
			"HOME":                "/home/testuser",
			"CHROME_BIN":          "/usr/bin/chromium",
			"CHROMEDP_NO_SANDBOX": "true",
		},
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"echo", "ready"}).WithStartupTimeout(30 * time.Second),
	}

	var err error
	s.claudeContainer, err = testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: claudeReq,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start claude container: %w", err)
	}

	fmt.Println("Claude container started")
	return nil
}

// ExecInClaude executes a command in the Claude container.
func (s *TestSetup) ExecInClaude(cmd ...string) (int, string, error) {
	if s.claudeContainer == nil {
		return -1, "", fmt.Errorf("claude container not started")
	}

	exitCode, reader, err := s.claudeContainer.Exec(s.ctx, cmd)
	if err != nil {
		return -1, "", err
	}

	output := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			output = append(output, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return exitCode, string(output), nil
}

// ExecInIter executes a command in the iter container.
func (s *TestSetup) ExecInIter(cmd ...string) (int, string, error) {
	if s.iterContainer == nil {
		return -1, "", fmt.Errorf("iter container not started")
	}

	exitCode, reader, err := s.iterContainer.Exec(s.ctx, cmd)
	if err != nil {
		return -1, "", err
	}

	output := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			output = append(output, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return exitCode, string(output), nil
}

// CopyToIter copies a file to the iter container.
func (s *TestSetup) CopyToIter(content []byte, containerPath string) error {
	if s.iterContainer == nil {
		return fmt.Errorf("iter container not started")
	}
	return s.iterContainer.CopyToContainer(s.ctx, content, containerPath, 0644)
}

// InternalBaseURL returns the internal Docker network URL for iter.
// Use this when making requests from the Claude container.
func (s *TestSetup) InternalBaseURL() string {
	if s.Mode == ModeDocker {
		return "http://iter:19000"
	}
	return s.ServiceEnv.BaseURL
}

// createEnv creates a TestEnv for local mode.
func (s *TestSetup) createEnv(testType, testName string) *TestEnv {
	// Create results directory
	resultsDir := filepath.Join(s.projectRoot, "tests", "results", testType, testName)
	os.RemoveAll(resultsDir)
	os.MkdirAll(resultsDir, 0755)

	// Create data directory
	dataDir := filepath.Join(resultsDir, "data")
	os.MkdirAll(dataDir, 0755)

	port := allocatePort()

	env := &TestEnv{
		Name:       testName,
		Type:       testType,
		DataDir:    dataDir,
		ConfigPath: filepath.Join(dataDir, "config.toml"),
		ResultsDir: resultsDir,
		Port:       port,
		BaseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		external:   false,
	}

	// Write config
	env.writeConfigForSetup()

	return env
}

// writeConfigForSetup writes config without requiring *testing.T.
func (e *TestEnv) writeConfigForSetup() error {
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
output = ["stdout"]

[index]
debounce_ms = 100
watch_enabled = true
`, e.Port, e.DataDir, e.DataDir)

	return os.WriteFile(e.ConfigPath, []byte(config), 0644)
}

// Cleanup stops the service and cleans up resources.
func (s *TestSetup) Cleanup() {
	fmt.Println("Cleaning up test environment...")

	if s.Mode == ModeDocker {
		if s.claudeContainer != nil {
			s.claudeContainer.Terminate(s.ctx)
		}
		if s.iterContainer != nil {
			s.iterContainer.Terminate(s.ctx)
		}
		if s.network != nil {
			s.network.Remove(s.ctx)
		}
		if s.cancel != nil {
			s.cancel()
		}
	} else {
		if s.ServiceEnv != nil {
			s.ServiceEnv.Stop()
		}
	}
}

// Env returns the test environment for use in individual tests.
func (s *TestSetup) Env() *TestEnv {
	return s.ServiceEnv
}

// BaseURL returns the service base URL (for external access).
func (s *TestSetup) BaseURL() string {
	if s.ServiceEnv != nil {
		return s.ServiceEnv.BaseURL
	}
	return ""
}

// Context returns the context (for Docker operations).
func (s *TestSetup) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// WaitForHealthy waits for the service to be healthy.
func (s *TestSetup) WaitForHealthy(timeout time.Duration) error {
	if s.ServiceEnv != nil {
		return s.ServiceEnv.waitForReady(timeout)
	}
	return fmt.Errorf("service not started")
}

// Container support for integration tests.
// Tests can build images, start containers, execute commands, and collect results.
package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	IterImage   = "iter-test:latest"
	ClaudeImage = "claude-test:latest"
)

var (
	buildOnce sync.Once
	buildErr  error
)

// BuildImages builds the Docker images for testing.
// Safe to call multiple times - only builds once per test run.
func BuildImages(t *testing.T) error {
	t.Helper()

	buildOnce.Do(func() {
		t.Log("Building Docker images...")
		root := getProjectRoot()

		// Build iter image
		t.Log("Building iter-test image...")
		cmd := exec.Command("docker", "build",
			"-t", IterImage,
			"-f", filepath.Join(root, "tests/docker/Dockerfile.iter"),
			root)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("build iter image: %w", err)
			return
		}

		// Build claude image
		t.Log("Building claude-test image...")
		cmd = exec.Command("docker", "build",
			"-t", ClaudeImage,
			"-f", filepath.Join(root, "tests/docker/Dockerfile.claude"),
			root)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("build claude image: %w", err)
			return
		}

		t.Log("Docker images built successfully")
	})

	return buildErr
}

// ForceBuildImages rebuilds images even if already built.
// Use this for a completely clean environment.
func ForceBuildImages(t *testing.T) error {
	t.Helper()

	// Reset the once so we rebuild
	buildOnce = sync.Once{}
	buildErr = nil

	return BuildImages(t)
}

// Env holds the container test environment.
type Env struct {
	T          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	network    *testcontainers.DockerNetwork
	iter       testcontainers.Container
	claude     testcontainers.Container
	ResultsDir string
	iterURL    string
}

// NewEnv creates a new container test environment.
// Call BuildImages() first if you want to ensure fresh images.
// testName is used for per-test result directories.
func NewEnv(t *testing.T, testName ...string) (*Env, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	// Create results directory: tests/results/containers/{testName}/
	// Per-test directories - each run overwrites previous results
	root := getProjectRoot()
	name := "default"
	if len(testName) > 0 && testName[0] != "" {
		name = testName[0]
	}
	resultsDir := filepath.Join(root, "tests", "results", "containers", name)

	// Remove old results to ensure clean state
	os.RemoveAll(resultsDir)

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("create results dir: %w", err)
	}

	// Create network
	net, err := network.New(ctx, network.WithDriver("bridge"))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create network: %w", err)
	}

	return &Env{
		T:          t,
		ctx:        ctx,
		cancel:     cancel,
		network:    net,
		ResultsDir: resultsDir,
	}, nil
}

// StartIter starts the iter-service container.
func (e *Env) StartIter() error {
	req := testcontainers.ContainerRequest{
		Image:        IterImage,
		ExposedPorts: []string{"19000/tcp"},
		Networks:     []string{e.network.Name},
		NetworkAliases: map[string][]string{
			e.network.Name: {"iter"},
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("19000/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(e.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start iter: %w", err)
	}

	e.iter = container
	e.iterURL = "http://iter:19000"
	e.T.Log("iter-service started")
	return nil
}

// StartClaude starts the Claude test runner container.
func (e *Env) StartClaude() error {
	req := testcontainers.ContainerRequest{
		Image:    ClaudeImage,
		Networks: []string{e.network.Name},
		NetworkAliases: map[string][]string{
			e.network.Name: {"claude"},
		},
		Env: map[string]string{
			"ITER_BASE_URL":       e.iterURL,
			"HOME":                "/home/testuser",
			"CHROME_BIN":          "/usr/bin/chromium",
			"CHROMEDP_NO_SANDBOX": "true",
		},
		Cmd:        []string{"tail", "-f", "/dev/null"},
		WaitingFor: wait.ForExec([]string{"echo", "ready"}).WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(e.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	e.claude = container
	e.T.Log("Claude runner started")
	return nil
}

// Start starts both containers.
func (e *Env) Start() error {
	if err := e.StartIter(); err != nil {
		return err
	}
	return e.StartClaude()
}

// IterURL returns the iter-service URL accessible from the claude container.
func (e *Env) IterURL() string {
	return e.iterURL
}

// CopyCredentials copies Claude credentials into the container.
func (e *Env) CopyCredentials() error {
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		e.T.Log("No Claude credentials found")
		return nil
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	// Create directory
	e.Exec("mkdir", "-p", "/home/testuser/.claude")

	// Copy file
	if err := e.claude.CopyToContainer(e.ctx, data, "/home/testuser/.claude/.credentials.json", 0644); err != nil {
		return fmt.Errorf("copy credentials: %w", err)
	}

	// Fix ownership
	e.Exec("chown", "-R", "testuser:testuser", "/home/testuser/.claude")

	e.T.Log("Credentials copied")
	return nil
}

// ConfigureMCP configures the iter MCP server in Claude.
func (e *Env) ConfigureMCP() error {
	// Remove existing
	e.Exec("claude", "mcp", "remove", "iter")

	// Add new
	exitCode, output, err := e.Exec("claude", "mcp", "add", "--transport", "http", "iter", e.iterURL+"/mcp/v1")
	if err != nil {
		return fmt.Errorf("add MCP: %w", err)
	}
	if exitCode != 0 && !strings.Contains(output, "already") {
		return fmt.Errorf("add MCP failed: %s", output)
	}

	// Verify
	_, output, _ = e.Exec("claude", "mcp", "list")
	e.T.Logf("MCP configured: %s", strings.TrimSpace(output))

	return nil
}

// Exec executes a command in the Claude container.
func (e *Env) Exec(cmd ...string) (int, string, error) {
	if e.claude == nil {
		return -1, "", fmt.Errorf("claude container not started")
	}

	exitCode, reader, err := e.claude.Exec(e.ctx, cmd)
	if err != nil {
		return -1, "", err
	}

	output, _ := io.ReadAll(reader)
	return exitCode, string(output), nil
}

// ExecBash executes a bash command in the Claude container.
func (e *Env) ExecBash(script string) (int, string, error) {
	return e.Exec("bash", "-c", script)
}

// ExecIter executes a command in the iter container.
func (e *Env) ExecIter(cmd ...string) (int, string, error) {
	if e.iter == nil {
		return -1, "", fmt.Errorf("iter container not started")
	}

	exitCode, reader, err := e.iter.Exec(e.ctx, cmd)
	if err != nil {
		return -1, "", err
	}

	output, _ := io.ReadAll(reader)
	return exitCode, string(output), nil
}

// RunGoTest runs Go tests in the Claude container.
func (e *Env) RunGoTest(testPath string, args ...string) (int, string, error) {
	cmdArgs := []string{"go", "test", "-v", "-timeout", "300s"}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, testPath)

	script := fmt.Sprintf("cd /app && %s", strings.Join(cmdArgs, " "))
	return e.ExecBash(script)
}

// RunClaude runs a Claude query and returns the output.
func (e *Env) RunClaude(prompt string) (string, error) {
	script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && claude -p --dangerously-skip-permissions --max-turns 10 %q`, prompt)
	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		e.T.Logf("Claude returned exit code %d", exitCode)
	}
	return output, nil
}

// SaveResult saves data to the results directory.
func (e *Env) SaveResult(name string, data []byte) error {
	path := filepath.Join(e.ResultsDir, name)
	return os.WriteFile(path, data, 0644)
}

// CopyResults copies results from the container.
func (e *Env) CopyResults() error {
	if e.claude == nil {
		return nil
	}

	reader, err := e.claude.CopyFileFromContainer(e.ctx, "/home/testuser/results")
	if err != nil {
		return nil // May not exist
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	return e.SaveResult("container-results.tar", data)
}

// Cleanup stops containers and cleans up resources.
func (e *Env) Cleanup() {
	e.CopyResults()

	if e.claude != nil {
		e.claude.Terminate(e.ctx)
	}
	if e.iter != nil {
		e.iter.Terminate(e.ctx)
	}
	if e.network != nil {
		e.network.Remove(e.ctx)
	}
	if e.cancel != nil {
		e.cancel()
	}
}

// RequireCredentials skips the test if Claude credentials are not available.
func (e *Env) RequireCredentials() {
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		e.T.Skip("No Claude credentials - skipping")
	}
}

// Package docker provides container-based integration tests for iter-service.
// Uses testcontainers-go to manage iter and Claude containers.
//
// Prerequisites:
//   - Build images first: docker compose -f tests/docker/docker-compose.yml build
//   - Or run: go test -tags=docker ./tests/docker/... -build-images
package docker

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var buildImages = flag.Bool("build-images", false, "Build Docker images before running tests")

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	flag.Parse()

	if *buildImages {
		fmt.Println("Building Docker images...")
		cmd := exec.Command("docker", "compose", "-f", "tests/docker/docker-compose.yml", "build")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Failed to build images: %v\n", err)
			os.Exit(1)
		}
	}

	// Check if images exist
	for _, img := range []string{"docker-iter:latest", "docker-claude:latest"} {
		cmd := exec.Command("docker", "image", "inspect", img)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Image %s not found. Build with:\n", img)
			fmt.Println("  docker compose -f tests/docker/docker-compose.yml build")
			fmt.Println("Or run with: go test ./tests/docker/... -build-images")
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

// TestEnv holds the test environment with both containers.
type TestEnv struct {
	t          *testing.T
	ctx        context.Context
	network    *testcontainers.DockerNetwork
	iter       testcontainers.Container
	claude     testcontainers.Container
	resultsDir string
}

// NewTestEnv creates a new container-based test environment.
func NewTestEnv(t *testing.T) (*TestEnv, error) {
	t.Helper()
	ctx := context.Background()

	// Create results directory
	resultsDir := filepath.Join("tests", "results", "docker", time.Now().Format("2006-01-02_15-04-05"))
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("create results dir: %w", err)
	}

	// Create network for container communication
	net, err := network.New(ctx, network.WithDriver("bridge"))
	if err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	env := &TestEnv{
		t:          t,
		ctx:        ctx,
		network:    net,
		resultsDir: resultsDir,
	}

	return env, nil
}

// StartIter starts the iter-service container.
// Uses pre-built image "docker-iter:latest" (build with: docker compose -f tests/docker/docker-compose.yml build iter)
func (e *TestEnv) StartIter() error {
	req := testcontainers.ContainerRequest{
		Image:        "docker-iter:latest",
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
		return fmt.Errorf("start iter container: %w", err)
	}

	e.iter = container
	e.t.Log("iter-service container started")
	return nil
}

// StartClaude starts the Claude test runner container.
// Uses pre-built image "docker-claude:latest" (build with: docker compose -f tests/docker/docker-compose.yml build claude)
func (e *TestEnv) StartClaude() error {
	req := testcontainers.ContainerRequest{
		Image:    "docker-claude:latest",
		Networks: []string{e.network.Name},
		NetworkAliases: map[string][]string{
			e.network.Name: {"claude"},
		},
		Env: map[string]string{
			"ITER_BASE_URL":       "http://iter:19000",
			"HOME":                "/home/testuser",
			"CHROME_BIN":          "/usr/bin/chromium",
			"CHROMEDP_NO_SANDBOX": "true",
		},
		Cmd:        []string{"tail", "-f", "/dev/null"}, // Keep container running
		WaitingFor: wait.ForExec([]string{"echo", "ready"}).WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(e.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start claude container: %w", err)
	}

	e.claude = container
	e.t.Log("Claude container started")
	return nil
}

// CopyCredentials copies Claude credentials into the container.
func (e *TestEnv) CopyCredentials() error {
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		e.t.Log("No Claude credentials found, skipping")
		return nil
	}

	// Read credentials
	data, err := os.ReadFile(credPath)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	// Create .claude directory in container
	_, _, err = e.claude.Exec(e.ctx, []string{"mkdir", "-p", "/home/testuser/.claude"})
	if err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	// Copy credentials file
	err = e.claude.CopyToContainer(e.ctx, data, "/home/testuser/.claude/.credentials.json", 0644)
	if err != nil {
		return fmt.Errorf("copy credentials: %w", err)
	}

	// Fix ownership
	_, _, err = e.claude.Exec(e.ctx, []string{"chown", "-R", "testuser:testuser", "/home/testuser/.claude"})
	if err != nil {
		return fmt.Errorf("fix ownership: %w", err)
	}

	e.t.Log("Claude credentials copied")
	return nil
}

// ConfigureMCP adds the iter MCP server to Claude.
func (e *TestEnv) ConfigureMCP() error {
	// Remove existing config
	e.Exec("claude", "mcp", "remove", "iter")

	// Add iter MCP server
	exitCode, output, err := e.Exec("claude", "mcp", "add", "--transport", "http", "iter", "http://iter:19000/mcp/v1")
	if err != nil {
		return fmt.Errorf("add MCP server: %w", err)
	}
	if exitCode != 0 && !strings.Contains(output, "already") {
		return fmt.Errorf("add MCP server failed: %s", output)
	}

	// Verify
	_, output, _ = e.Exec("claude", "mcp", "list")
	e.t.Logf("MCP servers: %s", output)

	return nil
}

// Exec executes a command in the Claude container.
func (e *TestEnv) Exec(cmd ...string) (int, string, error) {
	exitCode, reader, err := e.claude.Exec(e.ctx, cmd)
	if err != nil {
		return -1, "", err
	}

	output, _ := io.ReadAll(reader)
	return exitCode, string(output), nil
}

// ExecBash executes a bash script in the Claude container.
func (e *TestEnv) ExecBash(script string) (int, string, error) {
	return e.Exec("bash", "-c", script)
}

// RunGoTest runs Go tests in the Claude container.
func (e *TestEnv) RunGoTest(testPath string, args ...string) (int, string, error) {
	cmdArgs := []string{"go", "test", "-v", "-timeout", "300s"}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, testPath)

	script := fmt.Sprintf("cd /app && %s", strings.Join(cmdArgs, " "))
	return e.ExecBash(script)
}

// CopyResults copies test results from the container.
func (e *TestEnv) CopyResults() error {
	// Copy from container results directory
	reader, err := e.claude.CopyFileFromContainer(e.ctx, "/home/testuser/results")
	if err != nil {
		// Results dir might not exist
		e.t.Log("No results to copy from /home/testuser/results")
		return nil
	}
	defer reader.Close()

	// Write to local results
	data, _ := io.ReadAll(reader)
	localPath := filepath.Join(e.resultsDir, "container-results.tar")
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return fmt.Errorf("write results: %w", err)
	}

	e.t.Logf("Results saved to %s", e.resultsDir)
	return nil
}

// SaveOutput saves command output to results directory.
func (e *TestEnv) SaveOutput(name, output string) error {
	path := filepath.Join(e.resultsDir, name)
	return os.WriteFile(path, []byte(output), 0644)
}

// Cleanup stops and removes containers and network.
func (e *TestEnv) Cleanup() {
	if e.claude != nil {
		e.CopyResults()
		e.claude.Terminate(e.ctx)
	}
	if e.iter != nil {
		e.iter.Terminate(e.ctx)
	}
	if e.network != nil {
		e.network.Remove(e.ctx)
	}
}

// --- Tests ---

// TestMCPIntegration tests MCP functionality with real containers.
func TestMCPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	env, err := NewTestEnv(t)
	if err != nil {
		t.Fatalf("Failed to create test env: %v", err)
	}
	defer env.Cleanup()

	// Start iter-service
	t.Log("Starting iter-service container...")
	if err := env.StartIter(); err != nil {
		t.Fatalf("Failed to start iter: %v", err)
	}

	// Start Claude container
	t.Log("Starting Claude container...")
	if err := env.StartClaude(); err != nil {
		t.Fatalf("Failed to start Claude: %v", err)
	}

	// Copy credentials
	if err := env.CopyCredentials(); err != nil {
		t.Fatalf("Failed to copy credentials: %v", err)
	}

	// Configure MCP
	t.Log("Configuring MCP...")
	if err := env.ConfigureMCP(); err != nil {
		t.Fatalf("Failed to configure MCP: %v", err)
	}

	// Run subtests
	t.Run("HealthCheck", func(t *testing.T) {
		exitCode, output, err := env.ExecBash("curl -s http://iter:19000/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("Health check returned %d: %s", exitCode, output)
		}
		if !strings.Contains(output, "ok") {
			t.Errorf("Unexpected health response: %s", output)
		}
		t.Logf("Health: %s", output)
	})

	t.Run("MCPInitialize", func(t *testing.T) {
		script := `curl -s -X POST -H "Content-Type: application/json" \
			-d '{"jsonrpc":"2.0","id":1,"method":"initialize"}' \
			http://iter:19000/mcp/v1`
		exitCode, output, err := env.ExecBash(script)
		if err != nil {
			t.Fatalf("MCP initialize failed: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("MCP initialize returned %d: %s", exitCode, output)
		}
		if !strings.Contains(output, "iter-service") {
			t.Errorf("Unexpected MCP response: %s", output)
		}
		env.SaveOutput("mcp-initialize.json", output)
		t.Logf("MCP Initialize: %s", output)
	})

	t.Run("MCPToolsList", func(t *testing.T) {
		script := `curl -s -X POST -H "Content-Type: application/json" \
			-d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
			http://iter:19000/mcp/v1`
		exitCode, output, err := env.ExecBash(script)
		if err != nil {
			t.Fatalf("MCP tools/list failed: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("MCP tools/list returned %d: %s", exitCode, output)
		}
		if !strings.Contains(output, "list_projects") {
			t.Errorf("Expected list_projects tool, got: %s", output)
		}
		env.SaveOutput("mcp-tools-list.json", output)
		t.Logf("MCP Tools: %s", output[:min(200, len(output))])
	})

	t.Run("ClaudeToolDiscovery", func(t *testing.T) {
		// Skip if no credentials
		credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
		if _, err := os.Stat(credPath); os.IsNotExist(err) {
			t.Skip("No Claude credentials")
		}

		script := `export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 5 \
			"What MCP tools are available from the iter server? List them briefly."`
		exitCode, output, err := env.ExecBash(script)
		if err != nil {
			t.Fatalf("Claude query failed: %v", err)
		}
		env.SaveOutput("claude-tool-discovery.txt", output)

		if exitCode != 0 {
			t.Logf("Claude returned %d (may be expected)", exitCode)
		}

		if strings.TrimSpace(output) == "" {
			t.Fatal("Claude returned empty output")
		}

		outputLower := strings.ToLower(output)
		if strings.Contains(outputLower, "no mcp") || strings.Contains(outputLower, "not configured") {
			t.Fatalf("Claude reports no MCP access: %s", output)
		}

		t.Logf("Claude output: %s", output[:min(300, len(output))])
	})
}

// TestAPIIntegration runs API tests in containers.
func TestAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	env, err := NewTestEnv(t)
	if err != nil {
		t.Fatalf("Failed to create test env: %v", err)
	}
	defer env.Cleanup()

	// Start iter-service
	if err := env.StartIter(); err != nil {
		t.Fatalf("Failed to start iter: %v", err)
	}

	// Start Claude container (for running Go tests)
	if err := env.StartClaude(); err != nil {
		t.Fatalf("Failed to start Claude: %v", err)
	}

	// Run API tests in container
	t.Run("APITests", func(t *testing.T) {
		exitCode, output, err := env.RunGoTest("./tests/api/...")
		env.SaveOutput("api-tests.txt", output)

		if err != nil {
			t.Fatalf("API tests error: %v", err)
		}
		if exitCode != 0 {
			t.Fatalf("API tests failed:\n%s", output)
		}
		t.Log("API tests passed")
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

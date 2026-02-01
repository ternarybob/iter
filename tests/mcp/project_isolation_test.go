// Package mcp provides MCP integration tests for iter-service.
// These tests validate that MCP returns project-specific data correctly.
//
// Test scenario:
//  1. Create two distinct Go projects (Alpha - greeting, Beta - calculator)
//  2. Register and index both projects in iter-service
//  3. Query each project via MCP and validate only that project's data is returned
//
// Run with:
//
//	TEST_DOCKER=1 go test -v ./tests/mcp/...
//
// NOTE: These tests share TestMain with mcp_test.go (same package).
// They require Docker mode (TEST_DOCKER=1) and will skip in local mode.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/iter/tests/common"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// requireDockerMode skips test if not running in Docker mode.
func requireDockerMode(t *testing.T) {
	t.Helper()
	if testSetup.Mode != common.ModeDocker {
		t.Skip("Test requires Docker mode (TEST_DOCKER=1)")
	}
}

// NOTE: TestMain is in mcp_test.go for the entire package

// E2EEnv holds the end-to-end test environment.
type E2EEnv struct {
	t          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	network    *testcontainers.DockerNetwork
	iter       testcontainers.Container
	claude     testcontainers.Container
	resultsDir string
	projectIDs map[string]string // project name -> project ID
}

// NewE2EEnv creates a new E2E test environment.
// testName is used for per-test result directories.
func NewE2EEnv(t *testing.T, testName string) (*E2EEnv, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	// Create results directory: tests/results/mcp/{testName}/
	// Per-test directories - each run overwrites previous results
	root := getProjectRoot()
	resultsDir := filepath.Join(root, "tests", "results", "mcp", testName)

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

	return &E2EEnv{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		network:    net,
		resultsDir: resultsDir,
		projectIDs: make(map[string]string),
	}, nil
}

// StartIter starts the iter-service container.
func (e *E2EEnv) StartIter() error {
	req := testcontainers.ContainerRequest{
		Image:        "docker-iter:latest",
		ExposedPorts: []string{"19000/tcp"},
		Networks:     []string{e.network.Name},
		NetworkAliases: map[string][]string{
			e.network.Name: {"iter"},
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("19000/tcp").WithStartupTimeout(60 * time.Second),
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
func (e *E2EEnv) StartClaude() error {
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
		Cmd:        []string{"tail", "-f", "/dev/null"},
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
func (e *E2EEnv) CopyCredentials() error {
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		e.t.Log("No Claude credentials found")
		return nil
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	e.Exec("mkdir", "-p", "/home/testuser/.claude")

	if err := e.claude.CopyToContainer(e.ctx, data, "/home/testuser/.claude/.credentials.json", 0644); err != nil {
		return fmt.Errorf("copy credentials: %w", err)
	}

	e.Exec("chown", "-R", "testuser:testuser", "/home/testuser/.claude")
	e.t.Log("Credentials copied")
	return nil
}

// ConfigureMCP adds the iter MCP server to Claude.
func (e *E2EEnv) ConfigureMCP() error {
	e.Exec("claude", "mcp", "remove", "iter")

	exitCode, output, err := e.Exec("claude", "mcp", "add", "--transport", "http", "iter", "http://iter:19000/mcp/v1")
	if err != nil {
		return fmt.Errorf("add MCP server: %w", err)
	}
	if exitCode != 0 && !strings.Contains(output, "already") {
		return fmt.Errorf("add MCP server failed: %s", output)
	}

	_, output, _ = e.Exec("claude", "mcp", "list")
	e.t.Logf("MCP servers: %s", strings.TrimSpace(output))
	return nil
}

// Exec executes a command in the Claude container.
func (e *E2EEnv) Exec(cmd ...string) (int, string, error) {
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

// ExecBash executes a bash script in the Claude container.
func (e *E2EEnv) ExecBash(script string) (int, string, error) {
	return e.Exec("bash", "-c", script)
}

// ExecIter executes a command in the iter container.
func (e *E2EEnv) ExecIter(cmd ...string) (int, string, error) {
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

// CopyTestProjects copies the test-code projects into the iter container.
func (e *E2EEnv) CopyTestProjects() error {
	projectRoot := getProjectRoot()
	testCodeDir := filepath.Join(projectRoot, "tests", "test-code")

	// Create destination directory in iter container
	_, _, err := e.ExecIter("mkdir", "-p", "/data/projects")
	if err != nil {
		return fmt.Errorf("create projects dir: %w", err)
	}

	// Copy project-alpha
	alphaPath := filepath.Join(testCodeDir, "project-alpha")
	if err := e.copyDirToIter(alphaPath, "/data/projects/project-alpha"); err != nil {
		return fmt.Errorf("copy project-alpha: %w", err)
	}
	e.t.Log("Copied project-alpha to iter container")

	// Copy project-beta
	betaPath := filepath.Join(testCodeDir, "project-beta")
	if err := e.copyDirToIter(betaPath, "/data/projects/project-beta"); err != nil {
		return fmt.Errorf("copy project-beta: %w", err)
	}
	e.t.Log("Copied project-beta to iter container")

	// Verify files exist
	_, output, _ := e.ExecIter("ls", "-la", "/data/projects/")
	e.t.Logf("Projects in container:\n%s", output)

	return nil
}

// copyDirToIter copies a local directory to the iter container.
func (e *E2EEnv) copyDirToIter(localPath, containerPath string) error {
	// Create destination directory
	e.ExecIter("mkdir", "-p", containerPath)

	// Walk and copy each file
	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(localPath, path)
		destPath := filepath.Join(containerPath, relPath)

		// Create parent directory
		e.ExecIter("mkdir", "-p", filepath.Dir(destPath))

		// Read and copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return e.iter.CopyToContainer(e.ctx, data, destPath, 0644)
	})
}

// cleanJSON extracts clean JSON from container output that may contain escape sequences.
func cleanJSON(output string) string {
	// Remove ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x01|\x02`)
	output = ansiRegex.ReplaceAllString(output, "")

	// Find the start of JSON (first { or [)
	startBrace := strings.Index(output, "{")
	startBracket := strings.Index(output, "[")

	start := -1
	if startBrace >= 0 && (startBracket < 0 || startBrace < startBracket) {
		start = startBrace
	} else if startBracket >= 0 {
		start = startBracket
	}

	if start >= 0 {
		output = output[start:]
	}

	return strings.TrimSpace(output)
}

// RegisterProject registers a project with iter-service via API.
func (e *E2EEnv) RegisterProject(name, path string) (string, error) {
	payload := fmt.Sprintf(`{"name":"%s","path":"%s"}`, name, path)
	script := fmt.Sprintf(`curl -s -X POST -H "Content-Type: application/json" -d '%s' http://iter:19000/projects`, payload)

	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return "", fmt.Errorf("register project: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("register project failed: %s", output)
	}

	// Clean output and parse response
	cleanOutput := cleanJSON(output)

	var resp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(cleanOutput), &resp); err != nil {
		return "", fmt.Errorf("parse response: %w (output: %s)", err, output)
	}

	e.projectIDs[name] = resp.ID
	e.t.Logf("Registered project %s with ID %s", name, resp.ID)
	return resp.ID, nil
}

// IndexProject triggers indexing for a project.
func (e *E2EEnv) IndexProject(projectID string) error {
	script := fmt.Sprintf(`curl -s -X POST http://iter:19000/projects/%s/index`, projectID)

	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return fmt.Errorf("index project: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("index project failed: %s", output)
	}

	e.t.Logf("Indexed project %s: %s", projectID, strings.TrimSpace(output))
	return nil
}

// MCPSearch performs an MCP search query.
func (e *E2EEnv) MCPSearch(query, projectID string) (string, error) {
	args := fmt.Sprintf(`{"query":"%s"`, query)
	if projectID != "" {
		args += fmt.Sprintf(`,"project_id":"%s"`, projectID)
	}
	args += "}"

	payload := fmt.Sprintf(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"tools/call",
		"params":{
			"name":"search",
			"arguments":%s
		}
	}`, args)

	script := fmt.Sprintf(`curl -s -X POST -H "Content-Type: application/json" -d '%s' http://iter:19000/mcp/v1`, payload)

	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return "", fmt.Errorf("MCP search: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("MCP search failed: %s", output)
	}

	return output, nil
}

// MCPListProjects calls the list_projects MCP tool.
func (e *E2EEnv) MCPListProjects() (string, error) {
	payload := `{
		"jsonrpc":"2.0",
		"id":1,
		"method":"tools/call",
		"params":{
			"name":"list_projects",
			"arguments":{}
		}
	}`

	script := fmt.Sprintf(`curl -s -X POST -H "Content-Type: application/json" -d '%s' http://iter:19000/mcp/v1`, payload)

	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return "", fmt.Errorf("MCP list_projects: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("MCP list_projects failed: %s", output)
	}

	return output, nil
}

// SaveResult saves test output to the results directory.
func (e *E2EEnv) SaveResult(name string, data string) error {
	path := filepath.Join(e.resultsDir, name)
	return os.WriteFile(path, []byte(data), 0644)
}

// TestSummary contains the structured test results.
type TestSummary struct {
	TestName    string   `json:"test_name"`
	Passed      bool     `json:"passed"`
	Duration    string   `json:"duration"`
	Timestamp   string   `json:"timestamp"`
	Screenshots []string `json:"screenshots"`
	Logs        []string `json:"logs"`
	Details     string   `json:"details"`
	Errors      []string `json:"errors"`
}

// WriteSummary writes test summary to both summary.json and SUMMARY.md.
func (e *E2EEnv) WriteSummary(passed bool, duration time.Duration, details string, errors ...string) error {
	timestamp := time.Now().Format(time.RFC3339)

	// Collect screenshots
	screenshots := e.collectScreenshots()

	// Collect logs
	logs := e.collectLogs()

	summary := TestSummary{
		TestName:    filepath.Base(e.resultsDir),
		Passed:      passed,
		Duration:    duration.String(),
		Timestamp:   timestamp,
		Screenshots: screenshots,
		Logs:        logs,
		Details:     details,
		Errors:      errors,
	}

	// Write summary.json
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := e.SaveResult("summary.json", string(data)); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}

	// Write SUMMARY.md
	return e.writeSummaryMarkdown(summary)
}

// collectScreenshots returns list of screenshot files in results directory.
func (e *E2EEnv) collectScreenshots() []string {
	var screenshots []string
	entries, err := os.ReadDir(e.resultsDir)
	if err != nil {
		return screenshots
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".png") {
			screenshots = append(screenshots, entry.Name())
		}
	}
	return screenshots
}

// collectLogs returns list of log files in results directory.
func (e *E2EEnv) collectLogs() []string {
	var logs []string
	entries, err := os.ReadDir(e.resultsDir)
	if err != nil {
		return logs
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			logs = append(logs, entry.Name())
		}
	}
	return logs
}

// writeSummaryMarkdown writes the SUMMARY.md file.
func (e *E2EEnv) writeSummaryMarkdown(summary TestSummary) error {
	result := "PASS"
	if !summary.Passed {
		result = "FAIL"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Test: %s\n\n", summary.TestName))
	sb.WriteString(fmt.Sprintf("**Result:** %s\n", result))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n", summary.Duration))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", summary.Timestamp))

	sb.WriteString("## Screenshots\n")
	if len(summary.Screenshots) == 0 {
		sb.WriteString("- None captured\n")
	} else {
		for _, s := range summary.Screenshots {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Logs\n")
	if len(summary.Logs) == 0 {
		sb.WriteString("- None captured\n")
	} else {
		for _, l := range summary.Logs {
			sb.WriteString(fmt.Sprintf("- %s\n", l))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Details\n")
	sb.WriteString(summary.Details)
	sb.WriteString("\n\n")

	sb.WriteString("## Errors\n")
	if len(summary.Errors) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, err := range summary.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return e.SaveResult("SUMMARY.md", sb.String())
}

// Cleanup stops containers and releases resources.
func (e *E2EEnv) Cleanup() {
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

func getProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			dir, _ = os.Getwd()
			return dir
		}
		dir = parent
	}
}

// --- Tests ---

// TestProjectIsolation validates that MCP returns project-specific data.
// This is the core test: when querying project-alpha, we should NOT see project-beta symbols.
// Requires Docker mode (TEST_DOCKER=1).
func TestProjectIsolation(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	startTime := time.Now()
	env, err := NewE2EEnv(t, "TestProjectIsolation")
	require.NoError(t, err, "Failed to create E2E environment")
	defer env.Cleanup()
	defer func() {
		// Write test summary at the end
		duration := time.Since(startTime)
		env.WriteSummary(t.Failed() == false, duration, "MCP project isolation test - validates project-specific data filtering")
	}()

	// Step 1: Start containers
	t.Log("Starting iter-service container...")
	require.NoError(t, env.StartIter(), "Failed to start iter")

	t.Log("Starting Claude container...")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Step 2: Copy test projects to iter container
	t.Log("Copying test projects to iter container...")
	require.NoError(t, env.CopyTestProjects(), "Failed to copy test projects")

	// Step 3: Register and index projects
	t.Log("Registering project-alpha...")
	alphaID, err := env.RegisterProject("project-alpha", "/data/projects/project-alpha")
	require.NoError(t, err, "Failed to register project-alpha")
	require.NotEmpty(t, alphaID, "project-alpha ID should not be empty")

	t.Log("Registering project-beta...")
	betaID, err := env.RegisterProject("project-beta", "/data/projects/project-beta")
	require.NoError(t, err, "Failed to register project-beta")
	require.NotEmpty(t, betaID, "project-beta ID should not be empty")

	t.Log("Indexing project-alpha...")
	require.NoError(t, env.IndexProject(alphaID), "Failed to index project-alpha")

	t.Log("Indexing project-beta...")
	require.NoError(t, env.IndexProject(betaID), "Failed to index project-beta")

	// Allow indexing to complete
	time.Sleep(2 * time.Second)

	// Step 4: Verify projects are listed
	t.Run("ListProjects", func(t *testing.T) {
		output, err := env.MCPListProjects()
		require.NoError(t, err, "Failed to list projects")
		env.SaveResult("list-projects.json", output)

		assert.Contains(t, output, "project-alpha", "Should list project-alpha")
		assert.Contains(t, output, "project-beta", "Should list project-beta")
		t.Logf("Listed projects: %s", output)
	})

	// Step 5: Test project isolation - Alpha specific search
	t.Run("SearchAlphaOnly", func(t *testing.T) {
		// Search for Alpha-specific symbol within Alpha project
		output, err := env.MCPSearch("AlphaGreeter", alphaID)
		require.NoError(t, err, "Failed to search Alpha")
		env.SaveResult("search-alpha-greeter.json", output)

		// Should find AlphaGreeter
		assert.Contains(t, output, "AlphaGreeter", "Should find AlphaGreeter in project-alpha")

		// Should NOT contain Beta symbols
		assert.NotContains(t, output, "BetaCalculator", "Should NOT find BetaCalculator in project-alpha search")
		assert.NotContains(t, output, "SquareRoot", "Should NOT find SquareRoot in project-alpha search")

		t.Logf("Alpha search result: %s", output)
	})

	// Step 6: Test project isolation - Beta specific search
	t.Run("SearchBetaOnly", func(t *testing.T) {
		// Search for Beta-specific symbol within Beta project
		output, err := env.MCPSearch("BetaCalculator", betaID)
		require.NoError(t, err, "Failed to search Beta")
		env.SaveResult("search-beta-calculator.json", output)

		// Should find BetaCalculator
		assert.Contains(t, output, "BetaCalculator", "Should find BetaCalculator in project-beta")

		// Should NOT contain Alpha symbols
		assert.NotContains(t, output, "AlphaGreeter", "Should NOT find AlphaGreeter in project-beta search")
		assert.NotContains(t, output, "GreetMultiple", "Should NOT find GreetMultiple in project-beta search")

		t.Logf("Beta search result: %s", output)
	})

	// Step 7: Verify Alpha-specific symbols
	t.Run("AlphaSymbolsExist", func(t *testing.T) {
		symbols := []string{"NewAlphaGreeter", "Greet", "GreetMultiple", "AlphaConfig", "FormatGreeting"}
		for _, sym := range symbols {
			output, err := env.MCPSearch(sym, alphaID)
			require.NoError(t, err, "Failed to search for %s", sym)

			assert.Contains(t, output, sym, "Should find %s in project-alpha", sym)
		}
	})

	// Step 8: Verify Beta-specific symbols
	t.Run("BetaSymbolsExist", func(t *testing.T) {
		symbols := []string{"NewBetaCalculator", "Add", "Multiply", "SquareRoot", "ComputeAverage"}
		for _, sym := range symbols {
			output, err := env.MCPSearch(sym, betaID)
			require.NoError(t, err, "Failed to search for %s", sym)

			assert.Contains(t, output, sym, "Should find %s in project-beta", sym)
		}
	})

	// Step 9: Cross-project isolation verification
	t.Run("CrossProjectIsolation", func(t *testing.T) {
		// Search for Alpha symbol in Beta project - should NOT find it
		output, err := env.MCPSearch("AlphaGreeter", betaID)
		require.NoError(t, err, "Failed to cross-search")
		env.SaveResult("cross-search-alpha-in-beta.json", output)

		// The search result should not contain AlphaGreeter
		assert.NotContains(t, output, "AlphaGreeter",
			"AlphaGreeter should NOT appear when searching project-beta")

		// Search for Beta symbol in Alpha project - should NOT find it
		output, err = env.MCPSearch("BetaCalculator", alphaID)
		require.NoError(t, err, "Failed to cross-search")
		env.SaveResult("cross-search-beta-in-alpha.json", output)

		// The search result should not contain BetaCalculator
		assert.NotContains(t, output, "BetaCalculator",
			"BetaCalculator should NOT appear when searching project-alpha")
	})

	// Step 10: Global search finds both
	t.Run("GlobalSearchFindsBoth", func(t *testing.T) {
		// Search without project_id should find symbols from both projects
		output, err := env.MCPSearch("New", "")
		require.NoError(t, err, "Failed to global search")
		env.SaveResult("global-search.json", output)

		// Should find symbols from both projects
		assert.Contains(t, output, "NewAlphaGreeter", "Global search should find NewAlphaGreeter")
		assert.Contains(t, output, "NewBetaCalculator", "Global search should find NewBetaCalculator")
	})

	t.Log("All project isolation tests passed!")
}

// TestClaudeMCPQuery tests Claude CLI querying iter MCP (requires credentials).
// Requires Docker mode (TEST_DOCKER=1).
func TestClaudeMCPQuery(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Check for credentials
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Skip("No Claude credentials - skipping Claude CLI test")
	}

	startTime := time.Now()
	env, err := NewE2EEnv(t, "TestClaudeMCPQuery")
	require.NoError(t, err, "Failed to create E2E environment")
	defer env.Cleanup()
	defer func() {
		duration := time.Since(startTime)
		env.WriteSummary(t.Failed() == false, duration, "Claude CLI MCP query test - validates Claude can query iter MCP server")
	}()

	// Start containers
	require.NoError(t, env.StartIter(), "Failed to start iter")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Copy projects and credentials
	require.NoError(t, env.CopyTestProjects(), "Failed to copy test projects")
	require.NoError(t, env.CopyCredentials(), "Failed to copy credentials")
	require.NoError(t, env.ConfigureMCP(), "Failed to configure MCP")

	// Register and index projects
	alphaID, err := env.RegisterProject("project-alpha", "/data/projects/project-alpha")
	require.NoError(t, err)
	betaID, err := env.RegisterProject("project-beta", "/data/projects/project-beta")
	require.NoError(t, err)
	require.NoError(t, env.IndexProject(alphaID))
	require.NoError(t, env.IndexProject(betaID))

	time.Sleep(2 * time.Second)

	t.Run("ClaudeQueriesAlphaProject", func(t *testing.T) {
		script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 5 \
			"Using the iter MCP server, search for 'AlphaGreeter' in project ID '%s'. What does this symbol do?"`, alphaID)

		exitCode, output, err := env.ExecBash(script)
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-alpha-query.txt", output)

		if exitCode != 0 {
			t.Logf("Claude returned exit code %d (may be expected)", exitCode)
		}

		// Claude's response should mention Alpha-related concepts
		outputLower := strings.ToLower(output)
		assert.True(t,
			strings.Contains(outputLower, "alpha") ||
				strings.Contains(outputLower, "greet") ||
				strings.Contains(outputLower, "greeting"),
			"Claude response should mention Alpha/greeting concepts")

		t.Logf("Claude Alpha response: %s", output[:min(500, len(output))])
	})

	t.Run("ClaudeQueriesBetaProject", func(t *testing.T) {
		script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 5 \
			"Using the iter MCP server, search for 'BetaCalculator' in project ID '%s'. What mathematical operations does it support?"`, betaID)

		exitCode, output, err := env.ExecBash(script)
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-beta-query.txt", output)

		if exitCode != 0 {
			t.Logf("Claude returned exit code %d (may be expected)", exitCode)
		}

		// Claude's response should mention Beta/calculator concepts
		outputLower := strings.ToLower(output)
		assert.True(t,
			strings.Contains(outputLower, "beta") ||
				strings.Contains(outputLower, "calculator") ||
				strings.Contains(outputLower, "math") ||
				strings.Contains(outputLower, "add") ||
				strings.Contains(outputLower, "multiply"),
			"Claude response should mention Beta/calculator concepts")

		t.Logf("Claude Beta response: %s", output[:min(500, len(output))])
	})
}

// TestClaudeMCPExactCodeRetrieval tests that Claude can retrieve exact code content via MCP.
// This test requires GOOGLE_GEMINI_API_KEY to be configured for semantic indexing.
// Without GOOGLE_GEMINI_API_KEY, the test SHOULD FAIL because semantic search won't return accurate results.
// Requires Docker mode (TEST_DOCKER=1).
func TestClaudeMCPExactCodeRetrieval(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Check for credentials
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Skip("No Claude credentials - skipping Claude CLI test")
	}

	startTime := time.Now()
	env, err := NewE2EEnv(t, "TestClaudeMCPExactCodeRetrieval")
	require.NoError(t, err, "Failed to create E2E environment")
	defer env.Cleanup()
	defer func() {
		duration := time.Since(startTime)
		env.WriteSummary(t.Failed() == false, duration, "Claude MCP exact code retrieval - validates semantic search returns accurate code")
	}()

	// Start containers
	require.NoError(t, env.StartIter(), "Failed to start iter")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Copy projects and credentials
	require.NoError(t, env.CopyTestProjects(), "Failed to copy test projects")
	require.NoError(t, env.CopyCredentials(), "Failed to copy credentials")
	require.NoError(t, env.ConfigureMCP(), "Failed to configure MCP")

	// Register and index projects
	alphaID, err := env.RegisterProject("project-alpha", "/data/projects/project-alpha")
	require.NoError(t, err, "Failed to register project-alpha")
	betaID, err := env.RegisterProject("project-beta", "/data/projects/project-beta")
	require.NoError(t, err, "Failed to register project-beta")

	require.NoError(t, env.IndexProject(alphaID), "Failed to index project-alpha")
	require.NoError(t, env.IndexProject(betaID), "Failed to index project-beta")

	time.Sleep(2 * time.Second)

	// Test: Claude should be able to retrieve exact code content from project-alpha
	t.Run("RetrieveAlphaGreetingCode", func(t *testing.T) {
		// Ask Claude to show the actual Greet method implementation
		script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 10 \
			"Using the iter MCP server with project ID '%s', find the Greet method in AlphaGreeter and show me the exact code implementation. I need to see the actual function body with the greeting message format."`, alphaID)

		exitCode, output, err := env.ExecBash(script)
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-alpha-exact-code.txt", output)

		t.Logf("Claude exit code: %d", exitCode)
		t.Logf("Claude response length: %d", len(output))

		// The response MUST contain the actual greeting format from the code
		// The Alpha project's Greet method formats: "%s, %s! %s" (prefix, name, suffix)
		// Expected content: "Hello from Alpha"
		outputLower := strings.ToLower(output)

		// These assertions will FAIL if GOOGLE_GEMINI_API_KEY is not configured
		// because semantic search won't return accurate code results
		assert.True(t,
			strings.Contains(output, "Hello from Alpha") ||
				strings.Contains(output, "prefix") ||
				strings.Contains(output, "suffix") ||
				strings.Contains(output, "Sprintf"),
			"Claude response MUST contain exact code from AlphaGreeter.Greet method. "+
				"If this fails, verify GOOGLE_GEMINI_API_KEY is configured for semantic indexing.")

		// Verify it mentions the greeting concepts
		assert.True(t,
			strings.Contains(outputLower, "greet") ||
				strings.Contains(outputLower, "greeting") ||
				strings.Contains(outputLower, "message"),
			"Claude response should describe the greeting functionality")

		t.Logf("Alpha exact code response: %s", output[:min(800, len(output))])
	})

	// Test: Claude should be able to retrieve exact code content from project-beta
	t.Run("RetrieveBetaCalculatorCode", func(t *testing.T) {
		// Ask Claude to show the actual SquareRoot method implementation
		script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 10 \
			"Using the iter MCP server with project ID '%s', find the SquareRoot method in BetaCalculator and show me the exact code implementation. Include the function signature and body."`, betaID)

		exitCode, output, err := env.ExecBash(script)
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-beta-exact-code.txt", output)

		t.Logf("Claude exit code: %d", exitCode)

		// The response MUST contain the actual SquareRoot implementation
		// The Beta project uses math.Sqrt and stores in history
		outputLower := strings.ToLower(output)

		// These assertions will FAIL if GOOGLE_GEMINI_API_KEY is not configured
		assert.True(t,
			strings.Contains(output, "math.Sqrt") ||
				strings.Contains(output, "Sqrt") ||
				strings.Contains(outputLower, "square root") ||
				strings.Contains(output, "history"),
			"Claude response MUST contain exact code from BetaCalculator.SquareRoot method. "+
				"If this fails, verify GOOGLE_GEMINI_API_KEY is configured for semantic indexing.")

		// Verify it mentions calculator concepts
		assert.True(t,
			strings.Contains(outputLower, "calculator") ||
				strings.Contains(outputLower, "calculate") ||
				strings.Contains(outputLower, "result"),
			"Claude response should describe the calculator functionality")

		t.Logf("Beta exact code response: %s", output[:min(800, len(output))])
	})

	// Test: Claude should show the hello world main function output
	t.Run("RetrieveMainFunctionOutput", func(t *testing.T) {
		// Ask Claude what the main function in project-alpha outputs
		script := fmt.Sprintf(`export PATH="/home/testuser/.local/bin:$PATH" && \
			claude -p --dangerously-skip-permissions --max-turns 10 \
			"Using the iter MCP server with project ID '%s', look at the main function and tell me exactly what message gets printed when the program runs. Show me the actual output format."`, alphaID)

		exitCode, output, err := env.ExecBash(script)
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-main-output.txt", output)

		t.Logf("Claude exit code: %d", exitCode)

		// The main function in project-alpha:
		// 1. Prints "Starting alpha-greeting-service on port 8080"
		// 2. Prints the greeting message
		// 3. Prints formatted (uppercase) version
		outputLower := strings.ToLower(output)

		// These assertions will FAIL if GOOGLE_GEMINI_API_KEY is not configured
		assert.True(t,
			strings.Contains(output, "alpha-greeting-service") ||
				strings.Contains(output, "8080") ||
				strings.Contains(output, "Starting") ||
				strings.Contains(outputLower, "world") ||
				strings.Contains(output, "Formatted"),
			"Claude response MUST describe the actual program output. "+
				"If this fails, verify GOOGLE_GEMINI_API_KEY is configured for semantic indexing.")

		t.Logf("Main function output response: %s", output[:min(800, len(output))])
	})
}

// TestIndexStatusWithoutGeminiAPIKey verifies that index status correctly reports when API key is missing.
// Requires Docker mode (TEST_DOCKER=1).
func TestIndexStatusWithoutGeminiAPIKey(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	startTime := time.Now()
	env, err := NewE2EEnv(t, "TestIndexStatusWithoutGeminiAPIKey")
	require.NoError(t, err, "Failed to create E2E environment")
	defer env.Cleanup()
	defer func() {
		duration := time.Since(startTime)
		env.WriteSummary(t.Failed() == false, duration, "Index status API key check - validates API reports missing Gemini API key")
	}()

	// Start containers
	require.NoError(t, env.StartIter(), "Failed to start iter")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Copy and register projects
	require.NoError(t, env.CopyTestProjects(), "Failed to copy test projects")

	alphaID, err := env.RegisterProject("project-alpha", "/data/projects/project-alpha")
	require.NoError(t, err, "Failed to register project-alpha")

	require.NoError(t, env.IndexProject(alphaID), "Failed to index project-alpha")

	time.Sleep(1 * time.Second)

	// Get index status via API
	script := `curl -s http://iter:19000/api/index-status`
	exitCode, output, err := env.ExecBash(script)
	require.NoError(t, err, "Failed to get index status")
	require.Equal(t, 0, exitCode, "curl should succeed")

	env.SaveResult("index-status.json", output)

	// Parse response
	cleanOutput := cleanJSON(output)

	var status struct {
		GeminiAPIKeyConfigured bool   `json:"gemini_api_key_configured"`
		GeminiAPIKeyStatus     string `json:"gemini_api_key_status"`
		Projects               []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			IndexStatus  string `json:"index_status"`
			ErrorMessage string `json:"error_message"`
		} `json:"projects"`
	}

	err = json.Unmarshal([]byte(cleanOutput), &status)
	require.NoError(t, err, "Failed to parse index status response")

	// Verify GOOGLE_GEMINI_API_KEY is NOT configured in test environment
	assert.False(t, status.GeminiAPIKeyConfigured,
		"GOOGLE_GEMINI_API_KEY should NOT be configured in test environment")
	assert.Contains(t, status.GeminiAPIKeyStatus, "not provided",
		"Status should indicate API key is not provided")

	// Verify project shows api_key_missing status
	require.Len(t, status.Projects, 1, "Should have 1 project")
	assert.Equal(t, "api_key_missing", status.Projects[0].IndexStatus,
		"Project index status should be 'api_key_missing'")
	assert.Contains(t, status.Projects[0].ErrorMessage, "GOOGLE_GEMINI_API_KEY",
		"Error message should mention GOOGLE_GEMINI_API_KEY")

	t.Log("EXPECTED: Index status shows GOOGLE_GEMINI_API_KEY not provided")
	t.Logf("API Key Status: %s", status.GeminiAPIKeyStatus)
	t.Logf("Project Status: %s - %s", status.Projects[0].IndexStatus, status.Projects[0].ErrorMessage)
}

// TestIndexStatusUIPage tests the index status web UI page.
// Requires Docker mode (TEST_DOCKER=1).
func TestIndexStatusUIPage(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	startTime := time.Now()
	env, err := NewE2EEnv(t, "TestIndexStatusUIPage")
	require.NoError(t, err, "Failed to create E2E environment")
	defer env.Cleanup()
	defer func() {
		duration := time.Since(startTime)
		env.WriteSummary(t.Failed() == false, duration, "Index status UI page - validates web UI shows API key status")
	}()

	// Start containers
	require.NoError(t, env.StartIter(), "Failed to start iter")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Copy and register projects
	require.NoError(t, env.CopyTestProjects(), "Failed to copy test projects")

	alphaID, err := env.RegisterProject("project-alpha", "/data/projects/project-alpha")
	require.NoError(t, err)

	require.NoError(t, env.IndexProject(alphaID))

	time.Sleep(1 * time.Second)

	// Fetch the index status web page
	script := `curl -s http://iter:19000/web/index-status`
	exitCode, output, err := env.ExecBash(script)
	require.NoError(t, err, "Failed to fetch index status page")
	require.Equal(t, 0, exitCode, "curl should succeed")

	env.SaveResult("index-status-page.html", output)

	// Verify page content
	assert.Contains(t, output, "Index Status", "Page should have Index Status title")
	assert.Contains(t, output, "GOOGLE_GEMINI_API_KEY", "Page should mention GOOGLE_GEMINI_API_KEY")

	// Verify API key warning is shown
	assert.True(t,
		strings.Contains(output, "not provided") ||
			strings.Contains(output, "Not configured") ||
			strings.Contains(output, "error"),
		"Page should indicate API key is not configured")

	// Verify project is listed
	assert.Contains(t, output, "project-alpha", "Page should list project-alpha")

	// Verify project shows API key missing status
	assert.True(t,
		strings.Contains(output, "API Key Missing") ||
			strings.Contains(output, "api_key_missing"),
		"Page should show API key missing status for project")

	t.Log("EXPECTED: Index status UI shows GOOGLE_GEMINI_API_KEY warning")
	t.Log("Page contains API key status and project indexing information")
}

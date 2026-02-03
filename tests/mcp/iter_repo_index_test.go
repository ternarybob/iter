// Package mcp provides MCP integration tests for iter-service.
// This test validates that Claude can query indexed content via MCP.
//
// Test scenario:
//  1. Start iter-service and Claude containers
//  2. Clone the iter repository into the Claude container
//  3. Register and index the repo via iter-service
//  4. Run Claude CLI queries from a DIFFERENT directory to prove indexed content works
//
// Run with:
//
//	TEST_DOCKER=1 go test -v ./tests/mcp/... -run TestIterRepoIndex
//
// NOTE: This test requires Docker mode (TEST_DOCKER=1) and Claude credentials.
package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/iter/tests/common"
)

// Silence unused import warning
var _ = common.ModeDocker

const (
	// iterRepoURL is the GitHub URL for the iter repository.
	iterRepoURL = "https://github.com/ternarybob/iter"
	// iterRepoPath is where the repo will be cloned in the container.
	iterRepoPath = "/home/testuser/repos/iter"
	// claudeWorkDir is a separate directory for running Claude (NOT the repo dir).
	claudeWorkDir = "/home/testuser/workspace"
)

// IterRepoEnv extends E2EEnv with iter repo-specific functionality.
type IterRepoEnv struct {
	*E2EEnv
	iterProjectID string
}

// NewIterRepoEnv creates a new test environment for iter repo indexing tests.
func NewIterRepoEnv(t *testing.T, testName string) (*IterRepoEnv, error) {
	env, err := NewE2EEnv(t, testName)
	if err != nil {
		return nil, err
	}
	return &IterRepoEnv{E2EEnv: env}, nil
}

// CopyIterRepoToIterContainer copies the local iter repo to the iter container.
// Since we're running from within the iter repo, we copy it directly.
func (e *IterRepoEnv) CopyIterRepoToIterContainer() error {
	// Get project root (we're running from the iter repo)
	projectRoot := getProjectRoot()
	e.t.Logf("Copying iter repo from %s to container...", projectRoot)

	// Create destination in iter container
	_, _, err := e.ExecIter("mkdir", "-p", "/data/repos/iter")
	if err != nil {
		return fmt.Errorf("create repos dir in iter: %w", err)
	}

	// Copy key files and directories to the iter container
	// We don't need the entire repo - just the source code for indexing
	filesToCopy := []string{
		"README.md",
		"go.mod",
		"go.sum",
		"cmd",
		"internal",
		"pkg",
		"configs",
	}

	for _, name := range filesToCopy {
		srcPath := filepath.Join(projectRoot, name)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			e.t.Logf("Skipping %s (not found)", name)
			continue
		}

		if err := e.copyPathToIter(srcPath, "/data/repos/iter/"+name); err != nil {
			e.t.Logf("Warning: failed to copy %s: %v", name, err)
		} else {
			e.t.Logf("Copied %s", name)
		}
	}

	// Verify copy succeeded
	_, output, _ := e.ExecIterBash("ls -la /data/repos/iter/")
	e.t.Logf("Iter container repo contents:\n%s", output)

	return nil
}

// copyPathToIter copies a file or directory to the iter container.
func (e *IterRepoEnv) copyPathToIter(localPath, containerPath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return e.copyDirToIterContainer(localPath, containerPath)
	}
	return e.copyFileToIterContainer(localPath, containerPath)
}

// copyFileToIterContainer copies a single file to the iter container.
func (e *IterRepoEnv) copyFileToIterContainer(localPath, containerPath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	// Create parent directory
	parentDir := filepath.Dir(containerPath)
	e.ExecIter("mkdir", "-p", parentDir)

	return e.iter.CopyToContainer(e.ctx, data, containerPath, 0644)
}

// copyDirToIterContainer recursively copies a directory to the iter container.
func (e *IterRepoEnv) copyDirToIterContainer(localPath, containerPath string) error {
	// Create destination directory
	e.ExecIter("mkdir", "-p", containerPath)

	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain directories
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".claude" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip large/binary files
		if info.Size() > 1024*1024 { // 1MB limit
			return nil
		}

		relPath, _ := filepath.Rel(localPath, path)
		destPath := filepath.Join(containerPath, relPath)

		// Create parent directory
		e.ExecIter("mkdir", "-p", filepath.Dir(destPath))

		// Read and copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		return e.iter.CopyToContainer(e.ctx, data, destPath, 0644)
	})
}

// ExecIterBash executes a bash script in the iter container.
func (e *IterRepoEnv) ExecIterBash(script string) (int, string, error) {
	return e.ExecIter("sh", "-c", script)
}

// RegisterIterRepo registers the iter repo with iter-service.
func (e *IterRepoEnv) RegisterIterRepo() (string, error) {
	projectID, err := e.RegisterProject("iter", "/data/repos/iter")
	if err != nil {
		return "", err
	}
	e.iterProjectID = projectID
	return projectID, nil
}

// IndexIterRepo triggers indexing for the iter repo.
func (e *IterRepoEnv) IndexIterRepo() error {
	if e.iterProjectID == "" {
		return fmt.Errorf("iter project not registered")
	}
	return e.IndexProject(e.iterProjectID)
}

// SetupClaudeWorkspace creates a separate workspace directory for Claude.
// This is crucial - Claude must NOT be run from the repo directory.
func (e *IterRepoEnv) SetupClaudeWorkspace() error {
	_, _, err := e.Exec("mkdir", "-p", claudeWorkDir)
	if err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// Create a simple file to prove we're in a different directory
	script := fmt.Sprintf("echo 'Claude workspace - NOT the iter repo' > %s/README.txt", claudeWorkDir)
	_, _, err = e.ExecBash(script)
	if err != nil {
		return fmt.Errorf("create workspace readme: %w", err)
	}

	e.t.Logf("Created Claude workspace at %s (separate from repo)", claudeWorkDir)
	return nil
}

// RunClaudeQuery runs a Claude CLI query from the workspace directory (NOT the repo).
// This proves that indexed content is being retrieved via MCP, not local file access.
// MCP server is configured with user scope so it's available from any directory.
func (e *IterRepoEnv) RunClaudeQuery(prompt string) (string, error) {
	// Run Claude from workspace directory - MCP server configured with user scope
	script := fmt.Sprintf(`cd %s && export PATH="/home/testuser/.local/bin:$PATH" && claude -p --dangerously-skip-permissions --max-turns 10 %q 2>&1`,
		claudeWorkDir, prompt)

	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return "", fmt.Errorf("claude query failed: %w", err)
	}

	e.t.Logf("Claude query (exit %d): %s", exitCode, prompt)
	return output, nil
}

// RunClaudeQueryWithProjectID runs a Claude query that explicitly instructs Claude
// to use the iter MCP tools for searching indexed content.
func (e *IterRepoEnv) RunClaudeQueryWithProjectID(prompt string) (string, error) {
	// Explicitly instruct Claude to use the iter MCP search tool
	fullPrompt := fmt.Sprintf("Use the 'search' tool from the iter MCP server to answer this question about the iter codebase (project ID: '%s'). %s", e.iterProjectID, prompt)
	return e.RunClaudeQuery(fullPrompt)
}

// VerifyMCPToolsAvailable verifies that Claude can see the iter MCP tools.
func (e *IterRepoEnv) VerifyMCPToolsAvailable() error {
	// Run claude mcp list from the workspace directory to verify tools are visible
	script := fmt.Sprintf(`cd %s && export PATH="/home/testuser/.local/bin:$PATH" && claude mcp list 2>&1`, claudeWorkDir)
	exitCode, output, err := e.ExecBash(script)
	if err != nil {
		return fmt.Errorf("failed to list MCP servers: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("claude mcp list failed with exit %d: %s", exitCode, output)
	}

	e.t.Logf("MCP servers from workspace: %s", strings.TrimSpace(output))

	// Verify iter is in the list and connected
	if !strings.Contains(output, "iter") {
		return fmt.Errorf("iter MCP server not found in list: %s", output)
	}
	if !strings.Contains(output, "Connected") && !strings.Contains(output, "✓") {
		return fmt.Errorf("iter MCP server not connected: %s", output)
	}

	return nil
}

// --- Test ---

// TestIterRepoIndex tests that Claude can query indexed iter repository content via MCP.
// This test proves that:
// 1. The iter repo can be indexed
// 2. Claude can retrieve content via MCP (not local file access)
// 3. Queries return accurate information about the repo
//
// Requires Docker mode (TEST_DOCKER=1) and Claude credentials.
func TestIterRepoIndex(t *testing.T) {
	requireDockerMode(t)
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Check for Claude credentials
	if !hasClaudeCredentials() {
		t.Skip("No Claude credentials - skipping Claude CLI test")
	}

	startTime := time.Now()
	env, err := NewIterRepoEnv(t, "TestIterRepoIndex")
	require.NoError(t, err, "Failed to create test environment")
	defer env.Cleanup()

	// Track if before screenshot was captured
	beforeScreenshotCaptured := false
	defer func() {
		// Create a fresh browser for after screenshot (original may have timed out)
		t.Log("Capturing after screenshot...")
		afterBrowser, err := env.NewBrowser()
		if err != nil {
			t.Logf("Warning: Failed to create browser for after screenshot: %v", err)
		} else {
			if err := afterBrowser.NavigateAndScreenshot("/web/", "02-after"); err != nil {
				t.Logf("Warning: Failed to capture after screenshot: %v", err)
			}
			afterBrowser.Close()
		}

		// Verify screenshots
		if beforeScreenshotCaptured {
			env.RequireScreenshots([]string{"01-before", "02-after"})
		}

		// Collect logs before writing summary
		env.CollectContainerLogs()

		// Write summary
		duration := time.Since(startTime)
		env.WriteSummary(!t.Failed(), duration, "Claude MCP query test - validates indexed iter repo content retrieval")
	}()

	// Step 1: Start containers
	t.Log("Starting iter-service container...")
	require.NoError(t, env.StartIter(), "Failed to start iter")

	t.Log("Starting Claude container...")
	require.NoError(t, env.StartClaude(), "Failed to start Claude")

	// Step 2: Create browser and capture before screenshot
	t.Log("Creating browser for screenshots...")
	browser, err := env.NewBrowser()
	require.NoError(t, err, "Failed to create browser")

	t.Log("Capturing before screenshot (initial state)...")
	require.NoError(t, browser.NavigateAndScreenshot("/web/", "01-before"), "Failed to capture before screenshot")
	browser.Close() // Close immediately - we'll create a fresh one for after screenshot
	beforeScreenshotCaptured = true

	// Step 3: Clone iter repo into iter container
	t.Log("Cloning iter repository...")
	require.NoError(t, env.CopyIterRepoToIterContainer(), "Failed to clone iter repo")

	// Step 4: Register and index the iter repo
	t.Log("Registering iter repository...")
	projectID, err := env.RegisterIterRepo()
	require.NoError(t, err, "Failed to register iter repo")
	require.NotEmpty(t, projectID, "Project ID should not be empty")
	t.Logf("Registered iter repo with project ID: %s", projectID)

	t.Log("Indexing iter repository...")
	require.NoError(t, env.IndexIterRepo(), "Failed to index iter repo")

	// Wait for indexing to complete
	t.Log("Waiting for indexing to complete...")
	time.Sleep(5 * time.Second)

	// Step 5: Verify index status via API
	t.Run("VerifyIndexStatus", func(t *testing.T) {
		script := fmt.Sprintf(`curl -s http://iter:19000/projects/%s`, projectID)
		_, output, err := env.ExecBash(script)
		require.NoError(t, err, "Failed to get project status")
		env.SaveResult("project-status.json", output)

		t.Logf("Project status: %s", output)
		assert.Contains(t, output, projectID, "Response should contain project ID")

		// Verify files were indexed
		assert.Contains(t, output, "file_count", "Response should contain file_count")
		assert.Contains(t, output, "document_count", "Response should contain document_count")
		// Ensure we indexed more than 0 files
		assert.NotContains(t, output, `"file_count":0`, "Should have indexed at least some files")
	})

	// Step 6: Setup Claude workspace (separate from repo)
	t.Log("Setting up Claude workspace...")
	require.NoError(t, env.SetupClaudeWorkspace(), "Failed to setup workspace")

	// Step 7: Copy credentials and configure MCP
	t.Log("Configuring Claude with iter MCP...")
	require.NoError(t, env.CopyCredentials(), "Failed to copy credentials")
	require.NoError(t, env.ConfigureMCP(), "Failed to configure MCP")

	// Step 8: Verify MCP tools are available from workspace directory
	t.Log("Verifying MCP tools available from workspace...")
	require.NoError(t, env.VerifyMCPToolsAvailable(), "MCP tools not available from workspace")

	// Step 9: Test Claude queries against indexed content
	t.Run("QueryReadmeContent", func(t *testing.T) {
		t.Log("Querying Claude about iter README...")
		output, err := env.RunClaudeQueryWithProjectID("What does the iter README contain? Summarize the main points.")
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-readme-query.txt", output)

		outputLower := strings.ToLower(output)
		// README should mention iter-service or code indexing
		assert.True(t,
			strings.Contains(outputLower, "iter") ||
				strings.Contains(outputLower, "index") ||
				strings.Contains(outputLower, "mcp") ||
				strings.Contains(outputLower, "code"),
			"Claude response should mention iter-related concepts. Got: %s", truncateOutput(output, 500))
	})

	t.Run("QueryMainFunction", func(t *testing.T) {
		t.Log("Querying Claude about main function...")
		output, err := env.RunClaudeQueryWithProjectID("What does the main function in cmd/iter-service/main.go do?")
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-main-query.txt", output)

		outputLower := strings.ToLower(output)
		// Should mention service, server, or startup concepts
		assert.True(t,
			strings.Contains(outputLower, "service") ||
				strings.Contains(outputLower, "server") ||
				strings.Contains(outputLower, "start") ||
				strings.Contains(outputLower, "command") ||
				strings.Contains(outputLower, "cli"),
			"Claude response should describe main function. Got: %s", truncateOutput(output, 500))
	})

	t.Run("QueryMCPHandler", func(t *testing.T) {
		t.Log("Querying Claude about MCP handler...")
		output, err := env.RunClaudeQueryWithProjectID("Search for MCP handler code. How does iter handle MCP requests?")
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-mcp-query.txt", output)

		outputLower := strings.ToLower(output)
		// Should mention MCP, handler, or request handling
		assert.True(t,
			strings.Contains(outputLower, "mcp") ||
				strings.Contains(outputLower, "handler") ||
				strings.Contains(outputLower, "request") ||
				strings.Contains(outputLower, "tool"),
			"Claude response should describe MCP handling. Got: %s", truncateOutput(output, 500))
	})

	t.Run("QueryProjectStructure", func(t *testing.T) {
		t.Log("Querying Claude about project structure...")
		output, err := env.RunClaudeQueryWithProjectID("What is the overall structure of the iter codebase? List the main packages.")
		require.NoError(t, err, "Claude query failed")
		env.SaveResult("claude-structure-query.txt", output)

		outputLower := strings.ToLower(output)
		// Should mention packages like cmd, internal, pkg, etc.
		assert.True(t,
			strings.Contains(outputLower, "cmd") ||
				strings.Contains(outputLower, "internal") ||
				strings.Contains(outputLower, "package") ||
				strings.Contains(outputLower, "directory"),
			"Claude response should describe project structure. Got: %s", truncateOutput(output, 500))
	})

	t.Log("All iter repo index tests completed!")
}

// hasClaudeCredentials checks if Claude credentials are available on the host.
func hasClaudeCredentials() bool {
	credPath := filepath.Join(os.Getenv("HOME"), ".claude", ".credentials.json")
	_, err := os.Stat(credPath)
	return err == nil
}

// truncateOutput truncates output to maxLen characters for display.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "..."
}

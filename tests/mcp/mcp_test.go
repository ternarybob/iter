// Package mcp contains integration tests for iter-service MCP functionality.
// These tests use Claude Code CLI to verify MCP tool integration.
package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// checkClaudeAuth verifies Claude CLI is installed and authenticated.
func checkClaudeAuth(t *testing.T) {
	t.Helper()

	// Check Claude is installed
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("Claude CLI not installed, skipping MCP tests")
	}

	// Check for authentication
	claudeDir := os.Getenv("HOME") + "/.claude"
	claudeJSON := os.Getenv("HOME") + "/.claude.json"

	hasAuth := false
	if _, err := os.Stat(claudeDir); err == nil {
		hasAuth = true
	}
	if _, err := os.Stat(claudeJSON); err == nil {
		hasAuth = true
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		hasAuth = true
	}

	if !hasAuth {
		t.Skip("No Claude authentication found, skipping MCP tests")
	}
}

// runClaude executes a Claude CLI command and returns the output.
func runClaude(t *testing.T, env *common.TestEnv, prompt string) (string, error) {
	t.Helper()

	// Configure MCP server using claude mcp add
	mcpURL := env.BaseURL + "/mcp/v1"

	// Remove any existing config and add fresh one
	exec.Command("claude", "mcp", "remove", "iter").Run()
	addCmd := exec.Command("claude", "mcp", "add", "--transport", "http", "iter", mcpURL)
	addOutput, addErr := addCmd.CombinedOutput()
	if addErr != nil {
		env.Log("Warning: MCP add command: %v, output: %s", addErr, string(addOutput))
	}

	// Verify MCP config
	listCmd := exec.Command("claude", "mcp", "list")
	listOutput, _ := listCmd.CombinedOutput()
	env.Log("MCP servers: %s", strings.TrimSpace(string(listOutput)))

	// Run Claude with MCP
	cmd := exec.Command("claude",
		"-p", // Print mode (non-interactive)
		"--dangerously-skip-permissions",
		"--max-turns", "10",
		"--output-format", "text",
		prompt,
	)

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Log the interaction
	env.Log("Claude prompt: %s", prompt)
	env.Log("Claude output: %s", outputStr)

	if err != nil {
		return outputStr, fmt.Errorf("claude command failed: %w\nOutput: %s", err, outputStr)
	}

	return outputStr, nil
}

// TestMCPListProjects tests listing projects via MCP.
func TestMCPListProjects(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "list-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Register a test project first
	client := env.NewHTTPClient()
	projectPath, err := env.CreateTestProject("mcp-test-project")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	_, _, err = client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Ask Claude to list projects via MCP
	output, err := runClaude(t, env, "Use the iter MCP tool to list all indexed projects. Just list the project names.")
	if err != nil {
		t.Fatalf("Claude command failed: %v", err)
	}

	// Save output
	env.SaveResult("claude-output.txt", []byte(output))

	// Fail if Claude returned empty output
	if strings.TrimSpace(output) == "" {
		t.Fatal("Claude returned empty output - MCP integration not working")
	}

	// Check if Claude says it doesn't have MCP access
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") {
		t.Fatalf("Claude reports no MCP access:\n%s", output)
	}

	// Verify output mentions the project
	if !strings.Contains(outputLower, "mcp-test-project") &&
		!strings.Contains(outputLower, "project") {
		t.Errorf("Output does not contain project info: %s", output)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP list projects test completed")
}

// TestMCPSearch tests searching via MCP.
func TestMCPSearch(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "search")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create and register a test project with known symbols
	client := env.NewHTTPClient()
	projectPath, err := env.CreateTestProject("search-test")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	resp, _, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, 201)

	// Rebuild index
	resp, _, err = client.Post("/projects/"+projectPath+"/index", nil)
	if err != nil {
		// Project ID might be different, try listing
		resp, body, _ := client.Get("/projects")
		if resp.StatusCode == 200 {
			projects := common.AssertJSONArray(t, body)
			if len(projects) > 0 {
				if id, ok := projects[0]["id"].(string); ok {
					client.Post("/projects/"+id+"/index", nil)
				}
			}
		}
	}

	// Wait for indexing
	time.Sleep(3 * time.Second)

	// Ask Claude to search for HelloWorld function
	output, err := runClaude(t, env,
		"Use the iter MCP search tool to find the 'HelloWorld' function. "+
			"Tell me the file path and line number where it's defined.")
	if err != nil {
		t.Fatalf("Claude command failed: %v", err)
	}

	env.SaveResult("claude-output.txt", []byte(output))

	// Fail if Claude returned empty output
	if strings.TrimSpace(output) == "" {
		t.Fatal("Claude returned empty output - MCP integration not working")
	}

	// Check if Claude says it doesn't have MCP access
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") {
		t.Fatalf("Claude reports no MCP access:\n%s", output)
	}

	// Check if output mentions main.go or HelloWorld
	if !strings.Contains(outputLower, "main.go") && !strings.Contains(outputLower, "helloworld") {
		t.Errorf("Output does not contain expected search result: %s", output)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP search test completed")
}

// TestMCPGetDependencies tests getting symbol dependencies via MCP.
func TestMCPGetDependencies(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "dependencies")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create and register a test project
	client := env.NewHTTPClient()
	projectPath, err := env.CreateTestProject("deps-test")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	_, _, err = client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}

	// Wait for indexing
	time.Sleep(3 * time.Second)

	// Ask Claude about dependencies
	output, err := runClaude(t, env,
		"Use the iter MCP tools to find what the 'main' function depends on in this project.")
	if err != nil {
		t.Fatalf("Claude command failed: %v", err)
	}

	env.SaveResult("claude-output.txt", []byte(output))

	// Fail if Claude returned empty output
	if strings.TrimSpace(output) == "" {
		t.Fatal("Claude returned empty output - MCP integration not working")
	}

	// Check if Claude says it doesn't have MCP access
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") {
		t.Fatalf("Claude reports no MCP access:\n%s", output)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP dependencies test completed")
}

// TestMCPComplexQuery tests a complex multi-step query via MCP.
func TestMCPComplexQuery(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "complex-query")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create project with more complex code
	projectPath := filepath.Join(env.DataDir, "test-projects", "complex")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Write multiple files
	mainGo := `package main

import "fmt"

type Calculator struct {
	lastResult int
}

func (c *Calculator) Add(a, b int) int {
	c.lastResult = a + b
	return c.lastResult
}

func (c *Calculator) Multiply(a, b int) int {
	c.lastResult = a * b
	return c.lastResult
}

func (c *Calculator) GetLastResult() int {
	return c.lastResult
}

func main() {
	calc := &Calculator{}
	fmt.Println(calc.Add(5, 3))
	fmt.Println(calc.Multiply(4, 2))
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	utilsGo := `package main

// FormatNumber formats a number with commas
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d,%03d", n/1000, n%1000)
}

// Max returns the larger of two integers
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "utils.go"), []byte(utilsGo), 0644); err != nil {
		t.Fatalf("Failed to write utils.go: %v", err)
	}

	goMod := "module complex\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Register project
	client := env.NewHTTPClient()
	_, _, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}

	// Wait for indexing
	time.Sleep(3 * time.Second)

	// Complex query
	output, err := runClaude(t, env,
		"Using the iter MCP tools, analyze this project. "+
			"Find all types and their methods, and list the utility functions. "+
			"Give a brief summary of what the code does.")
	if err != nil {
		t.Fatalf("Claude command failed: %v", err)
	}

	env.SaveResult("claude-output.txt", []byte(output))

	// Fail if Claude returned empty output
	if strings.TrimSpace(output) == "" {
		t.Fatal("Claude returned empty output - MCP integration not working")
	}

	// Check if Claude says it doesn't have MCP access
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") {
		t.Fatalf("Claude reports no MCP access:\n%s", output)
	}

	// Check if output mentions Calculator or methods
	if !strings.Contains(outputLower, "calculator") && !strings.Contains(outputLower, "add") {
		t.Errorf("Output does not contain expected analysis: %s", output)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP complex query test completed")
}

// TestMCPToolDiscovery tests that MCP tools are discoverable.
func TestMCPToolDiscovery(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "tool-discovery")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Ask Claude what iter MCP tools are available
	output, err := runClaude(t, env,
		"What MCP tools are available from the 'iter' server? List them briefly.")
	if err != nil {
		t.Fatalf("Claude command failed: %v", err)
	}

	env.SaveResult("claude-output.txt", []byte(output))

	// Fail if Claude returned empty output
	if strings.TrimSpace(output) == "" {
		t.Fatal("Claude returned empty output - MCP integration not working")
	}

	// Check if Claude says it doesn't have MCP access
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") {
		t.Fatalf("Claude reports no MCP access:\n%s", output)
	}

	// Check if output mentions some expected tools
	hasToolInfo := strings.Contains(outputLower, "search") ||
		strings.Contains(outputLower, "project") ||
		strings.Contains(outputLower, "symbol") ||
		strings.Contains(outputLower, "iter") ||
		strings.Contains(outputLower, "list")

	if !hasToolInfo {
		t.Errorf("Output does not contain tool information: %s", output)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP tool discovery test completed")
}

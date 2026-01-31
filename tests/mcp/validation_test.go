// Package mcp contains validation tests for iter-service MCP with Claude CLI.
// These tests verify Claude can connect to and use iter MCP tools.
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// terminalToHTML converts terminal output to HTML for screenshot purposes.
func terminalToHTML(title, command, output string, exitCode int) string {
	const tmpl = `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>
        body {
            background: #1e1e1e;
            color: #d4d4d4;
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
            font-size: 14px;
            padding: 20px;
            margin: 0;
        }
        .header {
            color: #569cd6;
            margin-bottom: 10px;
            font-size: 18px;
            font-weight: bold;
        }
        .command {
            color: #4ec9b0;
            margin-bottom: 15px;
            padding: 10px;
            background: #2d2d2d;
            border-radius: 4px;
        }
        .command::before {
            content: "$ ";
            color: #6a9955;
        }
        .output {
            white-space: pre-wrap;
            word-wrap: break-word;
            background: #252526;
            padding: 15px;
            border-radius: 4px;
            border-left: 3px solid {{if eq .ExitCode 0}}#4ec9b0{{else}}#f14c4c{{end}};
        }
        .status {
            margin-top: 15px;
            padding: 10px;
            border-radius: 4px;
            font-weight: bold;
        }
        .status.success {
            background: #1e3a1e;
            color: #4ec9b0;
        }
        .status.failure {
            background: #3a1e1e;
            color: #f14c4c;
        }
        .timestamp {
            color: #808080;
            font-size: 12px;
            margin-top: 10px;
        }
    </style>
</head>
<body>
    <div class="header">{{.Title}}</div>
    <div class="command">{{.Command}}</div>
    <div class="output">{{.Output}}</div>
    <div class="status {{if eq .ExitCode 0}}success{{else}}failure{{end}}">
        Exit Code: {{.ExitCode}}
    </div>
    <div class="timestamp">{{.Timestamp}}</div>
</body>
</html>`

	t, _ := template.New("terminal").Parse(tmpl)
	var buf bytes.Buffer
	t.Execute(&buf, map[string]interface{}{
		"Title":     title,
		"Command":   command,
		"Output":    output,
		"ExitCode":  exitCode,
		"Timestamp": time.Now().Format("2006-01-02 15:04:05 MST"),
	})
	return buf.String()
}

// saveTerminalScreenshot saves terminal output as HTML and captures a screenshot.
func saveTerminalScreenshot(env *common.TestEnv, name, title, command, output string, exitCode int) error {
	// Save HTML version
	html := terminalToHTML(title, command, output, exitCode)
	htmlPath := filepath.Join(env.ResultsDir, name+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("write HTML: %w", err)
	}

	// Save raw output
	txtPath := filepath.Join(env.ResultsDir, name+".txt")
	if err := os.WriteFile(txtPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("write text: %w", err)
	}

	env.Log("Terminal output saved: %s.html, %s.txt", name, name)

	// Try to capture screenshot with chromedp if available
	browser, err := env.NewBrowser()
	if err != nil {
		env.Log("Browser not available for screenshot: %v", err)
		return nil // Not fatal
	}
	defer browser.Close()

	// Navigate to local HTML file using absolute file URL
	fileURL := "file://" + htmlPath
	if err := browser.NavigateAbsolute(fileURL); err != nil {
		env.Log("Failed to navigate for screenshot: %v", err)
		return nil
	}

	if err := browser.FullPageScreenshot(name); err != nil {
		env.Log("Failed to capture screenshot: %v", err)
		return nil
	}

	return nil
}

// TestMCPValidation_Step1_ClaudeInstalled verifies Claude CLI is installed.
func TestMCPValidation_Step1_ClaudeInstalled(t *testing.T) {
	env := common.NewTestEnv(t, "mcp", "validation-step1-claude")
	defer env.Cleanup()

	startTime := time.Now()

	// Check if claude is installed
	cmd := exec.Command("claude", "--version")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	saveTerminalScreenshot(env, "01-claude-version", "Claude CLI Version Check", "claude --version", outputStr, exitCode)

	if err != nil {
		t.Fatalf("Claude CLI not installed: %v\nOutput: %s", err, outputStr)
	}

	if !strings.Contains(strings.ToLower(outputStr), "claude") {
		t.Fatalf("Unexpected claude --version output: %s", outputStr)
	}

	env.Log("Claude CLI installed: %s", strings.TrimSpace(outputStr))

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Step 1: Claude CLI installed and working")
}

// TestMCPValidation_Step2_ServiceRunning verifies iter-service starts and MCP is accessible.
func TestMCPValidation_Step2_ServiceRunning(t *testing.T) {
	env := common.NewTestEnv(t, "mcp", "validation-step2-service")
	defer env.Cleanup()

	startTime := time.Now()

	// Start service
	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start iter-service: %v", err)
	}

	// Test health endpoint
	cmd := exec.Command("curl", "-s", env.BaseURL+"/health")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	exitCode := 0
	if err != nil {
		exitCode = 1
	}

	saveTerminalScreenshot(env, "01-health-check", "Service Health Check", "curl -s "+env.BaseURL+"/health", outputStr, exitCode)

	if err != nil || !strings.Contains(outputStr, "ok") {
		t.Fatalf("Health check failed: %v\nOutput: %s", err, outputStr)
	}

	// Test MCP endpoint exists
	cmd = exec.Command("curl", "-s", "-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		env.BaseURL+"/mcp/v1")
	output, err = cmd.CombinedOutput()
	outputStr = string(output)

	exitCode = 0
	if err != nil {
		exitCode = 1
	}

	saveTerminalScreenshot(env, "02-mcp-initialize", "MCP Initialize Request", "curl -s -X POST -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}' "+env.BaseURL+"/mcp/v1", outputStr, exitCode)

	// Verify MCP response
	var mcpResp struct {
		Result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(output, &mcpResp); err != nil {
		t.Fatalf("Invalid MCP response: %v\nOutput: %s", err, outputStr)
	}

	if mcpResp.Error != nil {
		t.Fatalf("MCP initialize error: %s", mcpResp.Error.Message)
	}

	if mcpResp.Result.ServerInfo.Name != "iter-service" {
		t.Fatalf("Unexpected server name: %s", mcpResp.Result.ServerInfo.Name)
	}

	env.Log("MCP service responding correctly")

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Step 2: iter-service running with MCP enabled")
}

// TestMCPValidation_Step3_AddMCPServer tests adding iter as an MCP server to Claude.
func TestMCPValidation_Step3_AddMCPServer(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "validation-step3-add-server")
	defer env.Cleanup()

	startTime := time.Now()

	// Start service
	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start iter-service: %v", err)
	}

	// First, list existing MCP servers
	cmd := exec.Command("claude", "mcp", "list")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	saveTerminalScreenshot(env, "01-mcp-list-before", "MCP Servers (Before)", "claude mcp list", outputStr, exitCode)
	env.Log("MCP servers before: %s", outputStr)

	// Try to add iter MCP server using HTTP transport
	mcpURL := env.BaseURL + "/mcp/v1"
	cmd = exec.Command("claude", "mcp", "add", "--transport", "http", "iter-test", mcpURL)
	output, err = cmd.CombinedOutput()
	outputStr = string(output)

	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	saveTerminalScreenshot(env, "02-mcp-add", "Add MCP Server", fmt.Sprintf("claude mcp add --transport http iter-test %s", mcpURL), outputStr, exitCode)

	if err != nil {
		// Check if it's just because the server already exists
		if !strings.Contains(outputStr, "already") {
			t.Fatalf("Failed to add MCP server: %v\nOutput: %s", err, outputStr)
		}
		env.Log("MCP server may already exist, continuing...")
	}

	// List MCP servers again to verify
	cmd = exec.Command("claude", "mcp", "list")
	output, err = cmd.CombinedOutput()
	outputStr = string(output)

	saveTerminalScreenshot(env, "03-mcp-list-after", "MCP Servers (After)", "claude mcp list", outputStr, 0)

	if !strings.Contains(outputStr, "iter") {
		t.Fatalf("iter MCP server not in list after adding:\n%s", outputStr)
	}

	env.Log("MCP server added successfully")

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Step 3: iter MCP server added to Claude")
}

// TestMCPValidation_Step4_BasicQuery tests a basic Claude query using iter MCP tools.
func TestMCPValidation_Step4_BasicQuery(t *testing.T) {
	checkClaudeAuth(t)

	env := common.NewTestEnv(t, "mcp", "validation-step4-query")
	defer env.Cleanup()

	startTime := time.Now()

	// Start service
	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start iter-service: %v", err)
	}

	// Create and register a test project
	client := env.NewHTTPClient()
	projectPath, err := env.CreateTestProject("validation-test")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	_, _, err = client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Remove old MCP config and add fresh one with correct URL
	mcpURL := env.BaseURL + "/mcp/v1"
	exec.Command("claude", "mcp", "remove", "iter-test").Run()
	addCmd := exec.Command("claude", "mcp", "add", "--transport", "http", "iter-test", mcpURL)
	addOutput, addErr := addCmd.CombinedOutput()
	if addErr != nil {
		env.Log("Warning: MCP add command output: %s", string(addOutput))
	}

	// Verify MCP config
	listCmd := exec.Command("claude", "mcp", "list")
	listOutput, _ := listCmd.CombinedOutput()
	env.Log("MCP servers configured: %s", string(listOutput))
	saveTerminalScreenshot(env, "00-mcp-config", "MCP Configuration", "claude mcp list", string(listOutput), 0)

	// Run Claude with a simple query using MCP
	// Use print mode but rely on pre-configured MCP server
	cmd := exec.Command("claude", "-p",
		"--dangerously-skip-permissions",
		"--max-turns", "10",
		"Use the iter-test MCP server to list all projects. Just respond with the project names found.")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	saveTerminalScreenshot(env, "01-claude-query", "Claude MCP Query", "claude -p --max-turns 5 'Use the iter-test MCP server to list all projects...'", outputStr, exitCode)

	// Check for empty output - this is a FAILURE
	if strings.TrimSpace(outputStr) == "" {
		t.Fatal("Claude returned empty output - MCP integration NOT working")
	}

	env.Log("Claude output: %s", outputStr)

	// Check if Claude says it doesn't have MCP access - this is a FAILURE
	outputLower := strings.ToLower(outputStr)
	noAccess := strings.Contains(outputLower, "don't have access") ||
		strings.Contains(outputLower, "no mcp server") ||
		strings.Contains(outputLower, "not configured") ||
		strings.Contains(outputLower, "no mcp tools") ||
		strings.Contains(outputLower, "unable to") ||
		strings.Contains(outputLower, "cannot find")

	if noAccess {
		saveTerminalScreenshot(env, "02-no-mcp-access", "MCP Not Accessible",
			"Claude cannot access MCP server", outputStr, 1)
		t.Fatalf("Claude reports no MCP access - integration NOT working:\n%s", outputStr)
	}

	// Check if Claude acknowledged the MCP tools or returned results
	hasResults := strings.Contains(outputLower, "project") ||
		strings.Contains(outputLower, "validation") ||
		strings.Contains(outputLower, "found") ||
		strings.Contains(outputLower, "listed")

	if !hasResults {
		// Save the failure for analysis
		saveTerminalScreenshot(env, "02-failure-analysis", "Query Failed - No Results",
			"Analysis of Claude output", outputStr, 1)
		t.Fatalf("Claude output does not contain expected MCP results:\n%s", outputStr)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Step 4: Basic Claude MCP query successful")
}

// TestMCPValidation_Full runs all validation steps in sequence.
func TestMCPValidation_Full(t *testing.T) {
	t.Run("Step1_ClaudeInstalled", TestMCPValidation_Step1_ClaudeInstalled)
	t.Run("Step2_ServiceRunning", TestMCPValidation_Step2_ServiceRunning)
	t.Run("Step3_AddMCPServer", TestMCPValidation_Step3_AddMCPServer)
	t.Run("Step4_BasicQuery", TestMCPValidation_Step4_BasicQuery)
}

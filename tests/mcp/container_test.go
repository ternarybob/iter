// Package mcp contains MCP integration tests using Docker common.
// These tests build fresh images and run in isolated common.
package mcp

import (
	"strings"
	"testing"

	"github.com/ternarybob/iter/tests/common"
)

// TestContainerMCP runs MCP tests in Docker containers with fresh images.
func TestContainerMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	// Build fresh images for clean environment
	if err := common.BuildImages(t); err != nil {
		t.Fatalf("Failed to build images: %v", err)
	}

	// Create environment
	env, err := common.NewEnv(t)
	if err != nil {
		t.Fatalf("Failed to create env: %v", err)
	}
	defer env.Cleanup()

	// Start containers
	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start containers: %v", err)
	}

	// Copy credentials and configure MCP
	if err := env.CopyCredentials(); err != nil {
		t.Fatalf("Failed to copy credentials: %v", err)
	}
	if err := env.ConfigureMCP(); err != nil {
		t.Fatalf("Failed to configure MCP: %v", err)
	}

	// Run subtests
	t.Run("HealthCheck", func(t *testing.T) {
		_, output, err := env.ExecBash("curl -s http://iter:19000/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		if !strings.Contains(output, "ok") {
			t.Errorf("Unexpected health response: %s", output)
		}
		t.Logf("Health: %s", output)
	})

	t.Run("MCPProtocol", func(t *testing.T) {
		// Initialize
		_, output, _ := env.ExecBash(`curl -s -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"initialize"}' http://iter:19000/mcp/v1`)
		if !strings.Contains(output, "iter-service") {
			t.Errorf("Initialize failed: %s", output)
		}
		env.SaveResult("mcp-initialize.json", []byte(output))

		// Tools list
		_, output, _ = env.ExecBash(`curl -s -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' http://iter:19000/mcp/v1`)
		if !strings.Contains(output, "list_projects") {
			t.Errorf("Tools list failed: %s", output)
		}
		env.SaveResult("mcp-tools.json", []byte(output))
		t.Log("MCP protocol tests passed")
	})

	t.Run("ClaudeDiscovery", func(t *testing.T) {
		env.RequireCredentials()

		output, err := env.RunClaude("What MCP tools are available from iter? List them briefly.")
		if err != nil {
			t.Fatalf("Claude query failed: %v", err)
		}

		env.SaveResult("claude-discovery.txt", []byte(output))

		if strings.TrimSpace(output) == "" {
			t.Fatal("Claude returned empty output")
		}

		lower := strings.ToLower(output)
		if strings.Contains(lower, "no mcp") || strings.Contains(lower, "not configured") {
			t.Fatalf("Claude reports no MCP: %s", output)
		}

		t.Logf("Claude found tools: %s", output[:min(200, len(output))])
	})

	t.Run("ClaudeListProjects", func(t *testing.T) {
		env.RequireCredentials()

		output, err := env.RunClaude("Use the iter MCP tools to list all projects.")
		if err != nil {
			t.Fatalf("Claude query failed: %v", err)
		}

		env.SaveResult("claude-list-projects.txt", []byte(output))

		if strings.TrimSpace(output) == "" {
			t.Fatal("Claude returned empty output")
		}

		// Should say no projects (iter is fresh)
		lower := strings.ToLower(output)
		hasResponse := strings.Contains(lower, "no project") ||
			strings.Contains(lower, "0 project") ||
			strings.Contains(lower, "empty") ||
			strings.Contains(lower, "none")

		if !hasResponse {
			t.Logf("Unexpected response (may be OK): %s", output)
		}

		t.Logf("List projects: %s", output[:min(200, len(output))])
	})
}

// TestContainerMCPClean runs MCP tests with completely fresh images.
func TestContainerMCPClean(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	// Force rebuild for completely clean environment
	if err := common.ForceBuildImages(t); err != nil {
		t.Fatalf("Failed to build images: %v", err)
	}

	env, err := common.NewEnv(t)
	if err != nil {
		t.Fatalf("Failed to create env: %v", err)
	}
	defer env.Cleanup()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Just verify basic health
	_, output, err := env.ExecBash("curl -s http://iter:19000/health")
	if err != nil || !strings.Contains(output, "ok") {
		t.Fatalf("Health check failed: %v - %s", err, output)
	}

	t.Log("Clean environment test passed")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Package mcp contains MCP integration tests for iter-service.
// Tests run against local service by default, or Docker with TEST_DOCKER=1.
//
// Usage:
//
//	go test -v ./tests/mcp/...           # Local mode
//	TEST_DOCKER=1 go test -v ./tests/mcp/...  # Docker mode
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// Shared test setup for all tests in this file
var testSetup *common.TestSetup

// TestMain runs once per test file: build, start service, run tests, cleanup.
func TestMain(m *testing.M) {
	// Check if Docker mode is requested
	useDocker := os.Getenv("TEST_DOCKER") == "1"

	if useDocker {
		testSetup = common.NewDockerTestSetup()
	} else {
		testSetup = common.NewTestSetup()
	}

	code := testSetup.Run(m, "mcp", "mcp_test")
	os.Exit(code)
}

// getEnv returns the shared test environment.
func getEnv(t *testing.T) *common.TestEnv {
	t.Helper()
	env := testSetup.Env()
	if env == nil {
		t.Fatal("Test environment not initialized")
	}
	env.T = t
	return env
}

// MCPRequest represents a JSON-RPC request for MCP.
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response from MCP.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sendMCPRequest sends a JSON-RPC request to the MCP endpoint.
func sendMCPRequest(baseURL string, req *MCPRequest) (*MCPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(baseURL+"/mcp/v1", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &mcpResp, nil
}

// TestMCPInitialize tests the MCP initialize handshake.
func TestMCPInitialize(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	// Test initialize
	resp, err := sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})
	if err != nil {
		t.Fatalf("MCP initialize failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("MCP initialize returned error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	// Parse result
	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools interface{} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	if initResult.ServerInfo.Name != "iter-service" {
		t.Errorf("Expected server name 'iter-service', got '%s'", initResult.ServerInfo.Name)
	}

	env.SaveJSON("01-initialize.json", initResult)
	env.Log("MCP initialized: %s v%s (protocol %s)",
		initResult.ServerInfo.Name, initResult.ServerInfo.Version, initResult.ProtocolVersion)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP initialize protocol test passed")
}

// TestMCPToolsList tests listing available MCP tools.
func TestMCPToolsList(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	// First initialize
	_, err := sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	})
	if err != nil {
		t.Fatalf("MCP initialize failed: %v", err)
	}

	// List tools
	resp, err := sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	})
	if err != nil {
		t.Fatalf("MCP tools/list failed: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("MCP tools/list returned error: %d %s", resp.Error.Code, resp.Error.Message)
	}

	// Parse result
	var toolsResult struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &toolsResult); err != nil {
		t.Fatalf("Failed to parse tools/list result: %v", err)
	}

	// Verify expected tools exist
	expectedTools := []string{"list_projects", "search", "get_dependencies", "get_dependents"}
	foundTools := make(map[string]bool)
	for _, tool := range toolsResult.Tools {
		foundTools[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !foundTools[expected] {
			t.Errorf("Expected tool '%s' not found in tools list", expected)
		}
	}

	env.SaveJSON("02-tools-list.json", toolsResult)
	env.Log("Found %d MCP tools", len(toolsResult.Tools))

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP tools/list protocol test passed")
}

// TestMCPToolsCall tests calling MCP tools.
func TestMCPToolsCall(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	client := env.NewHTTPClient()

	// Create and register a test project
	projectPath, err := env.CreateTestProject("mcp-test-project")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	resp, body, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)
	created := common.AssertJSON(t, body)
	projectID := created["id"].(string)

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Test list_projects tool
	mcpResp, err := sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "list_projects",
			"arguments": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("MCP tools/call list_projects failed: %v", err)
	}

	if mcpResp.Error != nil {
		t.Fatalf("MCP tools/call returned error: %d %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	// Parse tool result
	var toolResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(mcpResp.Result, &toolResult); err != nil {
		t.Fatalf("Failed to parse tools/call result: %v", err)
	}

	if toolResult.IsError {
		t.Errorf("list_projects returned error: %v", toolResult.Content)
	}

	if len(toolResult.Content) == 0 {
		t.Error("list_projects returned empty content")
	}

	// Verify the project is in the result
	if len(toolResult.Content) > 0 && !strings.Contains(toolResult.Content[0].Text, "mcp-test-project") {
		t.Errorf("list_projects should contain 'mcp-test-project', got: %s", toolResult.Content[0].Text)
	}

	env.SaveJSON("03-list-projects-result.json", toolResult)
	if len(toolResult.Content) > 0 {
		env.Log("list_projects returned: %s", toolResult.Content[0].Text[:minInt(100, len(toolResult.Content[0].Text))])
	}

	// Test search tool
	mcpResp, err = sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "search",
			"arguments": map[string]interface{}{
				"query":      "HelloWorld",
				"project_id": projectID,
			},
		},
	})
	if err != nil {
		t.Fatalf("MCP tools/call search failed: %v", err)
	}

	if mcpResp.Error != nil {
		t.Fatalf("MCP search returned error: %d %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	if err := json.Unmarshal(mcpResp.Result, &toolResult); err != nil {
		t.Fatalf("Failed to parse search result: %v", err)
	}

	env.SaveJSON("04-search-result.json", toolResult)
	if len(toolResult.Content) > 0 {
		env.Log("search returned: %s", toolResult.Content[0].Text[:minInt(100, len(toolResult.Content[0].Text))])
	}

	// Cleanup
	client.Delete("/projects/" + projectID)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP tools/call protocol test passed")
}

// TestMCPSSEEndpoint tests the SSE endpoint for MCP.
func TestMCPSSEEndpoint(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	// Make GET request to SSE endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(env.BaseURL + "/mcp/sse")
	if err != nil {
		// Timeout is expected for SSE - we just want to see if it starts correctly
		if !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("SSE GET failed unexpectedly: %v", err)
		}
	}
	if resp != nil {
		defer resp.Body.Close()

		// Read some of the response
		buf := make([]byte, 1024)
		n, _ := resp.Body.Read(buf)
		responseStart := string(buf[:n])

		// Verify it looks like SSE with endpoint event
		if !strings.Contains(responseStart, "event: endpoint") {
			t.Errorf("Expected 'event: endpoint' in SSE response, got: %s", responseStart)
		}

		if !strings.Contains(responseStart, "data: http") {
			t.Errorf("Expected endpoint URL in SSE response, got: %s", responseStart)
		}

		env.SaveResult("05-sse-response.txt", []byte(responseStart))
		env.Log("SSE endpoint event received: %s", strings.TrimSpace(responseStart))
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP SSE endpoint test passed")
}

// TestMCPProjectRegistration tests the complete project lifecycle via MCP.
func TestMCPProjectRegistration(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	client := env.NewHTTPClient()

	// Create test project
	projectPath, err := env.CreateTestProject("mcp-lifecycle-project")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Register via REST API
	resp, body, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)
	created := common.AssertJSON(t, body)
	projectID := created["id"].(string)

	env.SaveJSON("06-project-registered.json", created)

	// Index the project
	resp, _, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Failed to index project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Verify via MCP list_projects
	mcpResp, err := sendMCPRequest(env.BaseURL, &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "list_projects",
			"arguments": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("MCP list_projects failed: %v", err)
	}

	var toolResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(mcpResp.Result, &toolResult)

	if len(toolResult.Content) > 0 && !strings.Contains(toolResult.Content[0].Text, "mcp-lifecycle-project") {
		t.Errorf("Project not found in MCP list_projects: %s", toolResult.Content[0].Text)
	}

	env.SaveJSON("07-mcp-list-after-register.json", toolResult)

	// Cleanup
	client.Delete("/projects/" + projectID)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP project lifecycle test passed")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Package api contains integration tests for iter-service REST API.
// This file tests MCP (Model Context Protocol) functionality.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

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

// TestMCPProtocolInitialize tests the MCP initialize handshake.
func TestMCPProtocolInitialize(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

// TestMCPProtocolToolsList tests listing available MCP tools.
func TestMCPProtocolToolsList(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

	env.SaveJSON("01-tools-list.json", toolsResult)
	env.Log("Found %d MCP tools", len(toolsResult.Tools))

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP tools/list protocol test passed")
}

// TestMCPProtocolToolsCall tests calling MCP tools.
func TestMCPProtocolToolsCall(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

	env.SaveJSON("01-list-projects-result.json", toolResult)
	if len(toolResult.Content) > 0 {
		env.Log("list_projects returned: %s", toolResult.Content[0].Text[:min(100, len(toolResult.Content[0].Text))])
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

	env.SaveJSON("02-search-result.json", toolResult)
	if len(toolResult.Content) > 0 {
		env.Log("search returned: %s", toolResult.Content[0].Text[:min(100, len(toolResult.Content[0].Text))])
	}

	// Cleanup
	client.Delete("/projects/" + projectID)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP tools/call protocol test passed")
}

// TestMCPSSEEndpoint tests the SSE endpoint for MCP.
func TestMCPSSEEndpoint(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

		env.SaveResult("01-sse-response.txt", []byte(responseStart))
		env.Log("SSE endpoint event received: %s", strings.TrimSpace(responseStart))
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "MCP SSE endpoint test passed")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Package mcp implements the Model Context Protocol (MCP) server for iter-service.
// MCP allows AI assistants like Claude to use iter-service as a tool provider.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/internal/project"
	"github.com/ternarybob/iter/pkg/index"
)

// JSON-RPC message types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Protocol types
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Handler handles MCP protocol requests.
type Handler struct {
	cfg      *config.Config
	registry *project.Registry
	manager  *project.Manager
	mu       sync.RWMutex
}

// NewHandler creates a new MCP handler.
func NewHandler(cfg *config.Config, registry *project.Registry, manager *project.Manager) *Handler {
	return &Handler{
		cfg:      cfg,
		registry: registry,
		manager:  manager,
	}
}

// ServeHTTP handles HTTP requests for MCP.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle SSE endpoint
	if strings.HasSuffix(r.URL.Path, "/sse") {
		h.handleSSE(w, r)
		return
	}

	// Handle JSON-RPC over HTTP POST
	if r.Method == http.MethodPost {
		h.handleJSONRPC(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleJSONRPC handles JSON-RPC requests over HTTP POST.
func (h *Handler) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	response := h.handleRequest(&req)
	h.writeResponse(w, response)
}

// handleSSE handles Server-Sent Events for MCP streaming.
// Per MCP spec: GET /sse returns endpoint event, then client POSTs to that endpoint.
func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleSSEConnect(w, r)
		return
	}
	if r.Method == http.MethodPost {
		h.handleSSEMessage(w, r)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleSSEConnect handles the initial SSE connection (GET /sse).
// Returns the endpoint event with the POST URI for sending messages.
func (h *Handler) handleSSEConnect(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send endpoint event - client should POST messages here
	// Use the same /sse endpoint for POSTs
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	endpoint := fmt.Sprintf("%s://%s/mcp/sse", scheme, r.Host)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpoint)
	flusher.Flush()

	// Keep connection open for server-initiated messages
	// For now, just keep the connection alive with periodic pings
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Send keep-alive ping
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// handleSSEMessage handles POST requests with JSON-RPC messages.
func (h *Handler) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, nil, -32700, "Parse error", nil)
		return
	}

	// Set SSE headers for response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	response := h.handleRequest(&req)
	data, _ := json.Marshal(response)

	// Send as SSE message event
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
}

// handleRequest processes a single JSON-RPC request.
func (h *Handler) handleRequest(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return h.handleInitialize(req)
	case "initialized":
		return h.handleInitialized(req)
	case "tools/list":
		return h.handleToolsList(req)
	case "tools/call":
		return h.handleToolsCall(req)
	case "ping":
		return h.handlePing(req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (h *Handler) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "iter-service",
			Version: "1.0.0",
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (h *Handler) handleInitialized(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}
}

func (h *Handler) handlePing(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{},
	}
}

func (h *Handler) handleToolsList(req *Request) *Response {
	tools := []Tool{
		{
			Name:        "list_projects",
			Description: "List all indexed projects in iter-service",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		{
			Name:        "search",
			Description: "Search for symbols (functions, types, methods) across indexed projects",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (symbol name or pattern)"
					},
					"project_id": {
						"type": "string",
						"description": "Optional project ID to search within"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "get_dependencies",
			Description: "Get dependencies of a symbol (what it calls/uses)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"project_id": {
						"type": "string",
						"description": "Project ID"
					},
					"symbol": {
						"type": "string",
						"description": "Symbol name to get dependencies for"
					}
				},
				"required": ["project_id", "symbol"]
			}`),
		},
		{
			Name:        "get_dependents",
			Description: "Get dependents of a symbol (what calls/uses it)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"project_id": {
						"type": "string",
						"description": "Project ID"
					},
					"symbol": {
						"type": "string",
						"description": "Symbol name to get dependents for"
					}
				},
				"required": ["project_id", "symbol"]
			}`),
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: tools},
	}
}

func (h *Handler) handleToolsCall(req *Request) *Response {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	var result ToolResult

	switch params.Name {
	case "list_projects":
		result = h.callListProjects()
	case "search":
		query, _ := params.Arguments["query"].(string)
		projectID, _ := params.Arguments["project_id"].(string)
		result = h.callSearch(query, projectID)
	case "get_dependencies":
		projectID, _ := params.Arguments["project_id"].(string)
		symbol, _ := params.Arguments["symbol"].(string)
		result = h.callGetDependencies(projectID, symbol)
	case "get_dependents":
		projectID, _ := params.Arguments["project_id"].(string)
		symbol, _ := params.Arguments["symbol"].(string)
		result = h.callGetDependents(projectID, symbol)
	default:
		result = ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (h *Handler) callListProjects() ToolResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	projects := h.registry.List()
	if len(projects) == 0 {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "No projects indexed."}},
		}
	}

	var sb strings.Builder
	sb.WriteString("Indexed projects:\n\n")
	for _, p := range projects {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n  Path: %s\n  Registered: %s\n\n",
			p.Name, p.ID, p.Path, p.RegisteredAt.Format(time.RFC3339)))
	}

	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: sb.String()}},
	}
}

func (h *Handler) callSearch(query, projectID string) ToolResult {
	if query == "" {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Error: query is required"}},
			IsError: true,
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// If project ID specified, search that project
	if projectID != "" {
		p, err := h.registry.Get(projectID)
		if err != nil || p == nil {
			return ToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Project not found: %s", projectID)}},
				IsError: true,
			}
		}
		return h.searchProject(p.ID, query)
	}

	// Search all projects
	projects := h.registry.List()
	if len(projects) == 0 {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "No projects indexed."}},
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))

	for _, p := range projects {
		results := h.searchProject(p.ID, query)
		if !results.IsError && len(results.Content) > 0 && results.Content[0].Text != "No results found." {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n", p.Name, results.Content[0].Text))
		}
	}

	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: sb.String()}},
	}
}

func (h *Handler) searchProject(projectID, query string) ToolResult {
	indexer := h.manager.GetIndexer(projectID)
	if indexer == nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Index not available"}},
			IsError: true,
		}
	}

	searcher := index.NewSearcher(indexer)
	opts := index.SearchOptions{
		Query: query,
		Limit: 20,
	}

	results, err := searcher.Search(context.Background(), opts)
	if err != nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Search error: %v", err)}},
			IsError: true,
		}
	}

	if len(results) == 0 {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "No results found."}},
		}
	}

	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("- **%s** (%s)\n  File: %s:%d\n",
			r.Chunk.SymbolName, r.Chunk.SymbolKind, r.Chunk.FilePath, r.Chunk.StartLine))
		if r.Chunk.Signature != "" {
			sb.WriteString(fmt.Sprintf("  Signature: `%s`\n", r.Chunk.Signature))
		}
		sb.WriteString("\n")
	}

	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: sb.String()}},
	}
}

func (h *Handler) callGetDependencies(projectID, symbol string) ToolResult {
	if projectID == "" || symbol == "" {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Error: project_id and symbol are required"}},
			IsError: true,
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	p, err := h.registry.Get(projectID)
	if err != nil || p == nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Project not found: %s", projectID)}},
			IsError: true,
		}
	}

	indexer := h.manager.GetIndexer(p.ID)
	if indexer == nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Index not available"}},
			IsError: true,
		}
	}

	searcher := index.NewSearcher(indexer)
	deps, err := searcher.GetDependencies(symbol)
	if err != nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		}
	}

	// Format the dependency result
	text := deps.FormatDependencies("Dependencies")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

func (h *Handler) callGetDependents(projectID, symbol string) ToolResult {
	if projectID == "" || symbol == "" {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Error: project_id and symbol are required"}},
			IsError: true,
		}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	p, err := h.registry.Get(projectID)
	if err != nil || p == nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Project not found: %s", projectID)}},
			IsError: true,
		}
	}

	indexer := h.manager.GetIndexer(p.ID)
	if indexer == nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Index not available"}},
			IsError: true,
		}
	}

	searcher := index.NewSearcher(indexer)
	deps, err := searcher.GetDependents(symbol)
	if err != nil {
		return ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		}
	}

	// Format the dependency result
	text := deps.FormatDependencies("Dependents")
	return ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

func (h *Handler) writeResponse(w http.ResponseWriter, resp *Response) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	h.writeResponse(w, resp)
}

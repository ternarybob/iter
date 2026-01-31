package index

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPServer wraps the indexer to provide MCP tool access.
type MCPServer struct {
	indexer *Indexer
	server  *server.MCPServer
}

// NewMCPServer creates a new MCP server with the given indexer.
func NewMCPServer(indexer *Indexer) *MCPServer {
	s := &MCPServer{
		indexer: indexer,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"iter-index",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	s.registerTools(mcpServer)

	s.server = mcpServer
	return s
}

// registerTools registers all MCP tools with the server.
func (s *MCPServer) registerTools(mcpServer *server.MCPServer) {
	// search - Semantic code search
	mcpServer.AddTool(
		mcp.NewTool("search",
			mcp.WithDescription("Semantic code search. Search for functions, types, and symbols in the codebase."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query (e.g., 'HTTP handler', 'parse config', 'error handling')"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results (default: 10)"),
			),
			mcp.WithString("kind",
				mcp.Description("Filter by symbol kind: function, method, type, const, var"),
			),
			mcp.WithString("path",
				mcp.Description("Filter by file path prefix (e.g., 'cmd/', 'internal/')"),
			),
		),
		s.handleSearch,
	)

	// deps - Get dependencies for a symbol
	mcpServer.AddTool(
		mcp.NewTool("deps",
			mcp.WithDescription("Get dependencies of a symbol. Shows what the symbol depends on (imports, calls, uses)."),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Symbol name to analyze (e.g., 'NewIndexer', 'Search')"),
			),
		),
		s.handleDeps,
	)

	// dependents - Get dependents of a symbol
	mcpServer.AddTool(
		mcp.NewTool("dependents",
			mcp.WithDescription("Get dependents of a symbol. Shows what depends on this symbol."),
			mcp.WithString("symbol",
				mcp.Required(),
				mcp.Description("Symbol name to analyze (e.g., 'NewIndexer', 'Config')"),
			),
		),
		s.handleDependents,
	)

	// impact - Change impact analysis for a file
	mcpServer.AddTool(
		mcp.NewTool("impact",
			mcp.WithDescription("Analyze the impact of changes to a file. Shows direct and transitive dependents."),
			mcp.WithString("file",
				mcp.Required(),
				mcp.Description("Relative file path to analyze (e.g., 'index/search.go')"),
			),
		),
		s.handleImpact,
	)

	// history - Commit history with summaries
	mcpServer.AddTool(
		mcp.NewTool("history",
			mcp.WithDescription("Get recent commit history with summaries."),
			mcp.WithNumber("limit",
				mcp.Description("Number of commits to show (default: 10)"),
			),
		),
		s.handleHistory,
	)

	// stats - Index statistics
	mcpServer.AddTool(
		mcp.NewTool("stats",
			mcp.WithDescription("Get index statistics including document count, file count, and DAG info."),
		),
		s.handleStats,
	)

	// reindex - Trigger full reindex
	mcpServer.AddTool(
		mcp.NewTool("reindex",
			mcp.WithDescription("Trigger a full reindex of the codebase. Use when the index seems stale or after major changes."),
		),
		s.handleReindex,
	)
}

// handleSearch handles the search tool.
func (s *MCPServer) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")
	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	opts := SearchOptions{
		Query:      query,
		Limit:      request.GetInt("limit", 10),
		SymbolKind: request.GetString("kind", ""),
		FilePath:   request.GetString("path", ""),
	}

	searcher := NewSearcher(s.indexer)
	results, err := searcher.Search(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	return mcp.NewToolResultText(FormatResults(results)), nil
}

// handleDeps handles the deps tool.
func (s *MCPServer) handleDeps(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := request.GetString("symbol", "")
	if symbol == "" {
		return mcp.NewToolResultError("symbol parameter is required"), nil
	}

	searcher := NewSearcher(s.indexer)
	deps, err := searcher.GetDependencies(symbol)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get dependencies failed: %v", err)), nil
	}

	return mcp.NewToolResultText(deps.FormatDependencies("Dependencies")), nil
}

// handleDependents handles the dependents tool.
func (s *MCPServer) handleDependents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	symbol := request.GetString("symbol", "")
	if symbol == "" {
		return mcp.NewToolResultError("symbol parameter is required"), nil
	}

	searcher := NewSearcher(s.indexer)
	dependents, err := searcher.GetDependents(symbol)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get dependents failed: %v", err)), nil
	}

	return mcp.NewToolResultText(dependents.FormatDependencies("Dependents")), nil
}

// handleImpact handles the impact tool.
func (s *MCPServer) handleImpact(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file := request.GetString("file", "")
	if file == "" {
		return mcp.NewToolResultError("file parameter is required"), nil
	}

	searcher := NewSearcher(s.indexer)
	impact, err := searcher.GetImpact(file)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get impact failed: %v", err)), nil
	}

	return mcp.NewToolResultText(impact.FormatImpact()), nil
}

// handleHistory handles the history tool.
func (s *MCPServer) handleHistory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := request.GetInt("limit", 10)

	lineage := s.indexer.GetLineage()
	if lineage == nil {
		return mcp.NewToolResultError("lineage tracking not initialized"), nil
	}

	summaries, err := lineage.GetRecentHistory(limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get history failed: %v", err)), nil
	}

	return mcp.NewToolResultText(FormatHistory(summaries)), nil
}

// handleStats handles the stats tool.
func (s *MCPServer) handleStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats := s.indexer.Stats()

	// Get DAG stats if available
	dag := s.indexer.GetDAG()
	var dagStats DAGStats
	if dag != nil {
		dagStats = dag.Stats()
	}

	// Get lineage stats if available
	lineage := s.indexer.GetLineage()
	var lineageStats LineageStats
	if lineage != nil {
		lineageStats = lineage.Stats()
	}

	result := map[string]interface{}{
		"index": map[string]interface{}{
			"document_count": stats.DocumentCount,
			"file_count":     stats.FileCount,
			"current_branch": stats.CurrentBranch,
			"last_updated":   stats.LastUpdated.Format("2006-01-02 15:04:05"),
		},
		"dag": map[string]interface{}{
			"node_count":    dagStats.NodeCount,
			"edge_count":    dagStats.EdgeCount,
			"file_count":    dagStats.FileCount,
			"package_count": dagStats.PackageCount,
		},
		"lineage": map[string]interface{}{
			"total_summaries": lineageStats.TotalSummaries,
			"llm_summaries":   lineageStats.LLMSummaries,
		},
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal stats failed: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleReindex handles the reindex tool.
func (s *MCPServer) handleReindex(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	err := s.indexer.IndexAll()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reindex failed: %v", err)), nil
	}

	stats := s.indexer.Stats()
	return mcp.NewToolResultText(fmt.Sprintf("Reindex complete. Indexed %d symbols from %d files.",
		stats.DocumentCount, stats.FileCount)), nil
}

// ServeStdio starts the MCP server on stdio.
func (s *MCPServer) ServeStdio() error {
	return server.ServeStdio(s.server)
}

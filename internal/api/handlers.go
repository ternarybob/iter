package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ternarybob/iter/pkg/index"
	"github.com/ternarybob/iter/web"
)

// version is set via -ldflags at build time
var version = "dev"

// SetVersion sets the version string (called from main).
func SetVersion(v string) {
	version = v
}

// Response types

// HealthResponse is the response for /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// VersionResponse is the response for /version.
type VersionResponse struct {
	Version string `json:"version"`
	Service string `json:"service"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ProjectResponse represents a project in API responses.
type ProjectResponse struct {
	ID           string              `json:"id"`
	Path         string              `json:"path"`
	Name         string              `json:"name"`
	IndexStats   *IndexStatsResponse `json:"index_stats,omitempty"`
	RegisteredAt string              `json:"registered_at"`
}

// IndexStatsResponse represents index statistics.
type IndexStatsResponse struct {
	DocumentCount int    `json:"document_count"`
	FileCount     int    `json:"file_count"`
	CurrentBranch string `json:"current_branch"`
	LastUpdated   string `json:"last_updated"`
}

// IndexStatusResponse represents the overall index status including API key status.
type IndexStatusResponse struct {
	GeminiAPIKeyConfigured bool                         `json:"gemini_api_key_configured"`
	GeminiAPIKeyStatus     string                       `json:"gemini_api_key_status"`
	Projects               []ProjectIndexStatusResponse `json:"projects"`
}

// ProjectIndexStatusResponse represents index status for a single project.
type ProjectIndexStatusResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	IndexStatus   string `json:"index_status"`
	DocumentCount int    `json:"document_count"`
	FileCount     int    `json:"file_count"`
	ErrorMessage  string `json:"error_message,omitempty"`
	LastUpdated   string `json:"last_updated,omitempty"`
}

// RegisterProjectRequest is the request body for registering a project.
type RegisterProjectRequest struct {
	Path string `json:"path"`
}

// SearchRequest is the request body for search.
type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Path  string `json:"path,omitempty"`
}

// SearchResponse wraps search results.
type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
	Query   string             `json:"query"`
	Total   int                `json:"total"`
}

// SearchResultItem represents a single search result.
type SearchResultItem struct {
	SymbolName string  `json:"symbol_name"`
	SymbolKind string  `json:"symbol_kind"`
	FilePath   string  `json:"file_path"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Signature  string  `json:"signature"`
	Score      float32 `json:"score"`
}

// Handlers

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, VersionResponse{
		Version: version,
		Service: "iter-service",
	})
}

func (s *Server) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	// Check GEMINI_API_KEY status
	apiKeyConfigured := s.cfg.LLM.APIKey != ""
	apiKeyStatus := "Not configured"
	if apiKeyConfigured {
		apiKeyStatus = "Configured"
	} else {
		apiKeyStatus = "GEMINI_API_KEY not provided"
	}

	// Get all projects and their index status
	projects := s.registry.List()
	projectStatuses := make([]ProjectIndexStatusResponse, 0, len(projects))

	for _, p := range projects {
		status := ProjectIndexStatusResponse{
			ID:   p.ID,
			Name: p.Name,
			Path: p.Path,
		}

		idx := s.manager.GetIndexer(p.ID)
		if idx == nil {
			status.IndexStatus = "not_indexed"
			status.ErrorMessage = "Indexer not initialized"
		} else {
			stats := idx.Stats()
			status.DocumentCount = stats.DocumentCount
			status.FileCount = stats.FileCount
			status.LastUpdated = stats.LastUpdated.Format("2006-01-02T15:04:05Z")

			if !apiKeyConfigured {
				status.IndexStatus = "api_key_missing"
				status.ErrorMessage = "GEMINI_API_KEY not provided - semantic indexing unavailable"
			} else if stats.DocumentCount == 0 {
				status.IndexStatus = "empty"
				status.ErrorMessage = "No documents indexed"
			} else {
				status.IndexStatus = "indexed"
			}
		}

		projectStatuses = append(projectStatuses, status)
	}

	response := IndexStatusResponse{
		GeminiAPIKeyConfigured: apiKeyConfigured,
		GeminiAPIKeyStatus:     apiKeyStatus,
		Projects:               projectStatuses,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects := s.registry.List()
	response := make([]ProjectResponse, 0, len(projects))

	for _, p := range projects {
		pr := ProjectResponse{
			ID:           p.ID,
			Path:         p.Path,
			Name:         p.Name,
			RegisteredAt: p.RegisteredAt.Format("2006-01-02T15:04:05Z"),
		}

		// Get index stats if indexer is available
		if idx := s.manager.GetIndexer(p.ID); idx != nil {
			stats := idx.Stats()
			pr.IndexStats = &IndexStatsResponse{
				DocumentCount: stats.DocumentCount,
				FileCount:     stats.FileCount,
				CurrentBranch: stats.CurrentBranch,
				LastUpdated:   stats.LastUpdated.Format("2006-01-02T15:04:05Z"),
			}
		}

		response = append(response, pr)
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRegisterProject(w http.ResponseWriter, r *http.Request) {
	var req RegisterProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "Path is required")
		return
	}

	project, err := s.manager.RegisterProject(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := ProjectResponse{
		ID:           project.ID,
		Path:         project.Path,
		Name:         project.Name,
		RegisteredAt: project.RegisteredAt.Format("2006-01-02T15:04:05Z"),
	}

	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	project, err := s.registry.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Project not found")
		return
	}

	response := ProjectResponse{
		ID:           project.ID,
		Path:         project.Path,
		Name:         project.Name,
		RegisteredAt: project.RegisteredAt.Format("2006-01-02T15:04:05Z"),
	}

	// Get index stats if indexer is available
	if idx := s.manager.GetIndexer(id); idx != nil {
		stats := idx.Stats()
		response.IndexStats = &IndexStatsResponse{
			DocumentCount: stats.DocumentCount,
			FileCount:     stats.FileCount,
			CurrentBranch: stats.CurrentBranch,
			LastUpdated:   stats.LastUpdated.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleUnregisterProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.manager.UnregisterProject(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRebuildIndex(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	if err := idx.IndexAll(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to rebuild index: "+err.Error())
		return
	}

	stats := idx.Stats()
	writeJSON(w, http.StatusOK, IndexStatsResponse{
		DocumentCount: stats.DocumentCount,
		FileCount:     stats.FileCount,
		CurrentBranch: stats.CurrentBranch,
		LastUpdated:   stats.LastUpdated.Format("2006-01-02T15:04:05Z"),
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "Query is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	opts := index.SearchOptions{
		Query:      req.Query,
		Limit:      req.Limit,
		SymbolKind: req.Kind,
		FilePath:   req.Path,
	}

	searcher := index.NewSearcher(idx)
	results, err := searcher.Search(context.Background(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	response := SearchResponse{
		Query:   req.Query,
		Total:   len(results),
		Results: make([]SearchResultItem, 0, len(results)),
	}

	for _, r := range results {
		response.Results = append(response.Results, SearchResultItem{
			SymbolName: r.Chunk.SymbolName,
			SymbolKind: r.Chunk.SymbolKind,
			FilePath:   r.Chunk.FilePath,
			StartLine:  r.Chunk.StartLine,
			EndLine:    r.Chunk.EndLine,
			Signature:  r.Chunk.Signature,
			Score:      r.Score,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetDeps(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	symbol := chi.URLParam(r, "symbol")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	searcher := index.NewSearcher(idx)
	deps, err := searcher.GetDependencies(symbol)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, deps)
}

func (s *Server) handleGetDependents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	symbol := chi.URLParam(r, "symbol")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	searcher := index.NewSearcher(idx)
	dependents, err := searcher.GetDependents(symbol)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, dependents)
}

func (s *Server) handleGetImpact(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	file := chi.URLParam(r, "file")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	searcher := index.NewSearcher(idx)
	impact, err := searcher.GetImpact(file)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, impact)
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		writeError(w, http.StatusNotFound, "Project not found or indexer not available")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	lineage := idx.GetLineage()
	if lineage == nil {
		writeError(w, http.StatusNotFound, "Lineage tracking not initialized")
		return
	}

	summaries, err := lineage.GetRecentHistory(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) handleWebRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/web/", http.StatusFound)
}

// Web UI template data types

// WebIndexData is the data for the index page template.
type WebIndexData struct {
	Version string
}

// WebProjectListData is the data for the project list partial.
type WebProjectListData struct {
	Projects []WebProjectData
}

// WebProjectData is the data for a single project in templates.
type WebProjectData struct {
	ID         string
	Name       string
	Path       string
	IndexStats *WebIndexStatsData
}

// WebIndexStatsData is the data for index stats in templates.
type WebIndexStatsData struct {
	DocumentCount int
	FileCount     int
	CurrentBranch string
	LastUpdated   string
}

// WebSearchResultsData is the data for search results partial.
type WebSearchResultsData struct {
	Query   string
	Total   int
	Results []WebSearchResultItem
}

// WebSearchResultItem is a single search result for templates.
type WebSearchResultItem struct {
	SymbolName string
	SymbolKind string
	FilePath   string
	StartLine  int
	EndLine    int
	Signature  string
}

func (s *Server) handleWebAssets(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/web")
	if path == "" || path == "/" {
		s.renderIndex(w, r)
		return
	}

	// Serve static files
	if strings.HasPrefix(path, "/static/") {
		s.serveStaticFile(w, r, path)
		return
	}

	// Handle specific pages
	switch {
	case strings.HasPrefix(path, "/project/"):
		s.renderProjectPage(w, r, strings.TrimPrefix(path, "/project/"))
	case path == "/settings":
		s.renderSettings(w, r)
	case path == "/docs":
		s.renderDocs(w, r)
	case path == "/mcp":
		s.renderMCP(w, r)
	case path == "/index-status":
		s.renderIndexStatus(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveStaticFile(w http.ResponseWriter, r *http.Request, path string) {
	// Remove leading slash for fs.Sub
	fsPath := strings.TrimPrefix(path, "/")

	// Get the static file system
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Determine content type
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// Serve the file
	fileName := strings.TrimPrefix(fsPath, "static/")
	data, err := fs.ReadFile(staticFS, fileName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Write(data)
}

func (s *Server) renderIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(web.Templates, "templates/index.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := WebIndexData{
		Version: version,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderProjectPage(w http.ResponseWriter, r *http.Request, projectID string) {
	tmpl, err := template.ParseFS(web.Templates, "templates/project.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	project, err := s.registry.Get(projectID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := WebProjectData{
		ID:   project.ID,
		Name: project.Name,
		Path: project.Path,
	}

	// Get index stats if indexer is available
	if idx := s.manager.GetIndexer(projectID); idx != nil {
		stats := idx.Stats()
		data.IndexStats = &WebIndexStatsData{
			DocumentCount: stats.DocumentCount,
			FileCount:     stats.FileCount,
			CurrentBranch: stats.CurrentBranch,
			LastUpdated:   stats.LastUpdated.Format("Jan 2, 2006 3:04 PM"),
		}
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Settings - iter-service</title>
    <link rel="stylesheet" href="/web/static/styles.css">
</head>
<body>
    <header class="header">
        <h1>
            <a href="/" style="color: inherit;">
                <svg class="logo" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                </svg>
                iter-service
            </a>
        </h1>
        <nav>
            <a href="/">Projects</a>
            <a href="/web/mcp">MCP Setup</a>
            <a href="/web/settings" class="active">Settings</a>
            <a href="/web/docs">API Docs</a>
        </nav>
    </header>
    <main class="container">
        <div class="card">
            <h2 class="card-title">Settings</h2>
            <p style="color: var(--text-muted);">Settings page coming soon.</p>
        </div>
    </main>
</body>
</html>`))
}

func (s *Server) renderDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Docs - iter-service</title>
    <link rel="stylesheet" href="/web/static/styles.css">
</head>
<body>
    <header class="header">
        <h1>
            <a href="/" style="color: inherit;">
                <svg class="logo" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                </svg>
                iter-service
            </a>
        </h1>
        <nav>
            <a href="/">Projects</a>
            <a href="/web/mcp">MCP Setup</a>
            <a href="/web/settings">Settings</a>
            <a href="/web/docs" class="active">API Docs</a>
        </nav>
    </header>
    <main class="container">
        <div class="card">
            <h2 class="card-title">API Documentation</h2>
            <table style="width: 100%; border-collapse: collapse; margin-top: 1rem;">
                <thead>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <th style="text-align: left; padding: 0.75rem;">Method</th>
                        <th style="text-align: left; padding: 0.75rem;">Endpoint</th>
                        <th style="text-align: left; padding: 0.75rem;">Description</th>
                    </tr>
                </thead>
                <tbody>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/health</code></td>
                        <td style="padding: 0.75rem;">Health check</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/version</code></td>
                        <td style="padding: 0.75rem;">Service version</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects</code></td>
                        <td style="padding: 0.75rem;">List all registered projects</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--warning-color);">POST</code></td>
                        <td style="padding: 0.75rem;"><code>/projects</code></td>
                        <td style="padding: 0.75rem;">Register a new project (body: <code>{"path": "/path/to/repo"}</code>)</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}</code></td>
                        <td style="padding: 0.75rem;">Get project details</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--error-color);">DELETE</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}</code></td>
                        <td style="padding: 0.75rem;">Unregister a project</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--warning-color);">POST</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/index</code></td>
                        <td style="padding: 0.75rem;">Rebuild project index</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--warning-color);">POST</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/search</code></td>
                        <td style="padding: 0.75rem;">Semantic code search (body: <code>{"query": "...", "limit": 10}</code>)</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/deps/{symbol}</code></td>
                        <td style="padding: 0.75rem;">Get symbol dependencies</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/dependents/{symbol}</code></td>
                        <td style="padding: 0.75rem;">Get symbol dependents</td>
                    </tr>
                    <tr style="border-bottom: 1px solid var(--border-color);">
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/impact/{file}</code></td>
                        <td style="padding: 0.75rem;">File impact analysis</td>
                    </tr>
                    <tr>
                        <td style="padding: 0.75rem;"><code style="color: var(--success-color);">GET</code></td>
                        <td style="padding: 0.75rem;"><code>/projects/{id}/history</code></td>
                        <td style="padding: 0.75rem;">Get commit history</td>
                    </tr>
                </tbody>
            </table>
        </div>
    </main>
</body>
</html>`))
}

func (s *Server) renderIndexStatus(w http.ResponseWriter, r *http.Request) {
	// Check GEMINI_API_KEY status
	apiKeyConfigured := s.cfg.LLM.APIKey != ""
	apiKeyStatus := "Configured"
	apiKeyClass := "success"
	if !apiKeyConfigured {
		apiKeyStatus = "GEMINI_API_KEY not provided"
		apiKeyClass = "error"
	}

	// Get all projects and their index status
	projects := s.registry.List()

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Index Status - iter-service</title>
    <link rel="stylesheet" href="/web/static/styles.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        .status-card {
            background-color: var(--surface-color);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .status-card h3 {
            margin-top: 0;
            margin-bottom: 1rem;
            color: var(--text-color);
        }
        .status-indicator {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            font-weight: 500;
        }
        .status-indicator.success {
            background-color: rgba(158, 206, 106, 0.1);
            color: var(--success-color);
            border: 1px solid var(--success-color);
        }
        .status-indicator.error {
            background-color: rgba(247, 118, 142, 0.1);
            color: var(--error-color);
            border: 1px solid var(--error-color);
        }
        .status-indicator.warning {
            background-color: rgba(224, 175, 104, 0.1);
            color: var(--warning-color);
            border: 1px solid var(--warning-color);
        }
        .project-status-table {
            width: 100%%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        .project-status-table th,
        .project-status-table td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid var(--border-color);
        }
        .project-status-table th {
            color: var(--text-muted);
            font-weight: 500;
        }
        .status-badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 500;
        }
        .status-badge.indexed {
            background-color: rgba(158, 206, 106, 0.2);
            color: var(--success-color);
        }
        .status-badge.empty {
            background-color: rgba(224, 175, 104, 0.2);
            color: var(--warning-color);
        }
        .status-badge.api_key_missing {
            background-color: rgba(247, 118, 142, 0.2);
            color: var(--error-color);
        }
        .status-badge.not_indexed {
            background-color: rgba(169, 177, 214, 0.2);
            color: var(--text-muted);
        }
        .refresh-btn {
            float: right;
        }
    </style>
</head>
<body>
    <header class="header">
        <h1>
            <a href="/" style="color: inherit;">
                <svg class="logo" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                </svg>
                iter-service
            </a>
        </h1>
        <nav>
            <a href="/">Projects</a>
            <a href="/web/index-status" class="active">Index Status</a>
            <a href="/web/mcp">MCP Setup</a>
            <a href="/web/settings">Settings</a>
            <a href="/web/docs">API Docs</a>
        </nav>
    </header>
    <main class="container">
        <div class="card">
            <div class="card-header">
                <h2 class="card-title">Index Status</h2>
                <button class="btn btn-secondary refresh-btn" onclick="location.reload()">
                    Refresh
                </button>
            </div>

            <div class="status-card">
                <h3>API Configuration</h3>
                <div>
                    <strong>GEMINI_API_KEY:</strong>
                    <span class="status-indicator %s">%s</span>
                </div>
            </div>

            <div class="status-card">
                <h3>Project Index Status</h3>`, apiKeyClass, apiKeyStatus)))

	if len(projects) == 0 {
		w.Write([]byte(`
                <div class="empty-state">
                    <p>No projects registered. Add a project to see index status.</p>
                </div>`))
	} else {
		w.Write([]byte(`
                <table class="project-status-table">
                    <thead>
                        <tr>
                            <th>Project</th>
                            <th>Path</th>
                            <th>Status</th>
                            <th>Documents</th>
                            <th>Files</th>
                            <th>Last Updated</th>
                        </tr>
                    </thead>
                    <tbody>`))

		for _, p := range projects {
			idx := s.manager.GetIndexer(p.ID)
			status := "not_indexed"
			statusLabel := "Not Indexed"
			docCount := 0
			fileCount := 0
			lastUpdated := "-"
			errorMsg := ""

			if idx != nil {
				stats := idx.Stats()
				docCount = stats.DocumentCount
				fileCount = stats.FileCount
				if !stats.LastUpdated.IsZero() {
					lastUpdated = stats.LastUpdated.Format("Jan 2, 2006 3:04 PM")
				}

				if !apiKeyConfigured {
					status = "api_key_missing"
					statusLabel = "API Key Missing"
					errorMsg = "GEMINI_API_KEY not provided"
				} else if docCount == 0 {
					status = "empty"
					statusLabel = "Empty"
				} else {
					status = "indexed"
					statusLabel = "Indexed"
				}
			} else {
				errorMsg = "Indexer not initialized"
			}

			row := fmt.Sprintf(`
                        <tr>
                            <td><a href="/web/project/%s">%s</a></td>
                            <td><code style="font-size: 0.8rem;">%s</code></td>
                            <td>
                                <span class="status-badge %s">%s</span>
                                %s
                            </td>
                            <td>%d</td>
                            <td>%d</td>
                            <td>%s</td>
                        </tr>`,
				p.ID, p.Name, p.Path, status, statusLabel,
				func() string {
					if errorMsg != "" {
						return fmt.Sprintf(`<br><small style="color: var(--error-color);">%s</small>`, errorMsg)
					}
					return ""
				}(),
				docCount, fileCount, lastUpdated)
			w.Write([]byte(row))
		}

		w.Write([]byte(`
                    </tbody>
                </table>`))
	}

	w.Write([]byte(`
            </div>
        </div>
    </main>
</body>
</html>`))
}

func (s *Server) renderMCP(w http.ResponseWriter, r *http.Request) {
	// Determine the service URL based on configuration and environment
	host := s.cfg.Service.Host
	port := s.cfg.Service.Port

	// Check if running in Docker (common indicators)
	inDocker := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		inDocker = true
	}

	// If host is 0.0.0.0 or running in Docker, try to get external IP
	externalHost := host
	if host == "0.0.0.0" || host == "" || inDocker {
		if ip := getOutboundIP(); ip != "" {
			externalHost = ip
		} else {
			externalHost = "localhost"
		}
	}

	serviceURL := fmt.Sprintf("http://%s:%d", externalHost, port)

	// Generate MCP config JSON
	mcpConfig := fmt.Sprintf(`{
  "mcpServers": {
    "iter": {
      "url": "%s/mcp"
    }
  }
}`, serviceURL)

	// Claude Desktop config
	claudeConfig := fmt.Sprintf(`{
  "mcpServers": {
    "iter": {
      "command": "curl",
      "args": ["-N", "%s/mcp/sse"]
    }
  }
}`, serviceURL)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MCP Setup - iter-service</title>
    <link rel="stylesheet" href="/web/static/styles.css">
    <style>
        .code-block {
            position: relative;
            background-color: var(--bg-color);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 1rem;
            margin: 1rem 0;
            font-family: 'JetBrains Mono', 'Fira Code', monospace;
            font-size: 0.875rem;
            overflow-x: auto;
        }
        .code-block pre {
            margin: 0;
            white-space: pre-wrap;
            word-break: break-all;
        }
        .copy-btn {
            position: absolute;
            top: 0.5rem;
            right: 0.5rem;
            background-color: var(--surface-color);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            padding: 0.25rem 0.5rem;
            color: var(--text-muted);
            cursor: pointer;
            font-size: 0.75rem;
            transition: all 0.2s;
        }
        .copy-btn:hover {
            color: var(--text-color);
            border-color: var(--accent-color);
        }
        .copy-btn.copied {
            color: var(--success-color);
            border-color: var(--success-color);
        }
        .config-section {
            margin-bottom: 2rem;
        }
        .config-section h3 {
            margin-bottom: 0.5rem;
            color: var(--accent-color);
        }
        .config-section p {
            color: var(--text-muted);
            margin-bottom: 0.5rem;
        }
        .info-box {
            background-color: rgba(122, 162, 247, 0.1);
            border: 1px solid var(--accent-color);
            border-radius: 6px;
            padding: 1rem;
            margin-bottom: 1.5rem;
        }
        .info-box h4 {
            color: var(--accent-color);
            margin-bottom: 0.5rem;
        }
        .service-info {
            display: grid;
            grid-template-columns: auto 1fr;
            gap: 0.5rem 1rem;
            font-size: 0.875rem;
        }
        .service-info dt {
            color: var(--text-muted);
        }
        .service-info dd {
            color: var(--text-color);
            font-family: 'JetBrains Mono', monospace;
        }
    </style>
</head>
<body>
    <header class="header">
        <h1>
            <a href="/" style="color: inherit;">
                <svg class="logo" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
                </svg>
                iter-service
            </a>
        </h1>
        <nav>
            <a href="/">Projects</a>
            <a href="/web/mcp" class="active">MCP Setup</a>
            <a href="/web/settings">Settings</a>
            <a href="/web/docs">API Docs</a>
        </nav>
    </header>
    <main class="container">
        <div class="card">
            <h2 class="card-title">MCP Configuration</h2>

            <div class="info-box">
                <h4>Service Information</h4>
                <dl class="service-info">
                    <dt>URL:</dt>
                    <dd>%s</dd>
                    <dt>Host:</dt>
                    <dd>%s</dd>
                    <dt>Port:</dt>
                    <dd>%d</dd>
                    <dt>Environment:</dt>
                    <dd>%s</dd>
                </dl>
            </div>

            <div class="config-section">
                <h3>Claude Code / Cline / Continue</h3>
                <p>Add this to your MCP client configuration:</p>
                <div class="code-block">
                    <button class="copy-btn" onclick="copyToClipboard(this, 'mcp-config')">Copy</button>
                    <pre id="mcp-config">%s</pre>
                </div>
            </div>

            <div class="config-section">
                <h3>Claude Desktop (SSE Mode)</h3>
                <p>For Claude Desktop, use this SSE-based configuration:</p>
                <div class="code-block">
                    <button class="copy-btn" onclick="copyToClipboard(this, 'claude-config')">Copy</button>
                    <pre id="claude-config">%s</pre>
                </div>
            </div>

            <div class="config-section">
                <h3>Available MCP Tools</h3>
                <table style="width: 100%%; border-collapse: collapse; margin-top: 1rem;">
                    <thead>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <th style="text-align: left; padding: 0.75rem;">Tool</th>
                            <th style="text-align: left; padding: 0.75rem;">Description</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <td style="padding: 0.75rem;"><code>search</code></td>
                            <td style="padding: 0.75rem;">Semantic code search across indexed projects</td>
                        </tr>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <td style="padding: 0.75rem;"><code>get_symbol</code></td>
                            <td style="padding: 0.75rem;">Get detailed information about a symbol</td>
                        </tr>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <td style="padding: 0.75rem;"><code>get_dependencies</code></td>
                            <td style="padding: 0.75rem;">Find what a symbol depends on</td>
                        </tr>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <td style="padding: 0.75rem;"><code>get_dependents</code></td>
                            <td style="padding: 0.75rem;">Find what depends on a symbol</td>
                        </tr>
                        <tr style="border-bottom: 1px solid var(--border-color);">
                            <td style="padding: 0.75rem;"><code>get_file_impact</code></td>
                            <td style="padding: 0.75rem;">Analyze impact of changes to a file</td>
                        </tr>
                        <tr>
                            <td style="padding: 0.75rem;"><code>list_projects</code></td>
                            <td style="padding: 0.75rem;">List all indexed projects</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </main>

    <script>
        function copyToClipboard(btn, elementId) {
            const text = document.getElementById(elementId).textContent;
            navigator.clipboard.writeText(text).then(() => {
                btn.textContent = 'Copied!';
                btn.classList.add('copied');
                setTimeout(() => {
                    btn.textContent = 'Copy';
                    btn.classList.remove('copied');
                }, 2000);
            }).catch(err => {
                console.error('Failed to copy:', err);
                btn.textContent = 'Error';
                setTimeout(() => {
                    btn.textContent = 'Copy';
                }, 2000);
            });
        }
    </script>
</body>
</html>`,
		serviceURL,
		externalHost,
		port,
		getEnvironmentLabel(inDocker),
		template.HTMLEscapeString(mcpConfig),
		template.HTMLEscapeString(claudeConfig),
	)))
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// getEnvironmentLabel returns a human-readable environment label
func getEnvironmentLabel(inDocker bool) string {
	if inDocker {
		return "Docker Container"
	}
	return "Local"
}

// handleProjectsList returns the project list as an HTML partial for HTMX.
func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(web.Templates, "templates/project-list.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	projects := s.registry.List()
	data := WebProjectListData{
		Projects: make([]WebProjectData, 0, len(projects)),
	}

	for _, p := range projects {
		pd := WebProjectData{
			ID:   p.ID,
			Name: p.Name,
			Path: p.Path,
		}

		// Get index stats if indexer is available
		if idx := s.manager.GetIndexer(p.ID); idx != nil {
			stats := idx.Stats()
			pd.IndexStats = &WebIndexStatsData{
				DocumentCount: stats.DocumentCount,
				FileCount:     stats.FileCount,
				CurrentBranch: stats.CurrentBranch,
				LastUpdated:   stats.LastUpdated.Format("Jan 2, 2006 3:04 PM"),
			}
		}

		data.Projects = append(data.Projects, pd)
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleWebSearch handles search from the web UI and returns HTML partial.
func (s *Server) handleWebSearch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Project not found or indexer not available</p></div>`))
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Invalid form data</p></div>`))
		return
	}

	query := r.FormValue("query")
	kind := r.FormValue("kind")

	if query == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Enter a search query to find code.</p></div>`))
		return
	}

	opts := index.SearchOptions{
		Query:      query,
		Limit:      20,
		SymbolKind: kind,
	}

	searcher := index.NewSearcher(idx)
	results, err := searcher.Search(context.Background(), opts)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Search failed: ` + err.Error() + `</p></div>`))
		return
	}

	tmpl, err := template.ParseFS(web.Templates, "templates/search-results.html")
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := WebSearchResultsData{
		Query:   query,
		Total:   len(results),
		Results: make([]WebSearchResultItem, 0, len(results)),
	}

	for _, r := range results {
		data.Results = append(data.Results, WebSearchResultItem{
			SymbolName: r.Chunk.SymbolName,
			SymbolKind: r.Chunk.SymbolKind,
			FilePath:   r.Chunk.FilePath,
			StartLine:  r.Chunk.StartLine,
			EndLine:    r.Chunk.EndLine,
			Signature:  r.Chunk.Signature,
		})
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Template execution error: "+err.Error(), http.StatusInternalServerError)
	}
}

// handleWebRegisterProject handles project registration from the web form.
func (s *Server) handleWebRegisterProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Invalid form data</p></div>`))
		return
	}

	path := r.FormValue("path")
	if path == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Path is required</p></div>`))
		return
	}

	_, err := s.manager.RegisterProject(path)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Error: ` + err.Error() + `</p></div>`))
		return
	}

	// Return updated project list
	s.handleProjectsList(w, r)
}

// handleWebRebuildIndex handles index rebuild from the web UI.
func (s *Server) handleWebRebuildIndex(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	idx := s.manager.GetIndexer(id)
	if idx == nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<span class="status"><span class="status-dot warning"></span>Indexer not available</span>`))
		return
	}

	if err := idx.IndexAll(); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<span class="status"><span class="status-dot error"></span>Error: ` + err.Error() + `</span>`))
		return
	}

	stats := idx.Stats()

	// Return updated stats HTML
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="project-stat">
    <strong>` + strconv.Itoa(stats.DocumentCount) + `</strong> symbols
</div>
<div class="project-stat">
    <strong>` + strconv.Itoa(stats.FileCount) + `</strong> files
</div>
<div class="project-stat">
    <span class="status">
        <span class="status-dot success"></span>
        ` + stats.CurrentBranch + `
    </span>
</div>
<div class="project-stat">
    Last updated: ` + stats.LastUpdated.Format("Jan 2, 2006 3:04 PM") + `
</div>`))
}

// handleWebUnregisterProject handles project removal from the web UI.
func (s *Server) handleWebUnregisterProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.manager.UnregisterProject(id); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="empty-state"><p>Error: ` + err.Error() + `</p></div>`))
		return
	}

	// Return empty response (element will be removed by hx-swap="outerHTML")
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
}

// handleWebProjectItem returns a single project item HTML partial.
func (s *Server) handleWebProjectItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	project, err := s.registry.Get(id)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		return
	}

	pd := WebProjectData{
		ID:   project.ID,
		Name: project.Name,
		Path: project.Path,
	}

	// Get index stats if indexer is available
	if idx := s.manager.GetIndexer(id); idx != nil {
		stats := idx.Stats()
		pd.IndexStats = &WebIndexStatsData{
			DocumentCount: stats.DocumentCount,
			FileCount:     stats.FileCount,
			CurrentBranch: stats.CurrentBranch,
			LastUpdated:   stats.LastUpdated.Format("Jan 2, 2006 3:04 PM"),
		}
	}

	// Return HTML for a single project item
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div class="project-item" id="project-` + pd.ID + `">
    <div class="project-info">
        <h3>` + pd.Name + `</h3>
        <div class="project-path">` + pd.Path + `</div>
    </div>
    <div class="project-stats">`))

	if pd.IndexStats != nil {
		w.Write([]byte(`
        <div class="project-stat">
            <span>` + strconv.Itoa(pd.IndexStats.DocumentCount) + ` symbols</span>
        </div>
        <div class="project-stat">
            <span>` + strconv.Itoa(pd.IndexStats.FileCount) + ` files</span>
        </div>
        <div class="project-stat">
            <span class="status">
                <span class="status-dot success"></span>
                ` + pd.IndexStats.CurrentBranch + `
            </span>
        </div>`))
	} else {
		w.Write([]byte(`
        <div class="project-stat">
            <span class="status">
                <span class="status-dot warning"></span>
                Not indexed
            </span>
        </div>`))
	}

	w.Write([]byte(`
    </div>
    <div class="project-actions">
        <a href="/web/project/` + pd.ID + `" class="btn btn-secondary btn-sm">View</a>
        <button class="btn btn-secondary btn-sm"
                hx-post="/projects/` + pd.ID + `/index"
                hx-target="#project-` + pd.ID + `"
                hx-swap="outerHTML">
            <span class="htmx-indicator spinner"></span>
            Reindex
        </button>
        <button class="btn btn-danger btn-sm"
                hx-delete="/projects/` + pd.ID + `"
                hx-target="#project-` + pd.ID + `"
                hx-swap="outerHTML"
                hx-confirm="Are you sure you want to remove this project?">
            Remove
        </button>
    </div>
</div>`))
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

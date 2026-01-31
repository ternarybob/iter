// Package api provides the REST API for iter-service.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/internal/project"
)

// Server represents the API server.
type Server struct {
	cfg      *config.Config
	router   chi.Router
	registry *project.Registry
	manager  *project.Manager
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, registry *project.Registry, manager *project.Manager) *Server {
	s := &Server{
		cfg:      cfg,
		registry: registry,
		manager:  manager,
	}

	s.setupRouter()
	return s
}

// setupRouter configures all routes.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * 1000000000)) // 60 seconds

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Optional API key authentication
	if s.cfg.API.APIKey != "" {
		r.Use(s.apiKeyAuth)
	}

	// Health and version endpoints (no auth)
	r.Get("/health", s.handleHealth)
	r.Get("/version", s.handleVersion)

	// API routes
	r.Route("/projects", func(r chi.Router) {
		r.Get("/", s.handleListProjects)
		r.Post("/", s.handleRegisterProject)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleGetProject)
			r.Delete("/", s.handleUnregisterProject)
			r.Post("/index", s.handleRebuildIndex)
			r.Post("/search", s.handleSearch)
			r.Get("/deps/{symbol}", s.handleGetDeps)
			r.Get("/dependents/{symbol}", s.handleGetDependents)
			r.Get("/impact/{file}", s.handleGetImpact)
			r.Get("/history", s.handleGetHistory)
		})
	})

	// API route for HTMX project list partial
	r.Get("/api/projects-list", s.handleProjectsList)

	// Web UI routes (served from /web)
	r.Get("/", s.handleWebRoot)
	r.Get("/web/*", s.handleWebAssets)

	s.router = r
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

// apiKeyAuth is middleware that validates API key.
func (s *Server) apiKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health and version
		if r.URL.Path == "/health" || r.URL.Path == "/version" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for localhost without API key configured
		if s.cfg.API.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check API key header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey != s.cfg.API.APIKey {
			writeError(w, http.StatusUnauthorized, "Invalid or missing API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

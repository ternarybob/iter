// Package project provides project lifecycle management for iter-service.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ternarybob/iter/internal/config"
)

// Project represents a registered project.
type Project struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	RegisteredAt time.Time `json:"registered_at"`
}

// Registry manages the collection of registered projects.
type Registry struct {
	mu       sync.RWMutex
	projects map[string]*Project
	path     string
}

// NewRegistry creates a new project registry.
func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		projects: make(map[string]*Project),
		path:     cfg.RegistryPath(),
	}
}

// Load loads the registry from disk.
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No registry file yet
		}
		return fmt.Errorf("read registry: %w", err)
	}

	var projects []*Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return fmt.Errorf("parse registry: %w", err)
	}

	for _, p := range projects {
		r.projects[p.ID] = p
	}

	return nil
}

// Save persists the registry to disk.
func (r *Registry) Save() error {
	r.mu.RLock()
	projects := make([]*Project, 0, len(r.projects))
	for _, p := range r.projects {
		projects = append(projects, p)
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return fmt.Errorf("create registry directory: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	return nil
}

// Add adds a project to the registry.
func (r *Registry) Add(project *Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	for _, p := range r.projects {
		if p.Path == project.Path {
			return fmt.Errorf("project already registered with ID: %s", p.ID)
		}
	}

	r.projects[project.ID] = project
	return nil
}

// Remove removes a project from the registry.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.projects[id]; !ok {
		return fmt.Errorf("project not found: %s", id)
	}

	delete(r.projects, id)
	return nil
}

// Get returns a project by ID.
func (r *Registry) Get(id string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	project, ok := r.projects[id]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", id)
	}

	return project, nil
}

// GetByPath returns a project by its path.
func (r *Registry) GetByPath(path string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for _, p := range r.projects {
		pAbs, _ := filepath.Abs(p.Path)
		if pAbs == absPath {
			return p, nil
		}
	}

	return nil, fmt.Errorf("project not found for path: %s", path)
}

// List returns all registered projects.
func (r *Registry) List() []*Project {
	r.mu.RLock()
	defer r.mu.RUnlock()

	projects := make([]*Project, 0, len(r.projects))
	for _, p := range r.projects {
		projects = append(projects, p)
	}

	return projects
}

// Count returns the number of registered projects.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.projects)
}

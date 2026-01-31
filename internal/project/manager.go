package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ternarybob/iter/internal/config"
	"github.com/ternarybob/iter/pkg/index"
)

// Manager handles project lifecycle including indexing and watching.
type Manager struct {
	cfg      *config.Config
	registry *Registry
	indexers map[string]*index.Indexer
	watchers map[string]*index.Watcher
	mu       sync.RWMutex
}

// NewManager creates a new project manager.
func NewManager(cfg *config.Config, registry *Registry) *Manager {
	return &Manager{
		cfg:      cfg,
		registry: registry,
		indexers: make(map[string]*index.Indexer),
		watchers: make(map[string]*index.Watcher),
	}
}

// Initialize loads all registered projects and starts their indexers.
func (m *Manager) Initialize() error {
	projects := m.registry.List()

	for _, p := range projects {
		if err := m.initializeProject(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to initialize project %s: %v\n", p.ID, err)
		}
	}

	return nil
}

// initializeProject initializes indexing for a single project.
func (m *Manager) initializeProject(p *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if path still exists
	if _, err := os.Stat(p.Path); os.IsNotExist(err) {
		return fmt.Errorf("project path does not exist: %s", p.Path)
	}

	// Create index config
	indexCfg := index.Config{
		ProjectID:    p.ID,
		ProjectPath:  p.Path,
		RepoRoot:     p.Path,
		IndexPath:    m.cfg.ProjectIndexDir(p.Path),
		ExcludeGlobs: []string{"vendor/**", "*_test.go", ".git/**", "node_modules/**"},
		DebounceMs:   500,
	}

	// Ensure index directory exists
	if err := os.MkdirAll(indexCfg.IndexPath, 0755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	// Create indexer
	idx, err := index.NewIndexer(indexCfg)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}

	m.indexers[p.ID] = idx

	// Auto-build if index is empty
	if idx.Stats().DocumentCount == 0 {
		if err := idx.IndexAll(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to build index for %s: %v\n", p.ID, err)
		}
	}

	// Start watcher
	watcher, err := index.NewWatcher(idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create watcher for %s: %v\n", p.ID, err)
		return nil
	}

	if err := watcher.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to start watcher for %s: %v\n", p.ID, err)
		return nil
	}

	m.watchers[p.ID] = watcher
	return nil
}

// RegisterProject registers a new project and initializes its index.
func (m *Manager) RegisterProject(path string) (*Project, error) {
	// Validate path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	// Check if already registered
	if existing, _ := m.registry.GetByPath(absPath); existing != nil {
		return nil, fmt.Errorf("project already registered")
	}

	// Create project
	project := &Project{
		ID:           config.ProjectHash(absPath),
		Path:         absPath,
		Name:         filepath.Base(absPath),
		RegisteredAt: time.Now(),
	}

	// Add to registry
	if err := m.registry.Add(project); err != nil {
		return nil, err
	}

	// Save registry
	if err := m.registry.Save(); err != nil {
		m.registry.Remove(project.ID)
		return nil, fmt.Errorf("save registry: %w", err)
	}

	// Initialize project
	if err := m.initializeProject(project); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to initialize new project: %v\n", err)
	}

	return project, nil
}

// UnregisterProject unregisters a project and cleans up its resources.
func (m *Manager) UnregisterProject(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop watcher
	if watcher, ok := m.watchers[id]; ok {
		watcher.Stop()
		delete(m.watchers, id)
	}

	// Remove indexer
	delete(m.indexers, id)

	// Remove from registry
	if err := m.registry.Remove(id); err != nil {
		return err
	}

	// Save registry
	if err := m.registry.Save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	// Optionally clean up data directory
	// Note: We don't delete by default to preserve index data
	// The user can manually clean up the data directory

	return nil
}

// GetIndexer returns the indexer for a project.
func (m *Manager) GetIndexer(id string) *index.Indexer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.indexers[id]
}

// GetWatcher returns the watcher for a project.
func (m *Manager) GetWatcher(id string) *index.Watcher {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.watchers[id]
}

// Shutdown stops all watchers and cleans up resources.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, watcher := range m.watchers {
		watcher.Stop()
		delete(m.watchers, id)
	}
}

// RebuildIndex rebuilds the index for a project.
func (m *Manager) RebuildIndex(id string) error {
	idx := m.GetIndexer(id)
	if idx == nil {
		return fmt.Errorf("project not found: %s", id)
	}

	return idx.IndexAll()
}

// Stats returns statistics for a project.
func (m *Manager) Stats(id string) (*index.IndexStats, error) {
	idx := m.GetIndexer(id)
	if idx == nil {
		return nil, fmt.Errorf("project not found: %s", id)
	}

	stats := idx.Stats()
	return &stats, nil
}

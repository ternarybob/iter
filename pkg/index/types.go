// Package index provides real-time local code indexing for iter-service.
package index

import (
	"time"
)

// Chunk represents an indexed code unit (function/method/type).
type Chunk struct {
	ID         string    `json:"id"`          // Unique identifier (file:line)
	FilePath   string    `json:"file_path"`   // Relative path
	SymbolName string    `json:"symbol_name"` // Function/method/type name
	SymbolKind string    `json:"symbol_kind"` // "function", "method", "type", "const"
	Content    string    `json:"content"`     // Actual source code
	Signature  string    `json:"signature"`   // Function signature for quick matching
	DocComment string    `json:"doc_comment"` // Godoc if present
	StartLine  int       `json:"start_line"`  // Start line number
	EndLine    int       `json:"end_line"`    // End line number
	Hash       string    `json:"hash"`        // SHA-256 of Content
	Branch     string    `json:"branch"`      // Git branch at index time
	IndexedAt  time.Time `json:"indexed_at"`  // Timestamp
}

// ToMetadata converts Chunk fields to map[string]string for chromem storage.
func (c *Chunk) ToMetadata() map[string]string {
	return map[string]string{
		"file_path":   c.FilePath,
		"symbol_name": c.SymbolName,
		"symbol_kind": c.SymbolKind,
		"signature":   c.Signature,
		"start_line":  itoa(c.StartLine),
		"end_line":    itoa(c.EndLine),
		"hash":        c.Hash,
		"git_branch":  c.Branch,
	}
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Query      string // Search query
	Branch     string // Filter by git branch (empty = all)
	SymbolKind string // Filter by kind (empty = all)
	FilePath   string // Filter by path prefix (empty = all)
	Limit      int    // Max results (default 10)
}

// SearchResult represents a single search match.
type SearchResult struct {
	Chunk      Chunk   // The matched chunk
	Score      float32 // Similarity score (0-1)
	Rank       int     // Position in results
	MatchCount int     // Number of keyword matches (for pre-filter)
}

// IndexStats provides statistics about the index.
type IndexStats struct {
	DocumentCount  int       // Total documents in index
	FileCount      int       // Number of unique files indexed
	CurrentBranch  string    // Current git branch
	LastUpdated    time.Time // Last index update time
	WatcherRunning bool      // Whether file watcher is active
}

// Config configures the Indexer.
type Config struct {
	ProjectID    string   // Unique project identifier (SHA256 hash of path)
	ProjectPath  string   // Absolute path to project root
	RepoRoot     string   // Repository root path (same as ProjectPath for now)
	IndexPath    string   // Path to index storage (in service data dir)
	ExcludeGlobs []string // Default vendor/**, *_test.go, .git/**
	DebounceMs   int      // Default 500
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(repoRoot string) Config {
	return Config{
		ProjectPath: repoRoot,
		RepoRoot:    repoRoot,
		IndexPath:   ".iter/index",
		ExcludeGlobs: []string{
			"vendor/**",
			"*_test.go",
			".git/**",
			"node_modules/**",
		},
		DebounceMs: 500,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

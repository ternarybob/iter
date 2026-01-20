// Package index provides codebase indexing and search capabilities.
// It uses SQLite FTS5 for full-text search.
package index

import (
	"context"
	"fmt"
)

// Index provides searchable access to a codebase.
type Index interface {
	// Indexing
	IndexFile(ctx context.Context, path string, content []byte) error
	IndexDirectory(ctx context.Context, root string, opts IndexOptions) error
	RemoveFile(ctx context.Context, path string) error

	// Search
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	FindSymbol(ctx context.Context, name string, kind SymbolKind) ([]Symbol, error)
	FindReferences(ctx context.Context, symbol Symbol) ([]Reference, error)

	// Context retrieval
	GetContext(ctx context.Context, query string, maxTokens int) ([]Chunk, error)
	GetFile(ctx context.Context, path string) (*File, error)
	GetChunk(ctx context.Context, id string) (*Chunk, error)

	// Management
	Stats(ctx context.Context) (*IndexStats, error)
	Clear(ctx context.Context) error
	Close() error
}

// IndexOptions configures indexing behavior.
type IndexOptions struct {
	// IncludePatterns are glob patterns for files to include.
	IncludePatterns []string

	// ExcludePatterns are glob patterns for files to exclude.
	ExcludePatterns []string

	// MaxFileSize limits individual file size.
	MaxFileSize int64

	// ChunkSize is the target chunk size in lines.
	ChunkSize int

	// ChunkOverlap is lines of overlap between chunks.
	ChunkOverlap int

	// ParseSymbols enables symbol extraction.
	ParseSymbols bool
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	// MaxResults limits the number of results.
	MaxResults int

	// FilePatterns filters by file glob patterns.
	FilePatterns []string

	// SymbolKinds filters by symbol kind.
	SymbolKinds []SymbolKind

	// IncludeContent includes content in results.
	IncludeContent bool

	// CaseSensitive enables case-sensitive matching.
	CaseSensitive bool

	// ContextLines is the number of surrounding lines.
	ContextLines int
}

// SearchResult represents a search match.
type SearchResult struct {
	// Path is the file path.
	Path string

	// Line is the line number (1-indexed).
	Line int

	// Column is the column number (1-indexed).
	Column int

	// Content is the matching line content.
	Content string

	// Score is the relevance score.
	Score float64

	// ContextBefore is lines before the match.
	ContextBefore []string

	// ContextAfter is lines after the match.
	ContextAfter []string

	// Symbol is the matched symbol (if applicable).
	Symbol *Symbol
}

// SymbolKind categorizes code symbols.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolClass     SymbolKind = "class"
	SymbolInterface SymbolKind = "interface"
	SymbolStruct    SymbolKind = "struct"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolType      SymbolKind = "type"
	SymbolPackage   SymbolKind = "package"
	SymbolModule    SymbolKind = "module"
	SymbolField     SymbolKind = "field"
	SymbolProperty  SymbolKind = "property"
	SymbolEnum      SymbolKind = "enum"
	SymbolEnumMember SymbolKind = "enum_member"
)

// Symbol represents a code symbol.
type Symbol struct {
	// Name is the symbol name.
	Name string

	// Kind is the symbol type.
	Kind SymbolKind

	// Path is the file path.
	Path string

	// Line is the line number.
	Line int

	// Column is the column number.
	Column int

	// EndLine is the ending line (for multi-line symbols).
	EndLine int

	// Signature is the full signature.
	Signature string

	// Documentation is the doc comment.
	Documentation string

	// Parent is the containing symbol name.
	Parent string

	// Children are nested symbols.
	Children []Symbol
}

// Reference represents a symbol reference.
type Reference struct {
	// Path is the file path.
	Path string

	// Line is the line number.
	Line int

	// Column is the column number.
	Column int

	// Content is the line content.
	Content string

	// IsDefinition indicates this is the definition.
	IsDefinition bool
}

// Chunk represents a code fragment.
type Chunk struct {
	// ID is a unique identifier.
	ID string

	// Path is the file path.
	Path string

	// StartLine is the starting line number.
	StartLine int

	// EndLine is the ending line number.
	EndLine int

	// Content is the code content.
	Content string

	// Language is the programming language.
	Language string

	// Symbols are symbols in this chunk.
	Symbols []Symbol

	// Hash is a content hash.
	Hash string
}

// File represents an indexed file.
type File struct {
	// Path is the file path.
	Path string

	// Content is the file content.
	Content string

	// Language is the programming language.
	Language string

	// Size is the file size in bytes.
	Size int64

	// ModTime is the modification time (unix timestamp).
	ModTime int64

	// Chunks are the code chunks.
	Chunks []Chunk

	// Symbols are the top-level symbols.
	Symbols []Symbol

	// Hash is a content hash.
	Hash string
}

// IndexStats contains indexing statistics.
type IndexStats struct {
	// FileCount is the number of indexed files.
	FileCount int

	// ChunkCount is the number of chunks.
	ChunkCount int

	// SymbolCount is the number of symbols.
	SymbolCount int

	// TotalSize is the total content size.
	TotalSize int64

	// Languages is a map of language to file count.
	Languages map[string]int

	// LastUpdated is the last update timestamp.
	LastUpdated int64
}

// DefaultIndexOptions returns sensible defaults.
func DefaultIndexOptions() IndexOptions {
	return IndexOptions{
		IncludePatterns: []string{"*"},
		ExcludePatterns: []string{
			"vendor/*",
			"node_modules/*",
			".git/*",
			"*.min.js",
			"*.min.css",
		},
		MaxFileSize:  1 << 20, // 1MB
		ChunkSize:    50,
		ChunkOverlap: 10,
		ParseSymbols: true,
	}
}

// DefaultSearchOptions returns sensible defaults.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		MaxResults:     100,
		IncludeContent: true,
		ContextLines:   3,
	}
}

// LanguageFromPath detects language from file extension.
func LanguageFromPath(path string) string {
	ext := ""
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}

	switch ext {
	case ".go":
		return "go"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".mts", ".cts":
		return "typescript"
	case ".jsx":
		return "javascriptreact"
	case ".tsx":
		return "typescriptreact"
	case ".py", ".pyi":
		return "python"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".hh", ".cxx":
		return "cpp"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".sh", ".bash":
		return "shell"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "css"
	case ".xml":
		return "xml"
	case ".tf":
		return "terraform"
	case ".swift":
		return "swift"
	case ".scala":
		return "scala"
	default:
		return ""
	}
}

// Error types
var (
	ErrNotFound     = fmt.Errorf("not found")
	ErrInvalidQuery = fmt.Errorf("invalid query")
	ErrClosed       = fmt.Errorf("index closed")
)

# Code Indexing

Real-time local code indexing for the iter Claude plugin using chromem-go, go/ast, and fsnotify.

## Overview

The code indexing system enables semantic and keyword search across Go codebases. It extracts symbols (functions, methods, types, constants) from source files and stores them in a persistent vector database for fast retrieval.

## Quick Start

```bash
# Build the index for current repository
iter index build

# Search for code
iter search "handler"

# Search with filters
iter search "Config" --kind=type
iter search "Parse" --path=index/
iter search "State" --limit=5
```

## Commands

### iter index

Manages the code index.

| Command | Description |
|---------|-------------|
| `iter index` | Show index status (document count, branch, last updated) |
| `iter index build` | Build/rebuild the full code index |
| `iter index clear` | Clear and rebuild the index |
| `iter index watch` | Start file watcher for real-time indexing |

### iter search

Search indexed code.

```
iter search "<query>" [options]
```

**Options:**

| Option | Description | Example |
|--------|-------------|---------|
| `--kind=<type>` | Filter by symbol type | `--kind=function` |
| `--path=<prefix>` | Filter by file path prefix | `--path=handlers/` |
| `--branch=<branch>` | Filter by git branch | `--branch=main` |
| `--limit=<n>` | Maximum results (default 10) | `--limit=5` |

**Symbol kinds:** `function`, `method`, `type`, `const`

## Architecture

### Package Structure

```
iter/
├── index/
│   ├── types.go      # Data model (Chunk, SearchOptions, Config)
│   ├── parser.go     # go/ast symbol extraction
│   ├── index.go      # Core Indexer with chromem-go
│   ├── watcher.go    # fsnotify file monitoring
│   └── search.go     # Query interface and result formatting
└── cmd/iter/main.go  # CLI commands (index, search)
```

### Data Model

#### Chunk

Represents an indexed code unit.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique identifier (`file:line`) |
| FilePath | string | Relative path from repo root |
| SymbolName | string | Function/method/type name |
| SymbolKind | string | `function`, `method`, `type`, `const` |
| Content | string | Full source code |
| Signature | string | Function/type signature |
| DocComment | string | Godoc comment if present |
| StartLine | int | Start line number |
| EndLine | int | End line number |
| Hash | string | SHA-256 of content |
| Branch | string | Git branch at index time |
| IndexedAt | time.Time | Indexing timestamp |

### Components

#### Parser (`parser.go`)

Uses `go/ast` and `go/parser` to extract symbols from Go source files.

**Extracts:**
- Function declarations (name, signature, body, doc)
- Method declarations (receiver, name, signature, body, doc)
- Type declarations (structs, interfaces, aliases)
- Exported constants

**Example extraction:**

```go
// Input file: handlers/status.go
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) error {
    // ...
}

// Extracted Chunk:
// - SymbolName: "GetStatus"
// - SymbolKind: "method"
// - Signature: "func (*Handler) GetStatus(http.ResponseWriter, *http.Request) error"
// - FilePath: "handlers/status.go"
```

#### Indexer (`index.go`)

Core type managing chromem-go database and document storage.

**Key methods:**
- `NewIndexer(cfg Config)` - Initialize with persistent chromem DB
- `IndexFile(path string)` - Parse and index single file
- `IndexAll()` - Full repository index
- `Clear()` - Delete and recreate collection
- `Stats()` - Return index statistics

**Embedding strategy:**

Uses a simple hash-based embedding function for local operation without external API dependencies:

```go
func simpleEmbedding(ctx context.Context, text string) ([]float32, error) {
    // 1. Tokenize text into words
    // 2. Hash each word using FNV-1a
    // 3. Accumulate into 256-dimension vector
    // 4. Normalize the vector
}
```

This enables chromem-go to store and query documents locally. For higher quality semantic search, replace with Voyage AI or OpenAI embeddings.

#### Watcher (`watcher.go`)

Uses `fsnotify` for real-time file monitoring.

**Behavior:**
- Watches all directories recursively
- Skips: `vendor/`, `.git/`, `node_modules/`, `.iter/`
- Filters: Only `.go` files, only WRITE/CREATE events
- Debounces: Waits 500ms after last event before indexing

#### Searcher (`search.go`)

Provides query interface over indexed documents.

**Search strategy:**
1. Try semantic search using chromem-go's vector similarity
2. Apply metadata filters (kind, path, branch) post-query
3. Fall back to keyword search if semantic search returns no results

**Result formatting:**

```go
// FormatResults() outputs markdown suitable for prompt injection:

## Relevant Code from Index

### 1. function `GetStatus` (89% match)
**File**: `handlers/status.go` L45-67
**Signature**: `func (h *Handler) GetStatus(...) error`
```

## Storage

### Location

Index is stored at `<project>/.iter/index/` using chromem-go's persistent storage (gob compressed).

### Git Integration

- `.iter/` is gitignored (configured in `.gitignore:38`)
- Each indexed chunk is tagged with current git branch
- Search can filter by branch: `--branch=main`

### Recovery

If the index becomes corrupted or out of sync:

```bash
iter index clear   # Removes all indexed data
iter index build   # Full reindex from source
```

## Configuration

Default configuration in `index/types.go`:

```go
Config{
    RepoRoot:  ".",              // Repository root path
    IndexPath: ".iter/index",    // Storage location
    ExcludeGlobs: []string{
        "vendor/**",
        "*_test.go",
        ".git/**",
        "node_modules/**",
    },
    DebounceMs: 500,             // File watcher debounce
}
```

## Agent Integration

The indexing system is designed to provide context to iter's agent phases.

### Architect Phase

Query for related patterns before planning:

```bash
iter search "authentication handler"
```

Inject results into architect prompt to inform step planning.

### Worker Phase

Query for implementation context:

```bash
iter search "Config" --kind=type
iter search "NewClient" --kind=function
```

Provides exact signatures to match and utilities to reuse.

### Validator Phase

Query for duplicate/impact detection:

```bash
iter search "NewHandler" --kind=function
```

Check if similar functions already exist (potential duplicates).

## Dependencies

```
github.com/philippgille/chromem-go  # Vector database (no CGO)
github.com/fsnotify/fsnotify        # File system notifications
```

Both dependencies are pure Go with no CGO requirements.

## Limitations

1. **Go-only**: Currently only parses `.go` files
2. **Simple embeddings**: Hash-based embeddings are less accurate than ML-based ones
3. **No incremental updates**: File changes require full file reindex (chromem-go limitation)
4. **Single branch**: Index doesn't maintain multiple branch versions simultaneously

## Future Improvements

1. **Voyage AI embeddings**: Add `--voyage` flag for higher quality semantic search
2. **Multi-language support**: Add parsers for TypeScript, Python, Rust
3. **Incremental updates**: Track chunk IDs for surgical updates
4. **Cross-branch search**: Index multiple branches simultaneously

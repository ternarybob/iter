package index

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryIndex implements Index with in-memory storage.
// Use this when SQLite is not available or for testing.
type MemoryIndex struct {
	mu sync.RWMutex

	files   map[string]*File
	chunks  map[string]*Chunk
	symbols map[string][]Symbol

	chunker *Chunker
	parser  *Parser
	closed  bool
}

// NewMemoryIndex creates a new in-memory index.
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{
		files:   make(map[string]*File),
		chunks:  make(map[string]*Chunk),
		symbols: make(map[string][]Symbol),
		chunker: NewChunker(50, 10),
		parser:  NewParser(),
	}
}

// IndexFile indexes a single file.
func (idx *MemoryIndex) IndexFile(ctx context.Context, path string, content []byte) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrClosed
	}

	language := LanguageFromPath(path)
	contentStr := string(content)

	// Parse symbols
	symbols := idx.parser.Parse(path, contentStr, language)

	// Create chunks
	chunks := idx.chunker.ChunkWithSymbols(path, contentStr, language, symbols)

	// Store file
	file := &File{
		Path:     path,
		Content:  contentStr,
		Language: language,
		Size:     int64(len(content)),
		ModTime:  time.Now().Unix(),
		Symbols:  symbols,
		Hash:     hashContent(contentStr),
	}

	for _, chunk := range chunks {
		file.Chunks = append(file.Chunks, chunk)
		idx.chunks[chunk.ID] = &chunk
	}

	idx.files[path] = file

	// Index symbols by name
	for _, sym := range symbols {
		nameLower := strings.ToLower(sym.Name)
		idx.symbols[nameLower] = append(idx.symbols[nameLower], sym)
	}

	return nil
}

// IndexDirectory indexes all files in a directory.
func (idx *MemoryIndex) IndexDirectory(ctx context.Context, root string, opts IndexOptions) error {
	walker := NewWalker(opts)
	return walker.Walk(ctx, root, func(path string, content []byte) error {
		return idx.IndexFile(ctx, path, content)
	})
}

// RemoveFile removes a file from the index.
func (idx *MemoryIndex) RemoveFile(ctx context.Context, path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrClosed
	}

	file, ok := idx.files[path]
	if !ok {
		return nil
	}

	// Remove chunks
	for _, chunk := range file.Chunks {
		delete(idx.chunks, chunk.ID)
	}

	// Remove symbols
	for _, sym := range file.Symbols {
		nameLower := strings.ToLower(sym.Name)
		symbols := idx.symbols[nameLower]
		filtered := make([]Symbol, 0, len(symbols))
		for _, s := range symbols {
			if s.Path != path {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			delete(idx.symbols, nameLower)
		} else {
			idx.symbols[nameLower] = filtered
		}
	}

	delete(idx.files, path)
	return nil
}

// Search performs full-text search.
func (idx *MemoryIndex) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	if opts.MaxResults == 0 {
		opts.MaxResults = 100
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for _, file := range idx.files {
		contentLower := strings.ToLower(file.Content)
		lines := strings.Split(file.Content, "\n")

		for lineNum, line := range lines {
			if strings.Contains(strings.ToLower(line), queryLower) {
				result := SearchResult{
					Path:    file.Path,
					Line:    lineNum + 1,
					Content: line,
					Score:   float64(strings.Count(strings.ToLower(line), queryLower)),
				}

				if opts.ContextLines > 0 {
					result.ContextBefore, result.ContextAfter = getContextLines(lines, lineNum, opts.ContextLines)
				}

				results = append(results, result)

				if len(results) >= opts.MaxResults {
					return results, nil
				}
			}
		}
		_ = contentLower
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

// FindSymbol finds symbols by name and kind.
func (idx *MemoryIndex) FindSymbol(ctx context.Context, name string, kind SymbolKind) ([]Symbol, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	nameLower := strings.ToLower(name)
	var results []Symbol

	for key, symbols := range idx.symbols {
		if strings.Contains(key, nameLower) {
			for _, sym := range symbols {
				if kind == "" || sym.Kind == kind {
					results = append(results, sym)
				}
			}
		}
	}

	return results, nil
}

// FindReferences finds references to a symbol.
func (idx *MemoryIndex) FindReferences(ctx context.Context, symbol Symbol) ([]Reference, error) {
	results, err := idx.Search(ctx, symbol.Name, SearchOptions{
		MaxResults:     100,
		IncludeContent: true,
	})
	if err != nil {
		return nil, err
	}

	var refs []Reference
	for _, r := range results {
		refs = append(refs, Reference{
			Path:         r.Path,
			Line:         r.Line,
			Content:      r.Content,
			IsDefinition: r.Path == symbol.Path && r.Line == symbol.Line,
		})
	}

	return refs, nil
}

// GetContext retrieves relevant code chunks up to token budget.
func (idx *MemoryIndex) GetContext(ctx context.Context, query string, maxTokens int) ([]Chunk, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	// Collect all chunks
	var allChunks []Chunk
	for _, chunk := range idx.chunks {
		allChunks = append(allChunks, *chunk)
	}

	// Sort by relevance
	allChunks = ChunksByRelevance(allChunks, query)

	// Select up to token budget
	var result []Chunk
	totalTokens := 0

	for _, chunk := range allChunks {
		tokens := EstimateTokens(chunk.Content)
		if totalTokens+tokens > maxTokens {
			break
		}
		result = append(result, chunk)
		totalTokens += tokens
	}

	return result, nil
}

// GetFile retrieves a file by path.
func (idx *MemoryIndex) GetFile(ctx context.Context, path string) (*File, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	file, ok := idx.files[path]
	if !ok {
		return nil, ErrNotFound
	}

	return file, nil
}

// GetChunk retrieves a chunk by ID.
func (idx *MemoryIndex) GetChunk(ctx context.Context, id string) (*Chunk, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	chunk, ok := idx.chunks[id]
	if !ok {
		return nil, ErrNotFound
	}

	return chunk, nil
}

// Stats returns index statistics.
func (idx *MemoryIndex) Stats(ctx context.Context) (*IndexStats, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrClosed
	}

	stats := &IndexStats{
		FileCount:  len(idx.files),
		ChunkCount: len(idx.chunks),
		Languages:  make(map[string]int),
	}

	for _, file := range idx.files {
		stats.TotalSize += file.Size
		stats.SymbolCount += len(file.Symbols)
		stats.Languages[file.Language]++
	}

	return stats, nil
}

// Clear removes all data from the index.
func (idx *MemoryIndex) Clear(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrClosed
	}

	idx.files = make(map[string]*File)
	idx.chunks = make(map[string]*Chunk)
	idx.symbols = make(map[string][]Symbol)

	return nil
}

// Close closes the index.
func (idx *MemoryIndex) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.closed = true
	return nil
}

// Helper functions

func getContextLines(lines []string, lineNum, contextLines int) ([]string, []string) {
	var before, after []string

	start := lineNum - contextLines
	if start < 0 {
		start = 0
	}

	end := lineNum + contextLines + 1
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < lineNum; i++ {
		before = append(before, lines[i])
	}

	for i := lineNum + 1; i < end; i++ {
		after = append(after, lines[i])
	}

	return before, after
}

// EstimateTokens estimates token count for text.
func EstimateTokens(text string) int {
	return (len(text) + 3) / 4
}

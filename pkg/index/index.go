package index

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/philippgille/chromem-go"
)

// embeddingDim is the dimension of our simple embeddings
const embeddingDim = 256

// simpleEmbedding creates a simple hash-based embedding for text.
// This enables chromem-go to store documents without requiring external APIs.
// For actual semantic search, use a proper embedding model (Voyage, OpenAI, etc.)
func simpleEmbedding(_ context.Context, text string) ([]float32, error) {
	// Tokenize and create a bag-of-words style embedding
	embedding := make([]float32, embeddingDim)

	// Split into words
	words := strings.Fields(strings.ToLower(text))

	// Hash each word and accumulate into embedding
	for _, word := range words {
		h := fnv.New32a()
		h.Write([]byte(word))
		idx := h.Sum32() % uint32(embeddingDim)
		embedding[idx] += 1.0
	}

	// Normalize
	var sum float32
	for _, v := range embedding {
		sum += v * v
	}
	if sum > 0 {
		norm := float32(1.0 / float64(sum))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding, nil
}

// Indexer manages the code index using chromem-go for vector storage.
type Indexer struct {
	cfg        Config
	db         *chromem.DB
	collection *chromem.Collection
	parser     *Parser
	dagParser  *DAGParser
	dag        *DependencyGraph
	lineage    *ContextLineage
	mu         sync.RWMutex

	// Stats tracking
	fileCount   int
	lastUpdated time.Time
}

// NewIndexer creates a new Indexer with the given configuration.
func NewIndexer(cfg Config) (*Indexer, error) {
	// Ensure index directory exists
	indexPath := cfg.IndexPath
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(cfg.RepoRoot, cfg.IndexPath)
	}
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}

	// Create persistent chromem database
	db, err := chromem.NewPersistentDB(indexPath, false)
	if err != nil {
		return nil, fmt.Errorf("create chromem db: %w", err)
	}

	// Get or create collection for code chunks
	// Using a simple hash-based embedding function for local operation
	collection, err := db.GetOrCreateCollection("code_chunks", nil, simpleEmbedding)
	if err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	// Initialize DAG
	dagPath := filepath.Join(indexPath, "dag.json")
	dag := NewDependencyGraph(dagPath)
	if err := dag.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load DAG: %v\n", err)
	}

	// Initialize LLM client and lineage tracker
	llmClient := NewLLMClient(DefaultLLMConfig())
	lineagePath := filepath.Join(indexPath, "lineage")
	lineage := NewContextLineage(cfg.RepoRoot, lineagePath, llmClient)
	if err := lineage.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load lineage: %v\n", err)
	}

	return &Indexer{
		cfg:        cfg,
		db:         db,
		collection: collection,
		parser:     NewParser(cfg.RepoRoot),
		dagParser:  NewDAGParser(cfg.RepoRoot),
		dag:        dag,
		lineage:    lineage,
	}, nil
}

// IndexFile parses and indexes a single file incrementally.
func (idx *Indexer) IndexFile(path string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if file should be excluded
	if idx.shouldExclude(path) {
		return nil
	}

	// Get relative path
	relPath, err := filepath.Rel(idx.cfg.RepoRoot, path)
	if err != nil {
		relPath = path
	}

	// Remove existing chunks for this file
	if err := idx.removeFileChunks(relPath); err != nil {
		return fmt.Errorf("remove existing chunks: %w", err)
	}

	// Parse file to extract chunks
	chunks, err := idx.parser.ParseFile(path)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	if len(chunks) == 0 {
		return nil
	}

	// Add chunks to collection
	ctx := context.Background()
	docs := make([]chromem.Document, 0, len(chunks))

	for _, chunk := range chunks {
		// Create searchable content combining name, signature, doc, and code
		searchContent := fmt.Sprintf("%s\n%s\n%s\n%s",
			chunk.SymbolName,
			chunk.Signature,
			chunk.DocComment,
			chunk.Content,
		)

		docs = append(docs, chromem.Document{
			ID:        chunk.ID,
			Content:   searchContent,
			Metadata:  chunk.ToMetadata(),
			Embedding: nil, // Will be computed by chromem
		})
	}

	if err := idx.collection.AddDocuments(ctx, docs, runtime); err != nil {
		return fmt.Errorf("add documents: %w", err)
	}

	idx.lastUpdated = time.Now()

	// Update DAG for this file
	if idx.dagParser != nil && idx.dag != nil {
		if err := idx.dagParser.UpdateDAGForFile(idx.dag, path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update DAG for %s: %v\n", path, err)
		}
	}

	return nil
}

// IndexAll performs a full repository index.
func (idx *Indexer) IndexAll() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear existing collection
	if err := idx.clearCollection(); err != nil {
		return fmt.Errorf("clear collection: %w", err)
	}

	// Find all Go files
	var files []string
	err := filepath.Walk(idx.cfg.RepoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip excluded directories
			rel, _ := filepath.Rel(idx.cfg.RepoRoot, path)
			for _, glob := range idx.cfg.ExcludeGlobs {
				if matched, _ := filepath.Match(glob, rel); matched {
					return filepath.SkipDir
				}
				// Check directory patterns (e.g., vendor/**)
				if strings.HasSuffix(glob, "/**") {
					dir := strings.TrimSuffix(glob, "/**")
					if rel == dir || strings.HasPrefix(rel, dir+string(filepath.Separator)) {
						return filepath.SkipDir
					}
				}
			}
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Check exclusions
		if idx.shouldExclude(path) {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}

	// Parse and index each file
	ctx := context.Background()
	var allDocs []chromem.Document
	fileSet := make(map[string]bool)

	for _, path := range files {
		chunks, err := idx.parser.ParseFile(path)
		if err != nil {
			// Log error but continue with other files
			fmt.Fprintf(os.Stderr, "warning: failed to parse %s: %v\n", path, err)
			continue
		}

		relPath, _ := filepath.Rel(idx.cfg.RepoRoot, path)
		fileSet[relPath] = true

		for _, chunk := range chunks {
			searchContent := fmt.Sprintf("%s\n%s\n%s\n%s",
				chunk.SymbolName,
				chunk.Signature,
				chunk.DocComment,
				chunk.Content,
			)

			allDocs = append(allDocs, chromem.Document{
				ID:        chunk.ID,
				Content:   searchContent,
				Metadata:  chunk.ToMetadata(),
				Embedding: nil,
			})
		}
	}

	// Batch add all documents
	if len(allDocs) > 0 {
		if err := idx.collection.AddDocuments(ctx, allDocs, runtime); err != nil {
			return fmt.Errorf("add documents: %w", err)
		}
	}

	idx.fileCount = len(fileSet)
	idx.lastUpdated = time.Now()

	// Build DAG for the repository
	if idx.dagParser != nil && idx.dag != nil {
		if err := idx.dagParser.BuildDAGForRepo(idx.dag, idx.cfg.ExcludeGlobs); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to build DAG: %v\n", err)
		} else {
			if err := idx.dag.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save DAG: %v\n", err)
			}
		}
	}

	return nil
}

// Clear deletes and recreates the collection.
func (idx *Indexer) Clear() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if err := idx.clearCollection(); err != nil {
		return err
	}

	idx.fileCount = 0
	idx.lastUpdated = time.Time{}
	return nil
}

// Stats returns current index statistics.
func (idx *Indexer) Stats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count := idx.collection.Count()
	branch := getCurrentBranch(idx.cfg.RepoRoot)

	return IndexStats{
		DocumentCount:  count,
		FileCount:      idx.fileCount,
		CurrentBranch:  branch,
		LastUpdated:    idx.lastUpdated,
		WatcherRunning: false, // Will be set by watcher
	}
}

// GetCollection returns the underlying chromem collection for search operations.
func (idx *Indexer) GetCollection() *chromem.Collection {
	return idx.collection
}

// GetConfig returns the indexer configuration.
func (idx *Indexer) GetConfig() Config {
	return idx.cfg
}

// GetDAG returns the dependency graph.
func (idx *Indexer) GetDAG() *DependencyGraph {
	return idx.dag
}

// GetLineage returns the context lineage tracker.
func (idx *Indexer) GetLineage() *ContextLineage {
	return idx.lineage
}

// SaveDAG persists the DAG to disk.
func (idx *Indexer) SaveDAG() error {
	if idx.dag != nil {
		return idx.dag.Save()
	}
	return nil
}

// shouldExclude checks if a path should be excluded based on glob patterns.
func (idx *Indexer) shouldExclude(path string) bool {
	relPath, err := filepath.Rel(idx.cfg.RepoRoot, path)
	if err != nil {
		return false
	}

	for _, glob := range idx.cfg.ExcludeGlobs {
		// Handle ** patterns
		if strings.Contains(glob, "**") {
			dir := strings.Split(glob, "**")[0]
			dir = strings.TrimSuffix(dir, "/")
			if strings.HasPrefix(relPath, dir+string(filepath.Separator)) || relPath == dir {
				return true
			}
		} else if matched, _ := filepath.Match(glob, relPath); matched {
			return true
		} else if matched, _ := filepath.Match(glob, filepath.Base(relPath)); matched {
			return true
		}
	}

	return false
}

// removeFileChunks removes all chunks for a given file path.
func (idx *Indexer) removeFileChunks(relPath string) error {
	// chromem-go doesn't have a delete by filter, so we need to track IDs
	// For now, we'll rely on full reindex for updates
	// This is a limitation of the current chromem-go API
	return nil
}

// clearCollection recreates the collection.
func (idx *Indexer) clearCollection() error {
	// Delete and recreate collection - ignore error if collection doesn't exist
	_ = idx.db.DeleteCollection("code_chunks")

	collection, err := idx.db.GetOrCreateCollection("code_chunks", nil, simpleEmbedding)
	if err != nil {
		return fmt.Errorf("recreate collection: %w", err)
	}

	idx.collection = collection
	return nil
}

// runtime is the concurrency level for embedding computation.
const runtime = 4

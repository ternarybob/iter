package index

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/philippgille/chromem-go"
)

// Searcher provides search functionality over the code index.
type Searcher struct {
	indexer *Indexer
}

// NewSearcher creates a new Searcher.
func NewSearcher(indexer *Indexer) *Searcher {
	return &Searcher{indexer: indexer}
}

// Search queries the index and returns matching chunks.
// Uses keyword pre-filtering for candidate selection.
func (s *Searcher) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	// Get all documents for keyword filtering
	collection := s.indexer.GetCollection()
	if collection.Count() == 0 {
		return nil, nil
	}

	// Try semantic search first if embeddings are available
	results, err := s.semanticSearch(ctx, opts)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fall back to keyword search
	return s.keywordSearch(ctx, opts)
}

// semanticSearch uses chromem-go's built-in vector search.
func (s *Searcher) semanticSearch(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	collection := s.indexer.GetCollection()

	// Build where filter for metadata - only use where if we have simple filters
	var where map[string]string
	if opts.Branch != "" {
		where = make(map[string]string)
		where["git_branch"] = opts.Branch
	}

	// Perform query - use a safe limit that won't exceed collection size
	maxResults := opts.Limit * 3
	if maxResults > 50 {
		maxResults = 50
	}
	// Don't request more results than we have documents
	count := collection.Count()
	if maxResults > count {
		maxResults = count
	}
	if maxResults < 1 {
		return nil, nil
	}

	docs, err := collection.Query(ctx, opts.Query, maxResults, where, nil)
	if err != nil {
		return nil, fmt.Errorf("query collection: %w", err)
	}

	var results []SearchResult
	for i, doc := range docs {
		// Apply symbol kind filter if specified
		if opts.SymbolKind != "" {
			if doc.Metadata["symbol_kind"] != opts.SymbolKind {
				continue
			}
		}

		// Apply file path filter if specified
		if opts.FilePath != "" {
			filePath := doc.Metadata["file_path"]
			if !strings.HasPrefix(filePath, opts.FilePath) {
				continue
			}
		}

		chunk := s.resultToChunk(doc)
		results = append(results, SearchResult{
			Chunk: chunk,
			Score: doc.Similarity,
			Rank:  i + 1,
		})

		if len(results) >= opts.Limit {
			break
		}
	}

	return results, nil
}

// keywordSearch performs simple keyword matching.
func (s *Searcher) keywordSearch(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	collection := s.indexer.GetCollection()

	// Get all documents
	// Note: chromem-go doesn't have a list all API, so we query with empty string
	// and high limit to get all documents
	docs, err := collection.Query(ctx, "", collection.Count(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	// Parse query into keywords
	keywords := tokenize(opts.Query)

	// Score and filter documents
	type scored struct {
		doc   docData
		score int
	}
	var scoredDocs []scored

	for _, doc := range docs {
		// Apply filters
		if opts.SymbolKind != "" && doc.Metadata["symbol_kind"] != opts.SymbolKind {
			continue
		}
		if opts.Branch != "" && doc.Metadata["git_branch"] != opts.Branch {
			continue
		}
		if opts.FilePath != "" && !strings.HasPrefix(doc.Metadata["file_path"], opts.FilePath) {
			continue
		}

		// Score by keyword matches
		content := strings.ToLower(doc.Content)
		symbolName := strings.ToLower(doc.Metadata["symbol_name"])
		signature := strings.ToLower(doc.Metadata["signature"])

		score := 0
		for _, kw := range keywords {
			kw = strings.ToLower(kw)

			// Exact symbol name match is worth more
			if symbolName == kw {
				score += 10
			} else if strings.Contains(symbolName, kw) {
				score += 5
			}

			// Signature matches
			if strings.Contains(signature, kw) {
				score += 3
			}

			// Content matches
			count := strings.Count(content, kw)
			score += count
		}

		if score > 0 {
			scoredDocs = append(scoredDocs, scored{
				doc:   docData{ID: doc.ID, Content: doc.Content, Metadata: doc.Metadata},
				score: score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	// Build results
	var results []SearchResult
	for i, sd := range scoredDocs {
		if i >= opts.Limit {
			break
		}

		chunk := s.metadataToChunk(sd.doc.ID, sd.doc.Metadata)
		results = append(results, SearchResult{
			Chunk:      chunk,
			Score:      float32(sd.score) / 100.0, // Normalize score
			Rank:       i + 1,
			MatchCount: sd.score,
		})
	}

	return results, nil
}

// docData holds document data for internal processing.
type docData struct {
	ID       string
	Content  string
	Metadata map[string]string
}

// resultToChunk converts a chromem.Result to a Chunk.
func (s *Searcher) resultToChunk(doc chromem.Result) Chunk {
	return s.metadataToChunk(doc.ID, doc.Metadata)
}

// metadataToChunk reconstructs a Chunk from metadata.
func (s *Searcher) metadataToChunk(id string, meta map[string]string) Chunk {
	startLine, _ := strconv.Atoi(meta["start_line"])
	endLine, _ := strconv.Atoi(meta["end_line"])

	return Chunk{
		ID:         id,
		FilePath:   meta["file_path"],
		SymbolName: meta["symbol_name"],
		SymbolKind: meta["symbol_kind"],
		Signature:  meta["signature"],
		StartLine:  startLine,
		EndLine:    endLine,
		Hash:       meta["hash"],
		Branch:     meta["git_branch"],
	}
}

// tokenize splits a query into keywords.
func tokenize(query string) []string {
	// Split on whitespace and common delimiters
	query = strings.ReplaceAll(query, ".", " ")
	query = strings.ReplaceAll(query, "_", " ")
	query = strings.ReplaceAll(query, "-", " ")
	query = strings.ReplaceAll(query, "(", " ")
	query = strings.ReplaceAll(query, ")", " ")

	words := strings.Fields(query)

	// Filter out very short words
	var keywords []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) >= 2 {
			keywords = append(keywords, w)
		}
	}

	return keywords
}

// FormatResults formats search results as markdown for prompt injection.
func FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No matching code found in index.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Code from Index\n\n")

	for i, r := range results {
		// Format header
		sb.WriteString(fmt.Sprintf("### %d. %s `%s` (%.0f%% match)\n",
			i+1,
			r.Chunk.SymbolKind,
			r.Chunk.SymbolName,
			r.Score*100))

		sb.WriteString(fmt.Sprintf("**File**: `%s` L%d-%d\n",
			r.Chunk.FilePath,
			r.Chunk.StartLine,
			r.Chunk.EndLine))

		if r.Chunk.Signature != "" {
			sb.WriteString(fmt.Sprintf("**Signature**: `%s`\n", r.Chunk.Signature))
		}

		if r.Chunk.DocComment != "" {
			sb.WriteString(fmt.Sprintf("\n> %s\n", strings.TrimSpace(r.Chunk.DocComment)))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatResultsWithCode includes the full source code in results.
func FormatResultsWithCode(results []SearchResult, indexer *Indexer) string {
	if len(results) == 0 {
		return "No matching code found in index.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Code from Index\n\n")

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s `%s` (%.0f%% match)\n",
			i+1,
			r.Chunk.SymbolKind,
			r.Chunk.SymbolName,
			r.Score*100))

		sb.WriteString(fmt.Sprintf("**File**: `%s` L%d-%d\n",
			r.Chunk.FilePath,
			r.Chunk.StartLine,
			r.Chunk.EndLine))

		if r.Chunk.Signature != "" {
			sb.WriteString(fmt.Sprintf("**Signature**: `%s`\n", r.Chunk.Signature))
		}

		// Note: Full content would require re-reading from collection
		// For now, show signature only. Full content can be retrieved via file read.

		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("\n*Found %d results. Indexed at: %s*\n",
		len(results),
		time.Now().Format(time.RFC3339)))

	return sb.String()
}

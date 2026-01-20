package index

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Chunker splits content into chunks.
type Chunker struct {
	size    int
	overlap int
}

// NewChunker creates a new chunker.
func NewChunker(size, overlap int) *Chunker {
	if size <= 0 {
		size = 50
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		overlap = size / 5
	}
	return &Chunker{
		size:    size,
		overlap: overlap,
	}
}

// Chunk splits content into overlapping chunks.
func (c *Chunker) Chunk(path, content, language string) []Chunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	var chunks []Chunk
	step := c.size - c.overlap
	if step <= 0 {
		step = 1
	}

	for start := 0; start < len(lines); start += step {
		end := start + c.size
		if end > len(lines) {
			end = len(lines)
		}

		chunkLines := lines[start:end]
		chunkContent := strings.Join(chunkLines, "\n")

		chunk := Chunk{
			ID:        generateChunkID(path, start+1, end),
			Path:      path,
			StartLine: start + 1, // 1-indexed
			EndLine:   end,
			Content:   chunkContent,
			Language:  language,
			Hash:      hashContent(chunkContent),
		}

		chunks = append(chunks, chunk)

		// If we've reached the end, stop
		if end >= len(lines) {
			break
		}
	}

	return chunks
}

// ChunkWithSymbols splits content into chunks, respecting symbol boundaries.
func (c *Chunker) ChunkWithSymbols(path, content, language string, symbols []Symbol) []Chunk {
	if len(symbols) == 0 {
		return c.Chunk(path, content, language)
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	var chunks []Chunk
	usedLines := make(map[int]bool)

	// First, create chunks around symbol definitions
	for _, sym := range symbols {
		if sym.Line < 1 || sym.Line > len(lines) {
			continue
		}

		// Find chunk boundaries around the symbol
		start := sym.Line - c.overlap
		if start < 1 {
			start = 1
		}

		end := sym.Line + c.size - c.overlap
		if sym.EndLine > sym.Line {
			end = sym.EndLine + c.overlap
		}
		if end > len(lines) {
			end = len(lines)
		}

		// Mark lines as used
		for i := start; i <= end; i++ {
			usedLines[i] = true
		}

		chunkLines := lines[start-1 : end]
		chunkContent := strings.Join(chunkLines, "\n")

		chunk := Chunk{
			ID:        generateChunkID(path, start, end),
			Path:      path,
			StartLine: start,
			EndLine:   end,
			Content:   chunkContent,
			Language:  language,
			Hash:      hashContent(chunkContent),
			Symbols:   findSymbolsInRange(symbols, start, end),
		}

		chunks = append(chunks, chunk)
	}

	// Then, fill in gaps with regular chunks
	start := 1
	for start <= len(lines) {
		// Skip used lines
		for start <= len(lines) && usedLines[start] {
			start++
		}
		if start > len(lines) {
			break
		}

		// Find end of unused section
		end := start
		for end <= len(lines) && !usedLines[end] && end-start < c.size {
			end++
		}
		end--

		if end >= start {
			chunkLines := lines[start-1 : end]
			chunkContent := strings.Join(chunkLines, "\n")

			chunk := Chunk{
				ID:        generateChunkID(path, start, end),
				Path:      path,
				StartLine: start,
				EndLine:   end,
				Content:   chunkContent,
				Language:  language,
				Hash:      hashContent(chunkContent),
			}

			chunks = append(chunks, chunk)
		}

		start = end + 1
	}

	// Sort chunks by start line
	sortChunks(chunks)

	return chunks
}

// generateChunkID creates a unique chunk identifier.
func generateChunkID(path string, startLine, endLine int) string {
	data := path + ":" + itoa(startLine) + "-" + itoa(endLine)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// hashContent creates a content hash.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16])
}

// itoa converts int to string.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// findSymbolsInRange returns symbols within a line range.
func findSymbolsInRange(symbols []Symbol, start, end int) []Symbol {
	var result []Symbol
	for _, sym := range symbols {
		if sym.Line >= start && sym.Line <= end {
			result = append(result, sym)
		}
	}
	return result
}

// sortChunks sorts chunks by start line.
func sortChunks(chunks []Chunk) {
	// Simple insertion sort
	for i := 1; i < len(chunks); i++ {
		key := chunks[i]
		j := i - 1
		for j >= 0 && chunks[j].StartLine > key.StartLine {
			chunks[j+1] = chunks[j]
			j--
		}
		chunks[j+1] = key
	}
}

// MergeChunks combines overlapping chunks.
func MergeChunks(chunks []Chunk) []Chunk {
	if len(chunks) <= 1 {
		return chunks
	}

	sortChunks(chunks)

	var merged []Chunk
	current := chunks[0]

	for i := 1; i < len(chunks); i++ {
		if chunks[i].StartLine <= current.EndLine+1 && chunks[i].Path == current.Path {
			// Merge overlapping chunks
			if chunks[i].EndLine > current.EndLine {
				// Need to extend content
				// This is a simplified merge - real implementation would rebuild content
				current.EndLine = chunks[i].EndLine
				current.Symbols = append(current.Symbols, chunks[i].Symbols...)
				current.ID = generateChunkID(current.Path, current.StartLine, current.EndLine)
				current.Hash = "" // Invalidate hash
			}
		} else {
			merged = append(merged, current)
			current = chunks[i]
		}
	}

	merged = append(merged, current)
	return merged
}

// ChunksByRelevance returns chunks containing relevant content first.
func ChunksByRelevance(chunks []Chunk, query string) []Chunk {
	queryLower := strings.ToLower(query)

	// Score each chunk
	type scoredChunk struct {
		chunk Chunk
		score int
	}

	scored := make([]scoredChunk, len(chunks))
	for i, chunk := range chunks {
		score := 0
		contentLower := strings.ToLower(chunk.Content)

		// Exact match scores highest
		if strings.Contains(contentLower, queryLower) {
			score += 100
			// Count occurrences
			score += strings.Count(contentLower, queryLower) * 10
		}

		// Symbol name matches
		for _, sym := range chunk.Symbols {
			if strings.Contains(strings.ToLower(sym.Name), queryLower) {
				score += 50
			}
		}

		scored[i] = scoredChunk{chunk, score}
	}

	// Sort by score descending
	for i := 1; i < len(scored); i++ {
		key := scored[i]
		j := i - 1
		for j >= 0 && scored[j].score < key.score {
			scored[j+1] = scored[j]
			j--
		}
		scored[j+1] = key
	}

	result := make([]Chunk, len(scored))
	for i, sc := range scored {
		result[i] = sc.chunk
	}

	return result
}

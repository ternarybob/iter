package index

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunker_Chunk(t *testing.T) {
	chunker := NewChunker(10, 2) // 10 lines per chunk, 2 overlap

	content := `line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10
line 11
line 12
line 13
line 14
line 15`

	chunks := chunker.Chunk("test.go", content, "go")

	assert.NotEmpty(t, chunks, "should create chunks")
	assert.Greater(t, len(chunks), 1, "should create multiple chunks for long content")

	// First chunk should start at line 1
	assert.Equal(t, 1, chunks[0].StartLine)

	// Chunks should have proper path and language
	for _, chunk := range chunks {
		assert.Equal(t, "test.go", chunk.Path)
		assert.Equal(t, "go", chunk.Language)
		assert.NotEmpty(t, chunk.ID)
		assert.NotEmpty(t, chunk.Content)
	}
}

func TestChunker_SmallContent(t *testing.T) {
	chunker := NewChunker(50, 10) // Large chunk size

	content := `package main

func main() {
	println("small file")
}`

	chunks := chunker.Chunk("small.go", content, "go")

	assert.Len(t, chunks, 1, "small content should be single chunk")
	assert.Contains(t, chunks[0].Content, "package main")
	assert.Contains(t, chunks[0].Content, "println")
}

func TestChunker_EmptyContent(t *testing.T) {
	chunker := NewChunker(10, 2)

	chunks := chunker.Chunk("empty.go", "", "go")

	assert.Len(t, chunks, 1, "empty content produces single chunk with empty content")
}

func TestChunker_SingleLine(t *testing.T) {
	chunker := NewChunker(10, 2)

	chunks := chunker.Chunk("single.go", "package main", "go")

	assert.Len(t, chunks, 1, "single line should produce one chunk")
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 1, chunks[0].EndLine)
}

func TestChunker_Overlap(t *testing.T) {
	chunker := NewChunker(5, 2) // 5 lines per chunk, 2 overlap

	content := `line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10`

	chunks := chunker.Chunk("test.go", content, "go")

	require.Greater(t, len(chunks), 1, "should have multiple chunks")

	// Check that chunks overlap
	if len(chunks) >= 2 {
		// Second chunk should start before first chunk ends (overlap)
		assert.LessOrEqual(t, chunks[1].StartLine, chunks[0].EndLine,
			"chunks should overlap")
	}
}

func TestChunker_Hash(t *testing.T) {
	chunker := NewChunker(10, 2)

	content := "package test\nfunc Test() {}"
	chunks := chunker.Chunk("test.go", content, "go")

	require.Len(t, chunks, 1)
	assert.NotEmpty(t, chunks[0].Hash, "chunk should have hash")

	// Same content should produce same hash
	chunks2 := chunker.Chunk("test.go", content, "go")
	assert.Equal(t, chunks[0].Hash, chunks2[0].Hash, "same content should have same hash")
}

func TestChunker_DifferentContentDifferentHash(t *testing.T) {
	chunker := NewChunker(10, 2)

	chunks1 := chunker.Chunk("a.go", "package a", "go")
	chunks2 := chunker.Chunk("b.go", "package b", "go")

	require.Len(t, chunks1, 1)
	require.Len(t, chunks2, 1)

	assert.NotEqual(t, chunks1[0].Hash, chunks2[0].Hash,
		"different content should have different hash")
}

func TestChunker_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		chunkSize  int
		overlap    int
		content    string
		wantChunks int
		wantFirst  int // expected start line of first chunk
		wantLast   int // expected end line of last chunk
	}{
		{
			name:       "empty",
			chunkSize:  10,
			overlap:    2,
			content:    "",
			wantChunks: 1,
			wantFirst:  1,
			wantLast:   1,
		},
		{
			name:       "single line",
			chunkSize:  10,
			overlap:    2,
			content:    "single",
			wantChunks: 1,
			wantFirst:  1,
			wantLast:   1,
		},
		{
			name:       "exactly chunk size",
			chunkSize:  3,
			overlap:    1,
			content:    "a\nb\nc",
			wantChunks: 1,
			wantFirst:  1,
			wantLast:   3,
		},
		{
			name:       "just over chunk size",
			chunkSize:  3,
			overlap:    1,
			content:    "a\nb\nc\nd",
			wantChunks: 2,
			wantFirst:  1,
			wantLast:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewChunker(tt.chunkSize, tt.overlap)
			chunks := chunker.Chunk("test.txt", tt.content, "text")

			assert.Len(t, chunks, tt.wantChunks, "chunk count mismatch")

			if tt.wantChunks > 0 {
				assert.Equal(t, tt.wantFirst, chunks[0].StartLine, "first chunk start line")
				assert.Equal(t, tt.wantLast, chunks[len(chunks)-1].EndLine, "last chunk end line")
			}
		})
	}
}


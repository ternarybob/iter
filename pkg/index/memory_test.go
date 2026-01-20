package index

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryIndex_IndexFile(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()
	content := []byte(`package main

func main() {
	fmt.Println("Hello, World!")
}
`)

	err := idx.IndexFile(ctx, "main.go", content)
	require.NoError(t, err, "should index file")

	// Verify file is indexed
	file, err := idx.GetFile(ctx, "main.go")
	require.NoError(t, err, "should get file")
	assert.Equal(t, "main.go", file.Path)
	assert.Equal(t, "go", file.Language)
}

func TestMemoryIndex_Search(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()

	// Index some files
	file1 := []byte(`package handlers

func HandleRequest() {
	// process request
}
`)
	file2 := []byte(`package utils

func FormatRequest(r Request) string {
	return r.String()
}
`)

	require.NoError(t, idx.IndexFile(ctx, "handlers/handler.go", file1))
	require.NoError(t, idx.IndexFile(ctx, "utils/format.go", file2))

	// Search for "Request"
	results, err := idx.Search(ctx, "Request", DefaultSearchOptions())
	require.NoError(t, err, "should search")

	assert.NotEmpty(t, results, "should find results")

	// Both files contain "Request"
	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	assert.Contains(t, paths, "handlers/handler.go")
	assert.Contains(t, paths, "utils/format.go")
}

func TestMemoryIndex_Search_CaseSensitive(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()
	content := []byte(`package test

func TestFunction() {}
func testHelper() {}
`)

	require.NoError(t, idx.IndexFile(ctx, "test.go", content))

	// Case sensitive search
	opts := DefaultSearchOptions()
	opts.CaseSensitive = true

	results, err := idx.Search(ctx, "Test", opts)
	require.NoError(t, err)

	// Should find TestFunction but context might vary
	assert.NotEmpty(t, results)
}

func TestMemoryIndex_FindSymbol(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()
	content := []byte(`package main

type User struct {
	Name string
	Age  int
}

func NewUser(name string) *User {
	return &User{Name: name}
}

func (u *User) GetName() string {
	return u.Name
}
`)

	require.NoError(t, idx.IndexFile(ctx, "user.go", content))

	// Find function
	funcs, err := idx.FindSymbol(ctx, "NewUser", SymbolFunction)
	require.NoError(t, err)
	assert.NotEmpty(t, funcs, "should find NewUser function")

	// Find struct
	structs, err := idx.FindSymbol(ctx, "User", SymbolStruct)
	require.NoError(t, err)
	assert.NotEmpty(t, structs, "should find User struct")
}

func TestMemoryIndex_GetContext(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()
	content := []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello")
}

func helper() {
	// helper function
}
`)

	require.NoError(t, idx.IndexFile(ctx, "main.go", content))

	chunks, err := idx.GetContext(ctx, "main", 1000)
	require.NoError(t, err)
	assert.NotEmpty(t, chunks, "should return context chunks")
}

func TestMemoryIndex_RemoveFile(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()
	content := []byte("package test")

	require.NoError(t, idx.IndexFile(ctx, "test.go", content))

	// Verify file exists
	_, err := idx.GetFile(ctx, "test.go")
	require.NoError(t, err, "file should exist")

	// Remove file
	err = idx.RemoveFile(ctx, "test.go")
	require.NoError(t, err, "should remove file")

	// Verify file is gone
	_, err = idx.GetFile(ctx, "test.go")
	assert.Error(t, err, "file should not exist after removal")
}

func TestMemoryIndex_Stats(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()

	// Index multiple files
	require.NoError(t, idx.IndexFile(ctx, "a.go", []byte("package a")))
	require.NoError(t, idx.IndexFile(ctx, "b.go", []byte("package b")))
	require.NoError(t, idx.IndexFile(ctx, "c.py", []byte("print('hello')")))

	stats, err := idx.Stats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 3, stats.FileCount, "should have 3 files")
	assert.NotEmpty(t, stats.Languages, "should have language counts")
}

func TestMemoryIndex_Clear(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()

	require.NoError(t, idx.IndexFile(ctx, "test.go", []byte("package test")))

	stats, _ := idx.Stats(ctx)
	require.Equal(t, 1, stats.FileCount, "should have 1 file")

	err := idx.Clear(ctx)
	require.NoError(t, err, "should clear")

	stats, _ = idx.Stats(ctx)
	assert.Equal(t, 0, stats.FileCount, "should have 0 files after clear")
}

func TestMemoryIndex_IndexDirectory(t *testing.T) {
	idx := NewMemoryIndex()
	defer idx.Close()

	ctx := context.Background()

	// Index the current package directory
	opts := DefaultIndexOptions()
	opts.IncludePatterns = []string{"*.go"}
	opts.ExcludePatterns = []string{"*_test.go"}

	// This will index actual files in the package
	err := idx.IndexDirectory(ctx, ".", opts)
	require.NoError(t, err, "should index directory")

	stats, _ := idx.Stats(ctx)
	assert.Greater(t, stats.FileCount, 0, "should have indexed some files")
}

func TestMemoryIndex_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		query       string
		wantResults bool
	}{
		{
			name: "single file match",
			files: map[string]string{
				"test.go": "package main\nfunc TestFunc() {}",
			},
			query:       "TestFunc",
			wantResults: true,
		},
		{
			name: "no match",
			files: map[string]string{
				"test.go": "package main\nfunc Other() {}",
			},
			query:       "NotFound",
			wantResults: false,
		},
		{
			name: "multiple files match",
			files: map[string]string{
				"a.go": "package a\nvar common = 1",
				"b.go": "package b\nvar common = 2",
			},
			query:       "common",
			wantResults: true,
		},
		{
			name:        "empty index",
			files:       map[string]string{},
			query:       "anything",
			wantResults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := NewMemoryIndex()
			defer idx.Close()

			ctx := context.Background()

			for path, content := range tt.files {
				require.NoError(t, idx.IndexFile(ctx, path, []byte(content)))
			}

			results, err := idx.Search(ctx, tt.query, DefaultSearchOptions())
			require.NoError(t, err)

			if tt.wantResults {
				assert.NotEmpty(t, results, "expected results")
			} else {
				assert.Empty(t, results, "expected no results")
			}
		})
	}
}

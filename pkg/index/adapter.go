package index

import (
	"context"

	"github.com/ternarybob/iter/pkg/sdk"
)

// SDKAdapter wraps an Index to implement sdk.Index.
type SDKAdapter struct {
	idx Index
	ctx context.Context
}

// NewSDKAdapter creates an adapter that implements sdk.Index.
func NewSDKAdapter(idx Index) *SDKAdapter {
	return &SDKAdapter{
		idx: idx,
		ctx: context.Background(),
	}
}

// Search implements sdk.Index.
func (a *SDKAdapter) Search(query string, opts sdk.SearchOptions) ([]sdk.SearchResult, error) {
	indexOpts := SearchOptions{
		MaxResults:     opts.MaxResults,
		FilePatterns:   opts.FilePatterns,
		IncludeContent: opts.IncludeContent,
		CaseSensitive:  opts.CaseSensitive,
	}

	results, err := a.idx.Search(a.ctx, query, indexOpts)
	if err != nil {
		return nil, err
	}

	var sdkResults []sdk.SearchResult
	for _, r := range results {
		sdkResult := sdk.SearchResult{
			Path:    r.Path,
			Line:    r.Line,
			Content: r.Content,
			Score:   r.Score,
		}
		// Combine context
		if len(r.ContextBefore) > 0 || len(r.ContextAfter) > 0 {
			sdkResult.Context = append(r.ContextBefore, r.ContextAfter...)
		}
		sdkResults = append(sdkResults, sdkResult)
	}

	return sdkResults, nil
}

// FindSymbol implements sdk.Index.
func (a *SDKAdapter) FindSymbol(name string, kind sdk.SymbolKind) ([]sdk.Symbol, error) {
	indexKind := SymbolKind(kind)
	symbols, err := a.idx.FindSymbol(a.ctx, name, indexKind)
	if err != nil {
		return nil, err
	}

	var sdkSymbols []sdk.Symbol
	for _, s := range symbols {
		sdkSymbols = append(sdkSymbols, sdk.Symbol{
			Name:          s.Name,
			Kind:          sdk.SymbolKind(s.Kind),
			Path:          s.Path,
			Line:          s.Line,
			Signature:     s.Signature,
			Documentation: s.Documentation,
		})
	}

	return sdkSymbols, nil
}

// GetContext implements sdk.Index.
func (a *SDKAdapter) GetContext(query string, maxTokens int) ([]sdk.Chunk, error) {
	chunks, err := a.idx.GetContext(a.ctx, query, maxTokens)
	if err != nil {
		return nil, err
	}

	var sdkChunks []sdk.Chunk
	for _, c := range chunks {
		sdkChunk := sdk.Chunk{
			ID:        c.ID,
			Path:      c.Path,
			StartLine: c.StartLine,
			EndLine:   c.EndLine,
			Content:   c.Content,
			Language:  c.Language,
		}
		for _, s := range c.Symbols {
			sdkChunk.Symbols = append(sdkChunk.Symbols, sdk.Symbol{
				Name:          s.Name,
				Kind:          sdk.SymbolKind(s.Kind),
				Path:          s.Path,
				Line:          s.Line,
				Signature:     s.Signature,
				Documentation: s.Documentation,
			})
		}
		sdkChunks = append(sdkChunks, sdkChunk)
	}

	return sdkChunks, nil
}

// GetFile implements sdk.Index.
func (a *SDKAdapter) GetFile(path string) (*sdk.File, error) {
	file, err := a.idx.GetFile(a.ctx, path)
	if err != nil {
		return nil, err
	}

	sdkFile := &sdk.File{
		Path:     file.Path,
		Content:  file.Content,
		Language: file.Language,
		Size:     file.Size,
		ModTime:  file.ModTime,
	}

	for _, c := range file.Chunks {
		sdkChunk := sdk.Chunk{
			ID:        c.ID,
			Path:      c.Path,
			StartLine: c.StartLine,
			EndLine:   c.EndLine,
			Content:   c.Content,
			Language:  c.Language,
		}
		sdkFile.Chunks = append(sdkFile.Chunks, sdkChunk)
	}

	for _, s := range file.Symbols {
		sdkFile.Symbols = append(sdkFile.Symbols, sdk.Symbol{
			Name:          s.Name,
			Kind:          sdk.SymbolKind(s.Kind),
			Path:          s.Path,
			Line:          s.Line,
			Signature:     s.Signature,
			Documentation: s.Documentation,
		})
	}

	return sdkFile, nil
}

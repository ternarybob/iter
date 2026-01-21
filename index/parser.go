package index

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Parser extracts indexable symbols from Go source files.
type Parser struct {
	repoRoot string
	fset     *token.FileSet
}

// NewParser creates a new Parser for extracting symbols.
func NewParser(repoRoot string) *Parser {
	return &Parser{
		repoRoot: repoRoot,
		fset:     token.NewFileSet(),
	}
}

// ParseFile extracts all indexable chunks from a Go source file.
func (p *Parser) ParseFile(path string) ([]Chunk, error) {
	// Reset file set for each file to avoid accumulation
	p.fset = token.NewFileSet()

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	file, err := parser.ParseFile(p.fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(p.repoRoot, path)
	if err != nil {
		relPath = path
	}

	// Get current git branch
	branch := getCurrentBranch(p.repoRoot)

	var chunks []Chunk

	// Extract function declarations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			chunk := p.extractFunc(d, src, relPath, branch)
			chunks = append(chunks, chunk)

		case *ast.GenDecl:
			// Extract type and const declarations
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					chunk := p.extractType(d, s, src, relPath, branch)
					chunks = append(chunks, chunk)

				case *ast.ValueSpec:
					if d.Tok == token.CONST {
						chunks = append(chunks, p.extractConsts(d, s, src, relPath, branch)...)
					}
				}
			}
		}
	}

	return chunks, nil
}

// extractFunc extracts a function/method declaration.
func (p *Parser) extractFunc(fn *ast.FuncDecl, src []byte, relPath, branch string) Chunk {
	startPos := p.fset.Position(fn.Pos())
	endPos := p.fset.Position(fn.End())

	// Extract the full function source
	content := string(src[fn.Pos()-1 : fn.End()-1])

	// Build signature
	var sig strings.Builder
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sig.WriteString("func (")
		sig.WriteString(p.nodeToString(fn.Recv.List[0].Type))
		sig.WriteString(") ")
	} else {
		sig.WriteString("func ")
	}
	sig.WriteString(fn.Name.Name)
	sig.WriteString(p.nodeToString(fn.Type.Params))
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		sig.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sig.WriteString(p.nodeToString(fn.Type.Results.List[0].Type))
		} else {
			sig.WriteString(p.nodeToString(fn.Type.Results))
		}
	}

	// Determine if method or function
	kind := "function"
	if fn.Recv != nil {
		kind = "method"
	}

	// Extract doc comment
	var doc string
	if fn.Doc != nil {
		doc = fn.Doc.Text()
	}

	return Chunk{
		ID:         fmt.Sprintf("%s:%d", relPath, startPos.Line),
		FilePath:   relPath,
		SymbolName: fn.Name.Name,
		SymbolKind: kind,
		Content:    content,
		Signature:  sig.String(),
		DocComment: doc,
		StartLine:  startPos.Line,
		EndLine:    endPos.Line,
		Hash:       hashContent(content),
		Branch:     branch,
		IndexedAt:  time.Now(),
	}
}

// extractType extracts a type declaration.
func (p *Parser) extractType(gen *ast.GenDecl, ts *ast.TypeSpec, src []byte, relPath, branch string) Chunk {
	startPos := p.fset.Position(gen.Pos())
	endPos := p.fset.Position(gen.End())

	// For single type decls, use the type spec position
	if len(gen.Specs) == 1 {
		startPos = p.fset.Position(ts.Pos())
		endPos = p.fset.Position(ts.End())
	}

	// Extract source content
	var content string
	if int(gen.Pos()-1) < len(src) && int(gen.End()-1) <= len(src) {
		content = string(src[gen.Pos()-1 : gen.End()-1])
	}

	// Build signature
	var sig strings.Builder
	sig.WriteString("type ")
	sig.WriteString(ts.Name.Name)
	switch t := ts.Type.(type) {
	case *ast.StructType:
		sig.WriteString(" struct{...}")
	case *ast.InterfaceType:
		sig.WriteString(" interface{...}")
	case *ast.Ident:
		sig.WriteString(" ")
		sig.WriteString(t.Name)
	default:
		sig.WriteString(" ")
		sig.WriteString(p.nodeToString(ts.Type))
	}

	// Extract doc comment
	var doc string
	if gen.Doc != nil {
		doc = gen.Doc.Text()
	} else if ts.Doc != nil {
		doc = ts.Doc.Text()
	}

	return Chunk{
		ID:         fmt.Sprintf("%s:%d", relPath, startPos.Line),
		FilePath:   relPath,
		SymbolName: ts.Name.Name,
		SymbolKind: "type",
		Content:    content,
		Signature:  sig.String(),
		DocComment: doc,
		StartLine:  startPos.Line,
		EndLine:    endPos.Line,
		Hash:       hashContent(content),
		Branch:     branch,
		IndexedAt:  time.Now(),
	}
}

// extractConsts extracts constant declarations.
func (p *Parser) extractConsts(gen *ast.GenDecl, vs *ast.ValueSpec, src []byte, relPath, branch string) []Chunk {
	var chunks []Chunk

	for _, name := range vs.Names {
		if !name.IsExported() && name.Name != "_" {
			continue // Skip unexported and blank identifiers
		}

		startPos := p.fset.Position(vs.Pos())
		endPos := p.fset.Position(vs.End())

		// Build content
		var content strings.Builder
		content.WriteString("const ")
		content.WriteString(name.Name)
		if vs.Type != nil {
			content.WriteString(" ")
			content.WriteString(p.nodeToString(vs.Type))
		}
		if len(vs.Values) > 0 {
			content.WriteString(" = ")
			content.WriteString(p.nodeToString(vs.Values[0]))
		}

		// Extract doc comment
		var doc string
		if vs.Doc != nil {
			doc = vs.Doc.Text()
		} else if gen.Doc != nil {
			doc = gen.Doc.Text()
		}

		chunks = append(chunks, Chunk{
			ID:         fmt.Sprintf("%s:%d:%s", relPath, startPos.Line, name.Name),
			FilePath:   relPath,
			SymbolName: name.Name,
			SymbolKind: "const",
			Content:    content.String(),
			Signature:  content.String(),
			DocComment: doc,
			StartLine:  startPos.Line,
			EndLine:    endPos.Line,
			Hash:       hashContent(content.String()),
			Branch:     branch,
			IndexedAt:  time.Now(),
		})
	}

	return chunks
}

// nodeToString converts an AST node to its string representation.
func (p *Parser) nodeToString(node ast.Node) string {
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, p.fset, node)
	return buf.String()
}

// hashContent returns a SHA-256 hash of the content.
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// getCurrentBranch reads the current git branch from .git/HEAD.
func getCurrentBranch(repoRoot string) string {
	headPath := filepath.Join(repoRoot, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))

	// Format: ref: refs/heads/branch-name
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}

	// Detached HEAD - return short hash
	if len(content) >= 7 {
		return content[:7]
	}

	return content
}

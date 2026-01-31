package index

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// DAGParser extracts dependency relationships from Go source files.
type DAGParser struct {
	repoRoot string
	fset     *token.FileSet
}

// NewDAGParser creates a new DAG parser.
func NewDAGParser(repoRoot string) *DAGParser {
	return &DAGParser{
		repoRoot: repoRoot,
		fset:     token.NewFileSet(),
	}
}

// ParseFileResult contains the extracted nodes and edges from a file.
type ParseFileResult struct {
	Nodes   []*Node
	Edges   []Edge
	Package string
	Imports []ImportInfo
}

// ImportInfo contains information about an import.
type ImportInfo struct {
	Path  string // Import path
	Alias string // Local alias (empty if none)
	Line  int    // Line number
}

// ParseFileForDependencies extracts nodes and edges from a Go file.
func (p *DAGParser) ParseFileForDependencies(path string) (*ParseFileResult, error) {
	// Reset file set
	p.fset = token.NewFileSet()

	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	file, err := parser.ParseFile(p.fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	relPath, err := filepath.Rel(p.repoRoot, path)
	if err != nil {
		relPath = path
	}

	result := &ParseFileResult{
		Package: file.Name.Name,
	}

	// Extract imports
	result.Imports = p.extractImports(file)

	// Extract all declarations as nodes
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			node := p.funcDeclToNode(d, relPath, result.Package)
			result.Nodes = append(result.Nodes, node)

			// Extract calls from function body
			if d.Body != nil {
				calls := p.extractCalls(d.Body, relPath, node.ID)
				result.Edges = append(result.Edges, calls...)
			}

		case *ast.GenDecl:
			nodes, edges := p.genDeclToNodesAndEdges(d, relPath, result.Package, src)
			result.Nodes = append(result.Nodes, nodes...)
			result.Edges = append(result.Edges, edges...)
		}
	}

	// Add import edges
	for _, imp := range result.Imports {
		// Create edge from file (or package) to imported package
		edge := Edge{
			Source:   fmt.Sprintf("%s.%s", result.Package, filepath.Base(relPath)),
			Target:   imp.Path,
			EdgeType: EdgeImports,
			FilePath: relPath,
			Line:     imp.Line,
		}
		result.Edges = append(result.Edges, edge)
	}

	return result, nil
}

// funcDeclToNode converts a function declaration to a Node.
func (p *DAGParser) funcDeclToNode(fn *ast.FuncDecl, relPath, pkg string) *Node {
	startPos := p.fset.Position(fn.Pos())
	endPos := p.fset.Position(fn.End())

	kind := "function"
	var receiver string
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		kind = "method"
		receiver = p.typeToString(fn.Recv.List[0].Type)
	}

	// Build full qualified name
	var id string
	if receiver != "" {
		id = fmt.Sprintf("%s.%s.%s", pkg, receiver, fn.Name.Name)
	} else {
		id = fmt.Sprintf("%s.%s", pkg, fn.Name.Name)
	}

	// Build signature
	sig := p.buildFuncSignature(fn, receiver)

	var doc string
	if fn.Doc != nil {
		doc = fn.Doc.Text()
	}

	return &Node{
		ID:         id,
		Name:       fn.Name.Name,
		Kind:       kind,
		FilePath:   relPath,
		Package:    pkg,
		StartLine:  startPos.Line,
		EndLine:    endPos.Line,
		Signature:  sig,
		DocComment: doc,
	}
}

// genDeclToNodesAndEdges extracts nodes and edges from a generic declaration.
func (p *DAGParser) genDeclToNodesAndEdges(gen *ast.GenDecl, relPath, pkg string, src []byte) ([]*Node, []Edge) {
	var nodes []*Node
	var edges []Edge

	for _, spec := range gen.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			node := p.typeSpecToNode(gen, s, relPath, pkg)
			nodes = append(nodes, node)

			// Extract implements/embeds relationships
			typeEdges := p.extractTypeRelationships(s, relPath, pkg)
			edges = append(edges, typeEdges...)

		case *ast.ValueSpec:
			// Constants and variables
			if gen.Tok == token.CONST || gen.Tok == token.VAR {
				for _, name := range s.Names {
					if !name.IsExported() && name.Name != "_" {
						continue
					}

					startPos := p.fset.Position(s.Pos())
					endPos := p.fset.Position(s.End())

					kind := "const"
					if gen.Tok == token.VAR {
						kind = "var"
					}

					id := fmt.Sprintf("%s.%s", pkg, name.Name)

					var doc string
					if s.Doc != nil {
						doc = s.Doc.Text()
					} else if gen.Doc != nil {
						doc = gen.Doc.Text()
					}

					var sig string
					if s.Type != nil {
						sig = fmt.Sprintf("%s %s %s", kind, name.Name, p.typeToString(s.Type))
					} else {
						sig = fmt.Sprintf("%s %s", kind, name.Name)
					}

					nodes = append(nodes, &Node{
						ID:         id,
						Name:       name.Name,
						Kind:       kind,
						FilePath:   relPath,
						Package:    pkg,
						StartLine:  startPos.Line,
						EndLine:    endPos.Line,
						Signature:  sig,
						DocComment: doc,
					})
				}
			}
		}
	}

	return nodes, edges
}

// typeSpecToNode converts a type specification to a Node.
func (p *DAGParser) typeSpecToNode(gen *ast.GenDecl, ts *ast.TypeSpec, relPath, pkg string) *Node {
	startPos := p.fset.Position(ts.Pos())
	endPos := p.fset.Position(ts.End())

	// Determine kind based on underlying type
	kind := "type"
	switch ts.Type.(type) {
	case *ast.InterfaceType:
		kind = "interface"
	case *ast.StructType:
		kind = "struct"
	}

	id := fmt.Sprintf("%s.%s", pkg, ts.Name.Name)

	var doc string
	if gen.Doc != nil {
		doc = gen.Doc.Text()
	} else if ts.Doc != nil {
		doc = ts.Doc.Text()
	}

	sig := fmt.Sprintf("type %s %s", ts.Name.Name, kind)

	return &Node{
		ID:         id,
		Name:       ts.Name.Name,
		Kind:       kind,
		FilePath:   relPath,
		Package:    pkg,
		StartLine:  startPos.Line,
		EndLine:    endPos.Line,
		Signature:  sig,
		DocComment: doc,
	}
}

// extractImports extracts import information from a file.
func (p *DAGParser) extractImports(file *ast.File) []ImportInfo {
	var imports []ImportInfo

	for _, imp := range file.Imports {
		info := ImportInfo{
			Line: p.fset.Position(imp.Pos()).Line,
		}

		// Remove quotes from path
		info.Path = strings.Trim(imp.Path.Value, `"`)

		if imp.Name != nil {
			info.Alias = imp.Name.Name
		}

		imports = append(imports, info)
	}

	return imports
}

// extractCalls extracts function call edges from a block statement.
func (p *DAGParser) extractCalls(block *ast.BlockStmt, relPath, sourceID string) []Edge {
	var edges []Edge

	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		line := p.fset.Position(call.Pos()).Line
		targetID := p.resolveCallTarget(call)

		if targetID != "" {
			edges = append(edges, Edge{
				Source:   sourceID,
				Target:   targetID,
				EdgeType: EdgeCalls,
				FilePath: relPath,
				Line:     line,
			})
		}

		return true
	})

	return edges
}

// resolveCallTarget attempts to resolve a call expression to a target ID.
func (p *DAGParser) resolveCallTarget(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		// Simple function call: funcName()
		return fn.Name

	case *ast.SelectorExpr:
		// Method call or package function: pkg.Func() or obj.Method()
		if x, ok := fn.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", x.Name, fn.Sel.Name)
		}
		// Chained call: a.b.Method() - just return the method name
		return fn.Sel.Name

	default:
		return ""
	}
}

// extractTypeRelationships extracts implements/embeds relationships from a type.
func (p *DAGParser) extractTypeRelationships(ts *ast.TypeSpec, relPath, pkg string) []Edge {
	var edges []Edge
	sourceID := fmt.Sprintf("%s.%s", pkg, ts.Name.Name)

	switch t := ts.Type.(type) {
	case *ast.StructType:
		if t.Fields != nil {
			for _, field := range t.Fields.List {
				// Anonymous field = embedded type
				if len(field.Names) == 0 {
					targetID := p.typeToString(field.Type)
					if targetID != "" {
						edges = append(edges, Edge{
							Source:   sourceID,
							Target:   targetID,
							EdgeType: EdgeEmbeds,
							FilePath: relPath,
							Line:     p.fset.Position(field.Pos()).Line,
						})
					}
				} else {
					// Named field - track type usage
					targetID := p.typeToString(field.Type)
					if targetID != "" && !isBuiltinType(targetID) {
						edges = append(edges, Edge{
							Source:   sourceID,
							Target:   targetID,
							EdgeType: EdgeUses,
							FilePath: relPath,
							Line:     p.fset.Position(field.Pos()).Line,
						})
					}
				}
			}
		}

	case *ast.InterfaceType:
		if t.Methods != nil {
			for _, method := range t.Methods.List {
				// Embedded interface
				if len(method.Names) == 0 {
					targetID := p.typeToString(method.Type)
					if targetID != "" {
						edges = append(edges, Edge{
							Source:   sourceID,
							Target:   targetID,
							EdgeType: EdgeEmbeds,
							FilePath: relPath,
							Line:     p.fset.Position(method.Pos()).Line,
						})
					}
				}
			}
		}
	}

	return edges
}

// typeToString converts a type expression to a string.
func (p *DAGParser) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name

	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", x.Name, t.Sel.Name)
		}
		return t.Sel.Name

	case *ast.StarExpr:
		return p.typeToString(t.X)

	case *ast.ArrayType:
		return "[]" + p.typeToString(t.Elt)

	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", p.typeToString(t.Key), p.typeToString(t.Value))

	case *ast.ChanType:
		return "chan " + p.typeToString(t.Value)

	case *ast.FuncType:
		return "func"

	case *ast.InterfaceType:
		return "interface{}"

	case *ast.StructType:
		return "struct{}"

	default:
		return ""
	}
}

// buildFuncSignature builds a function signature string.
func (p *DAGParser) buildFuncSignature(fn *ast.FuncDecl, receiver string) string {
	var sig strings.Builder

	sig.WriteString("func ")
	if receiver != "" {
		sig.WriteString("(")
		sig.WriteString(receiver)
		sig.WriteString(") ")
	}
	sig.WriteString(fn.Name.Name)
	sig.WriteString("(")

	// Parameters
	if fn.Type.Params != nil {
		var params []string
		for _, param := range fn.Type.Params.List {
			typeStr := p.typeToString(param.Type)
			if len(param.Names) > 0 {
				for _, name := range param.Names {
					params = append(params, fmt.Sprintf("%s %s", name.Name, typeStr))
				}
			} else {
				params = append(params, typeStr)
			}
		}
		sig.WriteString(strings.Join(params, ", "))
	}
	sig.WriteString(")")

	// Return types
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		sig.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sig.WriteString(p.typeToString(fn.Type.Results.List[0].Type))
		} else {
			sig.WriteString("(")
			var results []string
			for _, result := range fn.Type.Results.List {
				typeStr := p.typeToString(result.Type)
				if len(result.Names) > 0 {
					for _, name := range result.Names {
						results = append(results, fmt.Sprintf("%s %s", name.Name, typeStr))
					}
				} else {
					results = append(results, typeStr)
				}
			}
			sig.WriteString(strings.Join(results, ", "))
			sig.WriteString(")")
		}
	}

	return sig.String()
}

// isBuiltinType checks if a type name is a Go builtin.
func isBuiltinType(name string) bool {
	builtins := map[string]bool{
		"bool":       true,
		"byte":       true,
		"complex64":  true,
		"complex128": true,
		"error":      true,
		"float32":    true,
		"float64":    true,
		"int":        true,
		"int8":       true,
		"int16":      true,
		"int32":      true,
		"int64":      true,
		"rune":       true,
		"string":     true,
		"uint":       true,
		"uint8":      true,
		"uint16":     true,
		"uint32":     true,
		"uint64":     true,
		"uintptr":    true,
		"any":        true,
		"comparable": true,
	}
	return builtins[name]
}

// UpdateDAGForFile updates the DAG with the contents of a single file.
func (p *DAGParser) UpdateDAGForFile(dag *DependencyGraph, path string) error {
	// Remove existing data for this file
	relPath, err := filepath.Rel(p.repoRoot, path)
	if err != nil {
		relPath = path
	}
	dag.RemoveFile(relPath)

	// Parse the file
	result, err := p.ParseFileForDependencies(path)
	if err != nil {
		return err
	}

	// Add new nodes
	for _, node := range result.Nodes {
		dag.AddNode(node)
	}

	// Add new edges
	for _, edge := range result.Edges {
		dag.AddEdge(edge)
	}

	return nil
}

// BuildDAGForRepo builds a complete DAG for the repository.
func (p *DAGParser) BuildDAGForRepo(dag *DependencyGraph, excludeGlobs []string) error {
	dag.Clear()

	return filepath.Walk(p.repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip excluded directories
			relPath, _ := filepath.Rel(p.repoRoot, path)
			for _, glob := range excludeGlobs {
				if strings.HasSuffix(glob, "/**") {
					dir := strings.TrimSuffix(glob, "/**")
					if relPath == dir || strings.HasPrefix(relPath, dir+string(filepath.Separator)) {
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

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip excluded files
		relPath, _ := filepath.Rel(p.repoRoot, path)
		for _, glob := range excludeGlobs {
			if matched, _ := filepath.Match(glob, relPath); matched {
				return nil
			}
			if matched, _ := filepath.Match(glob, filepath.Base(relPath)); matched {
				return nil
			}
		}

		// Parse and add to DAG
		result, err := p.ParseFileForDependencies(path)
		if err != nil {
			// Log but continue with other files
			fmt.Fprintf(os.Stderr, "warning: failed to parse %s for DAG: %v\n", path, err)
			return nil
		}

		for _, node := range result.Nodes {
			dag.AddNode(node)
		}
		for _, edge := range result.Edges {
			dag.AddEdge(edge)
		}

		return nil
	})
}

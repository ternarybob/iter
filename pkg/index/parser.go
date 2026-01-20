package index

import (
	"regexp"
	"strings"
)

// Parser extracts symbols from source code.
// This is a regex-based parser. For more accurate parsing, use tree-sitter.
type Parser struct {
	patterns map[string][]*symbolPattern
}

type symbolPattern struct {
	kind    SymbolKind
	pattern *regexp.Regexp
	// Group indices: 0=full match, 1=name, 2=signature (optional)
	nameGroup      int
	signatureGroup int
}

// NewParser creates a new parser.
func NewParser() *Parser {
	p := &Parser{
		patterns: make(map[string][]*symbolPattern),
	}
	p.initPatterns()
	return p
}

// initPatterns initializes language-specific patterns.
func (p *Parser) initPatterns() {
	// Go patterns
	p.patterns["go"] = []*symbolPattern{
		{SymbolFunction, regexp.MustCompile(`(?m)^func\s+(\w+)\s*(\([^)]*\)(?:\s*\([^)]*\)|\s*\w+)?)`), 1, 2},
		{SymbolMethod, regexp.MustCompile(`(?m)^func\s*\([^)]+\)\s*(\w+)\s*(\([^)]*\)(?:\s*\([^)]*\)|\s*\w+)?)`), 1, 2},
		{SymbolStruct, regexp.MustCompile(`(?m)^type\s+(\w+)\s+struct\s*\{`), 1, 0},
		{SymbolInterface, regexp.MustCompile(`(?m)^type\s+(\w+)\s+interface\s*\{`), 1, 0},
		{SymbolType, regexp.MustCompile(`(?m)^type\s+(\w+)\s+[^{]+$`), 1, 0},
		{SymbolConstant, regexp.MustCompile(`(?m)^\s*const\s+(\w+)\s*=`), 1, 0},
		{SymbolVariable, regexp.MustCompile(`(?m)^var\s+(\w+)\s+`), 1, 0},
		{SymbolPackage, regexp.MustCompile(`(?m)^package\s+(\w+)`), 1, 0},
	}

	// Python patterns
	p.patterns["python"] = []*symbolPattern{
		{SymbolFunction, regexp.MustCompile(`(?m)^def\s+(\w+)\s*(\([^)]*\))`), 1, 2},
		{SymbolClass, regexp.MustCompile(`(?m)^class\s+(\w+)(?:\([^)]*\))?:`), 1, 0},
		{SymbolMethod, regexp.MustCompile(`(?m)^\s+def\s+(\w+)\s*\(self[^)]*\)`), 1, 0},
		{SymbolVariable, regexp.MustCompile(`(?m)^(\w+)\s*=\s*[^=]`), 1, 0},
	}

	// JavaScript/TypeScript patterns
	jsPatterns := []*symbolPattern{
		{SymbolFunction, regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*(\([^)]*\))`), 1, 2},
		{SymbolFunction, regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[^=]+)\s*=>`), 1, 0},
		{SymbolClass, regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`), 1, 0},
		{SymbolInterface, regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`), 1, 0},
		{SymbolType, regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*=`), 1, 0},
		{SymbolConstant, regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*=`), 1, 0},
		{SymbolVariable, regexp.MustCompile(`(?m)^(?:export\s+)?let\s+(\w+)`), 1, 0},
	}
	p.patterns["javascript"] = jsPatterns
	p.patterns["typescript"] = jsPatterns
	p.patterns["javascriptreact"] = jsPatterns
	p.patterns["typescriptreact"] = jsPatterns

	// Java patterns
	p.patterns["java"] = []*symbolPattern{
		{SymbolClass, regexp.MustCompile(`(?m)^(?:public\s+)?(?:abstract\s+)?class\s+(\w+)`), 1, 0},
		{SymbolInterface, regexp.MustCompile(`(?m)^(?:public\s+)?interface\s+(\w+)`), 1, 0},
		{SymbolMethod, regexp.MustCompile(`(?m)^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:\w+(?:<[^>]+>)?)\s+(\w+)\s*(\([^)]*\))`), 1, 2},
		{SymbolField, regexp.MustCompile(`(?m)^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:final\s+)?(\w+)\s+(\w+)\s*[;=]`), 2, 0},
		{SymbolEnum, regexp.MustCompile(`(?m)^(?:public\s+)?enum\s+(\w+)`), 1, 0},
	}

	// Rust patterns
	p.patterns["rust"] = []*symbolPattern{
		{SymbolFunction, regexp.MustCompile(`(?m)^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)\s*(<[^>]+>)?\s*(\([^)]*\))`), 1, 3},
		{SymbolStruct, regexp.MustCompile(`(?m)^(?:pub\s+)?struct\s+(\w+)`), 1, 0},
		{SymbolEnum, regexp.MustCompile(`(?m)^(?:pub\s+)?enum\s+(\w+)`), 1, 0},
		{SymbolType, regexp.MustCompile(`(?m)^(?:pub\s+)?type\s+(\w+)\s*=`), 1, 0},
		{SymbolConstant, regexp.MustCompile(`(?m)^(?:pub\s+)?const\s+(\w+):`), 1, 0},
		{SymbolModule, regexp.MustCompile(`(?m)^(?:pub\s+)?mod\s+(\w+)`), 1, 0},
	}

	// C/C++ patterns
	cPatterns := []*symbolPattern{
		{SymbolFunction, regexp.MustCompile(`(?m)^(?:\w+\s+)*(\w+)\s*\([^)]*\)\s*\{`), 1, 0},
		{SymbolStruct, regexp.MustCompile(`(?m)^(?:typedef\s+)?struct\s+(\w+)`), 1, 0},
		{SymbolEnum, regexp.MustCompile(`(?m)^(?:typedef\s+)?enum\s+(\w+)`), 1, 0},
		{SymbolType, regexp.MustCompile(`(?m)^typedef\s+.+\s+(\w+)\s*;`), 1, 0},
	}
	p.patterns["c"] = cPatterns
	p.patterns["cpp"] = append(cPatterns,
		&symbolPattern{SymbolClass, regexp.MustCompile(`(?m)^class\s+(\w+)`), 1, 0},
	)

	// Ruby patterns
	p.patterns["ruby"] = []*symbolPattern{
		{SymbolClass, regexp.MustCompile(`(?m)^class\s+(\w+)`), 1, 0},
		{SymbolModule, regexp.MustCompile(`(?m)^module\s+(\w+)`), 1, 0},
		{SymbolMethod, regexp.MustCompile(`(?m)^\s*def\s+(\w+)`), 1, 0},
		{SymbolConstant, regexp.MustCompile(`(?m)^\s*([A-Z][A-Z_0-9]*)\s*=`), 1, 0},
	}

	// PHP patterns
	p.patterns["php"] = []*symbolPattern{
		{SymbolClass, regexp.MustCompile(`(?m)^(?:abstract\s+)?class\s+(\w+)`), 1, 0},
		{SymbolInterface, regexp.MustCompile(`(?m)^interface\s+(\w+)`), 1, 0},
		{SymbolFunction, regexp.MustCompile(`(?m)^function\s+(\w+)\s*(\([^)]*\))`), 1, 2},
		{SymbolMethod, regexp.MustCompile(`(?m)^\s+(?:public|private|protected)?\s*(?:static\s+)?function\s+(\w+)\s*(\([^)]*\))`), 1, 2},
	}
}

// Parse extracts symbols from source code.
func (p *Parser) Parse(path, content, language string) []Symbol {
	patterns := p.patterns[language]
	if len(patterns) == 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	var symbols []Symbol

	for _, pattern := range patterns {
		matches := pattern.pattern.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			if len(match) < 4 {
				continue
			}

			// Get name
			nameStart, nameEnd := match[pattern.nameGroup*2], match[pattern.nameGroup*2+1]
			if nameStart < 0 || nameEnd < 0 {
				continue
			}
			name := content[nameStart:nameEnd]

			// Get signature if available
			var signature string
			if pattern.signatureGroup > 0 && len(match) > pattern.signatureGroup*2+1 {
				sigStart, sigEnd := match[pattern.signatureGroup*2], match[pattern.signatureGroup*2+1]
				if sigStart >= 0 && sigEnd >= 0 {
					signature = content[sigStart:sigEnd]
				}
			}

			// Find line number
			lineNum := 1
			for i := 0; i < nameStart && i < len(content); i++ {
				if content[i] == '\n' {
					lineNum++
				}
			}

			// Find column
			lineStart := nameStart
			for lineStart > 0 && content[lineStart-1] != '\n' {
				lineStart--
			}
			column := nameStart - lineStart + 1

			// Find end line (look for closing brace or end of block)
			endLine := findEndLine(lines, lineNum, pattern.kind)

			// Get documentation (preceding comment)
			doc := extractDocumentation(lines, lineNum)

			sym := Symbol{
				Name:          name,
				Kind:          pattern.kind,
				Path:          path,
				Line:          lineNum,
				Column:        column,
				EndLine:       endLine,
				Signature:     name + signature,
				Documentation: doc,
			}

			symbols = append(symbols, sym)
		}
	}

	return symbols
}

// findEndLine finds the end line of a symbol definition.
func findEndLine(lines []string, startLine int, kind SymbolKind) int {
	// For simple types, end on same line
	switch kind {
	case SymbolConstant, SymbolVariable, SymbolType, SymbolPackage, SymbolModule:
		return startLine
	}

	// For complex types, look for matching braces
	if startLine > len(lines) {
		return startLine
	}

	braceCount := 0
	started := false

	for i := startLine - 1; i < len(lines); i++ {
		line := lines[i]
		for _, c := range line {
			if c == '{' {
				braceCount++
				started = true
			} else if c == '}' {
				braceCount--
				if started && braceCount == 0 {
					return i + 1
				}
			}
		}
	}

	// If no braces found, look for next symbol or blank line
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || (len(line) > 0 && line[0] != ' ' && line[0] != '\t') {
			return i
		}
	}

	return startLine
}

// extractDocumentation extracts doc comments preceding a symbol.
func extractDocumentation(lines []string, symbolLine int) string {
	var docLines []string

	// Look backwards for comments
	for i := symbolLine - 2; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "//") {
			// Single-line comment
			doc := strings.TrimPrefix(line, "//")
			doc = strings.TrimSpace(doc)
			docLines = append([]string{doc}, docLines...)
		} else if strings.HasSuffix(line, "*/") {
			// Block comment - find start
			for j := i; j >= 0; j-- {
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "/*") {
					// Extract block comment
					blockLines := lines[j : i+1]
					for _, bl := range blockLines {
						bl = strings.TrimSpace(bl)
						bl = strings.TrimPrefix(bl, "/*")
						bl = strings.TrimSuffix(bl, "*/")
						bl = strings.TrimPrefix(bl, "*")
						bl = strings.TrimSpace(bl)
						if bl != "" {
							docLines = append(docLines, bl)
						}
					}
					break
				}
			}
			break
		} else if strings.HasPrefix(line, "#") {
			// Python comment
			doc := strings.TrimPrefix(line, "#")
			doc = strings.TrimSpace(doc)
			docLines = append([]string{doc}, docLines...)
		} else if line == "" {
			// Blank line - stop if we have docs, continue if we don't
			if len(docLines) > 0 {
				break
			}
		} else {
			// Non-comment line
			break
		}
	}

	return strings.Join(docLines, " ")
}

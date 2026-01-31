package index

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// EdgeType represents the type of dependency relationship.
type EdgeType string

const (
	// EdgeCalls represents a function call relationship.
	EdgeCalls EdgeType = "calls"
	// EdgeImports represents an import relationship.
	EdgeImports EdgeType = "imports"
	// EdgeImplements represents a type implementing an interface.
	EdgeImplements EdgeType = "implements"
	// EdgeUses represents a type/variable usage relationship.
	EdgeUses EdgeType = "uses"
	// EdgeEmbeds represents a struct embedding relationship.
	EdgeEmbeds EdgeType = "embeds"
)

// Node represents a symbol in the dependency graph.
type Node struct {
	ID         string `json:"id"`          // Unique identifier: "package.Symbol" or "file:line"
	Name       string `json:"name"`        // Symbol name
	Kind       string `json:"kind"`        // function, method, type, interface, const, var
	FilePath   string `json:"file_path"`   // Relative file path
	Package    string `json:"package"`     // Package name
	StartLine  int    `json:"start_line"`  // Line number
	EndLine    int    `json:"end_line"`    // End line number
	Signature  string `json:"signature"`   // Function/type signature
	DocComment string `json:"doc_comment"` // Documentation
}

// Edge represents a directed dependency relationship.
type Edge struct {
	Source   string   `json:"source"`    // Source node ID
	Target   string   `json:"target"`    // Target node ID
	EdgeType EdgeType `json:"edge_type"` // Type of relationship
	FilePath string   `json:"file_path"` // File where the relationship exists
	Line     int      `json:"line"`      // Line number of the reference
}

// DependencyGraph is a directed acyclic graph tracking code dependencies.
type DependencyGraph struct {
	mu          sync.RWMutex
	nodes       map[string]*Node    // nodeID -> Node
	outEdges    map[string][]Edge   // nodeID -> outgoing edges (what this node depends on)
	inEdges     map[string][]Edge   // nodeID -> incoming edges (what depends on this node)
	fileNodes   map[string][]string // filePath -> nodeIDs in that file
	pkgNodes    map[string][]string // package -> nodeIDs in that package
	dirty       bool                // whether graph has unsaved changes
	storagePath string              // path to persist the graph
}

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph(storagePath string) *DependencyGraph {
	return &DependencyGraph{
		nodes:       make(map[string]*Node),
		outEdges:    make(map[string][]Edge),
		inEdges:     make(map[string][]Edge),
		fileNodes:   make(map[string][]string),
		pkgNodes:    make(map[string][]string),
		storagePath: storagePath,
	}
}

// AddNode adds or updates a node in the graph.
func (g *DependencyGraph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove from old file index if updating
	if existing, ok := g.nodes[node.ID]; ok && existing.FilePath != node.FilePath {
		g.removeFromFileIndex(existing.FilePath, node.ID)
	}

	g.nodes[node.ID] = node
	g.addToFileIndex(node.FilePath, node.ID)
	g.addToPkgIndex(node.Package, node.ID)
	g.dirty = true
}

// AddEdge adds a directed edge from source to target.
func (g *DependencyGraph) AddEdge(edge Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Avoid duplicate edges
	for _, e := range g.outEdges[edge.Source] {
		if e.Target == edge.Target && e.EdgeType == edge.EdgeType && e.Line == edge.Line {
			return
		}
	}

	g.outEdges[edge.Source] = append(g.outEdges[edge.Source], edge)
	g.inEdges[edge.Target] = append(g.inEdges[edge.Target], edge)
	g.dirty = true
}

// GetNode returns a node by ID.
func (g *DependencyGraph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[id]
	return node, ok
}

// GetDependencies returns all nodes that the given node depends on (outgoing edges).
func (g *DependencyGraph) GetDependencies(nodeID string) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.outEdges[nodeID]
}

// GetDependents returns all nodes that depend on the given node (incoming edges).
func (g *DependencyGraph) GetDependents(nodeID string) []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.inEdges[nodeID]
}

// GetNodesByFile returns all nodes in a given file.
func (g *DependencyGraph) GetNodesByFile(filePath string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeIDs := g.fileNodes[filePath]
	nodes := make([]*Node, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if node, ok := g.nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodesByPackage returns all nodes in a given package.
func (g *DependencyGraph) GetNodesByPackage(pkg string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeIDs := g.pkgNodes[pkg]
	nodes := make([]*Node, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if node, ok := g.nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// FindNodeByName finds nodes matching the given name (case-sensitive).
func (g *DependencyGraph) FindNodeByName(name string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var matches []*Node
	for _, node := range g.nodes {
		if node.Name == name {
			matches = append(matches, node)
		}
	}
	return matches
}

// GetImpact calculates transitive impact of changes to a file.
// Returns all nodes that could be affected by changes to the given file.
func (g *DependencyGraph) GetImpact(filePath string) *ImpactResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := &ImpactResult{
		SourceFile:     filePath,
		DirectImpact:   make(map[string][]*Node),
		IndirectImpact: make(map[string][]*Node),
	}

	// Get all nodes in the changed file
	changedNodes := g.fileNodes[filePath]
	visited := make(map[string]bool)

	// First pass: direct dependents
	for _, nodeID := range changedNodes {
		node := g.nodes[nodeID]
		if node == nil {
			continue
		}

		for _, edge := range g.inEdges[nodeID] {
			if !visited[edge.Source] {
				visited[edge.Source] = true
				if sourceNode, ok := g.nodes[edge.Source]; ok {
					result.DirectImpact[sourceNode.FilePath] = append(
						result.DirectImpact[sourceNode.FilePath], sourceNode)
				}
			}
		}
	}

	// Second pass: transitive dependents (BFS)
	queue := make([]string, 0)
	for nodeID := range visited {
		queue = append(queue, nodeID)
	}

	depth := 0
	maxDepth := 5 // Limit traversal depth

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			nodeID := queue[0]
			queue = queue[1:]

			for _, edge := range g.inEdges[nodeID] {
				if !visited[edge.Source] {
					visited[edge.Source] = true
					if sourceNode, ok := g.nodes[edge.Source]; ok {
						result.IndirectImpact[sourceNode.FilePath] = append(
							result.IndirectImpact[sourceNode.FilePath], sourceNode)
						queue = append(queue, edge.Source)
					}
				}
			}
		}
		depth++
	}

	return result
}

// ImpactResult contains the results of an impact analysis.
type ImpactResult struct {
	SourceFile     string             `json:"source_file"`
	DirectImpact   map[string][]*Node `json:"direct_impact"`   // file -> nodes directly depending on source
	IndirectImpact map[string][]*Node `json:"indirect_impact"` // file -> nodes transitively depending on source
}

// TotalImpactedFiles returns the total number of impacted files.
func (r *ImpactResult) TotalImpactedFiles() int {
	files := make(map[string]bool)
	for f := range r.DirectImpact {
		files[f] = true
	}
	for f := range r.IndirectImpact {
		files[f] = true
	}
	return len(files)
}

// TotalImpactedNodes returns the total number of impacted nodes.
func (r *ImpactResult) TotalImpactedNodes() int {
	count := 0
	for _, nodes := range r.DirectImpact {
		count += len(nodes)
	}
	for _, nodes := range r.IndirectImpact {
		count += len(nodes)
	}
	return count
}

// FormatImpact formats the impact result as markdown.
func (r *ImpactResult) FormatImpact() string {
	var sb []byte
	sb = append(sb, fmt.Sprintf("# Impact Analysis: %s\n\n", r.SourceFile)...)

	sb = append(sb, fmt.Sprintf("**Summary**: %d files, %d symbols affected\n\n",
		r.TotalImpactedFiles(), r.TotalImpactedNodes())...)

	if len(r.DirectImpact) > 0 {
		sb = append(sb, "## Direct Dependents\n\n"...)
		for file, nodes := range r.DirectImpact {
			sb = append(sb, fmt.Sprintf("### %s\n", file)...)
			for _, node := range nodes {
				sb = append(sb, fmt.Sprintf("- `%s` (%s) L%d\n", node.Name, node.Kind, node.StartLine)...)
			}
			sb = append(sb, '\n')
		}
	}

	if len(r.IndirectImpact) > 0 {
		sb = append(sb, "## Transitive Dependents\n\n"...)
		for file, nodes := range r.IndirectImpact {
			sb = append(sb, fmt.Sprintf("### %s\n", file)...)
			for _, node := range nodes {
				sb = append(sb, fmt.Sprintf("- `%s` (%s) L%d\n", node.Name, node.Kind, node.StartLine)...)
			}
			sb = append(sb, '\n')
		}
	}

	return string(sb)
}

// RemoveFile removes all nodes and edges associated with a file.
func (g *DependencyGraph) RemoveFile(filePath string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nodeIDs := g.fileNodes[filePath]
	for _, nodeID := range nodeIDs {
		// Remove edges from this node
		delete(g.outEdges, nodeID)

		// Remove edges to this node
		for source, edges := range g.inEdges {
			filtered := make([]Edge, 0)
			for _, e := range edges {
				if e.Target != nodeID {
					filtered = append(filtered, e)
				}
			}
			if len(filtered) > 0 {
				g.inEdges[source] = filtered
			} else {
				delete(g.inEdges, source)
			}
		}

		// Remove from outEdges of other nodes
		for target, edges := range g.outEdges {
			filtered := make([]Edge, 0)
			for _, e := range edges {
				if e.Target != nodeID {
					filtered = append(filtered, e)
				}
			}
			if len(filtered) > 0 {
				g.outEdges[target] = filtered
			} else {
				delete(g.outEdges, target)
			}
		}

		// Remove from package index
		if node, ok := g.nodes[nodeID]; ok {
			g.removeFromPkgIndex(node.Package, nodeID)
		}

		// Remove node
		delete(g.nodes, nodeID)
	}

	delete(g.fileNodes, filePath)
	g.dirty = true
}

// Stats returns statistics about the graph.
func (g *DependencyGraph) Stats() DAGStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edgeCount := 0
	for _, edges := range g.outEdges {
		edgeCount += len(edges)
	}

	edgeTypeCounts := make(map[EdgeType]int)
	for _, edges := range g.outEdges {
		for _, e := range edges {
			edgeTypeCounts[e.EdgeType]++
		}
	}

	return DAGStats{
		NodeCount:      len(g.nodes),
		EdgeCount:      edgeCount,
		FileCount:      len(g.fileNodes),
		PackageCount:   len(g.pkgNodes),
		EdgeTypeCounts: edgeTypeCounts,
	}
}

// DAGStats contains statistics about the dependency graph.
type DAGStats struct {
	NodeCount      int              `json:"node_count"`
	EdgeCount      int              `json:"edge_count"`
	FileCount      int              `json:"file_count"`
	PackageCount   int              `json:"package_count"`
	EdgeTypeCounts map[EdgeType]int `json:"edge_type_counts"`
}

// Save persists the graph to disk.
func (g *DependencyGraph) Save() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.dirty {
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(g.storagePath), 0755); err != nil {
		return fmt.Errorf("create dag directory: %w", err)
	}

	// Create serializable structure
	data := dagSerializable{
		Nodes: make([]*Node, 0, len(g.nodes)),
		Edges: make([]Edge, 0),
	}

	for _, node := range g.nodes {
		data.Nodes = append(data.Nodes, node)
	}
	for _, edges := range g.outEdges {
		data.Edges = append(data.Edges, edges...)
	}

	// Sort for deterministic output
	sort.Slice(data.Nodes, func(i, j int) bool {
		return data.Nodes[i].ID < data.Nodes[j].ID
	})
	sort.Slice(data.Edges, func(i, j int) bool {
		if data.Edges[i].Source != data.Edges[j].Source {
			return data.Edges[i].Source < data.Edges[j].Source
		}
		return data.Edges[i].Target < data.Edges[j].Target
	})

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dag: %w", err)
	}

	if err := os.WriteFile(g.storagePath, jsonData, 0644); err != nil {
		return fmt.Errorf("write dag: %w", err)
	}

	g.dirty = false
	return nil
}

// Load loads the graph from disk.
func (g *DependencyGraph) Load() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	jsonData, err := os.ReadFile(g.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing graph
		}
		return fmt.Errorf("read dag: %w", err)
	}

	var data dagSerializable
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("unmarshal dag: %w", err)
	}

	// Clear existing data
	g.nodes = make(map[string]*Node)
	g.outEdges = make(map[string][]Edge)
	g.inEdges = make(map[string][]Edge)
	g.fileNodes = make(map[string][]string)
	g.pkgNodes = make(map[string][]string)

	// Load nodes
	for _, node := range data.Nodes {
		g.nodes[node.ID] = node
		g.addToFileIndex(node.FilePath, node.ID)
		g.addToPkgIndex(node.Package, node.ID)
	}

	// Load edges
	for _, edge := range data.Edges {
		g.outEdges[edge.Source] = append(g.outEdges[edge.Source], edge)
		g.inEdges[edge.Target] = append(g.inEdges[edge.Target], edge)
	}

	g.dirty = false
	return nil
}

// Clear removes all nodes and edges from the graph.
func (g *DependencyGraph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[string]*Node)
	g.outEdges = make(map[string][]Edge)
	g.inEdges = make(map[string][]Edge)
	g.fileNodes = make(map[string][]string)
	g.pkgNodes = make(map[string][]string)
	g.dirty = true
}

// dagSerializable is the JSON-serializable form of the graph.
type dagSerializable struct {
	Nodes []*Node `json:"nodes"`
	Edges []Edge  `json:"edges"`
}

// Helper methods for index management

func (g *DependencyGraph) addToFileIndex(filePath, nodeID string) {
	if filePath == "" {
		return
	}
	for _, id := range g.fileNodes[filePath] {
		if id == nodeID {
			return
		}
	}
	g.fileNodes[filePath] = append(g.fileNodes[filePath], nodeID)
}

func (g *DependencyGraph) removeFromFileIndex(filePath, nodeID string) {
	if filePath == "" {
		return
	}
	nodes := g.fileNodes[filePath]
	for i, id := range nodes {
		if id == nodeID {
			g.fileNodes[filePath] = append(nodes[:i], nodes[i+1:]...)
			return
		}
	}
}

func (g *DependencyGraph) addToPkgIndex(pkg, nodeID string) {
	if pkg == "" {
		return
	}
	for _, id := range g.pkgNodes[pkg] {
		if id == nodeID {
			return
		}
	}
	g.pkgNodes[pkg] = append(g.pkgNodes[pkg], nodeID)
}

func (g *DependencyGraph) removeFromPkgIndex(pkg, nodeID string) {
	if pkg == "" {
		return
	}
	nodes := g.pkgNodes[pkg]
	for i, id := range nodes {
		if id == nodeID {
			g.pkgNodes[pkg] = append(nodes[:i], nodes[i+1:]...)
			return
		}
	}
}

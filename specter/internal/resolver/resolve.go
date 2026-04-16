// Package resolver implements spec-resolve: dependency graph builder.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-resolve
package resolver

import (
	"fmt"

	"github.com/Hanalyx/specter/internal/schema"
	"github.com/Masterminds/semver/v3"
)

// Diagnostic represents an issue found during resolution.
type Diagnostic struct {
	Kind             string   `json:"kind"`
	Severity         string   `json:"severity"`
	Message          string   `json:"message"`
	SpecID           string   `json:"spec_id,omitempty"`
	MissingDep       string   `json:"missing_dep,omitempty"`
	RequiredRange    string   `json:"required_range,omitempty"`
	ActualVersion    string   `json:"actual_version,omitempty"`
	CyclePath        []string `json:"cycle_path,omitempty"`
	Files            []string `json:"files,omitempty"`
	Suggestions      []string `json:"suggestions,omitempty"`        // C-10: closest existing spec IDs
	SuggestedFixPath string   `json:"suggested_fix_path,omitempty"` // C-10: likely file path to create
}

// SpecNode holds a parsed spec and its file path.
type SpecNode struct {
	Spec schema.SpecAST `json:"spec"`
	File string         `json:"file"`
}

// SpecEdge represents a dependency between two specs.
type SpecEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	VersionRange string `json:"version_range,omitempty"`
	Relationship string `json:"relationship"`
}

// SpecGraph is the resolved dependency graph.
type SpecGraph struct {
	Nodes            map[string]*SpecNode `json:"nodes"`
	Edges            []SpecEdge           `json:"edges"`
	TopologicalOrder []string             `json:"topological_order"`
	Diagnostics      []Diagnostic         `json:"diagnostics"`
}

// SpecInput is a parsed spec with its file path.
type SpecInput struct {
	Spec schema.SpecAST
	File string
}

// ResolveSpecs builds a dependency graph from parsed specs.
//
// C-03: Detects ALL circular dependencies.
// C-04: Reports full cycle paths.
// C-05: Detects dangling references.
// C-06: Validates semver ranges.
// C-07: Produces typed SpecGraph.
// C-08: Pure function.
func ResolveSpecs(inputs []SpecInput) *SpecGraph {
	graph := &SpecGraph{
		Nodes: make(map[string]*SpecNode),
	}

	// Step 1: Register nodes, detect duplicates (AC-07)
	for i := range inputs {
		id := inputs[i].Spec.ID
		if existing, ok := graph.Nodes[id]; ok {
			graph.Diagnostics = append(graph.Diagnostics, Diagnostic{
				Kind:     "duplicate_id",
				Severity: "error",
				Message:  fmt.Sprintf("Duplicate spec ID %q found in %s and %s", id, existing.File, inputs[i].File),
				SpecID:   id,
				Files:    []string{existing.File, inputs[i].File},
			})
			continue
		}
		graph.Nodes[id] = &SpecNode{Spec: inputs[i].Spec, File: inputs[i].File}
	}

	// Collect all known spec IDs for suggestion matching (C-10)
	allIDs := make([]string, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		allIDs = append(allIDs, id)
	}

	// Step 2: Build edges, detect dangling refs and version mismatches
	adjacency := make(map[string][]string) // from -> [to]
	for id, node := range graph.Nodes {
		for _, dep := range node.Spec.DependsOn {
			targetID := dep.SpecID

			// C-05: Dangling reference (AC-03)
			target, exists := graph.Nodes[targetID]
			if !exists {
				suggestions := closestMatches(targetID, allIDs, 3)
				d := Diagnostic{
					Kind:             "dangling_reference",
					Severity:         "error",
					Message:          fmt.Sprintf("Spec %q depends on %q which does not exist", id, targetID),
					SpecID:           id,
					MissingDep:       targetID,
					Suggestions:      suggestions,
					SuggestedFixPath: inferSpecFilePath(targetID),
				}
				graph.Diagnostics = append(graph.Diagnostics, d)
				continue
			}

			// C-06: Version mismatch (AC-04)
			if dep.VersionRange != "" {
				constraint, err := semver.NewConstraint(dep.VersionRange)
				if err != nil {
					graph.Diagnostics = append(graph.Diagnostics, Diagnostic{
						Kind:          "version_mismatch",
						Severity:      "error",
						Message:       fmt.Sprintf("Spec %q has invalid semver range %q for dependency %q", id, dep.VersionRange, targetID),
						SpecID:        id,
						RequiredRange: dep.VersionRange,
						ActualVersion: target.Spec.Version,
					})
				} else {
					ver, err := semver.NewVersion(target.Spec.Version)
					if err == nil && !constraint.Check(ver) {
						graph.Diagnostics = append(graph.Diagnostics, Diagnostic{
							Kind:          "version_mismatch",
							Severity:      "error",
							Message:       fmt.Sprintf("Spec %q requires %q@%s but found version %s", id, targetID, dep.VersionRange, target.Spec.Version),
							SpecID:        id,
							RequiredRange: dep.VersionRange,
							ActualVersion: target.Spec.Version,
						})
					}
				}
			}

			rel := dep.Relationship
			if rel == "" {
				rel = "requires"
			}
			graph.Edges = append(graph.Edges, SpecEdge{
				From:         id,
				To:           targetID,
				VersionRange: dep.VersionRange,
				Relationship: rel,
			})
			adjacency[id] = append(adjacency[id], targetID)
		}
	}

	// Step 3: Detect cycles (C-03, C-04)
	cycles := findCycles(graph.Nodes, adjacency)
	for _, cycle := range cycles {
		cyclePath := append(cycle, cycle[0])
		graph.Diagnostics = append(graph.Diagnostics, Diagnostic{
			Kind:      "circular_dependency",
			Severity:  "error",
			Message:   fmt.Sprintf("Circular dependency detected: %s", formatCyclePath(cyclePath)),
			CyclePath: cyclePath,
		})
	}

	// Step 4: Topological sort (empty if cycles)
	if len(cycles) == 0 {
		graph.TopologicalOrder = topologicalSort(graph.Nodes, adjacency)
	}

	return graph
}

// findCycles uses DFS to find all cycles in the graph.
func findCycles(nodes map[string]*SpecNode, adjacency map[string][]string) [][]string {
	const (
		white = 0 // unvisited
		grey  = 1 // in current path
		black = 2 // fully processed
	)

	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles [][]string
	cycleNodes := make(map[string]bool) // track which nodes are already in a reported cycle

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = grey
		for _, neighbor := range adjacency[node] {
			if _, exists := nodes[neighbor]; !exists {
				continue // skip dangling refs
			}
			if color[neighbor] == grey {
				// Found a cycle — reconstruct path
				cycle := []string{neighbor}
				curr := node
				for curr != neighbor {
					cycle = append([]string{curr}, cycle...)
					curr = parent[curr]
				}
				// Only add if we haven't reported this cycle already
				key := cycleKey(cycle)
				if !cycleNodes[key] {
					cycleNodes[key] = true
					cycles = append(cycles, cycle)
				}
			} else if color[neighbor] == white {
				parent[neighbor] = node
				dfs(neighbor)
			}
		}
		color[node] = black
	}

	for id := range nodes {
		if color[id] == white {
			dfs(id)
		}
	}

	return cycles
}

func cycleKey(cycle []string) string {
	// Normalize cycle to start with the smallest element
	min := 0
	for i, v := range cycle {
		if v < cycle[min] {
			min = i
		}
	}
	rotated := append(cycle[min:], cycle[:min]...)
	result := ""
	for _, v := range rotated {
		result += v + ","
	}
	return result
}

// topologicalSort returns nodes in dependency order (dependencies first).
func topologicalSort(nodes map[string]*SpecNode, adjacency map[string][]string) []string {
	inDegree := make(map[string]int)
	for id := range nodes {
		inDegree[id] = 0
	}
	for _, neighbors := range adjacency {
		for _, n := range neighbors {
			inDegree[n]++
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range adjacency[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Reverse: dependencies first (Kahn's produces dependents first)
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	return order
}

func formatCyclePath(path []string) string {
	result := ""
	for i, p := range path {
		if i > 0 {
			result += " -> "
		}
		result += p
	}
	return result
}

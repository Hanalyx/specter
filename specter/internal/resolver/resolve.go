// Package resolver implements spec-resolve: dependency graph builder.
//
// Pure functions. No CLI deps, no I/O.
//
// @spec spec-resolve
package resolver

import (
	"fmt"
	"slices"
	"sort"

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

	// Collect all known spec IDs, sorted, for suggestion matching (C-10) and
	// for the diagnostics loop below (C-07). graph.Nodes is a map, and the
	// order of both the suggestion candidates and the emitted diagnostics
	// must not depend on map iteration (AC-16, AC-18).
	allIDs := make([]string, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		allIDs = append(allIDs, id)
	}
	sort.Strings(allIDs)

	// Step 2: Build edges, detect dangling refs and version mismatches, in
	// spec id order so the diagnostics come out the same on every run.
	adjacency := make(map[string][]string) // from -> [to]
	for _, id := range allIDs {
		node := graph.Nodes[id]
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

// maxCycles caps simple-cycle enumeration (C-03). Dense strongly connected
// components can contain exponentially many simple cycles; spec graphs are
// small in practice, but the cap bounds pathological inputs. The cap is
// deterministic because enumeration order is deterministic.
const maxCycles = 1000

// findCycles enumerates ALL simple cycles in the graph (C-03), including
// overlapping cycles that share an edge or a node, and disjoint cycles
// (AC-15). It decomposes the graph into strongly connected components
// (Tarjan) and enumerates simple cycles per SCC (Johnson's algorithm).
//
// Output is deterministic regardless of map iteration order: vertices are
// processed in lexicographic order, so each cycle starts at its
// lexicographically smallest node, and the resulting list is sorted
// lexicographically. Each returned cycle lists its nodes once, without
// repeating the start node; the caller closes the loop.
func findCycles(nodes map[string]*SpecNode, adjacency map[string][]string) [][]string {
	// Index nodes in sorted order so vertex i < vertex j implies
	// ids[i] < ids[j] lexicographically.
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	index := make(map[string]int, len(ids))
	for i, id := range ids {
		index[id] = i
	}

	n := len(ids)
	adj := make([][]int, n)
	var cycles [][]string
	for i, id := range ids {
		seen := make(map[int]bool)
		for _, neighbor := range adjacency[id] {
			j, exists := index[neighbor]
			if !exists || seen[j] {
				continue // dangling ref or duplicate edge
			}
			seen[j] = true
			if j == i {
				// Self-loop: a one-node cycle. Johnson's algorithm
				// below enumerates cycles of length >= 2 only.
				cycles = append(cycles, []string{id})
				continue
			}
			adj[i] = append(adj[i], j)
		}
		sort.Ints(adj[i])
	}

	// Johnson's algorithm state.
	blocked := make([]bool, n)
	blockedBy := make([]map[int]bool, n)
	stack := make([]int, 0, n)

	var unblock func(v int)
	unblock = func(v int) {
		blocked[v] = false
		for w := range blockedBy[v] {
			delete(blockedBy[v], w)
			if blocked[w] {
				unblock(w)
			}
		}
	}

	var circuit func(v, start int, scc map[int]bool) bool
	circuit = func(v, start int, scc map[int]bool) bool {
		found := false
		stack = append(stack, v)
		blocked[v] = true
		for _, w := range adj[v] {
			if !scc[w] {
				continue
			}
			if w == start {
				if len(cycles) < maxCycles {
					cycle := make([]string, len(stack))
					for i, u := range stack {
						cycle[i] = ids[u]
					}
					cycles = append(cycles, cycle)
				}
				found = true
			} else if !blocked[w] {
				if circuit(w, start, scc) {
					found = true
				}
			}
		}
		if found {
			unblock(v)
		} else {
			for _, w := range adj[v] {
				if scc[w] {
					blockedBy[w][v] = true
				}
			}
		}
		stack = stack[:len(stack)-1]
		return found
	}

	for s := 0; s < n && len(cycles) < maxCycles; {
		// Find the non-trivial SCC containing the smallest vertex in
		// the subgraph induced by vertices >= s.
		least := -1
		var leastSCC map[int]bool
		for _, scc := range tarjanSCC(adj, s, n) {
			if len(scc) < 2 {
				continue
			}
			m := scc[0]
			for _, v := range scc[1:] {
				if v < m {
					m = v
				}
			}
			if least == -1 || m < least {
				least = m
				leastSCC = make(map[int]bool, len(scc))
				for _, v := range scc {
					leastSCC[v] = true
				}
			}
		}
		if least == -1 {
			break
		}
		s = least
		for v := range leastSCC {
			blocked[v] = false
			blockedBy[v] = make(map[int]bool)
		}
		circuit(s, s, leastSCC)
		s++
	}

	// Canonical order: cycles already start at their smallest node (the
	// Johnson start vertex is the minimum of its SCC's remaining
	// subgraph); sort the list for stable diagnostics.
	slices.SortFunc(cycles, slices.Compare)
	return cycles
}

// tarjanSCC returns the strongly connected components of the subgraph
// induced by vertices in [minV, n), ignoring edges to vertices below minV.
func tarjanSCC(adj [][]int, minV, n int) [][]int {
	const unvisited = -1
	order := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range order {
		order[i] = unvisited
	}
	var stack []int
	counter := 0
	var sccs [][]int

	var strongConnect func(v int)
	strongConnect = func(v int) {
		order[v] = counter
		low[v] = counter
		counter++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if w < minV {
				continue
			}
			if order[w] == unvisited {
				strongConnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && order[w] < low[v] {
				low[v] = order[w]
			}
		}
		if low[v] == order[v] {
			var scc []int
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for v := minV; v < n; v++ {
		if order[v] == unvisited {
			strongConnect(v)
		}
	}
	return sccs
}

// topologicalSort returns nodes in dependency order (dependencies first).
//
// The order is the same on every run over the same input (C-07, AC-17).
// Kahn's algorithm runs in the output direction: a node is ready once every
// spec it depends on has been emitted, and among the ready nodes the
// smallest spec id goes first. No reversal happens at the end, so the
// tie-break rule holds on the final output rather than on an intermediate
// order.
func topologicalSort(nodes map[string]*SpecNode, adjacency map[string][]string) []string {
	// pending counts how many dependencies of each node are not yet emitted.
	// dependents maps a dependency to the nodes that depend on it.
	pending := make(map[string]int, len(nodes))
	dependents := make(map[string][]string)
	for id := range nodes {
		pending[id] = len(adjacency[id])
		for _, target := range adjacency[id] {
			dependents[target] = append(dependents[target], id)
		}
	}

	var ready []string
	for id, n := range pending {
		if n == 0 {
			ready = append(ready, id)
		}
	}

	order := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		sort.Strings(ready)
		node := ready[0]
		ready = ready[1:]
		order = append(order, node)

		for _, dependent := range dependents[node] {
			pending[dependent]--
			if pending[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
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

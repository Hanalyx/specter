// Reverse traversal of the dependency graph (spec-resolve C-12, C-13).
//
// DependentsOf is the engine for `specter resolve dependents <spec-id>`.
// Given a resolved SpecGraph and a spec id, it returns the set of direct
// dependents — specs whose depends_on list includes the given spec id.
//
// Stays a pure function: no I/O, no subprocess calls. CLI layer is
// responsible for graph construction and output formatting.
//
// @spec spec-resolve
package resolver

import (
	"fmt"
	"sort"
)

// DependentsOf returns the direct dependents of specID (specs whose
// depends_on edge points at it). Sorted lexicographically.
//
// Returns ([], nil) when specID exists in the graph but has no
// dependents — an empty set is a valid result (C-12). Returns nil
// plus an error when specID is not in the graph — that's an operator
// error, not an empty set.
//
// Direct dependents only. A future flag may add `--transitive` to
// walk the dependent tree, but v0.13 ships the one-step query.
func DependentsOf(graph *SpecGraph, specID string) ([]string, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if _, ok := graph.Nodes[specID]; !ok {
		return nil, fmt.Errorf("spec %q not found in graph", specID)
	}
	var out []string
	for _, e := range graph.Edges {
		if e.To == specID {
			out = append(out, e.From)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{} // distinguish "empty result" from "error"
	}
	return out, nil
}

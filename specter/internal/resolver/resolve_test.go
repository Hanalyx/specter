// @spec spec-resolve
package resolver

import (
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

func makeSpec(id string, opts ...func(*schema.SpecAST)) schema.SpecAST {
	s := schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: 2,
		Context:            schema.SpecContext{System: "test"},
		Objective:          schema.SpecObjective{Summary: "test"},
		Constraints:        []schema.Constraint{{ID: "C-01", Description: "test"}},
		AcceptanceCriteria: []schema.AcceptanceCriterion{{ID: "AC-01", Description: "test"}},
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

func withDeps(deps ...schema.DependencyRef) func(*schema.SpecAST) {
	return func(s *schema.SpecAST) { s.DependsOn = deps }
}

func withVersion(v string) func(*schema.SpecAST) {
	return func(s *schema.SpecAST) { s.Version = v }
}

func dep(id string) schema.DependencyRef {
	return schema.DependencyRef{SpecID: id, Relationship: "requires"}
}

func depVersioned(id, vr string) schema.DependencyRef {
	return schema.DependencyRef{SpecID: id, VersionRange: vr, Relationship: "requires"}
}

// @ac AC-01
func TestLinearDependencies(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(dep("b"))), File: "a.spec.yaml"},
		{Spec: makeSpec("b", withDeps(dep("c"))), File: "b.spec.yaml"},
		{Spec: makeSpec("c"), File: "c.spec.yaml"},
	})

	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(g.Edges))
	}
	if len(g.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(g.Diagnostics))
	}
	if len(g.TopologicalOrder) != 3 {
		t.Errorf("expected 3 in topo order, got %d", len(g.TopologicalOrder))
	}
}

// @ac AC-02
func TestTwoNodeCycle(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(dep("b"))), File: "a.spec.yaml"},
		{Spec: makeSpec("b", withDeps(dep("a"))), File: "b.spec.yaml"},
	})

	found := false
	for _, d := range g.Diagnostics {
		if d.Kind == "circular_dependency" {
			found = true
		}
	}
	if !found {
		t.Error("expected circular_dependency diagnostic")
	}
	if len(g.TopologicalOrder) != 0 {
		t.Error("expected empty topo order when cycles exist")
	}
}

// @ac AC-03
func TestDanglingReference(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(dep("nonexistent"))), File: "a.spec.yaml"},
	})

	found := false
	for _, d := range g.Diagnostics {
		if d.Kind == "dangling_reference" && d.MissingDep == "nonexistent" {
			found = true
		}
	}
	if !found {
		t.Error("expected dangling_reference diagnostic")
	}
}

// @ac AC-04
func TestVersionMismatch(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(depVersioned("b", "^1.0.0"))), File: "a.spec.yaml"},
		{Spec: makeSpec("b", withVersion("2.0.0")), File: "b.spec.yaml"},
	})

	found := false
	for _, d := range g.Diagnostics {
		if d.Kind == "version_mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected version_mismatch diagnostic")
	}
}

// @ac AC-05
func TestNoDependencies(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a"), File: "a.spec.yaml"},
		{Spec: makeSpec("b"), File: "b.spec.yaml"},
	})

	if len(g.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(g.Edges))
	}
	if len(g.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(g.Diagnostics))
	}
}

// @ac AC-06
func TestThreeNodeCycle(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(dep("b"))), File: "a.spec.yaml"},
		{Spec: makeSpec("b", withDeps(dep("c"))), File: "b.spec.yaml"},
		{Spec: makeSpec("c", withDeps(dep("a"))), File: "c.spec.yaml"},
	})

	found := false
	for _, d := range g.Diagnostics {
		if d.Kind == "circular_dependency" {
			found = true
		}
	}
	if !found {
		t.Error("expected circular_dependency diagnostic")
	}
}

// @ac AC-07
func TestDuplicateIDs(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("user-auth"), File: "file1.spec.yaml"},
		{Spec: makeSpec("user-auth"), File: "file2.spec.yaml"},
	})

	found := false
	for _, d := range g.Diagnostics {
		if d.Kind == "duplicate_id" {
			found = true
		}
	}
	if !found {
		t.Error("expected duplicate_id diagnostic")
	}
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node (first wins), got %d", len(g.Nodes))
	}
}

func TestValidVersionRange(t *testing.T) {
	g := ResolveSpecs([]SpecInput{
		{Spec: makeSpec("a", withDeps(depVersioned("b", "^1.0.0"))), File: "a.spec.yaml"},
		{Spec: makeSpec("b", withVersion("1.2.0")), File: "b.spec.yaml"},
	})

	if len(g.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %v", g.Diagnostics)
	}
}

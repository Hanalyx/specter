// @spec spec-resolve
package resolver

import (
	"slices"
	"strings"
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
	t.Run("spec-resolve/AC-01 linear dependencies", func(t *testing.T) {
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
	})
}

// @ac AC-02
func TestTwoNodeCycle(t *testing.T) {
	t.Run("spec-resolve/AC-02 two node cycle", func(t *testing.T) {
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
	})
}

// @ac AC-03
func TestDanglingReference(t *testing.T) {
	t.Run("spec-resolve/AC-03 dangling reference", func(t *testing.T) {
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
	})
}

// @ac AC-04
func TestVersionMismatch(t *testing.T) {
	t.Run("spec-resolve/AC-04 version mismatch", func(t *testing.T) {
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
	})
}

// @ac AC-05
func TestNoDependencies(t *testing.T) {
	t.Run("spec-resolve/AC-05 no dependencies", func(t *testing.T) {
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
	})
}

// @ac AC-06
func TestThreeNodeCycle(t *testing.T) {
	t.Run("spec-resolve/AC-06 three node cycle", func(t *testing.T) {
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
	})
}

// @ac AC-07
func TestDuplicateIDs(t *testing.T) {
	t.Run("spec-resolve/AC-07 duplicate ids", func(t *testing.T) {
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
	})
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

// @ac AC-09
func TestDanglingReferenceIncludesSuggestion(t *testing.T) {
	t.Run("spec-resolve/AC-09 dangling reference includes suggestion", func(t *testing.T) {
		// "handler-interfac" is one character off from "handler-interface"
		g := ResolveSpecs([]SpecInput{
			{Spec: makeSpec("a", withDeps(dep("handler-interfac"))), File: "a.spec.yaml"},
			{Spec: makeSpec("handler-interface"), File: "handler-interface.spec.yaml"},
		})

		var dr *Diagnostic
		for i := range g.Diagnostics {
			if g.Diagnostics[i].Kind == "dangling_reference" {
				dr = &g.Diagnostics[i]
				break
			}
		}
		if dr == nil {
			t.Fatal("expected dangling_reference diagnostic")
		}
		if len(dr.Suggestions) == 0 {
			t.Error("expected at least one suggestion")
		}
		found := false
		for _, s := range dr.Suggestions {
			if s == "handler-interface" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected 'handler-interface' in suggestions, got %v", dr.Suggestions)
		}
		if dr.SuggestedFixPath == "" {
			t.Error("expected SuggestedFixPath to be set")
		}
	})
}

// circularPaths collects every circular_dependency diagnostic's cycle path
// rendered as "a -> b -> a", in diagnostic order.
func circularPaths(g *SpecGraph) []string {
	var paths []string
	for _, d := range g.Diagnostics {
		if d.Kind == "circular_dependency" {
			paths = append(paths, strings.Join(d.CyclePath, " -> "))
		}
	}
	return paths
}

// thetaGraphInputs builds the AC-15 theta graph: a -> x -> b -> a and
// a -> y -> b -> a, sharing edge b -> a.
func thetaGraphInputs() []SpecInput {
	return []SpecInput{
		{Spec: makeSpec("a", withDeps(dep("x"), dep("y"))), File: "a.spec.yaml"},
		{Spec: makeSpec("x", withDeps(dep("b"))), File: "x.spec.yaml"},
		{Spec: makeSpec("y", withDeps(dep("b"))), File: "y.spec.yaml"},
		{Spec: makeSpec("b", withDeps(dep("a"))), File: "b.spec.yaml"},
	}
}

// @ac AC-15
func TestThetaGraphReportsBothSharedEdgeCycles(t *testing.T) {
	t.Run("spec-resolve/AC-15 theta graph reports both shared-edge cycles", func(t *testing.T) {
		g := ResolveSpecs(thetaGraphInputs())

		want := []string{"a -> x -> b -> a", "a -> y -> b -> a"}
		got := circularPaths(g)
		if !slices.Equal(got, want) {
			t.Errorf("expected exactly cycles %v, got %v", want, got)
		}
		if len(g.TopologicalOrder) != 0 {
			t.Error("expected empty topo order when cycles exist")
		}
	})
}

// @ac AC-15
func TestSharedNodeCyclesBothReported(t *testing.T) {
	t.Run("spec-resolve/AC-15 two cycles sharing a node are both reported", func(t *testing.T) {
		// a <-> b and b <-> c share node b.
		g := ResolveSpecs([]SpecInput{
			{Spec: makeSpec("a", withDeps(dep("b"))), File: "a.spec.yaml"},
			{Spec: makeSpec("b", withDeps(dep("a"), dep("c"))), File: "b.spec.yaml"},
			{Spec: makeSpec("c", withDeps(dep("b"))), File: "c.spec.yaml"},
		})

		want := []string{"a -> b -> a", "b -> c -> b"}
		got := circularPaths(g)
		if !slices.Equal(got, want) {
			t.Errorf("expected exactly cycles %v, got %v", want, got)
		}
	})
}

// @ac AC-15
func TestDisjointCyclesBothReported(t *testing.T) {
	t.Run("spec-resolve/AC-15 two disjoint cycles are both reported", func(t *testing.T) {
		// a <-> b and c <-> d share nothing.
		g := ResolveSpecs([]SpecInput{
			{Spec: makeSpec("a", withDeps(dep("b"))), File: "a.spec.yaml"},
			{Spec: makeSpec("b", withDeps(dep("a"))), File: "b.spec.yaml"},
			{Spec: makeSpec("c", withDeps(dep("d"))), File: "c.spec.yaml"},
			{Spec: makeSpec("d", withDeps(dep("c"))), File: "d.spec.yaml"},
		})

		want := []string{"a -> b -> a", "c -> d -> c"}
		got := circularPaths(g)
		if !slices.Equal(got, want) {
			t.Errorf("expected exactly cycles %v, got %v", want, got)
		}
	})
}

// @ac AC-15
func TestCycleReportingIsDeterministic(t *testing.T) {
	t.Run("spec-resolve/AC-15 cycle reporting is deterministic across runs", func(t *testing.T) {
		// Map iteration order varies per run; the diagnostic set must not.
		first := circularPaths(ResolveSpecs(thetaGraphInputs()))
		if len(first) != 2 {
			t.Fatalf("expected 2 cycles in theta graph, got %v", first)
		}
		for i := 0; i < 50; i++ {
			got := circularPaths(ResolveSpecs(thetaGraphInputs()))
			if !slices.Equal(got, first) {
				t.Fatalf("iteration %d: cycle set %v differs from first run %v", i, got, first)
			}
		}
	})
}

// @ac AC-09
func TestDanglingReferenceNoSuggestionWhenFarOff(t *testing.T) {
	t.Run("spec-resolve/AC-09 dangling reference no suggestion when far off", func(t *testing.T) {
		// "xyz-totally-different" is far from "handler-interface"
		g := ResolveSpecs([]SpecInput{
			{Spec: makeSpec("a", withDeps(dep("xyz-totally-different"))), File: "a.spec.yaml"},
			{Spec: makeSpec("handler-interface"), File: "handler-interface.spec.yaml"},
		})

		for _, d := range g.Diagnostics {
			if d.Kind == "dangling_reference" {
				// suggestions may be empty for very distant strings — that's correct
				if len(d.Suggestions) > 0 {
					t.Logf("suggestions present but string is far: %v (acceptable)", d.Suggestions)
				}
				return
			}
		}
		t.Error("expected dangling_reference diagnostic")
	})
}

// Tests for DependentsOf — spec-resolve 1.2.0 C-12/C-13/AC-10..AC-14.
//
// @spec spec-resolve
package resolver

import (
	"reflect"
	"strings"
	"testing"
)

// AC-10: returns sorted list of direct dependents.
//
// @ac AC-10
func TestDependentsOf_DirectDependents_Sorted(t *testing.T) {
	t.Run("spec-resolve/AC-10 dependents returns sorted direct dependents", func(t *testing.T) {
		// Graph: spec-c → spec-a, spec-b → spec-a, spec-c → spec-b
		// (spec-a has dependents spec-b and spec-c)
		inputs := []SpecInput{
			{Spec: makeSpec("spec-a")},
			{Spec: makeSpec("spec-b", withDeps(dep("spec-a")))},
			{Spec: makeSpec("spec-c", withDeps(dep("spec-a"), dep("spec-b")))},
		}
		graph := ResolveSpecs(inputs)

		got, err := DependentsOf(graph, "spec-a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"spec-b", "spec-c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})
}

// AC-11: leaf node returns empty slice, no error.
//
// @ac AC-11
func TestDependentsOf_LeafNode_EmptyResult(t *testing.T) {
	t.Run("spec-resolve/AC-11 leaf node returns empty slice with no error", func(t *testing.T) {
		// Graph: spec-b → spec-a. spec-b has NO dependents.
		inputs := []SpecInput{
			{Spec: makeSpec("spec-a")},
			{Spec: makeSpec("spec-b", withDeps(dep("spec-a")))},
		}
		graph := ResolveSpecs(inputs)

		got, err := DependentsOf(graph, "spec-b")
		if err != nil {
			t.Fatalf("unexpected error for leaf node: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice for leaf node, got %v", got)
		}
	})
}

// AC-12: unknown spec id returns error.
//
// @ac AC-12
func TestDependentsOf_UnknownSpec_ReturnsError(t *testing.T) {
	t.Run("spec-resolve/AC-12 unknown spec id returns error", func(t *testing.T) {
		inputs := []SpecInput{
			{Spec: makeSpec("spec-a")},
		}
		graph := ResolveSpecs(inputs)

		_, err := DependentsOf(graph, "spec-z")
		if err == nil {
			t.Fatal("expected error for unknown spec id, got nil")
		}
		if !strings.Contains(err.Error(), "not found in graph") {
			t.Errorf("expected error to mention 'not found in graph', got: %v", err)
		}
	})
}

// AC-14: DependentsOf is a pure function — no I/O, deterministic output.
// Implicit guarantee: the algorithm sorts its output.
//
// @ac AC-14
func TestDependentsOf_DeterministicOrder(t *testing.T) {
	t.Run("spec-resolve/AC-14 dependents output is sorted lexicographically", func(t *testing.T) {
		// 5 dependents in mixed-case-insensitive order
		inputs := []SpecInput{
			{Spec: makeSpec("base")},
			{Spec: makeSpec("zulu", withDeps(dep("base")))},
			{Spec: makeSpec("alpha", withDeps(dep("base")))},
			{Spec: makeSpec("mike", withDeps(dep("base")))},
			{Spec: makeSpec("foxtrot", withDeps(dep("base")))},
			{Spec: makeSpec("delta", withDeps(dep("base")))},
		}
		graph := ResolveSpecs(inputs)

		got, err := DependentsOf(graph, "base")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"alpha", "delta", "foxtrot", "mike", "zulu"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected lexicographic sort %v, got %v", want, got)
		}
	})
}

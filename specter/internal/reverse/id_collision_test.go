// id_collision_test.go -- C-16: spec ids are unique within a run.
//
// @spec spec-reverse
package reverse

import (
	"sort"
	"strings"
	"testing"
)

// collidingGoFiles returns two source files that share a basename in different
// directories. Both yield the bare id "coverage" before disambiguation.
//
// No test files: a `_test.go` in the same directory is its own group and its
// own colliding id, which is a different defect (the group key does not fold a
// test file onto its source). Including one here would test two bugs at once
// and leave this fixture path-exhausted, where only the numeric fallback can
// separate the ids.
func collidingGoFiles() []SourceFile {
	return []SourceFile{
		{Path: "internal/coverage/coverage.go", Content: `package coverage

type Report struct {
	Name string ` + "`validate:\"required\"`" + `
}
`},
		{Path: "internal/diff/coverage.go", Content: `package diff

type Delta struct {
	Pct int ` + "`validate:\"required,min=0\"`" + `
}
`},
	}
}

// specIDsByFile maps each generated spec's provenance source file to its id, so
// an assertion can name a file rather than depend on result ordering.
func specIDsByFile(result *ReverseResult) map[string]string {
	out := make(map[string]string, len(result.Specs))
	for _, gs := range result.Specs {
		src := ""
		if gs.Spec.GeneratedFrom != nil {
			src = gs.Spec.GeneratedFrom.SourceFile
		}
		out[src] = gs.Spec.ID
	}
	return out
}

// @ac AC-21
// Two files sharing a basename in different directories get distinct ids, and
// the generated set contains no duplicate. This is the failure `resolve`
// rejects on 10 of the 12 repositories in the real-world corpus.
func TestReverse_CollidingBasenamesGetDistinctIDs(t *testing.T) {
	t.Run("spec-reverse/AC-21 colliding basenames get distinct ids", func(t *testing.T) {
		result := Reverse(
			ReverseInput{Files: collidingGoFiles(), Date: "2026-08-23"},
			[]Adapter{&GoAdapter{}},
		)
		if len(result.Specs) != 2 {
			t.Fatalf("expected 2 specs from 2 source files, got %d", len(result.Specs))
		}

		seen := make(map[string][]string)
		for _, gs := range result.Specs {
			seen[gs.Spec.ID] = append(seen[gs.Spec.ID], gs.FileName)
		}
		for id, files := range seen {
			if len(files) > 1 {
				sort.Strings(files)
				t.Errorf("duplicate spec id %q across %v; C-16 requires ids unique within a run", id, files)
			}
		}

		byFile := specIDsByFile(result)
		a := byFile["internal/coverage/coverage.go"]
		b := byFile["internal/diff/coverage.go"]
		if a == "" || b == "" {
			t.Fatalf("expected both source files represented, got %v", byFile)
		}
		// The disambiguated id must carry a path segment, not a bare counter,
		// so it still says something about where the spec came from.
		if !strings.Contains(b, "diff") {
			t.Errorf("id for internal/diff/coverage.go is %q; expected it to carry the "+
				"distinguishing path segment %q", b, "diff")
		}
	})
}

// @ac AC-22
// The same files in a different order produce the same id for the same file.
// Without this, two runs over an unchanged tree rename specs against each
// other, because groups are assembled from a Go map.
func TestReverse_DisambiguationIsOrderIndependent(t *testing.T) {
	t.Run("spec-reverse/AC-22 disambiguation is order independent", func(t *testing.T) {
		forward := collidingGoFiles()
		reversed := make([]SourceFile, len(forward))
		for i, f := range forward {
			reversed[len(forward)-1-i] = f
		}

		first := specIDsByFile(Reverse(
			ReverseInput{Files: forward, Date: "2026-08-23"}, []Adapter{&GoAdapter{}}))
		second := specIDsByFile(Reverse(
			ReverseInput{Files: reversed, Date: "2026-08-23"}, []Adapter{&GoAdapter{}}))

		if len(first) != 2 {
			t.Fatalf("expected 2 specs, got %d; the fixture cannot exercise determinism", len(first))
		}
		// Order-independence is vacuous while both files share one id: identical
		// ids are trivially stable under reordering. The property only has
		// content once ids are file-specific, so assert that first.
		if first["internal/coverage/coverage.go"] == first["internal/diff/coverage.go"] {
			t.Fatalf("both files still map to id %q; order-independence cannot be "+
				"observed until ids distinguish the files",
				first["internal/coverage/coverage.go"])
		}
		for src, id := range first {
			if second[src] != id {
				t.Errorf("id for %s depends on input order: %q forward, %q reversed",
					src, id, second[src])
			}
		}
	})
}

// @ac AC-23
// Each rename is reported. The operator's id is no longer the basename they
// would guess from the filename, and nothing else in the output says so.
func TestReverse_CollisionEmitsDiagnostic(t *testing.T) {
	t.Run("spec-reverse/AC-23 collision emits diagnostic", func(t *testing.T) {
		result := Reverse(
			ReverseInput{Files: collidingGoFiles(), Date: "2026-08-23"},
			[]Adapter{&GoAdapter{}},
		)

		var found *ReverseDiagnostic
		for i := range result.Diagnostics {
			if result.Diagnostics[i].Kind == "id_collision" {
				found = &result.Diagnostics[i]
				break
			}
		}
		if found == nil {
			kinds := make([]string, 0, len(result.Diagnostics))
			for _, d := range result.Diagnostics {
				kinds = append(kinds, d.Kind)
			}
			t.Fatalf("no id_collision diagnostic; C-16 requires one per rename. got kinds %v", kinds)
		}
		if found.Severity != "warning" {
			t.Errorf("id_collision severity is %q, want %q", found.Severity, "warning")
		}
		if !strings.Contains(found.Message, "coverage") {
			t.Errorf("id_collision message does not name the colliding id: %q", found.Message)
		}
	})
}

// @ac AC-21
// The numeric fallback, exercised directly. Two groups in the same directory
// with the same provisional id run out of path segments: no amount of
// deepening separates them, so one takes a suffix. Neither the unit fixtures
// above nor the 12-repo corpus reach this branch, because both separate on the
// first segment.
func TestDisambiguateSpecIDs_NumericFallbackWhenPathExhausted(t *testing.T) {
	t.Run("spec-reverse/AC-21 numeric fallback when path exhausted", func(t *testing.T) {
		keys := []string{"pkg/thing.go", "pkg/thing_test.go"}
		provisional := map[string]string{
			"pkg/thing.go":      "thing",
			"pkg/thing_test.go": "thing",
		}

		final, renames := DisambiguateSpecIDs(keys, provisional)
		if final[keys[0]] == final[keys[1]] {
			t.Fatalf("both keys still map to %q; C-16 requires unique ids", final[keys[0]])
		}
		// Both, not one. Every member of a colliding set is disambiguated, so
		// adding a third file later cannot silently take the short id away
		// from whichever spec happened to hold it.
		if len(renames) != 2 {
			t.Errorf("expected both members renamed, got %d: %v", len(renames), renames)
		}
		// Sorted order decides: the first key keeps the deepened id, the second
		// takes the suffix. The point is that it is decided, not which way.
		if final["pkg/thing.go"] != "pkg-thing" {
			t.Errorf("first key by sort order got %q, want %q", final["pkg/thing.go"], "pkg-thing")
		}
		if final["pkg/thing_test.go"] != "pkg-thing-2" {
			t.Errorf("second key by sort order got %q, want %q",
				final["pkg/thing_test.go"], "pkg-thing-2")
		}
	})
}

// @ac AC-21
// A deepened id can land on an id that was never part of the original
// colliding set. The uniqueness pass runs over every key, not only the ones
// that started out colliding, so this resolves rather than silently shipping a
// duplicate.
func TestDisambiguateSpecIDs_DeepenedIDCollidesWithUnrelated(t *testing.T) {
	t.Run("spec-reverse/AC-21 deepened id collides with unrelated", func(t *testing.T) {
		keys := []string{
			"coverage/coverage.go",
			"diff/coverage.go",
			"root/coverage-coverage.go",
		}
		provisional := map[string]string{
			"coverage/coverage.go":      "coverage",
			"diff/coverage.go":          "coverage",
			"root/coverage-coverage.go": "coverage-coverage",
		}

		final, _ := DisambiguateSpecIDs(keys, provisional)
		seen := make(map[string]string)
		for _, k := range keys {
			if prev, dup := seen[final[k]]; dup {
				t.Errorf("id %q assigned to both %s and %s", final[k], prev, k)
			}
			seen[final[k]] = k
		}
	})
}

// @ac AC-22
// Determinism at the unit level: the same keys in a different order produce
// the same assignment. DisambiguateSpecIDs documents that the caller sorts, so
// this feeds it sorted input both times and varies only the map insertion
// order, which is what a caller cannot control.
func TestDisambiguateSpecIDs_StableAcrossMapOrdering(t *testing.T) {
	t.Run("spec-reverse/AC-22 stable across map ordering", func(t *testing.T) {
		keys := []string{"a/dup.go", "b/dup.go", "c/dup.go"}
		build := func(order []string) map[string]string {
			m := make(map[string]string)
			for _, k := range order {
				m[k] = "dup"
			}
			return m
		}
		first, _ := DisambiguateSpecIDs(keys, build([]string{"a/dup.go", "b/dup.go", "c/dup.go"}))
		second, _ := DisambiguateSpecIDs(keys, build([]string{"c/dup.go", "a/dup.go", "b/dup.go"}))
		for _, k := range keys {
			if first[k] != second[k] {
				t.Errorf("%s got %q then %q", k, first[k], second[k])
			}
		}
	})
}

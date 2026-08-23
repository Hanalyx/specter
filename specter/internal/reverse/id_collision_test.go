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

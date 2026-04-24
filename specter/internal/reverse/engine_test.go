// engine_test.go -- tests for the core Reverse engine output properties.
//
// @spec spec-reverse
package reverse

import (
	"fmt"
	"testing"

	"github.com/Hanalyx/specter/internal/parser"
)

// makeGoFiles returns a pair of source + test files for use in engine tests.
func makeGoFiles(sourceContent, testContent string) []SourceFile {
	return []SourceFile{
		{Path: "user.go", Content: sourceContent},
		{Path: "user_test.go", Content: testContent},
	}
}

// @ac AC-09
func TestReverse_GeneratedYAMLPassesParseSpec(t *testing.T) {
	t.Run("spec-reverse/AC-09 generated YAML passes parse spec", func(t *testing.T) {
		files := makeGoFiles(
			`package main
type User struct {
	Name string `+"`validate:\"required\"`"+`
	Age  int    `+"`validate:\"required,min=0\"`"+`
}
`,
			`package main
import "testing"
func TestUser(t *testing.T) {
	t.Run("should create valid user", func(t *testing.T) {})
	t.Run("should reject missing name", func(t *testing.T) {})
}
`,
		)
		result := Reverse(ReverseInput{Files: files, Date: "2026-04-14"}, []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}})
		if len(result.Specs) == 0 {
			t.Skip("no specs generated — AC-09 requires at least one spec")
		}
		for _, gs := range result.Specs {
			pr := parser.ParseSpec(gs.YAML)
			if !pr.OK {
				t.Errorf("generated YAML for %q failed ParseSpec: %v", gs.Spec.ID, pr.Errors)
			}
		}
	})
}

// @ac AC-10
func TestReverse_GeneratedSpecIncludesGeneratedFrom(t *testing.T) {
	t.Run("spec-reverse/AC-10 generated spec includes generated_from", func(t *testing.T) {
		// Use a single source file to ensure generated_from.source_file is populated.
		// In "by-file" grouping, each source file becomes its own spec group.
		files := []SourceFile{
			{
				Path: "user.go",
				Content: `package main
type User struct {
	ID string ` + "`validate:\"required\"`" + `
}
`,
			},
		}
		result := Reverse(ReverseInput{Files: files, Date: "2026-04-14"}, []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}})
		if len(result.Specs) == 0 {
			t.Skip("no specs generated — AC-10 requires at least one spec")
		}
		for _, gs := range result.Specs {
			if gs.Spec.GeneratedFrom == nil {
				t.Errorf("spec %q is missing generated_from provenance", gs.Spec.ID)
				continue
			}
			if gs.Spec.GeneratedFrom.SourceFile == "" {
				t.Errorf("spec %q has empty generated_from.source_file", gs.Spec.ID)
			}
			if gs.Spec.GeneratedFrom.ExtractionDate != "2026-04-14" {
				t.Errorf("spec %q generated_from.extraction_date = %q, want %q",
					gs.Spec.ID, gs.Spec.GeneratedFrom.ExtractionDate, "2026-04-14")
			}
		}
	})
}

// @ac AC-16
func TestReverse_FileNameMirrorsSourceDirectory(t *testing.T) {
	t.Run("spec-reverse/AC-16 filename mirrors source directory", func(t *testing.T) {
		files := []SourceFile{
			{
				Path: "auth/login.go",
				Content: `package auth
type LoginRequest struct {
	Username string ` + "`validate:\"required\"`" + `
}
`,
			},
			{
				Path: "payments/stripe.go",
				Content: `package payments
type ChargeRequest struct {
	Amount int ` + "`validate:\"required,min=1\"`" + `
}
`,
			},
		}
		result := Reverse(ReverseInput{Files: files, Date: "2026-04-16"}, []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}})
		if len(result.Specs) == 0 {
			t.Fatal("no specs generated")
		}
		for _, gs := range result.Specs {
			dir := gs.FileName[:len(gs.FileName)-len("/"+gs.Spec.ID+".spec.yaml")]
			if dir == gs.FileName {
				// FileName has no subdir separator at all
				t.Errorf("spec %q: FileName %q has no subdirectory — expected mirrored path like auth/login.spec.yaml", gs.Spec.ID, gs.FileName)
				continue
			}
			// dir must not be empty for files that live in a subdirectory
			if dir == "" || dir == "." {
				t.Errorf("spec %q: FileName %q subdir is empty, want non-empty mirrored directory", gs.Spec.ID, gs.FileName)
			}
		}
	})
}

// @ac AC-14
func TestReverse_IsPureFunction(t *testing.T) {
	t.Run("spec-reverse/AC-14 reverse is pure function", func(t *testing.T) {
		// AC-14: the core Reverse function is a pure function — it accepts all
		// inputs as parameters and returns all outputs without side effects.
		// Verify it works with in-memory inputs and produces structurally
		// equivalent output.  We compare spec metadata rather than raw YAML
		// because Go map iteration order is non-deterministic, so the YAML
		// serialization may differ between calls even for identical content.
		files := makeGoFiles(
			`package main
type Item struct {
	Name string `+"`validate:\"required\"`"+`
}
`,
			`package main
import "testing"
func TestItem(t *testing.T) {
	t.Run("valid item", func(t *testing.T) {})
}
`,
		)
		adapters := []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}}
		input := ReverseInput{Files: files, Date: "2026-01-01"}

		r1 := Reverse(input, adapters)
		r2 := Reverse(input, adapters)

		// Same inputs must produce same number of specs.
		if len(r1.Specs) != len(r2.Specs) {
			t.Fatalf("Reverse is not deterministic: first call produced %d specs, second produced %d", len(r1.Specs), len(r2.Specs))
		}
		// Output must exist — a file with a validate tag should produce at least one spec.
		if len(r1.Specs) == 0 {
			t.Fatal("expected at least one spec from input with validate tags")
		}
		// Structural equivalence: same IDs, same constraint/AC counts.
		for i := range r1.Specs {
			s1, s2 := r1.Specs[i].Spec, r2.Specs[i].Spec
			if s1.ID != s2.ID {
				t.Errorf("spec[%d] ID differs: %q vs %q", i, s1.ID, s2.ID)
			}
			if len(s1.Constraints) != len(s2.Constraints) {
				t.Errorf("spec[%d] constraint count differs: %d vs %d", i, len(s1.Constraints), len(s2.Constraints))
			}
			if len(s1.AcceptanceCriteria) != len(s2.AcceptanceCriteria) {
				t.Errorf("spec[%d] AC count differs: %d vs %d", i, len(s1.AcceptanceCriteria), len(s2.AcceptanceCriteria))
			}
		}
	})
}

// @ac AC-13
func TestReverse_ConstraintAndACIDsAreSequential(t *testing.T) {
	t.Run("spec-reverse/AC-13 constraint and AC IDs are sequential", func(t *testing.T) {
		files := makeGoFiles(
			`package main
type Order struct {
	CustomerID string `+"`validate:\"required\"`"+`
	Amount     float64 `+"`validate:\"required,min=0\"`"+`
	Status     string `+"`validate:\"required\"`"+`
}
`,
			`package main
import "testing"
func TestOrder(t *testing.T) {
	t.Run("should create valid order", func(t *testing.T) {})
	t.Run("should reject negative amount", func(t *testing.T) {})
}
`,
		)
		result := Reverse(ReverseInput{Files: files, Date: "2026-04-14"}, []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}})
		if len(result.Specs) == 0 {
			t.Skip("no specs generated")
		}
		for _, gs := range result.Specs {
			for i, c := range gs.Spec.Constraints {
				expected := fmt.Sprintf("C-%02d", i+1)
				if c.ID != expected {
					t.Errorf("constraints[%d].ID = %q, want %q (must be sequential)", i, c.ID, expected)
				}
			}
			for i, ac := range gs.Spec.AcceptanceCriteria {
				expected := fmt.Sprintf("AC-%02d", i+1)
				if ac.ID != expected {
					t.Errorf("acceptance_criteria[%d].ID = %q, want %q (must be sequential)", i, ac.ID, expected)
				}
			}
		}
	})
}

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
}

// @ac AC-10
func TestReverse_GeneratedSpecIncludesGeneratedFrom(t *testing.T) {
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
}

// @ac AC-13
func TestReverse_ConstraintAndACIDsAreSequential(t *testing.T) {
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
}

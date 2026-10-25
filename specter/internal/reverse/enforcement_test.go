// enforcement_test.go -- C-15: generated constraints omit the enforcement key.
//
// @spec spec-reverse
package reverse

import (
	"strings"
	"testing"
)

// specWithFileName returns the generated spec whose FileName matches, so a test
// that cares about one of several outputs does not depend on ordering.
func specWithFileName(specs []GeneratedSpec, name string) *GeneratedSpec {
	for i := range specs {
		if specs[i].FileName == name {
			return &specs[i]
		}
	}
	return nil
}

// @ac AC-20
// Constraints extracted from source carry no enforcement key. The assertion is
// on the marshaled YAML rather than on Constraint.Enforcement, because an empty
// string and an absent key are different documents: only the absent key takes
// the tier default that spec-check applies to an orphan constraint.
func TestReverse_ExtractedConstraintsOmitEnforcement(t *testing.T) {
	t.Run("spec-reverse/AC-20 extracted constraints omit enforcement", func(t *testing.T) {
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
}
`,
		)
		result := Reverse(ReverseInput{Files: files, Date: "2026-08-23"}, []Adapter{&GoAdapter{}})
		if len(result.Specs) == 0 {
			t.Fatal("no specs generated; the fixture cannot exercise C-15")
		}

		gs := specWithFileName(result.Specs, "user.spec.yaml")
		if gs == nil {
			t.Fatalf("expected user.spec.yaml among generated specs, got %d spec(s)", len(result.Specs))
		}
		if len(gs.Spec.Constraints) == 0 {
			t.Fatal("fixture produced no constraints; it cannot show what enforcement they carry")
		}
		if strings.Contains(gs.YAML, "enforcement:") {
			t.Errorf("generated YAML carries an enforcement key on a constraint; C-15 requires it omitted so the tier default applies.\nYAML:\n%s", gs.YAML)
		}
	})
}

// @ac AC-20
// The placeholder constraint synthesized when extraction found nothing carries
// no enforcement key either. This is the path that hurts: the constraint exists
// only because the schema requires minItems 1, no acceptance criterion
// references it, so it is an orphan the moment it is written.
func TestReverse_PlaceholderConstraintOmitsEnforcement(t *testing.T) {
	t.Run("spec-reverse/AC-20 placeholder constraint omits enforcement", func(t *testing.T) {
		// A source file with no validation rules and a test file with one case,
		// so a spec is generated but nothing is extracted to constrain it.
		files := []SourceFile{
			{Path: "pattern.go", Content: `package main

func Match(s string) bool {
	return len(s) > 0
}
`},
			{Path: "pattern_test.go", Content: `package main
import "testing"
func TestMatch(t *testing.T) {
	t.Run("matches a non-empty string", func(t *testing.T) {})
}
`},
		}
		result := Reverse(ReverseInput{Files: files, Date: "2026-08-23"}, []Adapter{&GoAdapter{}})
		if len(result.Specs) == 0 {
			t.Fatal("no specs generated; the fixture cannot exercise the placeholder path")
		}

		gs := specWithFileName(result.Specs, "pattern.spec.yaml")
		if gs == nil {
			t.Fatalf("expected pattern.spec.yaml among generated specs, got %d spec(s)", len(result.Specs))
		}
		if len(gs.Spec.Constraints) != 1 {
			t.Fatalf("expected the single synthesized placeholder constraint, got %d", len(gs.Spec.Constraints))
		}
		if !strings.Contains(gs.Spec.Constraints[0].Description, "placeholder") {
			t.Fatalf("fixture did not take the placeholder path; constraint description was %q",
				gs.Spec.Constraints[0].Description)
		}
		if strings.Contains(gs.YAML, "enforcement:") {
			t.Errorf("the synthesized placeholder carries an enforcement key; C-15 requires it omitted.\nYAML:\n%s", gs.YAML)
		}
	})
}

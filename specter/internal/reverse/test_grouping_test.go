// test_grouping_test.go -- C-17: a test file joins the source it tests.
//
// These go through Reverse() rather than calling the adapter method directly.
// The method does not exist yet, so a direct call would fail to compile, and a
// compile failure is not a red test.
//
// @spec spec-reverse
package reverse

import (
	"strings"
	"testing"
)

// pairedGoFiles returns a source file with two validated fields, the test that
// covers one of them, and a test file whose source was never supplied.
func pairedGoFiles() []SourceFile {
	return []SourceFile{
		{Path: "user.go", Content: `package main

type User struct {
	Name string ` + "`validate:\"required\"`" + `
	Age  int    ` + "`validate:\"required,min=0\"`" + `
}
`},
		{Path: "user_test.go", Content: `package main
import "testing"
func TestUser(t *testing.T) {
	t.Run("should reject a user with no Name", func(t *testing.T) {})
	t.Run("should accept a user with a Name", func(t *testing.T) {})
}
`},
		{Path: "orphan_test.go", Content: `package main
import "testing"
func TestOrphan(t *testing.T) {
	t.Run("covers something whose source was not supplied", func(t *testing.T) {})
}
`},
	}
}

// @ac AC-24
// The pair becomes one spec carrying both halves, and the unpaired test still
// gets its own. Today all three files are separate groups and the run produces
// three specs.
func TestReverse_TestFileJoinsItsSource(t *testing.T) {
	t.Run("spec-reverse/AC-24 test file joins its source", func(t *testing.T) {
		result := Reverse(
			ReverseInput{Files: pairedGoFiles(), Date: "2026-08-23"},
			[]Adapter{&GoAdapter{}},
		)

		if len(result.Specs) != 2 {
			names := make([]string, 0, len(result.Specs))
			for _, gs := range result.Specs {
				names = append(names, gs.FileName)
			}
			t.Fatalf("expected 2 specs (the merged pair, plus the unpaired test), got %d: %v",
				len(result.Specs), names)
		}

		var merged *GeneratedSpec
		for i := range result.Specs {
			if result.Specs[i].FileName == "user.spec.yaml" {
				merged = &result.Specs[i]
			}
		}
		if merged == nil {
			t.Fatal("no user.spec.yaml among the generated specs")
		}

		// Both halves, in one spec: the constraints come from the source file,
		// the criteria from the test file's cases.
		if len(merged.Spec.Constraints) < 2 {
			t.Errorf("merged spec carries %d constraint(s), want the 2 from user.go",
				len(merged.Spec.Constraints))
		}
		fromTest := false
		for _, ac := range merged.Spec.AcceptanceCriteria {
			if strings.Contains(strings.ToLower(ac.Description), "name") && !ac.Gap {
				fromTest = true
			}
		}
		if !fromTest {
			t.Errorf("merged spec has no criterion derived from user_test.go; got %d criteria",
				len(merged.Spec.AcceptanceCriteria))
		}
	})
}

// @ac AC-25
// A constraint whose test is right there is not a gap. Grouped apart from its
// assertions, DetectGaps compares it against an empty list and every
// constraint comes back UNTESTED, so this is a false gap rather than a
// missing one.
func TestReverse_TestedConstraintIsNotAGap(t *testing.T) {
	t.Run("spec-reverse/AC-25 tested constraint is not a gap", func(t *testing.T) {
		result := Reverse(
			ReverseInput{Files: pairedGoFiles(), Date: "2026-08-23"},
			[]Adapter{&GoAdapter{}},
		)

		var merged *GeneratedSpec
		for i := range result.Specs {
			if result.Specs[i].FileName == "user.spec.yaml" {
				merged = &result.Specs[i]
			}
		}
		if merged == nil {
			t.Fatal("no user.spec.yaml among the generated specs")
		}

		// Name is required, and a test asserts a user with no Name is rejected.
		// That constraint must not be reported untested.
		for _, ac := range merged.Spec.AcceptanceCriteria {
			if !ac.Gap {
				continue
			}
			if strings.Contains(strings.ToLower(ac.Description), "name") {
				t.Errorf("constraint on Name reported as a gap, but user_test.go covers it: %q",
					ac.Description)
			}
		}
	})
}

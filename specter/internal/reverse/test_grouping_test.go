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

// @ac AC-24
// The parity property, across every adapter. If SourceKeyForTest names a
// source, that source must not itself be a test file. Without this, a mapping
// bug folds one test onto another and the source it was meant to join is never
// grouped with anything.
//
// This is the declaration-pair rule the project applies elsewhere: two
// functions that must agree get a test that fails when they disagree, not a
// reviewer who is expected to notice.
func TestAdapters_SourceKeyForTestNeverNamesAnotherTest(t *testing.T) {
	t.Run("spec-reverse/AC-24 source key never names another test", func(t *testing.T) {
		paths := []string{
			"user_test.go", "internal/pkg/user_test.go", "user.go",
			"login.test.ts", "login.spec.tsx", "src/__tests__/login.test.ts",
			"src/__tests__/helpers.ts", "login.ts",
			"test_auth.py", "auth_test.py", "pkg/tests/test_auth.py",
			"pkg/tests/conftest.py", "auth.py",
		}
		adapters := []Adapter{&GoAdapter{}, &TypeScriptAdapter{}, &PythonAdapter{}}

		for _, a := range adapters {
			for _, p := range paths {
				src := a.SourceKeyForTest(p)
				if src == "" {
					continue
				}
				if !a.IsTestFile(p) {
					t.Errorf("%s: SourceKeyForTest(%q) returned %q for a path that is not a test file",
						a.Name(), p, src)
				}
				if a.IsTestFile(src) {
					t.Errorf("%s: SourceKeyForTest(%q) named %q, which is itself a test file",
						a.Name(), p, src)
				}
				if src == p {
					t.Errorf("%s: SourceKeyForTest(%q) returned the path unchanged", a.Name(), p)
				}
			}
		}
	})
}

// @ac AC-24
// The mappings themselves, per adapter, including the cases that must return
// empty. An unmappable test is not a failure: it keeps its own group, which is
// the behavior it had before C-17.
func TestAdapters_SourceKeyForTestMappings(t *testing.T) {
	t.Run("spec-reverse/AC-24 source key mappings", func(t *testing.T) {
		cases := []struct {
			adapter Adapter
			path    string
			want    string
		}{
			{&GoAdapter{}, "user_test.go", "user.go"},
			{&GoAdapter{}, "internal/pkg/user_test.go", "internal/pkg/user.go"},
			{&GoAdapter{}, "user.go", ""},

			{&TypeScriptAdapter{}, "login.test.ts", "login.ts"},
			{&TypeScriptAdapter{}, "src/login.spec.tsx", "src/login.tsx"},
			{&TypeScriptAdapter{}, "src/__tests__/login.test.ts", "src/login.ts"},
			{&TypeScriptAdapter{}, "src/login.ts", ""},

			{&PythonAdapter{}, "test_auth.py", "auth.py"},
			{&PythonAdapter{}, "auth_test.py", "auth.py"},
			{&PythonAdapter{}, "pkg/tests/test_auth.py", "pkg/auth.py"},
			// Only a test because of where it sits. It names no source.
			{&PythonAdapter{}, "pkg/tests/conftest.py", ""},
			{&PythonAdapter{}, "pkg/auth.py", ""},
		}

		for _, c := range cases {
			if got := c.adapter.SourceKeyForTest(c.path); got != c.want {
				t.Errorf("%s.SourceKeyForTest(%q) = %q, want %q",
					c.adapter.Name(), c.path, got, c.want)
			}
		}
	})
}

// @ac AC-24
// The existence gate. A test whose named source is absent from the input keeps
// its own group rather than creating a phantom group named after a file
// nobody supplied.
func TestReverse_UnmatchedTestDoesNotCreateAPhantomGroup(t *testing.T) {
	t.Run("spec-reverse/AC-24 unmatched test does not create a phantom group", func(t *testing.T) {
		// An unrelated source file, so the run proceeds: Reverse returns early
		// with a no_source_files warning when nothing but tests is supplied.
		files := []SourceFile{
			{Path: "other.go", Content: `package main

type Other struct {
	Field string ` + "`validate:\"required\"`" + `
}
`},
			{Path: "alone_test.go", Content: `package main
import "testing"
func TestAlone(t *testing.T) {
	t.Run("has no source file in the input", func(t *testing.T) {})
}
`},
		}
		result := Reverse(ReverseInput{Files: files, Date: "2026-08-23"}, []Adapter{&GoAdapter{}})

		// alone_test.go names alone.go, which was not supplied, so it must keep
		// its own group rather than fold onto other.go or vanish.
		var alone *GeneratedSpec
		for i := range result.Specs {
			gf := result.Specs[i].Spec.GeneratedFrom
			if gf != nil && len(gf.TestFiles) == 1 && gf.TestFiles[0] == "alone_test.go" {
				alone = &result.Specs[i]
			}
		}
		if alone == nil {
			names := make([]string, 0, len(result.Specs))
			for _, gs := range result.Specs {
				names = append(names, gs.FileName)
			}
			t.Fatalf("alone_test.go produced no spec of its own; got %v", names)
		}
		if len(alone.Spec.Constraints) != 1 ||
			!strings.Contains(alone.Spec.Constraints[0].Description, "placeholder") {
			t.Errorf("expected the unpaired test to carry only the placeholder constraint, got %d: %+v",
				len(alone.Spec.Constraints), alone.Spec.Constraints)
		}
	})
}

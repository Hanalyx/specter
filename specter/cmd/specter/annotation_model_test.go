package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// annotationModelWorkspace builds the workspace C-38's criteria describe.
//
// annotatedACs get a source annotation. Criteria absent from that list carry no
// annotation anywhere, which is what makes them rule-1 violations. results maps
// an AC id to its status in .specter-results.json; an AC with an annotation and
// no entry is not the same case and is deliberately not used here.
func annotationModelWorkspace(t *testing.T, specID string, tier int, allACs, annotatedACs []string, results map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, specID+".spec.yaml", minimalValidSpec(specID, tier, allACs...))

	body := "package tests\n\nimport \"testing\"\n"
	for i, ac := range annotatedACs {
		body += "\n// @spec " + specID + "\n// @ac " + ac + "\n"
		body += "func Test" + string(rune('A'+i)) + "(t *testing.T) {\n"
		body += "\tt.Run(\"" + specID + "/" + ac + "\", func(t *testing.T) {})\n}\n"
	}
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "gen_test.go"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	if results != nil {
		entries := []string{}
		for ac, status := range results {
			entries = append(entries, `{"spec_id":"`+specID+`","ac_id":"`+ac+`","status":"`+status+`","test_name":"T`+ac+`"}`)
		}
		writeResultsJSON(t, dir, `{"results":[`+strings.Join(entries, ",")+`]}`)
	}
	return dir
}

// noTestLineNames reports whether ac appears on a `no test:` line, which C-38(a)
// requires to be distinct from the existing `uncovered:` line.
func noTestLineNames(out, ac string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "no test:") && strings.Contains(line, ac) {
			return true
		}
	}
	return false
}

func annotationModelManifest(permissive string) string {
	return "system:\n  name: am\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n  annotation:\n    permissive: " + permissive + "\n"
}

// @spec spec-coverage
// @ac AC-60
//
// C-38(a),(b),(d): a criterion with no test fails, the tier does not excuse it,
// and it exits 2.
func TestAnnotationModel_MissingTestFailsRegardlessOfTier(t *testing.T) {
	t.Run("spec-coverage/AC-60 missing test fails regardless of tier", func(t *testing.T) {
		mk := func() string {
			return annotationModelWorkspace(t, "ab-spec", 3,
				[]string{"AC-01", "AC-02"}, []string{"AC-01"},
				map[string]string{"AC-01": "passed"})
		}

		// Control: the same workspace under the ladder passes, because 1 of 2
		// is 50 percent and the Tier 3 threshold is 50. If this stops being
		// true the criterion's contrast is gone and the assertion below means
		// less than it claims.
		dir := mk()
		putManifest(t, dir, "system:\n  name: am\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n  strictness: annotation\n")
		_, _, code := runCLISplit(t, dir, "coverage")
		if code != 0 {
			t.Fatalf("control: same workspace under strictness: annotation should exit 0, got %d", code)
		}

		// C-38: the block is declared, so the tier arithmetic no longer excuses
		// a criterion with no test.
		dir = mk()
		putManifest(t, dir, annotationModelManifest("false"))
		stdout, stderr, code := runCLISplit(t, dir, "coverage")
		out := stdout + stderr
		if code != 2 {
			t.Errorf("C-38(d): a rule-1 violation must exit 2, got %d\noutput:\n%s", code, out)
		}
		if !noTestLineNames(out, "AC-02") {
			t.Errorf("C-38(a): AC-02 must appear on a `no test:` line, distinct from `uncovered:`, got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-61
//
// C-38(c): permissive is a severity switch, not a scope switch.
func TestAnnotationModel_PermissiveIsSeverityNotScope(t *testing.T) {
	t.Run("spec-coverage/AC-61 permissive is severity not scope", func(t *testing.T) {
		dir := annotationModelWorkspace(t, "ab-spec", 3,
			[]string{"AC-01", "AC-02"}, []string{"AC-01"},
			map[string]string{"AC-01": "passed"})
		putManifest(t, dir, annotationModelManifest("true"))
		stdout, stderr, code := runCLISplit(t, dir, "coverage")
		out := stdout + stderr

		if code != 0 {
			t.Errorf("C-38(c): permissive true must not fail the run, got exit %d\noutput:\n%s", code, out)
		}
		// The load-bearing half: it must still be reported AS A RULE-1
		// VIOLATION. A permissive mode that hides it is a scope switch, which
		// (c) forbids. Asserting the bare id would pass under the ladder,
		// where AC-02 already appears on the `uncovered:` line.
		if !noTestLineNames(out, "AC-02") {
			t.Errorf("C-38(c): AC-02 must still appear on a `no test:` line under permissive, got:\n%s", out)
		}
	})
}

// @spec spec-coverage
// @ac AC-62
//
// C-38(e): a pass rate below the tier is a different failure with a different
// code. Every criterion here has a test, so there is no rule-1 violation.
func TestAnnotationModel_PassRateFailureExitsOneNotTwo(t *testing.T) {
	t.Run("spec-coverage/AC-62 pass rate failure exits one not two", func(t *testing.T) {
		dir := annotationModelWorkspace(t, "pr-spec", 1,
			[]string{"AC-01", "AC-02"}, []string{"AC-01", "AC-02"},
			map[string]string{"AC-01": "passed", "AC-02": "failed"})
		putManifest(t, dir, annotationModelManifest("false"))
		stdout, stderr, code := runCLISplit(t, dir, "coverage")
		out := stdout + stderr

		if code == 2 {
			t.Errorf("C-38(e): every criterion has a test, so this is a rate failure and must not exit 2\noutput:\n%s", out)
		}
		if code != 1 {
			t.Errorf("C-38(e): a rate below the tier threshold must exit 1, got %d\noutput:\n%s", code, out)
		}
	})
}

// @spec spec-coverage
// @ac AC-63
//
// C-38(f): a declared block requires a results file and does not fall back to
// a structural check.
func TestAnnotationModel_BlockRequiresResultsFile(t *testing.T) {
	t.Run("spec-coverage/AC-63 block requires a results file", func(t *testing.T) {
		dir := annotationModelWorkspace(t, "ab-spec", 3,
			[]string{"AC-01", "AC-02"}, []string{"AC-01"}, nil)
		putManifest(t, dir, annotationModelManifest("false"))
		stdout, stderr, code := runCLISplit(t, dir, "coverage")
		out := stdout + stderr

		if code == 0 {
			t.Errorf("C-38(f): a declared block must not fall back to a structural check when results are missing\noutput:\n%s", out)
		}
		if !strings.Contains(out, ".specter-results.json") {
			t.Errorf("C-38(f): the message must name the missing results file, got:\n%s", out)
		}
	})
}

// coverage_results_order_test.go -- CLI integration tests for spec-coverage
// 1.16.0 C-33 and C-34: worst-status resolution of the results lookup, and
// the zero-tolerance count of criteria rather than entries.
//
// These criteria cross the CLI boundary because each one names a command, an
// exit code, or a stderr line. The pure half of C-33 is in
// internal/coverage/results_worst_status_test.go.
//
// Every fixture here carries more than one entry per pair, which is the whole
// point: a first-match lookup and a worst-status lookup agree on any file
// that holds one entry per pair.
//
// @spec spec-coverage
package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AC-39 -- row order does not change the emitted document
// ---------------------------------------------------------------------------

// The two results files hold the same four entries in different row order.
// The workspace, the spec, and the annotations are byte-identical between the
// two runs, so the emitted document is a function of the results file alone
// and the byte comparison is a comparison of row order and nothing else.
//
// AC-01 and AC-02 each carry one passed and one failed entry, so both pairs
// resolve to failed and both criteria are uncovered under --strict. That is
// the value half; the byte comparison is the ordering half.
//
// @ac AC-39
func TestCoverageResultsOrder_JSONIdenticalAcrossRowOrderings(t *testing.T) {
	const resultsA = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"passed"}]}`
	const resultsB = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"failed"}]}`

	// build writes the shared workspace and the given results file.
	build := func(t *testing.T, results string) string {
		t.Helper()
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 1, acIDs: []string{"AC-01", "AC-02"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})
		writeResultsJSON(t, dir, results)
		return dir
	}

	stdouts := make([]string, 2)
	orderings := []struct {
		name    string
		results string
	}{
		{"spec-coverage/AC-39 non-passing rows first reports both criteria uncovered", resultsA},
		{"spec-coverage/AC-39 passed rows first reports both criteria uncovered", resultsB},
	}

	for i, o := range orderings {
		t.Run(o.name, func(t *testing.T) {
			dir := build(t, o.results)
			stdout, stderr, code := runCLISplit(t, dir, "coverage", "--strict", "--json")
			stdouts[i] = stdout

			if code != 1 {
				t.Errorf("expected exit 1, got %d;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")
			if pct := floatField(t, entry, "coverage_pct"); !nearly(pct, 0) {
				t.Errorf("coverage_pct = %v, want 0", pct)
			}
			uncovered := stringsField(t, entry, "uncovered_acs")
			if len(uncovered) != 2 || uncovered[0] != "AC-01" || uncovered[1] != "AC-02" {
				t.Errorf("uncovered_acs = %v, want [AC-01 AC-02]", uncovered)
			}
		})
	}

	t.Run("spec-coverage/AC-39 both row orderings emit a byte-identical document", func(t *testing.T) {
		if stdouts[0] != stdouts[1] {
			t.Errorf("row order must not change the emitted document;\nnon-passing first:\n%s\npassed first:\n%s", stdouts[0], stdouts[1])
		}
	})
}

// ---------------------------------------------------------------------------
// AC-40 -- a status outside the enum ranks at failed, never at passed
// ---------------------------------------------------------------------------

// The pair carries one `passed` entry and one `ok` entry. A rank table that
// yields a zero value for an unrecognized key ranks `ok` at the passed level,
// resolves the pair as passed, and leaves AC-01 covered.
//
// `passed_ac01: false` is asserted through the consequence the criterion
// states for it: under --strict the criterion is uncovered. The method is
// unexported and the criterion names a command, so the CLI is where this
// claim is observable.
//
// The invalid_status_warnings assertion is the control that the `ok` entry
// reached the reader at all, rather than being dropped at parse.
//
// @ac AC-40
func TestCoverageResultsOrder_UnknownStatusRanksAtFailed(t *testing.T) {
	t.Run("spec-coverage/AC-40 a pair holding passed and an out-of-enum status is not passed", func(t *testing.T) {
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 1, acIDs: []string{"AC-01"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01"})
		writeResultsJSON(t, dir, `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"ok"}]}`)

		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--strict", "--json")
		if code == 0 {
			t.Errorf("expected a non-zero exit, got 0;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}

		doc := decodeCoverageJSON(t, stdout)
		entry := coverageEntry(t, doc, "demo-spec")
		uncovered := stringsField(t, entry, "uncovered_acs")
		if len(uncovered) != 1 || uncovered[0] != "AC-01" {
			t.Errorf("uncovered_acs = %v, want [AC-01]; an out-of-enum status must never rank at passed", uncovered)
		}

		warnings, ok := doc["invalid_status_warnings"].([]any)
		if !ok || len(warnings) != 1 {
			t.Fatalf("expected one invalid_status_warnings entry, got %v", doc["invalid_status_warnings"])
		}
		w, ok := warnings[0].(map[string]any)
		if !ok {
			t.Fatalf("invalid_status_warnings entry must be an object, got %T", warnings[0])
		}
		if w["status"] != "ok" {
			t.Errorf("invalid status warning names %v, want \"ok\"", w["status"])
		}
		if count := floatField(t, w, "count"); !nearly(count, 1) {
			t.Errorf("invalid status warning count = %v, want 1", count)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-41 -- the Tier 1 pass-rate path resolves the same way
// ---------------------------------------------------------------------------

// This is the non-strict call path. The manifest sets strictness to
// annotation, and the entries carry the back-compat `passed` boolean with no
// `status` field, so the C-21 derivation runs first and the false rows
// participate as failed.
//
// Both orderings are run against the same workspace, so a fix confined to the
// --strict path leaves one of the two arms reporting 100%.
//
// @ac AC-41
func TestCoverageResultsOrder_Tier1PassRatePathResolvesTheSameWay(t *testing.T) {
	const resultsA = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","passed":false},{"spec_id":"demo-spec","ac_id":"AC-01","passed":true},{"spec_id":"demo-spec","ac_id":"AC-02","passed":false},{"spec_id":"demo-spec","ac_id":"AC-02","passed":true}]}`
	const resultsB = `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","passed":true},{"spec_id":"demo-spec","ac_id":"AC-02","passed":true},{"spec_id":"demo-spec","ac_id":"AC-01","passed":false},{"spec_id":"demo-spec","ac_id":"AC-02","passed":false}]}`

	build := func(t *testing.T, results string) string {
		t.Helper()
		dir := t.TempDir()
		writeManifest(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  strictness: annotation\n")
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 1, acIDs: []string{"AC-01", "AC-02"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})
		writeResultsJSON(t, dir, results)
		return dir
	}

	pcts := make([]float64, 2)
	orderings := []struct {
		name    string
		results string
	}{
		{"spec-coverage/AC-41 pass-rate path with the false rows first reports 0%", resultsA},
		{"spec-coverage/AC-41 pass-rate path with the true rows first reports 0%", resultsB},
	}

	for i, o := range orderings {
		t.Run(o.name, func(t *testing.T) {
			dir := build(t, o.results)
			stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json")
			entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")
			pcts[i] = floatField(t, entry, "coverage_pct")
			if !nearly(pcts[i], 0) {
				t.Errorf("coverage_pct = %v, want 0;\nstdout:\n%s\nstderr:\n%s", pcts[i], stdout, stderr)
			}
			if covered := stringsField(t, entry, "covered_acs"); len(covered) != 0 {
				t.Errorf("covered_acs = %v, want empty", covered)
			}
		})
	}

	t.Run("spec-coverage/AC-41 both row orderings report the same coverage on the pass-rate path", func(t *testing.T) {
		if pcts[0] != pcts[1] {
			t.Errorf("row order must not change the pass-rate path; false first reported %v, true first reported %v", pcts[0], pcts[1])
		}
	})
}

// ---------------------------------------------------------------------------
// AC-42 -- zero tolerance counts criteria, not entries
// ---------------------------------------------------------------------------

// The five-entry file names three criteria. AC-01 carries three non-passing
// entries with two different statuses, AC-02 carries one, and AC-03 passes.
// Two criteria did not pass, so the message names 2 while an entry count
// would name 4.
//
// The three-entry file states the same facts one entry per pair, and is the
// control: it proves the sentence and the exit code are produced by this
// harness on a file where entry counting and pair counting agree, so the
// five-entry arm's failure is about the count and not about the plumbing.
//
// @ac AC-42
func TestCoverageZeroTolerance_CountsCriteriaNotEntries(t *testing.T) {
	build := func(t *testing.T, results string) string {
		t.Helper()
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01", "AC-02", "AC-03"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02", "AC-03"})
		writeResultsJSON(t, dir, results)
		return dir
	}

	cases := []struct {
		name    string
		results string
	}{
		{
			"spec-coverage/AC-42 five entries over three criteria report two criteria did not pass",
			`{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"failed"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"errored"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"skipped"},{"spec_id":"demo-spec","ac_id":"AC-03","status":"passed"}]}`,
		},
		{
			"spec-coverage/AC-42 the same facts as one entry per pair report the same count",
			`{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"errored"},{"spec_id":"demo-spec","ac_id":"AC-02","status":"skipped"},{"spec_id":"demo-spec","ac_id":"AC-03","status":"passed"}]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := build(t, c.results)
			stdout, stderr, code := runCLISplit(t, dir, "coverage", "--strictness", "zero-tolerance")
			if !strings.Contains(stderr, "2 annotated AC(s) did not pass") {
				t.Errorf("stderr must report `2 annotated AC(s) did not pass`, got:\n%s", stderr)
			}
			if code != 2 {
				t.Errorf("expected exit 2, got %d;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-43 -- InvalidStatuses keeps counting entries
// ---------------------------------------------------------------------------

// Three entries for one pair, all carrying the same unrecognized status. The
// C-30 warning text says entries, so the count is 3 and not 1. This is the
// criterion that fails a fix which applies the C-34 dedupe too widely, so it
// is expected to hold both before and after the change.
//
// @ac AC-43
func TestCoverageInvalidStatuses_CountEntriesNotPairs(t *testing.T) {
	build := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01"}})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01"})
		writeResultsJSON(t, dir, `{"results":[{"spec_id":"demo-spec","ac_id":"AC-01","status":"pass"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"pass"},{"spec_id":"demo-spec","ac_id":"AC-01","status":"pass"}]}`)
		return dir
	}

	t.Run("spec-coverage/AC-43 stderr reports three entries with the unrecognized status", func(t *testing.T) {
		dir := build(t)
		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--strict")
		if !strings.Contains(stderr, `3 entries with status="pass"`) {
			t.Errorf("stderr must report `3 entries with status=\"pass\"`, got:\n%s\nstdout:\n%s", stderr, stdout)
		}
	})

	t.Run("spec-coverage/AC-43 the JSON warning carries count 3", func(t *testing.T) {
		dir := build(t)
		stdout, _, _ := runCLISplit(t, dir, "coverage", "--strict", "--json")
		doc := decodeCoverageJSON(t, stdout)
		warnings, ok := doc["invalid_status_warnings"].([]any)
		if !ok || len(warnings) != 1 {
			t.Fatalf("expected one invalid_status_warnings entry, got %v", doc["invalid_status_warnings"])
		}
		w, ok := warnings[0].(map[string]any)
		if !ok {
			t.Fatalf("invalid_status_warnings entry must be an object, got %T", warnings[0])
		}
		if w["status"] != "pass" {
			t.Errorf("invalid status warning names %v, want \"pass\"", w["status"])
		}
		if count := floatField(t, w, "count"); !nearly(count, 3) {
			t.Errorf("invalid status warning count = %v, want 3", count)
		}
	})
}

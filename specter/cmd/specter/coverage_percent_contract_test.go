// coverage_percent_contract_test.go -- CLI integration tests for
// spec-coverage 1.16.0 C-35, the percentage contract (AC-44 through AC-46 and
// AC-48).
//
// Each of these criteria names a human-readable surface: a table cell, an
// `explain` line, the summary header, or an exit code driven by the threshold
// gate. None of them is observable from the pure functions alone, so they run
// the binary. The two pure functions are covered by AC-47 in
// internal/coverage/coverage_percent_test.go.
//
// @spec spec-coverage
package main

import (
	"strings"
	"testing"
)

// twoOfThreeWorkspace builds the workspace AC-44, AC-45 and AC-48 share: one
// spec declaring three criteria with two of them annotated. threshold is
// passed through so AC-45 can vary the declared threshold without varying
// anything else.
func twoOfThreeWorkspace(t *testing.T, threshold string) string {
	t.Helper()
	dir := t.TempDir()
	writeDefectSpec(t, dir, defectSpec{
		id:        "demo-spec",
		tier:      2,
		acIDs:     []string{"AC-01", "AC-02", "AC-03"},
		threshold: threshold,
	})
	writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})
	return dir
}

// ---------------------------------------------------------------------------
// AC-44 -- one value, three surfaces
// ---------------------------------------------------------------------------

// 2 of 3 is the ratio the criterion names, and it is the ratio that separates
// the three formatters: 66.666... rounds to 66.7, floors to 66, and truncates
// to 66.6. The three surfaces are read from three separate runs of the same
// workspace, so a surface that reads a cached value from another surface
// cannot pass by sharing.
//
// @ac AC-44
func TestCoveragePercent_ThreeSurfacesAgree(t *testing.T) {
	t.Run("spec-coverage/AC-44 coverage --json reports 66.7 for two of three criteria", func(t *testing.T) {
		dir := twoOfThreeWorkspace(t, "")
		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
		entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")
		if pct := floatField(t, entry, "coverage_pct"); !nearly(pct, 66.7) {
			t.Errorf("coverage_pct = %v, want 66.7;\nstderr:\n%s", pct, stderr)
		}
	})

	t.Run("spec-coverage/AC-44 the coverage table prints 66% for two of three criteria", func(t *testing.T) {
		dir := twoOfThreeWorkspace(t, "")
		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation")
		fields := coverageTableRowFields(t, stdout, "demo-spec")
		if !rowHasField(fields, "66%") {
			t.Errorf("the demo-spec table row must carry the cell `66%%`, got %v;\nstdout:\n%s\nstderr:\n%s", fields, stdout, stderr)
		}
	})

	t.Run("spec-coverage/AC-44 explain prints the same integer as the table", func(t *testing.T) {
		dir := twoOfThreeWorkspace(t, "")
		stdout, stderr, _ := runCLISplit(t, dir, "explain", "demo-spec")
		if !strings.Contains(stdout, "Coverage: 66% (2/3 ACs)") {
			t.Errorf("explain must print `Coverage: 66%% (2/3 ACs)`;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-45 -- the printed integer is an integer that passes
// ---------------------------------------------------------------------------

// The two cases differ in the declared threshold and in nothing else: same
// spec id, same tier, same criteria, same annotations. 66 is the integer the
// table prints on this workspace and 67 is one above it, so the pair brackets
// the boundary from both sides. The failing case is the control for the
// passing one, because a gate that never fails would pass case A vacuously.
//
// @ac AC-45
func TestCoveragePercent_PrintedIntegerPassesAsThreshold(t *testing.T) {
	cases := []struct {
		name      string
		threshold string
		wantPass  bool
		wantCode  int
	}{
		{"spec-coverage/AC-45 a declared threshold of 66 passes on a two-of-three spec", "66", true, 0},
		{"spec-coverage/AC-45 a declared threshold of 67 fails on a two-of-three spec", "67", false, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := twoOfThreeWorkspace(t, c.threshold)
			stdout, stderr, code := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
			entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")
			if got := boolField(t, entry, "passes_threshold"); got != c.wantPass {
				t.Errorf("passes_threshold = %v with coverage_threshold %s, want %v", got, c.threshold, c.wantPass)
			}
			if code != c.wantCode {
				t.Errorf("exit code = %d with coverage_threshold %s, want %d;\nstdout:\n%s\nstderr:\n%s",
					code, c.threshold, c.wantCode, stdout, stderr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-46 -- the display floors and does not round
// ---------------------------------------------------------------------------

// 7 of 9 and 5 of 8 test opposite failure modes of a rounding formatter. 77.8
// rounds up to 78 and floors to 77, so the first case fails any formatter
// built on `%.0f`. 62.5 is an exact tie, so the second case fixes the
// direction a round-half-up formatter would take. The two ratios also differ
// in denominator and in numerator, so neither case is derivable from the
// other.
//
// @ac AC-46
func TestCoveragePercent_DisplayFloors(t *testing.T) {
	cases := []struct {
		name          string
		acIDs         []string
		annotated     []string
		wantPct       float64
		wantTableCell string
	}{
		{
			name:          "spec-coverage/AC-46 seven of nine criteria report 77.8 and print 77%",
			acIDs:         []string{"AC-01", "AC-02", "AC-03", "AC-04", "AC-05", "AC-06", "AC-07", "AC-08", "AC-09"},
			annotated:     []string{"AC-01", "AC-02", "AC-03", "AC-04", "AC-05", "AC-06", "AC-07"},
			wantPct:       77.8,
			wantTableCell: "77%",
		},
		{
			name:          "spec-coverage/AC-46 five of eight criteria report 62.5 and print 62%",
			acIDs:         []string{"AC-01", "AC-02", "AC-03", "AC-04", "AC-05", "AC-06", "AC-07", "AC-08"},
			annotated:     []string{"AC-01", "AC-02", "AC-03", "AC-04", "AC-05"},
			wantPct:       62.5,
			wantTableCell: "62%",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: c.acIDs})
			writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", c.annotated)

			jsonOut, _, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
			entry := coverageEntry(t, decodeCoverageJSON(t, jsonOut), "demo-spec")
			if pct := floatField(t, entry, "coverage_pct"); !nearly(pct, c.wantPct) {
				t.Errorf("coverage_pct = %v, want %v", pct, c.wantPct)
			}

			textOut, textErr, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation")
			fields := coverageTableRowFields(t, textOut, "demo-spec")
			if !rowHasField(fields, c.wantTableCell) {
				t.Errorf("the demo-spec table row must carry the cell %q, got %v;\nstdout:\n%s\nstderr:\n%s",
					c.wantTableCell, fields, textOut, textErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-48 -- the summary header never disagrees with the column below it
// ---------------------------------------------------------------------------

// The single-spec case is the discriminator: 66.7 reads 66 in the header
// under a flooring formatter and 67 under a rounding one, and the criterion
// asserts the header and the cell below it read the same. The two-spec case
// aggregates, which is why it is here at all: it proves the header averages
// the entries rather than reporting the first one.
//
// @ac AC-48
func TestCoverageSummaryHeader_AgreesWithTheColumnBelow(t *testing.T) {
	t.Run("spec-coverage/AC-48 the header of a one-spec report reads the cell below it", func(t *testing.T) {
		dir := twoOfThreeWorkspace(t, "")
		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation")
		const want = "Spec Coverage Report — 1 specs · 66% avg coverage"
		if !strings.Contains(stdout, want) {
			t.Errorf("summary header must read %q;\nstdout:\n%s\nstderr:\n%s", want, stdout, stderr)
		}
		fields := coverageTableRowFields(t, stdout, "demo-spec")
		if !rowHasField(fields, "66%") {
			t.Errorf("the demo-spec table row must carry the cell `66%%`, got %v", fields)
		}
	})

	t.Run("spec-coverage/AC-48 the header of a two-spec report averages the entries", func(t *testing.T) {
		dir := t.TempDir()
		// alpha-spec is the 66.7 entry, beta-spec the 100 entry. They differ
		// in criterion count as well as in how many are annotated, so the
		// mean cannot be reproduced by counting criteria across the report.
		writeDefectSpec(t, dir, defectSpec{id: "alpha-spec", tier: 2, acIDs: []string{"AC-01", "AC-02", "AC-03"}})
		writeAnnotatedTestFile(t, dir, "tests/alpha_test.go", "alpha-spec", []string{"AC-01", "AC-02"})
		writeDefectSpec(t, dir, defectSpec{id: "beta-spec", tier: 2, acIDs: []string{"AC-01", "AC-02"}})
		writeAnnotatedTestFile(t, dir, "tests/beta_test.go", "beta-spec", []string{"AC-01", "AC-02"})

		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--strictness", "annotation")
		const want = "Spec Coverage Report — 2 specs · 83% avg coverage"
		if !strings.Contains(stdout, want) {
			t.Errorf("summary header must read %q;\nstdout:\n%s\nstderr:\n%s", want, stdout, stderr)
		}
	})
}

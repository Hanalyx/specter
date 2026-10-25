// coverage_percent_surfaces_test.go -- CLI integration tests for
// spec-coverage 1.17.0 C-35, the three percentage surfaces C-35(b) names that
// 1.16.0 left unpinned, plus the summary header rule of C-35(e)
// (AC-56 through AC-59).
//
// C-35 binds five sites that print or emit a percentage. The 1.16.0 criteria
// pinned the coverage table and the JSON field, so `explain`, doctor's
// per-spec line, and the approval-gate demotion could each be reverted and
// ship green (`bugs/SP-SP-041`). Each criterion here takes one of those three,
// at a ratio where the wrong formatter and the right one print different
// integers. AC-59 covers the header, which is not a per-spec value and has its
// own rule (`bugs/SP-SP-040`).
//
// Every criterion here names a process surface: a printed line, a serialized
// field, or an exit code. None is observable from the pure functions, so all
// of them run the binary.
//
// @spec spec-coverage
package main

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The AC-56 workspace, shared with AC-57
// ---------------------------------------------------------------------------

// wideAndNineWorkspace builds the workspace AC-56 and AC-57 share: two Tier 1
// specs, one covered 17 of 21 and one covered 7 of 9, each annotated in its
// own test file. The criterion names the two spec file paths, so they are
// written at those paths rather than at the id-derived default.
//
// The two ratios are here because each kills a different wrong formatter, and
// neither kills the other's. 17 of 21 is 80.952, which stores as 81.0 and
// floors to 81; the integer division this surface used before 1.16.0 gives 80.
// A formatter that rounds the stored value also gives 81, so that ratio alone
// says nothing about rounding. 7 of 9 is 77.777, which stores as 77.8 and
// floors to 77, while a rounding formatter gives 78; integer division also
// gives 77 there. Together the pair admits only the floor of the stored value.
//
// The workspace carries no specter.yaml and no .specter-results.json, so
// `explain`, `coverage` and `doctor` all read the same structural input. What
// varies between a passing and a failing run is the formatter, not the input.
func wideAndNineWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeRawFile(t, dir, "specs/wide.spec.yaml",
		defectSpecYAML(defectSpec{id: "wide-spec", tier: 1, acIDs: acRange(1, 21)}))
	writeAnnotatedTestFile(t, dir, "tests/wide_test.go", "wide-spec", acRange(1, 17))

	writeRawFile(t, dir, "specs/nine.spec.yaml",
		defectSpecYAML(defectSpec{id: "nine-spec", tier: 1, acIDs: acRange(1, 9)}))
	writeAnnotatedTestFile(t, dir, "tests/nine_test.go", "nine-spec", acRange(1, 7))

	return dir
}

// acRange returns AC-01 through AC-NN for the given inclusive bounds. The
// fixtures here declare up to 21 criteria, which is past the point where
// writing the list out is readable.
func acRange(first, last int) []string {
	ids := make([]string, 0, last-first+1)
	for i := first; i <= last; i++ {
		ids = append(ids, fmt.Sprintf("AC-%02d", i))
	}
	return ids
}

// ---------------------------------------------------------------------------
// AC-56 -- `explain` calls the shared formatter
// ---------------------------------------------------------------------------

// The table cells are read from a separate run of the same workspace, so
// `explain` cannot pass by reading a value another surface computed. The
// criterion's claim is that the two surfaces agree because both call the
// shared formatter, and the pair of ratios is what rules out the two ways they
// could agree by accident.
//
// @ac AC-56
func TestCoveragePercent_ExplainUsesTheSharedFormatter(t *testing.T) {
	t.Run("spec-coverage/AC-56 explain prints 81% for seventeen of twenty-one criteria", func(t *testing.T) {
		dir := wideAndNineWorkspace(t)
		stdout, stderr, code := runCLISplit(t, dir, "explain", "wide-spec")
		if !strings.Contains(stdout, "Coverage: 81% (17/21 ACs)") {
			t.Errorf("explain must print `Coverage: 81%% (17/21 ACs)`;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})

	t.Run("spec-coverage/AC-56 explain prints 77% for seven of nine criteria", func(t *testing.T) {
		dir := wideAndNineWorkspace(t)
		stdout, stderr, code := runCLISplit(t, dir, "explain", "nine-spec")
		if !strings.Contains(stdout, "Coverage: 77% (7/9 ACs)") {
			t.Errorf("explain must print `Coverage: 77%% (7/9 ACs)`;\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})

	t.Run("spec-coverage/AC-56 the coverage table prints the same two integers", func(t *testing.T) {
		dir := wideAndNineWorkspace(t)
		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--strictness", "annotation")

		for _, c := range []struct{ specID, cell string }{
			{"wide-spec", "81%"},
			{"nine-spec", "77%"},
		} {
			fields := coverageTableRowFields(t, stdout, c.specID)
			if !rowHasField(fields, c.cell) {
				t.Errorf("the %s table row must carry the cell %q, got %v;\nstdout:\n%s\nstderr:\n%s",
					c.specID, c.cell, fields, stdout, stderr)
			}
		}
		if code != 1 {
			t.Errorf("exit code = %d, want 1;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-57 -- doctor's per-spec line calls the shared formatter
// ---------------------------------------------------------------------------

// doctor exits 1 for several reasons, so the exit code is not what makes this
// case discriminating. The two exact lines are: they print only beneath a
// failing coverage check, so a doctor run that exited 1 for a parse failure or
// a missing spec directory would not produce them. The two ratios do the same
// two jobs they do in AC-56. The `%.0f` formatter this line used before 1.16.0
// prints 78% for nine-spec, and a line dividing covered by total itself prints
// 80% for wide-spec.
//
// The criterion makes no claim about which strictness doctor applies
// (`bugs/SP-SP-038`) and none about the manifest health line, so neither is
// asserted here.
//
// @ac AC-57
func TestCoveragePercent_DoctorPerSpecLineUsesTheSharedFormatter(t *testing.T) {
	t.Run("spec-coverage/AC-57 doctor prints the shared formatter's integer beneath a failing coverage check", func(t *testing.T) {
		dir := wideAndNineWorkspace(t)
		stdout, stderr, code := runCLISplit(t, dir, "doctor")

		for _, want := range []string{
			"    nine-spec: 77% coverage (T1 requires 100%)",
			"    wide-spec: 81% coverage (T1 requires 100%)",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("doctor stdout must carry %q;\nstdout:\n%s\nstderr:\n%s", want, stdout, stderr)
			}
		}
		if code != 1 {
			t.Errorf("exit code = %d, want 1;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-58 -- the approval-gate demotion emits a stored value
// ---------------------------------------------------------------------------

// gateSpecYAML is written out rather than built from defectSpec because
// approval_gate is the field this case turns on, and defectSpec does not carry
// it. AC-02 and AC-03 declare the gate with no approval_date; AC-01 does not,
// so the demotion has something to leave behind.
const gateSpecYAML = `spec:
  id: gate-spec
  version: "1.0.0"
  status: approved
  tier: 3

  context:
    system: gate-spec control system
    feature: gate-spec approval gate fixture

  objective:
    summary: Fixture workspace for the approval-gate demotion recompute.

  constraints:
    - id: C-01
      description: "MUST behave as the fixture describes"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Fixture criterion AC-01"
      references_constraints: ["C-01"]
      priority: high
    - id: AC-02
      description: "Fixture criterion AC-02"
      references_constraints: ["C-01"]
      priority: high
      approval_gate: true
    - id: AC-03
      description: "Fixture criterion AC-03"
      references_constraints: ["C-01"]
      priority: high
      approval_gate: true
`

// All three criteria are annotated and all three carry a passing result, so
// the entry reads 3 of 3 before the demotion. Everything the case asserts is
// therefore produced by the demotion path rather than by the ordinary count,
// which is what makes it a test of the recompute.
//
// 1 of 3 is the ratio because its exact quotient is not representable in
// float64: a recompute doing its own division emits 33.333333333333336, and
// the criterion requires 33.3.
//
// The decoded value and the serialized text are both asserted because they
// catch different wrong answers. The decoded comparison catches a wrong
// magnitude, which is what a naive division produces: 33.333333333333336 is
// 0.033 away from 33.3 and fails the tolerance. The raw-stdout check catches
// the right magnitude serialized long, such as 33.300000000000004, which
// passes at 1e-9 and is not the field the criterion states. The trailing comma
// in the wanted string is load-bearing: without it the match is a prefix of
// `33.333333333333336` and admits the value it exists to forbid.
//
// @ac AC-58
func TestCoveragePercent_ApprovalGateDemotionEmitsAStoredValue(t *testing.T) {
	t.Run("spec-coverage/AC-58 the demoted entry reports 33.3 for one of three criteria", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "gate.spec.yaml", gateSpecYAML)
		writeAnnotatedTestFile(t, dir, "tests/gate_test.go", "gate-spec", []string{"AC-01", "AC-02", "AC-03"})
		writeResultsJSON(t, dir, `{"results":[{"spec_id":"gate-spec","ac_id":"AC-01","status":"passed"},{"spec_id":"gate-spec","ac_id":"AC-02","status":"passed"},{"spec_id":"gate-spec","ac_id":"AC-03","status":"passed"}]}`)

		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--json", "--strictness", "zero-tolerance")
		entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "gate-spec")

		if pct := floatField(t, entry, "coverage_pct"); !nearly(pct, 33.3) {
			t.Errorf("coverage_pct = %v, want 33.3;\nstderr:\n%s", pct, stderr)
		}
		if !strings.Contains(stdout, `"coverage_pct": 33.3,`) {
			t.Errorf("the serialized field must read `\"coverage_pct\": 33.3`;\nstdout:\n%s", stdout)
		}
		if got := strings.Join(stringsField(t, entry, "covered_acs"), ","); got != "AC-01" {
			t.Errorf("covered_acs = [%s], want [AC-01]", got)
		}
		if got := strings.Join(stringsField(t, entry, "uncovered_acs"), ","); got != "AC-02,AC-03" {
			t.Errorf("uncovered_acs = [%s], want [AC-02 AC-03]", got)
		}
		if got := floatField(t, entry, "threshold"); !nearly(got, 50) {
			t.Errorf("threshold = %v, want 50", got)
		}
		if boolField(t, entry, "passes_threshold") {
			t.Errorf("passes_threshold = true, want false")
		}
		if code != 3 {
			t.Errorf("exit code = %d, want 3;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-59 -- the summary header floors the exact mean
// ---------------------------------------------------------------------------

// The three specs store 33.3, 44.4 and 33.3, whose exact mean is 37.0.
// Accumulating them left to right in float64 reaches 110.99999999999999, a
// third of which is 36.99999999999999 and floors to 36. That is the value the
// 1.16.0 header prints, so the header assertion is the one thing here expected
// to fail before the fix.
//
// The cells and the exit code are asserted alongside it because a header that
// reads 37 while the column beneath it moved would satisfy C-35(e) and break
// C-35(b). They are the control for the header line, not decoration.
//
// Row order among the three is not asserted. All three sit in one sort bucket
// at one tier, and C-15 does not order within that, so each row is looked up by
// spec id.
//
// @ac AC-59
func TestCoverageSummaryHeader_FloorsTheExactMean(t *testing.T) {
	t.Run("spec-coverage/AC-59 the header of a three-spec report floors the exact mean", func(t *testing.T) {
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{id: "alpha-spec", tier: 3, acIDs: acRange(1, 3), threshold: "30"})
		writeAnnotatedTestFile(t, dir, "tests/alpha_test.go", "alpha-spec", acRange(1, 1))
		writeDefectSpec(t, dir, defectSpec{id: "beta-spec", tier: 3, acIDs: acRange(1, 9), threshold: "30"})
		writeAnnotatedTestFile(t, dir, "tests/beta_test.go", "beta-spec", acRange(1, 4))
		writeDefectSpec(t, dir, defectSpec{id: "gamma-spec", tier: 3, acIDs: acRange(1, 3), threshold: "30"})
		writeAnnotatedTestFile(t, dir, "tests/gamma_test.go", "gamma-spec", acRange(1, 1))

		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--strictness", "annotation")

		const wantHeader = "Spec Coverage Report — 3 specs · 37% avg coverage"
		if !strings.Contains(stdout, wantHeader) {
			t.Errorf("summary header must read %q;\nstdout:\n%s\nstderr:\n%s", wantHeader, stdout, stderr)
		}
		const wantTier = "  Tier 3: 3/3 passing (100%)"
		if !strings.Contains(stdout, wantTier) {
			t.Errorf("tier line must read %q;\nstdout:\n%s", wantTier, stdout)
		}

		for _, c := range []struct{ specID, cell string }{
			{"alpha-spec", "33%"},
			{"beta-spec", "44%"},
			{"gamma-spec", "33%"},
		} {
			fields := coverageTableRowFields(t, stdout, c.specID)
			if !rowHasField(fields, c.cell) {
				t.Errorf("the %s table row must carry the cell %q, got %v;\nstdout:\n%s",
					c.specID, c.cell, fields, stdout)
			}
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

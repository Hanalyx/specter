// coverage_percent_test.go -- unit tests for spec-coverage 1.16.0 C-35, the
// percentage contract (AC-47).
//
// AC-47 states its inputs as raw (covered, total) pairs, including 3000
// criteria and an empty spec, so it is a statement about the two pure
// functions rather than about a workspace. The surfaces that consume them
// (the table, the summary header, `explain`, the threshold gate) are covered
// by AC-44 through AC-46 and AC-48 in cmd/specter.
//
// @spec spec-coverage
package coverage

import (
	"math"
	"testing"
)

// closeEnough compares a percentage against the value AC-47 names. The
// tolerance keeps the comparison about the stated value rather than about
// float64 representation. NaN fails it, which is the "not NaN" half of the
// empty case.
func closeEnough(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// The three cases vary independently. The incomplete case is one criterion
// short of complete out of 3000, which is the case a plain round to one
// decimal reports as 100.0; the complete case is the only case allowed to
// report 100.0; the empty case has a zero denominator. No case is derivable
// from another.
//
// @ac AC-47
func TestCoveragePercent_NeverReports100ForAnIncompleteSpec(t *testing.T) {
	t.Run("spec-coverage/AC-47 2999 of 3000 reports 99.9 and formats as 99%", func(t *testing.T) {
		pct := CoveragePercent(2999, 3000)
		if !closeEnough(pct, 99.9) {
			t.Errorf("CoveragePercent(2999, 3000) = %v, want 99.9", pct)
		}
		if got := FormatCoveragePct(pct); got != "99%" {
			t.Errorf("FormatCoveragePct(%v) = %q, want \"99%%\"", pct, got)
		}
	})

	t.Run("spec-coverage/AC-47 3000 of 3000 reports 100.0 and formats as 100%", func(t *testing.T) {
		pct := CoveragePercent(3000, 3000)
		if !closeEnough(pct, 100.0) {
			t.Errorf("CoveragePercent(3000, 3000) = %v, want 100.0", pct)
		}
		if got := FormatCoveragePct(pct); got != "100%" {
			t.Errorf("FormatCoveragePct(%v) = %q, want \"100%%\"", pct, got)
		}
	})

	t.Run("spec-coverage/AC-47 0 of 0 reports 0 rather than NaN", func(t *testing.T) {
		pct := CoveragePercent(0, 0)
		if !closeEnough(pct, 0) {
			t.Errorf("CoveragePercent(0, 0) = %v, want 0", pct)
		}
	})
}

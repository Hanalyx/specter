// Tests for DiffCoverageReports — spec-diff C-10/C-11 / AC-13/AC-14.
//
// @spec spec-diff
package diff

import (
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

// Helper: build a minimal CoverageReport from a list of specs with
// their covered/uncovered AC sets.
func makeCoverageReport(specs ...coverage.SpecCoverageEntry) coverage.CoverageReport {
	return coverage.CoverageReport{Entries: specs}
}

func specEntry(id string, covered, uncovered []string, pct float64, passes bool) coverage.SpecCoverageEntry {
	return coverage.SpecCoverageEntry{
		SpecID:          id,
		Tier:            2,
		TotalACs:        len(covered) + len(uncovered),
		CoveredACs:      covered,
		UncoveredACs:    uncovered,
		CoveragePct:     pct,
		PassesThreshold: passes,
	}
}

// AC-14: identical reports → empty diff.
//
// @ac AC-14
func TestDiffCoverageReports_Identical_EmptyDiff(t *testing.T) {
	t.Run("spec-diff/AC-14 identical CoverageReports return empty diff", func(t *testing.T) {
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, nil, 100.0, true),
			specEntry("spec-bar", []string{"AC-01"}, []string{"AC-02"}, 50.0, false),
		)
		current := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, nil, 100.0, true),
			specEntry("spec-bar", []string{"AC-01"}, []string{"AC-02"}, 50.0, false),
		)
		diff := DiffCoverageReports(baseline, current)
		if !diff.IsEmpty() {
			t.Errorf("expected empty diff for identical reports, got %+v", diff)
		}
	})
}

// AC-13: GAINED / LOST / spec-added / pct-changed surface correctly.
//
// @ac AC-13
func TestDiffCoverageReports_PerSpecDelta(t *testing.T) {
	t.Run("spec-diff/AC-13 GAINED AC surfaces in per-spec change", func(t *testing.T) {
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, []string{"AC-02"}, 50.0, false),
		)
		current := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, nil, 100.0, true),
		)
		diff := DiffCoverageReports(baseline, current)
		if len(diff.SpecChanges) != 1 {
			t.Fatalf("expected 1 SpecChange, got %d: %+v", len(diff.SpecChanges), diff.SpecChanges)
		}
		ch := diff.SpecChanges[0]
		if ch.SpecID != "spec-foo" {
			t.Errorf("expected SpecID=spec-foo, got %q", ch.SpecID)
		}
		if len(ch.GainedACs) != 1 || ch.GainedACs[0] != "AC-02" {
			t.Errorf("expected GainedACs=[AC-02], got %v", ch.GainedACs)
		}
		if len(ch.LostACs) != 0 {
			t.Errorf("expected LostACs empty, got %v", ch.LostACs)
		}
		if ch.BaselineCoveragePct != 50.0 || ch.CurrentCoveragePct != 100.0 {
			t.Errorf("expected coverage_pct 50.0 → 100.0, got %v → %v",
				ch.BaselineCoveragePct, ch.CurrentCoveragePct)
		}
		if ch.BaselinePassesThreshold || !ch.CurrentPassesThreshold {
			t.Errorf("expected passes_threshold false → true, got %v → %v",
				ch.BaselinePassesThreshold, ch.CurrentPassesThreshold)
		}
	})

	t.Run("spec-diff/AC-13 LOST AC surfaces in per-spec change", func(t *testing.T) {
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, nil, 100.0, true),
		)
		current := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, []string{"AC-02"}, 50.0, false),
		)
		diff := DiffCoverageReports(baseline, current)
		if len(diff.SpecChanges) != 1 {
			t.Fatalf("expected 1 SpecChange, got %d", len(diff.SpecChanges))
		}
		if got := diff.SpecChanges[0].LostACs; len(got) != 1 || got[0] != "AC-02" {
			t.Errorf("expected LostACs=[AC-02], got %v", got)
		}
	})

	t.Run("spec-diff/AC-13 spec added in current surfaces in SpecsAdded", func(t *testing.T) {
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, nil, 100.0, true),
		)
		current := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, nil, 100.0, true),
			specEntry("spec-bar", []string{"AC-01"}, nil, 100.0, true),
		)
		diff := DiffCoverageReports(baseline, current)
		if len(diff.SpecsAdded) != 1 || diff.SpecsAdded[0] != "spec-bar" {
			t.Errorf("expected SpecsAdded=[spec-bar], got %v", diff.SpecsAdded)
		}
		if len(diff.SpecsRemoved) != 0 {
			t.Errorf("expected SpecsRemoved empty, got %v", diff.SpecsRemoved)
		}
	})

	t.Run("spec-diff/AC-13 spec removed from current surfaces in SpecsRemoved", func(t *testing.T) {
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, nil, 100.0, true),
			specEntry("spec-stale", []string{"AC-01"}, nil, 100.0, true),
		)
		current := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01"}, nil, 100.0, true),
		)
		diff := DiffCoverageReports(baseline, current)
		if len(diff.SpecsRemoved) != 1 || diff.SpecsRemoved[0] != "spec-stale" {
			t.Errorf("expected SpecsRemoved=[spec-stale], got %v", diff.SpecsRemoved)
		}
	})

	t.Run("spec-diff/AC-13 coverage_pct change alone surfaces (no AC delta)", func(t *testing.T) {
		// Same ACs covered in both reports, but coverage_pct changed
		// (could happen if total_acs changed, e.g., spec added new ACs
		// that are uncovered).
		baseline := makeCoverageReport(
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, nil, 100.0, true),
		)
		current := makeCoverageReport(
			// Same covered ACs as baseline + new uncovered ACs.
			specEntry("spec-foo", []string{"AC-01", "AC-02"}, []string{"AC-03"}, 66.7, false),
		)
		diff := DiffCoverageReports(baseline, current)
		if len(diff.SpecChanges) != 1 {
			t.Fatalf("expected 1 SpecChange (pct dropped), got %d", len(diff.SpecChanges))
		}
		ch := diff.SpecChanges[0]
		if len(ch.GainedACs) != 0 || len(ch.LostACs) != 0 {
			t.Errorf("expected no AC delta, got gained=%v lost=%v", ch.GainedACs, ch.LostACs)
		}
		if ch.BaselineCoveragePct == ch.CurrentCoveragePct {
			t.Errorf("expected coverage_pct change to surface, got %v == %v",
				ch.BaselineCoveragePct, ch.CurrentCoveragePct)
		}
	})
}

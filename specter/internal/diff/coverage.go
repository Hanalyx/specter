// Coverage report diff — `specter diff coverage <baseline.json> <current.json>`.
//
// Pure function over two `coverage.CoverageReport` snapshots. Used by
// the polymorphic `specter diff coverage` subcommand (spec-diff C-10,
// C-11) to surface AC coverage drift between CI runs.
//
// Three kinds of change:
//   - SpecsAdded:   spec present in current, absent in baseline
//   - SpecsRemoved: spec present in baseline, absent in current
//   - SpecChanges:  per-spec AC delta (GAINED / LOST) plus
//                   coverage_pct and passes_threshold transitions
//
// @spec spec-diff
package diff

import (
	"sort"

	"github.com/Hanalyx/specter/internal/coverage"
)

// CoverageDiff captures the per-spec delta between two CoverageReports.
type CoverageDiff struct {
	SpecsAdded   []string             `json:"specs_added"`
	SpecsRemoved []string             `json:"specs_removed"`
	SpecChanges  []SpecCoverageChange `json:"spec_changes"`
}

// IsEmpty reports whether the diff has no changes — identical
// CoverageReports.
func (d *CoverageDiff) IsEmpty() bool {
	return len(d.SpecsAdded) == 0 && len(d.SpecsRemoved) == 0 && len(d.SpecChanges) == 0
}

// SpecCoverageChange captures the delta for one spec present in both
// reports.
type SpecCoverageChange struct {
	SpecID                  string   `json:"spec_id"`
	GainedACs               []string `json:"gained_acs"`
	LostACs                 []string `json:"lost_acs"`
	BaselineCoveragePct     float64  `json:"baseline_coverage_pct"`
	CurrentCoveragePct      float64  `json:"current_coverage_pct"`
	BaselinePassesThreshold bool     `json:"baseline_passes_threshold"`
	CurrentPassesThreshold  bool     `json:"current_passes_threshold"`
}

// HasDelta reports whether the per-spec change is non-trivial. A spec
// with no gained ACs, no lost ACs, and unchanged coverage_pct +
// passes_threshold is omitted from CoverageDiff.SpecChanges.
func (s *SpecCoverageChange) HasDelta() bool {
	return len(s.GainedACs) > 0 ||
		len(s.LostACs) > 0 ||
		s.BaselineCoveragePct != s.CurrentCoveragePct ||
		s.BaselinePassesThreshold != s.CurrentPassesThreshold
}

// DiffCoverageReports returns the per-spec coverage delta between
// baseline and current. Pure function — no I/O, no subprocess calls.
//
// Stub: real implementation lands in v0.13 C3 commit 3/3. Returning
// an empty diff here lets the test file compile while keeping every
// AC-13 / AC-14 assertion red — the contract gap the SDD cycle
// expects between commit 2 (test) and commit 3 (implementation).
func DiffCoverageReports(baseline, current coverage.CoverageReport) *CoverageDiff {
	_ = baseline
	_ = current
	return &CoverageDiff{}
}

// Use sort to keep emitted slices deterministic — even from the stub.
var _ = sort.Strings

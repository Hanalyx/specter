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
//     coverage_pct and passes_threshold transitions
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
// Algorithm:
//  1. Index baseline entries by spec_id.
//  2. Walk current entries, classifying each as either a SpecChange
//     (present in both) or a SpecAdded (present only in current).
//  3. After the walk, any baseline entry whose spec_id wasn't seen
//     during the walk is a SpecRemoved.
//  4. For each in-both spec, compute set differences on CoveredACs
//     to derive GainedACs and LostACs. Capture coverage_pct and
//     passes_threshold from both sides for downstream reporting.
//  5. Omit no-op SpecChange entries (a spec where the entry is
//     bit-identical between reports contributes nothing).
//
// Slices in the output are sorted lexicographically for deterministic
// output — important when this is piped into a `git diff`-style review
// or compared in tests.
func DiffCoverageReports(baseline, current coverage.CoverageReport) *CoverageDiff {
	out := &CoverageDiff{}

	baselineByID := make(map[string]*coverage.SpecCoverageEntry, len(baseline.Entries))
	for i := range baseline.Entries {
		e := &baseline.Entries[i]
		baselineByID[e.SpecID] = e
	}

	seen := make(map[string]bool, len(current.Entries))
	for i := range current.Entries {
		cur := &current.Entries[i]
		seen[cur.SpecID] = true

		base, inBaseline := baselineByID[cur.SpecID]
		if !inBaseline {
			out.SpecsAdded = append(out.SpecsAdded, cur.SpecID)
			continue
		}

		change := SpecCoverageChange{
			SpecID:                  cur.SpecID,
			BaselineCoveragePct:     base.CoveragePct,
			CurrentCoveragePct:      cur.CoveragePct,
			BaselinePassesThreshold: base.PassesThreshold,
			CurrentPassesThreshold:  cur.PassesThreshold,
			GainedACs:               setDifference(cur.CoveredACs, base.CoveredACs),
			LostACs:                 setDifference(base.CoveredACs, cur.CoveredACs),
		}
		if change.HasDelta() {
			out.SpecChanges = append(out.SpecChanges, change)
		}
	}

	// Specs removed from current: present in baseline, not seen.
	for id := range baselineByID {
		if !seen[id] {
			out.SpecsRemoved = append(out.SpecsRemoved, id)
		}
	}

	sort.Strings(out.SpecsAdded)
	sort.Strings(out.SpecsRemoved)
	sort.Slice(out.SpecChanges, func(i, j int) bool {
		return out.SpecChanges[i].SpecID < out.SpecChanges[j].SpecID
	})
	return out
}

// setDifference returns elements in a that are not in b, sorted.
// O(N+M) via a hash set on b.
func setDifference(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	bSet := make(map[string]bool, len(b))
	for _, x := range b {
		bSet[x] = true
	}
	var out []string
	for _, x := range a {
		if !bSet[x] {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

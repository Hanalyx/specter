// zero_tolerance.go -- shared zero-tolerance enforcement primitives.
//
// `coverage` (spec-coverage C-25/C-26) and `sync` (spec-sync C-09) enforce
// the same two zero-tolerance gates with the same exit codes. Keeping the
// counting and report-demotion logic here — pure functions, no I/O — is what
// makes the parity mechanical rather than a review-time promise.
//
// @spec spec-coverage
package coverage

import "github.com/Hanalyx/specter/internal/schema"

// CountNonPassed returns the number of results-file entries whose status is
// set and not "passed". Under zero-tolerance, a non-zero count fails the run
// with exit code 2 (spec-coverage C-25, spec-sync C-09). Nil results count
// as zero — the missing-results case is handled earlier by the strict path
// (ErrMissingResults).
func CountNonPassed(results *ResultsFile) int {
	if results == nil {
		return 0
	}
	nonPassed := 0
	for _, r := range results.Results {
		if r.Status != "" && r.Status != "passed" {
			nonPassed++
		}
	}
	return nonPassed
}

// CountApprovalGateViolations returns the number of ACs carrying
// approval_gate: true with an unset approval_date. Under zero-tolerance, a
// non-zero count fails the run with exit code 3 (spec-coverage C-26,
// spec-sync C-09).
func CountApprovalGateViolations(specs []schema.SpecAST) int {
	violations := 0
	for i := range specs {
		for _, ac := range specs[i].AcceptanceCriteria {
			if ac.ApprovalGate && ac.ApprovalDate == "" {
				violations++
			}
		}
	}
	return violations
}

// DemoteApprovalGateViolations is the v0.11.1 fix for GH #94. Under
// strictness=zero-tolerance, an AC with approval_gate: true and unset
// approval_date is a demotion — it must show up in the report as
// uncovered, not just trigger the exit-3 code path. Walks the report
// in place: moves violating ACs from CoveredACs to UncoveredACs,
// recomputes per-entry CoveragePct + PassesThreshold, and recomputes
// Summary.Passing / Summary.Failing.
//
// v0.11.0 emitted the exit code but left the report identical to
// threshold mode — operator-visible report stayed PASS while the run
// exited 3. This function aligns the report with the exit signal.
// Moved from cmd/specter in spec-sync 1.4.0 (C-09) so sync's coverage
// phase demotes identically (spec-sync AC-13).
func DemoteApprovalGateViolations(report *CoverageReport, specs []schema.SpecAST) {
	// Build (specID → set of AC IDs to demote) from the spec AST.
	violations := make(map[string]map[string]bool)
	for i := range specs {
		s := &specs[i]
		var demoted map[string]bool
		for _, ac := range s.AcceptanceCriteria {
			if ac.ApprovalGate && ac.ApprovalDate == "" {
				if demoted == nil {
					demoted = make(map[string]bool)
				}
				demoted[ac.ID] = true
			}
		}
		if demoted != nil {
			violations[s.ID] = demoted
		}
	}
	if len(violations) == 0 {
		return
	}

	// Walk report entries and demote.
	report.Summary.Passing = 0
	report.Summary.Failing = 0
	for i := range report.Entries {
		e := &report.Entries[i]
		demoted := violations[e.SpecID]
		if demoted == nil {
			if e.PassesThreshold {
				report.Summary.Passing++
			} else {
				report.Summary.Failing++
			}
			continue
		}
		var keptCovered []string
		for _, acID := range e.CoveredACs {
			if demoted[acID] {
				e.UncoveredACs = append(e.UncoveredACs, acID)
				continue
			}
			keptCovered = append(keptCovered, acID)
		}
		e.CoveredACs = keptCovered
		if e.TotalACs > 0 {
			e.CoveragePct = float64(len(e.CoveredACs)) * 100 / float64(e.TotalACs)
		} else {
			e.CoveragePct = 0
		}
		// PassesThreshold uses the per-tier threshold the entry was built with.
		e.PassesThreshold = int(e.CoveragePct) >= e.Threshold
		if e.PassesThreshold {
			report.Summary.Passing++
		} else {
			report.Summary.Failing++
		}
	}
}

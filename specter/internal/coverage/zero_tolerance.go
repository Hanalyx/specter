// zero_tolerance.go -- shared zero-tolerance enforcement primitives.
//
// `coverage` (spec-coverage C-25/C-26) and `sync` (spec-sync C-09) enforce
// the same two zero-tolerance gates with the same exit codes. Keeping the
// counting and report-demotion logic here — pure functions, no I/O — is what
// makes the parity mechanical rather than a review-time promise.
//
// @spec spec-coverage
package coverage

import (
	"fmt"

	"github.com/Hanalyx/specter/internal/schema"
)

// AnnotationRuleVerdict reports how many acceptance criteria across a report
// have no test at all, which is rule 1 of the SSRB-104 model.
//
// It lives here rather than in the CLI so every caller reaches the same answer.
// bugs/SP-SP-058 is what happens otherwise: roadmap item 1D-b wired this
// decision into the coverage command alone, so sync returned a different exit
// code and named a different cause for the same workspace. That is the eighth
// instance of a documented pattern in this repository, and roadmap item 1C
// exists to stop the ninth.
func AnnotationRuleVerdict(report *CoverageReport) int {
	if report == nil {
		return 0
	}
	total := 0
	for _, e := range report.Entries {
		total += len(e.NoTestACs)
	}
	return total
}

// AnnotationRuleMessage returns the stderr line for a rule-1 verdict, so the
// wording cannot drift between callers either. spec-sync C-11 binds the cause
// line and not only the integer.
func AnnotationRuleMessage(total int, permissive bool) string {
	if permissive {
		return fmt.Sprintf("warn: %d acceptance criterion(s) have no test. settings.annotation.permissive is true, so this does not fail the run", total)
	}
	return fmt.Sprintf("error: %d acceptance criterion(s) have no test. The tier threshold does not excuse a missing test; set settings.annotation.permissive: true to warn instead", total)
}

// CountNonPassed returns the number of distinct (spec_id, ac_id) pairs whose
// resolved status is not "passed". Under zero-tolerance, a non-zero count
// fails the run with exit code 2 (spec-coverage C-25, spec-sync C-09). Nil
// results count as zero, because the missing-results case is handled earlier
// by the strict path (ErrMissingResults).
//
// C-34: the count names criteria, not entries, because the message it feeds
// says `N annotated AC(s) did not pass`. Three failing entries for one
// criterion are one criterion. An operator reading a count of 2 for a single
// failing criterion goes looking for a second one that does not exist.
//
// The exit code is unchanged by this, because "at least one non-passed entry"
// and "at least one non-passed pair" are the same condition.
// ResultsFile.InvalidStatuses is deliberately not deduplicated this way: its
// C-30 warning text says entries.
func CountNonPassed(results *ResultsFile) int {
	if results == nil {
		return 0
	}
	nonPassed := make(map[string]bool)
	for _, r := range results.Results {
		if entryStatus(r) == "passed" {
			continue
		}
		nonPassed[r.SpecID+"/"+r.ACID] = true
	}
	return len(nonPassed)
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
		// C-35: the demoted entry is recomputed by the same function that
		// built it, so a demotion cannot produce a percentage the rest of the
		// report could not.
		e.CoveragePct = CoveragePercent(len(e.CoveredACs), e.TotalACs)
		// PassesThreshold uses the per-tier threshold the entry was built with,
		// compared against the stored value per C-35(c).
		e.PassesThreshold = e.CoveragePct >= float64(e.Threshold)
		if e.PassesThreshold {
			report.Summary.Passing++
		} else {
			report.Summary.Failing++
		}
	}
}

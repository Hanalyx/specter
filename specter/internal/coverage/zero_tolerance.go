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

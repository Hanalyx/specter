package coverage

import (
	"fmt"
	"strings"
)

// The coverage exit gates, decided in one place.
//
// Two callers reach this: the `coverage` command and the `sync` command. They
// used to each carry the ordering, the messages, and the codes, which is the
// shape bugs/done/SP-SP-066 and bugs/done/SP-SP-058 both take. A private copy
// of a gate sequence is correct when written and goes stale when a gate is
// added beside it, and nothing fails when it does.
//
// This function is pure. It takes counts and returns violations, so the
// package keeps its no-I/O rule and the callers keep the printing.

// GateInputs is what the gates decide on. Counts, mostly: `sync` has already
// reduced the report, the specs and the results to counts by the time it needs
// a verdict, and passing raw inputs would mean keeping them alive across the
// phase boundary for no gain.
//
// Stream validation is the exception and is passed whole. spec-sync AC-20
// binds the cause line on all four surfaces, and a count cannot name the
// stream a refusal concerns. Both callers take it from the same field of the
// report they already hold, so the two cannot derive it differently. Deriving
// one input two ways is bugs/SP-SP-073.
type GateInputs struct {
	// AnnotationDeclared reports whether settings.annotation is declared.
	// Under it, GoverningStrictness resolves to threshold, so ZeroTolerance
	// and this are mutually exclusive in practice.
	AnnotationDeclared bool
	// AnnotationPermissive decides rule 1's severity and nothing else, per
	// SSRB-104 section 7.4 and spec-coverage C-40(a).
	AnnotationPermissive bool
	// AnnotationRuleViolations counts criteria with no test at all.
	AnnotationRuleViolations int

	// ZeroTolerance reports whether the ladder is in force.
	ZeroTolerance bool
	// ZeroToleranceNonPassed counts annotated criteria whose result is not
	// passed. Meaningful only under the ladder.
	ZeroToleranceNonPassed int

	// ApprovalGateViolations counts criteria carrying approval_gate: true
	// with an unset approval_date. It fires under both models, per
	// spec-coverage C-40, and callers must compute it under both.
	ApprovalGateViolations int

	// ThresholdFailing counts specs below their tier threshold.
	ThresholdFailing int

	// StreamValidationErrors carries the C-44 violations the shared builder
	// recorded on the report. Take it from CoverageReport.ResultsValidationErrors
	// and nowhere else.
	StreamValidationErrors []ResultsValidationError
}

// GateViolation is one gate's finding.
type GateViolation struct {
	// Stderr is the complete line the command writes, prefix included. Empty
	// when the gate decides the code without reporting a line, which the tier
	// threshold does. Callers MUST skip an empty one rather than print a blank.
	Stderr string
	// Phase is the same finding worded for a sync PhaseResult, which carries
	// its own prefix conventions and must not change.
	Phase string
	// Code is the exit code this violation decides, or 0 when it reports
	// without deciding. Rule 1 under permissive is the only zero today: it
	// warns and lets a later gate choose the code.
	Code int
}

// GateVerdict returns every violation the inputs produce, in the order
// spec-coverage C-40(d) fixes, and the code the process should return.
//
// Every violation is returned rather than the first, because C-40(e) requires
// both to be named in one run. An operator who fixes the missing test, re-runs,
// and only then learns about an unmet approval gate has paid for a sequencing
// choice nobody made deliberately.
//
// The code is the first violation's non-zero code. Rule 1 before the approval
// gate is ascending-code order and matches the ladder's own precedence, so the
// two models cannot disagree about which integer a workspace produces.
func GateVerdict(in GateInputs) (violations []GateViolation, code int) {
	// Rule 1, ahead of everything. spec-coverage C-38(b): the tier threshold
	// does not excuse a criterion with no test.
	if in.AnnotationDeclared && in.AnnotationRuleViolations > 0 {
		v := GateViolation{
			Stderr: AnnotationRuleMessage(in.AnnotationRuleViolations, in.AnnotationPermissive),
			Phase:  AnnotationRuleMessage(in.AnnotationRuleViolations, in.AnnotationPermissive),
		}
		// C-40(a): permissive decides rule 1's severity and nothing else. A
		// warning still reports; it just does not choose the code.
		if !in.AnnotationPermissive {
			v.Code = 2
		}
		violations = append(violations, v)
	}

	if in.ZeroTolerance && in.ZeroToleranceNonPassed > 0 {
		// Golden value for the propagation parity test, roadmap item 1A4.
		// It must stay identical across every site that emits it.
		msg := fmt.Sprintf("zero-tolerance strictness — %d annotated AC(s) did not pass", in.ZeroToleranceNonPassed)
		violations = append(violations, GateViolation{
			Stderr: "error: " + msg,
			Phase:  msg,
			Code:   2,
		})
	}

	// The approval gate, under both models. spec-coverage C-40(b) gives the
	// annotation model its own wording, because naming zero-tolerance in a run
	// that is not on the ladder would misdirect the operator to a setting they
	// did not choose.
	if in.ApprovalGateViolations > 0 {
		var msg string
		switch {
		case in.ZeroTolerance:
			// The second golden value for 1A4. Also unchanged.
			msg = fmt.Sprintf("zero-tolerance strictness — %d AC(s) carry approval_gate=true with unset approval_date", in.ApprovalGateViolations)
		case in.AnnotationDeclared:
			msg = fmt.Sprintf("%d AC(s) carry approval_gate=true with unset approval_date. An approval gate is a human sign-off, so settings.annotation.permissive does not soften it", in.ApprovalGateViolations)
		}
		if msg != "" {
			violations = append(violations, GateViolation{
				Stderr: "error: " + msg,
				Phase:  msg,
				Code:   3,
			})
		}
	}

	// The tier threshold decides the code and prints nothing, which is why its
	// Stderr is empty. Neither command wrote a stderr line for it before: the
	// detail is in the report body, and `coverage` returned its failure exit
	// silently. An earlier draft gave it a line, which changed `coverage` and
	// not `sync` and so put the two out of step on the cause line that
	// `spec-sync` C-11 binds. Giving it one on both surfaces is a real
	// improvement and needs its own criterion, not a side effect of this one.
	if in.ThresholdFailing > 0 {
		violations = append(violations, GateViolation{
			Phase: fmt.Sprintf("%d spec(s) below coverage threshold", in.ThresholdFailing),
			Code:  1,
		})
	}

	// Stream validation, last. spec-coverage C-44 makes its precedence
	// conditional, and ordering it after every gate that shipped before it is
	// what makes both halves true at once: the loop below returns the first
	// non-zero code, so a coexisting older gate wins and a violation standing
	// alone gets the band's own code. The violations are reported either way,
	// because they are appended whichever gate decides.
	//
	// docs/EXIT_CODES.md section 4 forbids a new gate preempting a shipped
	// one. Appending here is that rule expressed as position rather than as a
	// comparison nobody would re-check.
	if len(in.StreamValidationErrors) > 0 {
		msg := StreamValidationMessage(in.StreamValidationErrors)
		violations = append(violations, GateViolation{
			Stderr: msg,
			Phase:  strings.TrimPrefix(msg, "error: "),
			Code:   20,
		})
	}

	for _, v := range violations {
		if v.Code != 0 {
			return violations, v.Code
		}
	}
	return violations, 0
}

// FirstFailing returns the first violation that decides the code, which is the
// one a report names as the cause. Nil when nothing decided.
func FirstFailing(violations []GateViolation) *GateViolation {
	for i := range violations {
		if violations[i].Code != 0 {
			return &violations[i]
		}
	}
	return nil
}

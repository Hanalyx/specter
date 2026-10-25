package coverage

import "github.com/Hanalyx/specter/internal/schema"

// CriterionVerdict is everything true of one acceptance criterion that bears on
// whether it counts as covered. Built once per criterion and consumed by every
// surface, so no caller can answer the question a second way.
//
// C-39. Before roadmap item 1C the answer lived in two places: a switch inside
// the report-building loop, and a pass that walked the finished report moving
// criteria between its lists. The second could not see the first's invariants,
// which is how a demoted criterion ended up appended out of declaration order
// and how a fully demoted entry marshaled its covered list as null.
//
// The fields are the inputs that exist today. Multi-stream evidence (roadmap
// 3A) and deferral state (3C) belong here when those features land and can
// populate them. Adding them now would put fields in the type that nothing
// writes and nothing reads, which is the pattern this cycle has spent itself
// filing bugs about.
type CriterionVerdict struct {
	SpecID string
	ACID   string
	Tier   int

	// HasAnnotation is true when a source annotation names this criterion.
	// It is rule 1's input and is independent of any result.
	HasAnnotation bool

	// ResultPassed reports whether every matching results entry passed, the
	// C-07 Tier 1 question. ResultStatus carries the worst matching status
	// for the C-19 strict question. They differ: a criterion with no entry
	// at all is not passed and has status "unknown".
	ResultPassed bool
	ResultStatus string

	// ApprovalGateViolation is approval_gate true with an unset approval_date.
	// It demotes only under zero-tolerance, which is a mode question, so the
	// verdict records the fact and Covered decides what it means.
	ApprovalGateViolation bool

	// InScope is true when the strict path applies to this criterion's spec:
	// strict mode is on, and either no --scope was given or its spec is named
	// by the scoped domain. C-23.
	InScope bool
}

// ClassifyMode carries what is true of the run rather than of one criterion.
// One value rather than a set of booleans threaded separately, so a new mode
// dimension cannot reach one surface and miss another.
type ClassifyMode struct {
	// Strict routes classification through the C-19 demand: an annotated
	// criterion needs a passing result at every tier.
	Strict bool

	// ZeroTolerance makes an approval-gate violation demote the criterion,
	// per C-39.
	ZeroTolerance bool

	// ScopedSpecs limits Strict to the named specs. Nil or empty means no
	// restriction, which is the C-23 default.
	ScopedSpecs map[string]bool
}

// Covered is the single answer to whether a criterion counts, and it is a pure
// function of the verdict and the mode. Every ordering, percentage, threshold
// verdict and summary count in a coverage report derives from it.
func (v CriterionVerdict) Covered(mode ClassifyMode) bool {
	// C-39: a gate violation under zero-tolerance is uncovered whatever else
	// is true of it. Checked first because it overrides an otherwise passing
	// criterion, which is the demotion.
	if mode.ZeroTolerance && v.ApprovalGateViolation {
		return false
	}

	switch {
	case v.InScope:
		// C-19: under strict, all tiers need annotation and a passing status.
		return v.HasAnnotation && v.ResultStatus == "passed"
	case v.Tier == 1:
		// C-07: Tier 1 needs annotation and a passing result.
		return v.HasAnnotation && v.ResultPassed
	default:
		// Tier 2 and 3: annotation alone.
		return v.HasAnnotation
	}
}

// buildVerdicts returns one verdict per declared criterion, in declaration
// order, for one spec.
//
// Declaration order is load-bearing: the covered and uncovered lists are built
// by walking this slice, so it decides the order a reader and a JSON consumer
// see. AC-64.
func buildVerdicts(spec schema.SpecAST, annotated map[string]bool, results *ResultsFile, mode ClassifyMode) []CriterionVerdict {
	inScope := mode.Strict && (len(mode.ScopedSpecs) == 0 || mode.ScopedSpecs[spec.ID])

	out := make([]CriterionVerdict, 0, len(spec.AcceptanceCriteria))
	for _, ac := range spec.AcceptanceCriteria {
		v := CriterionVerdict{
			SpecID:                spec.ID,
			ACID:                  ac.ID,
			Tier:                  spec.Tier,
			HasAnnotation:         annotated != nil && annotated[ac.ID],
			ApprovalGateViolation: ac.ApprovalGate && ac.ApprovalDate == "",
			InScope:               inScope,

			// Both lookups are nil-safe and their nil answers are load-bearing,
			// so they are called unconditionally. `passed` returns **true**
			// when no entry matches, which is what lets a Tier 1 criterion
			// count on annotation alone when there is no results file at all
			// (C-07). `status` returns "unknown" in the same case, which the
			// strict path treats as not covered (C-19). Guarding these behind
			// a nil check silently swaps the first answer for false and takes
			// every Tier 1 spec to zero.
			ResultPassed: results.passed(spec.ID, ac.ID),
			ResultStatus: results.status(spec.ID, ac.ID),
		}
		out = append(out, v)
	}
	return out
}

// CountApprovalGateViolationsIn counts gate violations over a verdict list.
// C-39 / 1C3: the guards read the same verdicts classification read, so a
// criterion cannot be a violation for the exit code and not for the report.
func CountApprovalGateViolationsIn(verdicts []CriterionVerdict) int {
	n := 0
	for _, v := range verdicts {
		if v.ApprovalGateViolation {
			n++
		}
	}
	return n
}

// CountMissingAnnotationsIn counts criteria with no annotation over a verdict
// list. This is rule 1 of the annotation model, C-38(a).
func CountMissingAnnotationsIn(verdicts []CriterionVerdict) int {
	n := 0
	for _, v := range verdicts {
		if !v.HasAnnotation {
			n++
		}
	}
	return n
}

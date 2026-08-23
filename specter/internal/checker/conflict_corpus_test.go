// conflict_corpus_test.go -- roadmap 2A3. The three corpora structural-conflict
// detection is measured against, committed as data.
//
// `bugs/doing/SP-SP-004` records why these are the deliverable rather than any
// particular matcher: every pass on that bug measured false positives and true
// positives, reported a rule that looked good, and missed the class that only
// appears once you write down the sentences that must NOT fire.
//
// These tests characterize the SHIPPED behavior, including where it is wrong.
// A run that changes any count fails, which is the point: the numbers move
// deliberately or not at all.
//
// @spec spec-check
package checker

import (
	"testing"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// conflictCase is one upstream constraint against one downstream criterion,
// with the verdict a careful reader gives it.
type conflictCase struct {
	constraint string
	criterion  string
	// conflict is the ground truth: does the criterion actually contradict the
	// constraint? Set by a human, never by the detector.
	conflict bool
	// fires records what the shipped detector does today. Where it disagrees
	// with conflict, the case is a known defect and the note says which.
	fires bool
	note  string
}

// trueConflicts: the criterion genuinely contradicts the constraint.
//
// The first is the only true positive that was committed before 2A3.
var trueConflicts = []conflictCase{
	{"email MUST be required", "Process checkout when email is absent", true, true, "the original suite fixture"},
	{"email MUST be required", "Guest checkout succeeds when email is missing", true, true, "copular, missing"},
	{"email MUST be required", "Order is accepted without email", true, true, "backward, without"},
	{"email MUST be required", "Submit the form when email is not provided", true, true, "copular, negated"},
	{"email MUST be required", "Checkout proceeds when the email is empty", true, true, "article before subject"},
	{"The refresh token MUST be present on every request", "Session renews when the refresh token is absent", true, true, "multi-word subject"},
	{"The refresh token MUST be present on every request", "Renewal succeeds without the refresh token", true, true, "backward, multi-word"},
	{"api_key MUST be supplied by the caller", "Request is served when api_key is missing", true, true, "snake_case subject"},
	{"A tenant id MUST accompany every write", "Write is accepted when a tenant id is not present", true, true, "article, negated"},
	{"The signature header MUST be verified", "Webhook is processed when the signature header is absent", true, true, "long subject"},
	{"user_id MUST be non-empty", "Lookup returns a record when user_id is empty", true, true, "empty as the predicate"},
	// The only true positive the shipped detector misses. extractSubject keeps
	// the article, so the subject is "The audit record" and the criterion says
	// "an audit record".
	{"The audit record MUST be written", "Deletion completes without an audit record", true, false, "FALSE NEGATIVE: article mismatch"},
}

// nearMisses: the subject and an absence word both appear and there is no
// conflict. Six of seven fire. This is the corpus that refuted the bound-rule
// fix, and it is the reason `structural_conflict` is advisory under C-15.
var nearMisses = []conflictCase{
	{"email MUST be required", "Registration fails when email is absent", false, true,
		"FALSE POSITIVE: the criterion ENFORCES the constraint. Lexically identical to the first true conflict; only the outcome verb differs"},
	{"email MUST be required", "Error is returned when email is missing", false, true,
		"FALSE POSITIVE: enforcement, different verb"},
	{"email MUST be required", "Validation rejects a payload without email", false, true,
		"FALSE POSITIVE: enforcement, backward form"},
	{"email MUST be required", "Confirm that email is not absent before sending", false, true,
		"FALSE POSITIVE: double negation, means present"},
	{"email MUST be required", "The phone number is absent from the email template", false, true,
		"FALSE POSITIVE: subject inside an unrelated noun phrase"},
	{"The refresh token MUST be present on every request", "Audit log records that the refresh token is present", false, false,
		"correctly silent: no absence word"},
	{"api_key MUST be supplied by the caller", "Rate limit applies without regard to api_key", false, true,
		"FALSE POSITIVE: 'without' governs 'regard', not the subject"},
}

// ciProximitySurvivors are the two real-corpus cases that survive a word
// boundary plus proximity tightening. SP-004 asks for them by name, so a later
// simplification to a proximity rule cannot pass unnoticed.
var ciProximitySurvivors = []conflictCase{
	{"CI MUST validate the PR title", "in a ci environment without a controlling terminal", false, true,
		"FALSE POSITIVE: survives boundaries plus proximity"},
	{"CI MUST validate the PR title", "any ci script that pipes confirmation tokens) without", false, true,
		"FALSE POSITIVE: survives boundaries plus proximity"},
}

func detectorFires(c conflictCase) bool {
	upstream := makeSpec("up", 1)
	upstream.Constraints = []schema.Constraint{{ID: "C-01", Description: c.constraint}}
	upstream.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: "covers it", ReferencesConstraints: []string{"C-01"}},
	}
	downstream := makeSpec("down", 2)
	downstream.Constraints = []schema.Constraint{{ID: "C-01", Description: "downstream"}}
	downstream.AcceptanceCriteria = []schema.AcceptanceCriterion{
		{ID: "AC-01", Description: c.criterion, ReferencesConstraints: []string{"C-01"}},
	}
	g := makeGraph(
		map[string]*resolver.SpecNode{
			"up":   {Spec: upstream, File: "u.yaml"},
			"down": {Spec: downstream, File: "d.yaml"},
		},
		[]resolver.SpecEdge{{From: "down", To: "up", Relationship: "requires"}},
	)
	for _, d := range CheckSpecs(g, nil).Diagnostics {
		if d.Kind == "structural_conflict" {
			return true
		}
	}
	return false
}

// @ac AC-03
// Every case behaves as recorded. A change to the matcher moves one of these
// and fails here, which forces the change to be deliberate.
func TestConflictCorpus_MatchesRecordedBehavior(t *testing.T) {
	all := append(append(append([]conflictCase{}, trueConflicts...), nearMisses...), ciProximitySurvivors...)
	for _, c := range all {
		t.Run("spec-check/AC-03 "+c.note, func(t *testing.T) {
			if got := detectorFires(c); got != c.fires {
				t.Errorf("detector fires=%v, recorded %v\n  constraint: %s\n  criterion:  %s\n  note: %s",
					got, c.fires, c.constraint, c.criterion, c.note)
			}
		})
	}
}

// @ac AC-03
// The accuracy of the shipped detector, stated as numbers rather than left to
// be inferred from the table above.
//
// These are not aspirational. They are what ships, and they are the argument
// for C-15 making the diagnostic advisory: a check wrong on six of seven
// non-conflicts cannot be a gate.
func TestConflictCorpus_AccuracyIsRecorded(t *testing.T) {
	t.Run("spec-check/AC-03 accuracy is recorded", func(t *testing.T) {
		var detected, missed int
		for _, c := range trueConflicts {
			if detectorFires(c) {
				detected++
			} else {
				missed++
			}
		}
		var falsePositives int
		for _, c := range append(append([]conflictCase{}, nearMisses...), ciProximitySurvivors...) {
			if detectorFires(c) {
				falsePositives++
			}
		}

		const (
			wantDetected       = 11
			wantMissed         = 1
			wantFalsePositives = 8
		)
		if detected != wantDetected || missed != wantMissed {
			t.Errorf("true conflicts: %d detected, %d missed; recorded %d and %d",
				detected, missed, wantDetected, wantMissed)
		}
		if falsePositives != wantFalsePositives {
			t.Errorf("false positives: %d of %d non-conflicts fired; recorded %d",
				falsePositives, len(nearMisses)+len(ciProximitySurvivors), wantFalsePositives)
		}
	})
}

// verdict_test.go -- C-39: the approval-gate demotion is part of classifying a
// criterion, not a pass over a finished report.
//
// @spec spec-coverage
package coverage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

// gateSpec returns a spec whose first criterion violates the approval gate,
// second is plainly covered, and third carries no annotation. The three
// reasons a criterion can end up uncovered, in one spec, in declaration order.
func gateSpec(tier int) schema.SpecAST {
	return schema.SpecAST{
		ID:   "ord",
		Tier: tier,
		AcceptanceCriteria: []schema.AcceptanceCriterion{
			{ID: "AC-01", Description: "gate violating, otherwise covered", ApprovalGate: true},
			{ID: "AC-02", Description: "plainly covered"},
			{ID: "AC-03", Description: "no annotation"},
		},
	}
}

func gateAnnotations() []AnnotationMatch {
	return []AnnotationMatch{{SpecID: "ord", ACIDs: []string{"AC-01", "AC-02"}, File: "ord_test.go"}}
}

func gateResults() *ResultsFile {
	return &ResultsFile{Results: []ResultEntry{
		{SpecID: "ord", ACID: "AC-01", Status: "passed", Passed: true},
		{SpecID: "ord", ACID: "AC-02", Status: "passed", Passed: true},
	}}
}

// @ac AC-64
// A demoted criterion is placed where it was declared, not appended after the
// criteria that were already uncovered.
//
// The pass this replaced appended, so the same workspace reported
// [AC-03, AC-01]. Invisible on any spec with only one uncovered criterion,
// which is why every fixture before this one missed it.
func TestClassify_DemotedCriterionKeepsDeclarationOrder(t *testing.T) {
	t.Run("spec-coverage/AC-64 demoted criterion keeps declaration order", func(t *testing.T) {
		report, err := BuildCoverageReportMode(
			[]schema.SpecAST{gateSpec(3)}, gateAnnotations(), map[int]int{3: 50}, gateResults(),
			ClassifyMode{Strict: true, ZeroTolerance: true})
		if err != nil {
			t.Fatal(err)
		}
		e := report.Entries[0]

		if got := strings.Join(e.CoveredACs, ","); got != "AC-02" {
			t.Errorf("covered = [%s], want [AC-02]", got)
		}
		if got := strings.Join(e.UncoveredACs, ","); got != "AC-01,AC-03" {
			t.Errorf("uncovered = [%s], want [AC-01,AC-03]. A demoted criterion must sit "+
				"where it was declared, not after criteria uncovered for other reasons.", got)
		}
	})
}

// @ac AC-65
// An entry whose only covered criterion is demoted reports an empty array, not
// null. The JSON contract declares these non-nullable and the extension's
// client types agree, so a null is what SP-025 was.
func TestClassify_FullyDemotedEntryMarshalsEmptyArray(t *testing.T) {
	t.Run("spec-coverage/AC-65 fully demoted entry marshals an empty array", func(t *testing.T) {
		spec := schema.SpecAST{
			ID:   "nul",
			Tier: 3,
			AcceptanceCriteria: []schema.AcceptanceCriterion{
				{ID: "AC-01", Description: "the only criterion, and it is demoted", ApprovalGate: true},
			},
		}
		annotations := []AnnotationMatch{{SpecID: "nul", ACIDs: []string{"AC-01"}, File: "nul_test.go"}}
		results := &ResultsFile{Results: []ResultEntry{
			{SpecID: "nul", ACID: "AC-01", Status: "passed", Passed: true},
		}}

		report, err := BuildCoverageReportMode(
			[]schema.SpecAST{spec}, annotations, map[int]int{3: 50}, results,
			ClassifyMode{Strict: true, ZeroTolerance: true})
		if err != nil {
			t.Fatal(err)
		}
		if got := report.Entries[0].CoveredACs; got == nil {
			t.Error("CoveredACs is nil; it must be an empty slice so the JSON is [] and not null")
		} else if len(got) != 0 {
			t.Errorf("CoveredACs = %v, want empty", got)
		}

		raw, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"covered_acs":null`) {
			t.Error(`marshaled "covered_acs":null; the contract declares a non-nullable array`)
		}
	})
}

// @ac AC-64
// The summary counters are derived from the same verdicts as the entries, so
// one document cannot call a spec fully covered while showing it at 50 percent.
// The pass this replaced rebuilt Passing and Failing and left these two alone
// (bugs/SP-SP-068).
func TestClassify_SummaryCountersFollowTheDemotion(t *testing.T) {
	t.Run("spec-coverage/AC-64 summary counters follow the demotion", func(t *testing.T) {
		report, err := BuildCoverageReportMode(
			[]schema.SpecAST{gateSpec(3)}, gateAnnotations(), map[int]int{3: 50}, gateResults(),
			ClassifyMode{Strict: true, ZeroTolerance: true})
		if err != nil {
			t.Fatal(err)
		}
		if report.Summary.FullyCovered != 0 {
			t.Errorf("fully_covered = %d on a spec at %.1f%%, want 0",
				report.Summary.FullyCovered, report.Entries[0].CoveragePct)
		}
		if report.Summary.PartiallyCovered != 1 {
			t.Errorf("partially_covered = %d, want 1", report.Summary.PartiallyCovered)
		}
	})
}

// @ac AC-66
// Coverage and sync classify identically because they read one classification.
// Asserted at the level that matters: the same mode over the same inputs gives
// the same per-criterion verdicts, whichever entry point built the report.
func TestClassify_EveryEntryPointAgreesPerCriterion(t *testing.T) {
	t.Run("spec-coverage/AC-66 every entry point agrees per criterion", func(t *testing.T) {
		mode := ClassifyMode{Strict: true, ZeroTolerance: true}
		specs := []schema.SpecAST{gateSpec(3)}

		a, err := BuildCoverageReportMode(specs, gateAnnotations(), map[int]int{3: 50}, gateResults(), mode)
		if err != nil {
			t.Fatal(err)
		}
		b, err := BuildCoverageReportMode(specs, gateAnnotations(), map[int]int{3: 50}, gateResults(), mode)
		if err != nil {
			t.Fatal(err)
		}

		if len(a.Verdicts) != 3 {
			t.Fatalf("expected one verdict per declared criterion, got %d", len(a.Verdicts))
		}
		for i := range a.Verdicts {
			if a.Verdicts[i] != b.Verdicts[i] {
				t.Errorf("verdict %d differs between two builds: %+v vs %+v", i, a.Verdicts[i], b.Verdicts[i])
			}
			if a.Verdicts[i].Covered(mode) != b.Verdicts[i].Covered(mode) {
				t.Errorf("verdict %d classifies differently across builds", i)
			}
		}

		// The guards read the same verdicts the entries were built from, so a
		// criterion cannot be a violation for the exit code and not for the
		// report.
		if got := CountApprovalGateViolationsIn(a.Verdicts); got != 1 {
			t.Errorf("CountApprovalGateViolationsIn = %d, want 1", got)
		}
		if got := CountMissingAnnotationsIn(a.Verdicts); got != 1 {
			t.Errorf("CountMissingAnnotationsIn = %d, want 1", got)
		}
	})
}

// @ac AC-66
// Covered is a pure function of the verdict and the mode. The table is the
// point: every branch that used to live inside the build loop is now readable
// and testable without building a report at all.
func TestCriterionVerdict_CoveredIsPure(t *testing.T) {
	t.Run("spec-coverage/AC-66 covered is a pure function of verdict and mode", func(t *testing.T) {
		cases := []struct {
			name string
			v    CriterionVerdict
			mode ClassifyMode
			want bool
		}{
			{"tier 3, annotation alone",
				CriterionVerdict{Tier: 3, HasAnnotation: true}, ClassifyMode{}, true},
			{"tier 3, no annotation",
				CriterionVerdict{Tier: 3}, ClassifyMode{}, false},
			{"tier 1 needs a passing result",
				CriterionVerdict{Tier: 1, HasAnnotation: true}, ClassifyMode{}, false},
			{"tier 1 with a passing result",
				CriterionVerdict{Tier: 1, HasAnnotation: true, ResultPassed: true}, ClassifyMode{}, true},
			{"in scope needs status passed",
				CriterionVerdict{Tier: 3, HasAnnotation: true, ResultStatus: "failed", InScope: true},
				ClassifyMode{Strict: true}, false},
			{"in scope with status passed",
				CriterionVerdict{Tier: 3, HasAnnotation: true, ResultStatus: "passed", InScope: true},
				ClassifyMode{Strict: true}, true},
			{"gate violation demotes under zero tolerance",
				CriterionVerdict{Tier: 3, HasAnnotation: true, ResultStatus: "passed",
					InScope: true, ApprovalGateViolation: true},
				ClassifyMode{Strict: true, ZeroTolerance: true}, false},
			{"gate violation is metadata under threshold",
				CriterionVerdict{Tier: 3, HasAnnotation: true, ApprovalGateViolation: true},
				ClassifyMode{}, true},
		}
		for _, c := range cases {
			if got := c.v.Covered(c.mode); got != c.want {
				t.Errorf("%s: Covered = %v, want %v", c.name, got, c.want)
			}
		}
	})
}

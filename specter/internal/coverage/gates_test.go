package coverage

import (
	"strings"
	"testing"
)

// @spec spec-coverage
// @ac AC-67
//
// C-40(a), (d) and (e) as a table. The CLI criteria assert the four surfaces
// end to end; this asserts the decision itself, including two combinations the
// fixtures do not produce: rule 1 warning beside a real gate, and a threshold
// failure behind a gate that already decided the code.
func TestGateVerdict(t *testing.T) {
	t.Run("spec-coverage/AC-67 the gate decision, ordered", func(t *testing.T) {
		cases := []struct {
			name       string
			in         GateInputs
			wantCode   int
			wantCount  int
			wantFirst  int // the code the first violation carries, -1 for none
			wantSecond int // the code the second carries, -1 when there is none
		}{
			{
				name:       "clean",
				in:         GateInputs{AnnotationDeclared: true},
				wantCode:   0,
				wantCount:  0,
				wantFirst:  -1,
				wantSecond: -1,
			},
			{
				name:       "rule 1 alone, strict",
				in:         GateInputs{AnnotationDeclared: true, AnnotationRuleViolations: 1},
				wantCode:   2,
				wantCount:  1,
				wantFirst:  2,
				wantSecond: -1,
			},
			{
				// C-40(a): permissive changes the severity of rule 1 and
				// nothing else. The line is still reported.
				name:       "rule 1 alone, permissive",
				in:         GateInputs{AnnotationDeclared: true, AnnotationPermissive: true, AnnotationRuleViolations: 1},
				wantCode:   0,
				wantCount:  1,
				wantFirst:  0,
				wantSecond: -1,
			},
			{
				name:       "approval gate alone, annotation model",
				in:         GateInputs{AnnotationDeclared: true, ApprovalGateViolations: 1},
				wantCode:   3,
				wantCount:  1,
				wantFirst:  3,
				wantSecond: -1,
			},
			{
				// C-40(d) and (e): rule 1 takes the code, both are reported.
				name:       "both, strict",
				in:         GateInputs{AnnotationDeclared: true, AnnotationRuleViolations: 1, ApprovalGateViolations: 1},
				wantCode:   2,
				wantCount:  2,
				wantFirst:  2,
				wantSecond: 3,
			},
			{
				// Not reachable from the CLI fixtures. A permissive rule-1
				// warning must not swallow the gate's code, which is the
				// failure mode C-40(a) exists to prevent.
				name:       "both, permissive",
				in:         GateInputs{AnnotationDeclared: true, AnnotationPermissive: true, AnnotationRuleViolations: 1, ApprovalGateViolations: 1},
				wantCode:   3,
				wantCount:  2,
				wantFirst:  0,
				wantSecond: 3,
			},
			{
				name:       "ladder, non-passed before the gate",
				in:         GateInputs{ZeroTolerance: true, ZeroToleranceNonPassed: 1, ApprovalGateViolations: 1},
				wantCode:   2,
				wantCount:  2,
				wantFirst:  2,
				wantSecond: 3,
			},
			{
				// A gate that decided the code does not suppress the threshold
				// line. Same reasoning as (e): the operator should not have to
				// re-run to find the next thing.
				name:       "gate ahead of a threshold failure",
				in:         GateInputs{AnnotationDeclared: true, ApprovalGateViolations: 1, ThresholdFailing: 2},
				wantCode:   3,
				wantCount:  2,
				wantFirst:  3,
				wantSecond: 1,
			},
			{
				name:       "threshold alone",
				in:         GateInputs{AnnotationDeclared: true, ThresholdFailing: 1},
				wantCode:   1,
				wantCount:  1,
				wantFirst:  1,
				wantSecond: -1,
			},
		}

		for _, c := range cases {
			got, code := GateVerdict(c.in)
			if code != c.wantCode {
				t.Errorf("%s: code %d, want %d", c.name, code, c.wantCode)
			}
			if len(got) != c.wantCount {
				t.Errorf("%s: %d violation(s), want %d", c.name, len(got), c.wantCount)
				continue
			}
			if c.wantFirst >= 0 && got[0].Code != c.wantFirst {
				t.Errorf("%s: first violation carries code %d, want %d", c.name, got[0].Code, c.wantFirst)
			}
			if c.wantSecond >= 0 && got[1].Code != c.wantSecond {
				t.Errorf("%s: second violation carries code %d, want %d", c.name, got[1].Code, c.wantSecond)
			}
			for i, v := range got {
				if v.Stderr == "" || v.Phase == "" {
					t.Errorf("%s: violation %d has an empty message, so a caller would print a blank line", c.name, i)
				}
			}
		}
	})
}

// @spec spec-coverage
// @ac AC-67
//
// C-40(b): the annotation model does not borrow the ladder's wording. A run
// that is not on the ladder must not name zero-tolerance, or the operator is
// sent to a setting they did not choose.
func TestGateVerdict_ApprovalGateWordingPerModel(t *testing.T) {
	t.Run("spec-coverage/AC-67 the approval gate names the model it fired under", func(t *testing.T) {
		ladder, _ := GateVerdict(GateInputs{ZeroTolerance: true, ApprovalGateViolations: 1})
		model, _ := GateVerdict(GateInputs{AnnotationDeclared: true, ApprovalGateViolations: 1})
		if len(ladder) != 1 || len(model) != 1 {
			t.Fatalf("expected one violation each, got %d and %d", len(ladder), len(model))
		}
		if !strings.Contains(ladder[0].Stderr, "zero-tolerance strictness") {
			t.Errorf("the ladder message lost its golden wording, which the 1A4 parity test pins: %q", ladder[0].Stderr)
		}
		if strings.Contains(model[0].Stderr, "zero-tolerance") {
			t.Errorf("the annotation-model message names zero-tolerance, a setting the workspace did not choose: %q", model[0].Stderr)
		}
		if ladder[0].Stderr == model[0].Stderr {
			t.Error("both models produced one message, so the wording cannot distinguish which fired")
		}
	})
}

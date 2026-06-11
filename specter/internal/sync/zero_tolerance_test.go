// zero_tolerance_test.go -- spec-sync C-09: zero-tolerance gates in
// sync's coverage phase (AC-12/AC-13). Exit-code assertions live at the
// CLI layer (cmd/specter/sync_strictness_exit_test.go); these tests pin
// the pure-function contract: phase failure, message wording, and
// report demotion parity with coverage.
//
// @spec spec-sync
package sync

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

// twoACSpecYAML returns a Tier-<tier> spec with AC-01 and AC-02.
func twoACSpecYAML(id string, tier int) string {
	return fmt.Sprintf(`spec:
  id: %s
  version: "1.0.0"
  status: approved
  tier: %d

  context:
    system: test

  objective:
    summary: test spec

  constraints:
    - id: C-01
      description: "test constraint"

  acceptance_criteria:
    - id: AC-01
      description: "test ac one"
      references_constraints: ["C-01"]
    - id: AC-02
      description: "test ac two"
      references_constraints: ["C-01"]
`, id, tier)
}

// gatedSpecYAML returns a spec whose AC-01 carries approval_gate: true
// with no approval_date set.
func gatedSpecYAML(id string, tier int) string {
	return fmt.Sprintf(`spec:
  id: %s
  version: "1.0.0"
  status: approved
  tier: %d

  context:
    system: test

  objective:
    summary: test spec

  constraints:
    - id: C-01
      description: "test constraint"

  acceptance_criteria:
    - id: AC-01
      description: "gated ac"
      approval_gate: true
      references_constraints: ["C-01"]
`, id, tier)
}

// coveragePhase returns the coverage PhaseResult, or nil.
func coveragePhase(result *SyncResult) *PhaseResult {
	for i := range result.Phases {
		if result.Phases[i].Phase == "coverage" {
			return &result.Phases[i]
		}
	}
	return nil
}

// AC-12: a Tier 3 spec with one passing and one failing annotated AC
// meets its 50% tier threshold after demotion — but under
// zero-tolerance the failing AC MUST fail the coverage phase anyway
// (spec-coverage C-25 semantics). Under threshold, the same workspace
// passes. Pre-C-09 sync gated only on Summary.Failing, so
// zero-tolerance produced a false green here.
//
// @ac AC-12
func TestSyncZeroTolerance_FailsOnNonPassedACDespiteThreshold(t *testing.T) {
	input := func(strictness string) SyncInput {
		return SyncInput{
			SpecFiles: []FileContent{{Path: "a.spec.yaml", Content: twoACSpecYAML("a", 3)}},
			TestFiles: []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01", "AC-02")}},
			Results: &coverage.ResultsFile{
				Results: []coverage.ResultEntry{
					{SpecID: "a", ACID: "AC-01", Status: "passed", Passed: true},
					{SpecID: "a", ACID: "AC-02", Status: "failed", Passed: false},
				},
			},
			Strictness: strictness,
		}
	}

	t.Run("spec-sync/AC-12 zero-tolerance fails coverage phase on failed AC despite tier threshold met", func(t *testing.T) {
		result := RunSync(input("zero-tolerance"))
		if result.Passed {
			t.Fatal("expected sync to fail under zero-tolerance with one failed annotated AC, got pass (false green)")
		}
		phase := coveragePhase(result)
		if phase == nil {
			t.Fatal("expected a coverage phase result")
		}
		if phase.Passed {
			t.Error("expected coverage phase to fail under zero-tolerance")
		}
		if !strings.Contains(phase.Message, "zero-tolerance") {
			t.Errorf("expected coverage phase message to name the zero-tolerance violation, got %q", phase.Message)
		}
	})

	t.Run("spec-sync/AC-12 threshold mode passes the same workspace (demoted 50% meets Tier 3 threshold)", func(t *testing.T) {
		result := RunSync(input("threshold"))
		if !result.Passed {
			phase := coveragePhase(result)
			msg := ""
			if phase != nil {
				msg = phase.Message
			}
			t.Errorf("expected sync to pass under threshold (50%% >= Tier 3 threshold), got fail: %s", msg)
		}
	})
}

// AC-13: an AC with approval_gate: true and unset approval_date, all
// tests passing. Under zero-tolerance the coverage phase MUST demote
// the gated AC in the report (spec-coverage GH #94 semantics) and fail.
// Under threshold, approval_gate stays metadata and sync passes.
//
// @ac AC-13
func TestSyncZeroTolerance_ApprovalGateViolation(t *testing.T) {
	input := func(strictness string) SyncInput {
		return SyncInput{
			SpecFiles: []FileContent{{Path: "g.spec.yaml", Content: gatedSpecYAML("g", 3)}},
			TestFiles: []FileContent{{Path: "g.test.ts", Content: testFileContent("g", "AC-01")}},
			Results: &coverage.ResultsFile{
				Results: []coverage.ResultEntry{
					{SpecID: "g", ACID: "AC-01", Status: "passed", Passed: true},
				},
			},
			Strictness: strictness,
		}
	}

	t.Run("spec-sync/AC-13 zero-tolerance fails coverage phase and demotes the gated AC in the report", func(t *testing.T) {
		result := RunSync(input("zero-tolerance"))
		if result.Passed {
			t.Fatal("expected sync to fail under zero-tolerance with an approval_gate violation, got pass")
		}
		phase := coveragePhase(result)
		if phase == nil {
			t.Fatal("expected a coverage phase result")
		}
		if phase.Passed {
			t.Error("expected coverage phase to fail under zero-tolerance")
		}
		if !strings.Contains(phase.Message, "approval_gate") {
			t.Errorf("expected coverage phase message to name the approval_gate violation, got %q", phase.Message)
		}
		if result.CoverageReport == nil {
			t.Fatal("expected coverage report")
		}
		demoted := false
		for _, e := range result.CoverageReport.Entries {
			if e.SpecID == "g" {
				for _, ac := range e.UncoveredACs {
					if ac == "AC-01" {
						demoted = true
					}
				}
			}
		}
		if !demoted {
			t.Error("expected AC-01 demoted to UncoveredACs in sync's report (parity with coverage's GH #94 demotion)")
		}
	})

	t.Run("spec-sync/AC-13 threshold mode treats approval_gate as metadata and passes", func(t *testing.T) {
		result := RunSync(input("threshold"))
		if !result.Passed {
			phase := coveragePhase(result)
			msg := ""
			if phase != nil {
				msg = phase.Message
			}
			t.Errorf("expected sync to pass under threshold (approval_gate is metadata), got fail: %s", msg)
		}
	})
}

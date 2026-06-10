// @spec spec-sync
package sync

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

func validSpecYAML(id string, tier int, deps string) string {
	dependsOn := ""
	if deps != "" {
		dependsOn = fmt.Sprintf("\n  depends_on:\n    - spec_id: %s\n      relationship: requires", deps)
	}
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
      description: "test ac"
      references_constraints: ["C-01"]%s
`, id, tier, dependsOn)
}

func testFileContent(specID string, acIDs ...string) string {
	result := fmt.Sprintf("// @spec %s\n", specID)
	for _, id := range acIDs {
		result += fmt.Sprintf("// @ac %s\n", id)
	}
	return result
}

// @ac AC-01
func TestAllPhasesPass(t *testing.T) {
	t.Run("spec-sync/AC-01 all phases pass", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "a.spec.yaml", Content: validSpecYAML("a", 2, "")}},
			TestFiles: []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
		})

		if !result.Passed {
			t.Errorf("expected pass, got fail at %s", result.StoppedAt)
		}
		if len(result.Phases) != 4 {
			t.Errorf("expected 4 phases, got %d", len(result.Phases))
		}
	})
}

// @ac AC-02
func TestParseErrorStopsPipeline(t *testing.T) {
	t.Run("spec-sync/AC-02 parse error stops pipeline", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "bad.yaml", Content: "not: valid: yaml: {{{"}},
		})

		if result.Passed {
			t.Error("expected failure")
		}
		if result.StoppedAt != "parse" {
			t.Errorf("expected stopped at parse, got %s", result.StoppedAt)
		}
	})
}

// @ac AC-03
func TestDanglingDepStopsAtResolve(t *testing.T) {
	t.Run("spec-sync/AC-03 dangling dep stops at resolve", func(t *testing.T) {
		yaml := `spec:
  id: broken
  version: "1.0.0"
  status: approved
  tier: 2
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "test"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
  depends_on:
    - spec_id: nonexistent
      relationship: requires
`
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "broken.yaml", Content: yaml}},
		})

		if result.Passed {
			t.Error("expected failure")
		}
		if result.StoppedAt != "resolve" {
			t.Errorf("expected stopped at resolve, got %s", result.StoppedAt)
		}
	})
}

// @ac AC-04
func TestCheckErrorsFail(t *testing.T) {
	t.Run("spec-sync/AC-04 check errors fail", func(t *testing.T) {
		yaml := `spec:
  id: strict
  version: "1.0.0"
  status: approved
  tier: 1
  context:
    system: test
  objective:
    summary: test
  constraints:
    - id: C-01
      description: "referenced"
    - id: C-02
      description: "orphan"
  acceptance_criteria:
    - id: AC-01
      description: "test"
      references_constraints: ["C-01"]
`
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "strict.yaml", Content: yaml}},
			TestFiles: []FileContent{{Path: "strict.test.ts", Content: testFileContent("strict", "AC-01")}},
		})

		if result.Passed {
			t.Error("expected failure due to Tier 1 orphan")
		}
		if result.StoppedAt != "check" {
			t.Errorf("expected stopped at check, got %s", result.StoppedAt)
		}
	})
}

// @ac AC-05
func TestCoverageBelowThresholdFails(t *testing.T) {
	t.Run("spec-sync/AC-05 coverage below threshold fails", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "critical.yaml", Content: validSpecYAML("critical", 1, "")}},
			TestFiles: nil, // 0% coverage
		})

		if result.Passed {
			t.Error("expected failure due to Tier 1 at 0% coverage")
		}
		if result.StoppedAt != "coverage" {
			t.Errorf("expected stopped at coverage, got %s", result.StoppedAt)
		}
	})
}

func TestMultiSpecPipeline(t *testing.T) {
	result := RunSync(SyncInput{
		SpecFiles: []FileContent{
			{Path: "a.yaml", Content: validSpecYAML("a", 2, "")},
			{Path: "b.yaml", Content: validSpecYAML("b", 2, "a")},
		},
		TestFiles: []FileContent{
			{Path: "a.test.ts", Content: testFileContent("a", "AC-01")},
			{Path: "b.test.ts", Content: testFileContent("b", "AC-01")},
		},
	})

	if !result.Passed {
		t.Errorf("expected pass, got fail at %s", result.StoppedAt)
	}
}

// @ac AC-06
func TestOnlyPhase_Coverage_ContinuesDespiteResolveError(t *testing.T) {
	t.Run("spec-sync/AC-06 only phase coverage continues despite resolve error", func(t *testing.T) {
		// Spec with a dangling reference — resolve will fail
		specWithDanglingRef := validSpecYAML("a", 2, "nonexistent-spec")

		result := RunSync(SyncInput{
			SpecFiles: []FileContent{{Path: "a.yaml", Content: specWithDanglingRef}},
			TestFiles: []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
			OnlyPhase: "coverage",
		})

		// All four phases should have been attempted
		phaseNames := make(map[string]bool)
		for _, p := range result.Phases {
			phaseNames[p.Phase] = true
		}
		for _, required := range []string{"parse", "resolve", "check", "coverage"} {
			if !phaseNames[required] {
				t.Errorf("expected phase %q to be recorded, got phases: %v", required, result.Phases)
			}
		}

		// resolve failed but we continued — coverage is what determines Passed
		resolvePhase := ""
		for _, p := range result.Phases {
			if p.Phase == "resolve" {
				if p.Passed {
					resolvePhase = "passed"
				} else {
					resolvePhase = "failed"
				}
			}
		}
		if resolvePhase != "failed" {
			t.Errorf("expected resolve to fail, got %q", resolvePhase)
		}
	})
}

// @ac AC-07
func TestOnlyPhase_Check_ContinuesDespiteParseError(t *testing.T) {
	t.Run("spec-sync/AC-07 only phase check continues despite parse error", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles: []FileContent{
				{Path: "valid.yaml", Content: validSpecYAML("valid", 2, "")},
				{Path: "bad.yaml", Content: "not: valid: yaml: at: all"},
			},
			TestFiles: nil,
			OnlyPhase: "check",
		})

		// check phase should be reached (may pass with 0 diagnostics on the valid spec)
		phaseNames := make(map[string]bool)
		for _, p := range result.Phases {
			phaseNames[p.Phase] = true
		}
		if !phaseNames["check"] {
			t.Errorf("expected check phase to be reached, got phases: %v", result.Phases)
		}
		// pipeline should NOT have stopped at parse
		if result.StoppedAt == "parse" {
			t.Error("expected pipeline not to stop at parse in --only check mode")
		}
	})
}

func TestOnlyPhase_Parse_StopsAfterParse(t *testing.T) {
	result := RunSync(SyncInput{
		SpecFiles: []FileContent{{Path: "a.yaml", Content: validSpecYAML("a", 2, "")}},
		TestFiles: []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
		OnlyPhase: "parse",
	})

	if len(result.Phases) != 1 || result.Phases[0].Phase != "parse" {
		t.Errorf("expected only parse phase, got %v", result.Phases)
	}
	if !result.Passed {
		t.Error("expected pass for --only parse with valid spec")
	}
}

// AC-08: --strictness <level> is accepted at all three levels; --strict
// is treated as zero-tolerance alias. The test passes inputs and
// confirms SyncInput accepts the Strictness field.
//
// @ac AC-08
func TestSyncStrictness_AcceptedLevels(t *testing.T) {
	for _, level := range []string{"annotation", "threshold", "zero-tolerance"} {
		t.Run("spec-sync/AC-08 --strictness "+level+" is accepted by SyncInput", func(t *testing.T) {
			result := RunSync(SyncInput{
				SpecFiles:  []FileContent{{Path: "a.yaml", Content: validSpecYAML("a", 2, "")}},
				TestFiles:  []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
				Strictness: level,
			})
			// The minimum contract for this AC: SyncInput accepts the
			// field without panic. Behavioral assertions live in AC-09
			// (parity with coverage --strict) and AC-10/AC-11
			// (missing-results behavior).
			if result == nil {
				t.Errorf("RunSync returned nil for Strictness=%q", level)
			}
		})
	}
}

// AC-09: sync's coverage phase under strict mode produces a report
// identical to `coverage --strictness <level>` on the same workspace.
// The probe: a Tier 2 spec with one annotated AC whose .specter-results.json
// entry says "failed". Under BuildCoverageReportWithResults (the current
// sync path), Tier 2 doesn't demote on failing results; under
// BuildCoverageReportStrict, it does. After commit 3 wires sync to the
// strict path, sync's report MUST demote the AC matching coverage.
//
// @ac AC-09
func TestSyncStrictness_CoverageMatchesStrictPath(t *testing.T) {
	t.Run("spec-sync/AC-09 sync --strictness threshold demotes failing-test AC matching coverage --strict", func(t *testing.T) {
		results := &coverage.ResultsFile{
			Results: []coverage.ResultEntry{
				{SpecID: "a", ACID: "AC-01", Status: "failed", Passed: false},
			},
		}

		result := RunSync(SyncInput{
			SpecFiles:  []FileContent{{Path: "a.yaml", Content: validSpecYAML("a", 2, "")}},
			TestFiles:  []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
			Results:    results,
			Strictness: "threshold",
		})

		// The contract: under strict mode, a failing AC must demote
		// and sync's coverage phase must fail. Currently sync uses
		// BuildCoverageReportWithResults which doesn't demote Tier 2;
		// the test fails until commit 3 wires the strict path.
		if result.Passed {
			t.Errorf("expected sync --strictness threshold to fail on AC-01 with failed test result, got pass")
		}
		if result.CoverageReport == nil {
			t.Fatal("expected coverage report")
		}
		if result.CoverageReport.Summary.Failing == 0 {
			t.Errorf("expected at least one spec to be failing under strict mode, got Failing=%d", result.CoverageReport.Summary.Failing)
		}
	})
}

// AC-10: missing .specter-results.json under strict mode fails the
// coverage phase with a message that names the active strictness mode,
// references `specter ingest`, and offers `--strictness annotation` as
// the alternative remedy — and does NOT attribute the requirement to a
// `--strict` flag the sync operator never passed.
//
// @ac AC-10
func TestSyncStrictness_MissingResultsUnderStrict(t *testing.T) {
	t.Run("spec-sync/AC-10 sync --strictness zero-tolerance with missing results fails with mode-named, non-misleading hint", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles:  []FileContent{{Path: "a.yaml", Content: validSpecYAML("a", 2, "")}},
			TestFiles:  []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
			Results:    nil, // simulates missing .specter-results.json
			Strictness: "zero-tolerance",
		})

		if result.Passed {
			t.Errorf("expected sync --strictness zero-tolerance to fail when Results is nil, got pass")
		}
		// Find the coverage phase result and verify the message.
		var coveragePhase *PhaseResult
		for i := range result.Phases {
			if result.Phases[i].Phase == "coverage" {
				coveragePhase = &result.Phases[i]
			}
		}
		if coveragePhase == nil {
			t.Fatal("expected coverage phase result")
		}
		if coveragePhase.Passed {
			t.Error("expected coverage phase to fail under strict mode with missing Results")
		}
		msg := coveragePhase.Message
		// (a) names the active strictness mode.
		if !strings.Contains(msg, "zero-tolerance") {
			t.Errorf("expected message to name the active strictness mode %q, got: %q", "zero-tolerance", msg)
		}
		// (b) references specter ingest as one remedy.
		if !strings.Contains(msg, "specter ingest") {
			t.Errorf("expected message to mention `specter ingest`, got: %q", msg)
		}
		// (c) offers --strictness annotation as the alternative remedy.
		if !strings.Contains(msg, "--strictness annotation") {
			t.Errorf("expected message to offer `--strictness annotation`, got: %q", msg)
		}
		// MUST NOT misattribute the requirement to a --strict flag.
		if strings.Contains(msg, "--strict requires") {
			t.Errorf("message must not attribute the requirement to `--strict` in sync, got: %q", msg)
		}
	})
}

// AC-11: under annotation mode (or no strict flag), missing
// .specter-results.json is NOT an error. The annotation-counting
// path proceeds.
//
// @ac AC-11
func TestSyncStrictness_MissingResultsUnderAnnotation(t *testing.T) {
	t.Run("spec-sync/AC-11 sync --strictness annotation with missing results succeeds", func(t *testing.T) {
		result := RunSync(SyncInput{
			SpecFiles:  []FileContent{{Path: "a.yaml", Content: validSpecYAML("a", 2, "")}},
			TestFiles:  []FileContent{{Path: "a.test.ts", Content: testFileContent("a", "AC-01")}},
			Results:    nil,
			Strictness: "annotation",
		})

		if !result.Passed {
			t.Errorf("expected sync --strictness annotation with missing Results to pass (annotation-count path), got fail at %q with phases %+v", result.StoppedAt, result.Phases)
		}
	})
}

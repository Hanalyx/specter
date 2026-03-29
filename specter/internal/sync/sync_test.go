// @spec spec-sync
package sync

import (
	"fmt"
	"testing"
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
}

// @ac AC-02
func TestParseErrorStopsPipeline(t *testing.T) {
	result := RunSync(SyncInput{
		SpecFiles: []FileContent{{Path: "bad.yaml", Content: "not: valid: yaml: {{{"}},
	})

	if result.Passed {
		t.Error("expected failure")
	}
	if result.StoppedAt != "parse" {
		t.Errorf("expected stopped at parse, got %s", result.StoppedAt)
	}
}

// @ac AC-03
func TestDanglingDepStopsAtResolve(t *testing.T) {
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
}

// @ac AC-04
func TestCheckErrorsFail(t *testing.T) {
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
}

// @ac AC-05
func TestCoverageBelowThresholdFails(t *testing.T) {
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

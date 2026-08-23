// CLI integration tests for spec-check C-16 and C-17, the opt-in concreteness
// rule.
//
// These are CLI tests rather than unit tests because CheckOptions has no
// Concrete field yet, so a unit test would fail to compile. A compile failure
// says the code is unfinished, not that the behavior is wrong.
//
// @spec spec-check
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// bareCriterionSpec renders a spec whose single criterion carries neither
// inputs nor expected_output, which is what C-16 reports.
func bareCriterionSpec(id string, tier int) string {
	return `spec:
  id: ` + id + `
  version: "1.0.0"
  status: approved
  tier: ` + strconv.Itoa(tier) + `
  context: {system: fixture, feature: ` + id + `}
  objective: {summary: "concreteness fixture."}
  constraints:
    - id: C-01
      description: "The value MUST be present"
  acceptance_criteria:
    - id: AC-01
      description: "handles it correctly"
      references_constraints: ["C-01"]
`
}

func concreteWorkspace(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.spec.yaml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "specter.yaml"),
		[]byte("schema_version: 1\nsystem:\n  name: fixture\nsettings:\n  specs_dir: .\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// @ac AC-37
// Severity follows the tier: error at 1, warning at 2, info at 3.
func TestCheckConcrete_SeverityFollowsTier(t *testing.T) {
	cases := []struct {
		tier     int
		want     string
		wantFail bool
	}{
		// The rendered label, not the severity name: the reporter prints
		// "warn" for a warning.
		{1, "error", true},
		{2, "warn", false},
		{3, "info", false},
	}
	for _, c := range cases {
		t.Run("spec-check/AC-37 tier severity", func(t *testing.T) {
			dir := concreteWorkspace(t, bareCriterionSpec("bare", c.tier))
			out, code := runCLI(t, dir, "check", "--concrete")

			if !strings.Contains(out, "vague_criterion") {
				t.Fatalf("tier %d: no vague_criterion diagnostic:\n%s", c.tier, out)
			}
			if !strings.Contains(out, c.want+" [vague_criterion]") {
				t.Errorf("tier %d: expected severity %q, got:\n%s", c.tier, c.want, out)
			}
			if c.wantFail && code == 0 {
				t.Errorf("tier 1 must fail the run, got exit 0")
			}
			if !c.wantFail && code != 0 {
				t.Errorf("tier %d must not fail the run, got exit %d", c.tier, code)
			}
		})
	}
}

// @ac AC-37
// Either field satisfies the rule. It asks whether the criterion says
// something concrete, not whether it says everything.
func TestCheckConcrete_EitherFieldSatisfies(t *testing.T) {
	cases := []struct{ name, extra string }{
		{"inputs only", "      inputs:\n        payload: \"one\"\n"},
		{"expected_output only", "      expected_output:\n        status: \"ok\"\n"},
		{"both", "      inputs:\n        payload: \"one\"\n      expected_output:\n        status: \"ok\"\n"},
	}
	for _, c := range cases {
		t.Run("spec-check/AC-37 "+c.name, func(t *testing.T) {
			dir := concreteWorkspace(t, bareCriterionSpec("ok", 1)+c.extra)
			out, code := runCLI(t, dir, "check", "--concrete")
			if strings.Contains(out, "vague_criterion") {
				t.Errorf("%s was reported vague:\n%s", c.name, out)
			}
			if code != 0 {
				t.Errorf("%s exited %d, want 0", c.name, code)
			}
		})
	}
}

// @ac AC-38
// The rule is opt-in, and --strict does not arm it.
//
// The strict half is the one that matters. A rule that is silent by default and
// armed by --strict is not opt-in, because --strict is what CI passes.
func TestCheckConcrete_OptInAndSilentByDefault(t *testing.T) {
	t.Run("spec-check/AC-38 opt-in and silent by default", func(t *testing.T) {
		for _, args := range [][]string{{"check"}, {"check", "--strict"}} {
			dir := concreteWorkspace(t, bareCriterionSpec("bare", 1))
			out, code := runCLI(t, dir, args...)
			if strings.Contains(out, "vague_criterion") {
				t.Errorf("%v reported vague_criterion without --concrete:\n%s", args, out)
			}
			if code != 0 {
				t.Errorf("%v exited %d without --concrete, want 0", args, code)
			}
		}
	})
}

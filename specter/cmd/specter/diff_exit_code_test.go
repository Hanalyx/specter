// CLI integration tests for spec-diff C-03 reporting of a contract-only change
// and C-14 opt-in --exit-code.
//
// @spec spec-diff
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// contractSpec renders a one-criterion spec whose description never changes,
// so the only thing a diff can find is the contract.
func contractSpec(order, gate, priority string) string {
	return `spec:
  id: demo
  version: "1.0.0"
  status: approved
  tier: 2
  context: {system: fixture, feature: demo}
  objective: {summary: "contract fixture."}
  constraints:
    - id: C-01
      description: "The value MUST be present"
  acceptance_criteria:
    - id: AC-01
      description: "same description throughout"
      references_constraints: ["C-01"]
      approval_gate: ` + gate + `
      priority: ` + priority + `
      inputs:
        payload: "one"
      expected_output:
        order: ` + order + `
`
}

func writeTwo(t *testing.T, before, after string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.spec.yaml")
	b := filepath.Join(dir, "b.spec.yaml")
	if err := os.WriteFile(a, []byte(before), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(after), 0644); err != nil {
		t.Fatal(err)
	}
	return dir, a, b
}

// @ac AC-20
// The rendered line names the field that changed instead of printing the
// identical description on both sides of an arrow.
func TestDiffExitCode_ContractOnlyChangeNamesTheField(t *testing.T) {
	t.Run("spec-diff/AC-20 contract-only change names the field", func(t *testing.T) {
		dir, a, b := writeTwo(t,
			contractSpec("[1, 2, 3]", "true", "high"),
			contractSpec("[3, 2, 1]", "true", "high"))

		out, code := runCLI(t, dir, "diff", a, b)
		if code != 0 {
			t.Fatalf("expected exit 0 without --exit-code, got %d. output:\n%s", code, out)
		}
		if !strings.Contains(out, "[breaking]") {
			t.Fatalf("expected a breaking classification, got:\n%s", out)
		}
		if strings.Contains(out, "same description throughout → same description throughout") {
			t.Errorf("the line prints the identical description twice, which tells the reader "+
				"nothing about what changed:\n%s", out)
		}
		if !strings.Contains(out, "expected_output") {
			t.Errorf("the line does not name expected_output, the field that differs:\n%s", out)
		}
	})
}

// @ac AC-19
// --exit-code is opt-in. Without it a breaking change still exits 0, because
// `diff` is a documented diagnostic surface and changing that silently breaks
// every caller.
func TestDiffExitCode_DefaultStaysZeroOnBreaking(t *testing.T) {
	t.Run("spec-diff/AC-19 default stays zero on breaking", func(t *testing.T) {
		dir, a, b := writeTwo(t,
			contractSpec("[1, 2, 3]", "true", "high"),
			contractSpec("[3, 2, 1]", "true", "high"))

		out, code := runCLI(t, dir, "diff", a, b)
		if code != 0 {
			t.Errorf("default exit is %d, want 0; --exit-code is the opt-in", code)
		}
		if !strings.Contains(out, "[breaking]") {
			t.Errorf("expected a breaking classification, got:\n%s", out)
		}
	})
}

// @ac AC-19
// With the flag, a breaking change exits in the orchestration band and
// anything else exits 0.
func TestDiffExitCode_FlagFailsOnlyOnBreaking(t *testing.T) {
	cases := []struct {
		name        string
		before      string
		after       string
		wantBand    bool
		wantClassIn string
	}{
		{"breaking, contract changed",
			contractSpec("[1, 2, 3]", "true", "high"),
			contractSpec("[3, 2, 1]", "true", "high"), true, "breaking"},
		{"breaking, approval gate removed",
			contractSpec("[1, 2, 3]", "true", "high"),
			contractSpec("[1, 2, 3]", "false", "high"), true, "breaking"},
		{"unchanged",
			contractSpec("[1, 2, 3]", "true", "high"),
			contractSpec("[1, 2, 3]", "true", "high"), false, "no changes"},
	}

	for _, c := range cases {
		t.Run("spec-diff/AC-19 "+c.name, func(t *testing.T) {
			dir, a, b := writeTwo(t, c.before, c.after)
			out, code := runCLI(t, dir, "diff", a, b, "--exit-code")

			if !strings.Contains(out, c.wantClassIn) {
				t.Fatalf("expected output to contain %q, got:\n%s", c.wantClassIn, out)
			}
			if c.wantBand {
				if code < 10 || code > 19 {
					t.Errorf("--exit-code on a breaking change exited %d, want the orchestration "+
						"band 10 to 19", code)
				}
			} else if code != 0 {
				t.Errorf("--exit-code on a non-breaking change exited %d, want 0", code)
			}
		})
	}
}

// CLI integration tests for `specter resolve` sub-subcommands
// (v0.13 C4 / spec-resolve 1.2.0).
//
// @spec spec-resolve
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-13: bare `specter resolve` preserves v1.x build-and-validate
// behavior. The new dependents subcommand MUST NOT shadow the
// parent's RunE.
//
// @ac AC-13
func TestResolveBareCommand_PreservesV1Behavior(t *testing.T) {
	t.Run("spec-resolve/AC-13 bare specter resolve runs v1.x build-and-validate", func(t *testing.T) {
		dir := t.TempDir()

		// Two specs: spec-base (no deps), spec-leaf (depends on spec-base)
		specBase := `spec:
  id: spec-base
  version: "1.0.0"
  status: approved
  tier: 2
  context:
    system: test
  objective:
    summary: base
  constraints:
    - id: C-01
      description: test
  acceptance_criteria:
    - id: AC-01
      description: test
      references_constraints: ["C-01"]
`
		specLeaf := `spec:
  id: spec-leaf
  version: "1.0.0"
  status: approved
  tier: 2
  context:
    system: test
  objective:
    summary: leaf
  constraints:
    - id: C-01
      description: test
  acceptance_criteria:
    - id: AC-01
      description: test
      references_constraints: ["C-01"]
  depends_on:
    - spec_id: spec-base
      relationship: requires
`
		if err := os.WriteFile(filepath.Join(dir, "base.spec.yaml"), []byte(specBase), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "leaf.spec.yaml"), []byte(specLeaf), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "resolve")
		if code != 0 {
			t.Errorf("expected bare `specter resolve` to exit 0 with valid specs, got %d.\noutput:\n%s", code, out)
		}
		// The v1.x output includes build/resolve diagnostics or a
		// successful resolve message. Assert it doesn't look like a
		// dependents query (which would print spec IDs only).
		if !strings.Contains(out, "resolved") && !strings.Contains(out, "specs") {
			t.Errorf("expected bare `resolve` output to mention resolution, got: %s", out)
		}
	})
}

// `specter resolve dependents <spec-id>` returns the set of direct
// dependents.
//
// @ac AC-10
func TestResolveDependentsCmd_DirectDependents(t *testing.T) {
	t.Run("spec-resolve/AC-10 dependents subcommand returns direct dependents (CLI level)", func(t *testing.T) {
		dir := t.TempDir()

		specBase := `spec:
  id: spec-base
  version: "1.0.0"
  status: approved
  tier: 2
  context: {system: test}
  objective: {summary: base}
  constraints:
    - {id: C-01, description: test}
  acceptance_criteria:
    - {id: AC-01, description: test, references_constraints: [C-01]}
`
		specLeaf := `spec:
  id: spec-leaf
  version: "1.0.0"
  status: approved
  tier: 2
  context: {system: test}
  objective: {summary: leaf}
  constraints:
    - {id: C-01, description: test}
  acceptance_criteria:
    - {id: AC-01, description: test, references_constraints: [C-01]}
  depends_on:
    - {spec_id: spec-base, relationship: requires}
`
		if err := os.WriteFile(filepath.Join(dir, "base.spec.yaml"), []byte(specBase), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "leaf.spec.yaml"), []byte(specLeaf), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "resolve", "dependents", "spec-base")
		if code != 0 {
			t.Errorf("expected exit 0 for valid dependents query, got %d.\noutput:\n%s", code, out)
		}
		if !strings.Contains(out, "spec-leaf") {
			t.Errorf("expected output to list spec-leaf as dependent of spec-base, got: %s", out)
		}
	})
}

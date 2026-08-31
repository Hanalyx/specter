// check_json_early_return_test.go -- `check --json` writes a document in the
// three states that end a run early, spec-check 3.2.0 C-14/AC-46,
// bugs/SP-SP-032.
//
// C-14 already required the document to be written in full before the
// exit-code decision. Three early returns write nothing, so a caller reading
// stdout gets an empty stream and cannot tell a broken workspace from a
// crashed process.
//
// Exit-code parity is not the defect and is asserted anyway: text and --json
// already agree at 1 in all three states. What is missing is the document.
//
// @spec spec-check
package main

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validSpec parses, resolves, and reaches the checker cleanly.
const validSpec = `spec:
  id: ok-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: s, feature: f, description: A valid fixture used beside the failing ones.}
  objective: {summary: Parse, resolve and reach the checker.}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
  acceptance_criteria:
    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}
`

// orphanSpec reaches the checker and produces exactly one diagnostic. It is the
// control: without it, a command that emitted an empty document and never
// checked anything would satisfy every assertion below.
const orphanSpec = `spec:
  id: ok-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: s, feature: f, description: A valid fixture carrying one unreferenced constraint.}
  objective: {summary: Reach the checker and report one orphan constraint.}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
    - {id: C-02, description: "MUST also hold, referenced by nothing", type: technical}
  acceptance_criteria:
    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}
`

// danglingSpec is schema-valid and names a dependency that does not exist, so
// it fails at the resolver rather than at parse. The distinction matters: an
// earlier attempt at this fixture used an unknown field inside depends_on,
// which fails at parse and silently tested the wrong branch.
const danglingSpec = `spec:
  id: dep-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: s, feature: f, description: A schema-valid spec depending on one that does not exist.}
  objective: {summary: Produce a resolver error.}
  depends_on:
    - {spec_id: no-such-spec}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
  acceptance_criteria:
    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}
`

// earlyReturnWorkspace builds a workspace in one of the failing states and
// returns its root.
func earlyReturnWorkspace(t *testing.T, state string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	switch state {
	case "parse":
		write("specs/ok.spec.yaml", validSpec)
		write("specs/bad.spec.yaml", "spec:\n  id: bad\n  bogus_field: 1\n")
	case "resolver":
		write("specs/dep.spec.yaml", danglingSpec)
	case "manifest":
		write("specs/ok.spec.yaml", validSpec)
		write("specter.yaml", "schema_version: 1\nsystem: {name: s}\nsettings:\n  bogus_key: 1\n")
	case "control":
		write("specs/ok.spec.yaml", orphanSpec)
	default:
		t.Fatalf("unknown state %q", state)
	}
	return dir
}

// @spec spec-check
// @ac AC-46
//
// C-14: the document is written in every state, and the exit code comes from
// the same verdict the ordinary path uses.
func TestCheckJSONWritesADocumentOnEveryEarlyReturn(t *testing.T) {
	// The control runs first. Every assertion below is about a document being
	// present, and a command that always emitted one while checking nothing
	// would satisfy all of them.
	t.Run("spec-check/AC-46 control: a run that reaches the checker reports its diagnostic", func(t *testing.T) {
		dir := earlyReturnWorkspace(t, "control")
		stdout, _, code := runCLISplit(t, dir, "check", "--json")
		if code != 0 {
			t.Fatalf("AC-46 (control): a Tier 3 orphan is info severity and must exit 0, got %d.\n%s", code, stdout)
		}
		var doc struct {
			Diagnostics []struct {
				Kind     string `json:"kind"`
				Severity string `json:"severity"`
			} `json:"diagnostics"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("AC-46 (control): stdout does not parse: %v\n%s", err, stdout)
		}
		var found bool
		for _, d := range doc.Diagnostics {
			if d.Kind == "orphan_constraint" {
				found = true
			}
		}
		if !found {
			t.Fatalf("AC-46 (control): the run reached the checker and reported no orphan_constraint, so the assertions below would pass on a command that checks nothing.\n%s", stdout)
		}
	})

	for _, tc := range []struct {
		state string
		why   string
	}{
		{"parse", "one spec in the workspace fails to parse"},
		{"resolver", "the resolver reports a dangling reference"},
		{"manifest", "the manifest fails to load"},
	} {
		t.Run("spec-check/AC-46 "+tc.state+" failure still writes a document", func(t *testing.T) {
			// Three subtests rather than one loop assertion, because these are
			// three separate early returns in the command and a fix applied to
			// one of them is exactly the defect this criterion exists to catch.
			dir := earlyReturnWorkspace(t, tc.state)

			textOut, _, textCode := runCLISplit(t, dir, "check")
			stdout, stderr, jsonCode := runCLISplit(t, dir, "check", "--json")

			if len(strings.TrimSpace(stdout)) == 0 {
				t.Fatalf("AC-46: stdout is empty when %s. C-14 requires the document to be written in full before the exit-code decision, so a caller reading stdout cannot tell a broken workspace from a crashed process.\nstderr:\n%s", tc.why, stderr)
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Errorf("AC-46: stdout does not parse as one JSON document when %s: %v\nstdout:\n%s", tc.why, err, stdout)
			}
			if _, ok := doc["diagnostics"]; !ok {
				t.Errorf("AC-46: the document carries no diagnostics key when %s, so it does not say why the run stopped.\nstdout:\n%s", tc.why, stdout)
			}
			// C-14: stdout carries the document alone. Human-readable notices
			// belong on stderr, and the text run above shows there are some.
			if strings.Contains(stdout, "error [") || strings.Contains(stdout, "error:") {
				t.Errorf("AC-46: stdout carries human-readable notices beside the document when %s. C-14 requires stdout to hold the document alone.\nstdout:\n%s", tc.why, stdout)
			}
			if jsonCode != textCode {
				t.Errorf("AC-46: exit codes disagree when %s. text=%d json=%d. The verdict is a function of the diagnostics, not of how they are rendered.\ntext:\n%s", tc.why, textCode, jsonCode, textOut)
			}
		})
	}
}

// @spec spec-check
// @ac AC-46
//
// C-14's one-path half, asserted structurally because no runtime observation
// can see it.
//
// Adding an encoder to each early return produces identical output for every
// input above, so every behavioral assertion stays green while the condition
// that caused the defect is restored: an early return with no emitter is one
// edit away, and the next one added will be forgotten the same way.
func TestCheckRendersTheJSONDocumentInOnePlace(t *testing.T) {
	t.Run("spec-check/AC-46 checkCmd encodes to stdout at exactly one site", func(t *testing.T) {
		fset := token.NewFileSet()
		files := parseProductionFiles(t, fset, ".")

		sites := funcSitesMatching(fset, files, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewEncoder" {
				return false
			}
			pkg, ok := sel.X.(*ast.Ident)
			return ok && pkg.Name == "json"
		})

		var inCheck []string
		for _, s := range sites {
			if strings.HasPrefix(s, "checkCmd:") {
				inCheck = append(inCheck, s)
			}
		}

		// Positive control. A guard that finds no encoder anywhere is matching
		// the wrong pattern, and would pass this claim vacuously.
		if len(sites) == 0 {
			t.Fatalf("AC-46: no json.NewEncoder call found in any production file, so the matcher is wrong and the count below is meaningless")
		}
		if len(inCheck) != 1 {
			t.Errorf("AC-46: checkCmd encodes a document at %d site(s), want exactly 1. Sites: %v. Every early return that renders its own document is another place for the next early return to be forgotten, which is how bugs/SP-SP-032 began", len(inCheck), inCheck)
		}
	})
}

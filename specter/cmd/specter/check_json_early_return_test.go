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
	"sort"
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
  objective: {summary: "Parse, resolve, and reach the checker"}
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
		// stage is a string only the intended failure stage emits. It is not
		// decoration: the manifest fixture once carried a spec whose objective
		// read `{summary: Parse, resolve and reach the checker.}`, where the
		// comma splits the flow mapping into two keys. That spec did not parse,
		// so the "manifest" case exercised the parse branch, passed, and went on
		// passing when the manifest branch was reverted. A mutation found it;
		// the green suite could not.
		stage string
		// kind is the diagnostic the document must carry. Without it the
		// criterion is satisfied by an empty diagnostics list, which counts
		// zero errors, so both modes exit 0 and the parity assertion holds
		// too. Measured: dropping the diagnostic from all three early returns
		// left this test green.
		kind string
	}{
		{"parse", "one spec in the workspace fails to parse", "FAIL specs/bad.spec.yaml", "parse_error"},
		{"resolver", "the resolver reports a dangling reference", "dangling_reference", "dangling_reference"},
		{"manifest", "the manifest fails to load", "unknown settings key", "manifest_error"},
	} {
		t.Run("spec-check/AC-46 "+tc.state+" failure still writes a document", func(t *testing.T) {
			// Three subtests rather than one loop assertion, because these are
			// three separate early returns in the command and a fix applied to
			// one of them is exactly the defect this criterion exists to catch.
			dir := earlyReturnWorkspace(t, tc.state)

			textOut, _, textCode := runCLISplit(t, dir, "check")
			stdout, stderr, jsonCode := runCLISplit(t, dir, "check", "--json")

			// The fixture reaches the branch it names, before anything else is
			// asserted. Without this, a fixture that fails earlier tests some
			// other branch twice and reports success for this one.
			if !strings.Contains(stderr, tc.stage) {
				t.Fatalf("AC-46: the %s fixture did not reach the %s stage. stderr should name %q, so this case is exercising a different early return than the one it claims.\nstderr:\n%s", tc.state, tc.state, tc.stage, stderr)
			}

			if len(strings.TrimSpace(stdout)) == 0 {
				t.Fatalf("AC-46: stdout is empty when %s. C-14 requires the document to be written in full before the exit-code decision, so a caller reading stdout cannot tell a broken workspace from a crashed process.\nstderr:\n%s", tc.why, stderr)
			}
			var doc struct {
				Diagnostics []struct {
					Kind     string `json:"kind"`
					Severity string `json:"severity"`
					Message  string `json:"message"`
				} `json:"diagnostics"`
				Summary struct {
					Errors int `json:"errors"`
				} `json:"summary"`
			}
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("AC-46: stdout does not parse as one JSON document when %s: %v\nstdout:\n%s", tc.why, err, stdout)
			}

			// The document has to say why the run stopped. A present but empty
			// diagnostics list passes every structural check, counts zero
			// errors, and exits 0 in both modes, which is worse than writing
			// nothing: an empty stream is at least obviously broken.
			var got string
			for _, d := range doc.Diagnostics {
				if d.Kind != tc.kind {
					continue
				}
				got = d.Kind
				if d.Severity != "error" {
					t.Errorf("AC-46: the %s diagnostic has severity %q, want \"error\". A non-error severity gives the run a zero summary and a passing exit code", tc.state, d.Severity)
				}
				if strings.TrimSpace(d.Message) == "" {
					t.Errorf("AC-46: the %s diagnostic carries an empty message, so the document names a kind and explains nothing", tc.state)
				}
			}
			if got == "" {
				kinds := make([]string, 0, len(doc.Diagnostics))
				for _, d := range doc.Diagnostics {
					kinds = append(kinds, d.Kind)
				}
				t.Errorf("AC-46: the document carries no %q diagnostic when %s. Kinds present: %v.\nstdout:\n%s", tc.kind, tc.why, kinds, stdout)
			}
			if doc.Summary.Errors < 1 {
				t.Errorf("AC-46: the summary reports %d error(s) when %s, want at least 1. The summary is what the exit code is computed from, so a zero here is a passing verdict on a failed run", doc.Summary.Errors, tc.why)
			}
			// C-14: stdout carries the document alone. Human-readable notices
			// belong on stderr, and the text run above shows there are some.
			if strings.Contains(stdout, "error [") || strings.Contains(stdout, "error:") {
				t.Errorf("AC-46: stdout carries human-readable notices beside the document when %s. C-14 requires stdout to hold the document alone.\nstdout:\n%s", tc.why, stdout)
			}
			// Exit 1 in its own right, not only as parity. Both modes agree at
			// 0 when the document is empty, so parity alone cannot see the
			// failure this criterion is about.
			if jsonCode != 1 {
				t.Errorf("AC-46: --json exited %d when %s, want 1", jsonCode, tc.why)
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
// C-14's ownership half, asserted structurally because no runtime observation
// can see it. Adding an encoder to each early return produces identical output
// for every input above, so every behavioral assertion stays green while the
// condition that caused the defect is restored.
//
// Ownership, not placement. An earlier version of this guard required the
// encoder to appear lexically inside checkCmd, which would fail a valid
// extraction of the renderer into a named helper even though the property it
// cares about was preserved. Private policy about where code sits is not the
// rule; one owner that every exit routes through is.
func TestCheckRendersThroughOneNamedOwner(t *testing.T) {
	t.Run("spec-check/AC-46 one discovered function owns encoding, and the command routes to it", func(t *testing.T) {
		fset := token.NewFileSet()
		files := parseProductionFiles(t, fset, ".")

		encodes := func(n ast.Node) bool {
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
		}
		bareErrSilent := func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return false
			}
			id, ok := ret.Results[0].(*ast.Ident)
			return ok && id.Name == "errSilent"
		}
		// calleesOf collects the plain-identifier calls made inside fn, which
		// is how the owner is discovered rather than named.
		calleesOf := func(fn string) map[string]int {
			out := map[string]int{}
			for _, f := range files {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok || fd.Name.Name != fn {
						continue
					}
					ast.Inspect(fd, func(n ast.Node) bool {
						if ce, ok := n.(*ast.CallExpr); ok {
							if id, ok := ce.Fun.(*ast.Ident); ok {
								out[id.Name]++
							}
						}
						return true
					})
				}
			}
			return out
		}
		sitesIn := func(fn string, want func(ast.Node) bool) []string {
			var out []string
			for _, s := range funcSitesMatching(fset, files, want) {
				if strings.HasPrefix(s, fn+":") {
					out = append(out, s)
				}
			}
			return out
		}

		// Positive control: the matcher finds encoders somewhere, so a zero
		// below means absence rather than a broken pattern.
		if len(funcSitesMatching(fset, files, encodes)) == 0 {
			t.Fatalf("AC-46: no json.NewEncoder call found in any production file, so this matcher is wrong and every count below is meaningless")
		}

		// The owner is DISCOVERED, not named. Of everything checkCmd calls,
		// exactly one callee may encode a document. Hard-coding the owner's
		// name would make a rename fail a guard whose property survived it,
		// which is private policy rather than the rule.
		callees := calleesOf("checkCmd")
		if len(callees) == 0 {
			t.Fatalf("AC-46: checkCmd appears to call nothing, so the owner cannot be discovered and every claim below is vacuous")
		}
		var owners []string
		for name := range callees {
			if len(sitesIn(name, encodes)) > 0 {
				owners = append(owners, name)
			}
		}
		sort.Strings(owners)
		if len(owners) != 1 {
			t.Fatalf("AC-46: %d of checkCmd's callees encode a document, want exactly 1. Found: %v. One owner is what keeps a state added later from acquiring an empty stdout", len(owners), owners)
		}
		owner := owners[0]

		if got := len(sitesIn(owner, encodes)); got != 1 {
			t.Errorf("AC-46: the owner %s encodes at %d site(s), want exactly 1", owner, got)
		}
		if got := sitesIn("checkCmd", encodes); len(got) != 0 {
			t.Errorf("AC-46: checkCmd encodes a document itself at %v. Encoding belongs to the owner; a command that encodes is a second renderer whatever it is named", got)
		}
		if got := sitesIn("checkCmd", bareErrSilent); len(got) != 0 {
			t.Errorf("AC-46: checkCmd returns errSilent directly at %v. That exit writes no document, which is exactly bugs/SP-SP-032", got)
		}
		if n := callees[owner]; n < 4 {
			t.Errorf("AC-46: checkCmd routes to its render owner %s at %d site(s), want at least 4: three early returns and the ordinary path", owner, n)
		}
	})
}

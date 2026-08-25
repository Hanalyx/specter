// parse_json_exit_test.go -- CLI integration test for SP-022: exit-code parity
// between `specter parse` and `specter parse --json` (spec-parse 1.3.0, C-11,
// AC-19).
//
// This crosses the CLI boundary on purpose. C-11 is a statement about a process
// exit code, and the parser package is pure: it returns a result and cannot
// observe an exit status. Only a subprocess run can assert the criterion, so
// both cases go through runCLISplit, which is defined in check_json_exit_test.go
// and keeps stdout apart from stderr. That separation matters here because the
// criterion says the document is still written when the run fails, and a merged
// stream cannot tell the document from the error lines beside it.
//
// @spec spec-parse
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// parseProbeInvalid is missing `status` and `tier` and carries a field the
// schema does not allow. Two errors of different types, so the assertion does
// not rest on one validator path.
const parseProbeInvalid = `spec:
  id: broken-probe
  version: "1.0.0"
  bogus_field: true
  context:
    system: probe
  objective:
    summary: A spec missing status and tier, carrying an unknown field.
  constraints:
    - id: C-01
      description: "MUST hold"
  acceptance_criteria:
    - id: AC-01
      description: "something"
      references_constraints: ["C-01"]
`

const parseProbeValid = `spec:
  id: clean-probe
  version: "1.0.0"
  status: draft
  tier: 3
  context:
    system: probe
  objective:
    summary: A spec that parses cleanly.
  constraints:
    - id: C-01
      description: "MUST hold"
  acceptance_criteria:
    - id: AC-01
      description: "something"
      references_constraints: ["C-01"]
`

func parseProbeWorkspace(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, "probe.spec.yaml", body)
	putManifest(t, dir, "system:\n  name: probe\nsettings:\n  specs_dir: specs\n")
	return dir
}

// @spec spec-parse
// @ac AC-19
//
// C-11: `parse --json` exits with the code `parse` exits with, and still writes
// its document. bugs/SP-SP-022.
func TestParseJSONExitParity(t *testing.T) {
	t.Run("spec-parse/AC-19 parse and parse --json agree on the exit code", func(t *testing.T) {
		// The failing workspace. Both modes must exit 1.
		bad := parseProbeWorkspace(t, parseProbeInvalid)
		_, textErr, textCode := runCLISplit(t, bad, "parse")
		jsonOut, _, jsonCode := runCLISplit(t, bad, "parse", "--json")

		// Precondition. If text mode stops failing, the parity below holds for
		// the wrong reason and proves nothing.
		if textCode == 0 {
			t.Fatalf("the invalid workspace parsed cleanly under text mode, so this case no longer reaches the defect.\nstderr:\n%s", textErr)
		}
		if textCode != 1 {
			t.Errorf("AC-19: `parse` exited %d on the invalid workspace, expected 1", textCode)
		}
		if jsonCode != textCode {
			t.Errorf("AC-19: `parse` exited %d and `parse --json` exited %d on the same workspace. C-11 makes the code a function of the parse result, not of the rendering", textCode, jsonCode)
		}

		// The document is still written, and still reports the same errors the
		// text run wrote. The fix is to the verdict, not to the output.
		var doc struct {
			OK     bool `json:"ok"`
			Errors []struct {
				Type    string `json:"type"`
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.NewDecoder(strings.NewReader(jsonOut)).Decode(&doc); err != nil {
			t.Fatalf("AC-19: `parse --json` wrote no decodable document on a failing run: %v\nstdout:\n%s", err, jsonOut)
		}
		if doc.OK {
			t.Error("AC-19: the document reports ok: true for a spec that does not parse")
		}

		// Compared against the text run rather than pinned to literals, so the
		// assertion is about the two modes agreeing and does not have to be
		// rewritten when a validator message changes. Counting alone is not
		// enough either: two errors of different types could be swapped for two
		// copies of one and the count would still match.
		textErrors := countTextParseErrors(textErr)
		if textErrors == 0 {
			t.Fatalf("the text run named no errors, so there is nothing to compare the document against.\nstderr:\n%s", textErr)
		}
		if len(doc.Errors) != textErrors {
			t.Errorf("AC-19: the document carries %d error(s) and the text run named %d. The two modes must report the same errors, not merely both fail.\nstdout:\n%s\nstderr:\n%s",
				len(doc.Errors), textErrors, jsonOut, textErr)
		}
		for _, e := range doc.Errors {
			want := fmt.Sprintf("error [%s] %s: %s", e.Type, e.Path, e.Message)
			if !strings.Contains(textErr, want) {
				t.Errorf("AC-19: the document carries an error the text run never printed.\n  document: %s\n  stderr:\n%s", want, textErr)
			}
		}

		// The clean workspace. Both modes must exit 0, so parity cannot be
		// satisfied by failing everything.
		good := parseProbeWorkspace(t, parseProbeValid)
		_, goodErr, goodText := runCLISplit(t, good, "parse")
		_, _, goodJSON := runCLISplit(t, good, "parse", "--json")
		if goodText != 0 || goodJSON != 0 {
			t.Errorf("AC-19: the clean workspace exited %d under `parse` and %d under `parse --json`, expected 0 for both.\nstderr:\n%s", goodText, goodJSON, goodErr)
		}
	})
}

// countTextParseErrors counts the error lines `parse` writes to stderr, which
// is the set the JSON document has to match. The FAIL line naming the file is
// not one of them.
func countTextParseErrors(stderr string) int {
	n := 0
	for _, l := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "error [") {
			n++
		}
	}
	return n
}

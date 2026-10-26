// parse_required_test.go -- CLI integration test for bugs/SP-SP-093: a spec
// missing several required fields gets one required error per field
// (spec-parse 1.5.0, AC-22).
//
// The parser test proves the errors. This one proves what reaches a user
// through two surfaces that read them: the error type in `parse --json`, and
// the pattern doctor prints, which groups errors by type and path.
//
// @spec spec-parse
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMissingThreeFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "missing-three-required.spec.yaml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

// @spec spec-parse
// @ac AC-22
func TestParseJSONTypesEachMissingFieldRequired(t *testing.T) {
	t.Run("spec-parse/AC-22 parse --json gives one required error per missing field", func(t *testing.T) {
		dir := parseProbeWorkspace(t, readMissingThreeFixture(t))
		out, stderr, code := runCLISplit(t, dir, "parse", "--json")
		if code != 1 {
			t.Errorf("expected exit 1 on a spec that does not parse, got %d\nstderr:\n%s", code, stderr)
		}

		var doc struct {
			Errors []struct {
				Type    string `json:"type"`
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&doc); err != nil {
			t.Fatalf("parse --json wrote no decodable document: %v\nstdout:\n%s", err, out)
		}
		if len(doc.Errors) != 3 {
			t.Fatalf("expected 3 errors, got %d: %+v", len(doc.Errors), doc.Errors)
		}
		for _, field := range []string{"status", "tier", "context"} {
			found := false
			for _, e := range doc.Errors {
				if strings.Contains(e.Message, "required field '"+field+"'") {
					found = true
				}
			}
			if !found {
				t.Errorf("no error names the missing field %q: %+v", field, doc.Errors)
			}
		}
		for _, e := range doc.Errors {
			if e.Type != "required" {
				t.Errorf("expected type required, got %s at %q: %s", e.Type, e.Path, e.Message)
			}
			if strings.Contains(e.Message, "at '") || strings.Contains(e.Message, "missing propert") {
				t.Errorf("message carries validator text: %s", e.Message)
			}
		}
	})
}

// @spec spec-parse
// @ac AC-22
func TestDoctorPatternNamesMissingFieldsRequired(t *testing.T) {
	t.Run("spec-parse/AC-22 doctor reports the pattern as required, not validation", func(t *testing.T) {
		dir := t.TempDir()
		body := readMissingThreeFixture(t)
		writeSpec(t, dir, "pay-a.spec.yaml", body)
		writeSpec(t, dir, "pay-b.spec.yaml", body)
		writeManifest(t, dir, "system:\n  name: probe\n")

		out, _ := runCLI(t, dir, "doctor")
		if !strings.Contains(out, "Pattern analysis") {
			t.Fatalf("expected a pattern analysis block, so this case reaches the grouping.\noutput:\n%s", out)
		}
		if !strings.Contains(out, `hit the same failure: [required] at "spec"`) {
			t.Errorf("expected doctor to name the shared failure as [required] at \"spec\".\noutput:\n%s", out)
		}
		if strings.Contains(out, "[validation]") {
			t.Errorf("doctor still reports the missing fields as [validation].\noutput:\n%s", out)
		}
		if strings.Contains(out, "missing propert") {
			t.Errorf("doctor prints the validator's raw message.\noutput:\n%s", out)
		}
	})
}

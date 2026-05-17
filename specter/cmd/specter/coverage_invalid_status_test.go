// CLI integration tests for v0.13 D2 — unrecognized status value
// diagnostic for .specter-results.json (spec-coverage 1.14.0 C-30 / AC-35).
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-35 default: stderr warning naming the offending value AND the
// documented enum, printed BEFORE the coverage table. Exit-code
// behavior unchanged from today (status="pass" still treated as not-
// passed and demotes under --strict).
//
// @ac AC-35
func TestCoverageInvalidStatus_DefaultEmitsWarningAboveTable(t *testing.T) {
	t.Run("spec-coverage/AC-35 unrecognized status emits stderr warning naming the value and the documented enum", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))
		testFile := "// @spec my-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// status: "pass" — the common typo case (canonical example
		// from the C-30 rationale).
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "pass", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage", "--strict")

		// Warning must name the offending value `"pass"` AND the documented
		// enum so the operator can self-diagnose.
		if !strings.Contains(out, `"pass"`) {
			t.Errorf("expected warning to name the offending value `\"pass\"`, got:\n%s", out)
		}
		if !strings.Contains(out, "passed|failed|skipped|errored") {
			t.Errorf("expected warning to name the documented enum `passed|failed|skipped|errored`, got:\n%s", out)
		}

		// Warning must appear ABOVE the coverage table (same ordering
		// contract as the C-22 empty-results warning and C-28 hints).
		warnIdx := strings.Index(out, `"pass"`)
		tableIdx := strings.Index(out, "Spec ID")
		if warnIdx < 0 {
			t.Fatalf("expected warning text in output for ordering check, got:\n%s", out)
		}
		if tableIdx < 0 {
			t.Fatalf("expected coverage table header `Spec ID` in output, got:\n%s", out)
		}
		if warnIdx >= tableIdx {
			t.Errorf("warning must appear ABOVE the coverage table; got warning at %d, table header at %d:\n%s",
				warnIdx, tableIdx, out)
		}
	})
}

// AC-35 quiet: --quiet suppresses the stderr warning but the parsing
// behavior (and exit code) is unchanged.
//
// @ac AC-35
func TestCoverageInvalidStatus_QuietSuppressesStderrWarning(t *testing.T) {
	t.Run("spec-coverage/AC-35 --quiet suppresses the stderr warning", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))
		testFile := "// @spec my-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "pass", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage", "--strict", "--quiet")

		// Under --quiet, the warning line must be absent. The documented-
		// enum string is unique to this warning, so its absence is the
		// strongest signal that the warning was suppressed.
		if strings.Contains(out, "passed|failed|skipped|errored") {
			t.Errorf("--quiet must suppress invalid-status warning; got:\n%s", out)
		}
	})
}

// AC-35 json: --json surfaces the same data as a top-level
// invalid_status_warnings array. Always emitted under --json,
// regardless of --quiet.
//
// @ac AC-35
func TestCoverageInvalidStatus_JsonSurfacesInvalidStatusWarnings(t *testing.T) {
	t.Run("spec-coverage/AC-35 --json emits invalid_status_warnings array entry per unique value", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01", "AC-02"))
		testFile := "// @spec my-spec\n// @ac AC-01\n// @ac AC-02\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		// Two entries with the SAME unrecognized value → one warning
		// with count=2. Aggregation rule per C-30.
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "pass", "test_name": "TestFoo"},
			{"spec_id": "my-spec", "ac_id": "AC-02", "status": "pass", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage", "--strict", "--json")

		// Parse the first JSON value from combined output (stderr may
		// trail with strictness messages).
		var parsed map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(out))
		if err := dec.Decode(&parsed); err != nil {
			t.Fatalf("expected valid JSON, got parse error: %v\noutput:\n%s", err, out)
		}

		warnings, ok := parsed["invalid_status_warnings"].([]interface{})
		if !ok {
			t.Fatalf("expected top-level `invalid_status_warnings` array in JSON, got: %v\nfull JSON:\n%s", parsed["invalid_status_warnings"], out)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected 1 aggregated warning (status=pass, count=2), got %d entries: %v", len(warnings), warnings)
		}
		entry, ok := warnings[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected warning entry to be an object, got: %T %v", warnings[0], warnings[0])
		}
		if entry["status"] != "pass" {
			t.Errorf("expected status=`pass`, got %v", entry["status"])
		}
		// JSON numbers decode as float64.
		if count, ok := entry["count"].(float64); !ok || int(count) != 2 {
			t.Errorf("expected count=2, got %v (%T)", entry["count"], entry["count"])
		}
	})

	t.Run("spec-coverage/AC-35 --json emits invalid_status_warnings even under --quiet", func(t *testing.T) {
		dir := t.TempDir()
		writeManifestWithStrictness(t, dir, "threshold")
		writeSpec(t, dir, "my-spec.spec.yaml", minimalValidSpec("my-spec", 2, "AC-01"))
		testFile := "// @spec my-spec\n// @ac AC-01\nfunc TestFoo(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0644); err != nil {
			t.Fatal(err)
		}
		results := `{"results": [
			{"spec_id": "my-spec", "ac_id": "AC-01", "status": "pass", "test_name": "TestFoo"}
		]}`
		if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(results), 0644); err != nil {
			t.Fatal(err)
		}

		out, _ := runCLI(t, dir, "coverage", "--strict", "--json", "--quiet")

		var parsed map[string]interface{}
		dec := json.NewDecoder(strings.NewReader(out))
		if err := dec.Decode(&parsed); err != nil {
			t.Fatalf("expected valid JSON, got parse error: %v\noutput:\n%s", err, out)
		}
		warnings, ok := parsed["invalid_status_warnings"].([]interface{})
		if !ok || len(warnings) == 0 {
			t.Errorf("--quiet must NOT suppress invalid_status_warnings in --json output; got: %v\nfull JSON:\n%s", parsed["invalid_status_warnings"], out)
		}
	})
}

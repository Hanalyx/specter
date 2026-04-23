// ingest_test.go -- CLI-level tests for `specter ingest`.
//
// @spec spec-ingest
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @ac AC-08
func TestIngest_JUnit_WritesResultsFile(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")
	junit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite>
    <testcase name="svc/AC-01 passes"/>
    <testcase name="svc/AC-02 fails">
      <failure message="bad"/>
    </testcase>
  </testsuite>
</testsuites>`
	if err := os.WriteFile(junitPath, []byte(junit), 0644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, ".specter-results.json")
	_, code := runCLI(t, dir, "ingest", "--junit", junitPath, "--output", outPath)
	if code != 0 {
		t.Fatalf("ingest exited non-zero (want 0)")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("results file not written: %v", err)
	}

	var parsed struct {
		Results []struct {
			SpecID string `json:"spec_id"`
			ACID   string `json:"ac_id"`
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("results file invalid JSON: %v\n%s", err, data)
	}
	if len(parsed.Results) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed.Results))
	}
}

// @ac AC-08
func TestIngest_DefaultOutputPath(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")
	junit := `<testsuites><testsuite><testcase name="s/AC-01"/></testsuite></testsuites>`
	_ = os.WriteFile(junitPath, []byte(junit), 0644)

	// No --output flag: default to .specter-results.json in the working dir.
	_, code := runCLI(t, dir, "ingest", "--junit", junitPath)
	if code != 0 {
		t.Fatalf("ingest exited non-zero")
	}
	if _, err := os.Stat(filepath.Join(dir, ".specter-results.json")); err != nil {
		t.Errorf("default output file missing: %v", err)
	}
}

// @ac AC-08
func TestIngest_MissingInputFile_ExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	_, code := runCLI(t, dir, "ingest", "--junit", filepath.Join(dir, "does-not-exist.xml"))
	if code == 0 {
		t.Errorf("expected non-zero exit for missing input file")
	}
}

// @ac AC-03
// go test -json flavor end-to-end.
func TestIngest_GoTest_WritesResultsFile(t *testing.T) {
	dir := t.TempDir()
	goJSON := filepath.Join(dir, "go-test.json")
	content := `{"Action":"pass","Package":"p","Test":"TestX/svc/AC-03"}` + "\n"
	_ = os.WriteFile(goJSON, []byte(content), 0644)

	out := filepath.Join(dir, ".specter-results.json")
	_, code := runCLI(t, dir, "ingest", "--go-test", goJSON, "--output", out)
	if code != 0 {
		t.Fatalf("ingest go-test exited non-zero")
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("output file not written: %v", err)
	}
}

// --- v0.10.0 adoption affordances ---

// @ac AC-09
// Default stderr summary line: "Scanned N test cases; extracted M (spec_id, ac_id)
// pairs; dropped K with no runner-visible annotation." Turns the ingest
// black box into something the operator can act on immediately.
func TestIngest_EmitsScanSummary(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")
	// 5 testcases: 2 annotated (svc/AC-01, svc/AC-02), 3 bare titles.
	junit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites><testsuite>
  <testcase name="svc/AC-01 passes"/>
  <testcase name="svc/AC-02 fails"><failure/></testcase>
  <testcase name="some bare title"/>
  <testcase name="another bare title"/>
  <testcase name="third bare title"/>
</testsuite></testsuites>`
	_ = os.WriteFile(junitPath, []byte(junit), 0644)

	out, code := runCLI(t, dir, "ingest", "--junit", junitPath)
	if code != 0 {
		t.Fatalf("ingest exited non-zero: %s", out)
	}

	expected := "Scanned 5 test cases; extracted 2 (spec_id, ac_id) pairs; dropped 3 with no runner-visible annotation."
	if !strings.Contains(out, expected) {
		t.Errorf("expected summary line:\n  %s\ngot:\n%s", expected, out)
	}
}

// @ac AC-09
// Zero-testcase fixture still emits the summary line with zeros. Counts
// must be independent of the fixture shape — empty is a valid result.
func TestIngest_EmitsScanSummary_EmptyFixture(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")
	junit := `<?xml version="1.0" encoding="UTF-8"?><testsuites><testsuite></testsuite></testsuites>`
	_ = os.WriteFile(junitPath, []byte(junit), 0644)

	out, _ := runCLI(t, dir, "ingest", "--junit", junitPath)
	expected := "Scanned 0 test cases; extracted 0 (spec_id, ac_id) pairs; dropped 0 with no runner-visible annotation."
	if !strings.Contains(out, expected) {
		t.Errorf("expected summary line for empty fixture:\n  %s\ngot:\n%s", expected, out)
	}
}

// @ac AC-10
// --verbose adds one stderr line per dropped testcase. Without --verbose,
// only the summary line is emitted; with it, per-case drop reasons follow.
func TestIngest_Verbose_EmitsPerTestDropReasons(t *testing.T) {
	dir := t.TempDir()
	junitPath := filepath.Join(dir, "junit.xml")
	junit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites><testsuite>
  <testcase name="svc/AC-01 passes"/>
  <testcase name="bare title one"/>
  <testcase name="bare title two"/>
</testsuite></testsuites>`
	_ = os.WriteFile(junitPath, []byte(junit), 0644)

	// Without --verbose: only the summary, no per-case lines.
	plainOut, _ := runCLI(t, dir, "ingest", "--junit", junitPath)
	if strings.Contains(plainOut, "  dropped:") {
		t.Errorf("without --verbose, per-case drop lines should be absent; got:\n%s", plainOut)
	}

	// With --verbose: per-case drop lines for each of the 2 dropped cases.
	verboseOut, _ := runCLI(t, dir, "ingest", "--junit", junitPath, "--verbose")
	if !strings.Contains(verboseOut, "  dropped: bare title one") {
		t.Errorf("expected per-case drop line for `bare title one`; got:\n%s", verboseOut)
	}
	if !strings.Contains(verboseOut, "  dropped: bare title two") {
		t.Errorf("expected per-case drop line for `bare title two`; got:\n%s", verboseOut)
	}
	if !strings.Contains(verboseOut, "no (spec_id, ac_id) pair found") {
		t.Errorf("expected drop reason text on at least one line; got:\n%s", verboseOut)
	}
}

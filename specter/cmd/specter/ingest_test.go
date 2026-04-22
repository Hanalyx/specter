// ingest_test.go -- CLI-level tests for `specter ingest`.
//
// @spec spec-ingest
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

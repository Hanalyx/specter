// @spec spec-ingest
package ingest

import (
	"testing"
)

// @ac AC-01
func TestParseJUnit_PassedTestcase_ReturnsPassedResult(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="engine">
    <testcase name="engine-transaction/AC-07 serializes per host" classname="engine.transaction"/>
  </testsuite>
</testsuites>`)

	results, err := ParseJUnit(xml)
	if err != nil {
		t.Fatalf("ParseJUnit returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.SpecID != "engine-transaction" {
		t.Errorf("SpecID = %q, want engine-transaction", r.SpecID)
	}
	if r.ACID != "AC-07" {
		t.Errorf("ACID = %q, want AC-07", r.ACID)
	}
	if r.Status != StatusPassed {
		t.Errorf("Status = %q, want passed", r.Status)
	}
}

// @ac AC-02
func TestParseJUnit_FailureChild_ReturnsFailedStatus(t *testing.T) {
	xml := []byte(`<testsuites>
  <testsuite>
    <testcase name="engine-transaction/AC-08 concurrent runs">
      <failure message="assertion failed">expected serialization</failure>
    </testcase>
  </testsuite>
</testsuites>`)

	results, err := ParseJUnit(xml)
	if err != nil {
		t.Fatalf("ParseJUnit returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusFailed {
		t.Errorf("Status = %q, want failed", results[0].Status)
	}
}

// @ac AC-02
func TestParseJUnit_SkippedChild_ReturnsSkippedStatus(t *testing.T) {
	xml := []byte(`<testsuites>
  <testsuite>
    <testcase name="engine-transaction/AC-09 flaky">
      <skipped/>
    </testcase>
  </testsuite>
</testsuites>`)

	results, _ := ParseJUnit(xml)
	if len(results) != 1 || results[0].Status != StatusSkipped {
		t.Fatalf("expected one skipped result, got %+v", results)
	}
}

// @ac AC-02
func TestParseJUnit_ErrorChild_ReturnsErroredStatus(t *testing.T) {
	xml := []byte(`<testsuites>
  <testsuite>
    <testcase name="engine-transaction/AC-10 setup broke">
      <error message="panic in setup"/>
    </testcase>
  </testsuite>
</testsuites>`)

	results, _ := ParseJUnit(xml)
	if len(results) != 1 || results[0].Status != StatusErrored {
		t.Fatalf("expected one errored result, got %+v", results)
	}
}

// @ac AC-05
func TestParseJUnit_NoAnnotation_DroppedSilently(t *testing.T) {
	xml := []byte(`<testsuites>
  <testsuite>
    <testcase name="some unrelated test" classname="junk"/>
    <testcase name="engine-transaction/AC-07 has annotation"/>
  </testsuite>
</testsuites>`)

	results, err := ParseJUnit(xml)
	if err != nil {
		t.Fatalf("ParseJUnit returned error: %v (must not error on unannotated tests)", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (the annotated one), got %d", len(results))
	}
	if results[0].ACID != "AC-07" {
		t.Errorf("wrong test kept; got ACID = %q", results[0].ACID)
	}
}

// @ac AC-01
// Alternate annotation style: spec-id:AC-NN (colon separator)
func TestParseJUnit_ColonSeparator_Supported(t *testing.T) {
	xml := []byte(`<testsuites>
  <testsuite>
    <testcase name="engine-transaction:AC-05 test"/>
  </testsuite>
</testsuites>`)

	results, _ := ParseJUnit(xml)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SpecID != "engine-transaction" || results[0].ACID != "AC-05" {
		t.Errorf("got SpecID=%q ACID=%q", results[0].SpecID, results[0].ACID)
	}
}

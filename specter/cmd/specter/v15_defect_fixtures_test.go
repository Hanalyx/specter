// v15_defect_fixtures_test.go -- shared fixture builders and readers for the
// v0.15 phase 1B defect criteria (spec-coverage 1.16.0, spec-manifest 1.12.0,
// spec-doctor 1.10.0).
//
// Every builder takes each field a criterion varies as its own parameter. A
// spec's tier, its declared threshold, and its criterion list are independent
// inputs, and no fixture derives one from another. A case that keys on the
// wrong field therefore cannot pass by accident.
//
// This file carries no annotations of its own. It is infrastructure for the
// files that do.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Workspace builders
// ---------------------------------------------------------------------------

// defectSpec describes one fixture spec. threshold is a string so that "0",
// "66", and "omitted" are three distinct fixtures: the zero-threshold
// criteria turn on the difference between a declared 0 and an absent field,
// and an int field cannot express it.
type defectSpec struct {
	id        string
	tier      int
	acIDs     []string
	threshold string // empty means the spec omits coverage_threshold
}

// defectSpecYAML renders a valid spec with the given id, tier, criterion set,
// and optional declared threshold.
func defectSpecYAML(s defectSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spec:\n  id: %s\n  version: \"1.0.0\"\n  status: approved\n  tier: %d\n", s.id, s.tier)
	if s.threshold != "" {
		fmt.Fprintf(&b, "  coverage_threshold: %s\n", s.threshold)
	}
	fmt.Fprintf(&b, "\n  context:\n    system: %s control system\n    feature: %s defect fixture\n\n", s.id, s.id)
	fmt.Fprintf(&b, "  objective:\n    summary: Fixture workspace for the v0.15 phase 1B defect criteria.\n\n")
	b.WriteString("  constraints:\n    - id: C-01\n      description: \"MUST behave as the fixture describes\"\n      type: technical\n      enforcement: error\n\n")
	b.WriteString("  acceptance_criteria:\n")
	for _, ac := range s.acIDs {
		fmt.Fprintf(&b, "    - id: %s\n      description: \"Fixture criterion %s\"\n      references_constraints: [\"C-01\"]\n      priority: high\n", ac, ac)
	}
	return b.String()
}

// writeDefectSpec writes one fixture spec into dir/specs/.
func writeDefectSpec(t *testing.T, dir string, s defectSpec) {
	t.Helper()
	writeSpec(t, dir, s.id+".spec.yaml", defectSpecYAML(s))
}

// writeAnnotatedTestFile writes a source-comment annotated test file at
// dir/<rel>, creating parent directories. specID and acIDs are separate
// parameters so a fixture can annotate a different set of criteria than the
// spec declares.
func writeAnnotatedTestFile(t *testing.T, dir, rel, specID string, acIDs []string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "// @spec %s\n", specID)
	for _, ac := range acIDs {
		fmt.Fprintf(&b, "// @ac %s\n", ac)
	}
	b.WriteString("func TestFixture(t *testing.T) {}\n")
	writeRawFile(t, dir, rel, b.String())
}

// writeRawFile writes content at dir/<rel>, creating parent directories.
func writeRawFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// writeResultsJSON writes .specter-results.json verbatim. The criteria state
// their results files as literal JSON, so the fixture passes the literal
// through rather than re-encoding it.
func writeResultsJSON(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".specter-results.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write .specter-results.json: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Readers for `coverage --json`
// ---------------------------------------------------------------------------

// decodeCoverageJSON decodes the first JSON document on stdout.
func decodeCoverageJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var doc map[string]any
	dec := json.NewDecoder(strings.NewReader(stdout))
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout must parse as a JSON document, got %v; stdout:\n%s", err, stdout)
	}
	return doc
}

// coverageEntry returns the report entry for one spec id.
func coverageEntry(t *testing.T, doc map[string]any, specID string) map[string]any {
	t.Helper()
	entries, ok := doc["entries"].([]any)
	if !ok {
		t.Fatalf("coverage document must carry an `entries` array, got %T", doc["entries"])
	}
	for _, raw := range entries {
		e, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if e["spec_id"] == specID {
			return e
		}
	}
	t.Fatalf("coverage document has no entry for %s; entries: %v", specID, entries)
	return nil
}

// floatField reads a JSON number field.
func floatField(t *testing.T, obj map[string]any, name string) float64 {
	t.Helper()
	v, ok := obj[name].(float64)
	if !ok {
		t.Fatalf("field %s must be a number, got %T (%v)", name, obj[name], obj[name])
	}
	return v
}

// boolField reads a JSON boolean field.
func boolField(t *testing.T, obj map[string]any, name string) bool {
	t.Helper()
	v, ok := obj[name].(bool)
	if !ok {
		t.Fatalf("field %s must be a boolean, got %T (%v)", name, obj[name], obj[name])
	}
	return v
}

// stringsField reads a JSON array of strings. A missing or null field reads as
// the empty slice, because `omitempty` on an empty list is the same claim.
func stringsField(t *testing.T, obj map[string]any, name string) []string {
	t.Helper()
	raw, present := obj[name]
	if !present || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("field %s must be an array, got %T (%v)", name, raw, raw)
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("field %s must hold strings, got %T (%v)", name, v, v)
		}
		out = append(out, s)
	}
	return out
}

// nearly compares two percentages. The criteria state percentages to one
// decimal place; the tolerance keeps the comparison about the stated value
// rather than about float64 representation.
func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// ---------------------------------------------------------------------------
// Readers for the human-readable table
// ---------------------------------------------------------------------------

// coverageTableRowFields returns the whitespace-separated fields of the table
// row for one spec id. Column positions are not pinned: the criteria name a
// cell value, not a layout.
func coverageTableRowFields(t *testing.T, out, specID string) []string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == specID {
			return fields
		}
	}
	t.Fatalf("output has no coverage table row for %s; output:\n%s", specID, out)
	return nil
}

// rowHasField reports whether one of the row's cells is exactly want.
func rowHasField(fields []string, want string) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

// firstLine returns the first line of out.
func firstLine(out string) string {
	return strings.TrimRight(strings.SplitN(out, "\n", 2)[0], "\r")
}

// @spec spec-coverage
package coverage

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/Hanalyx/specter/internal/checker"
	"github.com/Hanalyx/specter/internal/schema"
)

// Helpers for AC-14 JSON-shape assertions. The AC invariant is structural
// ("a TypeScript consumer sees an array or absence, never null") so the
// assertion is also structural — inspect the rendered JSON text rather
// than the typed report.
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func containsJSONNull(data []byte, key string) bool {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:\s*null`)
	return re.Match(data)
}

func containsJSONEmptyArray(data []byte, key string) bool {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:\s*\[\s*\]`)
	return re.Match(data)
}

func containsJSONKey(data []byte, key string) bool {
	re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:`)
	return re.Match(data)
}

func makeSpec(id string, tier int, acIDs ...string) schema.SpecAST {
	var acs []schema.AcceptanceCriterion
	for _, aid := range acIDs {
		acs = append(acs, schema.AcceptanceCriterion{ID: aid, Description: "test"})
	}
	return schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: tier,
		Context:            schema.SpecContext{System: "test"},
		Objective:          schema.SpecObjective{Summary: "test"},
		Constraints:        []schema.Constraint{{ID: "C-01", Description: "test"}},
		AcceptanceCriteria: acs,
	}
}

// @ac AC-01
func TestAnnotationExtraction(t *testing.T) {
	t.Run("spec-coverage/AC-01 annotation extraction", func(t *testing.T) {
		content := "// @spec user-auth\n// @ac AC-01\n// @ac AC-02\n"
		matches := ExtractAnnotations(content, "test.ts")

		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].SpecID != "user-auth" {
			t.Errorf("expected spec_id 'user-auth', got %q", matches[0].SpecID)
		}
		if len(matches[0].ACIDs) != 2 {
			t.Errorf("expected 2 AC IDs, got %d", len(matches[0].ACIDs))
		}
	})
}

// @ac AC-01
func TestCoverageMapping(t *testing.T) {
	t.Run("spec-coverage/AC-01 coverage mapping", func(t *testing.T) {
		specs := []schema.SpecAST{
			makeSpec("user-auth", 2, "AC-01", "AC-02", "AC-03"),
		}
		anns := []AnnotationMatch{
			{File: "test.ts", SpecID: "user-auth", ACIDs: []string{"AC-01", "AC-02"}},
		}

		report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
		e := report.Entries[0]

		if len(e.CoveredACs) != 2 {
			t.Errorf("expected 2 covered ACs, got %d", len(e.CoveredACs))
		}
		if len(e.UncoveredACs) != 1 {
			t.Errorf("expected 1 uncovered AC, got %d", len(e.UncoveredACs))
		}
		if e.CoveragePct < 66.0 || e.CoveragePct > 67.0 {
			t.Errorf("expected ~66.7%%, got %.1f%%", e.CoveragePct)
		}
	})
}

// @ac AC-02
func TestZeroCoverage(t *testing.T) {
	t.Run("spec-coverage/AC-02 zero coverage", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpec("orphan", 2, "AC-01", "AC-02")}
		report := BuildCoverageReport(specs, nil, checker.CoverageThresholdByTier)

		if report.Entries[0].CoveragePct != 0 {
			t.Errorf("expected 0%%, got %.1f%%", report.Entries[0].CoveragePct)
		}
		if report.Summary.Uncovered != 1 {
			t.Errorf("expected 1 uncovered, got %d", report.Summary.Uncovered)
		}
	})
}

// @ac AC-03
func TestTier1Below100Fails(t *testing.T) {
	t.Run("spec-coverage/AC-03 tier 1 below 100 fails", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpec("payment", 1, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")}
		anns := []AnnotationMatch{
			{File: "test.ts", SpecID: "payment", ACIDs: []string{"AC-01", "AC-02", "AC-03", "AC-04"}},
		}

		report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
		e := report.Entries[0]

		if e.PassesThreshold {
			t.Error("expected Tier 1 at 80% to fail (threshold 100%)")
		}
		if e.Threshold != 100 {
			t.Errorf("expected threshold 100, got %d", e.Threshold)
		}
	})
}

// @ac AC-04
func TestTier3At60Passes(t *testing.T) {
	t.Run("spec-coverage/AC-04 tier 3 at 60 passes", func(t *testing.T) {
		specs := []schema.SpecAST{makeSpec("utils", 3, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")}
		anns := []AnnotationMatch{
			{File: "test.ts", SpecID: "utils", ACIDs: []string{"AC-01", "AC-02", "AC-03"}},
		}

		report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
		e := report.Entries[0]

		if !e.PassesThreshold {
			t.Error("expected Tier 3 at 60% to pass (threshold 50%)")
		}
	})
}

// @ac AC-09
func TestGapACsCountAsUncovered(t *testing.T) {
	t.Run("spec-coverage/AC-09 gap acs count as uncovered", func(t *testing.T) {
		// A spec where every AC is gap: true with no test annotations must fail
		// its tier threshold — it has zero captured intent and cannot silently pass.
		spec := schema.SpecAST{
			ID: "draft-spec", Version: "1.0.0", Status: "draft", Tier: 3,
			Context:     schema.SpecContext{System: "test"},
			Objective:   schema.SpecObjective{Summary: "test"},
			Constraints: []schema.Constraint{{ID: "C-01", Description: "test"}},
			AcceptanceCriteria: []schema.AcceptanceCriterion{
				{ID: "AC-01", Description: "reverse-extracted", Gap: true},
				{ID: "AC-02", Description: "reverse-extracted", Gap: true},
				{ID: "AC-03", Description: "reverse-extracted", Gap: true},
			},
		}

		report := BuildCoverageReport([]schema.SpecAST{spec}, nil, checker.CoverageThresholdByTier)
		e := report.Entries[0]

		if e.TotalACs != 3 {
			t.Errorf("expected 3 total ACs (gaps must count), got %d", e.TotalACs)
		}
		if len(e.UncoveredACs) != 3 {
			t.Errorf("expected 3 uncovered ACs, got %d", len(e.UncoveredACs))
		}
		if e.CoveragePct != 0 {
			t.Errorf("expected 0%% coverage, got %.1f%%", e.CoveragePct)
		}
		if e.PassesThreshold {
			t.Error("expected 100 percent-gap spec to fail threshold (tier 3 needs 50 percent)")
		}
	})
}

// @ac AC-05
func TestPythonAnnotations(t *testing.T) {
	t.Run("spec-coverage/AC-05 python annotations", func(t *testing.T) {
		content := "# @spec user-auth\n# @ac AC-01\n"
		matches := ExtractAnnotations(content, "test.py")

		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].SpecID != "user-auth" {
			t.Errorf("expected 'user-auth', got %q", matches[0].SpecID)
		}
	})
}

// Regression: BUG-001 — multiple AC IDs on a single @ac line must all be registered.
func TestAnnotationExtraction_MultiACOnOneLine(t *testing.T) {
	content := "// @spec deadman-timer\n// @ac AC-02 AC-03 AC-04\n"
	matches := ExtractAnnotations(content, "timer_test.go")

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	acSet := make(map[string]bool)
	for _, id := range matches[0].ACIDs {
		acSet[id] = true
	}
	for _, want := range []string{"AC-02", "AC-03", "AC-04"} {
		if !acSet[want] {
			t.Errorf("expected %s to be covered from single-line @ac annotation, but it was not", want)
		}
	}
}

// @ac AC-06
func TestPerSpecCoverageThreshold_OverridesTierDefault(t *testing.T) {
	t.Run("spec-coverage/AC-06 per spec coverage threshold overrides tier default", func(t *testing.T) {
		// Tier 1 spec (default threshold 100%) with coverage_threshold: 75
		// 4 of 5 ACs covered = 80%. Should PASS because 80 >= 75.
		spec := makeSpec("payment", 1, "AC-01", "AC-02", "AC-03", "AC-04", "AC-05")
		spec.CoverageThreshold = 75
		specs := []schema.SpecAST{spec}
		anns := []AnnotationMatch{
			{File: "t.go", SpecID: "payment", ACIDs: []string{"AC-01", "AC-02", "AC-03", "AC-04"}},
		}
		report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
		e := report.Entries[0]
		if e.Threshold != 75 {
			t.Errorf("expected threshold 75 (per-spec override), got %d", e.Threshold)
		}
		if !e.PassesThreshold {
			t.Errorf("expected to pass at 80%% with threshold 75")
		}
	})
}

// @ac AC-07
func TestPassRateAwareCoverage_FailedResultNotCounted(t *testing.T) {
	t.Run("spec-coverage/AC-07 pass rate aware coverage failed result not counted", func(t *testing.T) {
		// Tier 1 spec: annotation exists but result entry says passed: false
		spec := makeSpec("engine", 1, "AC-01")
		anns := []AnnotationMatch{
			{File: "t.go", SpecID: "engine", ACIDs: []string{"AC-01"}},
		}
		results := &ResultsFile{
			Results: []ResultEntry{{SpecID: "engine", ACID: "AC-01", Passed: false}},
		}
		report := BuildCoverageReportWithResults([]schema.SpecAST{spec}, anns, checker.CoverageThresholdByTier, results)
		e := report.Entries[0]
		if e.CoveragePct != 0 {
			t.Errorf("expected 0%% coverage when result failed, got %.1f%%", e.CoveragePct)
		}
	})
}

// @ac AC-08
func TestCheckDependencyCoverage_EmitsWarning(t *testing.T) {
	t.Run("spec-coverage/AC-08 check dependency coverage emits warning", func(t *testing.T) {
		// spec A depends on spec B; spec B is below threshold
		specA := makeSpec("spec-a", 1, "AC-01")
		specB := makeSpec("spec-b", 1, "AC-01")
		specs := []schema.SpecAST{specA, specB}
		// Only spec A has annotations; spec B has none
		anns := []AnnotationMatch{
			{File: "t.go", SpecID: "spec-a", ACIDs: []string{"AC-01"}},
		}
		report := BuildCoverageReport(specs, anns, checker.CoverageThresholdByTier)
		edges := []DepEdge{{From: "spec-a", To: "spec-b"}}
		warnings := CheckDependencyCoverage(edges, report)
		if len(warnings) != 1 {
			t.Fatalf("expected 1 dependency_coverage warning, got %d", len(warnings))
		}
		if warnings[0].DependsOn != "spec-b" {
			t.Errorf("expected warning about spec-b, got %q", warnings[0].DependsOn)
		}
	})
}

// Regression: @spec inside a string literal must not hijack the current spec context.
func TestAnnotationExtraction_StringLiteralNotHijacked(t *testing.T) {
	// Simulate a test file where a Go string literal contains "// @spec other-spec".
	// The annotation extractor must not switch context to "other-spec".
	content := `// @spec my-spec
// @ac AC-01
func TestFoo(t *testing.T) {
	content := "// @spec other-spec\n// @ac AC-02\n"
	_ = content
}
// @ac AC-02
func TestBar(t *testing.T) {}
`
	matches := ExtractAnnotations(content, "foo_test.go")

	// Find the my-spec entry
	var mySpec *AnnotationMatch
	for i := range matches {
		if matches[i].SpecID == "my-spec" {
			mySpec = &matches[i]
		}
	}
	if mySpec == nil {
		t.Fatal("expected annotation for my-spec, got none")
	}

	// Both AC-01 and AC-02 should be under my-spec, not hijacked to other-spec
	acSet := make(map[string]bool)
	for _, id := range mySpec.ACIDs {
		acSet[id] = true
	}
	if !acSet["AC-01"] {
		t.Error("expected AC-01 under my-spec")
	}
	if !acSet["AC-02"] {
		t.Error("expected AC-02 under my-spec (string literal must not hijack spec context)")
	}

	// other-spec must not appear in results
	for _, m := range matches {
		if m.SpecID == "other-spec" {
			t.Error("other-spec from inside a string literal must not appear in results")
		}
	}
}

// @ac AC-11
// v1.5.0: a `// @spec` sequence appearing inside a multi-line TypeScript
// template literal (backtick) must not be parsed as a real annotation. The
// real `// @spec` on a proper comment line MUST still be detected.
func TestAnnotationExtraction_TemplateLiteralNotHijacked(t *testing.T) {
	t.Run("spec-coverage/AC-11 annotation extraction template literal not hijacked", func(t *testing.T) {
		content := "// @spec real-spec\n" +
			"// @ac AC-01\n" +
			"const payload = `\n" +
			"  // @spec ghost-spec\n" +
			"  // @ac AC-99\n" +
			"`;\n" +
			"// @ac AC-02\n"

		matches := ExtractAnnotations(content, "example.test.ts")

		for _, m := range matches {
			if m.SpecID == "ghost-spec" {
				t.Fatal("ghost-spec inside a template literal must not produce an annotation")
			}
		}

		// The real spec should still be present, and a @ac line AFTER the
		// template literal must still attach to real-spec (state preserved).
		var real *AnnotationMatch
		for i := range matches {
			if matches[i].SpecID == "real-spec" {
				real = &matches[i]
			}
		}
		if real == nil {
			t.Fatal("real-spec should still be detected around the template literal")
		}
		has := func(id string) bool {
			for _, a := range real.ACIDs {
				if a == id {
					return true
				}
			}
			return false
		}
		if !has("AC-01") {
			t.Error("expected AC-01 on real-spec")
		}
		if !has("AC-02") {
			t.Errorf("expected AC-02 on real-spec (after template literal closes), got %v", real.ACIDs)
		}
		if has("AC-99") {
			t.Error("AC-99 came from inside a template literal and must not be attached to real-spec")
		}
	})
}

// @ac AC-13
// SummarizeParseErrors groups entries by (type, path) and sorts by count desc.
// Enables one-sentence drift diagnosis ("20 specs missing objective").
func TestSummarizeParseErrors_GroupsAndSorts(t *testing.T) {
	t.Run("spec-coverage/AC-13 summarize parse errors groups and sorts", func(t *testing.T) {
		entries := []ParseErrorEntry{
			{File: "a.yaml", Type: "required", Path: "spec.objective", Message: "missing"},
			{File: "b.yaml", Type: "required", Path: "spec.objective", Message: "missing"},
			{File: "c.yaml", Type: "required", Path: "spec.objective", Message: "missing"},
			{File: "d.yaml", Type: "enum", Path: "spec.status", Message: "bad"},
		}
		patterns := SummarizeParseErrors(entries)
		if len(patterns) != 2 {
			t.Fatalf("expected 2 patterns, got %d", len(patterns))
		}
		if patterns[0].Type != "required" || patterns[0].Path != "spec.objective" {
			t.Errorf("expected most-frequent pattern first, got %+v", patterns[0])
		}
		if patterns[0].Count != 3 {
			t.Errorf("expected count 3 for top pattern, got %d", patterns[0].Count)
		}
		if len(patterns[0].Files) != 3 {
			t.Errorf("expected 3 files for top pattern, got %d", len(patterns[0].Files))
		}
		if patterns[1].Type != "enum" {
			t.Errorf("expected enum pattern second, got %+v", patterns[1])
		}
	})
}

func TestSummarizeParseErrors_DedupesFilesWithinPattern(t *testing.T) {
	// Same file hit by the same pattern twice (e.g. two fields missing
	// from the same spec) must appear once in Files, count twice.
	entries := []ParseErrorEntry{
		{File: "a.yaml", Type: "required", Path: "spec.objective", Message: "m1"},
		{File: "a.yaml", Type: "required", Path: "spec.objective", Message: "m2"},
	}
	patterns := SummarizeParseErrors(entries)
	if len(patterns[0].Files) != 1 {
		t.Errorf("expected Files deduped to 1, got %d", len(patterns[0].Files))
	}
	if patterns[0].Count != 2 {
		t.Errorf("expected Count 2, got %d", patterns[0].Count)
	}
}

func TestSummarizeParseErrors_EmptyInput(t *testing.T) {
	if got := SummarizeParseErrors(nil); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

// @ac AC-11
// Python multi-line string (triple-double) must not bleed annotations.
func TestAnnotationExtraction_PythonTripleQuoteNotHijacked(t *testing.T) {
	t.Run("spec-coverage/AC-11 annotation extraction python triple quote not hijacked", func(t *testing.T) {
		content := "# @spec real-py\n" +
			"# @ac AC-01\n" +
			"docstring = \"\"\"\n" +
			"# @spec ghost-py\n" +
			"# @ac AC-99\n" +
			"\"\"\"\n" +
			"# @ac AC-02\n"

		matches := ExtractAnnotations(content, "example_test.py")

		for _, m := range matches {
			if m.SpecID == "ghost-py" {
				t.Fatal("ghost-py inside a triple-quoted string must not produce an annotation")
			}
		}

		var real *AnnotationMatch
		for i := range matches {
			if matches[i].SpecID == "real-py" {
				real = &matches[i]
			}
		}
		if real == nil {
			t.Fatal("real-py should still be detected around the triple-quoted string")
		}
		has := func(id string) bool {
			for _, a := range real.ACIDs {
				if a == id {
					return true
				}
			}
			return false
		}
		if !has("AC-01") || !has("AC-02") {
			t.Errorf("expected AC-01 and AC-02 on real-py, got %v", real.ACIDs)
		}
		if has("AC-99") {
			t.Error("AC-99 from inside triple-quoted string must not be attached to real-py")
		}
	})
}

// @ac AC-14
// Coverage report emits `[]` for empty array fields without omitempty. A
// TypeScript consumer that declares `uncoveredACs: string[]` MUST never see
// `null` at runtime — that's a silent contract violation class.
func TestCoverageReport_EmitsEmptyArrayNotNull(t *testing.T) {
	t.Run("spec-coverage/AC-14 coverage report emits empty array not null", func(t *testing.T) {
		// One spec, both ACs covered by one annotation. Post-build entry.UncoveredACs
		// will be a zero-valued slice; the JSON marshal MUST emit `[]`, not `null`.
		spec := makeSpec("all-covered", 3, "AC-01", "AC-02")
		annotations := []AnnotationMatch{
			{File: "test.go", SpecID: "all-covered", ACIDs: []string{"AC-01", "AC-02"}},
		}
		report := BuildCoverageReport([]schema.SpecAST{spec}, annotations, map[int]int{3: 50})

		data, err := marshalJSON(report)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		// The JSON output MUST NOT contain `"uncovered_acs": null`.
		if containsJSONNull(data, "uncovered_acs") {
			t.Fatalf("uncovered_acs emitted as null; expected []. payload:\n%s", data)
		}
		// Positive: it MUST contain `"uncovered_acs": []`.
		if !containsJSONEmptyArray(data, "uncovered_acs") {
			t.Fatalf("uncovered_acs not emitted as []; payload:\n%s", data)
		}
	})
}

// @ac AC-14
// Omitempty fields stay absent (not null) when empty. parse_errors and
// parse_error_patterns are optional on the top-level CoverageReport.
func TestCoverageReport_OmitemptyFieldsAbsentNotNull(t *testing.T) {
	t.Run("spec-coverage/AC-14 coverage report omitempty fields absent not null", func(t *testing.T) {
		report := &CoverageReport{
			Entries: []SpecCoverageEntry{},
			Summary: CoverageSummary{},
		}
		data, err := marshalJSON(report)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		// These fields MUST be absent entirely — not emitted as `null`.
		if containsJSONKey(data, "parse_errors") {
			t.Errorf("parse_errors appeared in output; must be absent when nil/empty. payload:\n%s", data)
		}
		if containsJSONKey(data, "parse_error_patterns") {
			t.Errorf("parse_error_patterns appeared in output; must be absent when nil/empty. payload:\n%s", data)
		}
	})
}

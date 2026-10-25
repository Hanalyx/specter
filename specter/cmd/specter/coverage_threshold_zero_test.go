// coverage_threshold_zero_test.go -- CLI integration tests for spec-coverage
// 1.16.0 C-36: a declared coverage threshold of 0 means zero rather than the
// tier default (AC-49 through AC-52).
//
// C-36 says the effective number is the `threshold` field of `coverage
// --json`, so every case here reads it rather than inferring it from a pass
// or a fail. The two override layers are exercised separately (AC-49 and
// AC-51) and then together (AC-52), because a fix applied at one layer leaves
// the other falling through.
//
// @spec spec-coverage
package main

import "testing"

// thresholdZeroManifest renders a manifest whose only settings entry is the
// tier 2 coverage threshold. The strictness level comes from the command line
// in every case here, so the manifest carries exactly the one fact the
// criterion names.
func thresholdZeroManifest(t *testing.T, dir, tier2 string) {
	t.Helper()
	writeManifest(t, dir, "schema_version: 1\nsystem:\n  name: demo\nsettings:\n  coverage:\n    tier2: "+tier2+"\n")
}

// ---------------------------------------------------------------------------
// AC-49 -- a spec-level declared zero
// ---------------------------------------------------------------------------

// Tier 1's built-in default is 100 and the spec covers none of its criteria,
// so the run passes only if the declared 0 was read. There is no manifest, so
// nothing but the spec field can supply the threshold.
//
// @ac AC-49
func TestCoverageThresholdZero_SpecLevelDeclaredZero(t *testing.T) {
	t.Run("spec-coverage/AC-49 a tier 1 spec declaring coverage_threshold 0 passes with nothing covered", func(t *testing.T) {
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{
			id:        "demo-spec",
			tier:      1,
			acIDs:     []string{"AC-01", "AC-02"},
			threshold: "0",
		})

		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
		entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")

		if got := floatField(t, entry, "threshold"); !nearly(got, 0) {
			t.Errorf("threshold = %v, want 0", got)
		}
		if got := floatField(t, entry, "coverage_pct"); !nearly(got, 0) {
			t.Errorf("coverage_pct = %v, want 0", got)
		}
		if !boolField(t, entry, "passes_threshold") {
			t.Errorf("passes_threshold = false, want true")
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-50 -- an omitted field is not a declared zero
// ---------------------------------------------------------------------------

// This is the control for AC-49 and the trap for a fix built on a
// greater-than-zero test replaced by a zero-valued test. The spec is tier 1
// and omits the field entirely, so the tier default must still apply. The
// coverage_pct assertion is here because the criterion states it; it also
// keeps the case from passing on a run that reported no coverage at all.
//
// @ac AC-50
func TestCoverageThresholdZero_OmittedFieldInheritsTheTierDefault(t *testing.T) {
	t.Run("spec-coverage/AC-50 a tier 1 spec omitting coverage_threshold keeps the tier default of 100", func(t *testing.T) {
		dir := t.TempDir()
		writeDefectSpec(t, dir, defectSpec{
			id:    "demo-spec",
			tier:  1,
			acIDs: []string{"AC-01", "AC-02", "AC-03"},
		})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})

		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
		entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")

		if got := floatField(t, entry, "threshold"); !nearly(got, 100) {
			t.Errorf("threshold = %v, want 100;\nstderr:\n%s", got, stderr)
		}
		if got := floatField(t, entry, "coverage_pct"); !nearly(got, 66.7) {
			t.Errorf("coverage_pct = %v, want 66.7", got)
		}
		if boolField(t, entry, "passes_threshold") {
			t.Errorf("passes_threshold = true, want false")
		}
	})
}

// ---------------------------------------------------------------------------
// AC-51 -- a manifest-level declared zero
// ---------------------------------------------------------------------------

// The spec omits its own override, so the manifest tier value is the only
// place the threshold can come from. Tier 2's built-in default is 80 and the
// spec covers 2 of 3, so the run passes only if the manifest 0 was read.
//
// @ac AC-51
func TestCoverageThresholdZero_ManifestTierDeclaredZero(t *testing.T) {
	t.Run("spec-coverage/AC-51 settings.coverage.tier2 of 0 gives a tier 2 spec a threshold of 0", func(t *testing.T) {
		dir := t.TempDir()
		thresholdZeroManifest(t, dir, "0")
		writeDefectSpec(t, dir, defectSpec{
			id:    "demo-spec",
			tier:  2,
			acIDs: []string{"AC-01", "AC-02", "AC-03"},
		})
		writeAnnotatedTestFile(t, dir, "tests/a_test.go", "demo-spec", []string{"AC-01", "AC-02"})

		stdout, stderr, code := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
		entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")

		if got := floatField(t, entry, "threshold"); !nearly(got, 0) {
			t.Errorf("threshold = %v, want 0", got)
		}
		if !boolField(t, entry, "passes_threshold") {
			t.Errorf("passes_threshold = false, want true")
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0;\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// AC-52 -- precedence with both layers present in one run
// ---------------------------------------------------------------------------

// Both specs are tier 2 and both cover 2 of 3, so tier and coverage are held
// constant and the declared spec-level override is the only difference
// between them. The manifest value is 40 rather than 0, so the two expected
// thresholds are distinguishable: a fix that lets the spec-level 0 fall
// through reports 40 for both, and a fix that applies the spec-level 0 to
// every entry reports 0 for both.
//
// @ac AC-52
func TestCoverageThresholdZero_PrecedenceWithBothLayers(t *testing.T) {
	t.Run("spec-coverage/AC-52 a declared 0 wins over a manifest tier value of 40 in the same report", func(t *testing.T) {
		dir := t.TempDir()
		thresholdZeroManifest(t, dir, "40")
		writeDefectSpec(t, dir, defectSpec{
			id:        "alpha-spec",
			tier:      2,
			acIDs:     []string{"AC-01", "AC-02", "AC-03"},
			threshold: "0",
		})
		writeAnnotatedTestFile(t, dir, "tests/alpha_test.go", "alpha-spec", []string{"AC-01", "AC-02"})
		writeDefectSpec(t, dir, defectSpec{
			id:    "beta-spec",
			tier:  2,
			acIDs: []string{"AC-01", "AC-02", "AC-03"},
		})
		writeAnnotatedTestFile(t, dir, "tests/beta_test.go", "beta-spec", []string{"AC-01", "AC-02"})

		stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
		doc := decodeCoverageJSON(t, stdout)

		alpha := coverageEntry(t, doc, "alpha-spec")
		if got := floatField(t, alpha, "threshold"); !nearly(got, 0) {
			t.Errorf("alpha-spec threshold = %v, want 0;\nstderr:\n%s", got, stderr)
		}
		if !boolField(t, alpha, "passes_threshold") {
			t.Errorf("alpha-spec passes_threshold = false, want true")
		}

		beta := coverageEntry(t, doc, "beta-spec")
		if got := floatField(t, beta, "threshold"); !nearly(got, 40) {
			t.Errorf("beta-spec threshold = %v, want 40", got)
		}
		if !boolField(t, beta, "passes_threshold") {
			t.Errorf("beta-spec passes_threshold = false, want true")
		}

		entries, ok := doc["entries"].([]any)
		if !ok || len(entries) != 2 {
			t.Errorf("report must carry 2 entries, got %v", doc["entries"])
		}
	})
}

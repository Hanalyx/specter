// explain_bundle_test.go -- v0.11 explain bundle: annotation, schema, AC-less spec card.
//
// @spec spec-explain
package main

import (
	"strings"
	"testing"
)

// @ac AC-07
func TestExplainAnnotation_PrintsReference(t *testing.T) {
	t.Run("spec-explain/AC-07 annotation reference covers Convention A and B", func(t *testing.T) {
		dir := t.TempDir()
		out, code := runCLI(t, dir, "explain", "annotation")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d; output:\n%s", code, out)
		}
		if !strings.Contains(out, "Convention A") {
			t.Errorf("expected 'Convention A' section in output, got:\n%s", out)
		}
		if !strings.Contains(out, "Convention B") {
			t.Errorf("expected 'Convention B' section in output, got:\n%s", out)
		}
		// Convention A is runner-visible spec-id/AC-NN — must show a t.Run / describe style example.
		if !strings.Contains(out, "t.Run(") && !strings.Contains(out, "describe(") {
			t.Errorf("expected a runner-visible example (t.Run or describe) in Convention A output, got:\n%s", out)
		}
		// Convention B is source-comment style.
		if !strings.Contains(out, "// @spec") && !strings.Contains(out, "# @spec") {
			t.Errorf("expected source-comment annotation example in Convention B output, got:\n%s", out)
		}
	})
}

// @ac AC-08
func TestExplainSchema_FullReference(t *testing.T) {
	t.Run("spec-explain/AC-08 schema reference enumerates top-level fields", func(t *testing.T) {
		dir := t.TempDir()
		out, code := runCLI(t, dir, "explain", "schema")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d; output:\n%s", code, out)
		}
		// Every required top-level spec field from the embedded JSON schema must appear.
		// These are declared as required in internal/parser/spec-schema.json.
		requiredFields := []string{
			"id",
			"version",
			"status",
			"tier",
			"context",
			"objective",
			"constraints",
			"acceptance_criteria",
		}
		for _, field := range requiredFields {
			if !strings.Contains(out, field) {
				t.Errorf("expected schema reference to contain field %q, got:\n%s", field, out)
			}
		}
	})
}

// @ac AC-09
func TestExplainSchema_FieldPath_Unknown(t *testing.T) {
	t.Run("spec-explain/AC-09 unknown schema field path exits nonzero with did-you-mean", func(t *testing.T) {
		dir := t.TempDir()
		// Deliberate typo: accptance_criteria → acceptance_criteria (Levenshtein 1).
		out, code := runCLI(t, dir, "explain", "schema", "spec.accptance_criteria")

		if code == 0 {
			t.Fatalf("expected nonzero exit, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "unknown field path") {
			t.Errorf("expected 'unknown field path' in error output, got:\n%s", out)
		}
		// Close spelling → expect did-you-mean.
		if !strings.Contains(strings.ToLower(out), "did you mean") {
			t.Errorf("expected 'did you mean' suggestion for close typo, got:\n%s", out)
		}
	})
}

// @ac AC-10
func TestExplainSpecCard_RendersTierAndCoverage(t *testing.T) {
	t.Run("spec-explain/AC-10 spec card renders tier coverage and test files", func(t *testing.T) {
		dir := setupExplainDir(t, []string{"AC-01"}, "_test.go")
		out, code := runCLI(t, dir, "explain", "my-spec")

		if code != 0 {
			t.Fatalf("expected exit 0, got %d; output:\n%s", code, out)
		}
		// Spec card must show tier.
		if !strings.Contains(strings.ToLower(out), "tier") {
			t.Errorf("expected 'tier' in spec card output, got:\n%s", out)
		}
		// Spec card must show coverage percentage (e.g., "50%").
		if !strings.Contains(out, "%") {
			t.Errorf("expected coverage percentage in spec card output, got:\n%s", out)
		}
		// Spec card must name the test file covering AC-01. The test harness
		// appends the extension to a "my_spec_test" basename, yielding
		// my_spec_test_test.go — treat any *_test.go name as satisfactory.
		if !strings.Contains(out, "_test.go") {
			t.Errorf("expected a _test.go filename in spec card output, got:\n%s", out)
		}
	})
}

// Pure-function tests for the body-text annotation extractor (C-12, AC-12).
// Closes GH #79 — the body extractor now accepts //, #, and * comment
// markers identically, mirroring internal/coverage's source-file scanner.
//
// @spec spec-ingest
package ingest

import "testing"

// @ac AC-12
func TestExtractAnnotations_PythonHashMarker(t *testing.T) {
	t.Run("spec-ingest/AC-12 # @spec body marker extracts identically to // form", func(t *testing.T) {
		body := "# @spec my-spec\n# @ac AC-01\n"
		specID, acID := extractAnnotations("anonymous", "anonymous", body)
		if specID != "my-spec" || acID != "AC-01" {
			t.Errorf("expected (my-spec, AC-01), got (%q, %q)", specID, acID)
		}
	})
}

// @ac AC-12
func TestExtractAnnotations_GoSlashMarker(t *testing.T) {
	t.Run("spec-ingest/AC-12 // @spec body marker still extracts (regression)", func(t *testing.T) {
		body := "// @spec my-spec\n// @ac AC-01\n"
		specID, acID := extractAnnotations("anonymous", "anonymous", body)
		if specID != "my-spec" || acID != "AC-01" {
			t.Errorf("expected (my-spec, AC-01), got (%q, %q)", specID, acID)
		}
	})
}

// @ac AC-12
func TestExtractAnnotations_JSDocStarMarker(t *testing.T) {
	t.Run("spec-ingest/AC-12 * @spec body marker extracts identically to // form", func(t *testing.T) {
		body := "* @spec my-spec\n* @ac AC-01\n"
		specID, acID := extractAnnotations("anonymous", "anonymous", body)
		if specID != "my-spec" || acID != "AC-01" {
			t.Errorf("expected (my-spec, AC-01), got (%q, %q)", specID, acID)
		}
	})
}

// @ac AC-12
func TestExtractAnnotations_MixedMarkersInOneBody(t *testing.T) {
	t.Run("spec-ingest/AC-12 mixed markers in one body resolve to first match", func(t *testing.T) {
		// Defensive case: a CI log might contain `// @spec` from a Go subprocess
		// followed by `# @spec` from a Python subprocess. The regex matches the
		// first occurrence — verify deterministic behavior.
		body := "// @spec go-spec\n# @ac AC-01\n"
		specID, acID := extractAnnotations("anonymous", "anonymous", body)
		// First @spec wins (//), but @ac wins independently (#).
		if specID != "go-spec" {
			t.Errorf("expected first-match SpecID = go-spec, got %q", specID)
		}
		if acID != "AC-01" {
			t.Errorf("expected ACID = AC-01, got %q", acID)
		}
	})
}

// Regression guard: prose mentions of @spec must NOT extract if there's no
// comment marker. Same class as the test-annotation regex tightening in
// internal/checker (PR #74) — silent over-matching of free-text would
// false-positive on commit messages or doc strings captured into JUnit
// system-out by a misconfigured runner.
func TestExtractAnnotations_ProseMentionWithoutMarker_NoExtract(t *testing.T) {
	t.Run("spec-ingest/AC-12 prose mention without // # or * does not extract", func(t *testing.T) {
		body := "fixes the @spec my-spec issue mentioned in @ac AC-01"
		specID, acID := extractAnnotations("anonymous", "anonymous", body)
		if specID != "" || acID != "" {
			t.Errorf("expected no extraction from prose, got (%q, %q)", specID, acID)
		}
	})
}

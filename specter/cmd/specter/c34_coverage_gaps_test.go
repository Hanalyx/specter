package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unreachableWorkspace builds the workspace AC-61 names: one spec with one
// acceptance criterion, and one parseable Go test carrying @spec and @ac source
// comments whose name produces no runner-visible token. That yields a real
// unreachable_annotation rather than the _unknown fallback, which matters
// because _unknown is always a warning and would make the invariance trivial.
func unreachableWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, "ua-spec.spec.yaml", minimalValidSpec("ua-spec", 2, "AC-01"))
	body := "package tests\n\nimport \"testing\"\n\n// @spec ua-spec\n// @ac AC-01\nfunc TestNoToken(t *testing.T) {}\n"
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests", "ua_test.go"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// uaManifest builds a manifest varying exactly two things: the strictness value
// and whether an annotation block is declared. Every other byte is identical.
func uaManifest(strictness string, withBlock bool) string {
	body := "system:\n  name: ua\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n  strictness: " + strictness + "\n"
	if withBlock {
		body += "  annotation:\n    permissive: false\n"
	}
	return body
}

// severityOf reports how check --test rendered unreachable_annotation, or
// "suppressed" when it did not render it at all.
func severityOf(out string) string {
	switch {
	case strings.Contains(out, "error [unreachable_annotation]"):
		return "error"
	case strings.Contains(out, "warn [unreachable_annotation]"):
		return "warning"
	default:
		return "suppressed"
	}
}

// @spec spec-manifest
// @ac AC-61
//
// C-34(d) on check --test. AC-58 exercises coverage alone, so before this
// criterion existed, reverting the GoverningStrictness routing survived the
// whole suite while changing a user's severity and exit code.
func TestC34d_HoldsOnCheckTest(t *testing.T) {
	t.Run("spec-manifest/AC-61 c34d holds on check --test", func(t *testing.T) {
		levels := []string{"annotation", "threshold", "zero-tolerance"}

		// Precondition. Without the block the three values must diverge, or
		// the invariance below is a fact about the workspace rather than
		// about the block.
		wantSeverity := map[string]string{"annotation": "suppressed", "threshold": "warning", "zero-tolerance": "error"}
		wantCode := map[string]int{"annotation": 0, "threshold": 0, "zero-tolerance": 1}
		for _, lvl := range levels {
			dir := unreachableWorkspace(t)
			putManifest(t, dir, uaManifest(lvl, false))
			stdout, stderr, code := runCLISplit(t, dir, "check", "--test")
			got := severityOf(stdout + stderr)
			if got != wantSeverity[lvl] {
				t.Errorf("precondition, no block, strictness %s: severity = %q, want %q", lvl, got, wantSeverity[lvl])
			}
			if code != wantCode[lvl] {
				t.Errorf("precondition, no block, strictness %s: exit = %d, want %d", lvl, code, wantCode[lvl])
			}
		}

		// Assertion. With the block declared, nothing observable may vary
		// with the strictness value.
		var firstSeverity string
		var firstCode int
		for i, lvl := range levels {
			dir := unreachableWorkspace(t)
			putManifest(t, dir, uaManifest(lvl, true))
			stdout, stderr, code := runCLISplit(t, dir, "check", "--test")
			sev := severityOf(stdout + stderr)
			if i == 0 {
				firstSeverity, firstCode = sev, code
				continue
			}
			if sev != firstSeverity {
				t.Errorf("block declared: strictness %s gave severity %q, strictness %s gave %q; C-34(d) requires invariance",
					levels[0], firstSeverity, lvl, sev)
			}
			if code != firstCode {
				t.Errorf("block declared: strictness %s gave exit %d, strictness %s gave %d; C-34(d) requires invariance",
					levels[0], firstCode, lvl, code)
			}
		}
	})
}

// @spec spec-manifest
// @ac AC-62
//
// The --strictness flag outranks a declared annotation block, per SSRB-104
// section 7.8. Before this criterion, rewriting both call sites so the block
// outranks the flag also passed the whole suite.
func TestFlagOutranksAnnotationBlock(t *testing.T) {
	t.Run("spec-manifest/AC-62 flag outranks a declared annotation block", func(t *testing.T) {
		// Manifest declares the block and NO strictness key, so the block is
		// the only manifest-side input. No results file exists.
		body := "system:\n  name: prec\nsettings:\n  specs_dir: specs\n  annotation:\n    permissive: false\n"

		cases := []struct {
			name string
			args []string
			want int
		}{
			{"coverage, no flag: the block governs and selects the strict path", []string{"coverage"}, 1},
			{"coverage, flag wins and selects the lenient path", []string{"coverage", "--strictness", "annotation"}, 0},
			{"sync, no flag", []string{"sync"}, 1},
			{"sync, flag wins", []string{"sync", "--strictness", "annotation"}, 0},
		}
		for _, c := range cases {
			dir := annotationWorkspace(t)
			putManifest(t, dir, body)
			_, _, code := runCLISplit(t, dir, c.args...)
			if code != c.want {
				t.Errorf("%s: exit = %d, want %d", c.name, code, c.want)
			}
		}
	})
}

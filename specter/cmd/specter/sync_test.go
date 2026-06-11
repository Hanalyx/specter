// sync_test.go -- CLI integration tests for `specter sync` test-file handling.
//
// @spec spec-sync
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec-sync C-10 / AC-14 — parity fix for the v0.13 H5 hardening.
//
// A test file larger than MaxTestFileBytes (4 MiB) discovered by
// `sync` MUST be skipped — the file is NOT read into memory, a stderr
// warning names the file, and the run continues with remaining files.
// The skip alone MUST NOT fail the pipeline. Closes the gap left by
// commit 8b76961, which stat-guarded check, coverage, and explain but
// never touched sync's read loop: a large test artifact could OOM the
// primary CI command.
//
// @ac AC-14
func TestSync_OversizedTestFileSkipped(t *testing.T) {
	t.Run("spec-sync/AC-14 sync skips test file larger than MaxTestFileBytes", func(t *testing.T) {
		dir := t.TempDir()
		writeSpec(t, dir, "x.spec.yaml", minimalValidSpec("x-spec", 3, "AC-01"))

		// 4 MiB + 1 byte — one past the cap. Content is arbitrary
		// non-Go bytes; the stat guard must fire before any read.
		const cap = 4 << 20
		body := bytes.Repeat([]byte("x"), cap+1)
		oversized := filepath.Join(dir, "huge_test.go")
		if err := os.WriteFile(oversized, body, 0644); err != nil {
			t.Fatal(err)
		}

		// A small, valid test file annotating x-spec/AC-01. This file
		// MUST still be scanned (skip applies per-file, not whole run),
		// so the coverage phase passes from this file alone.
		small := "package foo\n\nimport \"testing\"\n\n// @spec x-spec\n// @ac AC-01\nfunc TestSmall(t *testing.T) {\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "small_test.go"), []byte(small), 0644); err != nil {
			t.Fatal(err)
		}

		// --strictness annotation: coverage counts source annotations,
		// no .specter-results.json required (spec-sync C-08/AC-11).
		out, code := runCLI(t, dir, "sync", "--strictness", "annotation")

		// The skip itself must not fail the pipeline: the spec parses,
		// resolve/check pass, and coverage is satisfied by the small
		// file's annotation (tier 3 threshold 50%, actual 100%).
		if code != 0 {
			t.Errorf("oversized test file skip should not drive non-zero exit (got %d); output:\n%s", code, out)
		}

		// Warning must name the offending file and contain the skip
		// reason. Per AC-14 the message contains `exceeds` and
		// `skipping` — the same operator vocabulary as `check --test`
		// and `coverage` (spec-check C-12).
		if !strings.Contains(out, "huge_test.go") {
			t.Errorf("expected warning to name the oversized file `huge_test.go`, got:\n%s", out)
		}
		if !strings.Contains(out, "exceeds") || !strings.Contains(out, "skipping") {
			t.Errorf("expected warning to contain `exceeds` and `skipping`, got:\n%s", out)
		}

		// Coverage MUST still be computed from the remaining in-cap
		// files: the coverage phase passes via small_test.go's
		// annotation, and the pipeline reports overall success.
		if !strings.Contains(out, "PASS coverage") {
			t.Errorf("expected coverage phase to pass from remaining files, got:\n%s", out)
		}
		if !strings.Contains(out, "All checks passed.") {
			t.Errorf("expected pipeline success despite the skip, got:\n%s", out)
		}
	})

	t.Run("spec-sync/AC-14 file at exactly the cap is scanned", func(t *testing.T) {
		// Boundary condition: len == cap (4 MiB) must scan. The guard
		// is `size > MaxTestFileBytes`, matching check --test exactly.
		dir := t.TempDir()
		writeSpec(t, dir, "x.spec.yaml", minimalValidSpec("x-spec", 3, "AC-01"))

		header := "package foo\n\nimport \"testing\"\n\n// @spec x-spec\n// @ac AC-01\nfunc TestBoundary(t *testing.T) {\n}\n// padding:\n"
		const cap = 4 << 20
		padding := bytes.Repeat([]byte("/"), cap-len(header))
		body := append([]byte(header), padding...)
		if err := os.WriteFile(filepath.Join(dir, "boundary_test.go"), body, 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "sync", "--strictness", "annotation")

		if code != 0 {
			t.Errorf("file at exactly the cap should be scanned and sync should pass (got %d); output:\n%s", code, out)
		}
		if strings.Contains(out, "skipping") {
			t.Errorf("file at exactly the cap must NOT be skipped, got:\n%s", out)
		}
		// The boundary file's annotation is the only coverage source;
		// a passing coverage phase proves the file was actually read.
		if !strings.Contains(out, "PASS coverage") {
			t.Errorf("expected coverage phase to pass via boundary_test.go's annotation, got:\n%s", out)
		}
	})
}

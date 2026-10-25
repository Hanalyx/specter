// coverage_json_determinism_test.go -- CLI integration tests for
// spec-coverage 1.16.0 C-37: `coverage --json` produces a byte-identical
// document across runs of one binary on an unchanged workspace (AC-53
// through AC-55).
//
// Determinism is a claim about repeated runs, so every case here runs the
// binary the number of times its criterion names: five for AC-53 and AC-55,
// twenty for AC-54. A single run cannot distinguish a total order from a map
// iteration that happened to come out right.
//
// @spec spec-coverage
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Workspace pieces
// ---------------------------------------------------------------------------

// writeEightTestFileWorkspace writes one spec covered by tests/a_test.go
// through tests/h_test.go. The files are written in reverse alphabetical
// order so that directory-insertion order and the expected order disagree.
func writeEightTestFileWorkspace(t *testing.T, dir string) {
	t.Helper()
	writeDefectSpec(t, dir, defectSpec{id: "demo-spec", tier: 2, acIDs: []string{"AC-01"}})
	for _, name := range []string{"h", "g", "f", "e", "d", "c", "b", "a"} {
		writeAnnotatedTestFile(t, dir, "tests/"+name+"_test.go", "demo-spec", []string{"AC-01"})
	}
}

// eightTestFilesInOrder is the array AC-53 names.
var eightTestFilesInOrder = []string{
	"tests/a_test.go", "tests/b_test.go", "tests/c_test.go", "tests/d_test.go",
	"tests/e_test.go", "tests/f_test.go", "tests/g_test.go", "tests/h_test.go",
}

// writeSixInvalidSpecs writes the AC-54 workspace: three specs missing
// `objective` and three declaring `tier: 9`, all six declaring empty
// constraints and criteria lists. The two defect shapes are split across two
// file-name prefixes so that the tie between the two count-6 patterns cannot
// be broken by file name alone.
func writeSixInvalidSpecs(t *testing.T, dir string) {
	t.Helper()
	for i := 1; i <= 3; i++ {
		missingObjective := fmt.Sprintf(`spec:
  id: bado%d-spec
  version: "1.0.0"
  status: draft
  tier: 2

  context:
    system: Demo System
    feature: Missing objective fixture

  constraints: []

  acceptance_criteria: []
`, i)
		writeSpec(t, dir, fmt.Sprintf("bado%d.spec.yaml", i), missingObjective)

		badTier := fmt.Sprintf(`spec:
  id: badc%d-spec
  version: "1.0.0"
  status: draft
  tier: 9

  context:
    system: Demo System
    feature: Out-of-range tier fixture

  objective:
    summary: Fixture spec declaring a tier outside the enum.

  constraints: []

  acceptance_criteria: []
`, i)
		writeSpec(t, dir, fmt.Sprintf("badc%d.spec.yaml", i), badTier)
	}
}

// ---------------------------------------------------------------------------
// AC-53 -- test_files ascending by path on every run
// ---------------------------------------------------------------------------

// Eight files is enough that a map iteration reproducing ascending order by
// chance is unlikely, and the criterion names eight. Every run's array is
// compared against the stated order, and then all five are compared against
// each other, because "sorted" and "stable" are two claims and a fix could
// satisfy the second alone.
//
// @ac AC-53
func TestCoverageJSONDeterminism_TestFilesAscending(t *testing.T) {
	t.Run("spec-coverage/AC-53 test_files is ascending by path on five consecutive runs", func(t *testing.T) {
		dir := t.TempDir()
		writeEightTestFileWorkspace(t, dir)

		var seen [][]string
		for run := 1; run <= 5; run++ {
			stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
			entry := coverageEntry(t, decodeCoverageJSON(t, stdout), "demo-spec")
			files := stringsField(t, entry, "test_files")
			if !equalStrings(files, eightTestFilesInOrder) {
				t.Errorf("run %d: test_files = %v, want %v;\nstderr:\n%s", run, files, eightTestFilesInOrder, stderr)
			}
			seen = append(seen, files)
		}
		for run := 1; run < len(seen); run++ {
			if !equalStrings(seen[run], seen[0]) {
				t.Errorf("test_files changed between runs: run 1 = %v, run %d = %v", seen[0], run+1, seen[run])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC-54 -- parse_error_patterns in one fixed order
// ---------------------------------------------------------------------------

// The workspace produces two patterns at count 6 and two at count 3, so
// ordering by count alone leaves both pairs unresolved. The count-6 pair is
// broken by path and the count-3 pair by type, which is why both ties are in
// the fixture: a comparator that adds only one of the two tiebreakers still
// leaves the other pair in map order.
//
// Only the three fields the criterion names are compared. example_file and
// the per-pattern file lists of the later patterns are not pinned; the first
// pattern's file list is, because the criterion names it.
//
// @ac AC-54
func TestCoverageJSONDeterminism_ParseErrorPatternsTotalOrder(t *testing.T) {
	t.Run("spec-coverage/AC-54 parse_error_patterns holds one fixed order over twenty consecutive runs", func(t *testing.T) {
		dir := t.TempDir()
		writeSixInvalidSpecs(t, dir)

		wantPatterns := []struct {
			ptype string
			path  string
			count float64
		}{
			{"minItems", "spec.acceptance_criteria", 6},
			{"minItems", "spec.constraints", 6},
			{"required", "spec", 3},
			{"validation", "spec.tier", 3},
		}
		wantFirstFiles := []string{
			"specs/badc1.spec.yaml", "specs/badc2.spec.yaml", "specs/badc3.spec.yaml",
			"specs/bado1.spec.yaml", "specs/bado2.spec.yaml", "specs/bado3.spec.yaml",
		}

		var signatures []string
		for run := 1; run <= 20; run++ {
			stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
			doc := decodeCoverageJSON(t, stdout)
			raw, ok := doc["parse_error_patterns"].([]any)
			if !ok {
				t.Fatalf("run %d: document must carry parse_error_patterns, got %v;\nstderr:\n%s", run, doc["parse_error_patterns"], stderr)
			}
			if len(raw) != len(wantPatterns) {
				t.Fatalf("run %d: parse_error_patterns has %d entries, want %d: %v", run, len(raw), len(wantPatterns), raw)
			}

			var sig string
			for i, item := range raw {
				p, ok := item.(map[string]any)
				if !ok {
					t.Fatalf("run %d: pattern %d must be an object, got %T", run, i, item)
				}
				gotType, _ := p["type"].(string)
				gotPath, _ := p["path"].(string)
				gotCount := floatField(t, p, "count")
				sig += fmt.Sprintf("%s|%s|%v;", gotType, gotPath, gotCount)

				w := wantPatterns[i]
				if gotType != w.ptype || gotPath != w.path || !nearly(gotCount, w.count) {
					t.Errorf("run %d: pattern %d = (%s, %s, %v), want (%s, %s, %v)",
						run, i, gotType, gotPath, gotCount, w.ptype, w.path, w.count)
				}
			}
			signatures = append(signatures, sig)

			if run == 1 {
				first, _ := raw[0].(map[string]any)
				files := stringsField(t, first, "files")
				if !equalStrings(files, wantFirstFiles) {
					t.Errorf("first pattern files = %v, want %v", files, wantFirstFiles)
				}
			}
		}

		for run := 1; run < len(signatures); run++ {
			if signatures[run] != signatures[0] {
				t.Errorf("parse_error_patterns order changed between runs;\nrun 1:  %s\nrun %d: %s", signatures[0], run+1, signatures[run])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// AC-55 -- five runs, one document
// ---------------------------------------------------------------------------

// The workspace holds both map-derived arrays at once, so this criterion is
// the one that fails if either ordering is left unfixed. It compares the
// whole document rather than a named array, which is the claim a CI job
// diffing coverage snapshots actually depends on.
//
// @ac AC-55
func TestCoverageJSONDeterminism_FiveRunsOneDocument(t *testing.T) {
	t.Run("spec-coverage/AC-55 five consecutive runs write five byte-identical documents", func(t *testing.T) {
		dir := t.TempDir()
		writeEightTestFileWorkspace(t, dir)
		writeSixInvalidSpecs(t, dir)

		hashes := map[string]int{}
		var order []string
		for run := 1; run <= 5; run++ {
			stdout, stderr, _ := runCLISplit(t, dir, "coverage", "--json", "--strictness", "annotation")
			if stdout == "" {
				t.Fatalf("run %d: stdout is empty;\nstderr:\n%s", run, stderr)
			}
			sum := sha256.Sum256([]byte(stdout))
			h := hex.EncodeToString(sum[:])
			if _, seen := hashes[h]; !seen {
				order = append(order, h)
			}
			hashes[h]++
		}
		if len(hashes) != 1 {
			t.Errorf("five runs produced %d distinct stdout hashes, want 1; hashes in first-seen order: %v", len(hashes), order)
		}
	})
}

// equalStrings reports whether two string slices hold the same values in the
// same order.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ingest_merge_refusal_test.go -- `--merge` refuses to write an artifact
// `coverage` would reject, spec-ingest 1.8.0 C-15/AC-18, bugs/SP-SP-075.
//
// @spec spec-ingest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// declaresGo carries a streams block naming every stream its entries label.
const declaresGo = `{"streams":[{"name":"go","scanned":2,"extracted":1}],
	"results":[{"spec_id":"svc","ac_id":"AC-01","status":"passed","stream":"go"}]}`

// labelsUndeclaredWithNoBlock is legal alone, because a file with no streams
// key is what C-14 promises to leave untouched. Merged with a file that
// declares its streams, it contributes a label nothing declares.
//
// The label is `handwritten` rather than something short. An earlier draft used
// `js` and asserted the refusal named it, which passed on a run that printed no
// validation message at all: the output path `fresh.json` carries `js` inside
// `.json`, so the assertion matched the filename in the "was not written" line.
const labelsUndeclaredWithNoBlock = `{"results":[{"spec_id":"svc","ac_id":"AC-02","status":"passed","stream":"handwritten"}]}`

// declaresJS is the control's second input: consistent on its own terms.
const declaresJS = `{"streams":[{"name":"js","scanned":3,"extracted":1}],
	"results":[{"spec_id":"svc","ac_id":"AC-02","status":"passed","stream":"js"}]}`

// @spec spec-ingest
// @ac AC-18
//
// C-15: the prospective merged artifact satisfies spec-coverage C-44 before
// anything is written, and a refusal leaves the filesystem as it found it.
func TestIngestMergeRefusesAnInconsistentArtifact(t *testing.T) {
	t.Run("spec-ingest/AC-18 a merge does not write what coverage refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "a.json", declaresGo)
		writeFixture(t, dir, "b.json", labelsUndeclaredWithNoBlock)
		writeFixture(t, dir, "c.json", declaresJS)
		a := filepath.Join(dir, "a.json")
		b := filepath.Join(dir, "b.json")
		c := filepath.Join(dir, "c.json")

		// Control first. Two inputs that each declare what they label merge
		// and are written. Without this, refusing every merge would pass every
		// assertion below.
		control := filepath.Join(dir, "control.json")
		if _, stderr, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", c, "--output", control); code != 0 {
			t.Fatalf("C-15: a consistent merge exited %d, want 0. Every refusal assertion below would pass on a command that refuses everything.\nstderr:\n%s", code, stderr)
		}
		if _, err := os.Stat(control); err != nil {
			t.Fatalf("C-15: a consistent merge wrote no output: %v", err)
		}

		// Refusal, with no prior output at the path.
		fresh := filepath.Join(dir, "fresh.json")
		_, stderr, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", b, "--output", fresh)
		if code == 0 {
			t.Errorf("C-15: merging a file that declares its streams with one carrying an undeclared label exited 0. The artifact it writes is one `coverage` refuses")
		}
		// The full quoted form, so no path or unrelated word can satisfy it.
		if !strings.Contains(stderr, `stream "handwritten"`) {
			t.Errorf("C-15: the refusal does not name the stream the artifact fails on.\nstderr:\n%s", stderr)
		}
		if _, err := os.Stat(fresh); !os.IsNotExist(err) {
			t.Errorf("C-15: a refused merge created %s. Nothing is written when the artifact is refused", fresh)
		}

		// Refusal, with an existing output that must survive untouched.
		kept := filepath.Join(dir, "kept.json")
		writeFixture(t, dir, "kept.json", declaresGo)
		before, err := os.ReadFile(kept)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", b, "--output", kept); code == 0 {
			t.Errorf("C-15: the same refused merge exited 0 when the output already existed")
		}
		after, err := os.ReadFile(kept)
		if err != nil {
			t.Fatalf("C-15: a refused merge removed the existing output: %v", err)
		}
		if string(before) != string(after) {
			t.Errorf("C-15: a refused merge rewrote the existing output. A destroyed artifact is worse than the one the merge refused.\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
}

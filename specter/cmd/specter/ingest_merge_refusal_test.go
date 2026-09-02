// ingest_merge_refusal_test.go -- `--merge` refuses to write an artifact
// `coverage` would reject, spec-ingest 2.0.0 C-15/AC-18, bugs/SP-SP-075.
//
// @spec spec-ingest
package main

import (
	"fmt"
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

// bigResultsFile writes an input carrying n entries under one declared stream.
// Each criterion id is distinct so nothing collapses on merge.
func bigResultsFile(t *testing.T, path, stream string, lo, n int) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, `{"streams":[{"name":%q,"scanned":%d,"extracted":%d}],"results":[`, stream, n, n)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"spec_id":"svc","ac_id":"AC-%07d","status":"passed","stream":%q}`, lo+i, stream)
	}
	b.WriteString(`]}`)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// @spec spec-ingest
// @ac AC-18
//
// C-15: the prospective artifact must also be one `coverage` can read, and the
// two refusals must be distinguishable in the message. They need different
// responses: a stream can be declared, a size cannot be edited away.
//
// Driven through the command, because that is where AC-18 states the case and
// where the message a reader sees is produced. A check below the CLI would stay
// green while the command swallowed or rewrote the diagnostic.
func TestIngestMergeRefusesAnOversizedArtifact(t *testing.T) {
	t.Run("spec-ingest/AC-18 an oversized merge names the size, not a stream", func(t *testing.T) {
		if testing.Short() {
			t.Skip("writes two multi-megabyte inputs")
		}
		dir := t.TempDir()
		a := filepath.Join(dir, "big-a.json")
		b := filepath.Join(dir, "big-b.json")
		// Each input is comfortably under the 16 MiB per-input cap C-17 sets,
		// and together they pass it. Neither is refusable on its own terms.
		bigResultsFile(t, a, "go", 0, 115000)
		bigResultsFile(t, b, "js", 115000, 115000)
		for _, in := range []string{a, b} {
			info, err := os.Stat(in)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() >= 16<<20 {
				t.Fatalf("C-17: fixture %s is %d bytes, which the per-input cap already refuses. The oversized case has to come from the sum", in, info.Size())
			}
		}

		out := filepath.Join(dir, "merged.json")
		_, stderr, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", b, "--output", out)

		if code == 0 {
			t.Errorf("C-15: a merge past the cap exited 0. `coverage` cannot read the artifact it wrote")
		}
		// The contract, not a wording. C-15 asks that the message name the
		// size and not read as an inconsistency. An earlier draft required the
		// phrase "too large", which was wrong in both directions: it failed a
		// conforming implementation reporting the same limit in other words,
		// and it would have passed `stream "go" is too large`, which names
		// exactly the thing the rule says not to name.
		for _, want := range []string{"exceeds", "byte limit"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("C-15: the refusal does not name the size. Missing %q.\nstderr:\n%s", want, stderr)
			}
		}
		// The artifact is coherent. Naming a stream, or calling it
		// inconsistent, sends an operator to look for something that is not
		// the problem.
		for _, forbidden := range []string{`stream "`, "inconsistent"} {
			if strings.Contains(stderr, forbidden) {
				t.Errorf("C-15: an oversized merge reports %q, which is the consistency refusal's language.\nstderr:\n%s", forbidden, stderr)
			}
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Errorf("C-15: the refused oversized merge created %s", out)
		}
	})
}

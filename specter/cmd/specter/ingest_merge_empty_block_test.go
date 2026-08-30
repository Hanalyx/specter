// ingest_merge_empty_block_test.go -- a declared but empty `streams` block
// survives a merge, spec-ingest 3.0.0 C-15/AC-18.
//
// The laundering path this closes: `[]` and an absent key are the same Go
// value inside the producer and different bytes to the consumer. An input
// carrying `streams: []` beside a labeled entry exits 20 at `coverage`, and
// the same content through `--merge` used to exit 0 and write an artifact
// `coverage` accepts. C-15 already forbade that; the producer simply lost the
// evidence before its own validator ran.
//
// @spec spec-ingest
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// emptyBlockWithLabel is the laundering input. The block is declared and
// empty, and an entry names `ghost`, which the block therefore does not
// declare. `coverage` exits 20 on exactly this shape.
const emptyBlockWithLabel = `{"streams":[],
	"results":[{"spec_id":"svc","ac_id":"AC-01","status":"passed","stream":"ghost"}]}`

// emptyBlockNoLabel is consistent: a block declared and empty, and no entry
// naming any stream. It must merge, and the block must survive as `[]`.
// Without this case a fix could satisfy the refusal above by dropping every
// empty block on the floor, which is the behavior being removed.
const emptyBlockNoLabel = `{"streams":[],
	"results":[{"spec_id":"svc","ac_id":"AC-02","status":"passed"}]}`

// legacyNoKey is what every producer wrote before the field existed. C-14
// promises such a run keeps producing exactly the file it produced before, so
// a merge of these must not grow a `streams` key.
const legacyNoKey = `{"results":[{"spec_id":"svc","ac_id":"AC-03","status":"passed"}]}`

// streamsKeyPresent reports whether the artifact at path carries a top-level
// `streams` key, and its length.
//
// Decoded through json.RawMessage rather than into a []StreamInfo, because
// that is the only place an absent key and an empty one are still
// distinguishable, and telling them apart is the whole criterion.
func streamsKeyPresent(t *testing.T, path string) (present bool, n int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var wire struct {
		Streams json.RawMessage `json:"streams"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("%s does not parse as JSON: %v\n%s", path, err, data)
	}
	if wire.Streams == nil {
		return false, 0
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(wire.Streams, &rows); err != nil {
		return true, 0
	}
	return true, len(rows)
}

// @spec spec-ingest
// @ac AC-18
//
// C-15: key presence survives read, merge and serialization, so the validator
// sees the artifact a consumer would.
func TestIngestMergePreservesStreamsKeyPresence(t *testing.T) {
	t.Run("spec-ingest/AC-18 a declared empty block beside a labeled entry is refused", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "empty_labeled.json", emptyBlockWithLabel)
		in := filepath.Join(dir, "empty_labeled.json")

		// Control first. A merge that refuses everything would satisfy every
		// refusal assertion in this file.
		writeFixture(t, dir, "legacy.json", legacyNoKey)
		control := filepath.Join(dir, "control.json")
		if _, stderr, code := runCLISplit(t, dir, "ingest", "--merge", filepath.Join(dir, "legacy.json"), "--output", control); code != 0 {
			t.Fatalf("AC-18 (control): a consistent merge exited %d, want 0. Every refusal below would pass on a command that refuses everything.\nstderr:\n%s", code, stderr)
		}

		// The refusal, with no prior output at the path. The name is `ghost`
		// rather than something short: an earlier test in this family matched
		// its own output filename because the label appeared inside `.json`.
		fresh := filepath.Join(dir, "fresh_out.json")
		_, stderr, code := runCLISplit(t, dir, "ingest", "--merge", in, "--output", fresh)
		if code == 0 {
			present, n := streamsKeyPresent(t, fresh)
			t.Fatalf("AC-18: the merge exited 0 on an input whose declared streams block is empty while an entry names \"ghost\". coverage exits 20 on that artifact, so the producer wrote what its own consumer refuses. Written output carries streams key=%v len=%d", present, n)
		}
		if !strings.Contains(stderr, "ghost") {
			t.Errorf("AC-18: the refusal does not name the undeclared stream \"ghost\". An operator cannot repair a block without knowing which label is missing.\nstderr:\n%s", stderr)
		}
		if _, err := os.Stat(fresh); err == nil {
			t.Errorf("AC-18: %s was created by a refused merge. C-15 requires nothing to be written on refusal", fresh)
		}
	})

	t.Run("spec-ingest/AC-18 a refused merge leaves an existing output byte for byte", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "empty_labeled.json", emptyBlockWithLabel)
		in := filepath.Join(dir, "empty_labeled.json")

		out := filepath.Join(dir, "prior.json")
		const sentinel = "PRIOR ARTIFACT, MUST SURVIVE\n"
		if err := os.WriteFile(out, []byte(sentinel), 0o644); err != nil {
			t.Fatal(err)
		}

		if _, _, code := runCLISplit(t, dir, "ingest", "--merge", in, "--output", out); code == 0 {
			t.Fatalf("AC-18: the merge exited 0, so the byte-for-byte claim below is unobserved")
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("AC-18: the prior output is gone after a refused merge: %v", err)
		}
		if string(got) != sentinel {
			t.Errorf("AC-18: a refused merge rewrote the existing output. A partial or replaced artifact destroys the file the operator still had.\ngot:  %q\nwant: %q", got, sentinel)
		}
	})

	t.Run("spec-ingest/AC-18 a declared empty block with no labels survives as an empty array", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "empty_nolabel.json", emptyBlockNoLabel)
		out := filepath.Join(dir, "out.json")

		if _, stderr, code := runCLISplit(t, dir, "ingest", "--merge", filepath.Join(dir, "empty_nolabel.json"), "--output", out); code != 0 {
			t.Fatalf("AC-18: a consistent input whose streams block is declared and empty was refused, exit %d. Nothing about it violates C-44.\nstderr:\n%s", code, stderr)
		}
		present, n := streamsKeyPresent(t, out)
		if !present {
			t.Errorf("AC-18: the merged output dropped the declared streams key. Dropping it is what let an inconsistent artifact through, and it also loses the producer's statement that it ran zero streams deliberately")
		}
		if present && n != 0 {
			t.Errorf("AC-18: the merged output declares %d stream(s), want 0. The input declared an empty block and the merge invented rows", n)
		}
	})

	t.Run("spec-ingest/AC-18 an input with no streams key does not grow one", func(t *testing.T) {
		dir := t.TempDir()
		writeFixture(t, dir, "legacy.json", legacyNoKey)
		out := filepath.Join(dir, "out.json")

		if _, stderr, code := runCLISplit(t, dir, "ingest", "--merge", filepath.Join(dir, "legacy.json"), "--output", out); code != 0 {
			t.Fatalf("AC-18: a legacy input with no streams key was refused, exit %d.\nstderr:\n%s", code, stderr)
		}
		if present, n := streamsKeyPresent(t, out); present {
			t.Errorf("AC-18: the merged output grew a streams key (len %d) from an input that carried none. C-14 promises an unlabeled producer keeps writing exactly the file it wrote before, so a fix that always emits the key breaks every existing pipeline", n)
		}
	})
}

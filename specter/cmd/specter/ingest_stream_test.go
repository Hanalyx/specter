// ingest_stream_test.go -- CLI integration tests for roadmap 3A4:
// `ingest --stream`, `ingest --merge`, and the silent-package count
// (spec-ingest 1.6.0 C-14, C-15, C-16, AC-14 through AC-16).
//
// These cross the CLI boundary because all three criteria are about flags and
// about the file a run writes. The flags do not exist yet, so a run naming one
// exits non-zero on an unknown flag, which is a runtime failure rather than a
// build failure and is what makes these red rather than uncompilable.
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

// streamArtifact is the shape these read back. Declared locally because the
// fields it names are what the implementation adds; naming them on the ingest
// package's type would turn a behavioral failure into a build failure.
type streamArtifact struct {
	Streams []struct {
		Name                  string `json:"name"`
		Scanned               int    `json:"scanned"`
		Extracted             int    `json:"extracted"`
		ZeroTestEventPackages int    `json:"zero_test_event_packages"`
	} `json:"streams"`
	Results []struct {
		SpecID string `json:"spec_id"`
		ACID   string `json:"ac_id"`
		Status string `json:"status"`
		Stream string `json:"stream"`
	} `json:"results"`
}

const junitTwoCases = `<?xml version="1.0"?>
<testsuites>
  <testsuite>
    <testcase name="svc/AC-01 passes"/>
    <testcase name="svc/AC-02 fails"><failure message="bad"/></testcase>
  </testsuite>
</testsuites>`

// goTestSilentPackage holds one package that reports a test and one whose
// terminal event arrives with no test event before it. The second is the
// package that produced zero test events, and its events are the ones the
// parser used to discard.
const goTestSilentPackage = `{"Action":"run","Package":"x/a","Test":"TestOne"}
{"Action":"output","Package":"x/a","Test":"TestOne","Output":"svc/AC-01\n"}
{"Action":"pass","Package":"x/a","Test":"TestOne"}
{"Action":"start","Package":"x/b"}
{"Action":"fail","Package":"x/b"}
`

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readArtifact(t *testing.T, path string) streamArtifact {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("results file not written: %v", err)
	}
	var out streamArtifact
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("the artifact did not decode: %v", err)
	}
	return out
}

// withoutLabels returns an artifact's results as raw maps with only the stream
// key removed, so two files compare on every other field the artifact carries.
//
// Deliberately not a typed shape. A struct compares the fields someone
// remembered to declare, and an earlier version of this helper decoded into one
// that omitted the back-compat `passed` boolean, so --stream could have
// corrupted it while status stayed intact and the comparison would have passed.
// A map compares fields nobody thought of, including ones added later.
func withoutLabels(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("results file not written: %v", err)
	}
	var doc struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the artifact did not decode: %v", err)
	}
	for _, r := range doc.Results {
		delete(r, "stream")
	}
	// json.Marshal orders map keys, so two files with the same fields in a
	// different order still compare equal.
	out, err := json.Marshal(doc.Results)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// @spec spec-ingest
// @ac AC-14
//
// C-14: --stream labels every entry, an absent flag labels none, and an empty
// value is refused.
func TestIngestStreamFlag(t *testing.T) {
	t.Run("spec-ingest/AC-14 --stream labels every entry it writes", func(t *testing.T) {
		dir := t.TempDir()
		junit := writeFixture(t, dir, "j.xml", junitTwoCases)

		labeled := filepath.Join(dir, "labeled.json")
		if _, _, code := runCLISplit(t, dir, "ingest", "--junit", junit, "--stream", "go", "--output", labeled); code != 0 {
			t.Fatalf("C-14: ingest --stream exited %d, want 0. The flag does not exist yet", code)
		}
		got := readArtifact(t, labeled)
		if len(got.Results) == 0 {
			t.Fatalf("C-14: the run extracted nothing, so the fixture no longer reaches the case")
		}
		for _, r := range got.Results {
			if r.Stream != "go" {
				t.Errorf("C-14: entry %s/%s carries stream %q, want go. One invocation writes one stream", r.SpecID, r.ACID, r.Stream)
			}
		}

		// C-14 and C-41: without the flag nothing is labeled, so a pipeline
		// that never learns the flag exists keeps writing the file it wrote.
		plain := filepath.Join(dir, "plain.json")
		if _, _, code := runCLISplit(t, dir, "ingest", "--junit", junit, "--output", plain); code != 0 {
			t.Fatalf("C-14: ingest without --stream exited %d, want 0", code)
		}
		for _, r := range readArtifact(t, plain).Results {
			if r.Stream != "" {
				t.Errorf("C-14: an unlabeled run wrote stream %q on %s/%s. Missing means default and the field is not written", r.Stream, r.SpecID, r.ACID)
			}
		}

		// The two files differ only in that field. Counting entries would let
		// --stream change a status, a spec id or a criterion id and still pass,
		// so both are normalized with the label stripped and compared whole.
		if a, b := withoutLabels(t, labeled), withoutLabels(t, plain); a != b {
			t.Errorf("C-14: with the stream labels removed the two runs wrote different results, so the flag changed more than the label.\n labeled: %s\n plain:   %s", a, b)
		}

		// An empty value is refused, so a producer that meant to name a stream
		// learns it from the run rather than from a coverage report later.
		if _, _, code := runCLISplit(t, dir, "ingest", "--junit", junit, "--stream", "", "--output", filepath.Join(dir, "empty.json")); code == 0 {
			t.Error("C-14: an empty --stream value was accepted, so a run that named nothing wrote an unlabeled file and said so nowhere")
		}
	})
}

// @spec spec-ingest
// @ac AC-15
//
// C-15: --merge builds from the named files alone. The stale-criterion case is
// the one behavior that separates it from accumulation.
func TestIngestMergeFlag(t *testing.T) {
	t.Run("spec-ingest/AC-15 --merge builds from the named inputs alone", func(t *testing.T) {
		dir := t.TempDir()
		// Both inputs carry streams blocks, and b declares e2e as ran-but-empty.
		// Without these the merge could drop all top-level metadata, losing the
		// only record that a stream ran and found nothing, and still pass.
		a := writeFixture(t, dir, "a.json", `{
			"streams":[{"name":"go","scanned":10,"extracted":1,"zero_test_event_packages":1}],
			"results":[{"spec_id":"svc","ac_id":"AC-01","status":"passed","stream":"go"}]}`)
		b := writeFixture(t, dir, "b.json", `{
			"streams":[{"name":"js","scanned":5,"extracted":1},
			           {"name":"e2e","scanned":3,"extracted":0}],
			"results":[{"spec_id":"svc","ac_id":"AC-02","status":"failed","stream":"js"}]}`)

		// An existing output holding a criterion neither input mentions. An
		// implementation that reads the output first keeps it.
		out := filepath.Join(dir, ".specter-results.json")
		writeFixture(t, dir, ".specter-results.json", `{"results":[{"spec_id":"svc","ac_id":"AC-99","status":"passed"}]}`)

		if _, _, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", b, "--output", out); code != 0 {
			t.Fatalf("C-15: ingest --merge exited %d, want 0. The flag does not exist yet", code)
		}
		got := readArtifact(t, out)

		for _, r := range got.Results {
			if r.ACID == "AC-99" {
				t.Error("C-15: a criterion neither input mentions survived the merge, so a stream re-run layers on a stale pass instead of replacing it")
			}
		}
		streams := map[string]bool{}
		for _, r := range got.Results {
			streams[r.Stream] = true
		}
		if !streams["go"] || !streams["js"] {
			t.Errorf("C-15: the merged file carries entry labels %v, want both go and js. Two inputs carrying different streams both survive", streams)
		}

		// C-15: the streams blocks merge by name. e2e produced no results, so
		// the block is the only thing that records it ran at all.
		meta := map[string][2]int{}
		for _, st := range got.Streams {
			meta[st.Name] = [2]int{st.Scanned, st.Extracted}
		}
		// Every count is preserved, not merely the stream's name. A merge that
		// carried names and reset the numbers would pass a presence check.
		for _, want := range []struct {
			name             string
			scanned, extract int
		}{
			{"go", 10, 1},
			{"js", 5, 1},
			{"e2e", 3, 0},
		} {
			got, ok := meta[want.name]
			if !ok {
				t.Errorf("C-15: the merged streams block is %v and does not carry %q. A stream that ran and found nothing is recorded nowhere else", meta, want.name)
				continue
			}
			if got[0] != want.scanned || got[1] != want.extract {
				t.Errorf("C-15: stream %q merged as scanned=%d extracted=%d, want %d and %d. Streams appearing in one input carry their counts through", want.name, got[0], got[1], want.scanned, want.extract)
			}
		}

		// Two inputs, one stream, one criterion: worst status wins, per C-08.
		c := writeFixture(t, dir, "c.json", `{
			"streams":[{"name":"go","scanned":7,"extracted":1,"zero_test_event_packages":2}],
			"results":[{"spec_id":"svc","ac_id":"AC-01","status":"failed","stream":"go"}]}`)
		same := filepath.Join(dir, "same.json")
		if _, _, code := runCLISplit(t, dir, "ingest", "--merge", a, "--merge", c, "--output", same); code != 0 {
			t.Fatalf("C-15: merging two files of one stream exited %d, want 0", code)
		}
		collapsed := readArtifact(t, same)
		if len(collapsed.Results) != 1 {
			t.Errorf("C-15: merging one criterion of one stream from two files wrote %d entries, want 1", len(collapsed.Results))
		} else if collapsed.Results[0].Status != "failed" {
			t.Errorf("C-15: the collapsed entry is %q, want failed. A passing result does not heal a failing sibling", collapsed.Results[0].Status)
		}
		// One stream declared by both inputs sums its counts, per C-15. a
		// scanned 10 with 1 silent package and c scanned 7 with 2.
		if len(collapsed.Streams) != 1 {
			t.Errorf("C-15: merging two files of one stream wrote %d stream entries, want 1. The blocks merge by name", len(collapsed.Streams))
		} else {
			st := collapsed.Streams[0]
			// All three counts sum. Asserting two would let a merge leave the
			// third stale, and extracted is a stream count exactly as the
			// other two are, per spec-coverage C-42.
			if st.Scanned != 17 || st.Extracted != 2 || st.ZeroTestEventPackages != 3 {
				t.Errorf("C-15: the merged stream is scanned=%d extracted=%d silent=%d, want 17, 2 and 3. A stream declared by more than one input sums every count, not the ones that were easy",
					st.Scanned, st.Extracted, st.ZeroTestEventPackages)
			}
		}

		// Mixing the modes is refused rather than given a precedence rule,
		// because a silent winner makes the artifact depend on flag order.
		junit := writeFixture(t, dir, "j.xml", junitTwoCases)
		gotestFile := writeFixture(t, dir, "conflict.json", goTestSilentPackage)
		for _, extra := range [][]string{{"--junit", junit}, {"--go-test", gotestFile}, {"--stream", "go"}} {
			args := append([]string{"ingest", "--merge", a, "--output", filepath.Join(dir, "mixed.json")}, extra...)
			if _, _, code := runCLISplit(t, dir, args...); code == 0 {
				t.Errorf("C-15: --merge combined with %v was accepted. The two modes disagree about what the output is built from", extra)
			}
		}
	})
}

// @spec spec-ingest
// @ac AC-16
//
// C-16: the streams entry is written when a run names a stream, even when
// nothing was extracted, and an unlabeled run writes no block at all.
func TestIngestStreamMetadata(t *testing.T) {
	t.Run("spec-ingest/AC-16 the streams entry records what the run observed", func(t *testing.T) {
		dir := t.TempDir()
		// A runner file that scans cases and matches no annotation.
		nothing := writeFixture(t, dir, "none.xml", `<?xml version="1.0"?>
<testsuites><testsuite>
  <testcase name="unannotated one"/>
  <testcase name="unannotated two"/>
</testsuite></testsuites>`)

		empty := filepath.Join(dir, "empty.json")
		if _, _, code := runCLISplit(t, dir, "ingest", "--junit", nothing, "--stream", "js", "--output", empty); code != 0 {
			t.Fatalf("C-16: ingest exited %d, want 0", code)
		}
		got := readArtifact(t, empty)
		if len(got.Streams) != 1 {
			t.Fatalf("C-16: a run that extracted nothing wrote %d stream entries, want 1. A stream that ran and found nothing and one that never ran both leave zero results, and only this block separates them", len(got.Streams))
		}
		if got.Streams[0].Name != "js" || got.Streams[0].Scanned == 0 || got.Streams[0].Extracted != 0 {
			t.Errorf("C-16: the entry is %+v, want js with a non-zero scanned count and zero extracted", got.Streams[0])
		}

		// C-14's back-compat promise: no flag, no block.
		plain := filepath.Join(dir, "plain.json")
		if _, _, code := runCLISplit(t, dir, "ingest", "--junit", nothing, "--output", plain); code != 0 {
			t.Fatalf("C-16: ingest without --stream exited %d, want 0", code)
		}
		// Checked on the raw bytes. A decoded length of zero is also what an
		// emitted "streams": [] produces, and an empty array is not the legacy
		// artifact C-14 promises: it is a new key in every existing producer's
		// output.
		plainRaw, err := os.ReadFile(plain)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(plainRaw), `"streams"`) {
			t.Errorf("C-16: an unlabeled run wrote a streams key. C-14 promises such a run writes exactly the file it wrote before, and even an empty array breaks that for every existing producer.\ngot: %s", plainRaw)
		}

		// The silent-package count, roadmap 3B5 rehomed. x/b reports a
		// terminal event with no test event before it.
		gotest := writeFixture(t, dir, "go.json", goTestSilentPackage)
		silent := filepath.Join(dir, "silent.json")
		_, stderr, code := runCLISplit(t, dir, "ingest", "--go-test", gotest, "--stream", "go", "--output", silent)
		if code != 0 {
			t.Fatalf("C-16: ingest exited %d, want 0.\nstderr:\n%s", code, stderr)
		}
		sa := readArtifact(t, silent)
		if len(sa.Streams) != 1 {
			t.Fatalf("C-16: wrote %d stream entries, want 1", len(sa.Streams))
		}
		if sa.Streams[0].ZeroTestEventPackages != 1 {
			t.Errorf("C-16: counted %d packages producing zero test events, want 1. Package x/b reports a terminal event with no test event, and those events used to be discarded", sa.Streams[0].ZeroTestEventPackages)
		}

		// Named for what was observed, in all three places C-16 binds: the
		// artifact, the summary line, and the field name. ingest cannot tell a
		// build failure from a filtered-out package or one with no tests.
		raw, err := os.ReadFile(silent)
		if err != nil {
			t.Fatal(err)
		}
		for _, place := range []struct{ what, text string }{
			{"the artifact", string(raw)},
			{"the summary line", stderr},
		} {
			lower := strings.ToLower(place.text)
			if strings.Contains(lower, "failed to build") || strings.Contains(lower, "build failure") || strings.Contains(lower, "build_fail") {
				t.Errorf("C-16: %s calls the silent package a build failure, which ingest cannot tell from a filtered-out package or one with no tests.\ngot: %s", place.what, place.text)
			}
		}
	})
}

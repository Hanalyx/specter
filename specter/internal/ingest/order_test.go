package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Writer ordering, roadmap 3A3, spec-ingest C-13.
//
// The total-order half is assertable against today's types and is red today:
// MergeResults preserves first-seen order, which is not an order at all, it is
// whatever the runner emitted. The worst-status-first half needs a stream on a
// result, and naming a field the implementation will introduce reports a build
// failure rather than a behavioral one, so it arrives with the field.

func writeAndRead(t *testing.T, results []TestResult) resultsFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".specter-results.json")
	if err := WriteResultsFile(path, results); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out resultsFile
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("the artifact did not decode: %v", err)
	}
	return out
}

func writeBytes(t *testing.T, results []TestResult) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".specter-results.json")
	if err := WriteResultsFile(path, results); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// @spec spec-ingest
// @ac AC-13
//
// C-13: the file carries a defined total order, ascending by spec id then
// criterion id, and two runs over one input produce identical bytes.
func TestWriterTotalOrder(t *testing.T) {
	t.Run("spec-ingest/AC-13 the results file carries a total order", func(t *testing.T) {
		// Deliberately out of order on both keys. A writer that preserves
		// first-seen order reproduces this exact sequence.
		in := []TestResult{
			{SpecID: "z-spec", ACID: "AC-02", Status: StatusPassed, Name: "T4"},
			{SpecID: "a-spec", ACID: "AC-10", Status: StatusPassed, Name: "T2"},
			{SpecID: "z-spec", ACID: "AC-01", Status: StatusFailed, Name: "T3"},
			{SpecID: "a-spec", ACID: "AC-02", Status: StatusPassed, Name: "T1"},
		}

		got := writeAndRead(t, in)
		if len(got.Results) != 4 {
			t.Fatalf("wrote %d entries, want 4; the fixture no longer reaches the case", len(got.Results))
		}

		// The control. If the input were already sorted, an ordering assertion
		// would pass over a writer that does nothing.
		if in[0].SpecID <= in[1].SpecID {
			t.Fatal("the fixture is already in order, so this cannot detect a writer that preserves input order")
		}

		want := [][2]string{
			{"a-spec", "AC-02"},
			{"a-spec", "AC-10"},
			{"z-spec", "AC-01"},
			{"z-spec", "AC-02"},
		}
		for i, w := range want {
			if got.Results[i].SpecID != w[0] || got.Results[i].ACID != w[1] {
				t.Errorf("C-13: entry %d is %s/%s, want %s/%s. The file keeps whatever order the runner emitted, so two producers over identical facts write different bytes",
					i, got.Results[i].SpecID, got.Results[i].ACID, w[0], w[1])
			}
		}

		// Byte-identical across runs, per C-13. Two separate writes of the same
		// input, not one value compared with itself.
		if a, b := writeBytes(t, in), writeBytes(t, in); a != b {
			t.Errorf("C-13: two writes of one input produced different bytes, so a CI job diffing them sees churn that is not a change.\n first: %s\n second: %s", a, b)
		}
	})

	// C-13's other half. It arrives here rather than in the red commit because
	// constructing it needs a stream on a result, and naming a field the
	// implementation will introduce reports a build failure rather than a
	// behavioral one.
	t.Run("spec-ingest/AC-13 a criterion's entries are worst-status-first", func(t *testing.T) {
		// One criterion, two streams, and the passing one listed first so a
		// writer preserving input order emits the optimistic row first.
		in := []TestResult{
			{SpecID: "s", ACID: "AC-01", Status: StatusPassed, Stream: "go", Name: "T1"},
			{SpecID: "s", ACID: "AC-01", Status: StatusFailed, Stream: "js", Name: "T2"},
		}
		got := writeAndRead(t, in)

		// Two streams reporting on one criterion are two facts. Collapsing
		// them here would destroy the distinction the foundation records.
		if len(got.Results) != 2 {
			t.Fatalf("C-08: wrote %d entries for one criterion across two streams, want 2. The merge key is (spec, AC, stream), so the streams do not collapse into one another", len(got.Results))
		}
		if got.Results[0].Status != StatusFailed {
			t.Errorf("C-13: the first row for this criterion is %q, want failed. A consumer resolving by first match reads the optimistic answer over a best-first file, which is the 83 against 33 the roadmap measured",
				got.Results[0].Status)
		}
		if got.Results[0].Stream != "js" || got.Results[1].Stream != "go" {
			t.Errorf("C-41: the labels did not survive the write: first %q, second %q, want js then go", got.Results[0].Stream, got.Results[1].Stream)
		}

		// A single-stream run writes exactly the file it wrote before the
		// field existed, per C-41's missing-means-default.
		legacy := writeBytes(t, []TestResult{{SpecID: "s", ACID: "AC-01", Status: StatusPassed, Name: "T1"}})
		explicit := writeBytes(t, []TestResult{{SpecID: "s", ACID: "AC-01", Status: StatusPassed, Stream: DefaultStream, Name: "T1"}})
		if strings.Contains(legacy, `"stream"`) {
			t.Errorf("C-41: an unlabeled result wrote a stream field, so a single-stream run no longer emits the file it emitted before.\ngot: %s", legacy)
		}
		if legacy != explicit {
			t.Errorf("C-41: naming the default stream explicitly changed the file, and missing means default.\n legacy: %s\n explicit: %s", legacy, explicit)
		}
	})
}

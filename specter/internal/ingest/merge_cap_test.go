package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/coverage"
)

// @spec spec-ingest
// @ac AC-17
//
// C-17: a --merge input is capped at 16 MiB, refused by stat before the read.
// bugs-free until 1.6.0 shipped a second reader of an artifact internal/coverage
// has capped since the v0.13 audit.
func TestMergeInputSizeCap(t *testing.T) {
	t.Run("spec-ingest/AC-17 an oversized merge input is refused before it is read", func(t *testing.T) {
		// The two readers of one file shape must agree. A cap that differed
		// would accept what its sibling refuses, and the artifact's meaning
		// would depend on which command opened it.
		if MaxResultsFileBytes != coverage.MaxResultsFileBytes {
			t.Errorf("C-17: ingest caps results files at %d and coverage at %d. One artifact, two readers, two answers about which files are readable",
				MaxResultsFileBytes, coverage.MaxResultsFileBytes)
		}

		dir := t.TempDir()
		body := []byte(`{"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed"}]}`)

		// Valid JSON padded past the cap, deliberately. A sparse or truncated
		// file fails to unmarshal, so an assertion over one is satisfied by any
		// error at all and would pass against a reader with no cap. This one
		// parses cleanly if it is ever read, so only the cap can refuse it.
		over := filepath.Join(dir, "over.json")
		if err := os.WriteFile(over, padTo(body, MaxResultsFileBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}

		_, _, err := ReadResultsFile(over)
		if err == nil {
			t.Fatal("C-17: a file one byte over the cap was read. A results file is a CI artifact and an unbounded read of one is the class the v0.13 audit closed for coverage")
		}
		msg := err.Error()
		for _, want := range []string{"exceeds", "byte limit"} {
			if !strings.Contains(msg, want) {
				t.Errorf("C-17: the refusal does not contain %q, so it does not match the wording spec-diff C-12 and spec-check C-12 use.\ngot: %s", want, msg)
			}
		}
		if !strings.Contains(msg, "over.json") {
			t.Errorf("C-17: the refusal does not name the file, so an operator merging several cannot tell which one.\ngot: %s", msg)
		}

		// At the cap is accepted, so the boundary is a limit and not an
		// off-by-one. Content is what matters past the stat, so this one holds
		// a real document padded to exactly the cap.
		at := filepath.Join(dir, "at.json")
		if err := os.WriteFile(at, padTo(body, MaxResultsFileBytes), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _, err := ReadResultsFile(at); err != nil {
			t.Errorf("C-17: a file of exactly %d bytes was refused, so the cap is an off-by-one: %v", MaxResultsFileBytes, err)
		} else if len(got) != 1 {
			t.Errorf("C-17: the at-cap file parsed to %d results, want 1", len(got))
		}
	})
}

// padTo returns body followed by spaces to exactly n bytes. Trailing whitespace
// keeps the document valid JSON, so a file over the cap is refused by the cap
// and not by a parse error that any oversized file would produce.
func padTo(body []byte, n int) []byte {
	out := make([]byte, n)
	copy(out, body)
	for i := len(body); i < n; i++ {
		out[i] = ' '
	}
	return out
}

package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @spec spec-ingest
// @ac AC-18
//
// C-15: the prospective artifact must also be one `coverage` can read, and a
// size refusal must not be reported as an inconsistency. The two need different
// responses from an operator: a stream can be declared, a size cannot be edited
// away.
//
// Driven through WriteMergedResultsFile rather than the CLI, because the
// fixture has to exceed 16 MiB and building it in memory is far cheaper than
// writing two multi-megabyte input files per run.
func TestMergedArtifactOverTheCapIsRefusedAsASize(t *testing.T) {
	t.Run("spec-ingest/AC-18 an oversized merge names the size, not a stream", func(t *testing.T) {
		// Enough entries to pass MaxResultsFileBytes once indented. Every
		// criterion id is distinct so MergeResults collapses nothing.
		results := make([]TestResult, 0, 200000)
		for i := 0; i < 200000; i++ {
			results = append(results, TestResult{
				SpecID: "svc",
				ACID:   "AC-" + pad(i),
				Status: StatusPassed,
				Stream: "go",
			})
		}
		streams := []StreamInfo{{Name: "go", Scanned: len(results), Extracted: len(results)}}

		dir := t.TempDir()
		out := filepath.Join(dir, "big.json")

		err := WriteMergedResultsFile(out, results, streams)
		if err == nil {
			t.Fatalf("C-15: a merged artifact past the cap was written. `coverage` cannot read it, so the merge produced a file its own consumer refuses")
		}
		if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
			t.Errorf("C-15: the refused oversized merge created %s", out)
		}

		msg := err.Error()
		// The artifact is coherent. Calling it inconsistent sends an operator
		// to look for a stream that is not the problem.
		if strings.Contains(msg, "inconsistent") {
			t.Errorf("C-15: an oversized merge is reported as an inconsistency. The two refusals need different responses and must read differently.\ngot: %s", msg)
		}
		for _, want := range []string{"exceeds", "byte limit"} {
			if !strings.Contains(msg, want) {
				t.Errorf("C-15: the refusal does not name the size. Missing %q.\ngot: %s", want, msg)
			}
		}
	})
}

func pad(i int) string {
	s := ""
	for n := i; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	if s == "" {
		s = "0"
	}
	for len(s) < 7 {
		s = "0" + s
	}
	return s
}

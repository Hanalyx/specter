// @spec spec-ingest
package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// @ac AC-06
func TestWriteResultsFile_EmitsStatusAndBackCompatPassed(t *testing.T) {
	t.Run("spec-ingest/AC-06 write emits status and back-compat passed", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, ".specter-results.json")
		results := []TestResult{
			{SpecID: "spec-a", ACID: "AC-01", Status: StatusPassed},
			{SpecID: "spec-a", ACID: "AC-02", Status: StatusFailed},
		}
		if err := WriteResultsFile(out, results); err != nil {
			t.Fatalf("WriteResultsFile error: %v", err)
		}

		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read error: %v", err)
		}

		var parsed struct {
			Results []struct {
				SpecID string `json:"spec_id"`
				ACID   string `json:"ac_id"`
				Status string `json:"status"`
				Passed bool   `json:"passed"`
			} `json:"results"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if len(parsed.Results) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(parsed.Results))
		}

		// Entry 0: passed
		if parsed.Results[0].Status != "passed" {
			t.Errorf("entry[0].status = %q, want passed", parsed.Results[0].Status)
		}
		if !parsed.Results[0].Passed {
			t.Errorf("entry[0].passed should be true (back-compat)")
		}

		// Entry 1: failed
		if parsed.Results[1].Status != "failed" {
			t.Errorf("entry[1].status = %q, want failed", parsed.Results[1].Status)
		}
		if parsed.Results[1].Passed {
			t.Errorf("entry[1].passed should be false (back-compat)")
		}
	})
}

// @ac AC-07
func TestMergeResults_WorstStatusWins(t *testing.T) {
	t.Run("spec-ingest/AC-07 merge worst status wins", func(t *testing.T) {
		in := []TestResult{
			{SpecID: "spec-a", ACID: "AC-07", Status: StatusPassed},
			{SpecID: "spec-a", ACID: "AC-07", Status: StatusFailed},
		}
		merged := MergeResults(in)
		if len(merged) != 1 {
			t.Fatalf("expected 1 merged entry, got %d", len(merged))
		}
		if merged[0].Status != StatusFailed {
			t.Errorf("expected worst status failed, got %q", merged[0].Status)
		}
	})
}

// @ac AC-07
func TestMergeResults_ErroredBeatsFailed(t *testing.T) {
	t.Run("spec-ingest/AC-07 merge errored beats failed", func(t *testing.T) {
		in := []TestResult{
			{SpecID: "s", ACID: "AC-01", Status: StatusFailed},
			{SpecID: "s", ACID: "AC-01", Status: StatusErrored},
			{SpecID: "s", ACID: "AC-01", Status: StatusPassed},
		}
		merged := MergeResults(in)
		if merged[0].Status != StatusErrored {
			t.Errorf("expected errored (worst), got %q", merged[0].Status)
		}
	})
}

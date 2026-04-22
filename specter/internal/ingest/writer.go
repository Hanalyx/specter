// writer.go — serialize []TestResult to .specter-results.json.
// C-07: back-compat boolean passed is emitted alongside the new status field.
//
// @spec spec-ingest
package ingest

import (
	"encoding/json"
	"os"
)

// resultsFile is the on-disk JSON shape. Mirrors the structure
// internal/coverage consumes via ParseResultsFile.
type resultsFile struct {
	Results []resultEntry `json:"results"`
}

type resultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Status Status `json:"status"`
	Passed bool   `json:"passed"` // back-compat — readers on spec-coverage < 1.9.0
}

// WriteResultsFile merges, sorts, and writes results to path. Existing content
// is overwritten.
func WriteResultsFile(path string, results []TestResult) error {
	merged := MergeResults(results)

	out := resultsFile{Results: make([]resultEntry, 0, len(merged))}
	for _, r := range merged {
		out.Results = append(out.Results, resultEntry{
			SpecID: r.SpecID,
			ACID:   r.ACID,
			Status: r.Status,
			Passed: r.Status == StatusPassed,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// MergeResults collapses duplicate (spec, AC) entries down to one using the
// worst-status rule (C-08: errored > failed > skipped > passed). Order of the
// returned slice is stable by (spec_id, ac_id) for deterministic output.
func MergeResults(in []TestResult) []TestResult {
	type key struct{ spec, ac string }
	best := make(map[key]TestResult, len(in))
	order := make([]key, 0, len(in))

	for _, r := range in {
		k := key{r.SpecID, r.ACID}
		cur, seen := best[k]
		if !seen {
			best[k] = r
			order = append(order, k)
			continue
		}
		if worstOrder[r.Status] > worstOrder[cur.Status] {
			best[k] = r
		}
	}

	// Preserve first-seen order; stable enough for deterministic tests.
	out := make([]TestResult, 0, len(best))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

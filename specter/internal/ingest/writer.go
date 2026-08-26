// writer.go — serialize []TestResult to .specter-results.json.
// C-07: back-compat boolean passed is emitted alongside the new status field.
//
// @spec spec-ingest
package ingest

import (
	"encoding/json"
	"os"
	"sort"
)

// resultsFile is the on-disk JSON shape. Mirrors the structure
// internal/coverage consumes via ParseResultsFile.
type resultsFile struct {
	// Streams is emitted ascending by name, spec-coverage C-42.
	Streams []streamInfo  `json:"streams,omitempty"`
	Results []resultEntry `json:"results"`
}

type streamInfo struct {
	Name      string `json:"name"`
	Scanned   int    `json:"scanned"`
	Extracted int    `json:"extracted"`
}

type resultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Status Status `json:"status"`
	Passed bool   `json:"passed"` // back-compat — readers on spec-coverage < 1.9.0
	Stream string `json:"stream,omitempty"`
}

// WriteResultsFile merges, sorts, and writes results to path. Existing content
// is overwritten.
func WriteResultsFile(path string, results []TestResult) error {
	merged := MergeResults(results)

	out := resultsFile{Results: make([]resultEntry, 0, len(merged))}
	for _, r := range merged {
		entry := resultEntry{
			SpecID: r.SpecID,
			ACID:   r.ACID,
			Status: r.Status,
			Passed: r.Status == StatusPassed,
		}
		// The label is written only when the producer set one, so a
		// single-stream run emits exactly the file it emitted before this
		// field existed. spec-coverage C-41: missing means default.
		if r.Stream != "" && r.Stream != DefaultStream {
			entry.Stream = r.Stream
		}
		out.Results = append(out.Results, entry)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// MergeResults collapses duplicates to one entry per (spec, AC, stream) using
// the worst-status rule (C-08: errored > failed > skipped > passed), then
// sorts the result into the total order C-13 requires.
//
// The key gained stream in 1.5.0. Collapsing across streams would destroy the
// distinction the foundation records: two streams reporting on one criterion
// are two facts, not a contradiction to resolve. One run of one stream still
// emits one entry per pair.
//
// The order is ascending by spec id, then criterion id, then status rank
// worst-first, then stream name. It used to be first-seen order, which is not
// an order at all: it is whatever the runner emitted, so two producers over
// identical facts wrote different bytes. Worst-first within a criterion is for
// one reader in particular, a consumer that resolves a pair by first match
// rather than across every entry. Specter does not, per spec-coverage C-33,
// but the results file is a published artifact and a first-match reader over a
// best-first file reports the optimistic answer.
func MergeResults(in []TestResult) []TestResult {
	type key struct{ spec, ac, stream string }
	best := make(map[key]TestResult, len(in))

	for _, r := range in {
		k := key{r.SpecID, r.ACID, r.StreamOf()}
		cur, seen := best[k]
		if !seen || worstOrder[r.Status] > worstOrder[cur.Status] {
			best[k] = r
		}
	}

	out := make([]TestResult, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.SpecID != b.SpecID {
			return a.SpecID < b.SpecID
		}
		if a.ACID != b.ACID {
			return a.ACID < b.ACID
		}
		if wa, wb := worstOrder[a.Status], worstOrder[b.Status]; wa != wb {
			return wa > wb // worst first, C-13
		}
		return a.StreamOf() < b.StreamOf()
	})
	return out
}

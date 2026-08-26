// merge.go — read existing results files and combine them, spec-ingest C-15.
//
// @spec spec-ingest
package ingest

import (
	"encoding/json"
	"fmt"
	"os"
)

// MaxResultsFileBytes caps a --merge input, C-17. The same figure
// internal/coverage applies to the file it reads: one artifact shape, and two
// readers that capped differently would disagree about which files are
// readable. A test asserts the two are equal rather than a comment asking the
// next editor to remember.
const MaxResultsFileBytes = 16 << 20 // 16 MiB

// ReadResultsFile parses a written .specter-results.json back into the results
// and stream metadata it carries, so `--merge` can build an output from files
// rather than from runner output.
func ReadResultsFile(path string) ([]TestResult, []StreamInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var in resultsFile
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make([]TestResult, 0, len(in.Results))
	for _, e := range in.Results {
		status := e.Status
		if status == "" {
			// C-21's back-compat derivation, mirrored here so a file written
			// by an older producer merges with the meaning it had.
			status = StatusFailed
			if e.Passed {
				status = StatusPassed
			}
		}
		out = append(out, TestResult{
			SpecID: e.SpecID,
			ACID:   e.ACID,
			Status: status,
			Stream: e.Stream,
		})
	}
	return out, in.Streams, nil
}

// MergeStreams combines stream blocks by name, summing every count.
//
// Every count, not the ones that are easy. A stream declared by two inputs
// scanned both their inputs and extracted both their results, and leaving one
// figure at whichever input happened to be last would describe neither run.
func MergeStreams(in []StreamInfo) []StreamInfo {
	byName := map[string]StreamInfo{}
	for _, s := range in {
		cur := byName[s.Name]
		cur.Name = s.Name
		cur.Scanned += s.Scanned
		cur.Extracted += s.Extracted
		cur.ZeroTestEventPackages += s.ZeroTestEventPackages
		byName[s.Name] = cur
	}
	out := make([]StreamInfo, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	return out // WriteResultsFileWithStreams sorts, per C-42
}

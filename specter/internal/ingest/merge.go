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
func ReadResultsFile(path string) ([]TestResult, StreamBlock, error) {
	// C-17: refused by stat, before the read. Checking the length after
	// os.ReadFile is not a cap, because the allocation has already happened.
	// That ordering is the mistake spec-diff C-12 records from the v0.13 audit,
	// so it is stated in the constraint and done here rather than inferred.
	info, err := os.Stat(path)
	if err != nil {
		return nil, StreamBlock{}, err
	}
	if info.Size() > int64(MaxResultsFileBytes) {
		return nil, StreamBlock{}, fmt.Errorf("%s exceeds the %d byte limit for a results file (got %d bytes)",
			path, MaxResultsFileBytes, info.Size())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, StreamBlock{}, err
	}
	// Decoded through a wire struct so the `streams` key's presence survives
	// the read. A json.RawMessage stays nil for an absent key and holds bytes
	// for a declared one, including a declared empty one. Decoding straight
	// into resultsFile collapses those two, and AC-18 turns on telling them
	// apart. internal/coverage reads the same artifact the same way; the two
	// sides of one file agreeing on what presence means is the point.
	var wire struct {
		Streams json.RawMessage `json:"streams"`
		Results []resultEntry   `json:"results"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, StreamBlock{}, fmt.Errorf("%s: %w", path, err)
	}
	in := resultsFile{Results: wire.Results}
	block := newStreamBlock(wire.Streams != nil, nil)
	if len(wire.Streams) > 0 && string(wire.Streams) != "null" {
		if err := json.Unmarshal(wire.Streams, &block.streams); err != nil {
			return nil, StreamBlock{}, fmt.Errorf("%s: %w", path, err)
		}
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
	return out, block, nil
}

// MergeStreams combines stream blocks by name, summing every count.
//
// Every count, not the ones that are easy. A stream declared by two inputs
// scanned both their inputs and extracted both their results, and leaving one
// figure at whichever input happened to be last would describe neither run.
// A merge declares the block when any input declared it, including an input
// that declared an empty one. Anything weaker loses the presence that C-44
// reads, which is how a merge came to launder an artifact `coverage` refuses.
func MergeStreams(in []StreamBlock) StreamBlock {
	out := StreamBlock{}
	byName := map[string]StreamInfo{}
	for _, b := range in {
		out.declared = out.declared || b.declared
		for _, s := range b.streams {
			cur := byName[s.Name]
			cur.Name = s.Name
			cur.Scanned += s.Scanned
			cur.Extracted += s.Extracted
			cur.ZeroTestEventPackages += s.ZeroTestEventPackages
			byName[s.Name] = cur
		}
	}
	out.streams = make([]StreamInfo, 0, len(byName))
	for _, s := range byName {
		out.streams = append(out.streams, s)
	}
	return out // serializeResultsFile sorts, per C-42
}

// results.go — .specter-results.json support for pass-rate-aware coverage.
//
// v1.3.0 shipped pass-rate-aware Tier 1 via a boolean `passed` field.
// v1.9.0 extends the schema with an explicit `status` enum (passed | failed |
// skipped | errored) so `specter coverage --strict` can demote non-passing
// annotated ACs across all tiers. The boolean is preserved for back-compat.
//
// @spec spec-coverage
package coverage

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ResultEntry records the outcome of a single AC in a specific spec.
// Status (v1.9.0+) is the canonical field; Passed is derived for back-compat.
type ResultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Status string `json:"status,omitempty"`
	Passed bool   `json:"passed"`

	// Stream names the body of evidence this entry came from. Optional, and
	// an empty one reads as DefaultStream, so every file written before the
	// field existed keeps its exact meaning. C-41: the label is opaque.
	// Specter never branches on its value, and no sibling field carries a
	// role instead.
	Stream string `json:"stream,omitempty"`
}

// DefaultStream is what an entry with no stream label belongs to. C-41.
const DefaultStream = "default"

// StreamInfo records that a stream ran and what it observed. C-42: a stream
// that ran and found nothing and one that never ran both leave zero entries,
// so only this block tells them apart. Absent from the array means never ran.
type StreamInfo struct {
	Name      string `json:"name"`
	Scanned   int    `json:"scanned"`
	Extracted int    `json:"extracted"`

	// ZeroTestEventPackages counts packages the runner reported on that
	// produced no test event at all. Named for what was observed rather than
	// diagnosed: a package may have failed to build, may have had every test
	// filtered out, or may have no tests, and runner output cannot tell those
	// apart. C-42.
	ZeroTestEventPackages int `json:"zero_test_event_packages,omitempty"`

	// sourceIndex is the row's position in the file as written, recorded by
	// ParseResultsFile before the C-42 sort moves it. C-44 requires a
	// violation to point at the file the operator can open, not at the array
	// after a sort they never see. Unexported, so the JSON contract is
	// unchanged and a hand-built ResultsFile simply carries zeroes, which
	// ValidateStreams detects and falls back from.
	sourceIndex int
}

// ResultsFile is the parsed .specter-results.json structure.
type ResultsFile struct {
	// Streams is emitted in ascending name order, per C-42, so two runs over
	// an unchanged workspace produce identical bytes.
	Streams []StreamInfo  `json:"streams,omitempty"`
	Results []ResultEntry `json:"results"`

	// streamsPresent records whether the document carried a `streams` key,
	// which C-44 makes the whole test of presence. It cannot be read off
	// Streams: encoding/json decodes an absent key and an explicit
	// `streams: null` to the same nil slice, and C-44 refuses the second and
	// exempts the first.
	streamsPresent bool
}

// StreamsBlockPresent reports whether C-44's rules apply to this artifact.
//
// True when the document carried the key at all, `null` included. Also true
// for a ResultsFile assembled in memory with rows, since a caller that built
// rows declared a block. A hand-built file with no rows cannot express the
// difference between absent and empty and is read as absent, which is the
// safe direction: it is the shape a legacy file has.
func (rf *ResultsFile) StreamsBlockPresent() bool {
	return rf.streamsPresent || rf.Streams != nil
}

// StreamOf returns the stream an entry belongs to, resolving the empty label
// to DefaultStream. C-41: missing means default, and callers must go through
// here rather than comparing the raw field, or a legacy entry and an entry
// naming default explicitly would read as two streams.
func (e ResultEntry) StreamOf() string {
	if e.Stream == "" {
		return DefaultStream
	}
	return e.Stream
}

// ParseResultsFile parses .specter-results.json content. Normalizes the
// back-compat boolean and the status enum into a consistent pair so callers
// can use either field.
//
// C-21: accepts entries with only `passed`, only `status`, or both.
//
// MaxResultsFileBytes caps the input size before json.Unmarshal to prevent
// memory exhaustion when a malicious CI runner / PR commits a multi-GB
// .specter-results.json into the workspace. The structure is flat (one
// entry per (spec_id, ac_id) pair); 16 MiB is generous for ~100k entries.
const MaxResultsFileBytes = 16 << 20 // 16 MiB

func ParseResultsFile(data []byte) (*ResultsFile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > MaxResultsFileBytes {
		return nil, fmt.Errorf(".specter-results.json exceeds %d byte limit (got %d bytes)", MaxResultsFileBytes, len(data))
	}
	// Decoded through a wire struct so the `streams` key's presence survives.
	// A json.RawMessage stays nil for an absent key and holds the four bytes
	// of `null` for an explicit one, which is the only place those two are
	// still distinguishable. Decoding straight into ResultsFile collapses
	// them, and C-44 turns on telling them apart.
	var wire struct {
		Streams json.RawMessage `json:"streams"`
		Results []ResultEntry   `json:"results"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, err
	}
	var rf ResultsFile
	rf.Results = wire.Results
	rf.streamsPresent = wire.Streams != nil
	if len(wire.Streams) > 0 && string(wire.Streams) != "null" {
		if err := json.Unmarshal(wire.Streams, &rf.Streams); err != nil {
			return nil, err
		}
	}
	for i := range rf.Results {
		r := &rf.Results[i]
		switch {
		case r.Status != "":
			// Status-first: derive Passed from Status.
			r.Passed = r.Status == "passed"
		case r.Passed:
			r.Status = "passed"
		default:
			// Explicit passed:false, no status → mark as failed.
			r.Status = "failed"
		}
	}
	// C-42: the streams array carries a total order, ascending by name, so two
	// producers writing the same facts write the same bytes. Sorted on read as
	// well as on write, because a file a consumer hand-assembled is still an
	// artifact Specter re-emits, and an order that depended on who wrote it
	// would make a diff of two runs show churn that is not a change.
	//
	// The source position is recorded first, because the sort is what destroys
	// it and C-44's coordinates are stated against the file as written.
	for i := range rf.Streams {
		rf.Streams[i].sourceIndex = i
	}
	sort.Slice(rf.Streams, func(i, j int) bool { return rf.Streams[i].Name < rf.Streams[j].Name })
	return &rf, nil
}

// Status ranks, worst last. A pair that carries several entries resolves to
// the highest rank among them.
//
// The ranks are explicit rather than a map lookup on purpose. A map returns
// its zero value for a key it does not hold, so an unrecognized status would
// rank at the passed level and a typo would silently mark a criterion as
// passing. C-33 requires the opposite: a status outside the C-21 enum ranks at
// the failed level and never at passed.
const (
	rankPassed = iota
	rankSkipped
	rankFailed
	rankErrored
)

// statusRank returns the rank of one status value.
func statusRank(status string) int {
	switch status {
	case "passed":
		return rankPassed
	case "skipped":
		return rankSkipped
	case "errored":
		return rankErrored
	default:
		// "failed" and every value outside the enum.
		return rankFailed
	}
}

// entryStatus returns the canonical status of one entry, applying the C-21
// derivation for entries that carry only the back-compat boolean. Entries that
// came through ParseResultsFile already carry a status; entries built in
// process may not.
func entryStatus(r ResultEntry) string {
	if r.Status != "" {
		return r.Status
	}
	if r.Passed {
		return "passed"
	}
	return "failed"
}

// resolve returns the status of the worst entry matching (specID, acID), and
// whether any entry matched at all. Duplicate pairs are legal input, because
// ParseResultsFile validates size and JSON shape and nothing about uniqueness.
//
// C-33. Resolving across every match rather than stopping at the first one is
// what makes the answer independent of row order. First-match resolution let
// two files describing identical facts report 0% and 100% on one workspace.
func (rf *ResultsFile) resolve(specID, acID string) (string, bool) {
	if rf == nil {
		return "", false
	}
	worst := ""
	worstRank := -1
	for _, r := range rf.Results {
		if r.SpecID != specID || r.ACID != acID {
			continue
		}
		s := entryStatus(r)
		if rank := statusRank(s); rank > worstRank {
			worst, worstRank = s, rank
		}
	}
	return worst, worstRank >= 0
}

// passed returns true only when every entry matching the given spec+AC
// resolves to passed, or when no entry exists (absent means "not recorded
// yet", not "failed"). Used by pre-1.9 pass-rate-aware Tier 1 coverage.
//
// C-33 binds this call path as well as the --strict one. Both took the first
// match before, so a fix confined to status() would leave the Tier 1 pass-rate
// path order-dependent.
func (rf *ResultsFile) passed(specID, acID string) bool {
	worst, found := rf.resolve(specID, acID)
	if !found {
		return true
	}
	return worst == "passed"
}

// InvalidStatuses scans the parsed results and returns a map of unique
// unrecognized `status` values to their occurrence counts. The documented
// enum is {passed, failed, skipped, errored} per C-21; any other non-empty
// value is reported here.
//
// Parsing behavior is unchanged — unrecognized values still derive
// `Passed = false` in ParseResultsFile (treated as not-passed). This
// helper is purely diagnostic; the CLI surfaces the result as a stderr
// warning (text mode) or `invalid_status_warnings` array (--json) so
// the operator can spot typos like `status: "pass"` that pre-v0.14
// silently demoted ACs under --strict.
//
// C-30 (AC-35). Returns nil-safe (zero map for a nil receiver).
func (rf *ResultsFile) InvalidStatuses() map[string]int {
	if rf == nil {
		return nil
	}
	var out map[string]int
	for _, r := range rf.Results {
		if r.Status == "" {
			continue
		}
		switch r.Status {
		case "passed", "failed", "skipped", "errored":
			continue
		}
		if out == nil {
			out = make(map[string]int)
		}
		out[r.Status]++
	}
	return out
}

// status returns the canonical status for a (spec, AC), or "unknown" if no
// entry exists. Under --strict (BuildCoverageReportStrict with strict=true),
// "unknown" is treated as uncovered — the point of --strict is that every
// annotated AC must have a verified passing result.
//
// When several entries name the same pair, the worst one wins, ranked passed
// (best), then skipped, then failed, then errored (worst).
//
// C-22 (AC-22), C-33 (AC-38).
func (rf *ResultsFile) status(specID, acID string) string {
	worst, found := rf.resolve(specID, acID)
	if !found {
		return "unknown"
	}
	return worst
}

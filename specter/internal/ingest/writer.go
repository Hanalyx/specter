// writer.go — serialize []TestResult to .specter-results.json.
// C-07: back-compat boolean passed is emitted alongside the new status field.
//
// @spec spec-ingest
package ingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Hanalyx/specter/internal/coverage"
)

// resultsFile is the on-disk JSON shape. Mirrors the structure
// internal/coverage consumes via ParseResultsFile.
type resultsFile struct {
	// Streams is emitted ascending by name, spec-coverage C-42.
	//
	// A pointer, not a slice, and that is the whole fix for AC-18. With a
	// plain slice plus omitempty, a declared-but-empty block and an absent
	// key are the same value and serialize to the same bytes, so a producer
	// silently converted `streams: []` into no key at all. C-44 turns on
	// telling those two apart, so an artifact that arrived inconsistent left
	// consistent and `coverage` accepted what it had just refused. A nil
	// pointer omits the key; a pointer to an empty slice writes `[]`.
	Streams *[]StreamInfo `json:"streams,omitempty"`
	Results []resultEntry `json:"results"`
}

// StreamBlock is a `streams` block together with whether the key was there.
//
// The two travel as one value on purpose. They were separate before, as a
// []StreamInfo whose emptiness stood in for absence, and that conflation is
// what shipped the laundering path.
//
// Both fields are unexported, and that is the part bundling alone did not
// buy. An earlier revision exported them and claimed in a comment that a
// caller could not forward the rows while dropping the presence. A caller
// could: `ingest.StreamBlock{Streams: rows}` compiles, reads as complete, and
// declares nothing. Adjacent is not inseparable. Construction now lives in
// this package, so outside it the undeclared-with-rows state cannot be
// written down at all.
type StreamBlock struct {
	// declared is true when the input carried a `streams` key, including
	// when it carried an empty or an explicitly null one.
	declared bool
	streams  []StreamInfo
}

// newStreamBlock is the only way a StreamBlock is built. Unexported on
// purpose: presence is a fact about an input this package read, not something
// a consumer supplies.
func newStreamBlock(declared bool, streams []StreamInfo) StreamBlock {
	return StreamBlock{declared: declared, streams: streams}
}

// Len reports how many streams the block declares.
//
// Exported because the CLI prints "across N stream(s)" and needs the count.
// A count is all it needs, so a count is all it gets: handing back the slice
// would put the rows in a caller's hands again without the presence.
func (b StreamBlock) Len() int { return len(b.streams) }

// Declared reports whether the input carried the key at all. Zero rows and no
// key are different facts, and this is the one C-44 turns on.
func (b StreamBlock) Declared() bool { return b.declared }

// StreamInfo records what one stream's run observed. Mirrors the shape
// internal/coverage reads, spec-coverage C-42.
type StreamInfo struct {
	Name      string `json:"name"`
	Scanned   int    `json:"scanned"`
	Extracted int    `json:"extracted"`
	// ZeroTestEventPackages is named for what was observed. C-16 forbids
	// calling it a build failure here, in the summary line, or in this name,
	// because ingest cannot tell a build failure from a filtered-out package
	// or one with no tests.
	ZeroTestEventPackages int `json:"zero_test_event_packages,omitempty"`
}

type resultEntry struct {
	SpecID string `json:"spec_id"`
	ACID   string `json:"ac_id"`
	Status Status `json:"status"`
	Passed bool   `json:"passed"` // back-compat — readers on spec-coverage < 1.9.0
	Stream string `json:"stream,omitempty"`
}

// WriteResultsFile merges, sorts, and writes results to path. Existing content
// is overwritten. No streams block, which is what an unlabeled run writes:
// spec-ingest C-14 promises such a run produces exactly the file it produced
// before the field existed, and even an empty array would break that.
func WriteResultsFile(path string, results []TestResult) error {
	return WriteResultsFileWithStreams(path, results, nil)
}

// WriteResultsFileWithStreams is WriteResultsFile with the top-level streams
// block C-16 requires when a run names a stream. A nil or empty slice writes
// no block at all rather than an empty array.
func WriteResultsFileWithStreams(path string, results []TestResult, streams []StreamInfo) error {
	// C-16: a run that names a stream writes the block, and an unlabeled run
	// writes none. Presence and non-emptiness coincide on this path, which is
	// why it kept working while `--merge` did not.
	data, err := serializeResultsFile(results, newStreamBlock(len(streams) > 0, streams))
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// serializeResultsFile builds the exact bytes WriteResultsFileWithStreams would
// write, without writing them.
//
// Split out so a caller can inspect the artifact it is about to produce. The
// bytes are what matters: `internal/ingest` and `internal/coverage` each carry
// their own StreamInfo, and a check over this package's structs would be a
// check over a conversion rather than over the artifact a consumer reads.
// Serializing and re-reading removes the conversion from the question, and it
// is the only way to test the distinction C-44 turns on, since an absent
// `streams` key and an empty one are the same Go value here and different
// bytes.
func serializeResultsFile(results []TestResult, block StreamBlock) ([]byte, error) {
	merged := MergeResults(results)

	out := resultsFile{Results: make([]resultEntry, 0, len(merged))}
	for _, r := range merged {
		entry := resultEntry{
			SpecID: r.SpecID,
			ACID:   r.ACID,
			Status: r.Status,
			Passed: r.Status == StatusPassed,
		}
		if r.Stream != "" && r.Stream != DefaultStream {
			entry.Stream = r.Stream
		}
		out.Results = append(out.Results, entry)
	}
	// Presence decides whether the key is written, and the rows decide what
	// is in it. Keying this off len(rows) is the bug: it drops a declared
	// empty block, and a declared empty block beside a labeled entry is
	// precisely the artifact C-44 rejects.
	if block.declared {
		rows := append([]StreamInfo(nil), block.streams...)
		if rows == nil {
			rows = []StreamInfo{}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
		out.Streams = &rows
	}
	return json.MarshalIndent(out, "", "  ")
}

// ErrMergeWouldBeRefused is the C-15 refusal: the artifact a merge is about to
// write is one `coverage` would reject.
var ErrMergeWouldBeRefused = errors.New("the merge was refused")

// ErrMergeTooLarge is the C-15 size refusal, kept distinct from the
// consistency one. They need different responses: a missing declaration is
// added to a block, and a size is not edited away. Reporting one as the other
// sends an operator looking for a stream that is not the problem.
var ErrMergeTooLarge = errors.New("the merged artifact is too large to be read back")

// WriteMergedResultsFile writes a `--merge` output, and refuses to write one
// `coverage` would refuse.
//
// C-15: the prospective artifact must satisfy `spec-coverage` C-44 before
// anything is written. Two inputs that are each valid alone can combine into
// one that is not, so no check over the inputs would catch this.
//
// The rules are not restated here. The serialized bytes go through the same
// reader and the same validator `coverage` uses, so a rule added to C-44 binds
// this refusal on the day it lands and this package holds no copy of the stream
// policy to go stale.
//
// Nothing is written on refusal, which is why the check happens before
// os.WriteFile rather than after: a truncated or replaced output would destroy
// the artifact the operator still had, which is worse than the one refused.
func WriteMergedResultsFile(path string, results []TestResult, block StreamBlock) error {
	data, err := serializeResultsFile(results, block)
	if err != nil {
		return err
	}
	// C-15: the artifact must also be one `coverage` can read. Two inputs that
	// each pass C-17's per-input cap can sum past it, and a file the consumer
	// cannot open is this rule's failure one command later.
	if len(data) > int(coverage.MaxResultsFileBytes) {
		return fmt.Errorf("%w: %d bytes exceeds the %d byte limit", ErrMergeTooLarge, len(data), coverage.MaxResultsFileBytes)
	}
	rf, err := coverage.ParseResultsFile(data)
	if err != nil {
		return fmt.Errorf("%w: the merged artifact does not parse: %v", ErrMergeWouldBeRefused, err)
	}
	if violations := coverage.ValidateStreams(rf); len(violations) > 0 {
		// The shared message carries its own "error: " prefix, because the
		// gates print it straight to stderr. Wrapped in another error here, so
		// the prefix is trimmed the same way spec-coverage C-40 trims it for a
		// sync phase message.
		msg := strings.TrimPrefix(coverage.StreamValidationMessage(violations), "error: ")
		return fmt.Errorf("%w: the merged artifact is inconsistent. %s", ErrMergeWouldBeRefused, msg)
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

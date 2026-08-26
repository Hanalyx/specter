package coverage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Streams-block consistency, spec-coverage C-44 and AC-72.
//
// The check lives here, in the shared pure builder, so every command that
// consumes the artifact as coverage evidence inherits it. That is `coverage`
// and `sync` today, and the three strictness modes reach this file through
// two different constructors. A rule wired into one command is
// bugs/done/SP-SP-058, where `sync` returned a different verdict for the same
// workspace.
//
// It deliberately does not reach `ingest --merge`, which reads the same file
// as a producer assembling a new one rather than as a consumer judging a
// workspace, and which has its own contract in spec-ingest C-15 and C-17.

// The five kinds C-44 names. Exported so a caller can compare without
// repeating a string literal that would then drift.
const (
	KindUndeclaredStream      = "undeclared_stream"
	KindDuplicateStream       = "duplicate_stream"
	KindEmptyStreamName       = "empty_stream_name"
	KindNegativeCount         = "negative_count"
	KindExtractedBelowEntries = "extracted_below_entries"
)

// ResultsValidationError is one C-44 violation.
//
// Exactly one coordinate is set, never both and never neither.
// undeclared_stream is about an entry, so it carries ResultIndex. Every other
// kind is about a declared row, so it carries StreamIndex. Both are pointers
// because a coordinate of zero is a real position that must serialize, and
// omitempty on a pointer drops only nil.
type ResultsValidationError struct {
	Kind    string `json:"kind"`
	Stream  string `json:"stream"`
	Message string `json:"message"`

	StreamIndex *int `json:"stream_index,omitempty"`
	ResultIndex *int `json:"result_index,omitempty"`
}

func intPtr(v int) *int { return &v }

// streamsInSourceOrder returns the declared rows in the order the file wrote
// them.
//
// ParseResultsFile sorts ascending by name, and sort.Slice is not stable, so
// two rows sharing a name can arrive in either order. Detecting a duplicate
// over the sorted array would then report a different row on different runs,
// which is the nondeterminism class bugs/done/SP-SP-009 already cost this
// project once.
//
// Falls back to the given order when the recorded indexes are not a
// permutation of 0..n-1, which is what a ResultsFile assembled by hand rather
// than parsed will carry.
func streamsInSourceOrder(streams []StreamInfo) []StreamInfo {
	out := make([]StreamInfo, len(streams))
	seen := make([]bool, len(streams))
	for _, s := range streams {
		if s.sourceIndex < 0 || s.sourceIndex >= len(streams) || seen[s.sourceIndex] {
			return streams
		}
		seen[s.sourceIndex] = true
		out[s.sourceIndex] = s
	}
	return out
}

// ValidateStreams reports every way a results file's streams block is
// inconsistent, in the total order C-44 fixes.
//
// A nil results file, or one whose block is absent, is a legacy artifact and
// produces nothing. An explicit empty block is present and is validated, which
// is the distinction C-44 rests on: Go decodes an absent key to a nil slice
// and `streams: []` to an empty non-nil one.
//
// Linear in the artifact plus the sort of what it finds, with no scan of
// results per stream. The entry counts come from one pass over results into a
// map, so a file just under the spec-ingest C-17 cap cannot buy an operator's
// CPU with a shape no runner would produce.
func ValidateStreams(rf *ResultsFile) []ResultsValidationError {
	if rf == nil || rf.Streams == nil {
		return nil
	}

	// One pass over results. Keyed by StreamOf, so an unlabeled entry counts
	// toward default rather than toward the empty name, which is what C-41
	// makes those two the same stream for.
	entriesPerStream := make(map[string]int, len(rf.Streams))
	for i := range rf.Results {
		entriesPerStream[rf.Results[i].StreamOf()]++
	}

	// One pass over streams for the declared set.
	declared := make(map[string]bool, len(rf.Streams))
	for i := range rf.Streams {
		declared[rf.Streams[i].Name] = true
	}

	var out []ResultsValidationError
	seen := make(map[string]bool, len(rf.Streams))

	for _, s := range streamsInSourceOrder(rf.Streams) {
		at := s.sourceIndex

		switch {
		case s.Name == "":
			// C-44(c): no name to print, so the row is identified by position
			// and the message says what is wrong with it.
			out = append(out, ResultsValidationError{
				Kind:        KindEmptyStreamName,
				Stream:      "",
				Message:     fmt.Sprintf("streams[%d] declares an empty name", at),
				StreamIndex: intPtr(at),
			})
		case seen[s.Name]:
			out = append(out, ResultsValidationError{
				Kind:        KindDuplicateStream,
				Stream:      s.Name,
				Message:     fmt.Sprintf("streams[%d] repeats the name %q, which leaves two counts for one stream", at, s.Name),
				StreamIndex: intPtr(at),
			})
		default:
			seen[s.Name] = true
		}

		// C-44(d): one violation per row, naming every negative field in the
		// fixed order, because three would otherwise share a kind, a stream
		// and a stream_index and the total order could not break the tie.
		var negatives []string
		if s.Scanned < 0 {
			negatives = append(negatives, "scanned "+strconv.Itoa(s.Scanned))
		}
		if s.Extracted < 0 {
			negatives = append(negatives, "extracted "+strconv.Itoa(s.Extracted))
		}
		if s.ZeroTestEventPackages < 0 {
			negatives = append(negatives, "zero_test_event_packages "+strconv.Itoa(s.ZeroTestEventPackages))
		}
		if len(negatives) > 0 {
			out = append(out, ResultsValidationError{
				Kind:        KindNegativeCount,
				Stream:      s.Name,
				Message:     fmt.Sprintf("streams[%d] declares a negative count: %s", at, strings.Join(negatives, ", ")),
				StreamIndex: intPtr(at),
			})
		}

		// C-44(e): lower than the evidence beside it is incoherent. Higher is
		// legitimate, because spec-ingest C-08 collapses duplicates within a
		// stream and --merge sums across inputs.
		//
		// A negative extracted suppresses this one. It is derived from the
		// first: a count below zero is below any entry count, and reporting
		// both tells an operator to fix a consequence beside its cause.
		if s.Extracted >= 0 {
			if n := entriesPerStream[s.Name]; s.Extracted < n {
				out = append(out, ResultsValidationError{
					Kind:        KindExtractedBelowEntries,
					Stream:      s.Name,
					Message:     fmt.Sprintf("stream %q declares extracted %d with %d entries carrying that label", s.Name, s.Extracted, n),
					StreamIndex: intPtr(at),
				})
			}
		}
	}

	// One pass over results. C-44(a): default never requires declaration,
	// whether an entry reaches it by carrying no label or by naming it
	// outright. That exemption is what lets a project adopt --stream one job
	// at a time.
	for i := range rf.Results {
		name := rf.Results[i].StreamOf()
		if name == DefaultStream || declared[name] {
			continue
		}
		out = append(out, ResultsValidationError{
			Kind:        KindUndeclaredStream,
			Stream:      name,
			Message:     fmt.Sprintf("results[%d] names stream %q, which the block does not declare", i, name),
			ResultIndex: intPtr(i),
		})
	}

	sortValidationErrors(out)
	return out
}

// sortValidationErrors puts the violations in the total order C-44 states:
// kind, then stream, then stream_index, then result_index. Every element
// carries exactly one coordinate, so an absent one sorts as -1 and never
// competes with a real position.
func sortValidationErrors(v []ResultsValidationError) {
	coord := func(p *int) int {
		if p == nil {
			return -1
		}
		return *p
	}
	sort.SliceStable(v, func(i, j int) bool {
		a, b := v[i], v[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Stream != b.Stream {
			return a.Stream < b.Stream
		}
		if ai, bi := coord(a.StreamIndex), coord(b.StreamIndex); ai != bi {
			return ai < bi
		}
		return coord(a.ResultIndex) < coord(b.ResultIndex)
	})
}

// StreamValidationMessage is the one stderr line a refusal writes. It names
// the count and the first violation, so an operator sees which stream is
// wrong without reading the JSON document.
func StreamValidationMessage(errs []ResultsValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	first := errs[0]
	subject := fmt.Sprintf("stream %q", first.Stream)
	if first.Stream == "" {
		subject = "an unnamed stream"
	}
	return fmt.Sprintf("error: the results file declares an inconsistent streams block: %d violation(s), first %s on %s. %s",
		len(errs), first.Kind, subject, first.Message)
}

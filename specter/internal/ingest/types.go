// Package ingest consumes CI-native test output formats (JUnit XML, go test
// -json) and converts them into the .specter-results.json shape that
// spec-coverage reads under --strict mode.
//
// The package is pure — parsers take []byte, writers take paths. cmd/specter
// is the thin I/O wrapper. (spec-ingest C-06)
//
// @spec spec-ingest
package ingest

// Status is the outcome of a single test relative to an acceptance criterion.
// Values follow spec-ingest C-05.
type Status string

const (
	StatusPassed  Status = "passed"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
	StatusErrored Status = "errored"
)

// TestResult is a single (spec, AC) → status mapping extracted from a runner's
// output. Tests that don't map to a (spec, AC) pair are dropped at parse time
// (C-04), so every TestResult has both SpecID and ACID populated.
type TestResult struct {
	SpecID string
	ACID   string
	Status Status
	Name   string // original test name, for diagnostics

	// Stream names the body of evidence this result came from. Empty means
	// DefaultStream. spec-coverage C-41: the label is opaque, so nothing here
	// branches on its value.
	Stream string
}

// DefaultStream is what a result with no stream label belongs to, matching
// spec-coverage C-41 so a file written here and a file read there agree.
const DefaultStream = "default"

// StreamOf resolves the empty label to DefaultStream. Callers compare through
// this rather than the raw field, or an unlabeled result and one naming
// default explicitly would merge as two streams instead of one.
func (r TestResult) StreamOf() string {
	if r.Stream == "" {
		return DefaultStream
	}
	return r.Stream
}

// worstOrder ranks statuses from best (passed) to worst (errored). MergeResults
// picks the worst when multiple results collide on the same (spec, AC) pair.
// C-08.
var worstOrder = map[Status]int{
	StatusPassed:  0,
	StatusSkipped: 1,
	StatusFailed:  2,
	StatusErrored: 3,
}

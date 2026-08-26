package coverage

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Hanalyx/specter/internal/schema"
)

// Streams-block validation complexity, roadmap 3A5, spec-coverage C-44/AC-72.
//
// C-44 states the linear bound as a requirement rather than an implementation
// note, and says the binding "has to be something a mutation reintroducing the
// per-stream scan fails, which a behavioral assertion over a small fixture
// cannot be". Every other expected output in AC-72 is satisfied by the nested
// version, because the answers are identical and only the work differs. So this
// file asserts the work.
//
// The artifact is untrusted and may sit just under the 16 MiB cap spec-ingest
// C-17 sets. A size limit bounds memory and not work, so a file that passes the
// cap can still buy an operator's CPU with a shape no runner would produce.
//
// Two assertions, and the first is what makes this red today. Validation must
// run at all, observed through the marshaled report rather than through a struct
// field, because the field does not exist yet and naming it would report a build
// failure. A build failure says the work is unfinished rather than that the
// behavior is wrong.

// complexityArtifact builds a legal streams block carrying one deliberate
// violation, with streamCount declared streams and resultCount entries.
//
// All entries name the bulk stream, so a nested implementation walks every
// entry once per declared stream while a map pass walks them once in total.
// The filler streams declare zero counts and carry no entries, which C-44
// allows: a stream that ran and found nothing is a state the block exists to
// record.
func complexityArtifact(streamCount, resultCount int) string {
	var b strings.Builder
	b.WriteString(`{"streams":[{"name":"bulk","scanned":`)
	b.WriteString(strconv.Itoa(resultCount))
	b.WriteString(`,"extracted":`)
	b.WriteString(strconv.Itoa(resultCount))
	b.WriteString(`}`)
	for i := 1; i < streamCount; i++ {
		b.WriteString(`,{"name":"filler`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`","scanned":0,"extracted":0}`)
	}
	b.WriteString(`],"results":[`)
	for i := 0; i < resultCount; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"bulk"}`)
	}
	// One entry names a stream the block never declares. C-44(a) refuses it,
	// which is what proves validation ran over an artifact this large.
	b.WriteString(`,{"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"ghost"}`)
	b.WriteString(`]}`)
	return b.String()
}

// timeBuild parses outside the measured region and times the builder alone, so
// the number reflects validation rather than JSON decoding. Decoding is the
// same work at both sizes and would dilute the ratio.
func timeBuild(t *testing.T, body string) (time.Duration, string) {
	t.Helper()
	rf, err := ParseResultsFile([]byte(body))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}
	annotations := []AnnotationMatch{{
		File:   "t_test.go",
		SpecID: "s",
		ACIDs:  []string{"AC-01", "AC-02"},
	}}
	start := time.Now()
	rep, err := BuildCoverageReportMode(
		[]schema.SpecAST{streamSpec()}, annotations, map[int]int{2: 80}, rf, streamMode)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("report did not build: %v", err)
	}
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	return elapsed, string(out)
}

// fastestBuild takes the minimum of several runs. A minimum is the right
// statistic for a scheduler-noise floor: noise only ever adds time. Each build
// costs about 1.2 ms, so five runs is cheap next to generating the artifact.
func fastestBuild(t *testing.T, body string) (time.Duration, string) {
	t.Helper()
	best := time.Duration(1<<62 - 1)
	var doc string
	for i := 0; i < 5; i++ {
		d, out := timeBuild(t, body)
		if d < best {
			best = d
		}
		doc = out
	}
	return best, doc
}

// @spec spec-coverage
// @ac AC-72
//
// C-44: validation runs in O(streams + results + violations log violations)
// with no scan of results per stream.
func TestStreamValidationIsLinearInTheArtifact(t *testing.T) {
	t.Run("spec-coverage/AC-72 results are counted once, not once per stream", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds two multi-megabyte artifacts")
		}

		const results = 40000
		const fewStreams = 50
		const manyStreams = 2000

		fast, doc := fastestBuild(t, complexityArtifact(fewStreams, results))
		slow, _ := fastestBuild(t, complexityArtifact(manyStreams, results))

		// Red today. Nothing validates the block, so the report carries no
		// violation array and the timing assertion below would pass on an
		// implementation that does no work at all.
		if !strings.Contains(doc, "results_validation_errors") {
			t.Fatalf("C-44: the report carries no results_validation_errors key, so validation did not run. The timing assertion below cannot mean anything until it does")
		}
		if !strings.Contains(doc, "ghost") {
			t.Errorf("C-44: an entry names the undeclared stream ghost and no violation names it, so validation ran without reaching C-44(a)")
		}

		// A nested scan multiplies work by the stream count, so a 40-fold
		// increase in streams over an unchanged result array shows up here.
		// A map pass adds one bucket per stream and stays flat.
		//
		// Calibrated by measurement rather than by arithmetic. The builder
		// costs 1.17 ms at 50 streams and 1.20 ms at 2000 before any
		// validation, so the constant term is flat and does not hide the
		// signal. Measured over this fixture shape, the per-stream scan adds
		// 1.05 ms and 40.5 ms, and a map pass adds 0.34 ms and 0.37 ms. That
		// puts a correct build near 1.04 and a nested one near 19.
		const allowed = 2.5
		ratio := float64(slow) / float64(fast)
		if ratio > allowed {
			t.Errorf("C-44: %d streams took %v and %d streams took %v over the same %d results, a factor of %.1f. Want under %.1f. Work grew with the stream count, which is the per-stream scan C-44 forbids",
				fewStreams, fast, manyStreams, slow, results, ratio, allowed)
		}
	})
}

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
// Every declared stream carries entries and declares counts matching them, and
// the total number of entries is the same at both sizes. Only the stream count
// changes. An earlier draft put every entry on one stream and gave the rest
// zero counts and no entries, which an implementation could skip: a nested scan
// that visits only streams with a nonzero count would avoid the multiplication
// this test exists to detect, and would have measured as linear.
//
// A nested implementation walks every entry once per declared stream. A map
// pass walks them once in total and then does one lookup per stream.
func complexityArtifact(streamCount, resultCount int) string {
	per := resultCount / streamCount
	var b strings.Builder
	b.WriteString(`{"streams":[`)
	for i := 0; i < streamCount; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"name":"s`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`","scanned":`)
		b.WriteString(strconv.Itoa(per))
		b.WriteString(`,"extracted":`)
		b.WriteString(strconv.Itoa(per))
		b.WriteString(`}`)
	}
	b.WriteString(`],"results":[`)
	for i := 0; i < streamCount; i++ {
		name := "s" + strconv.Itoa(i)
		for j := 0; j < per; j++ {
			if i > 0 || j > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"`)
			b.WriteString(name)
			b.WriteString(`"}`)
		}
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

		fast, fastDoc := fastestBuild(t, complexityArtifact(fewStreams, results))
		slow, slowDoc := fastestBuild(t, complexityArtifact(manyStreams, results))

		// Red today. Nothing validates the block, so the report carries no
		// violation array and the timing assertion below would pass on an
		// implementation that does no work at all.
		//
		// Both reports, not only the small one. An implementation that
		// validates small blocks and bails out above some size produces a flat
		// ratio and would pass a check that only ever read the fast document.
		for _, d := range []struct {
			label string
			doc   string
		}{{"50 streams", fastDoc}, {"2000 streams", slowDoc}} {
			if !strings.Contains(d.doc, "results_validation_errors") {
				t.Fatalf("C-44 (%s): the report carries no results_validation_errors key, so validation did not run. The timing assertion below cannot mean anything until it does", d.label)
			}
			if !strings.Contains(d.doc, "ghost") {
				t.Errorf("C-44 (%s): an entry names the undeclared stream ghost and no violation names it, so validation ran without reaching C-44(a)", d.label)
			}
		}

		// A nested scan multiplies work by the stream count, so a 40-fold
		// increase in streams over an unchanged result array shows up here.
		// A map pass adds one bucket per stream and stays flat.
		//
		// Calibrated by measurement rather than by arithmetic, and
		// re-measured after the fixture changed shape. The builder costs
		// 1.19 ms at both sizes before any validation, so the constant term is
		// flat and does not hide the signal. Over this fixture the per-stream
		// scan adds 4.13 ms and 119.7 ms, and a map pass adds 0.47 ms and
		// 0.82 ms. That puts a correct build near 1.21 and a nested one near
		// 22.7.
		//
		// The map pass is not flat here, unlike the one-stream draft this
		// replaced: 2000 distinct keys cost more to build and probe than one.
		// That is why the gate is not tighter than 2.5.
		const allowed = 2.5
		ratio := float64(slow) / float64(fast)
		if ratio > allowed {
			t.Errorf("C-44: %d streams took %v and %d streams took %v over the same %d results, a factor of %.1f. Want under %.1f. Work grew with the stream count, which is the per-stream scan C-44 forbids",
				fewStreams, fast, manyStreams, slow, results, ratio, allowed)
		}
	})
}

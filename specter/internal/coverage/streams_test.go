package coverage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/schema"
)

// Multi-stream evidence, roadmap 3A1 to 3A3, SSRB-103.
//
// These assert on observable output rather than on struct fields. ResultEntry
// has no Stream field and ResultsFile has no Streams field yet, so a test that
// named either would report a build failure, and a build failure says the work
// is unfinished rather than that the behavior is wrong.
//
// One fixture per criterion. An earlier draft shared one across all three, and
// the shared fixture contradicted AC-69: it declared the js stream with an
// extracted count of zero and then gave js an entry. A fixture describing a
// state that cannot occur proves nothing about the case it stands for.

// streamMode is the outcome-verified path, which is the one C-43 governs.
var streamMode = ClassifyMode{Strict: true}

func streamSpec() schema.SpecAST {
	return schema.SpecAST{
		ID:   "s",
		Tier: 2,
		AcceptanceCriteria: []schema.AcceptanceCriterion{
			{ID: "AC-01"},
			{ID: "AC-02"},
		},
	}
}

// coveredByCriterion runs the artifact through the verdict path and reports the
// covered answer per criterion, so these assert C-43's covered-rule contract
// rather than the lookup underneath it.
func coveredByCriterion(t *testing.T, body string) map[string]bool {
	t.Helper()
	rf, err := ParseResultsFile([]byte(body))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}
	annotated := map[string]bool{"AC-01": true, "AC-02": true}
	out := map[string]bool{}
	for _, v := range buildVerdicts(streamSpec(), annotated, rf, streamMode) {
		out[v.ACID] = v.Covered(streamMode)
	}
	return out
}

// reportJSON builds the whole coverage report over an artifact and returns its
// JSON. AC-68 says the three shapes agree field for field, and a covered map
// would miss covered_acs, uncovered_acs, coverage_pct, passes_threshold, the
// summary counts, and the document shape itself. Comparing the marshaled
// report covers all of them at once and fails on any field that moves.
func reportJSON(t *testing.T, body string) string {
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
	// Mode matters here. BuildCoverageReportWithResults carries ClassifyMode{},
	// where a result decides the verdict only at Tier 1, so a Tier 2 fixture on
	// that path produces a report no result can move and the comparison below
	// would pass whatever the stream semantics did. streamMode is Strict, which
	// is the outcome-verified path C-43 governs.
	rep, err := BuildCoverageReportMode([]schema.SpecAST{streamSpec()}, annotations, map[int]int{2: 80}, rf, streamMode)
	if err != nil {
		t.Fatalf("report did not build: %v", err)
	}
	out, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func marshalResults(t *testing.T, body string) string {
	t.Helper()
	rf, err := ParseResultsFile([]byte(body))
	if err != nil {
		t.Fatalf("fixture did not parse: %v", err)
	}
	out, err := json.Marshal(rf)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// streamNamesOf decodes the streams array out of a marshaled results file and
// returns the names in emitted order. A test-local shape, because the field it
// reads does not exist on ResultsFile yet and naming that field would turn a
// behavioral failure into a build failure.
func streamNamesOf(t *testing.T, artifact string) []string {
	t.Helper()
	var doc struct {
		Streams []struct {
			Name string `json:"name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(artifact), &doc); err != nil {
		t.Fatalf("the artifact did not decode: %v", err)
	}
	out := make([]string, 0, len(doc.Streams))
	for _, st := range doc.Streams {
		out = append(out, st.Name)
	}
	return out
}

// @spec spec-coverage
// @ac AC-68
//
// C-41: the label is optional, opaque, and moves no verdict. A legacy file, a
// file naming `default` explicitly, and a file split across two streams over
// the same facts must produce the same covered answer for every criterion.
func TestStreamLabelIsCarried(t *testing.T) {
	t.Run("spec-coverage/AC-68 the label is carried and no verdict moves", func(t *testing.T) {
		const legacy = `{"results":[
			{"spec_id":"s","ac_id":"AC-01","status":"passed"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed"}]}`
		const explicitDefault = `{"results":[
			{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"default"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"default"}]}`
		// The same facts, split across two streams.
		const split = `{"results":[
			{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"js"}]}`

		verdicts := coveredByCriterion(t, legacy)
		if len(verdicts) != 2 {
			t.Fatalf("built %d verdicts, want 2; the fixture no longer reaches the case", len(verdicts))
		}
		// The control that makes an equality assertion mean something: the two
		// criteria must disagree, or identical reports would prove nothing.
		if verdicts["AC-01"] == verdicts["AC-02"] {
			t.Fatalf("both criteria are covered=%v, so comparing whole reports cannot detect a change", verdicts["AC-01"])
		}

		// Field for field, per AC-68, which is the whole marshaled report and
		// not a covered map. A label that survived while a percentage moved
		// would pass a covered-only comparison.
		base := reportJSON(t, legacy)
		if got := reportJSON(t, explicitDefault); got != base {
			t.Errorf("C-41: naming the default stream explicitly changed the report.\n legacy: %s\n default: %s", base, got)
		}
		if got := reportJSON(t, split); got != base {
			t.Errorf("C-41: splitting one body of evidence into two labeled streams changed the report. Over criteria that have entries the multi-stream rule is C-33's existing merge, so nothing in the report can move.\n unsplit: %s\n split: %s", base, got)
		}

		// The label must survive the artifact, or nothing above is a
		// multi-stream test at all.
		if got := marshalResults(t, split); !strings.Contains(got, `"stream":"js"`) {
			t.Errorf("C-41: the stream label did not survive a parse and marshal, so an entry cannot carry one.\ngot: %s", got)
		}

		// Opaque, per C-41. These are the names the requests behind SSRB-103
		// use, and reserving them would make the foundation unusable for them.
		for _, label := range []string{"unit", "live", "mutation", "baseline"} {
			one := `{"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"` + label + `"}]}`
			if got := marshalResults(t, one); !strings.Contains(got, `"stream":"`+label+`"`) {
				t.Errorf("C-41: the label %q did not survive the round trip, and every non-empty string is valid", label)
			}
		}
	})
}

// @spec spec-coverage
// @ac AC-69
//
// C-42: a stream that ran and produced nothing is distinguishable from one
// that never ran. Both leave zero entries, so only the top-level block tells
// them apart. Two fixtures, because one file cannot be in both states.
func TestStreamMetadataDistinguishesRanFromNeverRan(t *testing.T) {
	t.Run("spec-coverage/AC-69 a stream that ran and found nothing is visible", func(t *testing.T) {
		// js ran, scanned 12 inputs, extracted none. It has no entries, which
		// is precisely what makes the metadata its only witness.
		const ranAndEmpty = `{
			"streams":[{"name":"go","scanned":40,"extracted":1},
			           {"name":"js","scanned":12,"extracted":0}],
			"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`
		// js never ran. Identical entries; the stream is absent from the block.
		const neverRan = `{
			"streams":[{"name":"go","scanned":40,"extracted":1}],
			"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`

		// Precondition. Neither fixture may hold a js entry, or the metadata
		// stops being the only thing that can report js and the case is gone.
		for name, body := range map[string]string{"ranAndEmpty": ranAndEmpty, "neverRan": neverRan} {
			if strings.Contains(body, `"stream":"js"`) {
				t.Fatalf("%s carries a js entry, so it cannot stand for a stream with no entries", name)
			}
		}

		// C-42's ordering half. The fixture below declares the streams in
		// descending name order on purpose: a parser that preserved input order
		// would pass an assertion made over an already-sorted fixture, and
		// preserving input order is exactly what an array built from a decoded
		// list does by default.
		const unsorted = `{
			"streams":[{"name":"js","scanned":12,"extracted":0},
			           {"name":"go","scanned":40,"extracted":1},
			           {"name":"e2e","scanned":3,"extracted":0}],
			"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`
		ordered := marshalResults(t, unsorted)
		// Read the streams array rather than searching the document. A
		// substring search would find "go" in a result entry's stream label
		// first, and this fixture has one, so it would be measuring the order
		// of top-level fields rather than the order of the array. Field order
		// is not something the spec binds.
		if got := streamNamesOf(t, ordered); len(got) != 3 {
			t.Errorf("C-42: the artifact declares %d stream(s), want 3, so ordering cannot be checked.\ngot: %s", len(got), ordered)
		} else if got[0] != "e2e" || got[1] != "go" || got[2] != "js" {
			t.Errorf("C-42: the streams array is %v, want [e2e go js]. Declared js, go, e2e, so the array keeps input order and two producers writing the same facts differ", got)
		}
		// Byte-identical across builds, per C-42. Two marshals of two separate
		// parses, not one value compared with itself.
		if again := marshalResults(t, unsorted); again != ordered {
			t.Errorf("C-42: two runs over one artifact produced different bytes, so a CI job diffing them sees churn that is not a change.\n first: %s\n second: %s", ordered, again)
		}

		ran, never := marshalResults(t, ranAndEmpty), marshalResults(t, neverRan)
		if ran == never {
			t.Errorf("C-42: a stream that ran and found nothing and one that never ran marshal to the same artifact, so a broken pipeline reads as a clean one.\n both: %s", ran)
		}
		if !strings.Contains(ran, `"streams"`) {
			t.Errorf("C-42: the results file carries no streams block.\ngot: %s", ran)
		}
		if !strings.Contains(ran, `"js"`) {
			t.Errorf("C-42: the js stream ran, scanned 12 inputs and extracted none, and does not appear in the artifact at all.\ngot: %s", ran)
		}
		if strings.Contains(never, `"js"`) {
			t.Errorf("C-42: a stream that never ran appears in the artifact.\ngot: %s", never)
		}
	})
}

// @spec spec-coverage
// @ac AC-70
//
// C-43: covered means at least one stream holds an entry AND every stream
// holding one reports passed. Asserted through Covered, because C-43 is a
// covered-rule contract rather than a lookup contract.
func TestCoveredRuleAcrossStreams(t *testing.T) {
	t.Run("spec-coverage/AC-70 the covered rule, in three cases", func(t *testing.T) {
		// AC-01 passed in two streams. AC-02 passed in one, failed in the other.
		const twoStreams = `{"results":[
			{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"js"},
			{"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"js"}]}`
		// No stream reports on either criterion of this spec.
		const unreported = `{"results":[
			{"spec_id":"other","ac_id":"AC-01","status":"passed","stream":"go"}]}`

		got := coveredByCriterion(t, twoStreams)
		if !got["AC-01"] {
			t.Error("C-43: a criterion passed in two streams is not covered. Both streams hold an entry and both report passed")
		}
		if got["AC-02"] {
			t.Error("C-43: a criterion passing in one stream and failing in another is covered. Every stream holding an entry must report passed")
		}

		// The clause that gets dropped. Written without it the rule is
		// vacuously true over an empty set and this criterion becomes covered.
		none := coveredByCriterion(t, unreported)
		if none["AC-01"] || none["AC-02"] {
			t.Errorf("C-43: a criterion no stream reported on is covered: %v. The first clause requires at least one stream to hold an entry, and without it the condition is vacuously true over an empty set", none)
		}

		// Differential precondition. If the covered and not-covered cases ever
		// answer alike, the assertions above agree for the wrong reason.
		if got["AC-01"] == got["AC-02"] {
			t.Fatalf("both criteria are covered=%v, so this test cannot distinguish the cases", got["AC-01"])
		}
	})
}

// stream_validation_test.go -- CLI integration tests for roadmap 3A5:
// streams-block consistency validation in the shared builder
// (spec-coverage 1.23.0 C-44/AC-72, spec-sync 2.3.0 AC-20).
//
// Everything goes through the CLI. The validator and its error type do not
// exist yet, so naming either would report a build failure, and a build failure
// says the work is unfinished rather than that the behavior is wrong. Exit code
// and emitted document are both observable from a subprocess run today.
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exitStreamValidationPlanned is the code C-44 takes from the evidence band.
// A literal rather than a reference, because the constant lands with the
// implementation and docs/EXIT_CODES.md carries no Stable row for it yet.
const exitStreamValidationPlanned = 20

type validationDoc struct {
	ResultsValidationErrors []struct {
		Kind        string `json:"kind"`
		Stream      string `json:"stream"`
		Message     string `json:"message"`
		StreamIndex *int   `json:"stream_index"`
		ResultIndex *int   `json:"result_index"`
	} `json:"results_validation_errors"`
}

// defaultStreamACs is the plain two-criterion block most cases use.
const defaultStreamACs = "    - id: AC-01\n      description: \"one\"\n      references_constraints: [\"C-01\"]\n" +
	"    - id: AC-02\n      description: \"two\"\n      references_constraints: [\"C-01\"]\n"

// noTestStreamACs adds a third criterion the test file never annotates, which
// is what rule 1 counts. A criterion with a marker and no result is a coverage
// question rather than a rule-1 one, so a fixture built that way leaves the
// gate silent and the coexistence case proves nothing.
const noTestStreamACs = defaultStreamACs +
	"    - id: AC-03\n      description: \"three\"\n      references_constraints: [\"C-01\"]\n"

// approvalGateStreamACs carries an unmet approval gate on the second criterion.
// approval_gate true with no approval_date is what C-26 counts, so this is the
// coexisting gate AC-72's approval-gate case needs.
const approvalGateStreamACs = "    - id: AC-01\n      description: \"one\"\n      references_constraints: [\"C-01\"]\n" +
	"    - id: AC-02\n      description: \"two\"\n      references_constraints: [\"C-01\"]\n      approval_gate: true\n"

// streamWorkspace writes a spec with two annotated criteria, a results file
// with whatever body the case needs, and a manifest.
func streamWorkspace(t *testing.T, results, settings string) string {
	t.Helper()
	return streamWorkspaceACs(t, results, settings, defaultStreamACs)
}

// streamWorkspaceACs is streamWorkspace with the criteria block chosen by the
// caller, so a case needing a coexisting gate can declare one.
func streamWorkspaceACs(t *testing.T, results, settings, acs string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, "s.spec.yaml",
		"spec:\n  id: s\n  version: \"1.0.0\"\n  status: approved\n  tier: 3\n"+
			"  context:\n    system: test\n"+
			"  objective:\n    summary: Two criteria for stream-validation fixtures.\n"+
			"  constraints:\n    - id: C-01\n      description: \"MUST hold\"\n"+
			"  acceptance_criteria:\n"+acs)
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package tests\n\nimport \"testing\"\n\n// @spec s\n// @ac AC-01\n// @ac AC-02\n" +
		"func TestS(t *testing.T) {\n\tt.Run(\"s/AC-01\", func(t *testing.T) {})\n\tt.Run(\"s/AC-02\", func(t *testing.T) {})\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "tests", "s_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	writeResultsJSON(t, dir, results)
	putManifest(t, dir, "system:\n  name: s\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n"+settings)
	return dir
}

const permissiveSettings = "  annotation:\n    permissive: true\n"

func runCoverageJSON(t *testing.T, dir string) (validationDoc, string, int) {
	t.Helper()
	stdout, stderr, code := runCLISplit(t, dir, "coverage", "--json")
	var doc validationDoc
	if stdout != "" {
		// C-10 requires a document in every state, so a decode failure here is
		// itself a finding rather than a reason to skip the assertions.
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Errorf("C-10: coverage --json emitted a document that does not decode: %v\nstdout:\n%s", err, stdout)
		}
	}
	return doc, stderr, code
}

// @spec spec-coverage
// @ac AC-72
//
// C-44: the five consistency rules, what each refusal carries, and what a
// legal artifact is spared.
func TestStreamValidationRules(t *testing.T) {
	t.Run("spec-coverage/AC-72 an inconsistent streams block is refused", func(t *testing.T) {
		twoPassed := `{"spec_id":"s","ac_id":"AC-01","status":"passed"},{"spec_id":"s","ac_id":"AC-02","status":"passed"}`

		cases := []struct {
			name    string
			results string
			refuse  bool
			kind    string
		}{
			{"undeclared stream", `{"streams":[{"name":"go","scanned":2,"extracted":2}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"js"}]}`, true, "undeclared_stream"},
			{"duplicate name", `{"streams":[{"name":"go","scanned":1,"extracted":1},{"name":"go","scanned":1,"extracted":1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "duplicate_stream"},
			{"empty name", `{"streams":[{"name":"","scanned":1,"extracted":1}],
				"results":[` + twoPassed + `]}`, true, "empty_stream_name"},
			{"negative scanned", `{"streams":[{"name":"go","scanned":-1,"extracted":1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "negative_count"},
			{"negative extracted", `{"streams":[{"name":"go","scanned":1,"extracted":-1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "negative_count"},
			{"negative silent packages", `{"streams":[{"name":"go","scanned":1,"extracted":1,"zero_test_event_packages":-1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "negative_count"},
			{"extracted below entries", `{"streams":[{"name":"go","scanned":2,"extracted":1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
				           {"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"go"}]}`, true, "extracted_below_entries"},
			// A declared count of zero beside one entry. Separated from the
			// case above because zero is the value an implementation is most
			// likely to special-case: skipping streams whose counts are zero
			// looks like a cheap win and silently drops this rule for them.
			{"extracted zero with an entry", `{"streams":[{"name":"go","scanned":1,"extracted":0}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "extracted_below_entries"},
			{"empty block beside a label", `{"streams":[],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, true, "undeclared_stream"},
			{"default declared and undercounted", `{"streams":[{"name":"default","scanned":2,"extracted":1}],
				"results":[` + twoPassed + `]}`, true, "extracted_below_entries"},

			{"extracted above entries", `{"streams":[{"name":"go","scanned":9,"extracted":5}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, false, ""},
			{"no block at all", `{"results":[` + twoPassed + `]}`, false, ""},
			{"unlabeled beside declared", `{"streams":[{"name":"go","scanned":2,"extracted":1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
				           {"spec_id":"s","ac_id":"AC-02","status":"passed"}]}`, false, ""},
			{"explicit default undeclared", `{"streams":[{"name":"go","scanned":2,"extracted":1}],
				"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
				           {"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"default"}]}`, false, ""},
		}

		for _, c := range cases {
			dir := streamWorkspace(t, c.results, permissiveSettings)
			doc, stderr, code := runCoverageJSON(t, dir)

			if !c.refuse {
				// Exit 0 rather than "anything but 20". These fixtures are
				// legal in every other way too, so a run that refused them for
				// some unrelated reason would satisfy a not-20 check while the
				// artifact was still being rejected.
				if code != 0 {
					t.Errorf("C-44 (%s): a legal artifact exited %d, want 0.\nstderr:\n%s", c.name, code, stderr)
				}
				if len(doc.ResultsValidationErrors) != 0 {
					t.Errorf("C-44 (%s): a legal artifact reported %d validation error(s). The array is present exactly when validation found something", c.name, len(doc.ResultsValidationErrors))
				}
				continue
			}

			if code != exitStreamValidationPlanned {
				t.Errorf("C-44 (%s): exited %d, want %d. A violation standing alone exits in the evidence band.\nstderr:\n%s",
					c.name, code, exitStreamValidationPlanned, stderr)
			}
			if len(doc.ResultsValidationErrors) == 0 {
				t.Errorf("C-44 (%s): the refusal reported no validation errors in the document. C-10 requires a report in every state and the refusal appears inside it", c.name)
				continue
			}
			if got := doc.ResultsValidationErrors[0].Kind; got != c.kind {
				t.Errorf("C-44 (%s): first violation is kind %q, want %q", c.name, got, c.kind)
			}
			// Exactly one coordinate, never neither and never both.
			for i, e := range doc.ResultsValidationErrors {
				hasStream, hasResult := e.StreamIndex != nil, e.ResultIndex != nil
				if hasStream == hasResult {
					t.Errorf("C-44 (%s): violation %d carries %v coordinates, want exactly one. undeclared_stream carries result_index and every other kind carries stream_index",
						c.name, i, map[bool]string{true: "two", false: "no"}[hasStream])
				}
				if (e.Kind == "undeclared_stream") != hasResult {
					t.Errorf("C-44 (%s): violation %d is kind %q with result_index present=%v, which is the wrong coordinate for that kind", c.name, i, e.Kind, hasResult)
				}
				// C-44(c): a violation must identify the stream it concerns.
				// Without this, every kind below could emit empty strings and
				// still satisfy the kind and coordinate checks above.
				if e.Message == "" {
					t.Errorf("C-44(c) (%s): violation %d of kind %q carries an empty message, so the refusal names no cause", c.name, i, e.Kind)
				}
				if e.Kind == "empty_stream_name" {
					// The one kind with no name to print. It identifies the
					// row by position instead, and says the name is empty.
					if e.Stream != "" {
						t.Errorf("C-44(c) (%s): violation %d is empty_stream_name carrying stream %q, want the empty string", c.name, i, e.Stream)
					}
					if !strings.Contains(e.Message, "empty") {
						t.Errorf("C-44(c) (%s): violation %d does not say the name is empty.\ngot: %s", c.name, i, e.Message)
					}
				} else if e.Stream == "" {
					t.Errorf("C-44(c) (%s): violation %d of kind %q names no stream. Only empty_stream_name has no name to print", c.name, i, e.Kind)
				}
			}
		}
	})

	t.Run("spec-coverage/AC-72 several negative fields give one violation naming each", func(t *testing.T) {
		dir := streamWorkspace(t, `{"streams":[{"name":"go","scanned":-1,"extracted":-2,"zero_test_event_packages":-3}],
			"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"}]}`, permissiveSettings)
		doc, _, _ := runCoverageJSON(t, dir)

		negatives := 0
		var msg string
		for _, e := range doc.ResultsValidationErrors {
			if e.Kind == "negative_count" {
				negatives++
				msg = e.Message
			}
			if e.Kind == "extracted_below_entries" {
				t.Error("C-44: a negative extracted also reported extracted_below_entries. The second is derived from the first, and reporting both tells an operator to fix a consequence beside its cause")
			}
		}
		if negatives != 1 {
			t.Errorf("C-44: %d negative_count violations for one row, want 1. Three would share a kind, a stream and a stream_index, which the total order cannot break", negatives)
		}
		// Fixed order, not merely present. C-44(d) names the order because the
		// three fields share a kind, a stream and a stream_index, so the
		// message is the only thing distinguishing them and a reader comparing
		// two runs needs it stable.
		prev := -1
		for _, field := range []string{"scanned", "extracted", "zero_test_event_packages"} {
			at := strings.Index(msg, field)
			if at < 0 {
				t.Errorf("C-44(d): the message does not name %q. One violation per row names every negative field.\ngot: %s", field, msg)
				continue
			}
			if at < prev {
				t.Errorf("C-44(d): %q appears before the field that precedes it. The order is scanned, extracted, zero_test_event_packages.\ngot: %s", field, msg)
			}
			prev = at
		}
	})
}

// @spec spec-coverage
// @ac AC-72
//
// C-44: every violation is collected, the order is the exact sequence stated,
// and an index points into the file as written rather than into the array
// after C-42's ascending-by-name sort.
func TestStreamValidationOrderAndIndexes(t *testing.T) {
	t.Run("spec-coverage/AC-72 violations are ordered and indexed against the file", func(t *testing.T) {
		// streams is declared descending, so the sort moves every row. The
		// empty-name rows sit at file positions 1 and 2. Entries name alpha at
		// result positions 0 and 3 and zeta at 1, all undeclared.
		//
		// zz declares extracted 1 and carries exactly one entry. An earlier
		// draft declared 0, which C-44(e) refuses, so the fixture produced a
		// sixth violation and the length check below failed on a rule this
		// case does not test.
		dir := streamWorkspace(t, `{
			"streams":[{"name":"zz","scanned":1,"extracted":1},
			           {"name":"","scanned":1,"extracted":0},
			           {"name":"","scanned":1,"extracted":0}],
			"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"alpha"},
			           {"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"zeta"},
			           {"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"zz"},
			           {"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"alpha"}]}`, permissiveSettings)
		doc, _, _ := runCoverageJSON(t, dir)

		type tup struct {
			kind, stream string
			si, ri       int
		}
		want := []tup{
			{"empty_stream_name", "", 1, -1},
			{"empty_stream_name", "", 2, -1},
			{"undeclared_stream", "alpha", -1, 0},
			{"undeclared_stream", "alpha", -1, 3},
			{"undeclared_stream", "zeta", -1, 1},
		}
		if len(doc.ResultsValidationErrors) != len(want) {
			t.Fatalf("C-44: %d violations, want %d. Every violation is collected, not the first", len(doc.ResultsValidationErrors), len(want))
		}
		for i, w := range want {
			e := doc.ResultsValidationErrors[i]
			si, ri := -1, -1
			if e.StreamIndex != nil {
				si = *e.StreamIndex
			}
			if e.ResultIndex != nil {
				ri = *e.ResultIndex
			}
			if e.Kind != w.kind || e.Stream != w.stream || si != w.si || ri != w.ri {
				t.Errorf("C-44: violation %d is (%s, %s, si=%d, ri=%d), want (%s, %s, si=%d, ri=%d). The order is kind, then stream, then stream_index, then result_index, and an index points into the file as written rather than into the array after C-42's sort",
					i, e.Kind, e.Stream, si, ri, w.kind, w.stream, w.si, w.ri)
			}
		}

		// Two empty names are two empty names and not also a repeat.
		//
		// C-44(b) reads "MUST NOT be empty and MUST NOT repeat within the
		// block" as one sentence over a stream name, and two rows named ""
		// do repeat that value. Read literally it would add a sixth
		// violation here, which AC-72's pinned five-element sequence forbids,
		// so the literal reading makes the criterion unsatisfiable. C-44(c)
		// settles it: "an empty one is not a label". An empty name is refused
		// as empty and never enters the repeat comparison.
		//
		// Asserted rather than left implicit, because the length check above
		// would report the same failure for a dozen unrelated reasons.
		empties, duplicates := 0, 0
		for _, e := range doc.ResultsValidationErrors {
			switch e.Kind {
			case "empty_stream_name":
				empties++
			case "duplicate_stream":
				duplicates++
			}
		}
		if empties != 2 {
			t.Errorf("C-44(b): %d empty_stream_name violations, want 2. Each empty row is refused on its own and carries its own array position", empties)
		}
		if duplicates != 0 {
			t.Errorf("C-44(c): two empty names also reported %d duplicate_stream violation(s). An empty one is not a label, so it is refused as empty and never enters the repeat comparison", duplicates)
		}

		// Two runs agree byte for byte, so the order is not incidental.
		second, _, _ := runCoverageJSON(t, dir)
		a, _ := json.Marshal(doc.ResultsValidationErrors)
		b, _ := json.Marshal(second.ResultsValidationErrors)
		if string(a) != string(b) {
			t.Errorf("C-44: two runs over one artifact ordered the violations differently.\n first: %s\n second: %s", a, b)
		}
	})
}

// @spec spec-coverage
// @ac AC-72
//
// C-44: the band's code is what a workspace exits with standing alone and
// never what it exits with beside a gate that shipped earlier. The threshold
// case carries the weight: it is the last shipped gate, so a code inserted
// anywhere before it passes every other case here.
func TestStreamValidationPrecedence(t *testing.T) {
	t.Run("spec-coverage/AC-72 an earlier gate keeps its code", func(t *testing.T) {
		// Every fixture declares an empty block beside a labeled entry, which
		// C-44 refuses. What changes across cases is which gate is failing
		// beside it. The threshold case carries the weight: the threshold is
		// the last shipped gate, so a code inserted anywhere before it passes
		// every other case here and fails only this one.
		bothPassed := `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"go"}]}`
		oneFailed := `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"failed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-02","status":"passed","stream":"go"}]}`
		bothFailed := `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"failed","stream":"go"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"go"}]}`

		cases := []struct {
			name     string
			settings string
			results  string
			acs      string
			want     int
		}{
			// Nothing else is wrong, so the band's own code is the answer.
			{"standalone", permissiveSettings, bothPassed, defaultStreamACs, exitStreamValidationPlanned},
			// The next two are one condition under both severities. AC-03 has
			// no marker, so rule 1 has something to say either way. Under
			// permissive it warns and the band still wins; at false it fails
			// and keeps code 2.
			{"alongside a permissive warning", permissiveSettings, bothPassed, noTestStreamACs, exitStreamValidationPlanned},
			{"alongside rule 1", "  annotation:\n    permissive: false\n", bothPassed, noTestStreamACs, 2},
			{"alongside zero-tolerance", "  strictness: zero-tolerance\n", oneFailed, defaultStreamACs, 2},
			{"alongside an approval gate", "  strictness: zero-tolerance\n", bothPassed, approvalGateStreamACs, 3},
			// Tier 3 defaults to 50 percent and this workspace passes none.
			{"alongside the threshold", "  strictness: threshold\n", bothFailed, defaultStreamACs, 1},
		}
		for _, c := range cases {
			dir := streamWorkspaceACs(t, c.results, c.settings, c.acs)
			doc, stderr, code := runCoverageJSON(t, dir)
			if code != c.want {
				t.Errorf("C-44 (%s): exited %d, want %d. A gate that shipped before stream validation keeps its own code.\nstderr:\n%s", c.name, code, c.want, stderr)
			}
			if len(doc.ResultsValidationErrors) == 0 {
				t.Errorf("C-44 (%s): the stream violation went unreported. Whichever gate chooses the code, the violations are still reported", c.name)
			}
		}
	})
}

// @spec spec-sync
// @ac AC-20
//
// C-11 and C-44: a refusal propagates to sync, in both output modes and under
// every strictness model. sync reaches the builder by a different constructor
// under annotation, so a rule added to the builder can reach two modes of three
// while a surface count still reads four.
func TestStreamValidationPropagatesToSync(t *testing.T) {
	t.Run("spec-sync/AC-20 sync returns what coverage returns", func(t *testing.T) {
		// The stream name is deliberately unusual. Asserting on "go" would
		// pass on any stderr containing "going" or "algorithm", so the
		// assertion would hold while the refusal named nothing.
		bad := `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"passed","stream":"e2e-webkit"},
			{"spec_id":"s","ac_id":"AC-02","status":"passed"}]}`
		// Below the tier threshold and inconsistent at the same time. This is
		// the case that separates one shared verdict from two, because the
		// threshold is the gate ordered last: sync builds its verdict with the
		// threshold input and the exit closure builds a second one without it,
		// so the two agree on every workspace where the threshold is silent.
		belowThreshold := `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"failed","stream":"e2e-webkit"},
			{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"e2e-webkit"}]}`
		modes := map[string]string{
			"annotation":                permissiveSettings,
			"threshold":                 "  strictness: threshold\n",
			"zero-tolerance":            "  strictness: zero-tolerance\n",
			"threshold, below the tier": "  strictness: threshold\n",
		}
		bodies := map[string]string{"threshold, below the tier": belowThreshold}

		// AC-20 names four surfaces, not two. coverage --json is one of them,
		// and C-09's --json clause is scoped to the ladder, so nothing else
		// binds it for this gate.
		surfaces := [][]string{{"coverage"}, {"coverage", "--json"}, {"sync"}, {"sync", "--json"}}

		for mode, settings := range modes {
			body := bad
			if b, ok := bodies[mode]; ok {
				body = b
			}
			// Every fixture labels its offending entry go, so the stream the
			// refusal names is known rather than merely non-empty. A generic
			// "stream validation failed" satisfies a substring check on the
			// word stream while naming nothing.
			const wantStream = "e2e-webkit"

			baseDir := streamWorkspace(t, body, settings)
			_, baseErr, baseCode := runCLISplit(t, baseDir, "coverage")

			for _, surface := range surfaces {
				dir := streamWorkspace(t, body, settings)
				_, stderr, code := runCLISplit(t, dir, surface...)
				label := mode + "/" + strings.Join(surface, " ")
				if code != baseCode {
					t.Errorf("AC-20 (%s): coverage exited %d and this exited %d on the same workspace. C-11 requires all four surfaces to agree, and sync is the CI entry point.\n coverage stderr:\n%s\n stderr:\n%s",
						label, baseCode, code, baseErr, stderr)
				}
				if !strings.Contains(stderr, wantStream) {
					t.Errorf("AC-20 (%s): stderr never names stream %q, so the surfaces cannot be naming the same one.\nstderr:\n%s", label, wantStream, stderr)
				}
			}
		}
	})
}

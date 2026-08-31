// coverage_json_stop_reason_test.go -- every return from `coverage --json`
// emits a document, spec-coverage 1.26.0 C-10 and AC-74, bugs/SP-SP-032.
//
// C-10 says the document is emitted always, and names the boundary: once Cobra
// has entered RunE, every JSON-mode return emits one. Seven early returns
// wrote nothing.
//
// The scenarios come from coverageScenarios(), the same table the pinned text
// baseline replays, so the two cannot disagree about which branches exist.
//
// @spec spec-coverage
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// stopReasonKinds maps each scenario to the kind its document must carry, or
// "" for a run that reaches the report.
//
// Seven refusals collapse to four kinds. The mapping is written out rather
// than derived, because deriving it from the scenario name would restate the
// implementation's own grouping and agree with it by construction.
var stopReasonKinds = map[string]string{
	// Three ways to be an invalid flag or flag combination.
	"refusal_scope_without_strict":      "invalid_flag",
	"refusal_invalid_strictness":        "invalid_flag",
	"refusal_strict_against_annotation": "invalid_flag",
	"refusal_manifest":                  "manifest_error",
	"refusal_unknown_domain":            "unknown_scope",
	// Two preconditions the run needs and does not have.
	"refusal_zero_tolerance_no_annotations": "unmet_precondition",
	"refusal_missing_results":               "unmet_precondition",

	// The control and the four gates all reach the report. A stop_reason here
	// would say the run refused when it measured, and a consumer reading it
	// would hide a real result.
	"control_success":              "",
	"gate_1_below_threshold":       "",
	"gate_2_annotation_no_test":    "",
	"gate_3_approval":              "",
	"gate_20_inconsistent_streams": "",
}

// jsonReachFromDocument holds the scenarios whose text cause is a rendered
// summary line. JSON mode does not print it, so reach is proven from the
// document the run produced rather than from prose it never emits.
var jsonReachFromDocument = map[string]func(jsonCoverageDoc) error{
	"control_success": func(d jsonCoverageDoc) error {
		if len(d.Entries) == 0 {
			return errors.New("the run measured no specs, so it did not reach the report")
		}
		for _, e := range d.Entries {
			if !e.PassesThreshold {
				return fmt.Errorf("spec %s is below its threshold, so this is not the passing control", e.SpecID)
			}
		}
		return nil
	},
	"gate_1_below_threshold": func(d jsonCoverageDoc) error {
		for _, e := range d.Entries {
			if !e.PassesThreshold {
				return nil
			}
		}
		return errors.New("no entry is below its threshold, so the threshold gate was not the branch taken")
	},
}

type jsonStopReason struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type jsonCoverageDoc struct {
	Entries []struct {
		SpecID          string `json:"spec_id"`
		PassesThreshold bool   `json:"passes_threshold"`
	} `json:"entries"`
	Summary struct {
		TotalSpecs int `json:"total_specs"`
		Passing    int `json:"passing"`
		Failing    int `json:"failing"`
	} `json:"summary"`
	ParseErrors []map[string]any `json:"parse_errors"`
	StopReason  *jsonStopReason  `json:"stop_reason"`
}

// @spec spec-coverage
// @ac AC-74
//
// C-10: every JSON-mode return emits a complete document, with stop_reason on
// a refusal and absent on a run that reached the report.
func TestCoverageJSONEmitsADocumentOnEveryReturn(t *testing.T) {
	scenarios := coverageScenarios()

	// Every scenario is mapped. Without this, a branch added to the table
	// later would be exercised with no expectation and pass silently.
	if len(stopReasonKinds) != len(scenarios) {
		t.Fatalf("AC-74: %d scenarios and %d kind expectations. Every branch needs one, or a new branch passes with nothing asserted about it", len(scenarios), len(stopReasonKinds))
	}
	validKinds := map[string]bool{
		"manifest_error": true, "invalid_flag": true,
		"unknown_scope": true, "unmet_precondition": true,
	}
	for _, sc := range scenarios {
		want, ok := stopReasonKinds[sc.name]
		if !ok {
			t.Fatalf("AC-74: scenario %s has no kind expectation", sc.name)
		}
		if want != "" && !validKinds[want] {
			t.Fatalf("AC-74: scenario %s expects kind %q, which C-10 does not define", sc.name, want)
		}
	}
	// All four kinds are actually exercised. Seven refusals collapsing to
	// three used kinds would leave one unreachable and untested.
	used := map[string]bool{}
	for _, k := range stopReasonKinds {
		if k != "" {
			used[k] = true
		}
	}
	if len(used) != 4 {
		names := make([]string, 0, len(used))
		for k := range used {
			names = append(names, k)
		}
		sort.Strings(names)
		t.Fatalf("AC-74: the scenarios exercise %d of the 4 kinds C-10 defines: %v", len(used), names)
	}

	for _, sc := range scenarios {
		t.Run("spec-coverage/AC-74 "+sc.name, func(t *testing.T) {
			dir := t.TempDir()
			sc.build(func(rel, body string) {
				full := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			})

			args := append([]string{"coverage"}, sc.args...)
			args = append(args, "--json")
			stdout, stderr, code := runCLISplit(t, dir, args...)

			// The scenario reaches its own branch first. A fixture that trips
			// an earlier gate would assert this branch's contract against a
			// different branch's output.
			//
			// Two scenarios prove reach differently here than in the text
			// baseline, and that is not a weakening. Their text cause is a
			// rendered summary line, which JSON mode correctly does not
			// print, so reach is proven from the document instead: entries
			// the run actually measured, and for the threshold gate an entry
			// that failed its threshold.
			if _, textOnly := jsonReachFromDocument[sc.name]; !textOnly {
				if !strings.Contains(stdout+stderr, sc.cause) {
					t.Fatalf("AC-74: %s did not reach its branch; output should contain %q.\nstdout:\n%s\nstderr:\n%s", sc.name, sc.cause, stdout, stderr)
				}
			}

			// The exit code is unchanged by C-10, which governs stdout only.
			// Taken from the pinned text golden rather than restated here, so
			// the two surfaces cannot drift and the gate codes 1, 2, 3 and 20
			// stay bound to one recorded set.
			wantExit := exitFromGolden(t, sc.name)
			if code != wantExit {
				t.Errorf("AC-74: %s exited %d under --json, want %d, the code pinned for text mode. C-10 governs what stdout carries and leaves the verdict alone", sc.name, code, wantExit)
			}

			if strings.TrimSpace(stdout) == "" {
				t.Fatalf("AC-74: %s wrote nothing to stdout. C-10 requires a document from every return once RunE has been entered.\nstderr:\n%s", sc.name, stderr)
			}
			var doc jsonCoverageDoc
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("AC-74: %s stdout does not parse as one document: %v\nstdout:\n%s", sc.name, err, stdout)
			}

			if check, ok := jsonReachFromDocument[sc.name]; ok {
				if err := check(doc); err != nil {
					t.Fatalf("AC-74: %s did not reach its branch: %v\nstdout:\n%s", sc.name, err, stdout)
				}
			}

			want := stopReasonKinds[sc.name]
			if want == "" {
				// A run that reached the report. stop_reason must be absent,
				// not merely empty: a consumer keys on its presence.
				if doc.StopReason != nil {
					t.Errorf("AC-74: %s reached the report and still carries stop_reason %+v. A consumer reading it would hide a real result", sc.name, *doc.StopReason)
				}
				if len(doc.Entries) == 0 {
					t.Errorf("AC-74: %s reached the report and carries no entries, so the refusal assertions elsewhere could be met by a command that measures nothing", sc.name)
				}
				return
			}

			if doc.StopReason == nil {
				t.Fatalf("AC-74: %s refused and carries no stop_reason. Without it the document is indistinguishable from a clean empty workspace.\nstdout:\n%s", sc.name, stdout)
			}
			if doc.StopReason.Kind != want {
				t.Errorf("AC-74: %s carries kind %q, want %q", sc.name, doc.StopReason.Kind, want)
			}
			if strings.TrimSpace(doc.StopReason.Message) == "" {
				t.Errorf("AC-74: %s carries an empty stop_reason message, so the document names a kind and explains nothing", sc.name)
			}
			// The placeholders. C-10 requires them present and meaningless,
			// guarded by stop_reason, rather than fabricated measurements.
			if len(doc.Entries) != 0 {
				t.Errorf("AC-74: %s refused and carries %d entries. The run measured nothing, so entries is a placeholder and must be empty", sc.name, len(doc.Entries))
			}
			if doc.Summary.TotalSpecs != 0 || doc.Summary.Passing != 0 || doc.Summary.Failing != 0 {
				t.Errorf("AC-74: %s refused and carries a non-zero summary %+v. A summary the run never computed reads as a measurement", sc.name, doc.Summary)
			}
			// The reason is its own field. Overloading parse_errors would
			// report a rejected manifest as a spec that failed to parse.
			if len(doc.ParseErrors) != 0 {
				t.Errorf("AC-74: %s put %d entries in parse_errors. A refusal is not a parse failure, and a consumer grouping by parse-error pattern would group them together", sc.name, len(doc.ParseErrors))
			}
		})
	}
}

// exitFromGolden reads the exit code the pinned text baseline recorded for a
// scenario, so the JSON criterion and the text baseline cannot disagree about
// what a branch exits with.
func exitFromGolden(t *testing.T, name string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(baselineDir, name+".txt"))
	if err != nil {
		t.Fatalf("AC-74: no pinned baseline for %s: %v", name, err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	n, err := strconv.Atoi(strings.TrimPrefix(first, "exit="))
	if err != nil {
		t.Fatalf("AC-74: baseline for %s does not start with an exit line: %q", name, first)
	}
	return n
}

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
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
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

// jsonCoverageDoc is used ONLY by the reach predicates, which ask what the run
// measured. The contract assertions read raw bytes instead, because a struct
// cannot tell a missing key from a null or an empty array.
type jsonCoverageDoc struct {
	Entries []struct {
		SpecID          string `json:"spec_id"`
		PassesThreshold bool   `json:"passes_threshold"`
	} `json:"entries"`
}

// summaryFields is every key CoverageSummary serializes. A refusal must zero
// all of them, not the three a reader happens to check.
var summaryFields = []string{
	"total_specs", "fully_covered", "partially_covered",
	"uncovered", "passing", "failing",
}

// rawDoc decodes one level into raw messages, so key PRESENCE survives.
//
// Decoding into a struct cannot enforce this contract: a missing key, an
// explicit null and an empty array all become a nil slice, and a missing
// summary becomes a zero summary that reads as a compliant refusal. The
// contract is about what the bytes carry, so the bytes are what is read.
func rawDoc(t *testing.T, stdout string) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("AC-74: stdout does not decode as a JSON object: %v\nstdout:\n%s", err, stdout)
	}
	return raw
}

// requireEmptyArray fails unless key is present and holds [] exactly, not null
// and not absent.
func requireEmptyArray(t *testing.T, raw map[string]json.RawMessage, key, scenario string) {
	t.Helper()
	v, ok := raw[key]
	if !ok {
		t.Errorf("AC-74: %s omits %q. C-10 keeps it required and present, carrying an empty list as a placeholder", scenario, key)
		return
	}
	s := strings.TrimSpace(string(v))
	if s == "null" {
		t.Errorf("AC-74: %s emits %q as null. Null and [] are different bytes to a consumer, and the placeholder is the empty list", scenario, key)
		return
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(v, &arr); err != nil {
		t.Errorf("AC-74: %s emits %q as %s, which is not an array", scenario, key, s)
		return
	}
	if len(arr) != 0 {
		t.Errorf("AC-74: %s refused and carries %d element(s) in %q. The run measured nothing", scenario, len(arr), key)
	}
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
			raw := rawDoc(t, stdout)
			var doc jsonCoverageDoc
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("AC-74: %s stdout does not parse as one document: %v\nstdout:\n%s", sc.name, err, stdout)
			}
			// Required keys, by presence in the bytes rather than by a decoded
			// zero value. entries and summary stay required in every state.
			for _, k := range []string{"entries", "summary"} {
				if _, ok := raw[k]; !ok {
					t.Errorf("AC-74: %s omits the required key %q", sc.name, k)
				}
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
				// Absent from the BYTES, not merely nil after decoding. An
				// explicit null decodes to nil too, and a consumer keying on
				// presence would see a refusal where the run measured.
				if v, ok := raw["stop_reason"]; ok {
					t.Errorf("AC-74: %s reached the report and its document carries a stop_reason key (%s). A consumer keying on presence would hide a real result", sc.name, string(v))
				}
				if len(doc.Entries) == 0 {
					t.Errorf("AC-74: %s reached the report and carries no entries, so the refusal assertions elsewhere could be met by a command that measures nothing", sc.name)
				}
				return
			}

			// Presence in the bytes, then shape. A struct decode would report
			// an absent key and an explicit null identically.
			rawReason, present := raw["stop_reason"]
			if !present || strings.TrimSpace(string(rawReason)) == "null" {
				t.Fatalf("AC-74: %s refused and carries no stop_reason. Without it the document is indistinguishable from a clean empty workspace.\nstdout:\n%s", sc.name, stdout)
			}
			var reason struct {
				Kind    string `json:"kind"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rawReason, &reason); err != nil {
				t.Fatalf("AC-74: %s stop_reason does not decode: %v", sc.name, err)
			}
			if reason.Kind != want {
				t.Errorf("AC-74: %s carries kind %q, want %q", sc.name, reason.Kind, want)
			}
			if strings.TrimSpace(reason.Message) == "" {
				t.Errorf("AC-74: %s carries an empty stop_reason message, so the document names a kind and explains nothing", sc.name)
			}
			// stop_reason carries exactly kind and message. An extra member
			// is a second channel a consumer may come to depend on, and C-10
			// fixes the shape rather than a minimum.
			var reasonKeys map[string]json.RawMessage
			if err := json.Unmarshal(raw["stop_reason"], &reasonKeys); err != nil {
				t.Fatalf("AC-74: %s stop_reason is not an object: %v", sc.name, err)
			}
			var got []string
			for k := range reasonKeys {
				got = append(got, k)
			}
			sort.Strings(got)
			if want := []string{"kind", "message"}; !reflect.DeepEqual(got, want) {
				t.Errorf("AC-74: %s stop_reason carries members %v, want exactly %v", sc.name, got, want)
			}

			// The placeholders. C-10 requires them present and meaningless,
			// guarded by stop_reason, rather than fabricated measurements.
			requireEmptyArray(t, raw, "entries", sc.name)
			// All six summary fields, not the three a reader happens to check.
			var summary map[string]int
			if err := json.Unmarshal(raw["summary"], &summary); err != nil {
				t.Errorf("AC-74: %s summary is not an object of numbers: %v", sc.name, err)
			} else {
				for _, f := range summaryFields {
					v, ok := summary[f]
					if !ok {
						t.Errorf("AC-74: %s summary omits %q", sc.name, f)
						continue
					}
					if v != 0 {
						t.Errorf("AC-74: %s refused and its summary reports %q = %d. A summary the run never computed reads as a measurement", sc.name, f, v)
					}
				}
			}
			// The reason is its own field. Overloading parse_errors would
			// report a rejected manifest as a spec that failed to parse.
			if v, ok := raw["parse_errors"]; ok && strings.TrimSpace(string(v)) != "null" {
				var pe []json.RawMessage
				_ = json.Unmarshal(v, &pe)
				if len(pe) != 0 {
					t.Errorf("AC-74: %s put %d entries in parse_errors. A refusal is not a parse failure, and a consumer grouping by parse-error pattern would group them together", sc.name, len(pe))
				}
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

// @spec spec-coverage
// @ac AC-74
//
// C-10's ownership half, asserted structurally because no runtime observation
// can see it. An encoder per early return produces identical output for every
// input above while restoring the condition that caused the defect: an exit
// with no emitter is one edit away, and the next one added is forgotten the
// same way.
//
// The owner is discovered, not named, for the reason spec-check AC-46 records:
// requiring a particular function name would fail a rename that preserved the
// property, which is private policy rather than the rule.
func TestCoverageRendersThroughOneDiscoveredOwner(t *testing.T) {
	t.Run("spec-coverage/AC-74 one discovered function owns encoding, and coverageCmd routes to it", func(t *testing.T) {
		fset := token.NewFileSet()
		files := parseProductionFiles(t, fset, ".")

		encodes := func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewEncoder" {
				return false
			}
			pkg, ok := sel.X.(*ast.Ident)
			return ok && pkg.Name == "json"
		}
		bareErrSilent := func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return false
			}
			id, ok := ret.Results[0].(*ast.Ident)
			return ok && id.Name == "errSilent"
		}
		calleesOf := func(fn string) map[string]int {
			out := map[string]int{}
			for _, f := range files {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok || fd.Name.Name != fn {
						continue
					}
					ast.Inspect(fd, func(n ast.Node) bool {
						if ce, ok := n.(*ast.CallExpr); ok {
							if id, ok := ce.Fun.(*ast.Ident); ok {
								out[id.Name]++
							}
						}
						return true
					})
				}
			}
			return out
		}
		sitesIn := func(fn string, want func(ast.Node) bool) []string {
			var out []string
			for _, s := range funcSitesMatching(fset, files, want) {
				if strings.HasPrefix(s, fn+":") {
					out = append(out, s)
				}
			}
			return out
		}

		if len(funcSitesMatching(fset, files, encodes)) == 0 {
			t.Fatalf("AC-74: no json.NewEncoder call found in any production file, so this matcher is wrong and every count below is meaningless")
		}

		if got := sitesIn("coverageCmd", encodes); len(got) != 0 {
			t.Errorf("AC-74: coverageCmd encodes a document itself at %v. Encoding belongs to one owner; a command that encodes is a second renderer whatever it is named", got)
		}
		if got := sitesIn("coverageCmd", bareErrSilent); len(got) != 0 {
			t.Errorf("AC-74: coverageCmd returns errSilent directly at %d site(s): %v. Each is an exit that writes no document, which is the defect C-10 names", len(got), got)
		}

		callees := calleesOf("coverageCmd")
		if len(callees) == 0 {
			t.Fatalf("AC-74: coverageCmd appears to call nothing, so the owner cannot be discovered and every claim here is vacuous")
		}
		var owners []string
		for name := range callees {
			if len(sitesIn(name, encodes)) > 0 {
				owners = append(owners, name)
			}
		}
		sort.Strings(owners)
		if len(owners) != 1 {
			t.Fatalf("AC-74: %d of coverageCmd's callees encode a document, want exactly 1. Found: %v", len(owners), owners)
		}
		if got := len(sitesIn(owners[0], encodes)); got != 1 {
			t.Errorf("AC-74: the owner %s encodes at %d site(s), want exactly 1", owners[0], got)
		}
		// Eight returns exist today: seven refusals and the ordinary path.
		if n := callees[owners[0]]; n < 8 {
			t.Errorf("AC-74: coverageCmd routes to its render owner %s at %d site(s), want at least 8: seven refusals and the ordinary path", owners[0], n)
		}
	})
}

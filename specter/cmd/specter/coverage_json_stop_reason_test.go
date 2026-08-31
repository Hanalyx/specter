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
	"bytes"
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

	"github.com/Hanalyx/specter/internal/coverage"
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
		// EVERY value-returning return in the command resolves to the owner.
		//
		// The bare-errSilent check above is not enough alone: a new exit
		// written as `return errors.New("coverage stopped")` is not bare
		// errSilent, leaves the routed sites intact, and bypasses the
		// renderer. A count of routed calls cannot see it either, because the
		// count does not change. Only the returns can.
		//
		// Nested closures are excluded. renderText returns nothing and is
		// invoked by the owner, so its returns are not exits from the command.
		routers := map[string]bool{owners[0]: true}
		for name := range callees {
			for _, s := range funcSitesMatching(fset, files, func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return false
				}
				id, ok := ce.Fun.(*ast.Ident)
				return ok && id.Name == owners[0]
			}) {
				if strings.HasPrefix(s, name+":") {
					routers[name] = true
				}
			}
		}

		var offending []string
		returns := 0
		for _, f := range files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Name.Name != "coverageCmd" {
					continue
				}
				ast.Inspect(fd, func(n ast.Node) bool {
					kv, ok := n.(*ast.KeyValueExpr)
					if !ok {
						return true
					}
					if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "RunE" {
						return true
					}
					body, ok := kv.Value.(*ast.FuncLit)
					if !ok {
						return true
					}
					ast.Inspect(body, func(n ast.Node) bool {
						if lit, nested := n.(*ast.FuncLit); nested && lit != body {
							return false // a closure's returns are not the command's exits
						}
						ret, ok := n.(*ast.ReturnStmt)
						if !ok || len(ret.Results) != 1 {
							return true
						}
						returns++
						line := fset.Position(ret.Pos()).Line
						call, ok := ret.Results[0].(*ast.CallExpr)
						if !ok {
							offending = append(offending, fmt.Sprintf("line %d: not a call", line))
							return true
						}
						id, ok := call.Fun.(*ast.Ident)
						if !ok {
							offending = append(offending, fmt.Sprintf("line %d: returns an expression", line))
							return true
						}
						if !routers[id.Name] {
							offending = append(offending, fmt.Sprintf("line %d: returns %s", line, id.Name))
						}
						return true
					})
					return false
				})
			}
		}

		// Positive control: the returns were found. Zero would make the
		// emptiness of `offending` meaningless.
		if returns == 0 {
			t.Fatalf("AC-74: no value-returning return found in coverageCmd's RunE, so the claim below is vacuous")
		}
		if len(offending) != 0 {
			names := make([]string, 0, len(routers))
			for k := range routers {
				names = append(names, k)
			}
			sort.Strings(names)
			t.Errorf("AC-74: %d of %d returns in coverageCmd do not resolve to the render owner %s (directly or through %v): %v. Every exit writes a document, or the next one added quietly does not", len(offending), returns, owners[0], names, offending)
		}

		// The verdict half, with its owner DISCOVERED rather than named.
		//
		// Recognizing coverageExitGates by name binds the current spelling: a
		// valid rename fails the guard while the policy boundary holds, which
		// is the mistake spec-check AC-46 already corrected once. The gate
		// owner is instead whichever function calls the shared policy,
		// coverage.GateVerdict, and that has to be exactly one function: a
		// second is a second verdict for one workspace, which is how the text
		// and JSON codes diverged in bugs/SP-SP-066.
		callsSharedPolicy := func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "GateVerdict" {
				return false
			}
			pkg, ok := sel.X.(*ast.Ident)
			return ok && pkg.Name == "coverage"
		}

		// CALL SITES, not owning functions. Counting owners lets one function
		// compute two verdicts:
		//
		//	coverage.GateVerdict(firstInputs)
		//	return coverage.GateVerdict(secondInputs)
		//
		// One owner, two decisions for one workspace. That is the SP-SP-073
		// distinction between sharing a decision function and sharing a
		// decision, and only the site count can see it.
		policySites := funcSitesMatching(fset, files, callsSharedPolicy)
		if len(policySites) != 1 {
			t.Fatalf("AC-74: coverage.GateVerdict is called at %d site(s): %v, want exactly 1. Zero leaves the gate owner undiscoverable and every claim below vacuous; more than one is more than one verdict for one workspace, whichever functions they sit in", len(policySites), policySites)
		}
		// The owner is derived FROM the single site, so it is discovered
		// rather than named and cannot be pinned to today's spelling.
		gateOwner := strings.SplitN(policySites[0], ":", 2)[0]

		callsGateOwner := func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return false
			}
			id, ok := ce.Fun.(*ast.Ident)
			return ok && id.Name == gateOwner
		}
		if got := sitesIn("coverageCmd", callsGateOwner); len(got) != 0 {
			t.Errorf("AC-74: coverageCmd calls the gate owner %s at %v. The verdict belongs with the renderer; a command that keeps its own gate call is a second verdict path", gateOwner, got)
		}
		if got := sitesIn(owners[0], callsGateOwner); len(got) != 1 {
			t.Errorf("AC-74: the render owner %s calls the gate owner %s at %d site(s), want exactly 1. Zero makes the renderer a JSON wrapper with no verdict; more than one is two verdicts for one run", owners[0], gateOwner, len(got))
		}
	})
}

// failingWriter refuses every write, so json.Encoder.Encode returns an error.
type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

// @spec spec-coverage
// @ac AC-74
//
// An unwritable document is the failure C-10 exists to prevent, one layer
// down: the caller gets a truncated stream and, if the error is discarded, a
// verdict computed as though the document had been written whole.
//
// A unit test on the owner, because the branch is unreachable through the CLI.
// That is also why it went unguarded: a mutation discarding the error survived
// the entire suite until the writer became a parameter.
func TestCoverageRenderReportsAnEncoderFailure(t *testing.T) {
	t.Run("spec-coverage/AC-74 a failed write is reported, and no verdict is computed", func(t *testing.T) {
		report := &coverage.CoverageReport{Entries: []coverage.SpecCoverageEntry{}}

		// Gates that would otherwise return nil, so a run that ignored the
		// encoder error would exit 0 and look like a pass.
		gates := &coverageGateInputs{strictness: "annotation"}

		err := renderCoverageResult(failingWriter{err: errors.New("disk full")}, true, report, gates, nil)
		if err == nil {
			t.Fatalf("AC-74: the document could not be written and the run returned nil. A discarded encoder error gives a caller a truncated document and a passing verdict")
		}

		// The positive control. The same inputs through a writer that works
		// must reach the gates and return their verdict, or this test would
		// pass on a renderer that always fails.
		var ok bytes.Buffer
		if err := renderCoverageResult(&ok, true, report, gates, nil); err != nil {
			t.Errorf("AC-74 (control): a writable document returned %v, want nil. Every assertion above would pass on a renderer that always errors", err)
		}
		if ok.Len() == 0 {
			t.Errorf("AC-74 (control): nothing was written to a working writer, so the failure case proves nothing")
		}
	})
}

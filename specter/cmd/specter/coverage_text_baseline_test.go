// coverage_text_baseline_test.go -- a pinned baseline of `coverage` text
// output, spec-coverage 1.26.0 C-10 and AC-74.
//
// C-10 governs what stdout carries in JSON mode. Text output must not move,
// and that claim has to be executable: a hand comparison between two builds
// binds nothing once the person who ran it moves on.
//
// The goldens are captured before the restructure and committed. Every branch
// coverageCmd can take is represented, because a baseline that skips a branch
// cannot notice the restructure changing it: seven refusals, one control, and
// the four exit gates.
//
// Regenerate deliberately, never to make a red test green:
//
//	go test ./cmd/specter/ -run TestCoverageTextBaseline -update
//
// @spec spec-coverage
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var updateBaseline = flag.Bool("update", false, "rewrite the pinned coverage text baseline")

const baselineDir = "testdata/coverage_text_baseline"

// baseSpec is the workspace's one spec: Tier 1, two criteria, so a single
// covered criterion sits at 50% and misses the 100% Tier 1 threshold.
const baseSpec = `spec:
  id: cov-spec
  version: "1.0.0"
  status: approved
  tier: 1
  context: {system: cov, feature: f, description: "A fixture for the pinned coverage text baseline"}
  objective: {summary: "Exercise one coverage branch per scenario"}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
  acceptance_criteria:
    - {id: AC-01, description: "the first", references_constraints: ["C-01"], priority: critical}
    - {id: AC-02, description: "the second", references_constraints: ["C-01"], priority: high}
`

// gateSpec carries an approval_gate criterion, for the code 3 path.
const gateSpec = `spec:
  id: cov-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: cov, feature: f, description: "A fixture carrying an approval gate"}
  objective: {summary: "Reach the approval gate"}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
  acceptance_criteria:
    - {id: AC-01, description: "the first", references_constraints: ["C-01"], priority: critical, approval_gate: true}
`

const testFileBoth = "package p\n\n// @spec cov-spec\n// @ac AC-01\nfunc TestOne(t *testing.T) {}\n\n// @spec cov-spec\n// @ac AC-02\nfunc TestTwo(t *testing.T) {}\n"
const testFileOne = "package p\n\n// @spec cov-spec\n// @ac AC-01\nfunc TestOne(t *testing.T) {}\n"

// coverageScenario is one branch of coverageCmd, its workspace, and the string
// that proves the run reached that branch rather than an earlier one.
type coverageScenario struct {
	name string
	args []string
	// build writes the workspace. Every file it writes is deterministic, so
	// two captures of the same scenario differ only where a path appears.
	build func(w func(rel, body string))
	// cause is a string only this branch emits. The gates are ordered, so a
	// fixture that trips an earlier one reports that gate's code and reads as
	// a pass for the gate it names.
	cause string
}

func coverageScenarios() []coverageScenario {
	manifest := func(extra string) string {
		return "schema_version: 1\nsystem:\n  name: cov\nsettings:\n  specs_dir: specs\n" + extra
	}
	results := func(body string) func(func(string, string)) {
		return func(w func(string, string)) { w(".specter-results.json", body) }
	}
	both := `{"results":[{"spec_id":"cov-spec","ac_id":"AC-01","status":"passed","passed":true},
	{"spec_id":"cov-spec","ac_id":"AC-02","status":"passed","passed":true}]}`
	oneOnly := `{"results":[{"spec_id":"cov-spec","ac_id":"AC-01","status":"passed","passed":true}]}`

	return []coverageScenario{
		// --- the seven refusals ---
		{"refusal_scope_without_strict", []string{"--scope", "core"}, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
		}, "--scope requires --strict"},

		{"refusal_invalid_strictness", []string{"--strictness", "bogus"}, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
		}, "is not a valid value"},

		{"refusal_strict_against_annotation", []string{"--strict"}, func(w func(string, string)) {
			w("specter.yaml", manifest("  strictness: annotation\n"))
			w("specs/cov.spec.yaml", baseSpec)
		}, "--strict requires settings.strictness"},

		{"refusal_manifest", nil, func(w func(string, string)) {
			w("specter.yaml", "schema_version: 1\nsystem:\n  name: cov\nsettings:\n  bogus_key: 1\n")
			w("specs/cov.spec.yaml", baseSpec)
		}, "unknown settings key"},

		{"refusal_unknown_domain", []string{"--scope", "no-such-domain", "--strict"}, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			results(both)(w)
		}, "unknown domain"},

		{"refusal_zero_tolerance_no_annotations", []string{"--strictness", "zero-tolerance"}, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			w("bare_test.go", "package p\n\nfunc TestNothing(t *testing.T) {}\n")
		}, "zero-tolerance strictness requires at least one annotated test file"},

		{"refusal_missing_results", nil, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			w("cov_test.go", testFileBoth)
		}, "requires .specter-results.json"},

		// --- the control ---
		{"control_success", nil, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			w("cov_test.go", testFileBoth)
			results(both)(w)
		}, "1 specs: 1 passing"},

		// --- the four gates. Each carries a results artifact so it reaches
		// its own gate rather than the missing-results refusal. ---
		{"gate_1_below_threshold", nil, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			w("cov_test.go", testFileOne)
			results(oneOnly)(w)
		}, "0 passing, 1 failing"},

		{"gate_2_annotation_no_test", nil, func(w func(string, string)) {
			w("specter.yaml", manifest("  annotation:\n    permissive: false\n"))
			w("specs/cov.spec.yaml", baseSpec)
			w("cov_test.go", testFileOne)
			results(oneOnly)(w)
		}, "no test"},

		{"gate_3_approval", []string{"--strictness", "zero-tolerance"}, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", gateSpec)
			w("cov_test.go", testFileOne)
			results(`{"results":[{"spec_id":"cov-spec","ac_id":"AC-01","status":"passed","passed":true}]}`)(w)
		}, "approval_gate"},

		{"gate_20_inconsistent_streams", nil, func(w func(string, string)) {
			w("specter.yaml", manifest(""))
			w("specs/cov.spec.yaml", baseSpec)
			w("cov_test.go", testFileBoth)
			results(`{"streams":[],"results":[{"spec_id":"cov-spec","ac_id":"AC-01","status":"passed","passed":true,"stream":"ghost"},
	{"spec_id":"cov-spec","ac_id":"AC-02","status":"passed","passed":true}]}`)(w)
		}, "inconsistent streams block"},
	}
}

// normalize replaces the scenario's temp directory with a stable token, so a
// golden captured in one run matches a replay in another.
func normalize(s, dir string) string {
	s = strings.ReplaceAll(s, dir, "<WS>")
	// t.TempDir() names embed the test name and a counter.
	s = regexp.MustCompile(`/tmp/[^\s"']*`).ReplaceAllString(s, "<TMP>")
	return s
}

// @spec spec-coverage
// @ac AC-74
//
// C-10 governs JSON-mode stdout. Text output is unchanged by it, pinned here so
// the restructure cannot move it on a branch nobody looked at.
func TestCoverageTextBaseline(t *testing.T) {
	scenarios := coverageScenarios()

	// Positive control on the baseline itself. A scenario list that shrank
	// would quietly stop pinning branches, and the count is the cheapest way
	// to notice.
	if len(scenarios) != 12 {
		t.Fatalf("AC-74: the baseline covers %d scenarios, want 12: seven refusals, one control, four gates. A branch with no golden cannot notice the restructure changing it", len(scenarios))
	}
	seen := map[string]bool{}
	for _, sc := range scenarios {
		if seen[sc.name] {
			t.Fatalf("AC-74: duplicate scenario name %q, so one golden overwrites another", sc.name)
		}
		seen[sc.name] = true
	}

	if *updateBaseline {
		if err := os.MkdirAll(baselineDir, 0o755); err != nil {
			t.Fatal(err)
		}
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
			stdout, stderr, code := runCLISplit(t, dir, args...)

			// The scenario reaches its own branch before anything is pinned.
			// A fixture that trips an earlier gate pins that gate's output
			// under this scenario's name and reads as coverage of a branch
			// nothing exercises.
			combined := stdout + stderr
			if !strings.Contains(combined, sc.cause) {
				t.Fatalf("AC-74: %s did not reach its branch. Output should contain %q.\nstdout:\n%s\nstderr:\n%s\nexit=%d", sc.name, sc.cause, stdout, stderr, code)
			}

			got := fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s",
				code, normalize(stdout, dir), normalize(stderr, dir))
			golden := filepath.Join(baselineDir, sc.name+".txt")

			if *updateBaseline {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("AC-74: no pinned baseline for %s: %v. Capture it with -update before changing coverageCmd, not after", sc.name, err)
			}
			if got != string(want) {
				t.Errorf("AC-74: %s text output moved. C-10 governs JSON-mode stdout and leaves text output unchanged.\n--- want ---\n%s\n--- got ---\n%s", sc.name, want, got)
			}
		})
	}
}

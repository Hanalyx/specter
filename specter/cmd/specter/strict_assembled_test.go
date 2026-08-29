package main

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Strictness reaches every diagnostic the command assembles, spec-check C-07
// and AC-45, spec-manifest C-35 and AC-67.
//
// The defect: tier_conflict and domain_tier_conflict are computed from the
// manifest and appended after checker.CheckSpecs returns, so both sit past the
// upgrade loop inside it. Under --strict they keep severity warning and the
// run exits 0, on a workspace two specs say should fail.
//
// Written on those two kinds deliberately. The orphan diagnostics AC-07, AC-31
// and AC-36 already cover are produced inside the checker, so an implementation
// that promotes there alone passes all three and fails this file.

// strictWorkspace builds a workspace whose one spec disagrees with both
// settings.tier_overrides and its domain's declared tier, so a single run
// carries a tier_conflict and a domain_tier_conflict and no other diagnostic.
//
// No orphan constraint and no structural conflict: the assertion is about the
// two kinds that escaped, and another diagnostic in the run could carry the
// exit code on its own and hide the defect.
func strictWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("specter.yaml", `schema_version: 1
system:
  name: st
domains:
  core:
    tier: 1
    specs:
      - dm
settings:
  specs_dir: specs
  tier_overrides:
    dm: 1
`)
	write("specs/dm.spec.yaml", `spec:
  id: dm
  version: "1.0.0"
  status: approved
  tier: 3
  context:
    system: st
    feature: strictness
    description: A spec whose declared tier disagrees with both the override and the domain.
  objective:
    summary: Carry one tier conflict and one domain tier conflict and nothing else.
  constraints:
    - id: C-01
      description: "MUST hold"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "It holds"
      references_constraints: ["C-01"]
      priority: critical
`)
	return dir
}

// severitiesByKind reads the check JSON document and returns each diagnostic's
// severity keyed by kind, plus the summary counts.
func severitiesByKind(t *testing.T, stdout string) (map[string]string, int, int) {
	t.Helper()
	var doc struct {
		Diagnostics []struct {
			Kind     string `json:"kind"`
			Severity string `json:"severity"`
		} `json:"diagnostics"`
		Summary struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("check --json did not produce a document: %v\nstdout:\n%s", err, stdout)
	}
	out := map[string]string{}
	for _, d := range doc.Diagnostics {
		out[d.Kind] = d.Severity
	}
	return out, doc.Summary.Errors, doc.Summary.Warnings
}

// @spec spec-check
// @ac AC-45
//
// C-07: the strict upgrade applies to every diagnostic the run reports,
// whatever stage assembled it.
func TestStrictReachesAssembledDiagnostics(t *testing.T) {
	t.Run("spec-check/AC-45 plain check reports both kinds as warnings and exits 0", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check")

		// The control. Both kinds must be present, or the strict assertions
		// below would pass on a run that produced neither.
		for _, kind := range []string{"tier_conflict", "domain_tier_conflict"} {
			if !strings.Contains(stdout, kind) {
				t.Fatalf("AC-45 (plain): %s is absent, so the fixture does not exercise the rule.\nstdout:\n%s", kind, stdout)
			}
		}
		if !strings.Contains(stdout, "warn [tier_conflict]") || !strings.Contains(stdout, "warn [domain_tier_conflict]") {
			t.Errorf("AC-45 (plain): both kinds must report at warning without --strict.\nstdout:\n%s", stdout)
		}
		if code != 0 {
			t.Errorf("AC-45 (plain): exited %d, want 0. A warning does not fail a run.", code)
		}
	})

	t.Run("spec-check/AC-45 strict promotes both kinds and exits 1", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check", "--strict")

		if !strings.Contains(stdout, "error [tier_conflict]") {
			t.Errorf("AC-45 (strict): tier_conflict is still a warning under --strict. C-07 exempts structural_conflict and nothing else, and this kind is appended after the checker's upgrade loop.\nstdout:\n%s", stdout)
		}
		if !strings.Contains(stdout, "error [domain_tier_conflict]") {
			t.Errorf("AC-45 (strict): domain_tier_conflict is still a warning under --strict. spec-manifest C-35 states that --strict promotes it.\nstdout:\n%s", stdout)
		}
		if code != 1 {
			t.Errorf("AC-45 (strict): exited %d, want 1. Two promoted diagnostics are errors, and the verdict follows severity.", code)
		}
	})

	t.Run("spec-check/AC-45 strict JSON agrees with strict text", func(t *testing.T) {
		dir := strictWorkspace(t)
		stdout, _, code := runCLISplit(t, dir, "check", "--strict", "--json")

		sev, errors, warnings := severitiesByKind(t, stdout)
		for _, kind := range []string{"tier_conflict", "domain_tier_conflict"} {
			got, ok := sev[kind]
			if !ok {
				t.Fatalf("AC-45 (strict json): %s is absent from the document. AC-64 requires it to reach --json at all.", kind)
			}
			if got != "error" {
				t.Errorf("AC-45 (strict json): %s carries severity %q, want \"error\". The two modes must not disagree about a severity the same flag decides.", kind, got)
			}
		}
		if warnings != 0 {
			t.Errorf("AC-45 (strict json): summary reports %d warning(s), want 0. Every diagnostic was promoted, so the counts must follow.", warnings)
		}
		if errors != 2 {
			t.Errorf("AC-45 (strict json): summary reports %d error(s), want 2.", errors)
		}
		if code != 1 {
			t.Errorf("AC-45 (strict json): exited %d, want 1, matching text mode on the same workspace.", code)
		}
	})
}

// @spec spec-manifest
// @ac AC-67
//
// C-35: --strict promotes the domain tier conflict.
func TestStrictPromotesDomainTierConflict(t *testing.T) {
	t.Run("spec-manifest/AC-67 the domain tier conflict is an error under strict", func(t *testing.T) {
		dir := strictWorkspace(t)

		plainOut, _, plainCode := runCLISplit(t, dir, "check")
		if !strings.Contains(plainOut, "warn [domain_tier_conflict]") || plainCode != 0 {
			t.Fatalf("AC-67 (control): without --strict the kind must warn and exit 0; got exit %d.\nstdout:\n%s", plainCode, plainOut)
		}

		strictOut, _, strictCode := runCLISplit(t, dir, "check", "--strict")
		if !strings.Contains(strictOut, "error [domain_tier_conflict]") {
			t.Errorf("AC-67: C-35 says --strict promotes this diagnostic, and it is still a warning.\nstdout:\n%s", strictOut)
		}
		if strictCode != 1 {
			t.Errorf("AC-67: exited %d under --strict, want 1. Exiting 0 here is a false green in the command CI runs.", strictCode)
		}

		// AC-67 declares JSON outputs of its own. They were declared and not
		// asserted: the behavior happened to be covered by AC-45, so this
		// criterion reported covered without running one of its stated cases.
		jsonOut, _, jsonCode := runCLISplit(t, dir, "check", "--strict", "--json")
		sev, _, warnings := severitiesByKind(t, jsonOut)
		if got := sev["domain_tier_conflict"]; got != "error" {
			t.Errorf("AC-67 (json): severity %q, want \"error\", matching text mode on the same workspace.", got)
		}
		if warnings != 0 {
			t.Errorf("AC-67 (json): summary reports %d warning(s), want 0.", warnings)
		}
		if jsonCode != 1 {
			t.Errorf("AC-67 (json): exited %d, want 1.", jsonCode)
		}
	})
}

// annotationWorkspace builds a workspace on settings.strictness: threshold
// carrying exactly one unreachable source annotation and nothing else.
//
// threshold is what routes unreachable_annotation to warning; under annotation
// it is suppressed and under zero-tolerance it is already an error, and neither
// of those observes the promotion this criterion is about.
func strictAnnotationWorkspace(t *testing.T, testBody string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("specter.yaml", `schema_version: 1
system:
  name: ua
settings:
  specs_dir: specs
  strictness: threshold
`)
	write("specs/u.spec.yaml", `spec:
  id: u
  version: "1.0.0"
  status: approved
  tier: 3
  context:
    system: ua
    feature: annotations
    description: A fixture whose source annotation is not visible to the runner.
  objective:
    summary: Carry one annotation diagnostic and nothing else.
  constraints:
    - id: C-01
      description: "MUST hold"
      type: technical
      enforcement: error
  acceptance_criteria:
    - id: AC-01
      description: "It holds"
      references_constraints: ["C-01"]
      priority: critical
`)
	write("u_test.go", testBody)
	return dir
}

// @spec spec-check
// @ac AC-45
//
// C-07 over the --test diagnostics: they are computed from test file contents,
// which the checker does not read, so they arrive by the same route the tier
// kinds do and must reach the same owner.
func TestStrictReachesAnnotationDiagnostics(t *testing.T) {
	// A Go test whose title carries no u/AC-01 token and whose body prints
	// nothing, so the scanner can read the shape and finds it unreachable.
	const unreachable = `package p

import "testing"

// @spec u
// @ac AC-01
func TestSomethingElse(t *testing.T) {
	_ = 1
}
`
	t.Run("spec-check/AC-45 an unreachable annotation promotes under --strict", func(t *testing.T) {
		dir := strictAnnotationWorkspace(t, unreachable)

		plainOut, _, plainCode := runCLISplit(t, dir, "check", "--test")
		if !strings.Contains(plainOut, "warn [unreachable_annotation]") || plainCode != 0 {
			t.Fatalf("AC-45 (annotation control): without --strict this must warn and exit 0; got exit %d.\nstdout:\n%s", plainCode, plainOut)
		}

		strictOut, _, strictCode := runCLISplit(t, dir, "check", "--test", "--strict")
		if !strings.Contains(strictOut, "error [unreachable_annotation]") {
			t.Errorf("AC-45 (annotation): unreachable_annotation is still a warning under --strict. It is not in C-07's non-promotable set, and it reaches the result after the checker returns.\nstdout:\n%s", strictOut)
		}
		if strictCode != 1 {
			t.Errorf("AC-45 (annotation): exited %d under --strict, want 1.", strictCode)
		}
	})

	t.Run("spec-check/AC-45 the unknown form is not promoted", func(t *testing.T) {
		// A shape the reachability scanner cannot read: the annotation sits on
		// a package-level var rather than inside a recognizable test function.
		const unknown = `package p

// @spec u
// @ac AC-01
var somethingUnrecognized = 1
`
		dir := strictAnnotationWorkspace(t, unknown)

		out, _, code := runCLISplit(t, dir, "check", "--test", "--strict")
		// Fatalf, not Skipf. A skip here makes this criterion green on a run
		// that never reached its subject, and the annotation still counts
		// toward coverage, so the criterion would report covered while
		// observing nothing.
		if !strings.Contains(out, "unreachable_annotation_unknown") {
			t.Fatalf("AC-45 (unknown): the fixture produced no unreachable_annotation_unknown, so the non-promotion claim is unobserved. Fix the fixture; do not skip.\nstdout:\n%s", out)
		}
		if strings.Contains(out, "error [unreachable_annotation_unknown]") {
			t.Errorf("AC-45 (unknown): the soft form was promoted. C-07's non-promotable set includes it, per C-10: a scanner that could not read a test shape has found no defect.\nstdout:\n%s", out)
		}
		if code != 0 {
			t.Errorf("AC-45 (unknown): exited %d, want 0. This diagnostic never fails a gate on its own.", code)
		}
	})
}

// @spec spec-check
// @ac AC-45
//
// C-07's ownership half, asserted structurally because no runtime observation
// can see it.
//
// The behavioral tests above prove the current severities. They cannot prove
// that one place decides them: restoring an equivalent promotion loop beside
// an assembly site produces identical output today and leaves every assertion
// green, while the non-promotable set has two definitions that will drift. Two
// copies of that set are what this constraint exists to prevent.
func TestStrictnessHasOneOwner(t *testing.T) {
	t.Run("spec-check/AC-45 one promotion site, one summary site, and neither in the command", func(t *testing.T) {
		fset := token.NewFileSet()
		checkerFiles := parseProductionFiles(t, fset, filepath.Join("..", "..", "internal", "checker"))
		cmdFiles := parseProductionFiles(t, fset, ".")

		// A promotion is an ASSIGNMENT to a .Severity field. Constructing a
		// diagnostic sets Severity too, in a composite literal, and that is not
		// a promotion, which is why this reads assignments only.
		promotes := func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return false
			}
			for _, lhs := range as.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Severity" {
					return true
				}
			}
			return false
		}

		// Any write to a summary counter, by increment or assignment.
		mutatesSummary := func(n ast.Node) bool {
			var lhs ast.Expr
			switch s := n.(type) {
			case *ast.IncDecStmt:
				lhs = s.X
			case *ast.AssignStmt:
				if len(s.Lhs) != 1 {
					return false
				}
				lhs = s.Lhs[0]
			default:
				return false
			}
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			switch sel.Sel.Name {
			case "Errors", "Warnings", "Info":
			default:
				return false
			}
			inner, ok := sel.X.(*ast.SelectorExpr)
			return ok && inner.Sel.Name == "Summary"
		}

		// Sites, with positions, not function names. A set of names cannot tell
		// one promotion loop from two inside the same function, and "exactly one
		// place applies it" is a claim about places.
		promoteSites := funcSitesMatching(fset, checkerFiles, promotes)
		if len(promoteSites) != 1 {
			t.Errorf("AC-45: severity is assigned at %d site(s) in internal/checker, %v, want exactly 1. C-07 requires one owner for the upgrade, because a second site is a second definition of the non-promotable set", len(promoteSites), promoteSites)
		} else if !strings.HasPrefix(promoteSites[0], "CheckSpecs:") {
			t.Errorf("AC-45: the one severity assignment is at %s, want it inside CheckSpecs", promoteSites[0])
		}

		if sites := funcSitesMatching(fset, cmdFiles, promotes); len(sites) != 0 {
			t.Errorf("AC-45: the command assigns severity at %v. Diagnostics it assembles are handed to the checker through CheckOptions.ExtraDiagnostics; promoting them here would be a private copy of the rule", sites)
		}

		// The summary is computed once. Counting write sites alone cannot say
		// that: the counter is a three-arm switch, so a correct implementation
		// has three writes and a second counting loop would have six. The
		// claim is instead that every write lives in summarize, and that
		// summarize is called once.
		//
		// Forbidding command-side mutation alone would let a second counting
		// loop inside the checker pass, which is the hole this closes.
		for _, site := range funcSitesMatching(fset, checkerFiles, mutatesSummary) {
			if !strings.HasPrefix(site, "summarize:") {
				t.Errorf("AC-45: a summary counter is written at %s, outside summarize. C-07 requires the summary to be computed once, from the final diagnostics, after the upgrade", site)
			}
		}
		callsSummarize := func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return false
			}
			id, ok := ce.Fun.(*ast.Ident)
			return ok && id.Name == "summarize"
		}
		if sites := funcSitesMatching(fset, checkerFiles, callsSummarize); len(sites) != 1 {
			t.Errorf("AC-45: summarize is called at %d site(s), %v, want exactly 1. Two calls are two summaries, and the second would disagree with the first about a diagnostic the upgrade moved", len(sites), sites)
		}

		if sites := funcSitesMatching(fset, cmdFiles, mutatesSummary); len(sites) != 0 {
			t.Errorf("AC-45: the command writes a result summary at %v. A count taken at an assembly site reports as a warning what the run reports as an error", sites)
		}
	})
}

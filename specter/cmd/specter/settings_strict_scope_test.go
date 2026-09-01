// settings_strict_scope_test.go -- settings.strict promotes severity and does
// not enable the test-annotation scan, spec-check 5.0.0 C-07/C-09, AC-47 and
// AC-48, bugs/SP-SP-047 finding 3.
//
// A severity setting cannot decide which defects are discovered. The manifest
// key did: removing it made a broken spec reference invisible.
//
// @spec spec-check
package main

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// scopeSpec is a Tier 3 spec with one criterion, annotated by one of the test
// files below so coverage passes and cannot supply a failure.
const scopeSpec = `spec:
  id: s-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: s, feature: f, description: "A fixture whose second test names a spec that does not exist"}
  objective: {summary: "Separate what the key promotes from what it discovers"}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
  acceptance_criteria:
    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}
`

// scopeTests annotates the real criterion, so coverage is satisfied, and then
// references a spec id that does not exist, which is the only defect in the
// workspace and the only thing the scan can report.
const scopeTests = `package p

// @spec s-spec
// @ac AC-01
func TestReal(t *testing.T) {}

// @spec bogus-spec
// @ac AC-01
func TestGhost(t *testing.T) {}
`

const scopeManifestBase = `schema_version: 1
system:
  name: s
settings:
  specs_dir: specs
  strictness: annotation
`

// scopeWorkspace writes the fixture and returns its root. withKey adds
// settings.strict: true and nothing else, so the key is the only variable.
func scopeWorkspace(t *testing.T, withKey bool) string {
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
	manifest := scopeManifestBase
	if withKey {
		manifest += "  strict: true\n"
	}
	write("specter.yaml", manifest)
	write("specs/s.spec.yaml", scopeSpec)
	write("s_test.go", scopeTests)
	return dir
}

// checkKinds returns the diagnostic kinds `check --json` reports, sorted.
func checkKinds(t *testing.T, stdout string) []string {
	t.Helper()
	var doc struct {
		Diagnostics []struct {
			Kind string `json:"kind"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("check --json did not parse: %v\n%s", err, stdout)
	}
	out := make([]string, 0, len(doc.Diagnostics))
	for _, d := range doc.Diagnostics {
		out = append(out, d.Kind)
	}
	sort.Strings(out)
	return out
}

// syncCheckPhase returns the message `sync --json` reports for the check phase.
// sync names no diagnostic kind in any output mode, which is why the criterion
// observes the scan here through counts and pins attribution separately.
func syncCheckPhase(t *testing.T, stdout string) string {
	t.Helper()
	var doc struct {
		Phases []struct {
			Phase   string `json:"phase"`
			Message string `json:"message"`
		} `json:"phases"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("sync --json did not parse: %v\n%s", err, stdout)
	}
	for _, p := range doc.Phases {
		if p.Phase == "check" {
			return p.Message
		}
	}
	return ""
}

// syncCheckErrors reads the error count out of sync's check-phase message.
//
// Parsed rather than prefix-matched, because the message omits the error term
// when there are none: a passing phase reads "0 warning(s), 1 info" and a
// failing one reads "1 error(s), 0 warning(s)". A prefix check on
// "0 error(s)" therefore fails a phase that reported no errors at all.
func syncCheckErrors(t *testing.T, message string) int {
	t.Helper()
	for _, part := range strings.Split(message, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasSuffix(part, "error(s)") {
			continue
		}
		n := 0
		for _, r := range strings.TrimSuffix(part, " error(s)") {
			if r < '0' || r > '9' {
				t.Fatalf("AC-47/48: cannot read an error count from %q", message)
			}
			n = n*10 + int(r-'0')
		}
		return n
	}
	// No error term at all means none were reported.
	return 0
}

func countKind(kinds []string, want string) int {
	n := 0
	for _, k := range kinds {
		if k == want {
			n++
		}
	}
	return n
}

// @spec spec-check
// @ac AC-47
//
// C-09: the scan is entered by `check --test` and `sync --strict`, and not by
// the manifest key.
func TestSettingsStrictDoesNotEnableTheAnnotationScan(t *testing.T) {
	// Attribution first. The sync assertions below read an error COUNT,
	// because sync names no kind, and a count only means what this criterion
	// says it means if the scan is the workspace's only error source.
	t.Run("spec-check/AC-47 attribution: the scan is the only defect in this workspace", func(t *testing.T) {
		dir := scopeWorkspace(t, false)

		withTest, _, code := runCLISplit(t, dir, "check", "--test", "--json")
		if got := countKind(checkKinds(t, withTest), "unknown_spec_ref"); got != 1 {
			t.Fatalf("AC-47: check --test reports %d unknown_spec_ref, want exactly 1. Without that the sync error counts below are not attributable to the scan", got)
		}
		if code != 1 {
			t.Errorf("AC-47: check --test exited %d, want 1", code)
		}

		plain, _, plainCode := runCLISplit(t, dir, "check", "--json")
		if got := checkKinds(t, plain); len(got) != 0 {
			t.Fatalf("AC-47: check without --test reports %v, want none. Any other diagnostic would make a sync error count ambiguous", got)
		}
		if plainCode != 0 {
			t.Errorf("AC-47: check without --test exited %d, want 0", plainCode)
		}
	})

	t.Run("spec-check/AC-47 sync does not run the scan, with or without the key", func(t *testing.T) {
		for _, withKey := range []bool{true, false} {
			label := "settings.strict absent"
			if withKey {
				label = "settings.strict: true"
			}
			dir := scopeWorkspace(t, withKey)

			stdout, _, code := runCLISplit(t, dir, "sync", "--json")
			phase := syncCheckPhase(t, stdout)
			if got := syncCheckErrors(t, phase); got != 0 {
				t.Errorf("AC-47 (%s): sync's check phase reports %d error(s) in %q, want none. The manifest key must not enable the scan, and the scan is this workspace's only error source", label, got, phase)
			}
			if code != 0 {
				t.Errorf("AC-47 (%s): sync exited %d, want 0", label, code)
			}
		}
	})

	t.Run("spec-check/AC-47 the two supported entries work under both manifests", func(t *testing.T) {
		// The control. Without it a build that deleted the scan entirely would
		// satisfy every assertion above.
		for _, withKey := range []bool{true, false} {
			label := "settings.strict absent"
			if withKey {
				label = "settings.strict: true"
			}
			dir := scopeWorkspace(t, withKey)

			stdout, _, code := runCLISplit(t, dir, "sync", "--strict", "--json")
			phase := syncCheckPhase(t, stdout)
			if got := syncCheckErrors(t, phase); got != 1 {
				t.Errorf("AC-47 (%s): sync --strict reports %d error(s) in check phase %q, want 1. The flag keeps its own behavior whatever the manifest says", label, got, phase)
			}
			if code != 1 {
				t.Errorf("AC-47 (%s): sync --strict exited %d, want 1", label, code)
			}

			out, _, ccode := runCLISplit(t, dir, "check", "--test", "--json")
			if got := countKind(checkKinds(t, out), "unknown_spec_ref"); got != 1 {
				t.Errorf("AC-47 (%s): check --test reports %d unknown_spec_ref, want 1", label, got)
			}
			if ccode != 1 {
				t.Errorf("AC-47 (%s): check --test exited %d, want 1", label, ccode)
			}
		}
	})
}

// @spec spec-check
// @ac AC-47
//
// C-09's ownership half, asserted structurally because no behavioral case can
// see a site added later: a new one has to be exercised to be observed.
//
// It follows RETURN FLOW rather than presence. A helper counts only when the
// value it returns depends on the key, local bindings are read in source
// order, and named callees are followed recursively with the caller's
// arguments substituted for their parameters.
//
// WHAT THIS IS NOT. It is a syntactic guard, not a type-resolved analysis. It
// does not use go/types, so it cannot follow the key through a struct field, a
// slice or map element, a closure capture, an interface method, or a value
// crossing a package boundary. A regression taking one of those routes would
// pass here.
//
// An earlier version of this comment said the residual could only over-report.
// That was wrong, and three false negatives followed from believing it: a
// `.Settings` value bound to a local, a literal built through a type alias,
// and a helper chain longer than the depth budget. Each is closed below, the
// first two by widening what counts as a key read and which literals are
// examined, the third by FAILING when the budget is exhausted rather than
// returning a clean answer.
//
// So the honest claim is narrow: this catches the shapes a refactor of the
// present code would plausibly take, and reports when it cannot decide. The
// behavioral cases in AC-47 are what bind the rule; this is a tripwire under
// them.

// maxFlowDepth bounds the recursive follow. Exceeding it is reported rather
// than treated as a clean result.
const maxFlowDepth = 8

func TestNoSiteFeedsSettingsStrictIntoTheScan(t *testing.T) {
	t.Run("spec-check/AC-47 nothing assigns settings.strict to the scan enable input", func(t *testing.T) {
		fset := token.NewFileSet()
		files := parseProductionFiles(t, fset, ".")

		// settingsAliases collects identifiers bound to a `.Settings` value, so
		// `s := m.Settings; ... s.Strict` reads the key as much as
		// `m.Settings.Strict` does. Without this the alias is a FALSE
		// NEGATIVE, which is the direction that matters for a guard.
		settingsAliases := map[string]bool{}
		isSettingsExpr := func(e ast.Expr) bool {
			sel, ok := e.(*ast.SelectorExpr)
			return ok && sel.Sel.Name == "Settings"
		}
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.AssignStmt:
					for i, lhs := range v.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && i < len(v.Rhs) && isSettingsExpr(v.Rhs[i]) {
							settingsAliases[id.Name] = true
						}
					}
				case *ast.ValueSpec:
					for i, nm := range v.Names {
						if i < len(v.Values) && isSettingsExpr(v.Values[i]) {
							settingsAliases[nm.Name] = true
						}
					}
				}
				return true
			})
		}

		// isKeyRead reports whether e reads the key: `<x>.Settings.Strict`, or
		// `<alias>.Strict` where the alias was bound from a `.Settings` value.
		isKeyRead := func(e ast.Expr) bool {
			sel, ok := e.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Strict" {
				return false
			}
			if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Settings" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			return ok && settingsAliases[id.Name]
		}

		decl := func(name string) *ast.FuncDecl {
			for _, f := range files {
				for _, d := range f.Decls {
					if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name && fd.Recv == nil {
						return fd
					}
				}
			}
			return nil
		}

		// gaveUp records that the analysis hit its depth budget. A budget that
		// silently returns "not tainted" is a false negative dressed as a
		// clean result, so exceeding it fails the test instead.
		gaveUp := []string{}

		var taintedExpr func(ast.Expr, map[string]bool, int) bool
		var returnsTainted func(*ast.FuncDecl, map[string]bool, int) bool

		// taintedExpr answers whether e's VALUE depends on the key.
		taintedExpr = func(e ast.Expr, env map[string]bool, depth int) bool {
			if e == nil {
				return false
			}
			if depth > maxFlowDepth {
				gaveUp = append(gaveUp, "expression at depth "+itoa(depth))
				return false
			}
			switch v := e.(type) {
			case *ast.SelectorExpr:
				return isKeyRead(v)
			case *ast.Ident:
				return env[v.Name]
			case *ast.ParenExpr:
				return taintedExpr(v.X, env, depth)
			case *ast.UnaryExpr:
				return taintedExpr(v.X, env, depth)
			case *ast.BinaryExpr:
				return taintedExpr(v.X, env, depth) || taintedExpr(v.Y, env, depth)
			case *ast.CallExpr:
				id, ok := v.Fun.(*ast.Ident)
				if !ok {
					return false
				}
				callee := decl(id.Name)
				if callee == nil || callee.Type.Params == nil {
					return false
				}
				// Substitute the caller's arguments for the callee's
				// parameters, so a helper is tainted only when the argument
				// it returns is.
				sub := map[string]bool{}
				argIdx := 0
				for _, field := range callee.Type.Params.List {
					for _, name := range field.Names {
						if argIdx < len(v.Args) {
							sub[name.Name] = taintedExpr(v.Args[argIdx], env, depth+1)
						}
						argIdx++
					}
				}
				return returnsTainted(callee, sub, depth+1)
			}
			return false
		}

		// returnsTainted answers whether any value fn RETURNS depends on the
		// key, given the taint of its parameters. Statements are walked in
		// order so a later binding does not color an earlier read.
		returnsTainted = func(fd *ast.FuncDecl, env map[string]bool, depth int) bool {
			if fd == nil || fd.Body == nil {
				return false
			}
			if depth > maxFlowDepth {
				gaveUp = append(gaveUp, "callee "+fd.Name.Name+" at depth "+itoa(depth))
				return false
			}
			local := map[string]bool{}
			for k, v := range env {
				local[k] = v
			}
			tainted := false
			walk := func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.FuncLit:
					return false // a nested closure returns to its own caller
				case *ast.AssignStmt:
					for i, lhs := range v.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && i < len(v.Rhs) {
							local[id.Name] = taintedExpr(v.Rhs[i], local, depth)
						}
					}
					return false
				case *ast.ValueSpec:
					for i, nm := range v.Names {
						if i < len(v.Values) {
							local[nm.Name] = taintedExpr(v.Values[i], local, depth)
						}
					}
					return false
				case *ast.ReturnStmt:
					for _, r := range v.Results {
						if taintedExpr(r, local, depth) {
							tainted = true
						}
					}
					return false
				}
				return true
			}
			ast.Inspect(fd.Body, walk)
			return tainted
		}

		var offenders []string
		sawField := false
		for _, f := range files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok {
					continue
				}
				// The enclosing function's locals, built in source order up to
				// each literal, so assignment order is respected.
				env := map[string]bool{}
				walk := func(n ast.Node) bool {
					switch v := n.(type) {
					case *ast.AssignStmt:
						for i, lhs := range v.Lhs {
							if id, ok := lhs.(*ast.Ident); ok && i < len(v.Rhs) {
								env[id.Name] = taintedExpr(v.Rhs[i], env, 0)
							}
						}
					case *ast.CompositeLit:
						// EVERY composite literal carrying the field, whatever
						// its type is written as. Matching only a selector
						// ending in SyncInput skipped a literal built through
						// a type alias, which is a false negative. Examining
						// an unrelated struct that happens to spell the field
						// the same way is a false positive, and that is the
						// direction to err in.
						for _, el := range v.Elts {
							kv, ok := el.(*ast.KeyValueExpr)
							if !ok {
								continue
							}
							key, ok := kv.Key.(*ast.Ident)
							if !ok || key.Name != "CheckTestAnnotations" {
								continue
							}
							sawField = true
							if taintedExpr(kv.Value, env, 0) {
								offenders = append(offenders,
									fd.Name.Name+":"+itoa(fset.Position(kv.Pos()).Line))
							}
						}
					}
					return true
				}
				ast.Inspect(fd, walk)
			}
		}

		// Positive control: the field was found on a SyncInput literal. A
		// rename would otherwise make the emptiness below meaningless.
		if !sawField {
			t.Fatalf("AC-47: no CheckTestAnnotations field found on a SyncInput literal, so the claim below is vacuous")
		}
		if len(gaveUp) != 0 {
			t.Errorf("AC-47: the flow analysis ran out of depth at %v, so a clean result here would mean it stopped looking rather than that nothing was found. Raise maxFlowDepth or simplify the chain", gaveUp)
		}
		if len(offenders) != 0 {
			t.Errorf("AC-47: settings.strict reaches the scan's enable input at %v. C-09 names check --test and sync --strict as the two ways in, and the manifest key is not one of them: a severity setting cannot decide which defects are discovered", offenders)
		}
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// runWatchFirstCycle runs `specter watch` in dir until a deadline, then
// returns everything it printed.
//
// watch loops, so it is observed from its first cycle, which completes before
// any file changes.
//
// The lifecycle is the context's, and Wait is called exactly once, by Run.
// Two earlier versions got this wrong: the first ignored the error from
// Signal and then waited forever, and the second added a post-kill timeout
// that logged and returned WITHOUT receiving from the wait channel, leaving
// the process unreaped and the buffer read while the output machinery was
// still writing to it.
//
// AC-48 asks nothing about graceful shutdown, so none is attempted. The
// context kills at the deadline; WaitDelay bounds how long Run will wait for
// the output pipes afterwards. Run returns only once the process is reaped,
// so the buffer is safe to read after it and only after it.
func runWatchFirstCycle(t *testing.T, dir string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "watch")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SPECTER_TEST=1")
	cmd.WaitDelay = 5 * time.Second

	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	// An error is expected ONLY when the deadline killed the process. Any
	// other error is a start failure or a premature exit, and logging it as
	// though it were the deadline would turn a broken run into a silently
	// empty output that the assertions then misread.
	if err := cmd.Run(); err != nil {
		if ctx.Err() == nil {
			t.Fatalf("watch failed before the deadline: %v\noutput:\n%s", err, buf.String())
		}
		t.Logf("watch was killed by the deadline: %v", err)
	}
	return buf.String()
}

// promoSpec carries one referenced constraint and one unreferenced one, at
// Tier 3, so the orphan routes to info and is promotable.
const promoSpec = `spec:
  id: p-spec
  version: "1.0.0"
  status: approved
  tier: 3
  context: {system: p, feature: f, description: "A tier 3 fixture carrying one unreferenced constraint"}
  objective: {summary: "Produce one promotable info diagnostic"}
  constraints:
    - {id: C-01, description: "MUST hold", type: technical, enforcement: error}
    - {id: C-02, description: "MUST also hold, referenced by nothing", type: technical}
  acceptance_criteria:
    - {id: AC-01, description: "It holds", references_constraints: ["C-01"], priority: critical}
`

const promoTests = `package p

// @spec p-spec
// @ac AC-01
func TestReal(t *testing.T) {}
`

func promoWorkspace(t *testing.T, withKey bool) string {
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
	manifest := "schema_version: 1\nsystem:\n  name: p\nsettings:\n  specs_dir: specs\n  strictness: annotation\n"
	if withKey {
		manifest += "  strict: true\n"
	}
	write("specter.yaml", manifest)
	write("specs/p.spec.yaml", promoSpec)
	write("p_test.go", promoTests)
	return dir
}

// rawDiagnostics returns the diagnostics list as generic maps, so the whole
// object is comparable rather than a chosen field or two.
func rawDiagnostics(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var doc struct {
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("check --json did not parse: %v\n%s", err, stdout)
	}
	return doc.Diagnostics
}

// @spec spec-check
// @ac AC-48
//
// C-07: settings.strict routes to CheckOptions.Strict on every surface that
// checks, and promotion is all it does.
//
// Green on arrival, because all three routes already conform. Red-first cannot
// verify it, so the evidence is mutation: deleting each route independently
// must fail this criterion, and runWatchCycle's is the one an earlier draft
// would have missed.
func TestSettingsStrictPromotesAndOnlyPromotes(t *testing.T) {
	t.Run("spec-check/AC-48 check promotes the same diagnostics, severity aside", func(t *testing.T) {
		without, _, codeWithout := runCLISplit(t, promoWorkspace(t, false), "check", "--json")
		with, _, codeWith := runCLISplit(t, promoWorkspace(t, true), "check", "--json")

		a := rawDiagnostics(t, without)
		b := rawDiagnostics(t, with)

		if len(a) != 1 || a[0]["kind"] != "orphan_constraint" || a[0]["severity"] != "info" {
			t.Fatalf("AC-48: without the key, want exactly one orphan_constraint at info, got %v", a)
		}
		if len(b) != 1 || b[0]["kind"] != "orphan_constraint" || b[0]["severity"] != "error" {
			t.Fatalf("AC-48: with the key, want exactly one orphan_constraint at error, got %v", b)
		}

		// The WHOLE object, severity normalized away. Comparing kinds alone
		// would let the message, the constraint id or the spec id change under
		// the key, and a setting that rewrites what a diagnostic says has not
		// merely promoted it.
		normalize := func(ds []map[string]any) string {
			out := make([]map[string]any, 0, len(ds))
			for _, d := range ds {
				c := map[string]any{}
				for k, v := range d {
					if k == "severity" {
						continue
					}
					c[k] = v
				}
				out = append(out, c)
			}
			raw, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			return string(raw)
		}
		if normalize(a) != normalize(b) {
			t.Errorf("AC-48: the key changed more than severity.\nwithout: %s\nwith:    %s", normalize(a), normalize(b))
		}

		if codeWithout != 0 {
			t.Errorf("AC-48: check without the key exited %d, want 0", codeWithout)
		}
		if codeWith != 1 {
			t.Errorf("AC-48: check with the key exited %d, want 1. The key promotes info to error, and an error fails the run", codeWith)
		}
	})

	t.Run("spec-check/AC-48 sync's check phase promotes the same way", func(t *testing.T) {
		without, _, codeWithout := runCLISplit(t, promoWorkspace(t, false), "sync", "--json")
		with, _, codeWith := runCLISplit(t, promoWorkspace(t, true), "sync", "--json")

		pw, pk := syncCheckPhase(t, without), syncCheckPhase(t, with)
		if got := syncCheckErrors(t, pw); got != 0 {
			t.Errorf("AC-48: sync without the key reports %d error(s) in check phase %q, want none", got, pw)
		}
		if got := syncCheckErrors(t, pk); got != 1 {
			t.Errorf("AC-48: sync with the key reports %d error(s) in check phase %q, want 1. sync's check phase is a separate consumption site from check's", got, pk)
		}
		if codeWithout != 0 || codeWith != 1 {
			t.Errorf("AC-48: sync exit codes are %d without the key and %d with it, want 0 and 1", codeWithout, codeWith)
		}
	})

	t.Run("spec-check/AC-48 watch's check phase promotes the same way", func(t *testing.T) {
		// The surface an earlier draft of C-07 missed. It already routed the
		// key, so a criterion naming only check and sync would have stayed
		// green if this routing were deleted.
		without := runWatchFirstCycle(t, promoWorkspace(t, false))
		with := runWatchFirstCycle(t, promoWorkspace(t, true))

		// The control this surface needs and the other two do not: watch
		// prints one summary line, so a FAIL could come from any phase.
		// Reporting coverage proves the baseline cycle reached PAST the check
		// phase, which is what localizes the keyed failure to promotion.
		if !strings.Contains(without, "ACs covered") {
			t.Fatalf("AC-48: watch without the key did not reach coverage, so a failure under the key cannot be attributed to the check phase.\n%s", without)
		}
		if strings.Contains(without, "FAIL  check") {
			t.Errorf("AC-48: watch without the key already fails the check phase, so the key is not what promotes.\n%s", without)
		}
		if !strings.Contains(with, "FAIL  check") {
			t.Errorf("AC-48: watch with the key does not fail the check phase. runWatchCycle routes settings.strict to CheckOptions.Strict, and C-07 requires it.\n%s", with)
		}
	})
}

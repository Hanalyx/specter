package main

import (
	"go/ast"
	"go/token"
	"testing"
)

// @spec spec-sync
// @ac AC-21
//
// C-11: one place owns the mapping from a verdict code to a process exit, and
// a code matching no case reports failure rather than success.
//
// Green on arrival. The routing it guards was consolidated before this
// criterion existed, so red-first cannot verify it and mutation did instead:
// changing the unmatched-case return from an error to nil left every other
// test in the repository green.
func TestUnroutedGateCodeReportsFailure(t *testing.T) {
	t.Run("spec-sync/AC-21 the gate router refuses to report success it cannot justify", func(t *testing.T) {
		// A code no case names. The routed non-zero codes call os.Exit and
		// cannot be exercised in-process, which is what leaves this the only
		// branch of the router a test can reach.
		if err := exitOnGateCode(99); err == nil {
			t.Error("C-11: an unmatched non-zero gate code returned nil, which reports success for a run that failed a gate. The two surfaces would also disagree about it, since one returns nil and the other falls through to its own failure exit")
		}
		if err := exitOnGateCode(0); err != nil {
			t.Errorf("C-11: code 0 returned %v, want nil. A passing run must not be reported as a failure", err)
		}
		if err := exitOnGateCode(1); err == nil {
			t.Error("C-11: code 1 returned nil, but it is the command's own unclassified failure exit")
		}
	})
}

// gateExitCodeNames are the codes only the shared router may raise. Named
// rather than numeric, which is how the source states them and how spec-sync
// AC-17 requires an exit argument to be readable.
var gateExitCodeNames = map[string]bool{
	"exitCoverageNoTest":       true,
	"exitCoverageApprovalGate": true,
	"exitStreamValidation":     true,
}

const gateRouterName = "exitOnGateCode"

// gateRoutingScan reads `cmd/specter` and reports, keyed by the top-level
// function containing it, where a gate code is raised and where the router is
// called.
//
// Keyed by the enclosing FuncDecl, so a closure counts as the function that
// declares it. That is what makes a private switch restored inside `sync`'s
// exit closure visible here: it would be attributed to `syncCmd`.
//
// parseExitScanDir skips `_test.go`, so this file's own calls to the router
// are not counted and cannot make the scan pass by being present.
func gateRoutingScan(t *testing.T) (raisers, callers map[string]int) {
	t.Helper()
	fset := token.NewFileSet()
	files, err := parseExitScanDir(fset, ".")
	if err != nil {
		t.Fatalf("could not read cmd/specter: %v", err)
	}
	raisers, callers = map[string]int{}, map[string]int{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isOsExit(call) && len(call.Args) == 1 {
					if id, ok := call.Args[0].(*ast.Ident); ok && gateExitCodeNames[id.Name] {
						raisers[fn.Name.Name]++
					}
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == gateRouterName {
					callers[fn.Name.Name]++
				}
				return true
			})
		}
	}
	return raisers, callers
}

// @spec spec-sync
// @ac AC-21
//
// C-11: one place owns the mapping from a verdict code to a process exit, and
// both commands route through it.
//
// The behavioral test above cannot see this. It calls the router directly, so
// it stays green while a command stops using the router entirely, which is the
// state C-11 forbids and the one that existed before the consolidation. Only a
// read of the source distinguishes them.
func TestGateRoutingHasOneOwner(t *testing.T) {
	t.Run("spec-sync/AC-21 both commands route through one owner", func(t *testing.T) {
		raisers, callers := gateRoutingScan(t)

		// Every gate code is raised inside the router and nowhere else. A
		// private switch restored in either command puts that command's name
		// in this map.
		wantRaisers := map[string]int{gateRouterName: len(gateExitCodeNames)}
		if len(raisers) != 1 || raisers[gateRouterName] != len(gateExitCodeNames) {
			t.Errorf("C-11: gate codes are raised from %v, want exactly %v. A command raising one itself owns a second copy of the mapping, which is what lets the two surfaces disagree about a code neither switch names",
				raisers, wantRaisers)
		}

		// Both commands call it. Positive and exact: a zero count would pass a
		// containment check vacuously, and a third caller is a site this
		// assertion has not been told about.
		total := 0
		for _, n := range callers {
			total += n
		}
		if total != 2 || callers["coverageExitGates"] != 1 || callers["syncCmd"] != 1 {
			t.Errorf("C-11: %s is called from %v, want exactly one call from coverageExitGates and one from syncCmd. A command that stopped calling it is routing its own codes again",
				gateRouterName, callers)
		}
	})
}

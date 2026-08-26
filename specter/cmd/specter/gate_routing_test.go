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

const gateRouterName = "exitOnGateCode"

// The two functions AC-21 names. Neither may raise an exit of its own: a
// command that does owns a second copy of the mapping.
var gateRoutingCommands = []string{"coverageExitGates", "syncCmd"}

// documentedForeignGateExits are the sites outside the router that raise a
// code the router also raises, and are allowed to.
//
// One entry. `main` exits 2 on a recovered panic, and 2 is also the
// annotation-rule gate's code. docs/EXIT_CODES.md section 1 carries that row
// with the standing "Accidental collision" rather than Stable, and section 3
// schedules the panic path onto the sysexits band. Until it moves, the
// collision is real and known.
//
// Compared for equality rather than membership, so this cannot rot quietly: a
// new foreign site fails, and so does the panic path moving off 2 while this
// map still claims it is there.
var documentedForeignGateExits = map[string]int{"main": 1}

// gateRoutingScan reads `cmd/specter` and reports how exits are raised and
// where the router is called, keyed by the top-level function containing each.
//
// Codes are resolved, not read as identifiers. An earlier draft recognized a
// gate exit only when its argument was one of three names it carried in a
// list. That refuses `os.Exit(20)`, which spec-sync AC-17 explicitly permits,
// so a private literal outside the router was invisible while the same literal
// inside it failed, and adding a gate meant updating one more list. Ownership
// is a question about codes, and the spelling is not the code.
//
// Keyed by the enclosing FuncDecl, so a closure counts as the function that
// declares it. That is what makes a private switch restored inside `sync`'s
// exit closure visible here: it is attributed to `syncCmd`.
//
// parseExitScanDir skips `_test.go`, so this file's own calls to the router
// are not counted and cannot make the scan pass by being present.
func gateRoutingScan(t *testing.T) (exits map[string][]string, callers map[string]int) {
	t.Helper()
	fset := token.NewFileSet()
	files, err := parseExitScanDir(fset, ".")
	if err != nil {
		t.Fatalf("could not read cmd/specter: %v", err)
	}
	consts := map[string]string{}
	for _, f := range files {
		collectIntConsts(f, consts)
	}

	exits, callers = map[string][]string{}, map[string]int{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isOsExit(call) {
					code, ok := resolveExitArg(call, consts)
					if !ok {
						// AC-17 refuses an argument it cannot resolve, and so
						// does this: an unresolvable exit could be a gate code
						// and nothing here could tell.
						t.Errorf("C-11: %s raises an os.Exit this scan cannot resolve to a code, so ownership cannot be decided", name)
						return true
					}
					exits[name] = append(exits[name], code)
				}
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == gateRouterName {
					callers[name]++
				}
				return true
			})
		}
	}
	return exits, callers
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
		exits, callers := gateRoutingScan(t)

		// The gate codes are whatever the router raises, read from the router
		// rather than listed here. A gate added to it needs no edit in this
		// file, and cannot be added to it and forgotten here either.
		gateCodes := map[string]bool{}
		for _, code := range exits[gateRouterName] {
			gateCodes[code] = true
		}
		if len(gateCodes) == 0 {
			t.Fatalf("C-11: %s raises no exit at all, so there is no ownership to check and every assertion below would pass vacuously", gateRouterName)
		}

		// Neither command raises an exit of its own, whatever the code and
		// however it is spelled.
		for _, cmd := range gateRoutingCommands {
			if got := exits[cmd]; len(got) != 0 {
				t.Errorf("C-11: %s raises os.Exit itself, with code(s) %v. Routing belongs to %s, and a second copy of the mapping is what lets the two surfaces disagree about a code neither one names",
					cmd, got, gateRouterName)
			}
		}

		// No other function raises a gate code, except the one site the
		// registry documents.
		foreign := map[string]int{}
		for fn, codes := range exits {
			if fn == gateRouterName {
				continue
			}
			for _, code := range codes {
				if gateCodes[code] {
					foreign[fn]++
				}
			}
		}
		if len(foreign) != len(documentedForeignGateExits) {
			t.Errorf("C-11: gate codes are raised outside %s at %v, want exactly %v. See docs/EXIT_CODES.md section 1 for why the one exemption exists",
				gateRouterName, foreign, documentedForeignGateExits)
		} else {
			for fn, want := range documentedForeignGateExits {
				if foreign[fn] != want {
					t.Errorf("C-11: %s raises %d gate code(s), want %d. %v", fn, foreign[fn], want, foreign)
				}
			}
		}

		// Both commands call it. Positive and exact: a zero count would pass a
		// containment check vacuously, and a third caller is a site this
		// assertion has not been told about.
		total := 0
		for _, n := range callers {
			total += n
		}
		if total != len(gateRoutingCommands) {
			t.Errorf("C-11: %s is called from %v, want exactly one call from each of %v", gateRouterName, callers, gateRoutingCommands)
		}
		for _, cmd := range gateRoutingCommands {
			if callers[cmd] != 1 {
				t.Errorf("C-11: %s calls %s %d time(s), want 1. A command that stopped calling it is routing its own codes again", cmd, gateRouterName, callers[cmd])
			}
		}
	})
}

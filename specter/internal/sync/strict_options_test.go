package sync

import (
	"testing"

	"github.com/Hanalyx/specter/internal/checker"
)

// RunSync does not mutate the options it is handed, spec-sync C-13 and AC-22.
//
// The pipeline extends CheckOptions.ExtraDiagnostics with the diagnostics it
// assembles. CheckOpts is a pointer the caller still owns, so appending through
// it would leave the caller's options carrying diagnostics from a run it did
// not ask about, and a second run would then start from the first one's list.

const optSpec = `spec:
  id: opt
  version: "1.0.0"
  status: approved
  tier: 3
  context:
    system: opt
    feature: options
    description: A spec used to check that RunSync leaves caller options alone.
  objective:
    summary: Carry one annotated criterion and one unreferenced constraint.
  constraints:
    - id: C-01
      description: "MUST hold"
      type: technical
      enforcement: error
    - id: C-02
      description: "MUST also hold, and no criterion references this one"
      type: technical
  acceptance_criteria:
    - id: AC-01
      description: "It holds"
      references_constraints: ["C-01"]
      priority: critical
`

// @spec spec-sync
// @ac AC-22
//
// spec-sync C-13: RunSync does not mutate the SyncInput it is given.
func TestRunSyncLeavesCallerOptionsAlone(t *testing.T) {
	t.Run("spec-sync/AC-22 RunSync does not write through the caller's CheckOptions", func(t *testing.T) {
		// Seeded with an existing diagnostic AND spare capacity. Both matter.
		//
		// Without an existing element, a run that REPLACED the caller's slice
		// instead of extending it would pass unnoticed.
		//
		// Without spare capacity, a run that copies the slice header and then
		// appends in place would pass too: the caller's length stays 1, and the
		// write lands in the shared backing array past that length, where a
		// length check cannot see it. The array is read directly below for
		// exactly that reason.
		backing := make([]checker.CheckDiagnostic, 1, 8)
		backing[0] = checker.CheckDiagnostic{Kind: "callers_own", Severity: "info", Message: "supplied by the caller", SpecID: "opt"}
		backing = backing[:1]
		sentinel := checker.CheckDiagnostic{Kind: "sentinel", Severity: "info", Message: "must not be overwritten", SpecID: "opt"}
		backing[:2][1] = sentinel

		opts := &checker.CheckOptions{Strict: true, ExtraDiagnostics: backing}

		input := SyncInput{
			SpecFiles:            []FileContent{{Path: "specs/opt.spec.yaml", Content: optSpec}},
			TestFiles:            []FileContent{{Path: "opt_test.go", Content: "package p\n\n// @spec opt\n// @ac AC-99\nfunc TestX(t *testing.T) {}\n"}},
			CheckOpts:            opts,
			CheckTestAnnotations: true,
			Strictness:           "annotation",
		}

		// The control. AC-99 does not exist on the spec, so the run really does
		// assemble a test-annotation diagnostic; without one there would be
		// nothing to leak into the caller's options.
		result := RunSync(input)
		if result == nil || result.CheckResult == nil {
			t.Fatal("AC-22: sync produced no check result, so the claim below is unobserved")
		}
		var found bool
		for _, d := range result.CheckResult.Diagnostics {
			if d.Kind == "unknown_ac_ref" {
				found = true
			}
		}
		if !found {
			t.Fatalf("AC-22: the run assembled no unknown_ac_ref, so nothing could have leaked. diagnostics: %+v", result.CheckResult.Diagnostics)
		}

		// The caller's own diagnostic survives, exactly once, and so does the
		// one the run assembled. A replacement would drop the first.
		count := func(diags []checker.CheckDiagnostic, kind string) int {
			n := 0
			for _, d := range diags {
				if d.Kind == kind {
					n++
				}
			}
			return n
		}
		if got := count(result.CheckResult.Diagnostics, "callers_own"); got != 1 {
			t.Errorf("AC-22: the caller's diagnostic appears %d time(s) in the result, want 1. A run that replaces ExtraDiagnostics drops it; one that appends twice duplicates it", got)
		}
		if got := count(result.CheckResult.Diagnostics, "unknown_ac_ref"); got != 1 {
			t.Errorf("AC-22: the assembled diagnostic appears %d time(s), want 1", got)
		}

		if len(opts.ExtraDiagnostics) != 1 {
			t.Errorf("AC-22: the caller's slice length is %d after the run, want 1. The options are the caller's; a run that appends through the pointer leaves them carrying a previous run's list", len(opts.ExtraDiagnostics))
		}
		// The half a length check cannot see. Re-slice into the caller's own
		// backing array and read past its length.
		if got := backing[:2][1]; got != sentinel {
			t.Errorf("AC-22: the run wrote into the caller's backing array past its length. Got %+v, want the sentinel untouched. Copying the slice header alone is not enough when the caller's slice has spare capacity", got)
		}

		// A second run over the same options must see the same result. If the
		// first run had appended, this one would start from its list and count
		// the same diagnostics twice.
		second := RunSync(input)
		if a, b := len(result.CheckResult.Diagnostics), len(second.CheckResult.Diagnostics); a != b {
			t.Errorf("AC-22: two runs over one CheckOptions produced %d and %d diagnostics. They must be independent", a, b)
		}
	})
}

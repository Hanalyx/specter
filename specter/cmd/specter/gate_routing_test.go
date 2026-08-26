package main

import (
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

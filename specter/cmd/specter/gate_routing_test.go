package main

import (
	"testing"
)

// The shared gate router's fallback.
//
// Not tied to an acceptance criterion. It guards the branch that runs when a
// code the shared verdict returns reaches a router that does not name it, and
// no criterion describes that state because no released version can reach it.
// It exists because the alternative to reporting is claiming a workspace
// passed a gate that failed it.
//
// Found by mutation against the routing consolidation: changing the fallback
// to return nil left every other test green.
func TestUnroutedGateCodeReportsFailure(t *testing.T) {
	// A code no case names. The routed non-zero codes call os.Exit and cannot
	// be exercised in-process, which is what leaves this branch the only part
	// of the router a test can reach.
	if err := exitOnGateCode(99); err == nil {
		t.Error("an unrouted non-zero gate code returned nil, which reports success for a run that failed a gate")
	}
	if err := exitOnGateCode(0); err != nil {
		t.Errorf("code 0 returned %v, want nil. A passing run must not be reported as a failure", err)
	}
	if err := exitOnGateCode(1); err == nil {
		t.Error("code 1 returned nil, but it is the command's own failure exit")
	}
}

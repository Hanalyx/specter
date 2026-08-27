// failing_gates_test.go -- `coverage --failing` and the exit gates,
// spec-coverage 1.25.0 C-17/AC-73, bugs/SP-SP-074.
//
// @spec spec-coverage
package main

import (
	"strings"
	"testing"
)

// bothPassedOn returns a results body covering both criteria, labeled or not.
func bothPassedOn(stream string) string {
	label := ""
	if stream != "" {
		label = `,"stream":"` + stream + `"`
	}
	return `{"spec_id":"s","ac_id":"AC-01","status":"passed"` + label + `},` +
		`{"spec_id":"s","ac_id":"AC-02","status":"passed"` + label + `}`
}

// @spec spec-coverage
// @ac AC-73
//
// C-17: `--failing` changes the table and nothing else. The gates it can skip
// are the ones that report without moving a coverage percentage, because every
// other gate leaves a row for the filter to keep and so never reaches the
// empty-table path.
func TestFailingFlagDoesNotChangeTheVerdict(t *testing.T) {
	t.Run("spec-coverage/AC-73 --failing returns what the plain run returns", func(t *testing.T) {
		cases := []struct {
			name     string
			settings string
			acs      string
			results  string
			want     int
			// namesInCause is a fragment the cause line must carry, on both
			// runs. Empty when the gate decides the code without printing.
			namesInCause string
		}{
			{
				// Reports without demoting, so the filter finds nothing to
				// show and the empty-table path is reached.
				name:         "approval gate at 100 percent",
				settings:     permissiveSettings,
				acs:          approvalGateStreamACs,
				results:      `{"results":[` + bothPassedOn("") + `]}`,
				want:         exitCoverageApprovalGate,
				namesInCause: "approval_gate=true",
			},
			{
				name:         "stream validation at 100 percent",
				settings:     permissiveSettings,
				acs:          defaultStreamACs,
				results:      `{"streams":[],"results":[` + bothPassedOn("e2e") + `]}`,
				want:         exitStreamValidationPlanned,
				namesInCause: "e2e",
			},
			{
				// Below the tier threshold as well. The threshold is ordered
				// first, so its code wins and the stream violations are still
				// reported. This one keeps a row, so it does not reach the
				// empty-table path; it is here so precedence and complete
				// reporting are bound on the flag too.
				name:     "stream validation beside the threshold",
				settings: "  strictness: threshold\n",
				acs:      defaultStreamACs,
				results: `{"streams":[],"results":[{"spec_id":"s","ac_id":"AC-01","status":"failed","stream":"e2e"},` +
					`{"spec_id":"s","ac_id":"AC-02","status":"failed","stream":"e2e"}]}`,
				want:         1,
				namesInCause: "e2e",
			},
			{
				// The control. Nothing is wrong, so the flag must still take
				// the early return and say so. A fix that simply stopped
				// returning early would pass every case above and fail here.
				name:     "nothing wrong at 100 percent",
				settings: permissiveSettings,
				acs:      defaultStreamACs,
				results:  `{"results":[` + bothPassedOn("") + `]}`,
				want:     0,
			},
		}

		for _, c := range cases {
			dir := streamWorkspaceACs(t, c.results, c.settings, c.acs)

			plainOut, plainErr, plainCode := runCLISplit(t, dir, "coverage")
			flagOut, flagErr, flagCode := runCLISplit(t, dir, "coverage", "--failing")

			if plainCode != c.want {
				t.Errorf("C-17 (%s): the plain run exited %d, want %d. The fixture does not reach the gate it is written for, so the comparison below would prove nothing.\nstderr:\n%s",
					c.name, plainCode, c.want, plainErr)
				continue
			}
			if flagCode != plainCode {
				t.Errorf("C-17 (%s): coverage exited %d and coverage --failing exited %d on the same workspace. --failing changes the table, not which gates are evaluated.\n plain stderr:\n%s\n --failing stderr:\n%s",
					c.name, plainCode, flagCode, plainErr, flagErr)
			}
			if flagErr != plainErr {
				t.Errorf("C-17 (%s): the two runs printed different causes.\n plain:\n%s\n --failing:\n%s", c.name, plainErr, flagErr)
			}
			if c.namesInCause != "" && !strings.Contains(flagErr, c.namesInCause) {
				t.Errorf("C-17 (%s): the --failing cause line does not name %q, so the diagnostic did not survive the flag.\nstderr:\n%s",
					c.name, c.namesInCause, flagErr)
			}

			// C-17's own presentation rule, on the control. The empty-table
			// confirmation is what the early return exists to print.
			if c.want == 0 {
				if !strings.Contains(flagOut, "at 100% coverage.") {
					t.Errorf("C-17 (%s): --failing printed no empty-table confirmation.\nstdout:\n%s", c.name, flagOut)
				}
				if strings.Contains(plainOut, "at 100% coverage.") {
					t.Errorf("C-17 (%s): the plain run printed the empty-table confirmation, which belongs to the filter", c.name)
				}
			}
		}
	})
}

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

			// C-17's own presentation rule, on the control.
			//
			// Both halves are asserted. The confirmation has to appear, and
			// the table it replaces has to be gone. Asserting only the
			// confirmation is a false green: a run that prints the line and
			// then renders the empty table and its footer satisfies a
			// contains check while breaking the rule the line exists to
			// serve, which C-17 states as an empty table with a single-line
			// confirmation.
			if c.want == 0 {
				const tableHeader = "Spec ID"
				const tableFooter = "specs: "
				got := map[string]bool{
					"confirmation": strings.Contains(flagOut, "at 100% coverage."),
					"table header": strings.Contains(flagOut, tableHeader),
					"table footer": strings.Contains(flagOut, tableFooter),
				}
				want := map[string]bool{"confirmation": true, "table header": false, "table footer": false}
				for k, w := range want {
					if got[k] != w {
						t.Errorf("C-17 (%s): under --failing the %s is present=%v, want %v. The filter replaces the table with one line; it does not print the line above the table.\nstdout:\n%s",
							c.name, k, got[k], w, flagOut)
					}
				}
				// Positive control on the same three, without the flag. If the
				// plain run stopped printing a table, every assertion above
				// would pass for the wrong reason.
				plain := map[string]bool{
					"confirmation": strings.Contains(plainOut, "at 100% coverage."),
					"table header": strings.Contains(plainOut, tableHeader),
					"table footer": strings.Contains(plainOut, tableFooter),
				}
				plainWant := map[string]bool{"confirmation": false, "table header": true, "table footer": true}
				for k, w := range plainWant {
					if plain[k] != w {
						t.Errorf("C-17 (%s): without the flag the %s is present=%v, want %v", c.name, k, plain[k], w)
					}
				}
			}
		}
	})
}

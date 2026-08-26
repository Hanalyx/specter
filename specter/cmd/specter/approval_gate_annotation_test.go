// approval_gate_annotation_test.go -- CLI integration tests for SP-071: the
// approval gate under `settings.annotation` (spec-coverage 1.20.0 C-40/AC-67,
// spec-sync 2.2.0 AC-19).
//
// These cross the CLI boundary because C-40 is a statement about a process exit
// code and about what reaches stderr, and the coverage package is pure. Every
// case goes through runCLISplit so the exit code and the diagnostics can be
// asserted separately.
//
// `sync` is exercised alongside `coverage` on purpose rather than trusted to
// inherit. It used to carry its own gate sequence instead of calling the one
// `coverage` uses, which is why these criteria exist: a one-sided fix would
// have left the two disagreeing, the shape of bugs/done/SP-SP-066. The copy is
// gone as of the SP-071 fix and both now route through coverage.GateVerdict.
// The assertions stay, because what stops the copy coming back is a test that
// fails when the two surfaces disagree, not the absence of the copy today.
//
// @spec spec-coverage
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// approvalWorkspace builds a workspace whose AC-01 is annotated, passing, and
// gated on a human sign-off that has not happened. With untested true it also
// declares AC-02, which carries no test at all, so the run holds a rule-1
// violation and an unmet approval gate at once.
//
// Tier 3 so the tier threshold does not decide the run in the two-criterion
// case. The gates under test are the subject; the threshold is not.
func approvalWorkspace(t *testing.T, settings string, untested bool) string {
	t.Helper()
	dir := t.TempDir()

	acs := "    - id: AC-01\n      description: \"annotated, passing, gated on a sign-off that has not happened\"\n" +
		"      references_constraints: [\"C-01\"]\n      approval_gate: true\n"
	if untested {
		acs += "    - id: AC-02\n      description: \"declared with no test at all\"\n" +
			"      references_constraints: [\"C-01\"]\n"
	}
	writeSpec(t, dir, "gate.spec.yaml",
		"spec:\n  id: gate-spec\n  version: \"1.0.0\"\n  status: approved\n  tier: 3\n"+
			"  context:\n    system: test\n"+
			"  objective:\n    summary: One criterion gated on a human sign-off.\n"+
			"  constraints:\n    - id: C-01\n      description: \"MUST hold\"\n"+
			"  acceptance_criteria:\n"+acs)

	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package tests\n\nimport \"testing\"\n\n// @spec gate-spec\n// @ac AC-01\n" +
		"func TestGate(t *testing.T) {\n\tt.Run(\"gate-spec/AC-01\", func(t *testing.T) {})\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "tests", "gate_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	writeResultsJSON(t, dir, `{"results":[{"spec_id":"gate-spec","ac_id":"AC-01","status":"passed","test_name":"TestGate"}]}`)
	putManifest(t, dir, "system:\n  name: g\nsettings:\n  specs_dir: specs\n  tests_glob: \"tests/*_test.go\"\n"+settings)
	return dir
}

const (
	annotationPermissive    = "  annotation:\n    permissive: true\n"
	annotationStrict        = "  annotation:\n    permissive: false\n"
	ladderZeroTolerance     = "  strictness: zero-tolerance\n"
	approvalGateStderrMatch = "approval_gate"
	ruleOneStderrMatch      = "no test"
)

// @spec spec-coverage
// @ac AC-67
//
// C-40: the approval gate fires under the annotation model, permissive does not
// soften it, and a run holding both violations exits 2 while naming both.
// bugs/SP-SP-071.
func TestApprovalGateUnderAnnotation(t *testing.T) {
	t.Run("spec-coverage/AC-67 the approval gate fires under settings.annotation", func(t *testing.T) {
		// C-40(a): permissive governs rule 1 and nothing else, so both values
		// produce the same code here.
		for _, c := range []struct {
			name     string
			settings string
		}{
			{"permissive true", annotationPermissive},
			{"permissive false", annotationStrict},
		} {
			// C-40(c): both surfaces, in exit code and in cause line. A
			// text-only fix would be the sixth instance of the --json family.
			for _, surface := range [][]string{{"coverage"}, {"coverage", "--json"}} {
				dir := approvalWorkspace(t, c.settings, false)
				_, stderr, code := runCLISplit(t, dir, surface...)
				label := strings.Join(surface, " ")
				if code != 3 {
					t.Errorf("AC-67 (%s, %s): exited %d on an unmet approval gate, expected 3. C-40(a) makes permissive govern rule 1 and not this gate.\nstderr:\n%s",
						c.name, label, code, stderr)
				}
				// C-40(b): a silent 3 is not the contract. The measured defect
				// was exit 0 with nothing on stderr, and an assertion on the
				// integer alone would accept a bare code.
				if !strings.Contains(stderr, approvalGateStderrMatch) {
					t.Errorf("AC-67 (%s, %s): the run did not name the unmet approval gate on stderr.\nstderr:\n%s",
						c.name, label, stderr)
				}
			}
		}

		// The ladder still exits 3. The amendment adds a path; it does not move
		// one. Without this the two cases above could pass on an implementation
		// that broke the ladder.
		ladder := approvalWorkspace(t, ladderZeroTolerance, false)
		_, ladderErr, ladderCode := runCLISplit(t, ladder, "coverage")
		if ladderCode != 3 {
			t.Errorf("AC-67: the ladder exited %d on the same workspace, expected 3. The annotation-model path must be added beside it, not in place of it.\nstderr:\n%s",
				ladderCode, ladderErr)
		}

		// C-40(d) and (e): rule 1 wins the code, and both violations are named.
		// Both modes, because (e) is a sequencing rule and sequencing is what a
		// duplicated branch gets wrong. A text-only fix would leave the JSON
		// surface exiting inside the rule-1 branch.
		for _, surface := range [][]string{{"coverage"}, {"coverage", "--json"}} {
			both := approvalWorkspace(t, annotationStrict, true)
			_, bothErr, bothCode := runCLISplit(t, both, surface...)
			label := strings.Join(surface, " ")
			if bothCode != 2 {
				t.Errorf("AC-67 (%s): a workspace with a rule-1 violation and an unmet approval gate exited %d, expected 2. C-40(d) evaluates rule 1 first.\nstderr:\n%s",
					label, bothCode, bothErr)
			}
			assertNamesBoth(t, label, bothErr)
		}
	})
}

// @spec spec-sync
// @ac AC-19
//
// C-11: sync returns the code coverage returns, and names the same causes.
// Asserted rather than inherited, because sync carries its own gate sequence.
// bugs/SP-SP-071.
func TestApprovalGateUnderAnnotation_SyncParity(t *testing.T) {
	t.Run("spec-sync/AC-19 sync matches coverage on the annotation-model approval gate", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			settings string
		}{
			{"permissive true", annotationPermissive},
			{"permissive false", annotationStrict},
		} {
			covDir := approvalWorkspace(t, c.settings, false)
			_, covErr, covCode := runCLISplit(t, covDir, "coverage")

			// C-40(c): sync in both modes. Nothing else binds sync --json for
			// this gate, because C-09's --json clause is scoped to the ladder.
			for _, surface := range [][]string{{"sync"}, {"sync", "--json"}} {
				syncDir := approvalWorkspace(t, c.settings, false)
				_, syncErr, syncCode := runCLISplit(t, syncDir, surface...)
				label := strings.Join(surface, " ")
				if syncCode != 3 {
					t.Errorf("AC-19 (%s, %s): exited %d on an unmet approval gate, expected 3.\nstderr:\n%s", c.name, label, syncCode, syncErr)
				}
				if syncCode != covCode {
					t.Errorf("AC-19 (%s, %s): coverage exited %d and it exited %d on the same workspace. C-11 requires them to agree, and sync is the CI entry point.\n  coverage stderr:\n%s\n  stderr:\n%s",
						c.name, label, covCode, syncCode, covErr, syncErr)
				}
				if !strings.Contains(syncErr, approvalGateStderrMatch) {
					t.Errorf("AC-19 (%s, %s): the run did not name the unmet approval gate on stderr. C-11 requires sync to name the cause coverage names.\nstderr:\n%s",
						c.name, label, syncErr)
				}
			}
		}

		for _, surface := range [][]string{{"sync"}, {"sync", "--json"}} {
			both := approvalWorkspace(t, annotationStrict, true)
			_, bothErr, bothCode := runCLISplit(t, both, surface...)
			label := strings.Join(surface, " ")
			if bothCode != 2 {
				t.Errorf("AC-19 (%s): exited %d on a workspace with both violations, expected 2.\nstderr:\n%s", label, bothCode, bothErr)
			}
			assertNamesBoth(t, label, bothErr)
		}
	})
}

// assertNamesBoth checks C-40(e): a run holding both violations names both
// before it exits. Asserting the exit code alone is satisfied by an
// implementation that exits inside the rule-1 branch and never reaches the
// gate, which is what both surfaces do today.
func assertNamesBoth(t *testing.T, command, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, ruleOneStderrMatch) {
		t.Errorf("C-40(e): `%s` did not name the rule-1 violation.\nstderr:\n%s", command, stderr)
	}
	if !strings.Contains(stderr, approvalGateStderrMatch) {
		t.Errorf("C-40(e): `%s` returned the rule-1 code and never named the unmet approval gate. Both violations must be reported in the one run, or an operator fixes the missing test, re-runs, and only then learns about the gate.\nstderr:\n%s",
			command, stderr)
	}
}

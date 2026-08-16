// @spec spec-check
//
// Tests for C-13: duplicate acceptance criterion IDs within a single spec.
//
// Two notes on how these tests assert, both forced by the spec text:
//
//  1. CheckDiagnostic carries no AC-id field, and C-13 requires the AC id,
//     the occurrence count, and the occurrence positions to appear in the
//     message. Those three are asserted against Message. The spec id is
//     asserted against the SpecID field, because AC-21 lists `spec_id`
//     plainly while it prefixes the other two with `message_names_`.
//
//  2. The checker package is pure and cannot observe a process exit code.
//     `exit_code_nonzero` is asserted as Summary.Errors > 0 and
//     `exit_code_zero` as Summary.Errors == 0. That is the exact rule the
//     CLI applies: cmd/specter/main.go returns errSilent if and only if
//     result.Summary.Errors > 0.
package checker

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// dupSpec builds a spec whose acceptance criteria carry the given ids in
// declaration order. Every criterion references C-01, so no orphan
// constraint diagnostic can fire and pollute the error count.
func dupSpec(id string, tier int, acIDs ...string) schema.SpecAST {
	spec := schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: tier,
		Context:   schema.SpecContext{System: "test"},
		Objective: schema.SpecObjective{Summary: "test"},
		Constraints: []schema.Constraint{
			{ID: "C-01", Description: "a constraint with no keyword the conflict scanner reads"},
		},
	}
	for _, acID := range acIDs {
		spec.AcceptanceCriteria = append(spec.AcceptanceCriteria, schema.AcceptanceCriterion{
			ID:                    acID,
			Description:           "criterion " + acID,
			ReferencesConstraints: []string{"C-01"},
		})
	}
	return spec
}

// dupDiagnostics returns every duplicate_ac_id diagnostic in a result.
func dupDiagnostics(result *CheckResult) []CheckDiagnostic {
	var out []CheckDiagnostic
	for _, d := range result.Diagnostics {
		if d.Kind == "duplicate_ac_id" {
			out = append(out, d)
		}
	}
	return out
}

// dupDiagnosticFor returns the duplicate_ac_id diagnostic whose message
// names the given AC id, or nil. "AC-01" is not a substring of "AC-001",
// so this match is exact enough for the AC-26 fixture too.
func dupDiagnosticFor(result *CheckResult, acID string) *CheckDiagnostic {
	diags := dupDiagnostics(result)
	for i := range diags {
		if acIDMentioned(diags[i].Message, acID) {
			return &diags[i]
		}
	}
	return nil
}

var acIDPattern = regexp.MustCompile(`AC-\d+`)
var numberPattern = regexp.MustCompile(`\d+`)

func acIDMentioned(message, acID string) bool {
	for _, found := range acIDPattern.FindAllString(message, -1) {
		if found == acID {
			return true
		}
	}
	return false
}

// assertMessageNames checks that the message names the occurrence count and
// every occurrence position, without pinning a wording. AC ids are stripped
// first so "AC-01" does not donate a 1. The remaining numbers must contain
// the count and the positions with multiplicity: in AC-23 the count 3 and
// the position 3 are distinct facts, and a message naming only the
// positions would satisfy a presence-only check.
func assertMessageNames(t *testing.T, message string, count int, positions []int) {
	t.Helper()

	stripped := acIDPattern.ReplaceAllString(message, "")
	pool := map[int]int{}
	for _, tok := range numberPattern.FindAllString(stripped, -1) {
		n, err := strconv.Atoi(tok)
		if err != nil {
			continue
		}
		pool[n]++
	}

	want := append([]int{count}, positions...)
	for _, n := range want {
		if pool[n] == 0 {
			t.Errorf("message %q does not name count %d and positions %v (missing an occurrence of %d)",
				message, count, positions, n)
			return
		}
		pool[n]--
	}
}

// @spec spec-check
// @ac AC-21
func TestDuplicateACIDEmitsOneErrorDiagnostic(t *testing.T) {
	t.Run("spec-check/AC-21 duplicate ac id emits one error diagnostic", func(t *testing.T) {
		spec := dupSpec("payment-charge", 1, "AC-01", "AC-02", "AC-01")
		g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

		result := CheckSpecs(g, nil)

		diags := dupDiagnostics(result)
		if len(diags) != 1 {
			t.Fatalf("expected 1 duplicate_ac_id diagnostic, got %d", len(diags))
		}
		d := diags[0]
		if d.Severity != "error" {
			t.Errorf("expected severity %q, got %q", "error", d.Severity)
		}
		if d.SpecID != "payment-charge" {
			t.Errorf("expected spec id %q, got %q", "payment-charge", d.SpecID)
		}
		if !acIDMentioned(d.Message, "AC-01") {
			t.Errorf("expected message to name the duplicated id AC-01, got %q", d.Message)
		}
		assertMessageNames(t, d.Message, 2, []int{1, 3})
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-22
func TestDuplicateACIDSeverityIgnoresTier(t *testing.T) {
	t.Run("spec-check/AC-22 duplicate ac id severity ignores tier", func(t *testing.T) {
		spec := dupSpec("ledger-export", 3, "AC-01", "AC-01")
		g := makeGraph(map[string]*resolver.SpecNode{"ledger-export": {Spec: spec, File: "ledger-export.spec.yaml"}}, nil)

		result := CheckSpecs(g, nil)

		diags := dupDiagnostics(result)
		if len(diags) == 0 {
			t.Fatal("expected a duplicate_ac_id diagnostic for a tier 3 spec")
		}
		for _, d := range diags {
			if d.Severity == "info" {
				t.Errorf("tier 3 must not demote duplicate_ac_id to info, got %q", d.Severity)
			}
			if d.Severity != "error" {
				t.Errorf("expected severity %q at every tier, got %q", "error", d.Severity)
			}
		}
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-23
func TestDuplicateACIDThreeOccurrencesOneDiagnostic(t *testing.T) {
	t.Run("spec-check/AC-23 duplicate ac id three occurrences one diagnostic", func(t *testing.T) {
		spec := dupSpec("payment-charge", 1, "AC-01", "AC-02", "AC-01", "AC-01")
		g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

		result := CheckSpecs(g, nil)

		diags := dupDiagnostics(result)
		if len(diags) != 1 {
			t.Fatalf("expected 1 duplicate_ac_id diagnostic (not one per occurrence, not one per pair), got %d", len(diags))
		}
		d := diags[0]
		if d.Severity != "error" {
			t.Errorf("expected severity %q, got %q", "error", d.Severity)
		}
		if !acIDMentioned(d.Message, "AC-01") {
			t.Errorf("expected message to name the duplicated id AC-01, got %q", d.Message)
		}
		assertMessageNames(t, d.Message, 3, []int{1, 3, 4})
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-24
func TestDuplicateACIDTwoDuplicatedIDsTwoDiagnostics(t *testing.T) {
	t.Run("spec-check/AC-24 duplicate ac id two duplicated ids two diagnostics", func(t *testing.T) {
		spec := dupSpec("payment-charge", 1, "AC-01", "AC-02", "AC-01", "AC-02", "AC-03")
		g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

		result := CheckSpecs(g, nil)

		diags := dupDiagnostics(result)
		if len(diags) != 2 {
			t.Fatalf("expected 2 duplicate_ac_id diagnostics, one per duplicated id, got %d", len(diags))
		}
		for _, d := range diags {
			if d.Severity != "error" {
				t.Errorf("expected severity %q, got %q", "error", d.Severity)
			}
			if acIDMentioned(d.Message, "AC-03") {
				t.Errorf("AC-03 is declared once and must not be reported: %q", d.Message)
			}
		}

		// The spec names the two diagnostics as diagnostic_1 and
		// diagnostic_2, but neither AC-24 nor C-13 states an order
		// between them, so each is located by the id its message names.
		first := dupDiagnosticFor(result, "AC-01")
		if first == nil {
			t.Fatal("expected a duplicate_ac_id diagnostic naming AC-01")
		}
		assertMessageNames(t, first.Message, 2, []int{1, 3})

		second := dupDiagnosticFor(result, "AC-02")
		if second == nil {
			t.Fatal("expected a duplicate_ac_id diagnostic naming AC-02")
		}
		assertMessageNames(t, second.Message, 2, []int{2, 4})

		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-25
func TestDuplicateACIDScopedToOneSpec(t *testing.T) {
	t.Run("spec-check/AC-25 duplicate ac id scoped to one spec", func(t *testing.T) {
		specA := dupSpec("payment-charge", 1, "AC-01", "AC-02")
		specB := dupSpec("payment-refund", 1, "AC-01", "AC-02")
		g := makeGraph(
			map[string]*resolver.SpecNode{
				"payment-charge": {Spec: specA, File: "payment-charge.spec.yaml"},
				"payment-refund": {Spec: specB, File: "payment-refund.spec.yaml"},
			},
			[]resolver.SpecEdge{{From: "payment-refund", To: "payment-charge", Relationship: "requires"}},
		)

		result := CheckSpecs(g, nil)

		if n := len(dupDiagnostics(result)); n != 0 {
			t.Errorf("expected 0 duplicate_ac_id diagnostics across two specs sharing ids, got %d", n)
		}
		if result.Summary.Errors != 0 {
			t.Errorf("expected Summary.Errors == 0 (the CLI exits zero on that condition), got %d", result.Summary.Errors)
		}

		// Control. Without it this test passes on a tree where the check
		// does not exist at all, which asserts nothing about scoping. The
		// control asserts only what AC-21 already specifies: a within-spec
		// duplicate produces one error diagnostic. Here it also pins the
		// attribution, which is the point of AC-25.
		specBDup := dupSpec("payment-refund", 1, "AC-01", "AC-02", "AC-01")
		gDup := makeGraph(
			map[string]*resolver.SpecNode{
				"payment-charge": {Spec: specA, File: "payment-charge.spec.yaml"},
				"payment-refund": {Spec: specBDup, File: "payment-refund.spec.yaml"},
			},
			[]resolver.SpecEdge{{From: "payment-refund", To: "payment-charge", Relationship: "requires"}},
		)

		controlDiags := dupDiagnostics(CheckSpecs(gDup, nil))
		if len(controlDiags) != 1 {
			t.Fatalf("control: expected 1 duplicate_ac_id diagnostic when payment-refund repeats AC-01, got %d", len(controlDiags))
		}
		if controlDiags[0].SpecID != "payment-refund" {
			t.Errorf("control: expected the diagnostic to name payment-refund, got %q", controlDiags[0].SpecID)
		}
	})
}

// @spec spec-check
// @ac AC-26
func TestDuplicateACIDComparesExactStrings(t *testing.T) {
	t.Run("spec-check/AC-26 duplicate ac id compares exact strings", func(t *testing.T) {
		spec := dupSpec("payment-charge", 1, "AC-01", "AC-001")
		g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

		result := CheckSpecs(g, nil)

		if n := len(dupDiagnostics(result)); n != 0 {
			t.Errorf("expected 0 duplicate_ac_id diagnostics for AC-01 and AC-001, got %d", n)
		}
		if result.Summary.Errors != 0 {
			t.Errorf("expected Summary.Errors == 0 (the CLI exits zero on that condition), got %d", result.Summary.Errors)
		}

		// Control, for the same reason as AC-25: a check that never fires
		// would satisfy the assertion above. The control asserts only the
		// AC-21 behavior, so exactness is what separates the two arms.
		exact := dupSpec("payment-charge", 1, "AC-01", "AC-01")
		gExact := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: exact, File: "payment-charge.spec.yaml"}}, nil)
		if n := len(dupDiagnostics(CheckSpecs(gExact, nil))); n != 1 {
			t.Errorf("control: expected 1 duplicate_ac_id diagnostic for AC-01 declared twice, got %d", n)
		}
	})
}

// @spec spec-check
// @ac AC-27
func TestDuplicateACIDRunsInDefaultCheckPass(t *testing.T) {
	t.Run("spec-check/AC-27 duplicate ac id runs in default check pass", func(t *testing.T) {
		spec := dupSpec("payment-charge", 1, "AC-01", "AC-02", "AC-01")
		g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

		// CheckSpecs with nil options is the default check pass. The CLI
		// calls it unconditionally; --test only appends the output of
		// CheckTestAnnotations and CheckUnreachableAnnotations, and
		// --strict only sets CheckOptions.Strict. An implementation bolted
		// onto the test-file scanner produces nothing here.
		result := CheckSpecs(g, nil)

		diags := dupDiagnostics(result)
		if len(diags) != 1 {
			t.Fatalf("expected 1 duplicate_ac_id diagnostic from the default pass with no flags, got %d", len(diags))
		}
		if diags[0].Severity != "error" {
			t.Errorf("expected severity %q, got %q", "error", diags[0].Severity)
		}
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

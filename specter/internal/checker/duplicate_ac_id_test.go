// @spec spec-check
//
// Tests for C-13: duplicate acceptance criterion IDs within a single spec.
//
// Three notes on how these tests assert, all forced by the spec text:
//
//  1. C-13 requires the message to name four facts: the spec id, the
//     duplicated AC id, the occurrence count, and the position of each
//     occurrence. All four are asserted against Message. The spec id is
//     asserted twice, once against Message and once against the SpecID
//     field, because AC-21 lists `spec_id` plainly while C-13 puts it in
//     the message. Both readings are satisfied.
//
//  2. Fixture fields vary independently. A criterion's id, its description,
//     and its constraint references are set separately, because a suite in
//     which they move together cannot tell which of the three a duplicate is
//     keyed on. See dupACFixture.
//
//  3. The checker package is pure and cannot observe a process exit code.
//     `exit_code_nonzero` is asserted as Summary.Errors > 0 and
//     `exit_code_zero` as Summary.Errors == 0. That is the exact rule the
//     CLI applies: cmd/specter/main.go returns errSilent if and only if
//     result.Summary.Errors > 0.
package checker

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/resolver"
	"github.com/Hanalyx/specter/internal/schema"
)

// dupACFixture is one acceptance criterion in a fixture spec. The id, the
// description, and the references are given separately on purpose. If every
// criterion carried a description derived from its id and the same references
// as its neighbors, then a duplicate id, a duplicate description, and a
// duplicate reference list would always occur together, and no assertion below
// could tell which one the check reads.
//
// Every fixture therefore carries at least one of two shapes:
//
//   - Two criteria sharing an id, with different descriptions and different
//     references. This is the realistic defect: an author copies a criterion
//     block, edits the description and the references, and forgets the id. A
//     check keyed on the description or the references finds nothing here.
//
//   - Two criteria with distinct ids, sharing a description and references. A
//     check keyed on the description or the references reports a duplicate
//     here, where C-13 requires none.
//
// Descriptions carry no digits and no AC ids, so that an implementation which
// quotes the description in its message cannot donate numerals or ids to the
// message assertions.
type dupACFixture struct {
	id   string
	desc string
	refs []string
}

func dupAC(id, desc string, refs ...string) dupACFixture {
	return dupACFixture{id: id, desc: desc, refs: refs}
}

// dupSpec builds a spec whose acceptance criteria are the given fixtures in
// declaration order. The constraint list is the union of everything the
// criteria reference, in first-seen order, so no constraint is orphaned and no
// reference dangles. Either would add diagnostics and pollute the error count.
func dupSpec(id string, tier int, acs ...dupACFixture) schema.SpecAST {
	spec := schema.SpecAST{
		ID: id, Version: "1.0.0", Status: "approved", Tier: tier,
		Context:   schema.SpecContext{System: "test"},
		Objective: schema.SpecObjective{Summary: "test"},
	}
	declared := map[string]bool{}
	for _, a := range acs {
		for _, ref := range a.refs {
			if !declared[ref] {
				declared[ref] = true
				spec.Constraints = append(spec.Constraints, schema.Constraint{
					ID:          ref,
					Description: "a constraint with no keyword the conflict scanner reads",
				})
			}
		}
		spec.AcceptanceCriteria = append(spec.AcceptanceCriteria, schema.AcceptanceCriterion{
			ID:                    a.id,
			Description:           a.desc,
			ReferencesConstraints: a.refs,
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

// dupDiagnosticFor returns the duplicate_ac_id diagnostic whose message names
// the given AC id, or nil. The match is on whole ids, so "AC-01" does not
// match a message that names only "AC-001".
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

// dupIDPattern matches any AC or constraint id, so ids can be removed before the
// message is read for numerals. Without this, "AC-01" donates a 1.
var dupIDPattern = regexp.MustCompile(`\b(?:AC|C)-\d+\b`)

// dupTokenPattern splits a message into words and numbers, discarding
// punctuation, so a number's neighbors can be read whatever separates them.
var dupTokenPattern = regexp.MustCompile(`[A-Za-z]+|\d+`)

func acIDMentioned(message, acID string) bool {
	for _, found := range acIDPattern.FindAllString(message, -1) {
		if found == acID {
			return true
		}
	}
	return false
}

// dupCountCues are the word stems that mark a number as a count of occurrences.
// Matching is by prefix and case-insensitive, so "declared", "declares", "2
// total", "occurrences", and "x2" all read as count words. The list is loose
// on purpose: it is the one wording commitment these tests make, and a
// too-narrow list would fail a correct implementation over phrasing.
var dupCountCues = []string{
	"time", "occurrence", "occur", "declar", "defin", "appear",
	"repeat", "count", "total", "instance", "entr", "dup", "x",
}

func dupIsCountCue(token string) bool {
	lower := strings.ToLower(token)
	if lower == "" {
		return false
	}
	for _, stem := range dupCountCues {
		if strings.HasPrefix(lower, stem) {
			return true
		}
	}
	return false
}

// dupNumeral is one number in a message, with the words on either side of it.
type dupNumeral struct {
	value int
	prev  string
	next  string
}

func dupNumerals(message string) []dupNumeral {
	tokens := dupTokenPattern.FindAllString(dupIDPattern.ReplaceAllString(message, " "), -1)
	var out []dupNumeral
	for i, tok := range tokens {
		value, err := strconv.Atoi(tok)
		if err != nil {
			continue
		}
		n := dupNumeral{value: value}
		if i > 0 {
			n.prev = tokens[i-1]
		}
		if i+1 < len(tokens) {
			n.next = tokens[i+1]
		}
		out = append(out, n)
	}
	return out
}

// assertDupNamesCountAndPositions checks the two numeric facts C-13 requires the
// message to state, and checks them as separate facts with separate roles: the
// number of occurrences, and the 1-based position of each occurrence in
// declaration order.
//
// The roles have to be bound to the numbers, and nothing read off the bag of
// numbers can do it. A message reading "is declared 1, 3 times, at positions 2"
// carries exactly the numbers of the correct message, so no check on
// multiplicity separates them. Naming the positions before the count is also
// legitimate wording ("declared at 1, 3 (2 occurrences)"), so no check on order
// separates them either. The smallest thing that does is the word next to the
// number. So:
//
//   - The positions must appear as one run of numbers, in declaration order.
//   - The count must appear as a number outside that run, with a count word
//     immediately beside it.
//
// Everything else about the wording stays free.
func assertDupNamesCountAndPositions(t *testing.T, message string, count int, positions []int) {
	t.Helper()

	nums := dupNumerals(message)

	runStart := -1
	for i := 0; i+len(positions) <= len(nums); i++ {
		match := true
		for j, want := range positions {
			if nums[i+j].value != want {
				match = false
				break
			}
		}
		if match {
			runStart = i
			break
		}
	}
	if runStart < 0 {
		t.Errorf("message %q does not name the occurrence positions %v in declaration order",
			message, positions)
	}

	counted := false
	for i, n := range nums {
		if runStart >= 0 && i >= runStart && i < runStart+len(positions) {
			continue // part of the position list, so it is not the count
		}
		if n.value == count && (dupIsCountCue(n.prev) || dupIsCountCue(n.next)) {
			counted = true
			break
		}
	}
	if !counted {
		t.Errorf("message %q does not name the occurrence count %d in a count role: expected the number %d beside a word like \"times\" or \"occurrences\", outside the list of positions",
			message, count, count)
	}
}

// assertDupNamesSpecID checks both places the spec id is required. C-13 puts it
// in the message; AC-21 lists `spec_id` as an output field.
func assertDupNamesSpecID(t *testing.T, d CheckDiagnostic, specID string) {
	t.Helper()
	if d.SpecID != specID {
		t.Errorf("expected SpecID field %q, got %q", specID, d.SpecID)
	}
	if !strings.Contains(d.Message, specID) {
		t.Errorf("C-13 requires the message to name the spec id %q, got %q", specID, d.Message)
	}
}

// @spec spec-check
// @ac AC-21
func TestDuplicateACIDEmitsOneErrorDiagnostic(t *testing.T) {
	t.Run("spec-check/AC-21 duplicate ac id emits one error diagnostic", func(t *testing.T) {
		// Positions 1 and 3 share the id and share nothing else. Position 3
		// shares its references with position 2 instead, so a check keyed on
		// the references reports the pair (2, 3) and a check keyed on the
		// description reports nothing.
		spec := dupSpec("payment-charge", 1,
			dupAC("AC-01", "a valid card is charged and the intent is captured", "C-01"),
			dupAC("AC-02", "an expired card is rejected before capture", "C-02"),
			dupAC("AC-01", "a second charge against the same intent is rejected", "C-02"),
		)
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
		assertDupNamesSpecID(t, d, "payment-charge")
		if !acIDMentioned(d.Message, "AC-01") {
			t.Errorf("expected message to name the duplicated id AC-01, got %q", d.Message)
		}
		assertDupNamesCountAndPositions(t, d.Message, 2, []int{1, 3})
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-22
func TestDuplicateACIDSeverityIgnoresTier(t *testing.T) {
	t.Run("spec-check/AC-22 duplicate ac id severity ignores tier", func(t *testing.T) {
		// A second spec id, so a message that hardcodes payment-charge fails
		// here even though it satisfies AC-21.
		spec := dupSpec("ledger-export", 3,
			dupAC("AC-01", "an export of a closed period is written once", "C-01"),
			dupAC("AC-01", "an export of an open period is refused", "C-02"),
		)
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
			assertDupNamesSpecID(t, d, "ledger-export")
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
		// The three AC-01 criteria differ in description and in references.
		// C-01 alone is referenced at positions 1 and 2, so a check keyed on
		// the references reports the count 2 at positions 1 and 2, which the
		// count and position assertions both reject.
		spec := dupSpec("payment-charge", 1,
			dupAC("AC-01", "a valid card is charged and the intent is captured", "C-01"),
			dupAC("AC-02", "an expired card is rejected before capture", "C-01"),
			dupAC("AC-01", "a second charge against the same intent is rejected", "C-02"),
			dupAC("AC-01", "a charge after a void is refused", "C-01", "C-02"),
		)
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
		assertDupNamesSpecID(t, d, "payment-charge")
		if !acIDMentioned(d.Message, "AC-01") {
			t.Errorf("expected message to name the duplicated id AC-01, got %q", d.Message)
		}
		// The count 3 and the position 3 are separate facts here, which is
		// why they are asserted in separate roles.
		assertDupNamesCountAndPositions(t, d.Message, 3, []int{1, 3, 4})
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-24
func TestDuplicateACIDTwoDuplicatedIDsTwoDiagnostics(t *testing.T) {
	t.Run("spec-check/AC-24 duplicate ac id two duplicated ids two diagnostics", func(t *testing.T) {
		// AC-03 is declared once, and it carries the description and the
		// references of the criterion at position 1. So a check keyed on
		// either of those reports a pair that C-13 says is not a duplicate,
		// and reports one diagnostic where two are required.
		const sharedDesc = "a valid card is charged and the intent is captured"
		spec := dupSpec("payment-charge", 1,
			dupAC("AC-01", sharedDesc, "C-01"),
			dupAC("AC-02", "an expired card is rejected before capture", "C-02"),
			dupAC("AC-01", "a second charge against the same intent is rejected", "C-02"),
			dupAC("AC-02", "a void after capture is refused", "C-01"),
			dupAC("AC-03", sharedDesc, "C-01"),
		)
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
			assertDupNamesSpecID(t, d, "payment-charge")
		}

		// The spec names the two diagnostics as diagnostic_1 and
		// diagnostic_2, but neither AC-24 nor C-13 states an order
		// between them, so each is located by the id its message names.
		first := dupDiagnosticFor(result, "AC-01")
		if first == nil {
			t.Fatal("expected a duplicate_ac_id diagnostic naming AC-01")
		}
		assertDupNamesCountAndPositions(t, first.Message, 2, []int{1, 3})

		second := dupDiagnosticFor(result, "AC-02")
		if second == nil {
			t.Fatal("expected a duplicate_ac_id diagnostic naming AC-02")
		}
		assertDupNamesCountAndPositions(t, second.Message, 2, []int{2, 4})

		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

// @spec spec-check
// @ac AC-25
func TestDuplicateACIDScopedToOneSpec(t *testing.T) {
	t.Run("spec-check/AC-25 duplicate ac id scoped to one spec", func(t *testing.T) {
		// Within each spec the two ids are distinct but the description and
		// the references are identical, so a check keyed on either of those
		// fires inside a single spec and fails the assertion below.
		const sharedDesc = "the ledger entry is written once per settled charge"
		specA := dupSpec("payment-charge", 1,
			dupAC("AC-01", sharedDesc, "C-01"),
			dupAC("AC-02", sharedDesc, "C-01"),
		)
		specB := dupSpec("payment-refund", 1,
			dupAC("AC-01", sharedDesc, "C-01"),
			dupAC("AC-02", sharedDesc, "C-01"),
		)
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
		// attribution, which is the point of AC-25. The repeated id carries
		// its own description and references, so the control also fails a
		// check keyed on either of those.
		specBDup := dupSpec("payment-refund", 1,
			dupAC("AC-01", sharedDesc, "C-01"),
			dupAC("AC-02", sharedDesc, "C-01"),
			dupAC("AC-01", "a refund of a settled charge reverses the ledger entry", "C-02"),
		)
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
		assertDupNamesSpecID(t, controlDiags[0], "payment-refund")
	})
}

// @spec spec-check
// @ac AC-26
func TestDuplicateACIDComparesExactStrings(t *testing.T) {
	t.Run("spec-check/AC-26 duplicate ac id compares exact strings", func(t *testing.T) {
		// AC-26 names one pair, AC-01 and AC-001. A comparison rule that
		// separates only that pair still collides on others, so the pairs
		// below cover the ways a rule can be inexact: zero padding, letter
		// case, and prefix matching. Each pair is two distinct ids under
		// exact string equality, so each requires no diagnostic. Both ids in
		// a pair carry the same description and the same references, so a
		// check keyed on either of those also fails here.
		const sharedDesc = "a valid card is charged and the intent is captured"
		pairs := []struct {
			name       string
			first      string
			second     string
			fromTheACs bool
		}{
			{"zero padded, the pair AC-26 names", "AC-01", "AC-001", true},
			{"unpadded against padded", "AC-1", "AC-01", false},
			{"same letters, different case", "AC-01", "ac-01", false},
			{"one id is a prefix of the other", "AC-1", "AC-10", false},
		}

		for _, pair := range pairs {
			t.Run(pair.name, func(t *testing.T) {
				spec := dupSpec("payment-charge", 1,
					dupAC(pair.first, sharedDesc, "C-01"),
					dupAC(pair.second, sharedDesc, "C-01"),
				)
				g := makeGraph(map[string]*resolver.SpecNode{"payment-charge": {Spec: spec, File: "payment-charge.spec.yaml"}}, nil)

				result := CheckSpecs(g, nil)

				if diags := dupDiagnostics(result); len(diags) != 0 {
					t.Errorf("expected 0 duplicate_ac_id diagnostics for %s and %s, got %d, first message %q",
						pair.first, pair.second, len(diags), diags[0].Message)
				}
				// Only the pair AC-26 names carries the exit code claim.
				// The others are this test broadening the rule, not the
				// spec, so they assert the diagnostic and nothing else.
				if pair.fromTheACs && result.Summary.Errors != 0 {
					t.Errorf("expected Summary.Errors == 0 (the CLI exits zero on that condition), got %d", result.Summary.Errors)
				}
			})
		}

		// Control, for the same reason as AC-25: a check that never fires
		// would satisfy the assertions above. The control asserts only the
		// AC-21 behavior, so exactness is what separates the two arms. The
		// repeated id carries its own description and references, so a check
		// keyed on either of those fails the control.
		exact := dupSpec("payment-charge", 1,
			dupAC("AC-01", sharedDesc, "C-01"),
			dupAC("AC-01", "a second charge against the same intent is rejected", "C-02"),
		)
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
		spec := dupSpec("payment-charge", 1,
			dupAC("AC-01", "a valid card is charged and the intent is captured", "C-01"),
			dupAC("AC-02", "an expired card is rejected before capture", "C-02"),
			dupAC("AC-01", "a second charge against the same intent is rejected", "C-02"),
		)
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
		assertDupNamesSpecID(t, diags[0], "payment-charge")
		assertDupNamesCountAndPositions(t, diags[0].Message, 2, []int{1, 3})
		if result.Summary.Errors == 0 {
			t.Error("expected Summary.Errors > 0 (the CLI exits non-zero on that condition)")
		}
	})
}

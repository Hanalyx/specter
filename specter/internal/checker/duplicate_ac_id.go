// Duplicate acceptance criterion id detection per spec-check C-13.
//
// An acceptance criterion id is the join key between a spec and its
// tests. A spec that declares the same id twice passes parse, because
// JSON Schema cannot express uniqueness over a property of array
// items, and one `@ac AC-01` annotation then credits every criterion
// carrying that id. The spec reports coverage it has not earned, so
// the rule belongs here.
//
// Three properties of this check, all of them deliberate:
//
//   - Severity is error at every tier. Tier routes orphan constraint
//     severity, but no tier makes two criteria answering to one key
//     correct.
//   - One diagnostic per duplicated id, not per occurrence. This is
//     the opposite of the C-09 rule for repeated `@ac` mentions in
//     test files, where each mention is a separate broken reference.
//     Here the occurrences are one broken declaration.
//   - Ids are compared as exact strings, so `AC-01` and `AC-001` are
//     distinct.
//
// The check reads only the spec, so it runs in the default `check`
// pass with no flag, and `sync` inherits it.
//
// @spec spec-check
package checker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Hanalyx/specter/internal/schema"
)

// checkDuplicateACIDs finds acceptance criterion ids declared more than
// once in a single spec, per C-13.
//
// Diagnostics come back in first-declaration order, so a spec whose
// AC-01 and AC-02 are both duplicated reports AC-01 first. Position
// numbers are 1-based indexes into the declaration order, which is
// what an author counts when reading the file.
func checkDuplicateACIDs(spec *schema.SpecAST) []CheckDiagnostic {
	positions := make(map[string][]int, len(spec.AcceptanceCriteria))
	var order []string
	for i, ac := range spec.AcceptanceCriteria {
		if _, seen := positions[ac.ID]; !seen {
			order = append(order, ac.ID)
		}
		positions[ac.ID] = append(positions[ac.ID], i+1)
	}

	var diagnostics []CheckDiagnostic
	for _, acID := range order {
		at := positions[acID]
		if len(at) < 2 {
			continue
		}
		diagnostics = append(diagnostics, CheckDiagnostic{
			Kind:     "duplicate_ac_id",
			Severity: "error",
			Message: fmt.Sprintf("Acceptance criterion %s in %q is declared %d times, at positions %s. Acceptance criterion ids must be unique within a spec.",
				acID, spec.ID, len(at), joinPositions(at)),
			SpecID: spec.ID,
		})
	}

	return diagnostics
}

// joinPositions renders 1-based occurrence positions as "1, 3, 4".
func joinPositions(positions []int) string {
	parts := make([]string, len(positions))
	for i, p := range positions {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

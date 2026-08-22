// annotation_conflict.go — detects a manifest that declares both
// `settings.strictness` and a `settings.annotation` block.
//
// @spec spec-manifest
package manifest

import "fmt"

// AnnotationStrictnessConflictWarning is emitted when a manifest declares both
// `settings.strictness` and a `settings.annotation` block. The annotation block
// takes precedence and the declared strictness is ignored.
type AnnotationStrictnessConflictWarning struct {
	DeclaredStrictness string
	Message            string
}

// CheckAnnotationStrictnessConflict returns the warning for a manifest that
// declares both keys, or nil.
//
// C-34(a): the trigger is declaredness of both keys, not their values. C-24
// rewrites an absent `strictness` to the default `threshold` at parse time, so
// a detector reading Settings.Strictness cannot tell an absent key from an
// explicit `threshold`. Declaredness comes from the raw key set, recorded by
// Settings.UnmarshalYAML.
//
// C-34(b): pure, no I/O, per C-10. The precedent is CheckTierConflicts (C-14).
// The caller prints Message to stderr.
//
// The rule reads manifest keys only (C-34(d) and AC-59). A `--strictness` flag
// is not an input here, so it cannot trigger the warning.
//
// C-34
func CheckAnnotationStrictnessConflict(m *Manifest) *AnnotationStrictnessConflictWarning {
	if m == nil {
		return nil
	}
	if !m.Settings.annotationDeclared || !m.Settings.strictnessDeclared {
		return nil
	}
	declared := m.Settings.Strictness
	return &AnnotationStrictnessConflictWarning{
		DeclaredStrictness: declared,
		Message: fmt.Sprintf(
			"settings.strictness: %s is ignored because settings.annotation is declared; "+
				"the annotation block takes precedence. Remove one of the two keys.",
			declared,
		),
	}
}

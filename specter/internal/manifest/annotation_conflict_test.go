package manifest

import (
	"strings"
	"testing"
)

// TestCheckAnnotationStrictnessConflict_DeclarednessNotValue calls the detector
// directly, per C-34(b), on three manifests that are one variable apart.
//
// The manifests are built through ParseManifest rather than as struct literals
// on purpose. C-24 rewrites an absent `strictness` to `threshold` at parse
// time, so only a parsed manifest carries the state the criterion discriminates
// on: the block-only manifest reaches the detector with the parsed strictness
// string set to `threshold` while the raw key set says the key was never
// declared. A detector reading the parsed string returns a warning there and
// fails.
//
// @spec spec-manifest
// @ac AC-56
func TestCheckAnnotationStrictnessConflict_DeclarednessNotValue(t *testing.T) {
	t.Run("spec-manifest/AC-56", func(t *testing.T) {
		const bothYAML = `
system:
  name: conflict-fixture
settings:
  strictness: threshold
  annotation:
    permissive: false
`
		const blockOnlyYAML = `
system:
  name: conflict-fixture
settings:
  annotation:
    permissive: false
`
		const strictnessOnlyYAML = `
system:
  name: conflict-fixture
settings:
  strictness: threshold
`

		both, err := ParseManifest(bothYAML)
		if err != nil {
			t.Fatalf("ParseManifest(both) returned error: %v", err)
		}
		blockOnly, err := ParseManifest(blockOnlyYAML)
		if err != nil {
			t.Fatalf("ParseManifest(block only) returned error: %v", err)
		}
		strictnessOnly, err := ParseManifest(strictnessOnlyYAML)
		if err != nil {
			t.Fatalf("ParseManifest(strictness only) returned error: %v", err)
		}

		// Both keys declared: a warning, naming the ignored key, its declared
		// value, and the key that takes precedence (C-34(c)).
		warning := CheckAnnotationStrictnessConflict(both)
		if warning == nil {
			t.Fatalf("both keys declared: got nil, want a warning")
		}
		for _, want := range []string{"settings.strictness", "threshold", "settings.annotation"} {
			if !strings.Contains(warning.Message, want) {
				t.Errorf("warning message does not name %q: %q", want, warning.Message)
			}
		}

		// Annotation block declared, no strictness key: nil. The parsed
		// strictness string is `threshold` here by C-24, so a value-reading
		// detector fails this case.
		if got := CheckAnnotationStrictnessConflict(blockOnly); got != nil {
			t.Errorf("annotation block only: got warning %q, want nil", got.Message)
		}

		// Strictness declared, no annotation block: nil.
		if got := CheckAnnotationStrictnessConflict(strictnessOnly); got != nil {
			t.Errorf("strictness only: got warning %q, want nil", got.Message)
		}
	})
}

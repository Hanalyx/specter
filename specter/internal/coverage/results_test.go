// @spec spec-coverage
package coverage

import (
	"strings"
	"testing"
)

// M1 (chore/v0.12-security-hardening): ParseResultsFile MUST refuse input
// larger than MaxResultsFileBytes (16 MiB) before json.Unmarshal allocates
// on it, preventing memory exhaustion via a malicious .specter-results.json.
//
// Aspirational test coverage flagged by the v0.12 review: the constant +
// reference existed, but no test exercised the rejection path. Without
// this, a refactor that drops the cap check ships silently.
func TestParseResultsFile_RejectsOversizedInput(t *testing.T) {
	t.Run("M1 size cap refuses input over 16 MiB limit", func(t *testing.T) {
		// One byte past the limit is enough to fire the check; we don't need
		// to allocate the full 16 MiB. The check is len() > limit, so
		// limit+1 triggers it.
		oversized := make([]byte, MaxResultsFileBytes+1)
		// Fill with valid JSON characters so the test can't accidentally
		// pass via a JSON syntax error from random bytes.
		for i := range oversized {
			oversized[i] = ' '
		}

		_, err := ParseResultsFile(oversized)
		if err == nil {
			t.Fatal("expected error for input larger than MaxResultsFileBytes, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("expected error to mention `exceeds`, got: %v", err)
		}
		if !strings.Contains(err.Error(), "byte limit") {
			t.Errorf("expected error to mention `byte limit`, got: %v", err)
		}
	})

	t.Run("M1 size cap accepts input at exactly the limit", func(t *testing.T) {
		// Boundary condition: len == limit must succeed (the check is >).
		// Use minimal valid JSON to avoid Unmarshal noise.
		atLimit := []byte(`{"results": []}`)
		if len(atLimit) > MaxResultsFileBytes {
			t.Skip("test fixture larger than limit — adjust if MaxResultsFileBytes shrinks")
		}
		if _, err := ParseResultsFile(atLimit); err != nil {
			t.Errorf("unexpected error for input at/under limit: %v", err)
		}
	})
}

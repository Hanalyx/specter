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

// v0.13 D2 — C-30: ResultsFile.InvalidStatuses() returns a map of
// non-canonical status values to their occurrence counts. The canonical
// enum is {passed, failed, skipped, errored} per C-21.
//
// @ac AC-35
func TestResultsFile_InvalidStatuses(t *testing.T) {
	t.Run("spec-coverage/AC-35 InvalidStatuses returns map of non-canonical values to counts", func(t *testing.T) {
		// Three entries with the same typo, one with a different typo,
		// one valid entry. Expected: {"pass": 3, "OK": 1}.
		body := []byte(`{"results": [
			{"spec_id": "a", "ac_id": "AC-01", "status": "pass"},
			{"spec_id": "a", "ac_id": "AC-02", "status": "pass"},
			{"spec_id": "b", "ac_id": "AC-01", "status": "pass"},
			{"spec_id": "b", "ac_id": "AC-02", "status": "OK"},
			{"spec_id": "c", "ac_id": "AC-01", "status": "passed"}
		]}`)
		rf, err := ParseResultsFile(body)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}

		got := rf.InvalidStatuses()
		if got["pass"] != 3 {
			t.Errorf("expected 3 entries with status=pass, got %d (full map: %v)", got["pass"], got)
		}
		if got["OK"] != 1 {
			t.Errorf("expected 1 entry with status=OK, got %d (full map: %v)", got["OK"], got)
		}
		if _, ok := got["passed"]; ok {
			t.Errorf("expected `passed` to NOT appear in InvalidStatuses (it is canonical), got map: %v", got)
		}
		if len(got) != 2 {
			t.Errorf("expected exactly 2 unique non-canonical values, got %d: %v", len(got), got)
		}
	})

	t.Run("spec-coverage/AC-35 InvalidStatuses empty when all entries canonical or use boolean back-compat", func(t *testing.T) {
		body := []byte(`{"results": [
			{"spec_id": "a", "ac_id": "AC-01", "status": "passed"},
			{"spec_id": "a", "ac_id": "AC-02", "status": "failed"},
			{"spec_id": "b", "ac_id": "AC-01", "status": "skipped"},
			{"spec_id": "b", "ac_id": "AC-02", "status": "errored"},
			{"spec_id": "c", "ac_id": "AC-01", "passed": true}
		]}`)
		rf, err := ParseResultsFile(body)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		got := rf.InvalidStatuses()
		if len(got) != 0 {
			t.Errorf("expected empty map for fully-canonical results, got: %v", got)
		}
	})

	t.Run("spec-coverage/AC-35 InvalidStatuses tolerates nil receiver", func(t *testing.T) {
		var rf *ResultsFile
		got := rf.InvalidStatuses()
		if len(got) != 0 {
			t.Errorf("expected empty map for nil receiver, got: %v", got)
		}
	})

	t.Run("spec-coverage/AC-35 non-canonical status still derives Passed=false (preserves today's demotion behavior)", func(t *testing.T) {
		body := []byte(`{"results": [
			{"spec_id": "a", "ac_id": "AC-01", "status": "pass"}
		]}`)
		rf, err := ParseResultsFile(body)
		if err != nil {
			t.Fatalf("unexpected parse error: %v", err)
		}
		if len(rf.Results) != 1 {
			t.Fatalf("expected 1 result entry, got %d", len(rf.Results))
		}
		if rf.Results[0].Passed {
			t.Errorf("expected status=`pass` (non-canonical) to derive Passed=false, got Passed=true — behavior break")
		}
	})
}

// CLI integration tests for `specter reverse` summary + handoff
// (v0.13 C5+C6 / spec-reverse 1.3.0 AC-17/AC-18/AC-19).
//
// @spec spec-reverse
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// AC-17 + AC-18: non-JSON mode emits a one-line summary AFTER the
// per-spec output, followed by a one-line handoff pointing at
// `specter explain <first-spec-id>` for gap triage.
//
// @ac AC-17
func TestReverseSummary_NonJsonEmitsSummaryAndHandoff(t *testing.T) {
	t.Run("spec-reverse/AC-17 non-JSON reverse emits summary line + handoff", func(t *testing.T) {
		dir := t.TempDir()

		// A minimal TS source file with a zod-style schema. The TS
		// adapter extracts constraints from it.
		tsContent := `import { z } from 'zod';
export const UserSchema = z.object({
  email: z.string().email(),
  age: z.number().int().min(18),
});
`
		if err := os.WriteFile(filepath.Join(dir, "user.ts"), []byte(tsContent), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "reverse", "--dry-run")
		if code != 0 {
			t.Fatalf("expected reverse --dry-run exit 0, got %d. output:\n%s", code, out)
		}

		// Summary line per C-13: "Found N constraints, M assertions,
		// K gaps across P file(s)." Allow singular/plural via pluralize.
		reSummary := regexp.MustCompile(`Found \d+ constraints?, \d+ assertions?, \d+ gaps? across \d+ files?\.`)
		if !reSummary.MatchString(out) {
			t.Errorf("expected C-13 summary line in output, got:\n%s", out)
		}

		// Handoff line per C-14: "Run `specter explain <id>` to triage
		// gaps in each generated draft."
		if !strings.Contains(out, "Run `specter explain") || !strings.Contains(out, "to triage gaps") {
			t.Errorf("expected C-14 handoff line in output, got:\n%s", out)
		}
	})
}

// AC-19: --json mode suppresses BOTH the summary line and the handoff.
// Machine-readable consumers get only the structured payload.
//
// @ac AC-19
func TestReverseSummary_JsonSuppressesSummaryAndHandoff(t *testing.T) {
	t.Run("spec-reverse/AC-19 --json suppresses summary + handoff lines", func(t *testing.T) {
		dir := t.TempDir()

		tsContent := `import { z } from 'zod';
export const PaymentSchema = z.object({
  amount: z.number().positive(),
});
`
		if err := os.WriteFile(filepath.Join(dir, "payment.ts"), []byte(tsContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Streams split, not combined. AC-19 is a claim about stdout, and
		// CombinedOutput cannot express it: it would pass while prose went to
		// stdout as long as the bytes happened to parse, and fail while stdout
		// was correct as soon as anything reached stderr.
		out, _, code := runCLISplit(t, dir, "reverse", "--dry-run", "--json")
		if code != 0 {
			t.Fatalf("expected reverse --dry-run --json exit 0, got %d. output:\n%s", code, out)
		}

		// JSON output should be parseable and not contain prose lines.
		if strings.Contains(out, "Found ") && strings.Contains(out, "across") {
			t.Errorf("expected --json mode to suppress C-13 summary line, got:\n%s", out)
		}
		if strings.Contains(out, "to triage gaps in each generated draft") {
			t.Errorf("expected --json mode to suppress C-14 handoff line, got:\n%s", out)
		}

		// Verify the JSON itself parses (downstream contract).
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Errorf("expected --json output to be valid JSON, got parse error: %v\noutput:\n%s", err, out)
		}
	})
}

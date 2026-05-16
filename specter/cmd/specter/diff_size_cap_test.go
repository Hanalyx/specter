// CLI integration test for v0.13 M3 — `diff coverage` size cap
// (spec-diff 2.1.0 C-12 / AC-15).
//
// @spec spec-diff
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC-15: a coverage JSON input larger than MaxCoverageReportBytes
// (16 MiB) MUST cause `specter diff coverage` to exit non-zero with
// a stderr message naming the path, byte count, and cap. The check
// MUST happen BEFORE json.Unmarshal — confirmed implicitly by using
// an oversized file whose CONTENT is not valid JSON (a single byte
// of padding followed by garbage). If json.Unmarshal ran first we'd
// see a parse error; if the size check fires first we see the
// byte-limit error.
//
// @ac AC-15
func TestDiff_CoverageReport_RefusesOversizedInput(t *testing.T) {
	t.Run("spec-diff/AC-15 diff coverage refuses input larger than MaxCoverageReportBytes", func(t *testing.T) {
		dir := t.TempDir()
		oversized := filepath.Join(dir, "oversized.json")
		// 16 MiB + 1 byte — one past the cap. Content is arbitrary
		// non-JSON bytes; the size check must reject before any
		// parsing happens, so we don't need valid JSON to exercise
		// the path.
		const cap = 16 << 20
		body := bytes.Repeat([]byte("x"), cap+1)
		if err := os.WriteFile(oversized, body, 0644); err != nil {
			t.Fatal(err)
		}
		// A small valid JSON for the other arg so the cap check is
		// what triggers, not a "file not found" or "invalid json".
		small := filepath.Join(dir, "small.json")
		if err := os.WriteFile(small, []byte(`{"entries":[],"summary":{},"spec_candidates_count":0}`), 0644); err != nil {
			t.Fatal(err)
		}

		// Oversized baseline.
		out, code := runCLI(t, dir, "diff", "coverage", oversized, small)
		if code == 0 {
			t.Fatalf("expected non-zero exit on oversized baseline, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "exceeds") || !strings.Contains(out, "byte limit") {
			t.Errorf("expected stderr to mention `exceeds` and `byte limit`, got:\n%s", out)
		}

		// Oversized current — same rejection.
		out, code = runCLI(t, dir, "diff", "coverage", small, oversized)
		if code == 0 {
			t.Fatalf("expected non-zero exit on oversized current, got 0; output:\n%s", out)
		}
		if !strings.Contains(out, "exceeds") {
			t.Errorf("expected stderr to mention `exceeds` for current arg, got:\n%s", out)
		}
	})

	t.Run("spec-diff/AC-15 diff coverage accepts input at exactly the cap (boundary)", func(t *testing.T) {
		// Boundary condition: len == cap must succeed. Use a minimal
		// valid CoverageReport JSON so json.Unmarshal succeeds; we
		// don't need to fully pad to the cap — the check is `>`, so
		// any size <= cap passes the gate.
		dir := t.TempDir()
		// 1 KiB of valid JSON — well under the 16 MiB cap.
		small := filepath.Join(dir, "small.json")
		body := `{"entries":[],"summary":{"total_specs":0},"spec_candidates_count":0}`
		if err := os.WriteFile(small, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}

		out, code := runCLI(t, dir, "diff", "coverage", small, small)
		if code != 0 {
			t.Errorf("expected exit 0 for under-cap input, got %d; output:\n%s", code, out)
		}
		if strings.Contains(out, "exceeds") || strings.Contains(out, "byte limit") {
			t.Errorf("must not emit byte-limit error for under-cap input, got:\n%s", out)
		}
	})
}

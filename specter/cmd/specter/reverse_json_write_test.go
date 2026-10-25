// CLI integration tests for C-18: --json selects the report format and does
// not decide whether files are written.
//
// @spec spec-reverse
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zodSource is a TypeScript file the adapter extracts constraints from.
const zodSource = `import { z } from 'zod';
export const UserSchema = z.object({
  email: z.string().email(),
  age: z.number().int().min(18),
});
`

// writeZodSource seeds a workspace with one extractable source file.
func writeZodSource(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "user.ts"), []byte(zodSource), 0644); err != nil {
		t.Fatal(err)
	}
}

// countSpecFiles returns how many .spec.yaml files exist under dir.
func countSpecFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".spec.yaml") {
			n++
		}
		return nil
	})
	return n
}

// @ac AC-26
// --json with an output directory writes the files, and stdout stays exactly
// one JSON document. Both halves matter: moving the encode below the write
// loop without routing the per-spec lines to stderr would satisfy the first
// assertion and break every JSON consumer.
func TestReverseJSON_WritesFilesAndKeepsStdoutPureJSON(t *testing.T) {
	t.Run("spec-reverse/AC-26 json writes files and stdout is pure json", func(t *testing.T) {
		dir := t.TempDir()
		writeZodSource(t, dir)
		outDir := filepath.Join(dir, "generated")

		stdout, _, code := runCLISplit(t, dir, "reverse", "--json", "-o", "generated")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d. stdout:\n%s", code, stdout)
		}

		if n := countSpecFiles(t, outDir); n == 0 {
			t.Errorf("--json -o wrote no spec files; C-18 requires --json to report, not to suppress writing")
		}

		// Exactly one JSON document and nothing else. Decode, then confirm the
		// stream is at EOF, which catches trailing lines a Contains check would
		// miss.
		dec := json.NewDecoder(strings.NewReader(stdout))
		var payload map[string]interface{}
		if err := dec.Decode(&payload); err != nil {
			t.Fatalf("stdout is not a JSON document: %v\nstdout:\n%s", err, stdout)
		}
		var trailing interface{}
		if err := dec.Decode(&trailing); err == nil {
			t.Errorf("stdout carries more than one JSON document; C-18 requires exactly one")
		}
		for _, line := range []string{"GENERATED ", "SKIPPED "} {
			if strings.Contains(stdout, line) {
				t.Errorf("per-spec line %q reached stdout under --json; it belongs on stderr", line)
			}
		}
		if _, ok := payload["summary"]; !ok {
			t.Errorf("JSON payload has no summary key; got keys %v", keysOf(payload))
		}
	})
}

// @ac AC-27
// --dry-run is the only flag that suppresses writing, and it still reports.
func TestReverseJSON_DryRunWritesNothingAndStillReports(t *testing.T) {
	t.Run("spec-reverse/AC-27 json dry run writes nothing and still reports", func(t *testing.T) {
		dir := t.TempDir()
		writeZodSource(t, dir)
		outDir := filepath.Join(dir, "generated")

		stdout, _, code := runCLISplit(t, dir, "reverse", "--json", "--dry-run", "-o", "generated")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d. stdout:\n%s", code, stdout)
		}
		if n := countSpecFiles(t, outDir); n != 0 {
			t.Errorf("--json --dry-run wrote %d spec file(s); --dry-run must suppress writing", n)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("--dry-run suppressed the report as well as the writes: %v\nstdout:\n%s", err, stdout)
		}

		// Differential, or this assertion is vacuous. While --json suppresses
		// writing on its own, "--dry-run wrote nothing" is trivially true and
		// says nothing about --dry-run. The same run without it must write.
		other := t.TempDir()
		writeZodSource(t, other)
		if _, _, c := runCLISplit(t, other, "reverse", "--json", "-o", "generated"); c != 0 {
			t.Fatalf("control run exited %d", c)
		}
		if n := countSpecFiles(t, filepath.Join(other, "generated")); n == 0 {
			t.Fatal("the same command without --dry-run also wrote nothing, so this test " +
				"cannot tell whether --dry-run is what suppressed the writes")
		}
	})
}

// @ac AC-28
// A write that cannot happen is an error, not a silent success. The output
// path is occupied by a plain file, so the directory cannot be created.
func TestReverseJSON_WriteFailureEmitsNoJSON(t *testing.T) {
	t.Run("spec-reverse/AC-28 json write failure emits no json", func(t *testing.T) {
		dir := t.TempDir()
		writeZodSource(t, dir)
		// A file, not a directory, at the output path.
		if err := os.WriteFile(filepath.Join(dir, "generated"), []byte("occupied"), 0644); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := runCLISplit(t, dir, "reverse", "--json", "-o", "generated")
		if code == 0 {
			t.Errorf("expected a non-zero exit when the output directory cannot be created, got 0")
		}
		if strings.TrimSpace(stdout) != "" {
			t.Errorf("a failed run emitted JSON on stdout, which a consumer would read as success:\n%s", stdout)
		}
		if strings.TrimSpace(stderr) == "" {
			t.Errorf("a failed run said nothing on stderr")
		}
	})
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// init_schema_version_test.go — CLI tests for spec-manifest C-28 (v1.9.0).
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hanalyx/specter/internal/manifest"
)

// @ac AC-42
// `specter init` (scaffold mode) writes `schema_version: 1` as the first
// non-empty, non-comment line of the emitted specter.yaml.
func TestInit_Scaffold_EmitsSchemaVersionAsFirstField(t *testing.T) {
	t.Run("spec-manifest/AC-42 init scaffold emits schema_version first", func(t *testing.T) {
		dir := t.TempDir()
		// init expects at least one .spec.yaml to scaffold against; provide
		// one so the command takes the scaffold path rather than the
		// no-specs guidance path.
		writeSpec(t, dir, "demo.spec.yaml", minimalValidSpec("demo", 3, "AC-01"))

		out, code := runCLI(t, dir, "init", "--name", "demo-system")
		if code != 0 {
			t.Fatalf("init exited %d, want 0; output:\n%s", code, out)
		}

		body, err := os.ReadFile(filepath.Join(dir, "specter.yaml"))
		if err != nil {
			t.Fatalf("read specter.yaml: %v", err)
		}

		// First non-empty, non-comment line must be `schema_version: 1`.
		var first string
		for _, line := range strings.Split(string(body), "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			first = t
			break
		}
		if first != "schema_version: 1" {
			t.Errorf("first non-comment line = %q, want %q\nfile:\n%s", first, "schema_version: 1", string(body))
		}

		m, perr := manifest.ParseManifest(string(body))
		if perr != nil {
			t.Fatalf("parse emitted manifest: %v", perr)
		}
		if m.SchemaVersion != 1 {
			t.Errorf("parsed SchemaVersion = %d, want 1", m.SchemaVersion)
		}
	})
}

// @ac AC-43
// `specter init --refresh` on an existing specter.yaml that already declares
// schema_version (and custom domains / settings / comments) leaves the
// schema_version line byte-unchanged regardless of value (1, 7, 42).
// AC-43 spec text: "leaves the schema_version line byte-unchanged" — value
// is not constrained to 1, so the test parameterizes over a non-default
// value to catch a regression where refresh rewrote the value (e.g., to 1).
func TestInit_Refresh_PreservesSchemaVersionAndCustomFields(t *testing.T) {
	cases := []struct {
		name              string
		schemaVersionLine string
	}{
		{"value=1 (default)", "schema_version: 1"},
		{"value=7 (non-default)", "schema_version: 7"},
		{"value=42 (arbitrary)", "schema_version: 42"},
	}
	for _, tc := range cases {
		t.Run("spec-manifest/AC-43 init refresh preserves schema_version line ("+tc.name+")", func(t *testing.T) {
			dir := t.TempDir()
			writeSpec(t, dir, "spec-a.spec.yaml", minimalValidSpec("spec-a", 3, "AC-01"))
			writeSpec(t, dir, "spec-c.spec.yaml", minimalValidSpec("spec-c", 3, "AC-01"))

			original := tc.schemaVersionLine + `
system:
  name: demo-system
  tier: 2
domains:
  default:
    specs: [spec-a]
  auth:
    description: "Authentication domain"
    specs: [spec-b]
settings:
  strict: true
  specs_dir: specs
`
			writeManifestRaw(t, dir, original)

			_, code := runCLI(t, dir, "init", "--refresh")
			if code != 0 {
				t.Fatalf("refresh exited %d, want 0", code)
			}

			body, err := os.ReadFile(filepath.Join(dir, "specter.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			got := string(body)

			// AC-43 byte-unchanged claim: the literal schema_version line
			// from the input must appear verbatim in the output. Substring
			// match is sufficient — the line is unique and ordering is
			// covered by the first-non-comment check below.
			if !strings.Contains(got, tc.schemaVersionLine) {
				t.Errorf("schema_version line %q not preserved verbatim after refresh\nfile:\n%s",
					tc.schemaVersionLine, got)
			}

			// First non-comment line must be exactly the input value (no
			// rewrite to 1, no key reordering that demotes it).
			var firstNonComment string
			for _, line := range strings.Split(got, "\n") {
				stripped := strings.TrimSpace(line)
				if stripped == "" || strings.HasPrefix(stripped, "#") {
					continue
				}
				firstNonComment = stripped
				break
			}
			if firstNonComment != tc.schemaVersionLine {
				t.Errorf("first non-comment line = %q, want %q\nfile:\n%s",
					firstNonComment, tc.schemaVersionLine, got)
			}
			// Custom domain "auth" must remain.
			if !strings.Contains(got, "auth:") || !strings.Contains(got, "spec-b") {
				t.Errorf("custom domain `auth` was not preserved; file:\n%s", got)
			}
			// Custom settings must remain.
			if !strings.Contains(got, "strict: true") {
				t.Errorf("custom settings.strict not preserved; file:\n%s", got)
			}
			// domains.default.specs must include the newly-discovered spec-c.
			if !strings.Contains(got, "spec-c") {
				t.Errorf("expected newly-discovered spec-c in default specs after refresh; file:\n%s", got)
			}
		})
	}
}

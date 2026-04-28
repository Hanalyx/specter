// schema_version_test.go — parser-side tests for spec-manifest C-27 (v1.9.0).
//
// @spec spec-manifest
package manifest

import "testing"

// @ac AC-41
// ParseManifest accepts an explicit `schema_version: 1` and exposes
// SchemaVersion=1 on the parsed Manifest.
func TestParseManifest_ExplicitSchemaVersion_PassedThrough(t *testing.T) {
	t.Run("spec-manifest/AC-41 explicit schema_version 1 passed through", func(t *testing.T) {
		yaml := `schema_version: 1
system:
  name: demo
domains:
  default:
    specs: []
`
		m, err := ParseManifest(yaml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if m.SchemaVersion != 1 {
			t.Errorf("SchemaVersion = %d, want 1", m.SchemaVersion)
		}
	})
}

// @ac AC-41
// ParseManifest with no `schema_version` field defaults SchemaVersion to 1.
func TestParseManifest_MissingSchemaVersion_DefaultsToOne(t *testing.T) {
	t.Run("spec-manifest/AC-41 missing schema_version defaults to 1", func(t *testing.T) {
		yaml := `system:
  name: demo
domains:
  default:
    specs: []
`
		m, err := ParseManifest(yaml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if m.SchemaVersion != 1 {
			t.Errorf("SchemaVersion = %d, want 1 (default)", m.SchemaVersion)
		}
	})
}

// @ac AC-41
// A future-version manifest (schema_version: 5) parses cleanly and exposes
// the value verbatim. Whether 5 is supported is a tool-layer concern, not a
// parse-layer one.
func TestParseManifest_FutureSchemaVersion_PreservedVerbatim(t *testing.T) {
	t.Run("spec-manifest/AC-41 future schema_version preserved verbatim", func(t *testing.T) {
		yaml := `schema_version: 5
system:
  name: demo
domains:
  default:
    specs: []
`
		m, err := ParseManifest(yaml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if m.SchemaVersion != 5 {
			t.Errorf("SchemaVersion = %d, want 5", m.SchemaVersion)
		}
	})
}

// @ac AC-41
// `schema_version` MUST be in validTopLevelKeys so v0.11's unknown-key
// rejection (C-26) accepts it. Without this guard, the test_glob/strictness
// hardening from v0.11 would reject every schema_version manifest at parse.
func TestParseManifest_SchemaVersion_NotRejectedAsUnknownKey(t *testing.T) {
	t.Run("spec-manifest/AC-41 schema_version is in validTopLevelKeys", func(t *testing.T) {
		yaml := `schema_version: 1
system:
  name: demo
`
		if _, err := ParseManifest(yaml); err != nil {
			t.Errorf("ParseManifest with schema_version must not error; v0.11 unknown-key rejection must accept it: %v", err)
		}
	})
}

// @spec spec-manifest
package manifest

import "testing"

// @ac AC-27
// ParseManifest accepts schema_version: 1 and returns SchemaVersion=1.
func TestParseManifest_ExplicitSchemaVersion1(t *testing.T) {
	yaml := `
schema_version: 1
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
}

// @ac AC-27
// ParseManifest without schema_version defaults to 1.
func TestParseManifest_MissingSchemaVersion_DefaultsTo1(t *testing.T) {
	yaml := `
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
		t.Errorf("SchemaVersion = %d, want 1 (default)", m.SchemaVersion)
	}
}

// @ac AC-27
// ParseManifest preserves an unrecognized schema_version value verbatim.
// Tool-layer validation of supported versions is a separate concern.
func TestParseManifest_UnrecognizedSchemaVersion_Preserved(t *testing.T) {
	yaml := `
schema_version: 2
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
	if m.SchemaVersion != 2 {
		t.Errorf("SchemaVersion = %d, want 2 (preserved)", m.SchemaVersion)
	}
}

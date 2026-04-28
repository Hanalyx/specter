// Pure-function tests for the internal/explain package.
//
// @spec spec-explain
package explain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Parity test: internal/explain/annotation_reference.md must match
// docs/TEST_ANNOTATION_REFERENCE.md byte-for-byte. The embed directive needs
// an in-package target; this keeps the two copies in sync.
func TestAnnotationReference_ParityWithDocs(t *testing.T) {
	t.Run("spec-explain/annotation-reference mirrors docs copy byte-for-byte", func(t *testing.T) {
		// Walk up to the specter module root so the test runs from any cwd.
		docsPath, err := findDocsCopy()
		if err != nil {
			t.Fatalf("locate docs copy: %v", err)
		}
		canonical, err := os.ReadFile(docsPath)
		if err != nil {
			t.Fatalf("read docs copy: %v", err)
		}
		embedded := AnnotationReference()
		if string(canonical) != embedded {
			t.Fatalf("annotation_reference.md drifted from docs/TEST_ANNOTATION_REFERENCE.md\n"+
				"  run: cp docs/TEST_ANNOTATION_REFERENCE.md internal/explain/annotation_reference.md\n"+
				"  canonical %d bytes, embedded %d bytes", len(canonical), len(embedded))
		}
	})
}

func findDocsCopy() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "docs", "TEST_ANNOTATION_REFERENCE.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func TestRenderSchemaReference_ContainsRequiredFields(t *testing.T) {
	t.Run("spec-explain/render-schema-reference covers required top-level fields", func(t *testing.T) {
		// Load schema bytes via the parser package's exported helper. Done here
		// without importing parser to avoid a package cycle (parser doesn't
		// import explain, but explain_test in this package would). Use the
		// embedded asset directly.
		schemaPath, err := findSchema()
		if err != nil {
			t.Fatalf("locate schema: %v", err)
		}
		schemaJSON, err := os.ReadFile(schemaPath)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		out, err := RenderSchemaReference(schemaJSON)
		if err != nil {
			t.Fatalf("RenderSchemaReference: %v", err)
		}
		for _, field := range []string{"spec.id", "spec.version", "spec.status", "spec.tier"} {
			if !strings.Contains(out, field) {
				t.Errorf("expected %q in schema reference, got:\n%s", field, out)
			}
		}
	})
}

func TestRenderSchemaField_UnknownPath_DidYouMean(t *testing.T) {
	t.Run("spec-explain/unknown-field-path returns did-you-mean", func(t *testing.T) {
		schemaPath, err := findSchema()
		if err != nil {
			t.Fatalf("locate schema: %v", err)
		}
		schemaJSON, err := os.ReadFile(schemaPath)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		_, err = RenderSchemaField(schemaJSON, "spec.accptance_criteria")
		if err == nil {
			t.Fatal("expected error for unknown field path, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "unknown field path") {
			t.Errorf("expected 'unknown field path' in error, got: %s", msg)
		}
		if !strings.Contains(strings.ToLower(msg), "did you mean") {
			t.Errorf("expected 'did you mean' suggestion, got: %s", msg)
		}
	})
}

func findSchema() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "internal", "parser", "spec-schema.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

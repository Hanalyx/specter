// @spec spec-reverse
package reverse

import "testing"

// mockAdapter is a minimal adapter for testing detection.
type mockAdapter struct {
	name       string
	extensions []string
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Detect(path, content string) bool {
	for _, ext := range m.extensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}
func (m *mockAdapter) IsTestFile(path string) bool                         { return false }
func (m *mockAdapter) ExtractRoutes(path, content string) []ExtractedRoute { return nil }
func (m *mockAdapter) ExtractConstraints(path, content string) []ExtractedConstraint {
	return nil
}
func (m *mockAdapter) ExtractAssertions(path, content string) []ExtractedAssertion { return nil }
func (m *mockAdapter) ExtractImports(path, content string) []ExtractedImport       { return nil }
func (m *mockAdapter) InferSystemName(files []SourceFile) string                   { return "" }

// @ac AC-11
func TestDetectAdapter_GoFiles(t *testing.T) {
	files := []SourceFile{
		{Path: "main.go", Content: "package main"},
		{Path: "handler.go", Content: "package main"},
		{Path: "handler_test.go", Content: "package main"},
	}
	adapters := []Adapter{
		&mockAdapter{name: "typescript", extensions: []string{".ts", ".tsx"}},
		&mockAdapter{name: "go", extensions: []string{".go"}},
		&mockAdapter{name: "python", extensions: []string{".py"}},
	}

	got := DetectAdapter(files, adapters)
	if got == nil {
		t.Fatal("expected adapter, got nil")
	}
	if got.Name() != "go" {
		t.Errorf("expected 'go' adapter, got %q", got.Name())
	}
}

// @ac AC-12
func TestDetectAdapter_TypeScriptFiles(t *testing.T) {
	files := []SourceFile{
		{Path: "index.ts", Content: "import foo from 'bar'"},
		{Path: "schema.ts", Content: "import z from 'zod'"},
		{Path: "auth.test.ts", Content: "describe('auth', () => {})"},
	}
	adapters := []Adapter{
		&mockAdapter{name: "typescript", extensions: []string{".ts", ".tsx"}},
		&mockAdapter{name: "go", extensions: []string{".go"}},
		&mockAdapter{name: "python", extensions: []string{".py"}},
	}

	got := DetectAdapter(files, adapters)
	if got == nil {
		t.Fatal("expected adapter, got nil")
	}
	if got.Name() != "typescript" {
		t.Errorf("expected 'typescript' adapter, got %q", got.Name())
	}
}

// @ac AC-11
func TestDetectAdapter_NoFiles(t *testing.T) {
	adapters := []Adapter{
		&mockAdapter{name: "go", extensions: []string{".go"}},
	}
	got := DetectAdapter(nil, adapters)
	if got != nil {
		t.Errorf("expected nil for no files, got %q", got.Name())
	}
}

// @ac AC-11
func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"file.ts", "typescript"},
		{"file.tsx", "typescript"},
		{"file.js", "typescript"},
		{"file.py", "python"},
		{"file.go", "go"},
		{"file.rb", ""},
		{"file.rs", ""},
	}
	for _, tt := range tests {
		got := DetectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

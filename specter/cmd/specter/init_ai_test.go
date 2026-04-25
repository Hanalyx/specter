// CLI integration tests for `specter init --ai <tool>`.
//
// @spec spec-manifest
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @ac AC-30
func TestInitAIClaude_NoAgentsMd_WritesCLAUDEMd(t *testing.T) {
	t.Run("spec-manifest/AC-30 --ai claude with no AGENTS.md writes CLAUDE.md inline body", func(t *testing.T) {
		dir := t.TempDir()
		_, code := runCLI(t, dir, "init", "--ai", "claude")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		body, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
		if err != nil {
			t.Fatalf("expected CLAUDE.md, stat err: %v", err)
		}
		s := string(body)
		if !strings.Contains(s, "<!-- specter:begin v1 -->") || !strings.Contains(s, "<!-- specter:end -->") {
			t.Errorf("expected fenced markers, got:\n%s", s)
		}
		if strings.Contains(s, "@AGENTS.md") {
			t.Errorf("did not expect @AGENTS.md import when AGENTS.md is absent, got:\n%s", s)
		}
		if !strings.Contains(s, "specter explain") {
			t.Errorf("expected inline body referencing specter explain, got:\n%s", s)
		}
	})
}

// @ac AC-31
func TestInitAICodex_WritesAGENTSMd(t *testing.T) {
	t.Run("spec-manifest/AC-31 --ai codex writes AGENTS.md", func(t *testing.T) {
		dir := t.TempDir()
		_, code := runCLI(t, dir, "init", "--ai", "codex")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatalf("expected AGENTS.md, stat err: %v", err)
		}
		if !strings.Contains(string(body), "<!-- specter:begin v1 -->") {
			t.Errorf("expected fenced markers, got:\n%s", string(body))
		}
	})
}

// @ac AC-32
func TestInitAICursor_CreatesNestedDir(t *testing.T) {
	t.Run("spec-manifest/AC-32 --ai cursor creates .cursor/rules and writes specter.md", func(t *testing.T) {
		dir := t.TempDir()
		_, code := runCLI(t, dir, "init", "--ai", "cursor")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		path := filepath.Join(dir, ".cursor", "rules", "specter.md")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s, stat err: %v", path, err)
		}
		if !strings.Contains(string(body), "<!-- specter:begin v1 -->") {
			t.Errorf("expected fenced markers, got:\n%s", string(body))
		}
	})
}

// @ac AC-33
func TestInitAICopilot_CapsAt4KB(t *testing.T) {
	t.Run("spec-manifest/AC-33 --ai copilot writes ≤4KB body to .github/copilot-instructions.md", func(t *testing.T) {
		dir := t.TempDir()
		_, code := runCLI(t, dir, "init", "--ai", "copilot")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		path := filepath.Join(dir, ".github", "copilot-instructions.md")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s, stat err: %v", path, err)
		}
		if len(body) > 4096 {
			t.Errorf("expected ≤4096 bytes for copilot, got %d", len(body))
		}
	})
}

// @ac AC-34
func TestInitAIGemini_WritesGEMINIMd(t *testing.T) {
	t.Run("spec-manifest/AC-34 --ai gemini writes GEMINI.md", func(t *testing.T) {
		dir := t.TempDir()
		_, code := runCLI(t, dir, "init", "--ai", "gemini")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		body, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
		if err != nil {
			t.Fatalf("expected GEMINI.md, stat err: %v", err)
		}
		if !strings.Contains(string(body), "<!-- specter:begin v1 -->") {
			t.Errorf("expected fenced markers, got:\n%s", string(body))
		}
	})
}

// @ac AC-35
func TestInitAI_IdempotentReRun_PreservesOutOfFence(t *testing.T) {
	t.Run("spec-manifest/AC-35 re-running --ai preserves user content outside fence", func(t *testing.T) {
		dir := t.TempDir()

		_, code := runCLI(t, dir, "init", "--ai", "codex")
		if code != 0 {
			t.Fatalf("first run expected exit 0, got %d", code)
		}
		path := filepath.Join(dir, "AGENTS.md")

		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		userPrefix := "# Project notes from the user\n\nLorem ipsum.\n\n"
		userSuffix := "\n\n## Trailing user content\n\nMore notes.\n"
		modified := []byte(userPrefix + string(original) + userSuffix)
		if err := os.WriteFile(path, modified, 0644); err != nil {
			t.Fatal(err)
		}

		_, code = runCLI(t, dir, "init", "--ai", "codex")
		if code != 0 {
			t.Fatalf("re-run expected exit 0, got %d", code)
		}

		final, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(final), userPrefix) {
			t.Errorf("expected pre-fence user prefix preserved, got:\n%s", string(final))
		}
		if !strings.Contains(string(final), userSuffix) {
			t.Errorf("expected post-fence user suffix preserved, got:\n%s", string(final))
		}
	})
}

// @ac AC-36
func TestInitAIClaude_WithAgentsMd_UsesImport(t *testing.T) {
	t.Run("spec-manifest/AC-36 --ai claude with existing AGENTS.md uses @AGENTS.md import", func(t *testing.T) {
		dir := t.TempDir()
		// Pre-create AGENTS.md.
		if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# pre-existing agents file\n"), 0644); err != nil {
			t.Fatal(err)
		}

		_, code := runCLI(t, dir, "init", "--ai", "claude")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}

		body, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if !strings.Contains(s, "@AGENTS.md") {
			t.Errorf("expected @AGENTS.md import when AGENTS.md exists, got:\n%s", s)
		}
		// Should NOT inline the full preflight body.
		if strings.Contains(s, "specter explain <spec-id>") {
			t.Errorf("did not expect inline preflight body when AGENTS.md handles it, got:\n%s", s)
		}
	})
}

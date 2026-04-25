// Pure-function tests for the per-tool AI instruction templates used by
// `init --ai <tool>`.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// Body content checks: the v0.11 instruction template must include the
// preflight self-check, Convention A example, validation gate, and on-demand
// explain references — the four load-bearing pieces from the BACKLOG synthesis.
func TestAIInstructionBody_ContainsPreflightAndExamples(t *testing.T) {
	t.Run("spec-manifest/AC-30 instruction body contains preflight and convention A", func(t *testing.T) {
		body := AIInstructionBody()

		// Preflight self-check / read-spec-first.
		if !strings.Contains(strings.ToLower(body), "before you") &&
			!strings.Contains(strings.ToLower(body), "before editing") {
			t.Errorf("expected preflight phrasing ('before you' / 'before editing'), got:\n%s", body)
		}
		// Convention A example with runner-visible spec-id/AC-NN form.
		if !strings.Contains(body, "spec-") || !strings.Contains(body, "AC-") {
			t.Errorf("expected Convention A example referencing spec-id and AC-NN, got:\n%s", body)
		}
		// Validation gate.
		if !strings.Contains(body, "make dogfood-strict") &&
			!strings.Contains(body, "specter coverage --strict") {
			t.Errorf("expected validation gate command (make dogfood-strict or specter coverage --strict), got:\n%s", body)
		}
		// On-demand reference: explain commands.
		if !strings.Contains(body, "specter explain") {
			t.Errorf("expected reference to `specter explain`, got:\n%s", body)
		}
	})
}

// AC-30 / AC-31 / AC-34: claude (no AGENTS.md) / codex / gemini all emit the
// inline body wrapped in fenced markers.
func TestRenderAIInstructions_ClaudeNoAgents_InlineBody(t *testing.T) {
	t.Run("spec-manifest/AC-30 claude with no AGENTS.md inlines body", func(t *testing.T) {
		got, err := RenderAIInstructions("claude", false)
		if err != nil {
			t.Fatalf("RenderAIInstructions claude: %v", err)
		}
		if !strings.Contains(got, "<!-- specter:begin v1 -->") || !strings.Contains(got, "<!-- specter:end -->") {
			t.Errorf("expected fenced markers, got:\n%s", got)
		}
		if !strings.Contains(got, "specter explain") {
			t.Errorf("expected inline body present (specter explain reference), got:\n%s", got)
		}
		if strings.Contains(got, "@AGENTS.md") {
			t.Errorf("did not expect @AGENTS.md import when AGENTS.md is absent, got:\n%s", got)
		}
	})
}

// AC-36: claude with existing AGENTS.md writes the @AGENTS.md import, NOT
// the inline body.
func TestRenderAIInstructions_ClaudeWithAgents_UsesImport(t *testing.T) {
	t.Run("spec-manifest/AC-36 claude with existing AGENTS.md uses @AGENTS.md import", func(t *testing.T) {
		got, err := RenderAIInstructions("claude", true)
		if err != nil {
			t.Fatalf("RenderAIInstructions claude (with AGENTS.md): %v", err)
		}
		if !strings.Contains(got, "@AGENTS.md") {
			t.Errorf("expected @AGENTS.md import when AGENTS.md is present, got:\n%s", got)
		}
		// Should NOT inline the full preflight body — that's in AGENTS.md.
		if strings.Contains(got, "specter explain <spec-id>") {
			t.Errorf("did not expect inline preflight body when AGENTS.md handles it, got:\n%s", got)
		}
	})
}

func TestRenderAIInstructions_Codex_InlineBody(t *testing.T) {
	t.Run("spec-manifest/AC-31 codex inlines body in AGENTS.md target", func(t *testing.T) {
		got, err := RenderAIInstructions("codex", false)
		if err != nil {
			t.Fatalf("RenderAIInstructions codex: %v", err)
		}
		if !strings.Contains(got, "<!-- specter:begin v1 -->") {
			t.Errorf("expected fenced markers, got:\n%s", got)
		}
		if !strings.Contains(got, "specter explain") {
			t.Errorf("expected inline body, got:\n%s", got)
		}
	})
}

// AC-33: copilot body capped at 4096 bytes for the code-review surface.
// Guard is enforced by RenderAIInstructions returning an error if the body
// exceeds CopilotMaxBytes — not by happenstance of body length.
func TestRenderAIInstructions_Copilot_Capped4KB(t *testing.T) {
	t.Run("spec-manifest/AC-33 copilot body capped at 4KB", func(t *testing.T) {
		got, err := RenderAIInstructions("copilot", false)
		if err != nil {
			t.Fatalf("RenderAIInstructions copilot: %v", err)
		}
		if len(got) > CopilotMaxBytes {
			t.Errorf("expected copilot body ≤ %d bytes, got %d bytes", CopilotMaxBytes, len(got))
		}
		// Load-bearing rule (read spec before code) must survive even at
		// the size limit — that's the priority guidance.
		if !strings.Contains(got, "specter explain") {
			t.Errorf("expected copilot body to retain `specter explain` reference, got:\n%s", got)
		}
	})
}

// Unknown tool argument errors clearly.
func TestRenderAIInstructions_UnknownTool_Errors(t *testing.T) {
	t.Run("spec-manifest/unknown ai tool errors out", func(t *testing.T) {
		_, err := RenderAIInstructions("notesnook", false)
		if err == nil {
			t.Fatal("expected error for unknown tool, got nil")
		}
	})
}

// AITargetPath maps each tool to its on-disk file path. Pure lookup.
func TestAITargetPath(t *testing.T) {
	t.Run("spec-manifest/AC-30..34 AI target paths per tool", func(t *testing.T) {
		cases := map[string]string{
			"claude":  "CLAUDE.md",
			"codex":   "AGENTS.md",
			"cursor":  ".cursor/rules/specter.md",
			"copilot": ".github/copilot-instructions.md",
			"gemini":  "GEMINI.md",
		}
		for tool, want := range cases {
			got, err := AITargetPath(tool)
			if err != nil {
				t.Errorf("AITargetPath(%q): %v", tool, err)
				continue
			}
			if got != want {
				t.Errorf("AITargetPath(%q) = %q, want %q", tool, got, want)
			}
		}

		if _, err := AITargetPath("unknown-tool"); err == nil {
			t.Errorf("expected error for unknown tool, got nil")
		}
	})
}

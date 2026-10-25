// Pure-function tests for the per-tool AI instruction templates used by
// `init --ai <tool>`.
//
// @spec spec-manifest
package manifest

import (
	"strings"
	"testing"
)

// Body content checks: the instruction template must include the preflight
// self-check, the Convention A example, the validation gate, and the on-demand
// explain references.
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
		// Validation gate. The generic gate only; Specter's own Makefile
		// target is not available in an adopter's project (C-38).
		if !strings.Contains(body, "specter coverage --strict") {
			t.Errorf("expected validation gate command `specter coverage --strict`, got:\n%s", body)
		}
		if strings.Contains(body, "make dogfood-strict") {
			t.Errorf("body must not name `make dogfood-strict`, got:\n%s", body)
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
		// Should NOT inline the full preflight body. AGENTS.md carries it.
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
// exceeds CopilotMaxBytes, not by happenstance of body length.
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
		// the size limit. That is the priority guidance.
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

// gateProblems lists every way a rendered instruction body falls short of
// C-38. An empty result means the body passes.
func gateProblems(body string) []string {
	var problems []string
	required := []string{
		"specter check --concrete",
		"inputs",
		"expected_output",
		"error_cases",
		"specter explain annotation",
		"// @spec",
		"// @ac",
		"Convention A",
		"Convention B",
		"JUnit XML",
		"go test -json",
		"specter ingest --junit",
		"specter ingest --go-test",
		"specter coverage --strict",
		"Exit 0 is the gate",
	}
	for _, r := range required {
		if !strings.Contains(body, r) {
			problems = append(problems, "missing "+r)
		}
	}
	// "// @spec" also appears in the Convention B text, so it alone cannot
	// show the body asks for source comments. Require the instruction itself.
	if !strings.Contains(strings.ToLower(body), "source comments") {
		problems = append(problems, "never tells the agent to write source comments")
	}
	diffWithExitCode := false
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "specter diff") && strings.Contains(line, "--exit-code") {
			diffWithExitCode = true
		}
	}
	if !diffWithExitCode {
		problems = append(problems, "no line runs specter diff with --exit-code")
	}
	if strings.Contains(body, "make dogfood-strict") {
		problems = append(problems, "names make dogfood-strict, which exists only in Specter's own Makefile")
	}
	// SSRB-104 deprecates settings.strictness; the body describes the
	// settings.annotation model that replaces it.
	if strings.Contains(body, "settings.strictness") {
		problems = append(problems, "names settings.strictness, which SSRB-104 deprecates")
	}
	if !strings.Contains(body, "settings.annotation") {
		problems = append(problems, "missing settings.annotation")
	}
	// Working order: check before ingest before coverage before diff.
	order := []string{"specter check --concrete", "specter ingest", "specter coverage --strict", "specter diff"}
	prev := -1
	for _, step := range order {
		i := strings.Index(body, step)
		if i < 0 {
			continue // already reported as missing above
		}
		if i < prev {
			problems = append(problems, "step out of working order: "+step)
		}
		prev = i
	}
	return problems
}

// @ac AC-73
func TestRenderAIInstructions_EveryInlineBodyNamesTheGenericGate(t *testing.T) {
	t.Run("spec-manifest/AC-73 every inline body names the generic gate in working order", func(t *testing.T) {
		// Differential preconditions: each kind of check must fire on a body
		// built to trip it. Without these, a checker that never fires would
		// pass every tool below.
		controls := map[string]string{
			"old dogfood line":  AIInstructionBody() + "\n\nRun `make dogfood-strict` before declaring work done.\n",
			"diff placed first": "specter diff specs/x.spec.yaml@abc specs/x.spec.yaml --exit-code\n\n" + AIInstructionBody(),
			"empty body":        "",
			"deprecated key":    AIInstructionBody() + "\n\n`settings.strictness` sets how strict the gate is.\n",
		}
		for label, control := range controls {
			if len(gateProblems(control)) == 0 {
				t.Fatalf("control body %q was accepted; the check cannot see the defect it guards", label)
			}
		}

		for _, tool := range []string{"claude", "codex", "cursor", "copilot", "gemini"} {
			got, err := RenderAIInstructions(tool, false)
			if err != nil {
				t.Fatalf("RenderAIInstructions(%q): %v", tool, err)
			}
			for _, p := range gateProblems(got) {
				t.Errorf("%s body: %s", tool, p)
			}
		}
	})
}

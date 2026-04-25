// Per-tool AI instruction templates for `init --ai <tool>`.
//
// Body content reflects the v0.11 design synthesis (see specter/BACKLOG.md
// "init --ai <tool>" entry): preflight self-check at the top, Convention A
// example, validation gate, on-demand `specter explain` references, no spec
// content dumped (the AI asks Specter for it on demand).
//
// @spec spec-manifest
package manifest

import (
	"fmt"
)

// AITargetPath maps each tool keyword to its on-disk instruction file path.
//
// AC-30..34: claude → CLAUDE.md, codex → AGENTS.md, cursor → .cursor/rules/specter.md,
// copilot → .github/copilot-instructions.md, gemini → GEMINI.md.
func AITargetPath(tool string) (string, error) {
	switch tool {
	case "claude":
		return "CLAUDE.md", nil
	case "codex":
		return "AGENTS.md", nil
	case "cursor":
		return ".cursor/rules/specter.md", nil
	case "copilot":
		return ".github/copilot-instructions.md", nil
	case "gemini":
		return "GEMINI.md", nil
	}
	return "", fmt.Errorf("unknown ai tool %q (supported: claude, codex, cursor, copilot, gemini)", tool)
}

// AIInstructionBody returns the v0.11 instruction template — the "project
// guide" body that sits inside the fenced region for codex / cursor / gemini
// (and for claude when no AGENTS.md is present).
//
// Length target ≤80 lines per Anthropic's CLAUDE.md guidance. Imperative
// voice; reserved MUST/NEVER for the highest-stakes rule. Convention A
// good/bad pair. Preflight self-check at the top. Load-bearing rule
// repeated at the top and bottom (primacy + recency).
func AIInstructionBody() string {
	return `# Specter project — read before writing code

Specs in ` + "`specs/*.spec.yaml`" + ` are the Single Source of Truth. Code is
derived from specs; when they disagree, the spec wins.

## Before you edit code

1. Identify which spec governs the file you're about to change
   (filename pattern: ` + "`specs/spec-<area>.spec.yaml`" + `).
2. Run ` + "`specter explain <spec-id>`" + ` and read the AC list it prints.
3. State the spec ID and the AC IDs your change touches before producing code.

## Tests trace to ACs (Convention A)

Every new test carries the spec ID and AC ID in its runner-visible name.

Good (Go):

` + "```go" + `
t.Run("spec-coverage/AC-19 failed result demotes all tiers", func(t *testing.T) {
    ...
})
` + "```" + `

Good (Jest / Vitest):

` + "```ts" + `
describe("[spec-extension/AC-12] command registration", () => { ... })
` + "```" + `

Bad — invisible to coverage, will fail the gate:

` + "```go" + `
func TestFailedResult(t *testing.T) { ... }
` + "```" + `

## Validation

Run ` + "`make dogfood-strict`" + ` before declaring work done. Exit 0 is the gate.
The strictness level for this project is in ` + "`specter.yaml`" + `.

## Boundaries

- Do not edit ` + "`specs/*.spec.yaml`" + ` to make code pass. Update the code,
  or propose a spec change in your reply for human review.
- If no spec covers your change, stop and ask which spec to read or create.

## On-demand reference

- ` + "`specter explain <spec-id>`" + ` — canonical spec content. Read it; do not guess.
- ` + "`specter explain schema`" + ` — schema field reference.
- ` + "`specter explain annotation`" + ` — test-annotation reference.

Reminder: read the spec before writing code. Tests without ` + "`@spec`/`@ac`" + `
annotations are invisible to ` + "`coverage --strict`" + ` and will fail the gate.`
}

// aiCopilotBody returns a tightened body for Copilot's 4096-byte cap.
// Strips longer examples; keeps the load-bearing rules (preflight + Convention A
// + validation gate + explain references).
func aiCopilotBody() string {
	return `# Specter project — read before writing code

Specs in ` + "`specs/*.spec.yaml`" + ` are the Single Source of Truth. Code is
derived from specs; the spec wins when they disagree.

## Before you edit code

1. Identify the matching spec (` + "`specs/spec-<area>.spec.yaml`" + `).
2. Run ` + "`specter explain <spec-id>`" + ` and read its ACs.
3. State the spec ID and AC IDs your change touches before writing code.

## Tests trace to ACs (Convention A)

Every test's runner-visible name includes the spec ID and AC ID.

Good: ` + "`t.Run(\"spec-x/AC-01 ...\", func(t *testing.T) { ... })`" + `
Good: ` + "`describe(\"[spec-x/AC-01] ...\", () => { ... })`" + `
Bad:  ` + "`func TestFoo(t *testing.T) { ... }`" + ` — invisible to coverage.

## Validation

Run ` + "`make dogfood-strict`" + ` before declaring work done. Exit 0 is the gate.

## Boundaries

- Do not edit specs to make code pass. Update code, or propose a spec change for review.
- If no spec covers your change, stop and ask which spec to read or create.

## On-demand reference

- ` + "`specter explain <spec-id>`" + ` — canonical spec content.
- ` + "`specter explain schema`" + ` — schema field reference.
- ` + "`specter explain annotation`" + ` — test-annotation reference.

Reminder: read the spec before writing code. Untraced tests fail ` + "`coverage --strict`" + `.`
}

// aiClaudeImportBody returns the body Claude's CLAUDE.md uses when an
// AGENTS.md is already present. Uses Claude's @AGENTS.md import directive
// to reference the canonical body without duplicating it.
func aiClaudeImportBody() string {
	return `@AGENTS.md

## Claude-specific notes

(Add Claude-specific guidance here. The @AGENTS.md import above carries the
canonical project preflight, Convention A examples, and validation gate.)`
}

// CopilotMaxBytes is the hard cap on the rendered Copilot instruction file.
// Copilot's code-review surface reads only the first 4KB of its instructions
// file; anything beyond is silently dropped, including load-bearing rules.
const CopilotMaxBytes = 4096

// RenderAIInstructions returns the full fenced-region body for one tool.
// hasAgentsMd is consulted only for tool="claude" (AC-36): present means
// emit the @AGENTS.md import; absent means inline the full body.
//
// AC-33 guard: if the rendered copilot body exceeds CopilotMaxBytes, this
// function returns an error rather than silently shipping a truncated file.
// Future template growth must be paired with copilot-specific trimming.
func RenderAIInstructions(tool string, hasAgentsMd bool) (string, error) {
	if _, err := AITargetPath(tool); err != nil {
		return "", err
	}

	var body string
	switch {
	case tool == "claude" && hasAgentsMd:
		body = aiClaudeImportBody()
	case tool == "copilot":
		body = aiCopilotBody()
	default:
		body = AIInstructionBody()
	}

	rendered, err := ReplaceFencedRegion("", MarkdownMarkers("v1"), body)
	if err != nil {
		return "", err
	}
	if tool == "copilot" && len(rendered) > CopilotMaxBytes {
		return "", fmt.Errorf("copilot body is %d bytes, exceeds the %d-byte cap; trim aiCopilotBody before merge", len(rendered), CopilotMaxBytes)
	}
	return rendered, nil
}

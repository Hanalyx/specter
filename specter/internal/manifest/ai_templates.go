// Per-tool AI instruction templates for `init --ai <tool>`.
//
// The body gives the agent its working order: read and check the spec, annotate
// tests, run the generic gate (test output, ingest, coverage --strict), and diff
// the spec before declaring done. spec-manifest C-38 defines that content. The
// body dumps no spec content; the agent asks Specter for it on demand.
//
// The gate must work in any adopter's project. It must never name a build
// target that exists only in Specter's own repository.
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

// AIInstructionBody returns the instruction template, the "project guide"
// body that sits inside the fenced region for codex, cursor and gemini, and
// for claude when no AGENTS.md is present.
//
// The body is about 100 lines. Anthropic's CLAUDE.md guidance suggests about
// 80; the gate steps and the annotation rule cost the extra lines.
// Imperative voice. The steps appear in the order the agent works.
// The load-bearing rule is repeated at the top and the bottom.
func AIInstructionBody() string {
	return `# Specter project: read before writing code

Specs in ` + "`specs/*.spec.yaml`" + ` are the Single Source of Truth. Code is
derived from specs. When they disagree, the spec wins.

## Before you edit code

1. Identify the spec that governs the file you are about to change
   (filename pattern: ` + "`specs/spec-<area>.spec.yaml`" + `).
2. Run ` + "`specter explain <spec-id>`" + ` to list its ACs and their coverage.
3. Read each AC in the spec file, including its ` + "`inputs`" + `,
   ` + "`expected_output`" + ` and ` + "`error_cases`" + ` fields. Your tests assert those.
4. Run ` + "`specter check --concrete`" + `. If it names an AC in your spec, that AC
   has neither ` + "`inputs`" + ` nor ` + "`expected_output`" + `, so nothing states what a
   test should assert. Stop and propose a spec fix first.
5. State the spec ID and the AC IDs your change touches before producing code.
6. Record the commit you started from with ` + "`git rev-parse HEAD`" + `. The last step uses it.

## Tests trace to ACs

Run ` + "`specter explain annotation`" + ` for the full rules. Write both parts on every test.

1. Source comments above the test: ` + "`// @spec <spec-id>`" + ` and one ` + "`// @ac AC-NN`" + `
   per AC (` + "`#`" + ` in Python). Without them the gate reports the AC uncovered.
2. The pair in the runner output, in one of two forms:
   - Convention A: ` + "`<spec-id>/AC-NN`" + ` in the test's runner-visible name.
   - Convention B: the test prints ` + "`// @spec <spec-id>`" + ` and ` + "`// @ac AC-NN`" + `
     on separate lines at runtime. Use it where a test name cannot hold ` + "`/`" + `,
     as in pytest.

Good (Go):

` + "```go" + `
// @spec spec-coverage
// @ac AC-19
func TestFailedResult(t *testing.T) {
    t.Run("spec-coverage/AC-19 failed result demotes all tiers", func(t *testing.T) {
        ...
    })
}
` + "```" + `

Good (Jest / Vitest):

` + "```ts" + `
// @spec spec-extension
// @ac AC-12
describe("[spec-extension/AC-12] command registration", () => { ... })
` + "```" + `

Bad: invisible to ` + "`coverage --strict`" + `, so it fails the gate:

` + "```go" + `
func TestFailedResult(t *testing.T) { ... }
` + "```" + `

## Validation gate

Run all three steps before declaring work done.

1. Run this project's own test command so that it writes JUnit XML
   (jest, vitest, pytest, playwright) or ` + "`go test -json`" + ` output to a file.
   Let it finish even when tests fail; the next steps record the failures.
2. Ingest the file. This writes ` + "`.specter-results.json`" + `.
   - ` + "`specter ingest --junit <file>`" + `
   - ` + "`specter ingest --go-test <file>`" + `
3. Run ` + "`specter coverage --strict`" + `. Exit 0 is the gate. Any other exit fails it.

` + "`specter.yaml`" + ` governs the gate. Its ` + "`settings.coverage`" + ` thresholds set the share
of ACs in each tier that must have a passing test. When it declares a
` + "`settings.annotation`" + ` block, an AC with no test fails the gate whatever the
threshold, and ` + "`permissive: true`" + ` turns that failure into a warning.

## Before declaring done

Run ` + "`specter diff`" + ` on each spec file that existed at the start commit:

` + "```sh" + `
specter diff specs/spec-<area>.spec.yaml@<start-commit> specs/spec-<area>.spec.yaml --exit-code
` + "```" + `

Exit 10 means a breaking change: an AC or constraint was removed, or an AC's
contract fields changed. Stop and ask for human review. Exit 1 means the diff
did not run; read the error. Exit 0 is not proof the spec is unchanged in
meaning. ` + "`diff`" + ` treats a reworded description as a patch, and it does not
compare constraint ` + "`enforcement`" + ` or the spec ` + "`tier`" + `. Read every line it prints.

## Boundaries

- Do not edit ` + "`specs/*.spec.yaml`" + ` to make code pass. Update the code,
  or propose a spec change in your reply for human review.
- If no spec covers your change, stop and ask which spec to read or create.

## On-demand reference

- ` + "`specter explain <spec-id>`" + `: ACs and coverage. The spec file holds the full text.
- ` + "`specter explain schema`" + `: schema field reference.
- ` + "`specter explain annotation`" + `: test-annotation reference.

Reminder: read the spec before writing code. A test needs both the source
comments and the pair in its runner output, or ` + "`coverage --strict`" + ` fails.`
}

// aiCopilotBody returns a tightened body for Copilot's 4096-byte cap.
// It drops the longer examples and keeps the load-bearing rules: the
// preflight, the annotation forms, the validation gate, the spec diff, and
// the explain references.
func aiCopilotBody() string {
	return `# Specter project: read before writing code

Specs in ` + "`specs/*.spec.yaml`" + ` are the Single Source of Truth. Code is
derived from specs. The spec wins when they disagree.

## Before you edit code

1. Identify the matching spec (` + "`specs/spec-<area>.spec.yaml`" + `).
2. Run ` + "`specter explain <spec-id>`" + ` to list its ACs.
3. Read each AC's ` + "`inputs`" + `, ` + "`expected_output`" + ` and ` + "`error_cases`" + ` in the spec file.
4. Run ` + "`specter check --concrete`" + `. An AC it names states nothing to assert. Propose a spec fix first.
5. State the spec ID and AC IDs your change touches before writing code.
6. Record the start commit with ` + "`git rev-parse HEAD`" + `.

## Tests trace to ACs

See ` + "`specter explain annotation`" + `. Write both parts on every test:

1. Source comments above it: ` + "`// @spec <id>`" + ` and ` + "`// @ac AC-NN`" + `.
2. The pair in runner output. Convention A puts ` + "`<id>/AC-NN`" + ` in the test name.
   Convention B prints ` + "`// @spec <id>`" + ` and ` + "`// @ac AC-NN`" + ` at runtime.

Good: the comments, then ` + "`t.Run(\"spec-x/AC-01 ...\", func(t *testing.T) { ... })`" + `
Good: the comments, then ` + "`describe(\"[spec-x/AC-01] ...\", () => { ... })`" + `
Bad:  ` + "`func TestFoo(t *testing.T) { ... }`" + `, invisible to ` + "`coverage --strict`" + `.

## Validation gate

1. Run the project's tests so they write JUnit XML or ` + "`go test -json`" + ` output.
2. ` + "`specter ingest --junit <file>`" + ` or ` + "`specter ingest --go-test <file>`" + `.
3. ` + "`specter coverage --strict`" + `. Exit 0 is the gate.

` + "`specter.yaml`" + ` governs the gate. Under a ` + "`settings.annotation`" + ` block, an AC with no
test fails unless the block sets ` + "`permissive: true`" + `.

## Before declaring done

For each spec that existed at the start commit, run
` + "`specter diff specs/<file>@<start-commit> specs/<file> --exit-code`" + `.
Exit 10 means an AC or constraint was removed or an AC's contract changed. Stop
and ask for review. Exit 0 does not cover reworded text, ` + "`enforcement`" + ` or
` + "`tier`" + `, so read every line it prints.

## Boundaries

- Do not edit specs to make code pass. Update code, or propose a spec change for review.
- If no spec covers your change, stop and ask which spec to read or create.

## On-demand reference

- ` + "`specter explain <spec-id>`" + `: ACs and coverage.
- ` + "`specter explain schema`" + `: schema field reference.
- ` + "`specter explain annotation`" + `: test-annotation reference.

Reminder: read the spec before writing code. A test missing either part fails ` + "`coverage --strict`" + `.`
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

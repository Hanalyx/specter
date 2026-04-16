# Specter SDD for VS Code

**Bring the spec→test→implement loop into your editor.**

In Spec-Driven Development, the specification is the source of truth — not the code. Every requirement has a test. Every test traces to a spec. Every spec is validated before the AI writes a line. This extension makes that discipline visible and low-friction: you see what's covered, what drifted, and what your AI assistant needs — without switching windows.

---

## Human Intent, AI Execution

Specter's schema is deliberately detailed — constraints, acceptance criteria, tiers, provenance, coverage thresholds. Writing all of that by hand for every module would be impractical, and that was never the intention.

The intended workflow is a collaboration between you and your AI coding assistant:

1. **You provide intent** — a brief description of what a module should do, its key constraints, and any non-obvious judgement calls or trade-offs
2. **The AI writes the spec** — translating your intent into a fully structured `.spec.yaml` file with constraints, ACs, and tier assignments
3. **The AI writes the tests** — derived directly from the ACs in the spec
4. **You review** — the spec and tests are the approval gate; you validate that the AI correctly captured your intent before any implementation begins
5. **The AI implements** — with the spec as the contract and the tests as the verification

Specter enforces the discipline at every step: the spec must exist before code, tests must trace to ACs, and coverage must meet the tier threshold before `specter sync` passes. It makes the process infrastructure, not a suggestion.

**The core mission: guide your AI coding assistant through spec → test → implement → eval in the right order, every time, with your intent preserved throughout.**

---

## The Problem This Solves

When you work with AI coding tools, two things go wrong silently:

**Coverage gaps you can't see.** You write a spec with eight requirements. The AI implements six. Tests pass for those six. The other two are simply absent — no error, no warning, no indicator anywhere. You find out in production.

**Specs that change after tests are written.** A requirement gets clarified, tightened, or removed. The test that covered it still says `@ac AC-03`. That annotation is now a lie — it references a requirement that no longer means the same thing. Nobody notices.

Specter SDD surfaces both problems in the editor, continuously, as you work.

---

## Features

### Know what's covered without running a report

Color-coded icons appear next to every requirement in your spec file the moment you open it — **green** means at least one test covers it, **red** means nothing does, **grey** means it's intentionally excluded. The status bar shows the aggregate across all specs at all times.

### Catch spec errors before you save

Mistakes in `.spec.yaml` files appear in the Problems panel within half a second of typing — missing required fields, broken cross-spec references, orphaned constraints. The same feedback loop TypeScript gives you for code, now for specs.

### Write `@spec` and `@ac` annotations in seconds

Type `// @spec` in a test file and Specter suggests spec names ranked by how close the spec file is to your test file. Type `// @ac` on the next line and completions are scoped to only the requirements from the spec you just referenced. No memorizing IDs, no switching to another file.

### See the full requirement on hover

Hover over `// @ac AC-03` in a test to see the requirement's full description, its current coverage status, and which other test files also cover it. Hover over a constraint ID in a spec to see which requirements depend on it and whether those requirements are covered.

### Get warned when a spec changes under your tests

If a spec requirement changes after you annotated a test against it, a **drift warning** icon appears in the gutter next to the annotation. Hover to see the original requirement and the current one side by side, classified as a breaking change, a new addition, or a wording clarification.

### Navigate specs like code

Go-to-definition works on spec cross-references: jump from a `depends_on` reference to the target spec file, or from a constraint reference to the exact line where that constraint is declared.

### See the full picture when something fails

Open the Specter sidebar to browse specs → requirements → test files in a tree. When something is below threshold, an Insights panel shows a plain-English explanation: what's uncovered, what the requirements actually say, and which constraints are affected.

### Hand your AI the full contract

**Specter: Copy Spec Context for AI** formats the current spec — tier, constraints, and all requirements with their full descriptions — as a structured markdown block and copies it to your clipboard. Paste into Claude, Cursor, or Copilot so your AI starts from the contract, not a guess.

### Get annotation suggestions automatically

A hint appears above any test function not yet linked to a spec, suggesting the most relevant requirements based on the test's name and body. Everything runs locally — no API call, no network access. Click to insert the annotation.

---

## How the SDD Workflow Looks in VS Code

**1. Write the spec.** Create a `.spec.yaml` file. Specter validates it immediately — schema errors appear in the Problems panel before you save.

**2. Annotate the test.** Use `// @spec` and `// @ac` completions to link your test function to a requirement. The gutter icon next to that requirement turns green.

**3. Implement with the contract.** Run **Copy Spec Context for AI** before prompting your AI assistant. It receives the exact requirements and constraints — not a paraphrase.

**4. See gaps instantly.** Any requirement still red in the gutter after implementation is a coverage gap. The status bar shows the count. The Insights panel explains what's missing.

**5. Stay aligned as specs evolve.** When a spec changes, drift warnings appear on any test whose annotation now references a different requirement than it did when it was written.

---

## Requirements

- VS Code 1.85 or later
- A workspace containing `specter.yaml` or at least one `*.spec.yaml` file
- The `specter` binary — downloaded automatically on first use if not found on PATH

---

## Annotation Format

```typescript
// @spec payment-create-intent
// @ac AC-01
function testValidCurrencyCreatesIntent() {
  const result = createIntent({ currency: 'USD', amount: 1000 });
  expect(result.status).toBe('pending');
}
```

The annotations are plain comments — no build step, no framework, works in any language.

---

## Settings

| Setting | Default | Description |
|---|---|---|
| `specter.binaryPath` | `""` | Path to the specter binary. Leave empty to auto-resolve. |
| `specter.autoDownload` | `true` | Download specter automatically if not found. |
| `specter.version` | `"latest"` | Binary version to download. |
| `specter.showInsightsOnFailure` | `true` | Open Insights panel automatically when a spec fails threshold. |

---

## Commands

| Command | What it does |
|---|---|
| `Specter: Open Insights Panel` | Plain-English health cards for all failing specs |
| `Specter: Copy Spec Context for AI` | Copy current spec as a structured AI prompt preamble |
| `Specter: Run Sync` | Re-run the full coverage pipeline manually |

---

## Links

- [Specter on GitHub](https://github.com/Hanalyx/specter)
- [AI Prompts](https://github.com/Hanalyx/specter/blob/main/specter/docs/AI_PROMPTS.md) — ready-to-use prompts for every stage of the SDD loop
- [Mastering Spec-Driven Development](https://github.com/Hanalyx/specter/tree/main/sddbook) — the methodology behind the tool
- [CLI Reference](https://github.com/Hanalyx/specter/blob/main/specter/docs/CLI_REFERENCE.md)
- [Report an issue](https://github.com/Hanalyx/specter/issues)

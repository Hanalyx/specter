# Spec-Driven Development

> *"Specs are not a crutch for weak models. They are safety equipment for powerful ones."* <!-- doc-style: allow --><!-- verbatim quotation from sddbook/MODULE_01/CHAPTER_01.md -->
>
> Mastering Spec-Driven Development, Chapter 1

AI coding tools generate code faster than any human can review it. The bottleneck is no longer writing code. It is knowing whether the code does what you actually intended. Natural language prompts are ambiguous. AI fills every gap silently, confidently, and often incorrectly. The code works. The intent drifted.

**Spec-Driven Development (SDD)** is the answer: write a structured specification before the AI writes a line of code. The spec resolves ambiguity, captures constraints, and defines what done looks like. The AI becomes an executor of a contract, not an interpreter of a wish.

This repository contains two things that work together:

---

## Specter: The Toolchain

**Specter** validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

Without Specter, a spec is just a document. With Specter, a spec becomes a checkable contract. Run it before implementation to catch spec errors, and add `specter sync` to CI to gate broken or below-threshold specs.

```
$ specter sync
Specter Sync

  PASS parse: 15 spec(s) parsed successfully
  PASS resolve: 15 specs, 23 dependencies resolved
  PASS check: 0 warning(s), 0 info
  PASS coverage: 15 spec(s) meet coverage thresholds


All checks passed.
```

Specter is content-agnostic. A `.spec.yaml` can describe runtime behavior, data invariants, security policy, schema contracts, architecture rules, or any other component contract. Anything with constraints and acceptance criteria qualifies. The pipeline validates the shape, not the category.

**→ [Get started with Specter](specter/README.md)**

---

## Mastering SDD: The Book

**Mastering Spec-Driven Development** is a 17-chapter course that teaches the full discipline: why natural language fails, how to write specs that AI can execute reliably, how to enforce the spec→test→implement loop, and how to scale SDD across teams and agents.

The book defines the methodology. Specter provides the infrastructure to enforce the checks you adopt.

**→ [Read the book](sddbook/INDEX.md)**

---

## Human Intent, AI Execution

Specter's schema is deliberately detailed: constraints, acceptance criteria, tiers, provenance, coverage thresholds. Writing all of that by hand for every module would be impractical, and that was never the intention.

The intended workflow is a collaboration between you and your AI coding assistant:

1. **You provide intent:** a brief description of what a module should do, its key constraints, and any non-obvious judgment calls or trade-offs
2. **The AI writes the spec:** translating your intent into a fully structured `.spec.yaml` file with constraints, ACs, and tier assignments
3. **The AI writes the tests:** derived directly from the ACs in the spec
4. **You review:** the spec and tests are the approval gate; you validate that the AI correctly captured your intent before any implementation begins
5. **The AI implements:** with the spec as the contract and the tests as the verification

Specter validates the artifacts in front of it, not the order you wrote them in. It checks that every spec parses and resolves. It measures coverage from the `@spec` and `@ac` annotations in your tests, and fails `specter sync` when a spec sits below its tier threshold. That makes the rules infrastructure rather than a suggestion.

**The core mission: guide your AI coding assistant through spec → test → implement → eval in the right order, every time, with your intent preserved throughout.**

---

## The Core Loop

```
Write spec  →  Validate spec  →  Generate code  →  Annotate tests  →  Enforce coverage
     ↑                                                                        |
     └────────────────── Refine spec when intent drifts ────────────────────┘
```

Every step in this loop has a Specter command behind it:

| Step | Command | What it enforces |
|---|---|---|
| Validate spec | `specter parse` | Schema correctness, required fields, valid IDs |
| Link specs | `specter resolve` | Dependencies, no cycles, version compatibility |
| Check structure | `specter check` | Orphan constraints, structural conflicts |
| Enforce coverage | `specter coverage` | Tests trace to ACs by annotation; each spec meets its tier threshold |
| Gate CI | `specter sync` | All of the above; exits non-zero on any failure |

---

## Why Structure Before Code

The book documents three failure modes that appear again and again in AI-assisted development:

**Ambiguity becomes decisions.** "Make a settings page" contains dozens of unanswered questions. The AI answers all of them silently, based on training data, not your intent. A spec forces those decisions to be made by a human before the AI starts.

**Code drifts from intent.** Tests pass. The feature ships. But the AI used a pattern you didn't want, skipped a constraint you cared about, or satisfied the letter of a requirement while violating its spirit. Without a spec, there is no reference to drift from. With a spec and Specter, drift is detectable.

**Knowledge evaporates between sessions.** Every new AI session starts from zero. The constraints you hammered out last sprint, the architectural decisions you made last month, all gone. A spec file is persistent memory that travels with the code and can be injected into any AI session as a contract.

---

## Quick Start

```bash
# Install Specter
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_${OS}_${ARCH}.tar.gz"
tar xzf "specter_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo mv specter /usr/local/bin/

# Validate your first spec
specter parse my-feature.spec.yaml

# Run the full pipeline
specter sync
```

→ [Full installation guide and first spec walkthrough](specter/docs/GETTING_STARTED.md)

→ [Ready-to-use AI prompts for every stage of the loop](specter/docs/AI_PROMPTS.md)

---

## VS Code Extension

The **Specter SDD** extension brings the SDD loop into the editor: live coverage decorations, spec diagnostics as you type, annotation completions, intent drift alerts, and a one-command AI context bridge.

→ [Install from the VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=Hanalyx.specter-vscode)

---

## License

MIT. See [LICENSE](LICENSE)

# Spec-Driven Development

> *"Specs are not a crutch for weak models. They are safety equipment for powerful ones."*
> — Mastering Spec-Driven Development, Chapter 1

AI coding tools generate code faster than any human can review it. The bottleneck is no longer writing code — it is knowing whether the code does what you actually intended. Natural language prompts are ambiguous. AI fills every gap silently, confidently, and often incorrectly. The code works. The intent drifted.

**Spec-Driven Development (SDD)** is the answer: write a structured specification before the AI writes a line of code. The spec resolves ambiguity, captures constraints, and defines what done looks like. The AI becomes an executor of a contract, not an interpreter of a wish.

This repository contains two things that work together:

---

## Specter — The Toolchain

**Specter** validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

Without Specter, a spec is just a document. With Specter, it is an enforced contract. Specter catches spec errors before code is generated, tracks which requirements are covered by tests, and blocks CI if your specs are broken or undertested.

```
$ specter sync

  PASS  parse     5 spec(s) parsed — no schema violations
  PASS  resolve   5 specs, 8 dependencies — no cycles or broken refs
  PASS  check     0 errors, 0 orphan constraints
  PASS  coverage  5 spec(s) meet coverage thresholds

All checks passed.
```

**→ [Get started with Specter](specter/README.md)**

---

## Mastering SDD — The Book

**Mastering Spec-Driven Development** is a 17-chapter course that teaches the full discipline: why natural language fails, how to write specs that AI can execute reliably, how to enforce the spec→test→implement loop, and how to scale SDD across teams and agents.

The book is the methodology. Specter is the infrastructure that makes it non-optional.

**→ [Read the book](sddbook/INDEX.md)**

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
| Enforce coverage | `specter coverage` | Every AC has a test; tiers met |
| Gate CI | `specter sync` | All of the above — exits non-zero on any failure |

---

## Why Structure Before Code

The book documents three failure modes that appear again and again in AI-assisted development:

**Ambiguity becomes decisions.** "Make a settings page" contains dozens of unanswered questions. The AI answers all of them — silently, based on training data, not your intent. A spec forces those decisions to be made by a human before the AI starts.

**Code drifts from intent.** Tests pass. The feature ships. But the AI used a pattern you didn't want, skipped a constraint you cared about, or satisfied the letter of a requirement while violating its spirit. Without a spec, there is no reference to drift from. With a spec and Specter, drift is detectable.

**Knowledge evaporates between sessions.** Every new AI session starts from zero. The constraints you hammered out last sprint, the architectural decisions you made last month — gone. A spec file is persistent memory that travels with the code and can be injected into any AI session as a contract.

---

## Quick Start

```bash
# Install Specter
curl -Lo specter.tar.gz https://github.com/Hanalyx/specter/releases/latest/download/specter_Linux_x86_64.tar.gz
tar xzf specter.tar.gz && sudo mv specter /usr/local/bin/

# Validate your first spec
specter parse my-feature.spec.yaml

# Run the full pipeline
specter sync
```

→ [Full installation guide and first spec walkthrough](specter/docs/GETTING_STARTED.md)

---

## VS Code Extension

The **Specter SDD** extension brings the SDD loop into the editor: live coverage decorations, spec diagnostics as you type, annotation completions, intent drift alerts, and a one-command AI context bridge.

→ [Install from the VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=Hanalyx.specter-vscode)

---

## License

MIT — see [LICENSE](LICENSE)

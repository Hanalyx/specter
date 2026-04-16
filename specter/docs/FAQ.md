# Specter FAQ

Frequently asked questions about Specter and Spec-Driven Development.

---

## What is SDD?

Spec-Driven Development (SDD) is a methodology where structured specification files are the Single Source of Truth (SSOT) for every feature in a system. The spec defines the "what and why" -- context, objective, constraints, and acceptance criteria -- before any code is written. When the spec and the code disagree, the spec is right and the code is wrong. SDD is designed for AI-assisted development, where specifications constrain the solution space and provide a verifiable contract for generated code. For the full methodology, see the [Mastering SDD course material](../../sddbook/INDEX.md).

---

## What is a micro-spec?

A micro-spec is the fundamental unit of specification in SDD. Every micro-spec has three pillars:

- **Context** -- What system, feature, and dependencies does this spec describe? What assumptions are being made?
- **Objective** -- What should this component do? What is in scope and what is explicitly excluded?
- **Constraints** -- What rules must be followed? Each constraint has an ID (e.g., `C-01`), a description, and an enforcement level (`error`, `warning`, or `info`).

A micro-spec also includes **acceptance criteria** -- testable conditions that prove the constraints are satisfied. Each AC references the constraints it validates, creating a traceable link from requirement to verification.

---

## Why YAML?

Specter uses YAML as the spec format for several reasons:

- **Human-readable.** YAML is easy to read and write without tooling. Developers can author specs in any text editor.
- **AI-readable.** Large language models parse YAML reliably. It can be included directly in prompts and context windows.
- **Diffable.** YAML produces clean diffs in version control, making spec changes easy to review in pull requests.
- **Existing ecosystem.** YAML has mature parsers in every language, JSON Schema validation support, and broad IDE support with syntax highlighting and autocompletion.
- **Structured but flexible.** YAML supports the nested, typed structure that specs require (objects, arrays, enums) without the syntactic noise of JSON or the ambiguity of Markdown.

---

## How is Specter different from OpenAPI / Swagger?

OpenAPI describes **API surfaces** -- endpoints, request/response schemas, status codes. It answers "what shape does the data have?"

Specter describes **behavioral contracts** -- context, intent, constraints, and acceptance criteria for any component in a system, not just APIs. It answers "what should this component do, why, and how do we verify it?"

Key differences:

- OpenAPI is scoped to HTTP APIs. Specter specs cover any feature: background jobs, state machines, authentication flows, data pipelines.
- OpenAPI does not express constraints like "MUST NOT exceed 200ms response time" or "MUST retry 3 times before failing." Specter constraints are first-class.
- OpenAPI does not model dependencies between specifications. Specter builds a dependency graph (`depends_on`) and detects circular dependencies, version mismatches, and structural conflicts across specs.
- Specter specs include acceptance criteria with explicit traceability to constraints. OpenAPI has no equivalent.

The two are complementary. An API spec in Specter might reference an OpenAPI schema for the data format while adding behavioral constraints and acceptance criteria on top.

---

## How is Specter different from Cucumber / BDD?

Cucumber and BDD tools are **test execution frameworks**. They run scenarios written in Gherkin against a live system to verify behavior.

Specter is a **pre-implementation validation tool**. It analyzes specs statically -- before any code or tests exist -- to catch structural problems: orphan constraints, circular dependencies, missing acceptance criteria, breaking changes between versions.

Key differences:

- Cucumber requires a running implementation. Specter works on specs alone.
- Cucumber validates "does the code match the scenario?" Specter validates "is the spec internally consistent and compatible with its dependencies?"
- Cucumber scenarios are prose. Specter specs are structured YAML with typed fields, constraint IDs, and explicit dependency declarations.
- Specter's coverage tool measures whether tests *exist* for each acceptance criterion. Cucumber measures whether tests *pass*.

A mature SDD workflow uses both: Specter validates specs before implementation, and test frameworks (including BDD tools) validate the implementation after.

---

## How do I adopt Specter on an existing codebase?

Start small and work outward:

1. **Generate draft specs from existing code.** Run `specter reverse src/ --output specs/` to extract draft specs from TypeScript, Python, or Go source files. Specter analyzes validation schemas, test assertions, and function signatures to produce `.spec.yaml` drafts.
2. **Review and complete the drafts.** The reverse compiler extracts structure but not intent. Review each generated spec, complete any ACs marked as gaps, and add missing constraints.
3. **Annotate existing tests.** Add `@spec` and `@ac` annotations to tests that cover the specified behavior. Run `specter coverage` to see where gaps remain.
4. **Integrate into CI.** Add `specter sync` to your CI pipeline. It exits 0 only when all Tier 1/2 specs meet their coverage thresholds.
5. **Expand incrementally.** Add specs for new features as they are built. Over time, spec coverage grows organically.

Use `specter doctor` to check project health at any time. It runs all pre-flight checks and tells you exactly what needs attention before running the full pipeline.

---

## What are tiers?

Tiers represent the risk level of a spec and determine how strictly Specter enforces rules.

| Tier | Label | Description | Example |
|------|-------|-------------|---------|
| 1 | Critical | Core business logic, payment flows, authentication, data integrity. Failures are costly or dangerous. | `payment-create-intent`, `user-auth` |
| 2 | Standard | Important features with moderate risk. The default tier for most specs. | `notification-send`, `report-generate` |
| 3 | Advisory | Low-risk features, internal tools, experimental work. | `admin-dashboard-layout`, `dev-metrics` |

Tier affects enforcement throughout the toolchain:

- **`specter check`**: Orphan constraints are errors in Tier 1, warnings in Tier 2, and info in Tier 3.
- **`specter coverage`**: Tier 1 requires 100% AC coverage by tests. Tier 2 requires 80%. Tier 3 requires 50%.

Set the tier in the spec file:

```yaml
spec:
  id: payment-create-intent
  version: "1.0.0"
  status: approved
  tier: 1
```

---

## What are constraint IDs and AC IDs?

Every constraint and acceptance criterion in a spec has a unique identifier:

- **Constraint IDs** follow the format `C-01`, `C-02`, `C-03`, etc.
- **AC IDs** follow the format `AC-01`, `AC-02`, `AC-03`, etc.

These IDs serve several purposes:

- **Traceability.** Each AC declares which constraints it validates via `references_constraints`. This creates a verifiable link from requirement to test.
- **Orphan detection.** `specter check` flags any constraint that is not referenced by at least one AC. If a constraint exists but nothing tests it, that is a gap.
- **Test annotation.** Test files reference AC IDs with `// @ac AC-01` annotations so that `specter coverage` can map tests back to specific acceptance criteria.
- **Communication.** In code reviews and discussions, "C-03 is not covered" is more precise than "that one constraint about retries."

Example:

```yaml
constraints:
  - id: C-01
    description: "MUST validate email format before submission"
    type: technical
    enforcement: error

acceptance_criteria:
  - id: AC-01
    description: "Valid email is accepted"
    references_constraints: ["C-01"]
    priority: critical

  - id: AC-02
    description: "Invalid email returns a validation error"
    references_constraints: ["C-01"]
    priority: critical
```

Specter enforces the ID format during parsing. `c1` or `ac-1` will be rejected; the correct formats are `C-01` and `AC-01`.

---

## How do I integrate Specter with CI?

Use `specter sync` as your CI gate. It runs the full pipeline (parse → resolve → check → coverage) in sequence and exits non-zero on any failure:

```yaml
# GitHub Actions
- name: Validate specs
  run: specter sync
```

`specter sync` exits `0` only when:
- All spec files parse without schema errors
- The dependency graph has no cycles or broken references
- No Tier 1/2 specs have orphan constraints (by default; configurable with `--strict`)
- All Tier 1 specs have 100% AC coverage, all Tier 2 specs have 80%, all Tier 3 specs have 50%

The exit code is `0` on success and `1` on failure, so it integrates with any CI system that checks exit codes. You can also run individual pipeline stages (`specter parse`, `specter resolve`, `specter check`, `specter coverage`) for more granular control.

---

## What languages does Specter support?

The `.spec.yaml` format is **language-agnostic**. Specs describe behavioral contracts, not implementation details. You can write specs for systems built in any language.

The toolchain is built in Go and distributed as a single binary with zero runtime dependencies.

For test coverage (`specter coverage`), annotation scanning supports:

- `//` comments (JavaScript, TypeScript, Go, Rust, Java, C#, etc.)
- `#` comments (Python, Ruby, Shell, YAML, etc.)

The reverse compiler (`specter reverse`) supports TypeScript, Python, and Go. It auto-detects the language from file extensions, or you can specify it with `--adapter typescript|python|go`.

---

## Can I use Specter with Claude Code / Cursor / Copilot?

Yes. Specter specs are designed to be consumed by AI tools as part of their context.

**Claude Code:** Add spec file paths to your `CLAUDE.md` so Claude reads them before writing code. Specter's own `CLAUDE.md` demonstrates this pattern -- it instructs Claude to read the relevant spec before writing or modifying any source file.

**Cursor:** Reference specs in your `.cursorrules` file. For example:

```
When working on the payment module, read specs/payment-create-intent.spec.yaml first.
All constraints in the spec are mandatory requirements.
```

**GitHub Copilot:** Include the spec content in your prompt or open the spec file in an adjacent tab so it is available as context.

The key principle: specs are plain YAML files that fit within AI context windows. Any AI tool that can read files can use them. The structured format (context, objective, constraints, acceptance criteria) gives the AI precisely the information it needs to generate correct implementations and tests.

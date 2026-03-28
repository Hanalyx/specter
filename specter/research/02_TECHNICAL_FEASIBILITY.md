# Technical Feasibility Assessment: SDD Toolchain ("Specter")

**Agent:** Technical Feasibility
**Date:** 2026-03-27
**Scope:** Evaluate buildability, risk, and implementation strategy for all 6 proposed tools
**Verdict:** Conditionally feasible. Four tools are buildable with known techniques. Two require carefully bounded AI integration. The critical risk is not any single tool -- it is the schema design that everything depends on.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Tool-by-Tool Feasibility](#2-tool-by-tool-feasibility)
   - [2.1 spec-parse](#21-spec-parse--yaml-to-typed-ast)
   - [2.2 spec-resolve](#22-spec-resolve--dependency-graph-builder)
   - [2.3 spec-check](#23-spec-check--the-type-checker)
   - [2.4 spec-coverage](#24-spec-coverage--usage-analysis)
   - [2.5 spec-sync](#25-spec-sync--ci-enforcer)
   - [2.6 Reverse Compiler](#26-reverse-compiler--code-to-spec)
3. [Cross-Cutting Concerns](#3-cross-cutting-concerns)
4. [Canonical Schema Design](#4-canonical-schema-design)
5. [Recommended Build Order](#5-recommended-build-order)
6. [Overall Verdict](#6-overall-verdict)
7. [Sources](#7-sources)

---

## 1. Executive Summary

| Tool | Complexity | AI Required? | CI-Speed? | MVP Feasible? |
|------|-----------|-------------|-----------|---------------|
| spec-parse | **Low** | No | Yes (ms) | Yes -- 2-3 weeks |
| spec-resolve | **Medium** | No | Yes (ms) | Yes -- 2-3 weeks |
| spec-check (structural) | **Medium** | No | Yes (seconds) | Yes -- 3-4 weeks |
| spec-check (semantic) | **Research-grade** | Yes | No (seconds-minutes) | Defer to Phase 6 |
| spec-coverage | **Medium** | No | Yes (seconds) | Yes -- 2-3 weeks |
| spec-sync | **Low-Medium** | No | Yes (seconds) | Yes -- 1-2 weeks (given other tools) |
| Reverse Compiler (structural) | **High** | No | N/A (offline) | Yes -- 4-6 weeks |
| Reverse Compiler (AI gap-fill) | **High** | Yes | N/A (offline) | Yes but bounded -- 2-3 weeks on top |

The honest summary: **the deterministic core (parse, resolve, structural check, coverage, sync) is solidly buildable** with existing open-source libraries and well-understood compiler construction patterns. The AI-assisted layers (semantic conflict detection, gap analysis, behavioral inference from tests) are feasible as advisory tools but must be clearly marked as probabilistic. The reverse compiler is the highest-effort item but also the highest-value item for adoption.

---

## 2. Tool-by-Tool Feasibility

### 2.1 `spec-parse` -- YAML to Typed AST

**Complexity:** Low
**AI Required:** No
**Performance:** Sub-millisecond per file

#### What It Actually Is

A YAML parser that validates `.spec.yaml` files against a JSON Schema and produces a typed in-memory object. This is a solved problem.

#### Required Technologies

| Need | Library | Maturity |
|------|---------|----------|
| YAML parsing | `js-yaml` (JS) or `PyYAML`/`ruamel.yaml` (Python) | Mature, stable |
| JSON Schema validation | `ajv` (JS) or `jsonschema`/`fastjsonschema` (Python) | Mature. Ajv supports draft-2020-12 |
| Typed AST output | TypeScript interfaces / Python dataclasses + Pydantic | Standard |

#### Known-Hard Problems

Essentially none. This is a well-trodden path. The only subtlety is designing the schema itself (see Section 4), not implementing the parser.

One minor challenge: constraint expressions. The groundwork doc shows constraints as free-text strings (`"amount must be positive integer in cents"`). If we want the type checker to reason about these structurally, we need a mini-grammar for constraints -- e.g., `field: amount, type: integer, min: 1`. Free-text constraints are parseable by humans and LLMs but not by deterministic tools.

**Decision point:** Do constraints remain free-text (easy to author, hard to check) or become structured (harder to author, easy to check)? The MVP should support both: structured constraints get deterministic checking, free-text constraints get AI-assisted checking (or are skipped).

#### Existing Building Blocks

- **Ajv** is the fastest JSON Schema validator in the JS ecosystem. It compiles schemas to validation functions. It handles YAML via `ajv-cli`. ([ajv.js.org](https://ajv.js.org/))
- **Spectral** by Stoplight demonstrates exactly this pattern: define a JSON/YAML schema, write custom validation rules, lint files against them. Spectral's architecture (rulesets + given/then JSONPath targeting + custom functions) is directly applicable. ([github.com/stoplightio/spectral](https://github.com/stoplightio/spectral))

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| Validate required fields exist | Validate constraint expressions structurally |
| Check types of known fields | Custom validation functions per constraint type |
| Produce typed object | Source-mapped error locations for IDE integration |
| CLI output: pass/fail + errors | LSP server for real-time validation in editors |

#### Risk Factors

- **Low risk.** The only way this fails is if the schema design is wrong, which is a design problem, not an implementation problem.

---

### 2.2 `spec-resolve` -- Dependency Graph Builder

**Complexity:** Medium
**AI Required:** No
**Performance:** Sub-second for hundreds of specs

#### What It Actually Is

Read all `.spec.yaml` files in a project, build a directed graph from `depends_on` fields, detect structural problems (cycles, dangling references, version conflicts).

#### Required Technologies

| Need | Library | Maturity |
|------|---------|----------|
| Graph data structure | `@dagrejs/graphlib` (JS) or `networkx` (Python) | Mature |
| Cycle detection | Built into graphlib (`alg.findCycles`) and networkx | Standard algorithm |
| Semver comparison | `semver` npm package or `packaging.version` (Python) | Mature |
| File discovery | Glob patterns (`fast-glob` / `pathlib`) | Trivial |

#### Known-Hard Problems

1. **Version range resolution.** The groundwork doc mentions `depends_on: user-registration@^1.0`. Implementing semver range matching (caret, tilde, exact) is not trivial from scratch, but the `semver` npm package handles it completely.

2. **Transitive dependency conflicts.** Spec A depends on B@^1.0 and C@^2.0, but C depends on B@^2.0. This is the classic diamond dependency problem. Detecting it is straightforward (graph traversal + version comparison). Resolving it is a design/human decision, not a tool decision. The tool should report the conflict, not auto-resolve.

3. **Performance at scale.** For typical projects (50-500 specs), graph operations are instant. For very large monorepos (5000+ specs), we would need to consider incremental graph updates. This is a future optimization, not an MVP concern.

#### Existing Building Blocks

- **`@dagrejs/graphlib`** provides directed multigraph with topological sort, cycle detection, and path-finding algorithms. Last updated March 2026, actively maintained. ([github.com/dagrejs/graphlib](https://github.com/dagrejs/graphlib))
- **`networkx`** (Python) is the gold standard for graph analysis. Overkill for this use case but extremely well-documented.
- The **npm `semver`** package handles all semver range operations.

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| Build graph from `depends_on` | Transitive conflict detection |
| Detect cycles | Version range compatibility matrix |
| Detect dangling references | Visualization output (DOT/Mermaid) |
| CLI output: graph summary + errors | Incremental graph updates for large repos |

#### Risk Factors

- **Low risk.** Graph algorithms are textbook material. The `depends_on` schema is the only design dependency.

---

### 2.3 `spec-check` -- The Type Checker

**Complexity:** Medium (structural checks) / Research-grade (semantic checks)
**AI Required:** No for structural, Yes for semantic
**Performance:** Seconds for structural, variable for semantic

This is the most important tool and the one with the widest feasibility range. The groundwork doc describes four checks. They are not equally hard.

#### Check 1: Conflict Detection

**Structural conflicts (Medium, deterministic):**
If Spec A says `email: required` and Spec B (which depends on A) has an AC that assumes email is absent, this is detectable by comparing structured constraint fields across the dependency graph. This requires constraints to be structured (not free-text).

**Semantic conflicts (Research-grade, AI-required):**
"Spec A assumes synchronous processing" vs. "Spec B assumes eventual consistency" -- this is natural language reasoning. No deterministic tool catches this. An LLM can flag it as suspicious, but with false positives.

**Implementation approach:** Start with structural conflict detection only. Build a rule engine that compares typed constraint fields across dependent specs. Use Spectral's "given/then" pattern: for each dependency edge in the graph, apply comparison rules to overlapping constraint domains.

#### Check 2: Orphan Detection

**Complexity:** Low, fully deterministic.

Walk each spec's constraints and check whether any AC references them. This is a set-membership check. The only requirement is a consistent naming/referencing convention between constraints and ACs.

**Implementation:** Parse constraint IDs and AC references. Compute `constraints - referenced_constraints = orphans`. Trivial.

#### Check 3: Gap Detection

**Complexity:** Research-grade for full coverage, Medium for basic gaps.

The groundwork doc's example ("no AC for password shorter than 8 chars") implies reasoning about the *input space* implied by constraints. A constraint `min: 8` implies boundary conditions at 7, 8, 0, empty string. Enumerating these systematically for arbitrary constraint types is a bounded combinatorial problem.

**MVP approach:** For structured constraints with known types (string length, numeric range, enum, required/optional), generate a fixed set of boundary cases and check if ACs cover them. This is deterministic and handles 70-80% of real gaps.

**Full approach:** Use an LLM to reason about uncovered paths in free-text constraints. Mark results as `confidence: ai-inferred`.

#### Check 4: Breaking Change Detection

**Complexity:** Medium, fully deterministic.

Compare two versions of the same spec. Classify each field difference as breaking/additive/patch. This is a structured diff with classification rules.

**Implementation:** Use `json-diff` or a custom tree-diff algorithm on the parsed spec ASTs. Apply classification rules:
- Field removed = breaking
- Required field added = breaking
- Optional field added = additive
- Type changed = breaking
- Constraint tightened = breaking
- Constraint relaxed = additive
- Description changed = patch

This is well-understood from API versioning tools. Optic, Bump.sh, and similar tools do exactly this for OpenAPI specs.

#### Required Technologies

| Need | Library | Notes |
|------|---------|-------|
| Tree diff | `deep-diff` (JS) / `deepdiff` (Python) | For breaking change detection |
| Rule engine | Custom, modeled on Spectral's architecture | Given/then pattern |
| LLM integration (semantic only) | Anthropic SDK / OpenAI SDK | For Phase 6 semantic checks |

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| Orphan detection | Semantic conflict detection (AI) |
| Breaking change detection | Full gap analysis (AI) |
| Structural conflict detection (typed constraints only) | Cross-project conflict detection |
| CLI output: errors + warnings | IDE integration with inline annotations |

#### Risk Factors

- **Medium risk for structural checks.** Dependent on schema design -- if constraints are free-text only, structural checks have limited value.
- **High risk for semantic checks.** LLM-based analysis will have false positives. Users will lose trust if the tool cries wolf. Must be clearly separated from deterministic checks and gated behind a `--semantic` flag.

---

### 2.4 `spec-coverage` -- Usage Analysis

**Complexity:** Medium
**AI Required:** No
**Performance:** Seconds (file scanning + annotation matching)

#### What It Actually Is

A traceability matrix: for each spec, how many ACs have corresponding tests? How many have corresponding code files? What percentage of the spec is "covered"?

#### Required Technologies

| Need | Library | Notes |
|------|---------|-------|
| File scanning | `fast-glob` (JS) / `pathlib` (Python) | Find test and code files |
| Annotation parsing | Regex or AST-based `@spec` / `@test` extraction | See below |
| Report generation | Table formatting (`cli-table3`, `chalk`) | CLI output |

#### Known-Hard Problems

1. **Annotation convention.** The system assumes code files have `@spec payment-create-intent` annotations and test files have `@test AC-01` annotations. If these don't exist (legacy code), coverage is zero. This is a chicken-and-egg problem with the reverse compiler.

2. **Fuzzy matching.** What if a test doesn't use the `@test` annotation but its `describe` block clearly references the spec? Pattern matching on test descriptions is fragile. The MVP should require explicit annotations and treat everything else as uncovered.

3. **Counting methodology.** "100% coverage" means every AC has at least one test. But does that test actually validate the AC, or does it just share a name? This tool counts annotations, not correctness. That limitation must be documented.

#### Existing Building Blocks

The groundwork doc mentions an existing `check-spec-coverage.py` script. This is the starting point. Extend it rather than rewrite.

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| Count `@spec` and `@test` annotations | Parse test assertions to validate AC coverage quality |
| Produce coverage percentage per spec | Track coverage trends over time |
| CLI table output | HTML report with drill-down |
| Fail/warn based on threshold | Integration with coverage tools (Istanbul, coverage.py) |

#### Risk Factors

- **Low-medium risk.** The tool is only as good as the annotation discipline. Without the reverse compiler to bootstrap annotations, adoption is manual and slow.

---

### 2.5 `spec-sync` -- CI Enforcer

**Complexity:** Low-Medium
**AI Required:** No
**Performance:** Seconds (runs other tools + checks git diff)

#### What It Actually Is

A CI pipeline step that runs `spec-parse`, `spec-resolve`, `spec-check`, and `spec-coverage`, then enforces tier-based thresholds. It also checks that PRs with spec changes include corresponding test changes.

#### Required Technologies

| Need | Library | Notes |
|------|---------|-------|
| Git diff analysis | `simple-git` (JS) / `gitpython` (Python) | Detect changed files |
| CI integration | GitHub Actions YAML | Standard |
| Configuration | `.specterrc.yaml` or similar | Tier definitions, thresholds |

#### Known-Hard Problems

1. **Configuration design.** The tiered enforcement table (Tier 1 = strict, Tier 3 = relaxed) requires a way to assign tiers to specs. This could be a field in each spec or a separate configuration file. Both have tradeoffs.

2. **PR-scoped analysis.** The tool must compare the PR's changes against the main branch, not just validate the entire project. This means running `spec-check` on the diff, which requires understanding which specs changed and which downstream specs are affected.

#### Existing Building Blocks

Standard CI tooling. GitHub Actions, `actions/checkout`, shell scripting. Nothing exotic.

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| Run all tools, fail on errors | Tier-based enforcement matrix |
| Check spec-change-implies-test-change | PR comment with coverage delta |
| Single-tier (everything is strict) | GitHub status checks per tool |
| `.specterrc.yaml` config | Auto-assignment of tiers based on path patterns |

#### Risk Factors

- **Low risk.** This is orchestration, not novel logic. Depends on the other tools being stable.

---

### 2.6 Reverse Compiler -- Code to Spec

**Complexity:** High
**AI Required:** Partially (structural extraction is deterministic; gap-filling requires LLM)
**Performance:** N/A (offline tool, not CI)

This is the highest-effort, highest-value tool. It is also the most technically interesting.

#### 2.6.1 Structural Extraction (Deterministic)

**What it does:** Parse source code ASTs to extract function signatures, types, imports, exports, route definitions, and validation schemas.

**TypeScript extraction:**

| Need | Library | Capability |
|------|---------|------------|
| AST parsing | **ts-morph** | Wraps TypeScript Compiler API. Extracts function declarations, parameter types, return types, interfaces, class members, decorators. Actively maintained. ([ts-morph.com](https://ts-morph.com/)) |
| Alternative | **TypeScript Compiler API** directly | Lower-level but more control. `ts.createProgram()` + `TypeChecker` gives full type resolution including inferred types. ([github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API](https://github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API)) |
| Route extraction | Custom AST visitors for Express/Fastify/NestJS patterns | Pattern-match `router.get()`, `@Get()` decorators, etc. |
| Zod schema extraction | **Zod's `z.toJSONSchema()`** or AST-based introspection | Zod v4 supports `z.toJSONSchema()` for converting schemas to JSON Schema. For static analysis (without running code), AST parsing of `z.object({...})` calls is needed. ([zod.dev/json-schema](https://zod.dev/json-schema)) |
| Pydantic extraction | AST parsing of class definitions + `model_json_schema()` at runtime | Pydantic models have `model_json_schema()` for runtime extraction. Static extraction requires parsing class definitions. |

**Python extraction:**

| Need | Library | Capability |
|------|---------|------------|
| AST parsing (lossless) | **LibCST** | Concrete syntax tree preserving whitespace/comments. Visitor/Transformer API. Supports Python 3.0-3.14. Maintained by Instagram/Meta. ([github.com/Instagram/LibCST](https://github.com/Instagram/LibCST)) |
| AST parsing (fast) | **`ast` module** (stdlib) | Faster but lossy (no comments, no whitespace). Sufficient for signature extraction. |
| Type extraction | `ast` + type stubs / `mypy` API | Full type resolution requires mypy or pyright. For basic extraction (annotations on function signatures), `ast` suffices. |

**Known-hard problems in structural extraction:**

1. **Inferred types.** TypeScript code without explicit type annotations requires the TypeChecker to resolve types. `ts-morph` handles this via `getType()`, but it requires a valid `tsconfig.json` and all dependencies installed. For a reverse compiler running against arbitrary codebases, this means the tool needs a working build environment.

2. **Dynamic patterns.** Express route handlers registered via loops, middleware chains, or dynamic imports are not statically extractable. The tool will miss these. Solution: document the limitation and provide a manual override mechanism.

3. **Framework diversity.** Express, Fastify, NestJS, Hono, Koa (TypeScript) and Flask, FastAPI, Django (Python) all have different route registration patterns. Supporting all of them is a long tail. MVP should target 2-3 frameworks.

4. **Validation schema extraction.** Zod schemas can be composed, merged, and transformed. Statically analyzing `z.object({}).merge(otherSchema).pick({...})` chains requires effectively re-implementing Zod's type algebra in the AST parser. The MVP should handle flat `z.object({})` definitions and flag composed schemas for manual review.

#### 2.6.2 Behavioral Inference from Tests (Deterministic + Heuristic)

**What it does:** Parse test files, extract assertion patterns, map them to behavioral statements.

**Implementation:** AST-parse test files looking for:
- `describe()` / `it()` / `test()` block names (behavioral descriptions)
- `expect().toBe()` / `assert` patterns (assertions)
- HTTP status code assertions (`expect(res.status).toBe(401)`)
- Error message assertions

Map these to AC templates: `"Returns {status} when {condition from test name}"`.

**Known-hard problems:**

1. **Test name quality.** If tests are named `it('works')`, there is no behavioral information to extract. The tool depends on decent test naming conventions.

2. **Assertion-to-behavior mapping.** `expect(result).toBeTruthy()` tells us almost nothing. `expect(result.status).toBe(401)` tells us a lot. The quality of extracted ACs varies wildly based on test style.

**MVP approach:** Extract test names and HTTP status assertions only. Mark everything as `confidence: low`. This is still valuable because it creates the AC skeleton that a human (or LLM) can refine.

#### 2.6.3 AI-Assisted Gap Filling

**What it does:** An LLM reads the structural extraction + test inference and drafts the Context/Objective/Constraints in micro-spec format.

**Implementation:** Construct a prompt with:
- Extracted function signatures and types
- Extracted test names and assertions
- Detected validation rules (Zod/Pydantic)
- Request: "Generate a micro-spec in the canonical YAML schema for this module."

**Known-hard problems:**

1. **Context window limits.** Large modules with many functions, types, and tests may exceed context windows. Solution: chunk by module/file and generate one spec per file.

2. **Hallucination in specs.** The LLM may infer constraints that don't exist in the code. Every AI-generated field must be marked `source: ai-inferred` and `status: draft`.

3. **Cost at scale.** Running an LLM over hundreds of files gets expensive. The structural extraction should be maximized to minimize what the LLM needs to fill in.

#### MVP vs. Full Scope

| MVP | Full |
|-----|------|
| TypeScript extraction only (ts-morph) | TypeScript + Python (LibCST) |
| Express/Fastify route extraction | All major frameworks |
| Flat Zod schema extraction | Composed/transformed schema analysis |
| Test name extraction | Assertion-to-behavior mapping |
| LLM gap-fill with draft status | Iterative refinement with human feedback loop |

#### Risk Factors

- **Medium-high risk.** The structural extraction is buildable but labor-intensive due to framework diversity. The AI gap-fill works but produces drafts that require significant human review. The risk is not technical failure but user disappointment -- if the generated specs are too rough, adoption suffers.

---

## 3. Cross-Cutting Concerns

### 3.1 Language Support: TypeScript AND Python?

**Assessment:** Start with TypeScript. Add Python later.

| Concern | TypeScript | Python | Verdict |
|---------|-----------|--------|---------|
| AST parsing | ts-morph (excellent) | LibCST (excellent) | Both have strong tooling |
| Type extraction | TypeChecker resolves inferred types | Requires mypy/pyright for full resolution | TS is easier for static analysis |
| Validation frameworks | Zod (dominant, introspectable) | Pydantic (dominant, introspectable) | Comparable |
| Framework patterns | Express/Fastify/NestJS | Flask/FastAPI/Django | TS frameworks are more pattern-consistent |
| Test frameworks | Vitest/Jest (consistent API) | pytest (flexible but diverse patterns) | TS is more predictable |

**Recommendation:** Build the tool core in TypeScript. Target TypeScript codebases for MVP. Python support is a Phase 2 effort that reuses the spec schema and all tools except the reverse compiler's AST extraction layer. The spec format itself is language-agnostic (YAML), so `spec-parse`, `spec-resolve`, `spec-check`, `spec-coverage`, and `spec-sync` work identically regardless of the target language.

### 3.2 AI Dependency Matrix

| Tool | Deterministic | AI-Assisted | Notes |
|------|:------------:|:-----------:|-------|
| spec-parse | 100% | 0% | Pure schema validation |
| spec-resolve | 100% | 0% | Graph algorithms |
| spec-check (structural) | 100% | 0% | Rule-based comparison |
| spec-check (semantic) | 0% | 100% | NLP-level reasoning required |
| spec-check (gap detection) | ~70% | ~30% | Boundary enumeration is deterministic; edge case reasoning is AI |
| spec-coverage | 100% | 0% | Annotation counting |
| spec-sync | 100% | 0% | Orchestration |
| Reverse compiler (structural) | 100% | 0% | AST parsing |
| Reverse compiler (behavioral) | ~60% | ~40% | Test name extraction is deterministic; interpretation is AI |
| Reverse compiler (gap-fill) | 0% | 100% | LLM-generated content |

**Key insight:** The core toolchain (parse, resolve, structural check, coverage, sync) is 100% deterministic. AI is only needed for the reverse compiler and advanced semantic checks. This means the core tools can ship without any LLM dependency and run in CI with predictable, fast, reproducible results.

### 3.3 Performance Budget

| Tool | Target | Achievable? | Notes |
|------|--------|-------------|-------|
| spec-parse | <100ms per file | Yes | JSON Schema validation is compiled by Ajv |
| spec-resolve | <1s for 500 specs | Yes | Graph construction is O(V+E) |
| spec-check (structural) | <5s for 500 specs | Yes | Rule application per edge in graph |
| spec-check (semantic) | 30-120s | Depends on LLM | One API call per flagged conflict |
| spec-coverage | <5s for 500 specs | Yes | File scanning + regex |
| spec-sync | <30s total | Yes | Sum of above (minus semantic) |
| Reverse compiler | Minutes per module | Acceptable | Offline tool, not CI |

**Verdict:** The deterministic toolchain fits comfortably in CI time budgets (under 30 seconds for a typical project). Semantic/AI checks should be opt-in and run as a separate, non-blocking CI job.

---

## 4. Canonical Schema Design

This is the single most important design decision. Every tool depends on it. Here is a proposed schema with mandatory vs. optional fields, informed by the patterns in the sddbook.

### 4.1 Proposed Mandatory Fields

```yaml
# .spec.yaml -- Canonical Micro-Spec Schema
spec:
  id: string                    # REQUIRED. Unique identifier (kebab-case). e.g., "payment-create-intent"
  version: string               # REQUIRED. Semver. e.g., "1.2.0"
  status: enum                  # REQUIRED. One of: draft, review, approved, deprecated
  tier: enum                    # REQUIRED. One of: 1, 2, 3 (security/business/utility)

  context:
    system: string              # REQUIRED. Which system/service this belongs to
    description: string         # REQUIRED. One-paragraph purpose statement

  objective: string             # REQUIRED. What this module/endpoint/function does

  acceptance_criteria:          # REQUIRED. At least one AC
    - id: string                # REQUIRED. e.g., "AC-01"
      description: string       # REQUIRED. What behavior this validates
```

### 4.2 Proposed Optional Fields

```yaml
  # Optional but strongly recommended
  context:
    dependencies: [string]      # Other specs this depends on (for spec-resolve)
    depends_on:                 # Versioned dependency references
      - spec: string            # Spec ID
        version: string         # Semver range (e.g., "^1.0.0")

  constraints:                  # Structured constraints (for deterministic checking)
    - id: string                # e.g., "C-01"
      description: string       # Human-readable
      field: string             # Optional: which field this constrains
      rule: object              # Optional: structured rule (type, min, max, enum, pattern, required)

  acceptance_criteria:
    - id: string
      description: string
      references: [string]      # Constraint IDs this AC validates (for orphan detection)
      gap: boolean              # True if this AC was inferred but has no test

  error_cases:
    - condition: string
      behavior: string
      status_code: integer

  changelog:
    - version: string
      date: string
      type: enum                # initial, major, minor, patch
      description: string
      changes: [object]

  metadata:
    created: date
    last_modified: date
    author: string
    source_files: [string]      # For reverse-compiled specs
    test_files: [string]
    generated_from: object      # Reverse compiler provenance
```

### 4.3 Schema Design Recommendations

1. **Constraint duality.** Support both `description` (free-text, always present) and `rule` (structured, optional). Deterministic tools check `rule` fields; AI tools check `description` fields. This avoids forcing users into rigid constraint syntax while enabling automated checking for those who adopt it.

2. **AC-to-constraint references.** The `references` field on ACs enables orphan detection without NLP. If `C-03` is not referenced by any AC's `references` array, it is an orphan. Simple set math.

3. **Tier as a first-class field.** Putting `tier` in the spec itself (rather than in a separate config) means every tool can read enforcement levels directly. No external lookup needed.

4. **Provenance tracking.** The `generated_from` and `metadata` sections are critical for the reverse compiler. They distinguish human-authored specs from machine-generated drafts and track what source files contributed to each spec.

5. **JSON Schema for the schema.** The canonical schema must itself be a JSON Schema document (draft 2020-12), validated by Ajv. This is the "type definition for the type system" that the groundwork doc correctly identifies as the first deliverable.

---

## 5. Recommended Build Order

The groundwork doc's proposed build sequence is correct. Here it is with time estimates and dependencies:

| Phase | Tool | Estimated Effort | Depends On | Milestone |
|-------|------|-----------------|------------|-----------|
| **1** | Schema definition + `spec-parse` | 2-3 weeks | Nothing | Can validate any `.spec.yaml` file |
| **2** | `spec-resolve` | 2-3 weeks | Phase 1 | Can detect broken references and cycles |
| **3** | `spec-check` (orphan + breaking change) | 3-4 weeks | Phase 1, 2 | Can catch real bugs in spec graphs |
| **4** | `spec-coverage` | 2-3 weeks | Phase 1 | Can measure spec-to-test traceability |
| **5** | `spec-sync` | 1-2 weeks | Phase 1-4 | Can gate PRs on spec quality |
| **6** | Reverse compiler (structural) | 4-6 weeks | Phase 1 | Can bootstrap draft specs from TS code |
| **7** | Reverse compiler (AI gap-fill) | 2-3 weeks | Phase 6 | Can produce fuller draft specs |
| **8** | `spec-check` (semantic + gap analysis) | 4-6 weeks | Phase 3 | AI-assisted conflict and gap detection |

**Total estimated effort for MVP (Phases 1-5):** 10-15 weeks of focused development.
**Total estimated effort for full toolchain (Phases 1-8):** 20-30 weeks.

**Critical path:** Phase 1 (schema design) blocks everything. Get this right. Iterate on it with real specs from existing projects before building downstream tools.

---

## 6. Overall Verdict

### What Is Buildable (High Confidence)

- **spec-parse:** Standard YAML/JSON Schema validation. No novel engineering.
- **spec-resolve:** Textbook graph algorithms with off-the-shelf libraries.
- **spec-check (structural):** Orphan detection is set math. Breaking change detection is tree diff + classification. Structural conflict detection is constraint comparison across graph edges. All deterministic.
- **spec-coverage:** File scanning + annotation matching. Straightforward.
- **spec-sync:** CI orchestration of the above. Standard DevOps engineering.

### What Is Buildable but Hard (Medium Confidence)

- **Reverse compiler (structural extraction):** Requires deep knowledge of TypeScript's compiler API and framework-specific patterns. Buildable but labor-intensive, especially to handle the long tail of framework diversity. The 80/20 rule applies -- handle the common patterns, flag the rest for manual review.
- **Reverse compiler (AI gap-fill):** Works but produces drafts that need human curation. The quality depends heavily on prompt engineering and the quality of structural extraction feeding into it.

### What Is Research-Grade (Low Confidence for Full Automation)

- **Semantic conflict detection.** Requires NLP-level reasoning about spec intent. An LLM can flag suspicious pairs, but false positives will be a persistent problem. This should be an advisory tool, never a CI blocker.
- **Full gap analysis.** Reasoning about uncovered input spaces from free-text constraints is open-ended. Boundary-case enumeration for structured constraints is tractable; general gap detection is AI-dependent and approximate.

### The Honest Bottom Line

The core of this toolchain -- the deterministic parse/resolve/check/coverage/sync pipeline -- is a **3-4 month project for a small team (2-3 engineers)**. It uses proven technologies, follows established patterns from tools like Spectral and OpenAPI linters, and solves a real problem (spec quality enforcement in CI).

The reverse compiler adds another 2-3 months and is where the hard engineering lives. It is also the adoption enabler -- without it, teams must manually write specs for existing code, which creates a cold-start barrier.

The AI-assisted layers (semantic checks, gap analysis) are the speculative bets. They should be built last, gated behind explicit flags, and presented as advisory rather than authoritative.

The single biggest risk is not technical -- it is **schema design**. If the canonical YAML schema is wrong, every tool built on top of it will need to be reworked. Invest disproportionate time in Phase 1. Write specs for real projects (using the proposed schema) before building the toolchain that validates them.

---

## 7. Sources

- [ts-morph Documentation](https://ts-morph.com/)
- [ts-morph on GitHub](https://github.com/dsherret/ts-morph)
- [TypeScript Compiler API Wiki](https://github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API)
- [LibCST on GitHub (Instagram/Meta)](https://github.com/Instagram/LibCST)
- [LibCST vs AST comparison](https://libcst.readthedocs.io/en/latest/why_libcst.html)
- [Ajv JSON Schema Validator](https://ajv.js.org/)
- [Ajv on GitHub](https://github.com/ajv-validator/ajv)
- [Spectral OpenAPI Linter (Stoplight)](https://github.com/stoplightio/spectral)
- [Spectral Custom Rules Guide](https://stoplight.io/open-source/spectral)
- [graphlib on GitHub (DagreJS)](https://github.com/dagrejs/graphlib)
- [Zod JSON Schema Support](https://zod.dev/json-schema)
- [Zod Schema Introspection Discussion](https://github.com/colinhacks/zod/issues/4824)
- [API Linting with Spectral (Axway)](https://blog.axway.com/learning-center/apis/api-design/api-linting-with-spectral)
- [AST-based Refactoring with ts-morph](https://kimmo.blog/posts/8-ast-based-refactoring-with-ts-morph/)

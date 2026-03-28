# Specter: Architecture and MVP Design

> **Agent Role:** Architecture and MVP Design Agent
> **Date:** 2026-03-27
> **Status:** Design proposal — contingent on feasibility confirmation from peer agents

---

## 1. Product Name and Identity

**Name:** Specter
**Tagline:** "A type system for specs."

**Positioning:** Specter is a spec compiler toolchain that turns SDD micro-specs from passive YAML documents into an enforced, validated, dependency-resolved contract graph. It does for specifications what TypeScript did for JavaScript: adds a structural type layer that catches errors before runtime.

**Core Analogy (from SDD_NEXT_STEP_GROUNDWORK.md Section 2):**

| Programming Concept | Specter Equivalent |
|---|---|
| Type definition | Constraint (defines what is allowed) |
| Function signature | Acceptance Criterion (defines input-to-output) |
| Import statement | `depends_on` (creates a contract between specs) |
| Type error | Spec conflict (caught before tests run) |
| Unused variable | Orphan constraint (no AC references it) |
| Missing null check | Spec gap (a path with no AC coverage) |

**One-liner for README:** Specter validates, links, and type-checks your `.spec.yaml` files the way `tsc` validates your `.ts` files — so spec errors are caught before code is ever generated.

---

## 2. MVP Scope

### 2.1 Which Tools Are In MVP?

| Tool | MVP? | Rationale |
|---|---|---|
| `spec-parse` | YES | Foundation. Nothing works without validated specs. |
| `spec-resolve` | YES | Dependency graph is core to the "type system" value prop. |
| `spec-check` | YES (partial) | Structural validation + orphan detection only. |
| `spec-coverage` | YES (basic) | Traceability matrix ties the story together. |
| `spec-sync` | NO (M5) | CI integration is a polish feature — users can run CLI manually first. |
| reverse compiler | NO (M6) | Highest risk, highest complexity, least necessary for proving the concept. |

**Rationale:** The MVP must prove that specs can be parsed, validated, linked into a graph, and checked for structural issues. This is the "type system" core. CI integration and reverse compilation are adoption accelerators, not proof-of-concept features.

### 2.2 What MVP spec-check Includes

**IN scope (deterministic, structural checks):**

1. **Schema validation** — Every `.spec.yaml` conforms to the canonical JSON Schema. Missing required fields, wrong types, malformed constraint expressions all fail here.
2. **Orphan constraint detection** — Constraints that no acceptance criterion references. These are either dead logic or missing test coverage (a warning, not an error).
3. **Dangling dependency detection** — `depends_on` references that point to spec IDs that do not exist in the registry.
4. **Circular dependency detection** — Cycles in the `depends_on` graph.
5. **Version incompatibility detection** — Spec A depends on B@^1.0, but B is at 2.0. Semver range checks on the dependency graph.
6. **Duplicate ID detection** — Two specs with the same `id` field.

**OUT of scope for MVP (requires AI or deep semantic analysis):**

- Semantic conflict detection (e.g., "Spec A assumes synchronous, Spec B assumes async"). This requires LLM-assisted reasoning and is explicitly Phase 6 in the groundwork document.
- Gap detection (input paths with no AC). This requires reasoning about the input space and is AI-assisted territory.
- Breaking change detection against git history. Valuable but requires git integration complexity.

### 2.3 What MVP spec-coverage Includes

**IN scope:**

- Parse all `.spec.yaml` files and extract AC IDs.
- Scan test files for `@spec` and `@ac` annotations (comments or decorators).
- Produce a traceability matrix: spec ID -> AC IDs -> test file(s).
- Report coverage percentage per spec and overall.
- Identify specs with zero test coverage ("SPEC ONLY") and ACs with no matching test ("UNCOVERED").

**OUT of scope:**

- Code file mapping (spec -> source files). Requires AST parsing of the target codebase.
- Automatic annotation detection without explicit `@spec`/`@ac` markers.

### 2.4 What the MVP Does NOT Include (see Section 8)

Explicitly excluded: reverse compiler, AI gap-filling, Python target support, IDE plugins, web dashboard, semantic conflict detection.

---

## 3. Tech Stack Decision

### 3.1 Language: TypeScript (Node.js CLI)

**Decision: TypeScript.**

**Justification:**

1. **Dogfooding alignment.** The SDD course uses TypeScript as its primary example language. Both JWTMS and OpenWatch have TypeScript frontends. Specter's first reverse-compilation target will be TypeScript. Building the tool in the same language eliminates context-switching and lets us use the TypeScript compiler API directly for future AST parsing.

2. **Ecosystem fit.** The best YAML parsing, JSON Schema validation, CLI framework, and AST tooling libraries are in the Node.js ecosystem. No other language has this combination at this maturity level.

3. **Developer audience.** Specter's target users are the same developers reading the SDD course — overwhelmingly TypeScript/JavaScript developers. A `npm install -g specter` onboarding is frictionless for them.

4. **Contribution barrier.** TypeScript has the lowest contribution barrier for the SDD community. Rust or Go would limit contributors to a much smaller pool.

**Why not Rust?** Performance is not the bottleneck — spec graphs will be hundreds of nodes, not millions. Rust's compilation time and learning curve would slow iteration without meaningful benefit. If Specter succeeds and needs to handle massive monorepo graphs (10,000+ specs), a Rust rewrite of the hot path (graph resolution) is a tractable future optimization.

**Why not Python?** Python lacks the TypeScript compiler API access needed for the reverse compiler, has weaker CLI tooling (compared to Node.js), and the SDD course's primary audience is TypeScript-first. Python support as a target language is a post-MVP feature.

**Why not Go?** Go's type system and lack of generics maturity (pre-1.18 patterns still dominant) make it a poor fit for the heavily structural, schema-driven code that Specter requires. Go would be viable but offers no advantage over TypeScript here.

### 3.2 Specific Libraries

| Concern | Library | Version | Rationale |
|---|---|---|---|
| **Package manager** | pnpm | 9.x | Fastest installs, strict dependency resolution, workspace support for monorepo if needed. |
| **Build system** | tsup | 8.x | Zero-config TypeScript bundling. Produces CJS + ESM. Fast (esbuild under the hood). |
| **Testing** | Vitest | 3.x | Native TypeScript, fast, compatible with Jest API. Aligns with SDD course examples. |
| **CLI framework** | Commander.js | 13.x | Lightweight, well-documented, 50M+ weekly downloads. Oclif is overkill for MVP; yargs API is messier. |
| **YAML parsing** | yaml (eemeli/yaml) | 2.x | Full YAML 1.2 spec, preserves comments (important for round-tripping), TypeScript types included. |
| **JSON Schema validation** | Ajv | 8.x | Industry standard. Fastest JSON Schema validator. Supports draft-2020-12. |
| **Graph library** | graphlib | 2.x | Lightweight directed graph with cycle detection, topological sort, and shortest path. Used by dagre. If more power needed, migrate to graphology later. |
| **AST parsing (future M6)** | ts-morph | 24.x | Wraps the TypeScript compiler API with a developer-friendly interface. Not needed until reverse compiler milestone. |
| **Output formatting** | chalk + cli-table3 | 5.x / 0.6.x | Colored terminal output and formatted tables for coverage reports. |

### 3.3 Node.js Version Target

**Minimum: Node.js 20 LTS** (active LTS until April 2026). This gives us native ESM support, `node:test` as a fallback, and the `fs/promises` API without polyfills.

---

## 4. Project Structure

```
specter/
├── .github/
│   └── workflows/
│       └── ci.yml                    # GitHub Actions: lint, test, build
├── specs/                            # Specter's own specs (dogfooding)
│   ├── specter.registry.yaml         # Master spec registry
│   ├── spec-parse.spec.yaml          # Parser spec
│   ├── spec-resolve.spec.yaml        # Resolver spec
│   ├── spec-check.spec.yaml          # Checker spec
│   └── spec-coverage.spec.yaml       # Coverage tool spec
├── src/
│   ├── index.ts                      # CLI entry point
│   ├── cli/
│   │   ├── commands/
│   │   │   ├── parse.ts              # `specter parse` command
│   │   │   ├── resolve.ts            # `specter resolve` command
│   │   │   ├── check.ts              # `specter check` command
│   │   │   ├── coverage.ts           # `specter coverage` command
│   │   │   └── init.ts               # `specter init` — scaffold a new spec
│   │   └── output/
│   │       ├── formatters.ts         # Table, JSON, plain text output
│   │       └── colors.ts             # Terminal color helpers
│   ├── core/
│   │   ├── schema/
│   │   │   ├── spec-schema.json      # THE canonical JSON Schema
│   │   │   ├── validator.ts          # Ajv-based schema validation
│   │   │   └── types.ts              # TypeScript types derived from schema
│   │   ├── parser/
│   │   │   ├── parse.ts              # YAML -> validated SpecAST
│   │   │   ├── spec-ast.ts           # SpecAST type definitions
│   │   │   └── errors.ts             # Parse error types
│   │   ├── resolver/
│   │   │   ├── resolve.ts            # Build dependency graph
│   │   │   ├── graph.ts              # Graph operations (cycle detection, topo sort)
│   │   │   └── registry.ts           # Spec registry (discovers all .spec.yaml files)
│   │   ├── checker/
│   │   │   ├── check.ts              # Orchestrates all checks
│   │   │   ├── rules/
│   │   │   │   ├── orphan-constraints.ts
│   │   │   │   ├── dangling-deps.ts
│   │   │   │   ├── circular-deps.ts
│   │   │   │   ├── version-compat.ts
│   │   │   │   └── duplicate-ids.ts
│   │   │   └── types.ts              # Check result types (error, warning, info)
│   │   └── coverage/
│   │       ├── coverage.ts           # Scan tests for @spec/@ac annotations
│   │       ├── matrix.ts             # Build traceability matrix
│   │       └── report.ts             # Generate coverage report
│   └── utils/
│       ├── file-discovery.ts         # Find .spec.yaml files recursively
│       ├── semver.ts                 # Semver parsing and range matching
│       └── logger.ts                 # Structured logging
├── tests/
│   ├── unit/
│   │   ├── parser/
│   │   │   └── parse.test.ts
│   │   ├── resolver/
│   │   │   └── resolve.test.ts
│   │   ├── checker/
│   │   │   ├── orphan-constraints.test.ts
│   │   │   ├── dangling-deps.test.ts
│   │   │   ├── circular-deps.test.ts
│   │   │   ├── version-compat.test.ts
│   │   │   └── duplicate-ids.test.ts
│   │   └── coverage/
│   │       └── coverage.test.ts
│   ├── integration/
│   │   └── cli.test.ts              # End-to-end CLI tests
│   └── fixtures/
│       ├── valid/                    # Valid spec files for testing
│       ├── invalid/                  # Invalid spec files (expected failures)
│       └── projects/                 # Mock project structures for coverage tests
├── research/                         # Agent research documents
│   └── 04_ARCHITECTURE_MVP.md        # This document
├── package.json
├── tsconfig.json
├── vitest.config.ts
├── .eslintrc.cjs
├── .prettierrc
└── README.md
```

**Key design decisions:**

1. **`specs/` directory at root.** Specter eats its own dogfood. Every tool has a spec written before implementation.
2. **`core/` is framework-agnostic.** The parser, resolver, checker, and coverage modules have no CLI dependencies. They accept inputs and return typed results. The CLI layer in `cli/` wraps them with argument parsing and output formatting. This enables future use as a library (e.g., for IDE plugins or programmatic access).
3. **Checker rules are individual files.** Each check rule is a function that takes a SpecAST (or SpecGraph) and returns a list of diagnostics. New rules are added by creating a new file and registering it — no monolithic check function.
4. **Tests mirror source structure.** `tests/unit/parser/` maps to `src/core/parser/`. Fixtures are shared.

---

## 5. The Canonical Spec Schema

This is the most critical deliverable. Every tool in the chain depends on this definition.

### 5.1 JSON Schema (Draft 2020-12)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://specter.dev/schemas/spec/v1.0.0",
  "title": "SDD Micro-Spec",
  "description": "Canonical schema for Spec-Driven Development .spec.yaml files",
  "type": "object",
  "required": ["spec"],
  "properties": {
    "spec": {
      "type": "object",
      "required": ["id", "version", "status", "tier", "context", "objective", "constraints", "acceptance_criteria"],
      "properties": {

        "id": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9-]*$",
          "description": "Unique identifier for this spec. Lowercase, alphanumeric, hyphens only. Used in depends_on references.",
          "examples": ["user-registration", "payment-create-intent", "auth-jwt-validation"]
        },

        "version": {
          "type": "string",
          "pattern": "^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(-[a-zA-Z0-9.]+)?$",
          "description": "Semantic version (MAJOR.MINOR.PATCH with optional pre-release tag).",
          "examples": ["1.0.0", "2.1.0", "1.0.0-draft"]
        },

        "status": {
          "type": "string",
          "enum": ["draft", "review", "approved", "deprecated", "removed"],
          "description": "Lifecycle status. Only 'approved' specs are enforced by spec-sync."
        },

        "tier": {
          "type": "integer",
          "enum": [1, 2, 3],
          "description": "Risk tier. 1 = Security/Money (strictest enforcement), 2 = Core Business Logic, 3 = Utility/Internal."
        },

        "context": {
          "type": "object",
          "required": ["system"],
          "properties": {
            "system": {
              "type": "string",
              "description": "What system or service does this spec belong to?"
            },
            "feature": {
              "type": "string",
              "description": "What feature area within the system?"
            },
            "dependencies": {
              "type": "array",
              "items": { "type": "string" },
              "description": "External dependencies (libraries, services, APIs) this spec relies on."
            },
            "existing_patterns": {
              "type": "string",
              "description": "Relevant coding patterns, conventions, or architectural decisions."
            }
          },
          "additionalProperties": true
        },

        "objective": {
          "type": "object",
          "required": ["summary"],
          "properties": {
            "summary": {
              "type": "string",
              "description": "1-3 sentence description of what this spec defines. Uses the Delta Principle: describes the change, not the state."
            },
            "scope": {
              "type": "object",
              "properties": {
                "includes": {
                  "type": "array",
                  "items": { "type": "string" }
                },
                "excludes": {
                  "type": "array",
                  "items": { "type": "string" }
                }
              }
            }
          }
        },

        "constraints": {
          "type": "array",
          "minItems": 1,
          "items": {
            "$ref": "#/$defs/constraint"
          },
          "description": "Inviolable rules. Each constraint is a hard boundary on the solution space."
        },

        "acceptance_criteria": {
          "type": "array",
          "minItems": 1,
          "items": {
            "$ref": "#/$defs/acceptance_criterion"
          },
          "description": "Testable conditions that define 'done'. Each AC maps to at least one test."
        },

        "depends_on": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/dependency_ref"
          },
          "description": "Other specs this spec depends on. Creates edges in the dependency graph."
        },

        "trust_level": {
          "type": "string",
          "enum": ["full_auto", "auto_with_review", "human_required"],
          "description": "How much autonomy the AI has when implementing this spec. From Module 05 Ch. 3 trust spectrum."
        },

        "environment": {
          "type": "object",
          "properties": {
            "required_vars": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Environment variables this spec's implementation requires."
            },
            "deployment_targets": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Where this feature deploys (e.g., 'production', 'staging', 'edge')."
            }
          }
        },

        "tags": {
          "type": "array",
          "items": { "type": "string" },
          "description": "Free-form tags for categorization and filtering."
        },

        "changelog": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/changelog_entry"
          },
          "description": "Version history. Most recent entry first."
        },

        "generated_from": {
          "type": "object",
          "properties": {
            "source_file": { "type": "string" },
            "test_files": {
              "type": "array",
              "items": { "type": "string" }
            },
            "extraction_date": {
              "type": "string",
              "format": "date"
            }
          },
          "description": "Present only on reverse-compiled specs. Tracks provenance."
        }
      },
      "additionalProperties": false
    }
  },

  "$defs": {

    "constraint": {
      "type": "object",
      "required": ["id", "description"],
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^C-\\d{2,}$",
          "description": "Constraint ID. Format: C-01, C-02, etc.",
          "examples": ["C-01", "C-02", "C-10"]
        },
        "description": {
          "type": "string",
          "description": "Human-readable constraint statement. Should use RFC 2119 language (MUST, MUST NOT, SHALL, etc.)."
        },
        "type": {
          "type": "string",
          "enum": ["technical", "security", "performance", "accessibility", "business"],
          "description": "Category of constraint."
        },
        "enforcement": {
          "type": "string",
          "enum": ["error", "warning", "info"],
          "default": "error",
          "description": "Severity when this constraint is violated."
        },
        "validation": {
          "$ref": "#/$defs/constraint_validation",
          "description": "Optional machine-readable validation rule. If present, spec-check can verify this structurally."
        }
      },
      "additionalProperties": false
    },

    "constraint_validation": {
      "type": "object",
      "properties": {
        "field": {
          "type": "string",
          "description": "The field this validation applies to."
        },
        "rule": {
          "type": "string",
          "enum": ["type", "min", "max", "pattern", "enum", "required", "format", "custom"],
          "description": "The type of validation rule."
        },
        "value": {
          "description": "The value for the rule. Type depends on the rule type.",
          "oneOf": [
            { "type": "string" },
            { "type": "number" },
            { "type": "boolean" },
            { "type": "array", "items": { "type": "string" } }
          ]
        }
      },
      "required": ["field", "rule", "value"]
    },

    "acceptance_criterion": {
      "type": "object",
      "required": ["id", "description"],
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^AC-\\d{2,}$",
          "description": "Acceptance criterion ID. Format: AC-01, AC-02, etc.",
          "examples": ["AC-01", "AC-02", "AC-10"]
        },
        "description": {
          "type": "string",
          "description": "Human-readable description of the expected behavior."
        },
        "inputs": {
          "type": "object",
          "additionalProperties": true,
          "description": "Input values or conditions that trigger this behavior."
        },
        "expected_output": {
          "type": "object",
          "additionalProperties": true,
          "description": "What should happen when the inputs are provided."
        },
        "error_cases": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["condition", "expected_behavior"],
            "properties": {
              "condition": { "type": "string" },
              "expected_behavior": { "type": "string" }
            }
          },
          "description": "Error conditions and their expected handling."
        },
        "references_constraints": {
          "type": "array",
          "items": { "type": "string", "pattern": "^C-\\d{2,}$" },
          "description": "Which constraints this AC validates. Used for orphan detection."
        },
        "gap": {
          "type": "boolean",
          "default": false,
          "description": "True if this AC was identified by gap analysis (no existing test covers it)."
        },
        "priority": {
          "type": "string",
          "enum": ["critical", "high", "medium", "low"],
          "description": "Implementation priority."
        }
      },
      "additionalProperties": false
    },

    "dependency_ref": {
      "type": "object",
      "required": ["spec_id"],
      "properties": {
        "spec_id": {
          "type": "string",
          "pattern": "^[a-z][a-z0-9-]*$",
          "description": "The id of the spec being depended on."
        },
        "version_range": {
          "type": "string",
          "description": "Semver range (e.g., '^1.0.0', '>=2.0.0 <3.0.0'). If omitted, any version is accepted.",
          "examples": ["^1.0.0", ">=2.0.0", "~1.2.0"]
        },
        "relationship": {
          "type": "string",
          "enum": ["requires", "extends", "conflicts_with"],
          "default": "requires",
          "description": "Nature of the dependency."
        }
      },
      "additionalProperties": false
    },

    "changelog_entry": {
      "type": "object",
      "required": ["version", "date", "description"],
      "properties": {
        "version": {
          "type": "string",
          "pattern": "^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(-[a-zA-Z0-9.]+)?$"
        },
        "date": {
          "type": "string",
          "format": "date"
        },
        "author": {
          "type": "string"
        },
        "type": {
          "type": "string",
          "enum": ["initial", "major", "minor", "patch"]
        },
        "description": {
          "type": "string"
        },
        "changes": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["type", "detail"],
            "properties": {
              "type": {
                "type": "string",
                "enum": ["addition", "removal", "modification", "deprecation"]
              },
              "section": { "type": "string" },
              "detail": { "type": "string" }
            }
          }
        }
      },
      "additionalProperties": false
    }
  }
}
```

### 5.2 Tier Definitions

Tiers control enforcement strictness in spec-check and spec-sync:

| Tier | Label | Examples | Enforcement |
|---|---|---|---|
| **1** | Security / Money | Payment processing, auth/JWT, PHI encryption, RBAC | All checks are errors. 100% spec coverage required. Breaking changes require migration spec. |
| **2** | Core Business Logic | Booking flow, availability engine, scan pipeline, drift detection | Conflicts are errors, orphans are warnings. 80% spec coverage required. |
| **3** | Utility / Internal | Logging, formatters, internal helpers, dev tooling | Conflicts are warnings, orphans are info. 50% spec coverage required. |

These tiers map directly to the risk assessments in both the JWTMS audit (Section 5: Money Path = Tier 1, Availability Engine = Tier 2) and the OpenWatch audit (Section 5: Remediation Pipeline = Tier 1, Scan Pipeline = Tier 2).

### 5.3 Design Decisions and Rationale

**Why `spec.id` uses kebab-case:** IDs appear in `depends_on` references, CLI output, test annotations (`@spec payment-create-intent`), and file names. Kebab-case is the most readable in all these contexts and avoids case-sensitivity issues.

**Why constraints and ACs have explicit IDs (C-01, AC-01):** Cross-referencing requires stable identifiers. `references_constraints: ["C-01", "C-03"]` on an AC enables orphan detection. Test annotations (`@ac AC-01`) enable spec coverage tracking. Without IDs, references would rely on description matching, which is fragile.

**Why `additionalProperties: false` on most objects:** Strict validation catches typos early. A spec with `aceeptance_criteria` (typo) would silently be ignored without strict mode. With strict mode, it fails at parse time.

**Why `generated_from` is a separate field:** Reverse-compiled specs need provenance tracking. The `generated_from` field marks a spec as machine-generated and links it to its source code. This enables spec-check to treat draft/generated specs differently (lower enforcement).

**Why `gap: true` on ACs:** The reverse compiler (M6) identifies constraints in code that have no matching test. These become ACs with `gap: true`, flagging them for human attention. This is the "spec gap" concept from the groundwork document Section 3.5.

---

## 6. Dogfooding Plan

Specter's own specs, written before implementation. These live in `specter/specs/`.

### 6.1 spec-parse.spec.yaml

```yaml
spec:
  id: spec-parse
  version: 1.0.0
  status: approved
  tier: 1

  context:
    system: Specter toolchain
    feature: YAML-to-AST parser
    dependencies:
      - yaml (eemeli/yaml 2.x)
      - ajv (8.x)

  objective:
    summary: >
      Parse .spec.yaml files into a validated, typed SpecAST.
      Reject malformed specs with actionable error messages.
    scope:
      includes:
        - YAML parsing
        - JSON Schema validation against canonical schema
        - Typed AST construction
        - Error reporting with line numbers and field paths
      excludes:
        - Dependency resolution (that is spec-resolve)
        - Semantic validation (that is spec-check)
        - File discovery (that is the registry)

  constraints:
    - id: C-01
      description: MUST validate against the canonical JSON Schema (spec-schema.json)
      type: technical
      enforcement: error
    - id: C-02
      description: MUST report errors with the YAML line number and JSON path of the failing field
      type: technical
      enforcement: error
    - id: C-03
      description: MUST NOT silently ignore unknown fields (additionalProperties enforcement)
      type: technical
      enforcement: error
    - id: C-04
      description: MUST produce a typed SpecAST object on success, never raw YAML
      type: technical
      enforcement: error
    - id: C-05
      description: MUST handle YAML syntax errors gracefully (not crash, return structured error)
      type: technical
      enforcement: error
    - id: C-06
      description: MUST support YAML anchors and aliases (&anchor / *alias)
      type: technical
      enforcement: warning

  acceptance_criteria:
    - id: AC-01
      description: Valid spec file is parsed into a SpecAST with all fields populated
      inputs:
        file: "fixtures/valid/simple.spec.yaml"
      expected_output:
        type: "SpecAST"
        fields_present: ["id", "version", "status", "tier", "context", "objective", "constraints", "acceptance_criteria"]
      references_constraints: ["C-01", "C-04"]

    - id: AC-02
      description: Spec missing required field 'id' returns error with field path
      inputs:
        file: "fixtures/invalid/missing-id.spec.yaml"
      expected_output:
        type: "ParseError"
        error_path: "spec.id"
        error_type: "required"
      references_constraints: ["C-01", "C-02"]

    - id: AC-03
      description: Spec with unknown field returns error identifying the extra field
      inputs:
        file: "fixtures/invalid/extra-field.spec.yaml"
      expected_output:
        type: "ParseError"
        error_type: "additionalProperties"
      references_constraints: ["C-03"]

    - id: AC-04
      description: Malformed YAML (bad indentation) returns error with line number
      inputs:
        file: "fixtures/invalid/bad-yaml.spec.yaml"
      expected_output:
        type: "ParseError"
        has_line_number: true
      references_constraints: ["C-02", "C-05"]

    - id: AC-05
      description: Spec with invalid version format returns error
      inputs:
        file: "fixtures/invalid/bad-version.spec.yaml"
      expected_output:
        type: "ParseError"
        error_path: "spec.version"
        error_type: "pattern"
      references_constraints: ["C-01"]

    - id: AC-06
      description: Spec with all optional fields omitted parses successfully
      inputs:
        file: "fixtures/valid/minimal.spec.yaml"
      expected_output:
        type: "SpecAST"
        optional_fields_absent: ["depends_on", "trust_level", "environment", "tags", "changelog"]
      references_constraints: ["C-01", "C-04"]

    - id: AC-07
      description: Spec using YAML anchors and aliases is parsed correctly
      inputs:
        file: "fixtures/valid/with-anchors.spec.yaml"
      expected_output:
        type: "SpecAST"
        anchors_resolved: true
      references_constraints: ["C-06"]

    - id: AC-08
      description: Multiple validation errors are collected and returned together (not fail-fast)
      inputs:
        file: "fixtures/invalid/multiple-errors.spec.yaml"
      expected_output:
        type: "ParseError[]"
        min_error_count: 2
      references_constraints: ["C-02"]

  changelog:
    - version: "1.0.0"
      date: "2026-03-27"
      author: "architecture-agent"
      type: initial
      description: "Initial spec for the Specter YAML parser"
```

### 6.2 spec-resolve.spec.yaml

```yaml
spec:
  id: spec-resolve
  version: 1.0.0
  status: approved
  tier: 1

  context:
    system: Specter toolchain
    feature: Dependency graph builder
    dependencies:
      - graphlib (2.x)
      - spec-parse (spec-parse.spec.yaml)

  objective:
    summary: >
      Build a directed acyclic graph from the depends_on fields across all specs
      in a project. Detect structural graph issues: missing references, circular
      dependencies, and version incompatibilities.
    scope:
      includes:
        - Discover all .spec.yaml files in a directory tree
        - Parse each spec using spec-parse
        - Build a directed graph where nodes are specs and edges are depends_on references
        - Detect and report circular dependencies
        - Detect and report dangling references (depends_on a spec that does not exist)
        - Detect and report semver range mismatches
        - Output: the SpecGraph (typed graph object) + list of diagnostics
      excludes:
        - Semantic conflict detection (that is spec-check)
        - Transitive dependency flattening (future feature)
        - Lock file generation (future feature)

  constraints:
    - id: C-01
      description: MUST discover .spec.yaml files recursively from the project root
      type: technical
      enforcement: error
    - id: C-02
      description: MUST respect .specterignore file if present (glob patterns, like .gitignore)
      type: technical
      enforcement: warning
    - id: C-03
      description: MUST detect all circular dependencies, not just the first one found
      type: technical
      enforcement: error
    - id: C-04
      description: MUST report the full cycle path for each circular dependency (e.g., A -> B -> C -> A)
      type: technical
      enforcement: error
    - id: C-05
      description: MUST validate version_range fields against semver syntax
      type: technical
      enforcement: error
    - id: C-06
      description: MUST produce a topologically sorted build order when no cycles exist
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: Three specs with linear dependency chain (A -> B -> C) produce correct graph
      inputs:
        specs: ["a.spec.yaml (depends_on: b)", "b.spec.yaml (depends_on: c)", "c.spec.yaml"]
      expected_output:
        graph_edges: [["a", "b"], ["b", "c"]]
        topological_order: ["c", "b", "a"]
        diagnostics: []
      references_constraints: ["C-06"]

    - id: AC-02
      description: Circular dependency (A -> B -> A) is detected and reported with full path
      inputs:
        specs: ["a.spec.yaml (depends_on: b)", "b.spec.yaml (depends_on: a)"]
      expected_output:
        diagnostics:
          - type: "error"
            rule: "circular-dependency"
            cycle: ["a", "b", "a"]
      references_constraints: ["C-03", "C-04"]

    - id: AC-03
      description: Dangling reference (A depends on nonexistent Z) is reported
      inputs:
        specs: ["a.spec.yaml (depends_on: z)"]
      expected_output:
        diagnostics:
          - type: "error"
            rule: "dangling-reference"
            spec: "a"
            missing: "z"
      references_constraints: ["C-01"]

    - id: AC-04
      description: Version mismatch (A depends on B@^1.0.0, B is at 2.0.0) is reported
      inputs:
        specs: ["a.spec.yaml (depends_on: b@^1.0.0)", "b.spec.yaml (version: 2.0.0)"]
      expected_output:
        diagnostics:
          - type: "error"
            rule: "version-incompatibility"
            spec: "a"
            dependency: "b"
            required: "^1.0.0"
            actual: "2.0.0"
      references_constraints: ["C-05"]

    - id: AC-05
      description: Multiple independent specs with no dependencies produce a valid graph with no edges
      inputs:
        specs: ["a.spec.yaml", "b.spec.yaml", "c.spec.yaml"]
      expected_output:
        graph_edges: []
        diagnostics: []
      references_constraints: ["C-01"]

    - id: AC-06
      description: .specterignore excludes matching files from discovery
      inputs:
        specterignore: "vendor/**"
        file_tree: ["specs/a.spec.yaml", "vendor/b.spec.yaml"]
      expected_output:
        discovered_specs: ["specs/a.spec.yaml"]
      references_constraints: ["C-02"]

    - id: AC-07
      description: Diamond dependency (A -> B, A -> C, B -> D, C -> D) is resolved correctly
      inputs:
        specs: ["a (depends_on: b, c)", "b (depends_on: d)", "c (depends_on: d)", "d"]
      expected_output:
        graph_edges: [["a","b"], ["a","c"], ["b","d"], ["c","d"]]
        topological_order_starts_with: "d"
        diagnostics: []
      references_constraints: ["C-06"]

  depends_on:
    - spec_id: spec-parse
      version_range: "^1.0.0"
      relationship: requires

  changelog:
    - version: "1.0.0"
      date: "2026-03-27"
      author: "architecture-agent"
      type: initial
      description: "Initial spec for the Specter dependency resolver"
```

---

## 7. MVP Milestone Plan

### M1: Schema + spec-parse (Weeks 1-2)

**Goal:** Define the canonical schema and build a parser that validates specs against it.

**Deliverables:**
- `src/core/schema/spec-schema.json` — the canonical JSON Schema (Section 5 of this document)
- `src/core/schema/types.ts` — TypeScript types derived from the schema
- `src/core/parser/parse.ts` — YAML-to-SpecAST parser with Ajv validation
- `specter parse <file>` CLI command
- Tests for all 8 ACs in `spec-parse.spec.yaml`
- Test fixtures: 4+ valid specs, 6+ invalid specs covering all error types

**Exit criteria:** `specter parse fixtures/valid/simple.spec.yaml` outputs a validated SpecAST. `specter parse fixtures/invalid/missing-id.spec.yaml` outputs a structured error with line number and field path.

### M2: spec-resolve (Weeks 3-4)

**Goal:** Build the dependency graph from parsed specs.

**Deliverables:**
- `src/core/resolver/registry.ts` — recursive `.spec.yaml` file discovery with `.specterignore` support
- `src/core/resolver/resolve.ts` — graph construction from `depends_on` fields
- `src/core/resolver/graph.ts` — cycle detection (Tarjan's algorithm), topological sort, version range checking
- `specter resolve <directory>` CLI command — outputs the spec graph and any structural diagnostics
- Tests for all 7 ACs in `spec-resolve.spec.yaml`

**Exit criteria:** `specter resolve ./specs` outputs a dependency graph visualization (text-based) and reports any cycles, dangling refs, or version mismatches.

### M3: spec-check (Weeks 5-6)

**Goal:** Walk the graph and run structural validation checks.

**Deliverables:**
- `src/core/checker/rules/orphan-constraints.ts` — constraints not referenced by any AC
- `src/core/checker/rules/dangling-deps.ts` — (already implemented in M2, exposed as a check rule)
- `src/core/checker/rules/circular-deps.ts` — (already implemented in M2, exposed as a check rule)
- `src/core/checker/rules/version-compat.ts` — (already implemented in M2, exposed as a check rule)
- `src/core/checker/rules/duplicate-ids.ts` — two specs with the same `id`
- `specter check <directory>` CLI command — runs all checks, outputs diagnostics with severity
- Spec file: `specs/spec-check.spec.yaml`

**Exit criteria:** `specter check ./specs` runs all 5 check rules and outputs a diagnostic report. Orphan constraints in Specter's own specs are identified and fixed (dogfooding).

### M4: spec-coverage (Weeks 7-8)

**Goal:** Build the traceability matrix from specs to tests.

**Deliverables:**
- `src/core/coverage/coverage.ts` — scan test files for `@spec` and `@ac` annotations
- `src/core/coverage/matrix.ts` — build the spec-to-AC-to-test mapping
- `src/core/coverage/report.ts` — generate coverage report (table, JSON, or plain text)
- `specter coverage <directory>` CLI command
- Spec file: `specs/spec-coverage.spec.yaml`
- Annotation convention defined: `// @spec spec-parse` and `// @ac AC-01` in test files

**Exit criteria:** `specter coverage .` on the Specter repo itself reports 100% spec coverage for spec-parse and spec-resolve (dogfooding proof).

### M5: spec-sync / CI Integration (Weeks 9-10)

**Goal:** Wire Specter into CI so spec violations fail the build.

**Deliverables:**
- `specter ci` command — runs parse + resolve + check + coverage in sequence, exits with non-zero on failure
- `.github/workflows/specter.yml` — GitHub Action that runs `specter ci`
- Configuration file: `.specterrc.yaml` — per-project settings (spec directory, test directory, coverage thresholds per tier, ignored rules)
- Tier-based enforcement: Tier 1 specs have stricter thresholds than Tier 3

**Exit criteria:** A PR to the Specter repo that introduces a malformed spec or drops below coverage threshold is blocked by CI.

### M6: Reverse Compiler — TypeScript Only (Weeks 11-14)

**Goal:** Extract draft specs from existing TypeScript code.

**Deliverables:**
- `src/core/reverse/extract.ts` — AST-based structural extraction using ts-morph
- `src/core/reverse/infer-constraints.ts` — extract Zod/TypeScript validation rules as constraints
- `src/core/reverse/infer-ac.ts` — extract test assertions as acceptance criteria
- `specter reverse <file.ts>` CLI command — outputs a draft `.spec.yaml`
- Draft specs are marked `status: draft` and `generated_from` is populated
- Test against real files from JWTMS or OpenWatch as validation

**Exit criteria:** `specter reverse src/auth/login.ts` produces a valid `.spec.yaml` that passes `specter parse`. The generated spec captures function signatures, Zod constraints, and test-derived ACs.

---

## 8. What NOT to Build in MVP

These items are explicitly out of scope. They are documented here so that scope creep does not pull them into the MVP under the guise of "it would be easy to add."

### 8.1 AI-Assisted Semantic Conflict Detection

**What it is:** Using an LLM to detect contradictions between specs that are not structurally apparent (e.g., "Spec A assumes synchronous processing" vs. "Spec B assumes eventual consistency").

**Why not MVP:** This requires integrating an LLM into the toolchain, which introduces non-determinism, API costs, and latency. The groundwork document (Section 6.1) explicitly warns that "subtle conflicts require AI-assisted semantic checking" and that the tool would have a "probabilistic layer." The MVP must prove the deterministic structural layer works first. Semantic checking is Phase 6 in the groundwork build sequence.

### 8.2 Python Target Support

**What it is:** The reverse compiler extracting specs from Python code (Pydantic models, FastAPI routes, pytest tests).

**Why not MVP:** OpenWatch is the Python target, but adding Python AST parsing (via child_process to a Python script, or a WASM-compiled Python parser) doubles the surface area of the reverse compiler. TypeScript first. Python support is the first post-MVP feature if OpenWatch adoption is a priority.

### 8.3 Multi-Language Reverse Compilation

**What it is:** Supporting Go, Rust, Java, or other languages as reverse compiler targets.

**Why not MVP:** Each language requires its own AST parser, its own constraint detection heuristics, and its own test-pattern extraction. This is a combinatorial expansion. TypeScript only for the foreseeable future.

### 8.4 IDE Plugins

**What it is:** VS Code extension, JetBrains plugin, or Neovim integration that shows spec diagnostics inline.

**Why not MVP:** IDE plugins are an adoption convenience, not a proof of concept. The CLI provides the same diagnostics. Plugins can be built once the diagnostic format is stable (after M3). The VS Code extension would be the first candidate (using the Language Server Protocol to surface `specter check` diagnostics).

### 8.5 Web Dashboard

**What it is:** A browser-based UI showing the spec dependency graph, coverage matrix, and check results.

**Why not MVP:** This is a visualization layer on top of the data Specter already produces. It can be built as a separate project consuming `specter check --json` and `specter coverage --json` output. It is not needed to prove the concept.

### 8.6 Gap Detection (AI-Assisted Input Space Analysis)

**What it is:** An LLM reads a spec and identifies uncovered input paths ("You have no AC for the case where `amount` is zero").

**Why not MVP:** Same rationale as semantic conflict detection. This is the AI-assisted layer that sits on top of the deterministic structural layer. Build the deterministic layer first.

### 8.7 Breaking Change Detection Against Git History

**What it is:** `specter check` compares the current version of a spec against the previous version in git and validates that version bumps match the change type (breaking = major, additive = minor, clarification = patch).

**Why deferring:** This requires git integration (reading previous file versions from git objects), diff computation between two SpecAST instances, and classification of changes as breaking/additive/patch. It is tractable but adds complexity to M3. Targeted for a post-MVP M3.5 milestone.

---

## 9. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Schema design is wrong and needs breaking changes | Medium | High | Dogfood aggressively in M1-M2. Use Specter's own specs as the first test case. Version the schema itself. |
| Orphan detection produces too many false positives | Medium | Medium | Allow `// @spec-ignore` annotations on constraints. Tune enforcement by tier. |
| Annotation-based coverage (`@spec`, `@ac`) is too manual | High | Medium | This is a known friction point. Mitigate by providing a `specter annotate` helper that suggests annotations. Long-term: use AST analysis to infer spec-test mapping without annotations. |
| graphlib is unmaintained or insufficient | Low | Low | graphlib is stable and well-tested. If it becomes a bottleneck, migrate to graphology (drop-in replacement with more features). |
| The reverse compiler (M6) produces specs too low-quality to be useful | Medium | High | Set expectations: reverse-compiled specs are `status: draft` and require human review. The value is in the skeleton, not the finished product. Track promotion rate (draft -> approved) as a quality metric. |

---

## 10. Success Criteria for the MVP

The MVP is successful if all of the following are true:

1. **Specter can validate its own specs.** Running `specter check ./specs` on Specter's own spec directory produces zero errors. This is the dogfooding proof.

2. **Specter can validate a real project's specs.** Running `specter check` on a set of specs written for JWTMS or OpenWatch (even a small subset) produces meaningful diagnostics — at least one orphan constraint and one dangling reference are caught.

3. **Spec coverage is measurable.** Running `specter coverage .` on Specter's own test suite shows 100% spec coverage, proving the annotation convention works end-to-end.

4. **The CLI is installable and usable.** `npx specter parse my-spec.spec.yaml` works without global installation. The output is human-readable and machine-parseable (JSON flag).

5. **A developer unfamiliar with the project can write a valid spec using only the schema and one example.** The schema is self-documenting enough that the `specter init` scaffolding command plus the schema produces a valid first spec.

---

## 11. Open Questions for Peer Agents

These questions are for the other agents on the team to address:

1. **For the Feasibility Agent:** Is graphlib still actively used in 2026, or has the ecosystem moved to graphology? Should we start with graphology instead?

2. **For the Market Research Agent:** Are there existing tools that validate YAML-based spec files against a schema and build dependency graphs? If so, what can we learn from them? The closest known tools are OpenAPI validators (Spectral by Stoplight) and Kubernetes manifest validators (kubeval). Neither targets general-purpose specs.

3. **For the Course Integration Agent:** Should the canonical schema (Section 5) be published as a new appendix in the sddbook? It would be the first machine-readable artifact in the course materials.

4. **For all agents:** The groundwork document proposes the reverse compiler as Phase 3 (before spec-check). This design moves it to M6 (after everything else). The rationale: the reverse compiler is the hardest, most speculative tool, and the type-system value prop can be proven without it. Do the other agents agree with this reordering?

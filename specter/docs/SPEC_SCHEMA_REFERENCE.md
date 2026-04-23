# Spec Schema Reference

> Canonical reference for `.spec.yaml` files validated by Specter.
> Schema source: `internal/parser/spec-schema.json` (v1.0.0)

---

## Table of Contents

- [Document Structure](#document-structure)
- [Required Fields](#required-fields)
- [Optional Fields](#optional-fields)
- [Context Object](#context-object)
- [Objective Object](#objective-object)
- [Constraint Object](#constraint-object)
- [Constraint Validation Object](#constraint-validation-object)
- [Acceptance Criterion Object](#acceptance-criterion-object)
- [Error Case Object](#error-case-object)
- [Dependency Reference Object](#dependency-reference-object)
- [Environment Object](#environment-object)
- [Changelog Entry Object](#changelog-entry-object)
- [Changelog Change Object](#changelog-change-object)
- [Generated From Object](#generated-from-object)
- [Naming Conventions](#naming-conventions)
- [Versioning Rules](#versioning-rules)
- [Tier Definitions](#tier-definitions)
- [Status Lifecycle](#status-lifecycle)
- [Worked Examples](#worked-examples)

---

## Document Structure

Every `.spec.yaml` file is a YAML document with a single top-level key: `spec`. All fields live inside this wrapper. No additional top-level keys are permitted.

```yaml
spec:
  id: my-feature
  version: "1.0.0"
  status: draft
  tier: 2
  context: { ... }
  objective: { ... }
  constraints: [ ... ]
  acceptance_criteria: [ ... ]
  # optional fields follow
```

The `spec:` wrapper exists so the schema can be extended in the future (e.g., adding a top-level `meta:` or `tooling:` key) without breaking existing files.

---

## Required Fields

These fields must be present inside `spec:`. Omitting any of them is a validation error.

| Field | Type | Format / Validation | Description |
|---|---|---|---|
| `id` | `string` | Regex: `^[a-z][a-z0-9-]*$` (kebab-case, starts with letter) | Unique identifier for this spec. Used in `depends_on` references and test annotations. |
| `version` | `string` | Semver: `MAJOR.MINOR.PATCH` with optional pre-release tag. Regex: `^(0\|[1-9]\d*)\.(0\|[1-9]\d*)\.(0\|[1-9]\d*)(-[a-zA-Z0-9.]+)?$` | Semantic version of the spec. Must be quoted in YAML to avoid float interpretation. |
| `status` | `string` | Enum: `draft`, `review`, `approved`, `deprecated`, `removed` | Lifecycle status. Only `approved` specs are enforced by spec-sync. |
| `tier` | `integer` | Enum: `1`, `2`, `3` | Risk tier. Determines enforcement strictness and coverage thresholds. |
| `context` | `object` | See [Context Object](#context-object) | What system this spec belongs to and why it exists. |
| `objective` | `object` | See [Objective Object](#objective-object) | What this spec aims to achieve, including scope boundaries. |
| `constraints` | `array` | Minimum 1 item. Items: [Constraint](#constraint-object) | Inviolable rules. Each is a hard boundary on the solution space. |
| `acceptance_criteria` | `array` | Minimum 1 item. Items: [Acceptance Criterion](#acceptance-criterion-object) | Testable conditions that define "done". Each AC maps to at least one test. |

### Field Examples

```yaml
spec:
  id: payment-create-intent         # kebab-case, starts with letter
  version: "1.0.0"                  # always quote — YAML parses 1.0 as float
  status: approved                  # no quotes needed for enum values
  tier: 1                           # integer, not string
```

---

## Optional Fields

These fields may be omitted entirely. When absent, Specter does not supply defaults at the document level (except where noted in sub-objects).

| Field | Type | Description |
|---|---|---|
| `title` | `string` | Human-readable display name. Defaults to a titlecased version of `id` when omitted. Used by the VS Code extension tree view, PR rendering, and `specter explain` output. Added in v0.7.0. |
| `depends_on` | `array` of [Dependency Reference](#dependency-reference-object) | Other specs this depends on. Creates edges in the dependency graph. |
| `environment` | [Environment Object](#environment-object) | Required environment variables and deployment targets. |
| `tags` | `array` of `string` | Free-form tags for categorization and filtering. |
| `changelog` | `array` of [Changelog Entry](#changelog-entry-object) | Version history. Most recent entry first. |
| `generated_from` | [Generated From Object](#generated-from-object) | Provenance tracking. Present only on reverse-compiled specs. |

---

## Context Object

Describes **what system** this spec belongs to and **why it exists**. The `context` object allows additional properties beyond those defined below.

| Field | Required | Type | Description |
|---|---|---|---|
| `system` | **Yes** | `string` | What system or service does this spec belong to? |
| `feature` | No | `string` | What feature area within the system? |
| `description` | No | `string` | Plain-language overview of why this spec exists. |
| `dependencies` | No | `array` of `string` | External dependencies (libraries, services, APIs). |
| `existing_patterns` | No | `string` | Relevant coding patterns, conventions, or architectural decisions. |
| `related_specs` | No | `array` of `string` | Other spec files that interact with this one. |
| `assumptions` | No | `array` of `string` | Things taken as given. If wrong, the spec needs revision. |

**Note:** All fields in the context object are declared above. Unknown fields are rejected at parse time — `specter parse` returns an error naming the offending key. If you need to capture additional metadata, use the spec-level `tags` array for categorical data, or `context.description` for free-form narrative. Extension by adding ad-hoc keys to `context` is not supported; propose a new schema field instead. (Changed in v0.7.0 — earlier versions silently dropped unknown context keys via `additionalProperties: true`, causing data loss for AI-assisted migrations.)

```yaml
context:
  system: Specter toolchain
  feature: YAML-to-AST parser
  description: >
    The foundational tool in the Specter toolchain. Parses .spec.yaml files
    and produces typed SpecAST objects.
  dependencies:
    - "yaml (eemeli/yaml 2.x)"
    - "ajv (8.x)"
  assumptions:
    - "Input files are UTF-8 encoded YAML"
```

---

## Objective Object

Describes **what this spec aims to achieve**. Uses the Delta Principle: describe the *change*, not the *state*.

| Field | Required | Type | Description |
|---|---|---|---|
| `summary` | **Yes** | `string` | 1-3 sentence description of what this spec achieves. |
| `scope` | No | `object` | Explicit scope boundaries. |
| `scope.includes` | No | `array` of `string` | What is in scope. |
| `scope.excludes` | No | `array` of `string` | What is out of scope. Prevents AI scope creep. |

`additionalProperties` is `false` on both the objective and scope objects.

```yaml
objective:
  summary: >
    Parse .spec.yaml files into validated, typed SpecAST objects.
    Reject malformed specs with actionable error messages.
  scope:
    includes:
      - "YAML parsing with syntax error handling"
      - "JSON Schema validation"
    excludes:
      - "Dependency resolution (that is spec-resolve)"
```

---

## Constraint Object

A constraint is an **inviolable rule** -- a hard boundary on the solution space. Every spec must have at least one.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `id` | **Yes** | `string` | Regex: `^C-\d{2,}$` (e.g., `C-01`, `C-02`, `C-10`) | Unique constraint ID within this spec. |
| `description` | **Yes** | `string` | Should use RFC 2119 language (MUST, MUST NOT, SHOULD, MAY) | Human-readable constraint statement. |
| `type` | No | `string` | Enum: `technical`, `security`, `performance`, `accessibility`, `business` | Category of constraint. Surfaces in `specter check` diagnostics so issues can be grouped by category. |
| `enforcement` | No | `string` | Enum: `error`, `warning`, `info` | Overrides the tier-based default severity when Specter emits a diagnostic about this constraint (orphan, structural conflict). Omit to use the tier default (T1=error, T2=warning, T3=info for orphans). |
| `validation` | No | [Constraint Validation](#constraint-validation-object) | Machine-readable validation rule | Enables deterministic checking by spec-check. |

```yaml
constraints:
  - id: C-01
    description: "MUST validate against the canonical JSON Schema"
    type: technical
    enforcement: error

  - id: C-02
    description: "SHOULD support YAML anchors and aliases"
    type: technical
    enforcement: warning
    validation:
      field: yaml_features
      rule: enum
      value: ["anchors", "aliases"]
```

---

## Constraint Validation Object

An optional machine-readable validation rule attached to a constraint. Enables deterministic checking by spec-check without requiring AI interpretation.

| Field | Required | Type | Description |
|---|---|---|---|
| `field` | **Yes** | `string` | The field this validation applies to. |
| `rule` | **Yes** | `string` — enum: `type`, `min`, `max`, `pattern`, `enum`, `required`, `format`, `custom` | The type of validation rule. |
| `value` | **Yes** | `string` or `number` or `boolean` or `string[]` | The value for the rule. Actual type depends on the rule type. |

```yaml
validation:
  field: password
  rule: min
  value: 8
```

---

## Acceptance Criterion Object

An acceptance criterion (AC) is a **testable condition** that defines "done." Each AC should map to at least one test. The `references_constraints` field links ACs to the constraints they validate -- this is used for orphan constraint detection.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `id` | **Yes** | `string` | Regex: `^AC-\d{2,}$` (e.g., `AC-01`, `AC-02`, `AC-10`) | Unique AC ID within this spec. |
| `description` | **Yes** | `string` | -- | Human-readable description of the expected behavior. |
| `inputs` | No | `object` | Free-form (additionalProperties: true) | Input values or conditions that trigger this behavior. |
| `expected_output` | No | `object` | Free-form (additionalProperties: true) | What should happen when the inputs are provided. |
| `error_cases` | No | `array` of [Error Case](#error-case-object) | -- | Error conditions and their expected handling. |
| `references_constraints` | No | `array` of `string` | Each item: regex `^C-\d{2,}$` | Which constraints this AC validates. Used for orphan detection. |
| `gap` | No | `boolean` | Default: `false` | `true` if this AC was identified by gap analysis (no existing test covers it). |
| `priority` | No | `string` | Enum: `critical`, `high`, `medium`, `low` | Implementation priority. |
| `notes` | No | `string` | -- | Free-form narrative about edge cases, rationale, or non-obvious implementation details. Rendered in the VS Code `@ac` hover and `specter explain` output. Added in v0.7.0. |
| `approval_gate` | No | `boolean` | Default: `false` | Marks this AC as requiring explicit human approval before it can be considered done. **Specter does not enforce approval semantics** — `specter coverage` counts the AC as covered when a matching `@ac` annotation exists, regardless of whether `approval_gate: true` or `approval_date` is set. Teams wire enforcement into their own PR/CI gates (e.g., a pre-push hook that rejects a diff where any AC has `approval_gate: true && approval_date == null`). Use for ACs whose correctness can't be verified by an automated test alone and require human sign-off. Added in v0.7.0. See `BACKLOG.md` → v0.11 "BUG-3 part 2" for the open design question on whether `specter coverage` should begin enforcing this in a future release. |
| `approval_date` | No | `string` | ISO-8601 date: `YYYY-MM-DD` | Date a human verified this AC. Meaningful only in conjunction with `approval_gate: true`. Specter does not read this field — it is metadata for human and CI consumers. Added in v0.7.0. |

```yaml
acceptance_criteria:
  - id: AC-01
    description: "Valid spec file is parsed into a SpecAST"
    inputs:
      file: "fixtures/valid/simple.spec.yaml"
    expected_output:
      type: "SpecAST"
      fields_present: ["id", "version", "status", "tier"]
    references_constraints: ["C-01", "C-04"]
    priority: critical

  - id: AC-02
    description: "Missing required field returns ParseError"
    inputs:
      file: "fixtures/invalid/missing-id.spec.yaml"
    expected_output:
      type: "ParseError"
      error_path: "spec.id"
    error_cases:
      - condition: "id field is absent"
        expected_behavior: "Return error with field path spec.id"
    references_constraints: ["C-01", "C-02"]
    priority: critical
```

---

## Error Case Object

Describes an error condition and its expected handling. Used inside acceptance criteria.

| Field | Required | Type | Description |
|---|---|---|---|
| `condition` | **Yes** | `string` | The error condition. |
| `expected_behavior` | **Yes** | `string` | How the system should handle this condition. |

---

## Dependency Reference Object

Declares a dependency on another spec. Creates an edge in the dependency graph built by spec-resolve.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `spec_id` | **Yes** | `string` | Regex: `^[a-z][a-z0-9-]*$` (same as spec `id`) | The `id` of the spec being depended on. |
| `version_range` | No | `string` | Semver range (e.g., `^1.0.0`, `>=2.0.0 <3.0.0`, `~1.2.0`) | If omitted, any version is accepted. |
| `relationship` | No | `string` | Enum: `requires`, `extends`, `conflicts_with`. Default: `requires` | Nature of the dependency. |

### Relationship Types

| Relationship | Meaning |
|---|---|
| `requires` | This spec cannot function without the dependency. The dependency must be present and its constraints satisfied. |
| `extends` | This spec builds on top of the dependency, adding to its capabilities. |
| `conflicts_with` | This spec is incompatible with the referenced spec. Both cannot be active simultaneously. |

```yaml
depends_on:
  - spec_id: spec-parse
    version_range: "^1.0.0"
    relationship: requires

  - spec_id: spec-resolve
    version_range: "^1.0.0"
    relationship: requires
```

---

## Environment Object

Declares runtime environment requirements for the implementation.

| Field | Required | Type | Description |
|---|---|---|---|
| `required_vars` | No | `array` of `string` | Environment variables this implementation requires. |
| `deployment_targets` | No | `array` of `string` | Where this deploys (e.g., `production`, `staging`, `edge`). |

```yaml
environment:
  required_vars:
    - STRIPE_SECRET_KEY
    - DATABASE_URL
  deployment_targets:
    - production
    - staging
```

---

## Changelog Entry Object

Records a version change. Entries are ordered most recent first.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `version` | **Yes** | `string` | Semver (same regex as spec `version`) | The version this entry describes. |
| `date` | **Yes** | `string` | ISO 8601 date (`YYYY-MM-DD`) | When this version was created. |
| `author` | No | `string` | -- | Who authored this change. |
| `type` | No | `string` | Enum: `initial`, `major`, `minor`, `patch` | Classification of the change. |
| `description` | **Yes** | `string` | -- | Summary of what changed. |
| `changes` | No | `array` of [Changelog Change](#changelog-change-object) | -- | Itemized list of individual changes. |

```yaml
changelog:
  - version: "1.0.0"
    date: "2026-03-28"
    author: "specter-team"
    type: initial
    description: "Initial spec for the parser"
    changes:
      - type: addition
        section: constraints
        detail: "Added C-01 through C-08"
```

---

## Changelog Change Object

An individual change within a changelog entry.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `type` | **Yes** | `string` | Enum: `addition`, `removal`, `modification`, `deprecation` | What kind of change. |
| `section` | No | `string` | -- | Which section of the spec was affected. |
| `detail` | **Yes** | `string` | -- | Description of the specific change. |

---

## Generated From Object

Provenance tracking for reverse-compiled specs -- specs that were extracted from existing code rather than written first. This field should only be present on reverse-compiled specs.

| Field | Required | Type | Format / Validation | Description |
|---|---|---|---|---|
| `source_file` | No | `string` | -- | Path to the source code this spec was reverse-compiled from. |
| `test_files` | No | `array` of `string` | -- | Paths to associated test files. |
| `extraction_date` | No | `string` | ISO 8601 date (`YYYY-MM-DD`) | When the reverse compilation occurred. |

```yaml
generated_from:
  source_file: "src/core/parser/parse.ts"
  test_files:
    - "tests/core/parser/parse.test.ts"
  extraction_date: "2026-03-28"
```

---

## Naming Conventions

| Element | Convention | Pattern | Examples |
|---|---|---|---|
| Spec ID | kebab-case, starts with a letter | `^[a-z][a-z0-9-]*$` | `user-registration`, `payment-create-intent`, `auth-jwt-validation` |
| Constraint ID | `C-` prefix + zero-padded number (2+ digits) | `^C-\d{2,}$` | `C-01`, `C-02`, `C-10` |
| Acceptance Criterion ID | `AC-` prefix + zero-padded number (2+ digits) | `^AC-\d{2,}$` | `AC-01`, `AC-02`, `AC-10` |
| Spec filename | `{spec-id}.spec.yaml` | -- | `user-registration.spec.yaml` |
| Version | Quoted semver | `MAJOR.MINOR.PATCH[-prerelease]` | `"1.0.0"`, `"2.1.0"`, `"1.0.0-draft"` |

**Important:** Always quote `version` values in YAML. Unquoted `1.0` is parsed as the float `1.0`, not the string `"1.0"`.

---

## Versioning Rules

Spec versions follow semantic versioning. The type of change determines which component to bump.

### MAJOR (breaking)

Increment MAJOR when the change could break downstream specs or implementations.

- Removing a constraint
- Removing an acceptance criterion
- Changing a constraint from `SHOULD` to `MUST NOT` (inverting intent)
- Removing a field from `context` that dependents rely on
- Changing the `id` of the spec
- Narrowing scope in a way that invalidates existing implementations

### MINOR (additive)

Increment MINOR when adding new capabilities that do not break existing behavior.

- Adding a new constraint
- Adding a new acceptance criterion
- Adding optional fields to context or objective
- Expanding scope (new `includes` items)
- Adding a new `depends_on` entry
- Changing enforcement from `error` to `warning` (relaxing)

### PATCH (clarification)

Increment PATCH for non-functional changes.

- Fixing typos in descriptions
- Improving clarity of constraint wording without changing meaning
- Adding or updating `changelog` entries
- Adding or modifying `tags`
- Updating `assumptions` or `description` text

---

## Tier Definitions

Tiers classify specs by risk level. The tier affects enforcement strictness, coverage thresholds, and diagnostic severity.

| Tier | Name | Description | Coverage Threshold | Orphan Severity |
|---|---|---|---|---|
| **1** | Security / Money | Specs governing authentication, authorization, payment processing, PII handling, cryptographic operations. Failures cause data breaches or financial loss. | 100% | `error` |
| **2** | Core Business Logic | Specs governing domain logic, workflow orchestration, data transformations. Failures cause incorrect behavior visible to users. | 80% | `warning` |
| **3** | Utility / Internal | Specs governing internal tooling, logging, formatting, dev-only features. Failures are inconvenient but not critical. | 50% | `info` |

### Choosing a Tier

- If a bug in this feature could lose money or leak data: **Tier 1**
- If a bug would be visible to end users and affect core functionality: **Tier 2**
- If a bug would only affect internal workflows or developer experience: **Tier 3**

When in doubt, choose the higher (stricter) tier.

---

## Status Lifecycle

Specs move through a linear lifecycle. Only `approved` specs are enforced by spec-sync in CI.

```
draft --> review --> approved --> deprecated --> removed
```

| Status | Meaning | Enforced by spec-sync? |
|---|---|---|
| `draft` | Work in progress. May be incomplete or rapidly changing. | No |
| `review` | Complete and submitted for team review. Should not change without discussion. | No |
| `approved` | Accepted as the source of truth. Implementation must conform. | **Yes** |
| `deprecated` | Superseded or scheduled for removal. Existing implementations still valid but should migrate. | No |
| `removed` | No longer active. Kept in version history only. | No |

---

## Worked Examples

### Minimal Spec

The smallest valid spec. Contains only required fields with the minimum required sub-fields.

```yaml
spec:
  id: test-minimal
  version: "1.0.0"
  status: draft
  tier: 3

  context:
    system: Test system

  objective:
    summary: A minimal spec with only required fields and no optional fields.

  constraints:
    - id: C-01
      description: "MUST work with minimal fields"

  acceptance_criteria:
    - id: AC-01
      description: "Parser succeeds with only required fields present"
```

### Full Spec

A production spec using most available fields.

```yaml
spec:
  id: spec-parse
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: Specter toolchain
    feature: YAML-to-AST parser
    description: >
      The foundational tool in the Specter toolchain. Parses .spec.yaml files,
      validates them against the canonical JSON Schema, and produces typed SpecAST
      objects. Every other tool depends on spec-parse producing correct, validated output.
    dependencies:
      - "yaml (eemeli/yaml 2.x)"
      - "ajv (8.x)"
    assumptions:
      - "Input files are UTF-8 encoded YAML"
      - "The canonical JSON Schema is the source of truth for spec structure"

  objective:
    summary: >
      Parse .spec.yaml files into validated, typed SpecAST objects.
      Reject malformed specs with actionable error messages that include
      line numbers and field paths.
    scope:
      includes:
        - "YAML parsing with syntax error handling"
        - "JSON Schema validation against the canonical spec-schema.json"
        - "Typed AST construction from validated YAML"
        - "Error reporting with line numbers and JSON field paths"
      excludes:
        - "Dependency resolution (that is spec-resolve)"
        - "Semantic validation across specs (that is spec-check)"
        - "File discovery and glob patterns (that is the registry)"

  constraints:
    - id: C-01
      description: "MUST validate against the canonical JSON Schema (spec-schema.json)"
      type: technical
      enforcement: error

    - id: C-02
      description: "MUST report errors with the YAML line number and JSON path"
      type: technical
      enforcement: error

    - id: C-03
      description: "SHOULD support YAML anchors and aliases"
      type: technical
      enforcement: warning

  acceptance_criteria:
    - id: AC-01
      description: "Valid spec file is parsed into a SpecAST with all required fields"
      inputs:
        file: "fixtures/valid/simple.spec.yaml"
      expected_output:
        type: "SpecAST"
        fields_present: ["id", "version", "status", "tier", "context", "objective", "constraints", "acceptance_criteria"]
      references_constraints: ["C-01"]
      priority: critical

    - id: AC-02
      description: "Spec missing required field returns ParseError with field path"
      inputs:
        file: "fixtures/invalid/missing-id.spec.yaml"
      expected_output:
        type: "ParseError"
        error_path: "spec.id"
      error_cases:
        - condition: "id field is absent"
          expected_behavior: "Return error with field path spec.id and error_type required"
      references_constraints: ["C-01", "C-02"]
      priority: critical

    - id: AC-03
      description: "Malformed YAML returns ParseError with line number"
      inputs:
        file: "fixtures/invalid/bad-yaml.spec.yaml"
      expected_output:
        type: "ParseError"
        has_line_number: true
      references_constraints: ["C-02"]
      priority: critical

  tags:
    - parser
    - core
    - foundational

  environment:
    required_vars: []
    deployment_targets:
      - production
      - staging

  changelog:
    - version: "1.0.0"
      date: "2026-03-28"
      author: "specter-team"
      type: initial
      description: "Initial spec for the Specter YAML parser"
      changes:
        - type: addition
          section: constraints
          detail: "Added C-01 through C-03"
        - type: addition
          section: acceptance_criteria
          detail: "Added AC-01 through AC-03"
```

### Spec with Dependencies

A spec that depends on other specs, creating edges in the dependency graph.

```yaml
spec:
  id: spec-check
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: Specter toolchain
    feature: Spec type checker
    description: >
      Validates semantic consistency across specs in the dependency graph.
    related_specs:
      - "spec-parse.spec.yaml"
      - "spec-resolve.spec.yaml"

  objective:
    summary: >
      Perform structural type-checking across the spec dependency graph.
      Detect orphan constraints and structural conflicts between connected specs.
    scope:
      includes:
        - "Orphan constraint detection"
        - "Structural conflict detection between dependent specs"
      excludes:
        - "Semantic conflict detection (AI-assisted, future phase)"

  depends_on:
    - spec_id: spec-parse
      version_range: "^1.0.0"
      relationship: requires
    - spec_id: spec-resolve
      version_range: "^1.0.0"
      relationship: requires

  constraints:
    - id: C-01
      description: "MUST detect all orphan constraints (constraints with no AC reference)"
      type: technical
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Constraint not referenced by any AC produces OrphanConstraint diagnostic"
      inputs:
        spec: "spec with C-01 (referenced), C-02 (not referenced)"
      expected_output:
        type: "Diagnostic"
        kind: "orphan_constraint"
        constraint_id: "C-02"
      references_constraints: ["C-01"]
      priority: critical

  changelog:
    - version: "1.0.0"
      date: "2026-03-28"
      author: "specter-team"
      type: initial
      description: "Initial spec for the Specter type checker"
```

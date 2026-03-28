# SDD Domain Extraction: Requirements Spec for the Spec Toolchain

> Extracted from all 17 chapters of the sddbook, the SDD_NEXT_STEP_GROUNDWORK document,
> the JWTMS and OpenWatch SDD audits, and the AI Collaborator Instructions.
>
> This document is the requirements specification for the SDD toolchain itself --
> dogfooding the methodology.

---

## Table of Contents

1. [The Micro-Spec Schema: Complete Field Inventory](#1-the-micro-spec-schema-complete-field-inventory)
2. [Spec Lifecycle Rules](#2-spec-lifecycle-rules)
3. [Validation Rules](#3-validation-rules)
4. [The Three Eras Framework](#4-the-three-eras-framework)
5. [Real-World Patterns from Audits](#5-real-world-patterns-from-audits)
6. [Industry Tool References](#6-industry-tool-references)
7. [Spec Kinds and Hierarchy](#7-spec-kinds-and-hierarchy)
8. [Anti-Patterns the Tool Must Detect](#8-anti-patterns-the-tool-must-detect)
9. [The SDD Maturity Model](#9-the-sdd-maturity-model)
10. [Toolchain Components Implied by the Book](#10-toolchain-components-implied-by-the-book)

---

## 1. The Micro-Spec Schema: Complete Field Inventory

Every field mentioned across all 17 chapters, consolidated into a single canonical schema. This is the "type definition for the type system" that GROUNDWORK.md identifies as the critical first step.

### 1.1 Top-Level Metadata Block

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `kind` | enum | YES | M01-CH03 sec 3.5 | What type of artifact this spec describes |
| `metadata.name` | string | YES | M01-CH03 sec 3.5 | The specific name; used for component/function/class name in generated code |
| `metadata.module` | string | YES | M01-CH03 sec 3.5 | Feature domain this belongs to; determines file placement |
| `metadata.version` | semver string | YES | M01-CH02 sec 2.6, M04-CH02 | Semantic version of this spec (independent of git commit) |
| `metadata.status` | enum | YES | M01-CH03 sec 3.5 | Lifecycle status of the spec |
| `metadata.owner` | string | RECOMMENDED | M01-CH03 sec 3.5 | Team responsible; used for review routing |
| `metadata.system_spec` | string (ref@version) | RECOMMENDED | M01-CH03 sec 3.5 | Reference to global system spec, pinned to version |
| `metadata.created` | date | RECOMMENDED | M01-CH03 sec 3.5 | Creation timestamp for auditability |
| `metadata.updated` | date | RECOMMENDED | M01-CH03 sec 3.5 | Last-modified timestamp |
| `metadata.author` | string | OPTIONAL | M04-CH01 | Who/what wrote this spec (human, architect-agent, etc.) |
| `metadata.tier` | enum | OPTIONAL | GROUNDWORK | Risk tier for enforcement level (Tier 1/2/3) |

#### `kind` Enum Values (discovered across all chapters)

| Value | Source | Description |
|-------|--------|-------------|
| `Component` | M01-CH01, M01-CH03, M02-CH02 | UI component specification |
| `Feature` | M01-CH04 | Multi-component feature specification |
| `Endpoint` | M01-CH04, M02-CH03 | API endpoint specification |
| `SystemContext` | M01-CH01, M01-CH02 | Global system-level context (no code generated directly) |
| `Migration` | M04-CH02 | Specifies transition from one version to another |
| `Environment` | M04-CH03 | Environment variable and deployment context specification |
| `Refactor` | M05-CH01 | Refactoring specification (archaeology + target + migration) |
| `Pipeline` | M04-CH03 | CI/CD pipeline specification |
| `StateManagement` | M02-CH04 | Store shape, actions, selectors specification |
| `Schema` | M02-CH01 | Data model / schema specification |

#### `metadata.status` Enum Values

| Value | Source | Meaning |
|-------|--------|---------|
| `draft` | M01-CH03 sec 3.5, M04-CH01 | Initial creation; not yet reviewed |
| `review` | M01-CH03 sec 3.5 | Under human review |
| `approved` | M01-CH03 sec 3.5 | Reviewed and approved; authoritative SSOT |
| `deprecated` | M01-CH03 sec 3.5, M04-CH02 sec 2.5 | Superseded; consumers should migrate |
| `removed` | M04-CH02 sec 2.5 | No longer valid; consumers must have migrated |
| `discovered` | M05-CH01 sec 1.3 | Reverse-spec: describes current behavior, not designed behavior |

### 1.2 Context Block

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `context.description` | string | YES | M01-CH03 sec 3.2, 3.5 | Plain-language overview of why this spec exists; business context |
| `context.technical_context` | string | YES | M01-CH03 sec 3.5 | Detailed technical information: API endpoints, data types, existing components |
| `context.system` | object | OPTIONAL | M01-CH03 sec 3.2.1 | System-level context (stack, runtime, framework, styling, etc.) |
| `context.patterns` | object | OPTIONAL | M01-CH03 sec 3.2.1 | Institutional knowledge: data fetching patterns, error handling, component structure |
| `context.infrastructure` | object | OPTIONAL | M01-CH03 sec 3.2.1 | API base URL, auth mechanism, existing endpoints |
| `context.feature` | object | OPTIONAL | M01-CH03 sec 3.2.2 | Feature-specific context: what exists in the immediate area |
| `context.feature.description` | string | OPTIONAL | M01-CH03 sec 3.2.2 | What exists in the immediate feature area |
| `context.feature.current_state` | string[] | OPTIONAL | M01-CH03 sec 3.2.2 | List of existing files/components/hooks relevant |
| `context.feature.related_features` | string[] | OPTIONAL | M01-CH03 sec 3.2.2 | Adjacent features and their interaction points |
| `context.related_specs` | string[] | RECOMMENDED | M01-CH03 sec 3.5 | Other spec files that interact with this one |
| `context.assumptions` | string[] | OPTIONAL | M01-CH03 sec 3.5 | Things taken as given; if wrong, spec needs revision |
| `context.dependencies` | object[] | OPTIONAL | M01-CH01 | Named dependencies with version constraints |
| `context.existing_patterns` | string[] | OPTIONAL | M01-CH01 | Description of existing codebase conventions |

### 1.3 Objective Block

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `objective.summary` | string | YES | M01-CH03 sec 3.3.1 | One-to-three sentence description of the delta (change, not state) |
| `objective.acceptance_criteria` | string[] | YES | M01-CH03 sec 3.3.2 | Testable conditions that must be true when feature is complete |
| `objective.scope.includes` | string[] | RECOMMENDED | M01-CH03 sec 3.3.3 | Explicit list of what is in scope |
| `objective.scope.excludes` | string[] | RECOMMENDED | M01-CH03 sec 3.3.3 | Explicit list of what is out of scope (prevents AI scope creep) |
| `objective.endpoints` | object[] | CONDITIONAL | M01-CH04, M02-CH03 | For Endpoint kind: full endpoint definitions with request/response/errors |

#### Acceptance Criteria Properties

Per M01-CH03 sec 3.3.2 and M03-CH01 sec 1.3, each AC must be:
- **Testable**: Can be verified mechanically
- **Independent**: Stands alone without ordering dependency
- **Complete**: Together, all ACs fully describe the feature
- **Unambiguous**: Only one possible interpretation
- **Traceable**: Maps to at least one test (One-to-One Minimum Principle)

### 1.4 Constraints Block

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `constraints` | string[] | YES | M01-CH03 sec 3.4 | Flat list using MUST/MUST NOT vocabulary |
| `constraints.technical` | string[] | OPTIONAL | M01-CH03 sec 3.4.1 | Technical constraints (libraries, patterns, compatibility) |
| `constraints.security` | string[] | OPTIONAL | M01-CH03 sec 3.4.1 | Security constraints (tokens, sanitization, encryption) |
| `constraints.performance` | string[] | OPTIONAL | M01-CH03 sec 3.4.1 | Performance constraints (debounce, pagination, lazy loading) |
| `constraints.accessibility` | string[] | OPTIONAL | M01-CH03 sec 3.4.1 | WCAG/ARIA constraints |
| `constraints.business` | string[] | OPTIONAL | M01-CH03 sec 3.4.1 | Business rules, GDPR, regional requirements |

All constraints follow **RFC 2119** vocabulary (M01-CH03 sec 3.4.2):
- **MUST** / **MUST NOT**: Absolute requirement/prohibition. Violation is a defect.
- **SHOULD** / **SHOULD NOT**: Strong recommendation. Violation requires justification.
- **MAY**: Truly optional.

### 1.5 Testing Block

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `testing.unit` | string[] | RECOMMENDED | M01-CH03 sec 3.5 | Unit test cases |
| `testing.integration` | string[] | OPTIONAL | M01-CH03 sec 3.5 | Integration test cases |
| `testing.accessibility` | string[] | OPTIONAL | M01-CH03 sec 3.5 | Accessibility test cases |
| `testing.security` | string[] | OPTIONAL | M01-CH04 | Security-specific test cases |
| `testing.responsive` | string[] | OPTIONAL | M01-CH04 | Responsive design test cases |

### 1.6 Evolutionary Fields (M04-CH02)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `changelog` | object[] | OPTIONAL | M04-CH02 sec 2.2 | Cumulative changelog with version, date, author, type, description |
| `changelog[].version` | semver | YES (if changelog) | M04-CH02 | Version this entry describes |
| `changelog[].date` | date | YES (if changelog) | M04-CH02 | Date of change |
| `changelog[].author` | string | YES (if changelog) | M04-CH02 | Who made the change |
| `changelog[].type` | enum | YES (if changelog) | M04-CH02 | `initial`, `major`, `minor`, `patch` |
| `changelog[].description` | string | YES (if changelog) | M04-CH02 | What changed |
| `changelog[].changes` | object[] | OPTIONAL | M04-CH02 | Structured list of individual changes (type, section, detail) |
| `since` | semver | OPTIONAL | M04-CH02 | Per-field annotation marking when a field was introduced |

### 1.7 Multi-Agent Workflow Fields (M04-CH01)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `spec_version` | semver | YES | M04-CH01 | Version of this spec document |
| `feature` | string | YES | M04-CH01 | Feature identifier |
| `purpose` | string | YES | M04-CH01 | Why this spec exists (business justification) |
| `inputs` | object[] | CONDITIONAL | M04-CH01 | Typed input definitions with name, type, required, description |
| `outputs` | object[] | CONDITIONAL | M04-CH01 | Typed output definitions with properties |
| `error_cases` | object[] | RECOMMENDED | M04-CH01 | Condition + expected behavior pairs |
| `dependencies` | object[] | OPTIONAL | M04-CH01 | References to other specs with reason |
| `open_questions` | string[] | OPTIONAL | M04-CH01 | Unresolved ambiguities flagged for human |

### 1.8 Environment-Aware Fields (M04-CH03)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `variables` | object[] | CONDITIONAL | M04-CH03 sec 3.2 | Environment variable declarations |
| `variables[].name` | string | YES | M04-CH03 | Variable name |
| `variables[].type` | string | YES | M04-CH03 | Type (string, integer, enum, string[]) |
| `variables[].required` | object | YES | M04-CH03 | Per-environment required status (local, ci, staging, production) |
| `variables[].sensitive` | boolean | YES | M04-CH03 | Whether this is a secret |
| `variables[].defaults` | object | OPTIONAL | M04-CH03 | Per-environment default values |
| `variables[].validation` | object | OPTIONAL | M04-CH03 | Validation rules (pattern, min, max, must_not_be, etc.) |
| `variables[].depends_on` | object | OPTIONAL | M04-CH03 | Conditional requirement based on another variable |

### 1.9 Migration Spec Fields (M04-CH02)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `migration.from_spec` | string | YES | M04-CH02 sec 2.4 | Source spec path |
| `migration.from_version` | semver | YES | M04-CH02 | Version migrating from |
| `migration.to_version` | semver | YES | M04-CH02 | Version migrating to |
| `migration.motivation` | string | YES | M04-CH02 | Why the breaking change is needed |
| `migration.timeline` | object | YES | M04-CH02 | v2_available, v1_deprecated, v1_removed dates |
| `migration.changes` | object[] | YES | M04-CH02 | Structured change list with before/after schemas |
| `migration.data_migration` | object | RECOMMENDED | M04-CH02 | Strategy (lazy/eager), function spec, constraints |
| `migration.api_compatibility` | object | OPTIONAL | M04-CH02 | Dual-support strategy during migration window |
| `migration.rollback_plan` | object | RECOMMENDED | M04-CH02 | Trigger, steps, constraints for reverting |
| `migration.acceptance_criteria` | string[] | YES | M04-CH02 | Testable migration success criteria |

### 1.10 Approval Gate Fields (M05-CH03)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `trust_level` | enum | OPTIONAL | M05-CH03 sec 3.2 | `HIGH` (auto-approve), `MEDIUM` (review-required), `LOW` (human-decides) |
| `approval_gate` | object | OPTIONAL | M05-CH03 sec 3.3 | Formal approval gate specification |
| `approval_gate.trigger` | object | YES (if gate) | M05-CH03 | What triggers the gate (phase-complete, risk-threshold, manual, anomaly-detected) |
| `approval_gate.reviewScope` | object | YES (if gate) | M05-CH03 | Artifacts, criteria, anti-patterns, context for review |
| `approval_gate.approvers` | object | YES (if gate) | M05-CH03 | Role, minimumCount, escalation |
| `approval_gate.outcomes` | object | YES (if gate) | M05-CH03 | approved, rejected, conditionallyApproved next steps |
| `approval_gate.sla` | object | OPTIONAL | M05-CH03 | maxWaitTime, escalationAfter, autoRejectAfter |

### 1.11 Confidence Score Fields (M05-CH03)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `confidence.overall` | float (0.0-1.0) | OPTIONAL | M05-CH03 sec 3.7 | AI self-reported overall confidence |
| `confidence.sections` | object[] | OPTIONAL | M05-CH03 | Per-section confidence with reasoning and suggested review level |
| `confidence.uncertainties` | object[] | OPTIONAL | M05-CH03 | Ambiguities the AI encountered with possible interpretations |

### 1.12 Reverse Spec Fields (M05-CH01)

| Field | Type | Required | Source | Description |
|-------|------|----------|--------|-------------|
| `generated_from` | object | CONDITIONAL | GROUNDWORK | For reverse-compiled specs: source file, test files, extraction date |
| `generated_from.source_file` | string | YES (if reverse) | GROUNDWORK | Path to source code |
| `generated_from.test_files` | string[] | OPTIONAL | GROUNDWORK | Paths to associated test files |
| `generated_from.extraction_date` | date | YES (if reverse) | GROUNDWORK | When extraction occurred |
| `gap` | boolean | OPTIONAL | GROUNDWORK | Per-AC flag indicating constraint exists but no test covers it |

---

## 2. Spec Lifecycle Rules

### 2.1 Creation (MODULE_01)

1. **Specs precede code.** The spec is written and reviewed before any implementation begins (M01-CH02 sec 2.6).
2. **Every spec references a system spec.** Feature specs pin to a specific system spec version via `system_spec: system.spec.yaml@3.1.0` (M01-CH02 sec 2.5).
3. **The Delta Principle.** A micro-spec describes a *change*, not a *state*. Use "Add a search bar to..." not "The application has a search feature that..." (M01-CH03 sec 3.3.4).
4. **Context Completeness Test.** Could a competent developer who has never seen your codebase implement this feature correctly using *only* the information in the spec? (M01-CH03 sec 3.2.3).
5. **Constraint Completeness Test (Malicious Compliance Test).** If the AI produced code that technically meets all ACs but does so in the worst possible way, what would go wrong? Each worst-case suggests a constraint (M01-CH03 sec 3.4.3).

### 2.2 Spec-to-Test Mapping (MODULE_03 CH01)

1. **The Pipeline:** `Spec -> Tests -> Implementation -> Validation` (M03-CH01 sec 1.2).
2. **One-to-One Minimum Principle:** Every spec statement maps to at least one test (M03-CH01 sec 1.3).
3. **Mapping table:**
   - Functional requirement: 1 positive test minimum
   - Constraint / boundary: 2 tests (at boundary, past boundary)
   - Negative requirement: 1 negative test
   - Performance requirement: 1 benchmark test
   - Error handling: 1 test per error condition
   - State transition: 1 test per transition
4. **Spec Coverage:** Analogous to code coverage. Measures how much of a spec has corresponding tests. Target: Tier 1 = 100%, Tier 2 = 80%, Tier 3 = 50% (GROUNDWORK).
5. **Test annotations:** Every test must reference its spec AC: `// AC-4: Gift card reduces charge amount correctly` (AI_COLLABORATOR sec Phase 4).

### 2.3 Spec Versioning (MODULE_04 CH02)

1. **Semantic Versioning for Specs:**
   - MAJOR (1.0 -> 2.0): Breaking change; all consumers must migrate
   - MINOR (1.0 -> 1.1): Additive change; backward compatible
   - PATCH (1.0.0 -> 1.0.1): Clarification/correction; no behavior change
2. **Cumulative changelog:** Every version's changes preserved in the spec (M04-CH02 sec 2.2).
3. **`since` annotations:** Per-field markers indicating when introduced (M04-CH02 sec 2.2).
4. **Generated code references spec version:** `// SearchBar.tsx -- Generated from search-bar.spec.yaml v1.2.0` (M01-CH02 sec 2.6).
5. **Deprecation lifecycle:** `ACTIVE -> DEPRECATED -> REMOVED` (M04-CH02 sec 2.5).

### 2.4 Spec Validation / Linting (MODULE_03 CH02)

1. **Three types of intent drift to detect:**
   - Architectural Drift: Spec says "use X", AI uses Y
   - Pattern Drift: Spec defines a pattern, AI uses a different valid pattern
   - Dependency Drift: Spec lists approved deps, AI introduces new ones
2. **Drift detection pipeline:** `ADR (Why) -> Spec Constraint (What) -> ESLint Rule (How) -> CI Check (When)` (M03-CH02 sec 2.4).
3. **Persistent spec constraints:** `.cursorrules`, `CLAUDE.md`, `.github/copilot-instructions.md` are living, versioned spec fragments (M03-CH02 sec 2.5).
4. **Drift detector tool:** Regex-based pattern matching against spec constraints with severity levels and spec references (M03-CH02 sec 2.6).
5. **CI enforcement:** Pre-commit hooks and CI steps that block merges on spec violations (M03-CH02 sec 2.8).

### 2.5 Context Window Management (MODULE_03 CH03)

1. **Registry Pattern:** A master index (`SPEC_REGISTRY.md`) catalogs all specs with ID, name, path, status, dependencies (M03-CH03 sec 3.3).
2. **Three-level hierarchy:**
   - Level 1: System-level specs (5-10 total; architecture, tech choices)
   - Level 2: Module-level specs (15-30 total; features, business logic)
   - Level 3: Component-level specs (30-100 total; individual UI/functions)
3. **Context budget:** Keep total context under 50% of window capacity (M03-CH03 sec 3.5).
4. **Spec summarization:** Each spec includes a 5-10 line summary for two-pass loading (M03-CH03 sec 3.4).
5. **Dependency graph for loading:** `depends_on` fields determine which related specs to load (M03-CH03 sec 3.9).
6. **Context Decision Framework:** Different loading strategies for new features, bug fixes, refactoring, and tests (M03-CH03 sec 3.5).

### 2.6 Multi-Agent Workflow (MODULE_04 CH01)

1. **Three-Agent Pattern:**
   - Architect Agent: Writes/refines specs (MUST NOT write code)
   - Builder Agent: Executes specs into code (MUST NOT add unspecified features; MUST report ambiguity)
   - Critic Agent: Validates code against spec
2. **Feedback flows to Architect, not Builder.** When Critic finds issues, it goes back to Architect to update the spec (M04-CH01 sec 1.1, 1.15).
3. **Builder generates ambiguity reports** when spec is unclear, with possible interpretations and recommendations (M04-CH01 sec 1.3).
4. **Specs as shared agent memory.** The spec is the shared contract binding all agents together (M04-CH01 sec 1.14).

### 2.7 Environment-Aware Specs (MODULE_04 CH03)

1. **Environment variables are spec inputs.** Every env var must be declared with type, per-environment required status, defaults, validation rules, and sensitivity marking (M04-CH03 sec 3.2).
2. **Fail-fast validation at startup.** Application MUST NOT start if environment is invalid (M04-CH03 sec 3.2).
3. **CI/CD pipelines are specs.** GitHub Actions workflows are declarative specifications of the validation process (M04-CH03 sec 3.3).
4. **Feature flags as environment-aware spec toggles** (M04-CH03 sec 3.7).
5. **Three layers:** Application Specs, Environment Specs, Infrastructure as Code (M04-CH03 sec 3.12).

### 2.8 Reverse Engineering (MODULE_05 CH01)

1. **Four layers of code archaeology:**
   - Structural: files, organization, dependencies
   - Behavioral: what the system actually does (not what docs say)
   - Contractual: implicit/explicit contracts, callers, expectations
   - Historical: git history, abandoned migrations, workarounds
2. **Reverse Spec Technique:** Write a spec describing what currently exists (status: `discovered`), then write a target spec; the gap IS the refactor scope (M05-CH01 sec 1.3).
3. **Scoping Matrix:** High Impact/High Effort = Do Now; High Impact/Low Effort = Schedule; Low Impact = Maybe Never or Think Twice (M05-CH01 sec 1.4).
4. **Strangler Fig Pattern:** Incremental specs, each strangling a piece of the old system; each phase is standalone with own success criteria and rollback (M05-CH01 sec 1.5).
5. **Risk Registry:** Probability, impact, mitigation, fallback for each identified risk (M05-CH01 sec 1.6).

### 2.9 Documentation Generation (MODULE_05 CH02)

1. **Specs ARE documentation.** Spec -> Code, Spec -> Tests, Spec -> Docs (all derived from same source) (M05-CH02 sec 2.1).
2. **Documentation Pyramid:** Spec is the canonical form; TypeDoc, OpenAPI, Storybook, Docusaurus are generated views (M05-CH02 sec 2.6).
3. **CI enforcement of doc sync:** Build fails if generated docs differ from committed docs (M05-CH02 sec 2.2).
4. **Tools:** TypeDoc (TypeScript), Swagger/OpenAPI (APIs), Storybook (components), Docusaurus (full sites) (M05-CH02 sec 2.3).

### 2.10 Human-in-the-Loop (MODULE_05 CH03)

1. **Trust Spectrum:** Full Automation (boilerplate) <-> Full Human Control (ethical decisions) (M05-CH03 sec 3.2).
2. **Per-section trust levels:** `[TRUST: HIGH -- AUTO-APPROVE]`, `[TRUST: MEDIUM -- REVIEW REQUIRED]`, `[TRUST: LOW -- HUMAN DECIDES]` (M05-CH03 sec 3.2).
3. **Approval Gate Placement Framework:**
   - ALWAYS gate: auth/authz, payment, PII, rate limiting, data deletion, first deployments
   - SOMETIMES gate: performance-critical, complex business logic, third-party integrations
   - RARELY gate: CRUD boilerplate, test generation, doc generation
4. **Layered Review Model (from Google):**
   - Layer 1: Automated CI/CD
   - Layer 2: Peer review (any team member)
   - Layer 3: Domain expert review (security, data, privacy, a11y, perf)
   - Layer 4: Architecture review (tech lead)
   - Layer 5: Business review (product owner, legal)
5. **Confidence scores route reviews:** >=0.95 auto-approve, >=0.80 quick-review, >=0.60 deep-review by senior, <0.60 tech lead (M05-CH03 sec 3.7).

---

## 3. Validation Rules

### 3.1 Required Fields

For a spec to be valid, it MUST have:
- `kind` (identifies what type of artifact)
- `metadata.name` (identifies which specific thing)
- `metadata.version` (enables versioning and traceability)
- `metadata.status` (determines whether spec is authoritative)
- `context` section (at minimum `context.description`)
- `objective.summary` (the delta being specified)
- `objective.acceptance_criteria` (at least one testable AC)
- `constraints` (at least one MUST or MUST NOT statement)

### 3.2 Naming Conventions

Per the system spec pattern in M01-CH02 sec 2.5:
- Components: PascalCase
- Hooks: camelCase with "use" prefix
- Files: kebab-case
- Types: PascalCase with no "I" prefix
- Spec files: `{feature-name}.spec.yaml` or `{feature-name}.spec.md`
- Reverse spec files: `{feature-name}.reverse-spec.md`
- Migration specs: `{feature}-v{from}-to-v{to}.migration.yaml`

### 3.3 Versioning Rules

**Breaking changes (MAJOR bump required):**
- Removing an existing field
- Changing a field's type
- Making an optional field required
- Changing error codes or messages
- Changing output format
- Removing an enum value
- Tightening a constraint (making more restrictive)

**Additive changes (MINOR bump):**
- Adding a new OPTIONAL field (MUST have default value)
- Adding a new error case
- Adding a new output field
- Adding a new enum value
- Relaxing a constraint (making less restrictive)
- Adding a new acceptance criterion

**Patch changes (PATCH bump):**
- Fixing a typo in a description
- Clarifying an ambiguous constraint
- Adding an example

**Simple test:** Take the existing test suite built from v1 ACs, run against v2 spec. If any tests should fail, it is a MAJOR change.

### 3.4 Tier Definitions and Enforcement Levels

From GROUNDWORK and the audits:

| Tier | Risk Level | Examples | Spec Granularity | Coverage Target |
|------|-----------|----------|------------------|-----------------|
| Tier 1 | Security / Financial / PHI | Payment processing, authentication, encryption, PHI handling | Per-endpoint, per-function | 100% |
| Tier 2 | Business Logic | Availability calculation, pricing, booking flow | Module-level with AC per public method | 80% |
| Tier 3 | Utility / CRUD | Config, static pages, simple data display | Lightweight specs, structural validation only | 50% |

**Enforcement by tier (GROUNDWORK sec 4.5):**

| Check | Tier 1 | Tier 2 | Tier 3 |
|-------|--------|--------|--------|
| Conflict detection | Error | Error | Warning |
| Orphan constraints | Error | Warning | Info |
| Gap detection | Error | Warning | Off |
| Breaking change enforcement | Error + migration spec | Error | Warning |
| Spec coverage minimum | 100% | 80% | 50% |

### 3.5 Trust Level Definitions

From M05-CH03 sec 3.2:

| Trust Level | AI Directive | Human Role | Examples |
|-------------|-------------|------------|----------|
| HIGH | Implement and proceed; no review unless tests fail | Post-hoc audit | Database queries, CRUD, standard pagination |
| MEDIUM | Implement then STOP; generate report for review | Review before deployment | Search ranking, caching strategy, complex validation |
| LOW | DO NOT implement; present requirements to human | Decide implementation approach | Content filtering, pricing rules, compliance logic |

### 3.6 Approval Gate Triggers

From M05-CH03 sec 3.3:

| Trigger Type | Condition |
|-------------|-----------|
| `phase-complete` | A defined development phase finishes (e.g., all tests pass) |
| `risk-threshold` | Confidence score drops below threshold |
| `manual` | Explicitly marked in spec as requiring human review |
| `anomaly-detected` | Automated checks detect unexpected patterns |

---

## 4. The Three Eras Framework

From M01-CH01 sec 1.2:

| Era | Period | Characteristics | Tooling Role |
|-----|--------|----------------|--------------|
| **Era 1: Vibe Coding** | 2022-2024 | Conversational prompts, unreproducible, ad hoc | The toolchain DETECTS this: code without specs, implicit decisions |
| **Era 2: Structured Prompting** | 2024-2025 | Better prompts, but ad hoc format, ephemeral, no composition | The toolchain MIGRATES from this: reverse-compile existing codebases |
| **Era 3: SDD** | 2025-Present | Formal specs, version-controlled, reviewed, testable, reusable | The toolchain ENFORCES this: spec-first workflow, CI gates |

**How the tool fits:** The toolchain is the infrastructure that makes Era 3 unforgeable. It provides:
- A **reverse compiler** for codebases stuck in Era 1/2 (GROUNDWORK sec 3)
- A **spec type system** for codebases in Era 3 (GROUNDWORK sec 4)
- A **CI enforcer** that prevents regression to earlier eras (GROUNDWORK sec 4.5)

---

## 5. Real-World Patterns from Audits

### 5.1 What Developers Already Have

**JWTMS (Next.js / TypeScript):**
- Zod schemas for API input validation (Schema-First at boundaries)
- Prisma schema as data SSOT (30+ models, generated client)
- Rich `context/` directory (10+ markdown files) functioning as persistent AI constraints
- 5-tier test priority matrix (CRITICAL -> CRUD)
- Husky + lint-staged pre-commit hooks
- Platform tier feature gating (declarative tier definitions)
- 333 tests (102 unit + 231 integration)

**OpenWatch (FastAPI / Python):**
- Pydantic schemas for API contracts (9 dedicated schema files)
- ORSA v2.0 plugin interface (abstract class + typed dataclasses)
- SQLAlchemy models as data SSOT (30+ models)
- Three CLAUDE.md files + 13 context files
- Pytest with semantic markers (unit, integration, regression, slow)
- 290+ backend tests, 246 Playwright E2E tests
- 338 Kensa YAML compliance rules (machine-readable specs)

### 5.2 What Is Missing (Both Projects)

1. **No formal `.spec` files for any module.** Behavioral truth is scattered across docs, context, schemas, and code.
2. **Tests are code-derived, not spec-derived.** Tests validate what code does, not what it should do. No traceability.
3. **API output contracts are implicit.** Input validation exists (Zod/Pydantic), but output shapes, side effects, error taxonomy, and authorization are undocumented.
4. **Business logic modules lack behavioral specs.** The most complex, risk-critical code (payments, availability, compliance) has no formal specification.
5. **Component contracts are absent.** Props are typed but behavioral specifications (state machines, side effects, edge cases) are not documented.
6. **No spec-to-test pipeline.** No mechanism to verify that tests cover spec requirements or that code implements spec constraints.

### 5.3 Transition Path

From AI_COLLABORATOR instructions:

```
Phase 1: Archaeology     -> Produce Archaeology Report (4-layer analysis)
Phase 2: Reverse Specs   -> Spec current behavior (as-is, including bugs)
Phase 3: Target Specs    -> Spec desired behavior + gap analysis table
Phase 4: Spec Tests      -> Write/annotate tests traced to spec ACs
Phase 5: Going-Forward   -> Establish spec-first workflow + registry
```

Priority order (by risk): Payments -> Availability -> API routes -> PHI/Security -> Cross-module flows -> Registry.

### 5.4 Common Anti-Patterns to Detect

**From M01-CH02 sec 2.12:**
- **The Retroactive Spec**: Writing the spec after the code (spec describes code, not governs it)
- **The Orphan Spec**: A spec that no code references and no one maintains
- **The Kitchen Sink Spec**: Tries to specify too much in one document
- **The Immutable Spec**: Treated as sacred and never updated when requirements change
- **The Ignored Spec**: Exists but development does not reference it

**From M01-CH04 sec 4.5:**
- **The Magic Word**: "Make it fast/modern/secure" -- unmeasurable criteria
- **The Implied Stack**: No explicit tech stack context
- **The Absent Boundary**: No scope excludes section
- **The Missing Negative**: No MUST NOT constraints
- **The Context Vacuum**: Assumes AI knows the project
- **The Testless Request**: No acceptance criteria or test plan
- **The Conversational Debug Loop**: Iterating via chat instead of spec refinement

**From M05-CH03 sec 3.8 (AI deviation patterns):**
- **The Helpful Addition**: AI adds features not in spec
- **The Premature Optimization**: AI over-engineers
- **The Silent Error Swallow**: AI catches but mishandles errors
- **The Library Substitution**: AI swaps approved deps
- **The Scope Creep**: AI extends scope beyond spec boundaries

---

## 6. Industry Tool References

### 6.1 AI Providers and Their Spec Patterns

| Company | Product | SDD-Relevant Pattern | Reference |
|---------|---------|---------------------|-----------|
| **Anthropic** | Claude, Claude Code | Constitutional AI as behavioral spec; system prompts as SSOT; tool use schemas as micro-specs; CLAUDE.md as persistent constraints; Task tool for multi-agent; RLHF/RLAIF as spec enforcement | M01-CH01 sec 1.4, M01-CH02 sec 2.4, M03-CH02 sec 2.2, M04-CH01 sec 1.6, M05-CH03 sec 3.4 |
| **OpenAI** | GPT, Structured Outputs, Assistants API, Swarm | Function calling as schema-first; JSON Schema for structured outputs; RLHF as automated behavior linting; Agent frameworks for multi-agent | M01-CH01 sec 1.4, M02-CH01 sec 1.3, M03-CH02 sec 2.2, M04-CH01 sec 1.8 |
| **Google** | Gemini, DeepMind, AlphaCode | Design docs culture; Gemini function calling; 1M+ token context windows; Responsible AI layered review; API versioning strategy; Rosie for large-scale changes | M01-CH01 sec 1.4, M02-CH01 sec 1.3, M03-CH03 sec 3.2, M05-CH03 sec 3.6, M04-CH02 sec 2.6, M05-CH01 sec 1.7 |
| **Meta** | Llama, React ecosystem | Open models + community spec patterns; Headless UI pattern; React component architecture | M01-CH01 sec 1.4, M02-CH02 sec 2.6, M04-CH01 sec 1.9 |
| **Microsoft** | AutoGen, Copilot | Multi-agent frameworks; `.github/copilot-instructions.md` as persistent constraints | M04-CH01 sec 1.10, M03-CH02 sec 2.5 |

### 6.2 Schema and Validation Ecosystem

| Tool | Category | How It Relates to SDD Toolchain |
|------|----------|--------------------------------|
| **Zod** (TypeScript) | Runtime validation | Existing constraint enforcement in JWTMS; reverse compiler extracts constraints from `z.string().min(1).max(255)` |
| **Pydantic** (Python) | Runtime validation | Existing constraint enforcement in OpenWatch; reverse compiler extracts from Pydantic models |
| **Prisma** | ORM / Schema-first DB | Already schema-first SSOT for data; toolchain should parse Prisma schemas |
| **SQLAlchemy** | ORM | OpenWatch data SSOT; toolchain should parse SQLAlchemy models |
| **JSON Schema** | Universal schema language | The lingua franca between humans and AI; basis for spec validation |
| **OpenAPI / Swagger** | API specification | Industry standard for REST APIs; toolchain should generate from/validate against specs |
| **Protocol Buffers / gRPC** | Schema-first RPC | Google's schema-first philosophy; reference implementation |
| **XState** | State machines | Component state machine specs translate directly to XState definitions |

### 6.3 Testing Frameworks

| Tool | Language | SDD Role |
|------|----------|----------|
| **Vitest** | TypeScript | Spec-derived test execution (JWTMS) |
| **Pytest** | Python | Spec-derived test execution (OpenWatch) |
| **Jest** | TypeScript | Test execution |
| **Playwright** | Cross-platform | E2E spec validation (OpenWatch has 246 tests) |
| **React Testing Library** | React | Component contract validation |
| **fast-check** | TypeScript | Property-based testing as spec validation (M03-CH01 sec 1.8) |
| **Hypothesis** | Python | Property-based testing as spec validation (M03-CH01 sec 1.8) |

### 6.4 Documentation and Dev Tools

| Tool | Category | SDD Integration |
|------|----------|----------------|
| **TypeDoc** | TypeScript docs | Generate from spec interfaces (M05-CH02 sec 2.3.1) |
| **Swagger UI / Redoc / Stoplight** | API docs | Generate from OpenAPI specs (M05-CH02 sec 2.3.2) |
| **Storybook** | Component docs | Generate stories from component contracts (M05-CH02 sec 2.3.3) |
| **Docusaurus** | Full doc sites | Generate site from spec hierarchy (M05-CH02 sec 2.3.4) |
| **ESLint** | Linting | Custom rules as spec enforcers (M03-CH02 sec 2.3) |
| **Cursor IDE** | AI IDE | `.cursorrules` as persistent spec constraints (M03-CH02 sec 2.5) |
| **MDX** | Docs format | Bridge between specs and readable documentation (M05-CH02 sec 2.8) |

### 6.5 API-First Companies (Design References)

| Company | What to Learn From | Reference |
|---------|-------------------|-----------|
| **Stripe** | API evolution strategy; legendary backward compatibility; spec-driven documentation | M04-CH02 sec 2.6, M05-CH02 sec 2.5 |
| **Twilio** | Documentation architecture; spec-driven doc generation | M05-CH02 sec 2.5 |
| **Google Cloud** | Declarative infrastructure specs (Cloud Run, GKE YAML) | M04-CH03 sec 3.6 |
| **AWS** | CloudFormation, CDK as infrastructure specs | M04-CH03 sec 3.6 |
| **Terraform** | Infrastructure as Code as the ultimate environment spec | M04-CH03 sec 3.4 |

---

## 7. Spec Kinds and Hierarchy

### 7.1 The System Spec (Constitution)

The `system.spec.yaml` is a special document referenced by all feature specs. It contains:
- **Technology stack** (language, runtime, framework, styling, state, routing, testing)
- **Architecture pattern** (feature-based modules, API style, auth strategy)
- **Conventions** (naming for components/hooks/files/types, error handling, data fetching, accessibility)
- **Security baseline** (input sanitization, CSRF, localStorage restrictions, logging)

The system spec answers questions that every feature spec would otherwise repeat.

### 7.2 File Structure Convention

From M01-CH02 sec 2.5:

```
project/
  specs/
    system.spec.yaml           # Global system context
    SPEC_REGISTRY.md           # Master index of all specs
    features/
      {domain}/
        {feature}.spec.yaml    # Feature specifications
    components/
      {component}.spec.yaml    # Component specifications
    migrations/
      {feature}-v{x}-to-v{y}.migration.yaml
    environment/
      environment-variables.spec.yaml
  src/
    features/
      {domain}/
        components/            # Generated/governed by specs
        hooks/
        types/
        __tests__/
```

### 7.3 Spec-Code Traceability

Every generated code file should include:
```typescript
// ComponentName.tsx -- Generated from component-name.spec.yaml v1.2.0
// @generated -- Manual edits will diverge from the spec.
// Update the spec and regenerate instead.
```

Every test file should include:
```typescript
// Spec: specs/features/search/search-bar.spec.yaml
// AC-1: Search input is visible in the top navigation bar
```

---

## 8. Anti-Patterns the Tool Must Detect

### 8.1 Spec Quality Anti-Patterns

| Anti-Pattern | Detection Method | Severity |
|-------------|-----------------|----------|
| Missing `excludes` section | Static analysis | Warning |
| No MUST NOT constraints | Static analysis | Warning |
| AC not testable (contains subjective words) | NLP/pattern matching for "good", "nice", "fast", "modern" | Error |
| Context describes state not delta (objective) | Pattern matching on objective.summary | Warning |
| No error cases defined | Static analysis | Warning (Tier 1: Error) |
| Orphan constraint (no AC references it) | Graph analysis | Warning |
| Gap (input path with no AC) | AI-assisted path analysis | Warning |
| Spec conflict (two specs contradict) | Cross-spec dependency analysis | Error |

### 8.2 Spec-Code Drift Detection

| Drift Type | Detection Method | Severity |
|-----------|-----------------|----------|
| Import not in spec's dependency list | AST import analysis | Warning |
| Function not in spec's endpoints | AST function analysis | Warning |
| Error handling mismatch | Status code analysis against spec | Error |
| Library substitution | Import analysis against approved deps | Error |
| CSS/styling drift | Import analysis for CSS files when Tailwind-only | Error |
| State management drift | Import analysis for prohibited state libraries | Error |
| Default export when named required | AST analysis | Error |
| `any` type usage | TypeScript AST analysis | Error |

### 8.3 Lifecycle Anti-Patterns

| Anti-Pattern | Detection Method | Severity |
|-------------|-----------------|----------|
| Code edited without spec update | Spec version in code vs current spec version | Warning |
| Spec updated without code regeneration | CI comparison of spec version vs code annotation | Warning |
| Breaking change without MAJOR version bump | Semantic diff of consecutive spec versions | Error |
| New required field added without default | Schema analysis | Error |
| Optional field made required (breaking) | Schema diff | Error |
| Constraint tightened (breaking) | Constraint comparison | Error |

---

## 9. The SDD Maturity Model

From M05-CH03 sec 3.16:

| Level | Name | Description | Toolchain Role |
|-------|------|-------------|----------------|
| **Level 1** | Spec-Aware | Team knows specs exist; documentation present but not governing | Reverse compiler generates draft specs; no enforcement |
| **Level 2** | Spec-Driven | Specs written before code; tests trace to specs; specs reviewed | `spec-parse` validates; `spec-coverage` measures; CI warns |
| **Level 3** | Spec-Optimized | Multi-agent workflows; context optimization; drift detection | `spec-check` validates cross-spec consistency; `spec-resolve` manages graph |
| **Level 4** | Spec-Native | Full spec type system; automated gap detection; specs as infrastructure | All toolchain components active; tiered enforcement; breaking change detection |

Both JWTMS and OpenWatch are assessed at **Level 1 (Spec-Aware)**: strong documentation culture but no formal spec-to-test pipeline.

---

## 10. Toolchain Components Implied by the Book

From the book's content and GROUNDWORK, the complete toolchain consists of:

### 10.1 Core Components

| Component | Analogous To | Function | Source |
|-----------|-------------|----------|--------|
| **`spec-parse`** | Lexer/Parser | Validate YAML against canonical JSON Schema; produce typed Spec AST | GROUNDWORK sec 4.1 |
| **`spec-resolve`** | Linker | Build dependency graph from `depends_on`; detect cycles, missing refs, version conflicts | GROUNDWORK sec 4.2, M03-CH03 sec 3.9 |
| **`spec-check`** | Type Checker | Conflict detection, orphan detection, gap detection, breaking change detection | GROUNDWORK sec 4.3 |
| **`spec-coverage`** | Code coverage | Traceability matrix: Spec ID -> ACs -> Tests -> Code Files -> Coverage % | GROUNDWORK sec 4.4, M03-CH01 sec 1.9 |
| **`spec-sync`** | CI enforcer | Runs in CI; gates PRs on spec consistency with tiered strictness | GROUNDWORK sec 4.5, M03-CH02 sec 2.8 |
| **`spec-reverse`** | Decompiler | Extract draft specs from existing TypeScript/Python code using AST + AI | GROUNDWORK sec 3, M05-CH01 sec 1.3 |

### 10.2 Supporting Components

| Component | Function | Source |
|-----------|----------|--------|
| **Spec Registry Generator** | Maintains `SPEC_REGISTRY.md` with discovery, scoping, dependency graph | M03-CH03 sec 3.3 |
| **Context Budget Calculator** | Estimates token cost of loading specs; optimizes for context window | M03-CH03 sec 3.6 |
| **Spec Summarizer** | Generates condensed summaries for two-pass context loading | M03-CH03 sec 3.6 |
| **Drift Detector** | Regex + AST pattern matching against spec constraints on git diffs | M03-CH02 sec 2.6 |
| **Doc Generator** | Generates OpenAPI, TypeDoc, Storybook stories from specs | M05-CH02 sec 2.1 |
| **Confidence Score Router** | Reads AI confidence reports; routes sections to appropriate reviewers | M05-CH03 sec 3.7 |
| **Migration Spec Validator** | Validates migration specs have rollback plans, timelines, data migration strategy | M04-CH02 sec 2.4 |
| **Env Var Validator Generator** | Generates Zod/Pydantic validation code from environment spec | M04-CH03 sec 3.2 |

### 10.3 Build Sequence (from GROUNDWORK)

```
Phase 1: JSON Schema definition for .spec.yaml files + spec-parse
Phase 2: spec-resolve + dependency graph
Phase 3: spec-reverse (code-to-spec) -- target JWTMS and OpenWatch
Phase 4: spec-check (conflict + orphan detection)
Phase 5: CI integration (spec-sync)
Phase 6: AI-assisted gap detection
```

### 10.4 The Canonical Micro-Spec JSON Schema

This is the "type definition for the type system" -- the single most important deliverable. It must answer:
- What fields are required vs. optional?
- What types do constraint values support?
- How are `depends_on` references structured?
- How are ACs identified and cross-referenced?
- How is versioning encoded?
- How are tiers assigned?

This schema MUST be defined as a JSON Schema document that `spec-parse` validates against.

---

## Appendix A: The SSOT Contract (7 Rules)

From M01-CH02 sec 2.10:

1. The spec is the source of truth. Code is a derivative.
2. When spec and code disagree, the spec is right and the code is wrong.
3. To understand what a feature does, read the spec.
4. To change a feature, change the spec first.
5. To review a feature, review the spec for intent, then code for implementation.
6. Code changes without spec changes are defects (if behavior changed).
7. Spec changes without code updates create technical debt.

## Appendix B: The SDD Manifesto

From M01-CH01 sec 1.12:

- Specifications over prompts
- Contracts over conventions
- Structured over conversational
- Reproducible over spontaneous
- Explicit over implicit
- "If the AI fails to build it correctly, the fault lies in the Spec, not the Code."

## Appendix C: Key Terminology

| Term | Definition | Source |
|------|-----------|--------|
| **Micro-Spec** | A structured specification with three pillars: Context, Objective, Constraints | M01-CH03 |
| **SSOT** | Single Source of Truth -- the `.spec` file is authoritative over `.code` | M01-CH02 |
| **Intent Drift** | When AI output gradually deviates from the original spec | M03-CH02 |
| **Approval Gate** | A checkpoint where humans validate AI work before it proceeds | M05-CH03 |
| **Spec Coverage** | Analogous to code coverage -- measures how much of a spec has corresponding tests | M03-CH01 |
| **Delta Principle** | A micro-spec describes a change, not a state | M01-CH03 |
| **Reverse Spec** | A spec written to describe what currently exists (status: discovered) | M05-CH01 |
| **Strangler Fig** | Incremental migration pattern: grow new around old | M05-CH01 |
| **Registry Pattern** | Master index of all specs with metadata and dependency graph | M03-CH03 |
| **Context Budget** | Token allocation strategy for AI context windows | M03-CH03 |
| **Spec Gap** | An input path through the spec with no AC covering it | GROUNDWORK |
| **Orphan Constraint** | A constraint that no AC references | GROUNDWORK |
| **Spec Conflict** | Two specs that contradict each other across the dependency graph | GROUNDWORK |

# Instructions for AI Collaborator: SDD Transition for JWTMS

> These instructions guide you through converting JWTMS (Bookwell) from a code-first project to Spec-Driven Development with spec-derived tests. Follow the phases in order — each one builds on the previous.

---

## Before You Begin: Learn the Methodology

You are adopting a methodology called **Spec-Driven Development (SDD)**. Before writing any specs or modifying any code, you must internalize the core principles by studying the reference material below.

### Required Reading (in this order)

Read these chapters from the `sddbook/` directory. Do not skim — these define the vocabulary, structure, and reasoning you will use in every subsequent phase.

**Reading 1: `sddbook/MODULE_01/CHAPTER_02.md` — The Single Source of Truth (SSOT)**

Learn:
- The spec is authoritative; code is derived from it
- Why code alone cannot serve as the source of truth (5 reasons)
- The SSOT Contract: 7 rules of engagement
- Anti-patterns to avoid (The Retroactive Spec, The Orphan Spec, etc.)

After reading, you should be able to answer: *Why can't we just use the existing code and tests as the specification?*

**Reading 2: `sddbook/MODULE_01/CHAPTER_03.md` — The Anatomy of a Micro-Spec**

Learn:
- The three pillars: Context, Objective, Constraints
- What belongs in each pillar and what does not
- How each pillar maps to AI behavior (Context → reduces hallucination, Objective → defines completeness, Constraints → prevents bad decisions)
- The MUST/MUST NOT convention (RFC 2119)
- The Spec Quality Evaluation criteria

After reading, you should be able to write a micro-spec from scratch that passes the Completeness, Clarity, and Appropriateness tests.

**Reading 3: `sddbook/MODULE_03/CHAPTER_01.md` — Spec-to-Test Mapping (TDD for AI)**

Learn:
- The pipeline: Spec → Tests → Implementation → Validation
- The One-to-One Minimum Principle (every spec line has at least one test)
- Boundary extraction and edge case taxonomy
- The concept of "Spec Coverage"
- How to teach the AI (yourself) to write tests FROM the spec

After reading, you should be able to take any micro-spec and produce a test suite where every test traces back to a specific acceptance criterion.

**Reading 4: `sddbook/MODULE_05/CHAPTER_01.md` — The Refactor Spec**

Learn:
- The Archaeology Phase and its four layers (Structural, Behavioral, Contractual, Historical)
- The Reverse Spec technique (specifying what exists, not what should exist)
- The Target Spec (specifying the desired end state)
- The gap between Reverse Spec and Target Spec = your work backlog
- The Scoping Matrix (what to touch vs. what to leave alone)
- The Strangler Fig pattern for incremental migration

After reading, you should be able to: given an existing codebase module, produce an Archaeology Report → Reverse Spec → Target Spec → Migration Path.

### Supplementary Reading (reference as needed)

- `sddbook/MODULE_02/CHAPTER_01.md` — Schema-First Design (relevant because JWTMS already uses Zod + Prisma schemas)
- `sddbook/MODULE_02/CHAPTER_03.md` — API Blueprinting (relevant for the 135+ API routes)
- `sddbook/MODULE_03/CHAPTER_02.md` — Automated Linting of Intent (relevant for the `context/` directory and CLAUDE.md patterns already in JWTMS)
- `sddbook/MODULE_03/CHAPTER_03.md` — Context Window Strategy (relevant for managing spec loading as the spec library grows)

---

## Existing Analysis to Review

Previous analysis has already been completed for JWTMS. Read these before starting any new work — they contain the audit, the phased plan, and a worked example:

1. **`project_review/04_JWTMS_SDD_AUDIT.md`** — SDD Readiness Audit. Covers what JWTMS already does well (Zod schemas, Prisma SSOT, context docs, test priority matrix) and the gap analysis (no formal spec files, tests are code-derived not spec-derived, implicit API contracts, missing behavioral specs for business logic).

2. **`project_review/05_JWTMS_SDD_TRANSITION_PLAN.md`** — Phased transition plan. Defines the spec directory structure, 6 phases prioritized by risk (payments → availability → API routes → PHI/security → cross-module flows → registry), and includes a complete micro-spec example for `create-intent`.

3. **`project_review/06_JWTMS_QUICK_START_EXAMPLE.md`** — End-to-end worked example. Walks through the full SDD cycle for one endpoint: write spec → map existing tests → identify gaps → write spec-derived tests → run and validate → establish going-forward contract.

**These documents are your starting context.** Do not repeat analysis that has already been done. Build on it.

---

## Phase 1: Archaeology (Understand What Exists)

**Goal:** For the first target module, produce a structured Archaeology Report following the four-layer framework from Module 05 Chapter 1.

**Start with the payment pipeline** (as recommended by the transition plan). The target modules are:
- `apps/web/src/app/api/payments/` (API routes)
- `packages/*/lib/payment-policy/` (business logic)
- `packages/*/lib/stripe/` (Stripe service wrapper)
- `packages/*/lib/pricing/` (pricing calculation)

### What to Produce

An **Archaeology Report** for the payment pipeline covering:

**Layer 1 — Structural:** File inventory, dependency graph between payment modules, imports and exports.

**Layer 2 — Behavioral:** What these modules actually do when called. Trace the flow from API request to Stripe call to database write. Document actual behavior, not assumed behavior.

**Layer 3 — Contractual:** Who calls these modules? What do callers expect? What implicit contracts exist between payment-policy, pricing, stripe-service, and the API routes? What Zod schemas already define input contracts? What output shapes are returned?

**Layer 4 — Historical:** Check git history for these files. When were they last modified? Were there any "temporary" workarounds? Any abandoned migrations? Any TODO comments?

### Rules for This Phase

- **Report facts, not opinions.** The archaeology report documents what IS, not what SHOULD BE.
- **Flag surprises.** If you find undocumented behavior, inconsistencies, or implicit contracts that seem fragile, flag them explicitly.
- **Do not propose changes yet.** That comes in Phase 3.

---

## Phase 2: Reverse Specs (Specify What Currently Exists)

**Goal:** Write micro-specs that describe the current behavior of the payment pipeline — as-is, warts included.

Using the Archaeology Report from Phase 1 and the micro-spec structure from Module 01 Chapter 3, produce Reverse Specs for:

1. `create-intent.reverse-spec.md` — What the create-intent route currently does
2. `payment-policy.reverse-spec.md` — Current payment policy business rules
3. `stripe-service.reverse-spec.md` — Current Stripe API interaction patterns

### Reverse Spec Format

Each reverse spec should follow this structure:

```markdown
# Reverse Spec: [Module Name]

## Status: DISCOVERED (not designed)
## Archaeology Date: [date]
## Source Files: [list of files examined]

## Context
[System context, dependencies, callers]

## Current Behavior
[What it actually does — document bugs, inconsistencies, and undocumented behavior honestly]

## Current Acceptance Criteria
[What would a test suite need to assert to fully describe current behavior?]

## Constraints (Observed)
[What constraints are enforced by the current code? What constraints are missing?]

## Known Issues
[Bugs, missing validation, implicit contracts, untested paths]
```

### Rules for This Phase

- **Describe current behavior faithfully.** If the code has a bug, the reverse spec documents the bug. Do not correct it.
- **Mark known issues separately.** Use a `## Known Issues` section so the gap is visible in Phase 3.
- **Use the acceptance criteria format.** Every behavior should be expressed as a testable AC. This makes the gap analysis in Phase 3 mechanical rather than subjective.

---

## Phase 3: Target Specs + Gap Analysis

**Goal:** Write target micro-specs for the desired behavior, then diff them against the reverse specs to produce a concrete work backlog.

### Target Specs

Using the micro-spec template (Context, Objective, Constraints) from Module 01 Chapter 3, write target specs for:

1. `specs/api/payments/create-intent.spec.md`
2. `specs/lib/payment-policy/payment-policy.spec.md`
3. `specs/lib/stripe/stripe-service.spec.md`

Use the example from `project_review/05_JWTMS_SDD_TRANSITION_PLAN.md` Section 1.1 as a starting template for `create-intent.spec.md`.

### Gap Analysis

For each module, produce a gap table:

| AC | Description | In Reverse Spec? | In Target Spec? | Test Exists? | Action Required |
|----|-------------|:-:|:-:|:-:|-----------------|
| AC-1 | ... | Yes | Yes | Yes | None — test traces to spec |
| AC-4 | ... | No (implicit) | Yes | No | Write spec AC + test |
| AC-9 | ... | No | Yes | No | Write spec AC + test + implement |

The "Action Required" column becomes your work backlog.

### Rules for This Phase

- **Target specs define the desired truth.** Once approved, these become the SSOT — code and tests must conform to them.
- **Do not over-spec.** Follow the book's guidance on over-specifying vs. under-specifying (Module 01, Chapter 3, Section 3.7). Spec the behavior, not the implementation.
- **Mark approval gates.** Any AC that involves financial transactions, PHI, or security-critical behavior should be marked `[APPROVAL GATE]` — these require human review before implementation.

---

## Phase 4: Spec-Derived Tests

**Goal:** Write tests that trace directly to target spec acceptance criteria.

Follow the Spec-to-Test Mapping pipeline from Module 03 Chapter 1:

1. **For each AC in the target spec**, write at least one test.
2. **Annotate every test** with a comment referencing its AC: `// AC-4: Gift card reduces charge amount correctly`
3. **For existing tests that already cover an AC**, add the annotation. Do not rewrite working tests unnecessarily.
4. **For gap ACs (from the gap table)**, write new tests. Use the examples in `project_review/06_JWTMS_QUICK_START_EXAMPLE.md` as templates.

### Test Organization

- Keep tests in their existing locations (co-located with routes/modules).
- Add a `// Spec: specs/api/payments/create-intent.spec.md` header comment to each test file to establish traceability.

### Rules for This Phase

- **Tests must fail before implementation.** If a gap AC test passes immediately, either the behavior already exists (update the reverse spec) or the test is not specific enough.
- **Test the spec, not the implementation.** Tests should assert on behavior (input → output, side effects, error responses), not on internal implementation details.
- **Use the boundary extraction method** from Module 03, Chapter 1, Section 1.4 to identify edge cases for each AC.

---

## Phase 5: Establish the Going-Forward Contract

**Goal:** Transition the specced modules from code-first to spec-first workflow.

Once the target specs are approved and tests pass:

1. **Update `CLAUDE.md`** (or the project's equivalent) to reference the spec directory and enforce the spec-first workflow for specced modules.
2. **Create `specs/SPEC_REGISTRY.md`** following the template in the transition plan (Section Phase 6). This is the master index.
3. **Document the workflow change:** For any module that has a spec, the development process is now: `update spec → update tests → update code → validate`. Not the reverse.

---

## Guiding Principles (Refer to These Throughout)

1. **The spec is the SSOT.** If spec and code disagree, the spec wins (once approved by a human). See Module 01, Chapter 2.

2. **Archaeology before architecture.** Never propose target specs without first completing the archaeology and reverse spec. See Module 05, Chapter 1, Section 1.2.

3. **One bounded slice at a time.** Do not spec the entire project at once. Complete one module through all phases before moving to the next. The transition plan defines the priority order.

4. **Spec the behavior, not the implementation.** A spec says "MUST return 402 when gift card balance is insufficient." It does NOT say "use an if statement to check the balance." See Module 01, Chapter 3, Section 3.7.

5. **Every AC must be testable.** If you cannot write a test for an acceptance criterion, the AC is too vague. Rewrite it. See Module 03, Chapter 1.

6. **Mark what you don't touch.** Use the Scoping Matrix from Module 05, Chapter 1, Section 1.4. Explicitly list modules that are out of scope for each phase.

7. **Human approval gates for high-risk changes.** Any spec AC that affects financial transactions, PHI, security, or cross-module contracts must be reviewed and approved by a human before tests or code are written. See Module 05, Chapter 3.

8. **Build on what exists.** JWTMS already has Zod input schemas, Prisma as data SSOT, a rich `context/` directory, and 333 tests. The goal is to formalize and extend, not replace.

---

## Sequence Summary

```
Phase 1: Archaeology     → Produce Archaeology Report for payment pipeline
Phase 2: Reverse Specs   → Spec current behavior (as-is, including bugs)
Phase 3: Target Specs    → Spec desired behavior + gap analysis table
Phase 4: Spec Tests      → Write/annotate tests traced to spec ACs
Phase 5: Going-Forward   → Establish spec-first workflow + registry

Then repeat Phases 1-5 for the next priority module:
  → Availability engine
  → High-traffic API routes
  → PHI/security modules
  → Cross-module flows
```

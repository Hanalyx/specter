# Instructions for AI Collaborator: SDD Transition for OpenWatch

> These instructions guide you through converting OpenWatch from a code-first project to Spec-Driven Development with spec-derived tests. Follow the phases in order — each one builds on the previous.

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

After reading, you should be able to answer: *Why can't we just use OpenWatch's existing code, Pydantic schemas, and context/ documentation as the specification?*

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
- How to write tests FROM the spec, not from the code

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

- `sddbook/MODULE_02/CHAPTER_03.md` — API Blueprinting (relevant for the 80+ API endpoints)
- `sddbook/MODULE_03/CHAPTER_02.md` — Automated Linting of Intent (relevant for the `context/` directory and CLAUDE.md patterns already in OpenWatch)
- `sddbook/MODULE_03/CHAPTER_03.md` — Context Window Strategy (relevant for managing spec loading as the spec library grows)
- `sddbook/MODULE_04/CHAPTER_01.md` — Multi-Agent Workflow (relevant if using team-based AI workflows for spec writing)

---

## Existing Analysis to Review

Previous analysis has already been completed for OpenWatch. Read these before starting any new work — they contain the audit and the phased plan:

1. **`project_review/08_OPENWATCH_SDD_AUDIT.md`** — SDD Readiness Audit. Covers what OpenWatch already does well (Pydantic schemas, ORSA plugin contract, SQLAlchemy SSOT, rich AI context, Kensa YAML rules) and the gap analysis (no formal spec files, tests are code-derived, scan pipeline unspecified, RBAC implicit, temporal engine edge cases undefined, remediation pipeline unspecified).

2. **`project_review/09_OPENWATCH_SDD_TRANSITION_PLAN.md`** — Phased transition plan. Defines the spec directory structure, 6 phases prioritized by risk (scan pipeline → remediation → temporal compliance → auth/RBAC → API routes → registry), and includes a complete micro-spec example for `start-kensa-scan`.

**These documents are your starting context.** Do not repeat analysis that has already been done. Build on it.

---

## Project-Specific Context to Internalize

Before you begin archaeology, understand these OpenWatch-specific facts:

### Architecture

- **Backend:** FastAPI (Python 3.12), async SQLAlchemy 2.0, Celery + Redis, Pydantic 2.x
- **Frontend:** React 19 + TypeScript (strict), Redux Toolkit, React Query, MUI v7, Playwright E2E
- **Database:** PostgreSQL 15 (UUID primary keys, parameterized SQL via QueryBuilder/MutationBuilders)
- **Compliance engine:** Kensa v1.1.0, accessed exclusively through the ORSA v2.0 plugin interface
- **SSH:** All remote host operations go through `SSHConnectionManager`
- **Deprecated:** MongoDB is removed from runtime; legacy code exists for import compatibility only — ignore it

### Existing Contracts (already partially spec-driven)

- **9 Pydantic schema files** in `backend/app/schemas/` — these define typed request/response models. They are the closest things to existing specs. Build on them, don't replace them.
- **ORSA v2.0** in `backend/app/services/plugins/orsa/` — abstract plugin interface with typed dataclasses. This is already a well-defined contract. The spec should formalize behavioral expectations, not the type signatures.
- **Kensa YAML rules** (338 rules, 5 frameworks) — declarative compliance check definitions. These are specs for compliance checks. Don't re-spec what Kensa already defines.
- **SQL builders** (`QueryBuilder`, `InsertBuilder`, `UpdateBuilder`, `DeleteBuilder`) — all PostgreSQL access goes through these. Specs should reference this constraint, not reinvent it.

### Existing Documentation (reference, don't duplicate)

- **Root `CLAUDE.md`** (39.8KB) — mandatory verification rules, architecture overview, agentic coding principles
- **`backend/CLAUDE.md`** — Python/FastAPI patterns, Kensa plugin architecture, service organization
- **`frontend/CLAUDE.md`** — React/TypeScript patterns, component architecture
- **`context/` directory** (13 files) — architecture, module boundaries, testing strategy, security standards, coding standards, debugging guide
- **`PRD/` directory** — completed 14-week roadmap with 7 epics
- **`BACKLOG.md`** — current prioritized work queue
- **`docs/` directory** — deployment guides, security hardening, runbooks, ADRs

### Testing State

- **Backend:** 290+ tests, 32% coverage (CI threshold: 31%), organized as `tests/unit/`, `tests/integration/`, `tests/security/`, `tests/regression/`
- **Frontend:** 246 Playwright E2E tests, 88 Vitest unit tests (~1.5% coverage)
- **Coverage targets:** 80% backend, 60% frontend (stretch goals in BACKLOG.md)
- **Key untested areas (from BACKLOG.md):** JWT token tests (E5-G3), credential encryption tests (E5-G4), scan integration tests (E5-G5), auth integration tests (E5-G6)

### Risk Priority (where SDD adds most value)

1. **Scan execution pipeline** — core product, every feature depends on it
2. **Remediation workflow** — SSH root operations on remote systems
3. **Temporal compliance** — audit-facing scores with regulatory implications
4. **Auth/RBAC** — security-critical, 100% coverage target declared

---

## Phase 1: Archaeology (Understand What Exists)

**Goal:** For the scan execution pipeline, produce a structured Archaeology Report following the four-layer framework from Module 05 Chapter 1.

**Target modules:**
- `backend/app/routes/scans/` — API endpoints for scan operations
- `backend/app/tasks/` — Celery task definitions for async scan execution
- `backend/app/services/engine/` — Scan execution engine (executors, scanners, result parsers, orchestration)
- `backend/app/services/plugins/orsa/` — ORSA plugin interface and registry
- `backend/app/services/compliance/temporal.py` — Posture snapshot creation from scan results
- `backend/app/services/compliance/alerts.py` — Alert generation from drift
- `backend/app/services/monitoring/` — Drift detection service

### What to Produce

An **Archaeology Report** for the scan execution pipeline covering:

**Layer 1 — Structural:** File inventory for each target module. Dependency graph between scan routes → tasks → engine → ORSA → compliance. What imports what? What are the entry points (API routes) and exit points (DB writes, snapshots)?

**Layer 2 — Behavioral:** Trace the complete flow from `POST /api/scans/kensa/start` through to posture snapshot creation. Document what actually happens at each step. What Celery task is dispatched? What Kensa methods are called? What database tables are written to? What happens on SSH timeout? What happens when Kensa returns partial results?

**Layer 3 — Contractual:** Who calls the scan pipeline? (Frontend ScanPanel, ComplianceSchedulerService, bulk scan orchestrator.) What do callers expect back? What Pydantic schemas already define input/output contracts? What implicit contracts exist between engine, ORSA, and compliance services? What does the ORSA `CheckResult` dataclass guarantee?

**Layer 4 — Historical:** Check git history for scan-related files. When was the Kensa integration added (replacing OpenSCAP)? What was the scan architecture before Kensa? Are there any `# TODO`, `# HACK`, or `# TEMPORARY` comments? Any abandoned migration artifacts?

### Rules for This Phase

- **Report facts, not opinions.** The archaeology report documents what IS, not what SHOULD BE.
- **Flag surprises.** If you find undocumented behavior, inconsistencies between ORSA interface and actual Kensa usage, or implicit contracts that seem fragile, flag them explicitly.
- **Do not propose changes yet.** That comes in Phase 3.
- **Read the existing CLAUDE.md and context/ files first.** They contain substantial architectural documentation. Don't rediscover what's already documented — verify it against the code and note discrepancies.

---

## Phase 2: Reverse Specs (Specify What Currently Exists)

**Goal:** Write micro-specs that describe the current behavior of the scan execution pipeline — as-is, warts included.

Using the Archaeology Report from Phase 1 and the micro-spec structure from Module 01 Chapter 3, produce Reverse Specs for:

1. **`scan-execution.reverse-spec.md`** — The complete pipeline from API request to posture snapshot
2. **`kensa-scan.reverse-spec.md`** — How OpenWatch currently invokes Kensa via ORSA, parses results, stores evidence
3. **`scan-orchestration.reverse-spec.md`** — Celery task lifecycle: how scans are queued, executed, timed out, retried

### Reverse Spec Format

Each reverse spec should follow this structure:

```markdown
# Reverse Spec: [Module Name]

## Status: DISCOVERED (not designed)
## Archaeology Date: [date]
## Source Files: [list of files examined]

## Context
[System context, dependencies, callers — reference existing context/ docs where accurate]

## Current Behavior
[What it actually does — document bugs, missing error handling, undocumented paths honestly]

## Current Acceptance Criteria
[What would a test suite need to assert to fully describe current behavior?]

## Constraints (Observed)
[What constraints are enforced by the current code? What constraints are missing?]

## Existing Contracts
[Reference Pydantic schemas, ORSA dataclasses, and type signatures that already serve as partial contracts]

## Known Issues
[Bugs, missing validation, implicit contracts, untested paths, gaps from BACKLOG.md]
```

### Rules for This Phase

- **Describe current behavior faithfully.** If the code has a missing error handler, the reverse spec documents that gap. Do not correct it.
- **Reference existing Pydantic schemas.** The `schemas/` files already define partial contracts. The reverse spec should note what they cover and what they don't (e.g., input shape is defined, but output shape, side effects, and error taxonomy are not).
- **Reference the ORSA interface.** The `ORSAPlugin` ABC and its dataclasses are already well-typed. The reverse spec should document the gap between what the interface promises and what the implementation actually does.
- **Mark known issues separately.** Use a `## Known Issues` section. Cross-reference with BACKLOG.md items (E5-G3 through E5-G6) where relevant.
- **Use the acceptance criteria format.** Every behavior should be expressed as a testable AC. This makes the gap analysis in Phase 3 mechanical rather than subjective.

---

## Phase 3: Target Specs + Gap Analysis

**Goal:** Write target micro-specs for the desired behavior, then diff them against the reverse specs to produce a concrete work backlog.

### Target Specs

Using the micro-spec template (Context, Objective, Constraints) from Module 01 Chapter 3, write target specs for:

1. `specs/pipelines/scan-execution.spec.md` — The complete scan pipeline
2. `specs/services/engine/kensa-scan.spec.md` — Kensa invocation and result processing
3. `specs/services/engine/scan-orchestration.spec.md` — Celery task lifecycle

Use the example from `project_review/09_OPENWATCH_SDD_TRANSITION_PLAN.md` Section 1.1 as a starting template for the scan execution spec.

### Gap Analysis

For each module, produce a gap table:

| AC | Description | In Reverse Spec? | In Target Spec? | Test Exists? | Action Required |
|----|-------------|:-:|:-:|:-:|-----------------|
| AC-1 | ANALYST can start scan → 202 | Yes | Yes | Yes | Annotate test with AC ref |
| AC-5 | Duplicate scan → 409 | Unknown | Yes | No | Write spec AC + test |
| AC-8 | SSH failure → FAILED status | Unknown | Yes | No | Write spec AC + test + verify impl |
| AC-15 | Scan timeout handling | Unknown | Yes | No | Write spec AC + test + implement |

The "Action Required" column becomes your work backlog. Cross-reference with BACKLOG.md items — the E5-G5 (scan integration tests) and E5-G6 (auth integration tests) gaps may overlap.

### Rules for This Phase

- **Target specs define the desired truth.** Once approved, these become the SSOT — code and tests must conform to them.
- **Do not over-spec.** Spec the behavior, not the implementation. "MUST create posture snapshot after successful scan" — not "MUST call `TemporalComplianceService.create_snapshot()` with parameters x, y, z."
- **Respect existing contracts.** If a Pydantic schema or ORSA dataclass already defines a shape, reference it by name in the spec rather than redefining it. The spec extends the contract (adding behavioral rules, error handling, side effects), it doesn't replace the type system.
- **Mark approval gates.** Any AC that involves SSH operations on remote hosts, credential handling, or data that feeds audit reports should be marked `[APPROVAL GATE]` — these require human review before implementation.
- **Align with BACKLOG.md priorities.** If the gap analysis reveals work items, classify them using OpenWatch's priority scheme (P0-P3) and note which BACKLOG items they satisfy.

---

## Phase 4: Spec-Derived Tests

**Goal:** Write tests that trace directly to target spec acceptance criteria.

Follow the Spec-to-Test Mapping pipeline from Module 03 Chapter 1:

1. **For each AC in the target spec**, write at least one test.
2. **Annotate every test** with a comment referencing its AC: `# AC-5: Duplicate scan → 409 SCAN_IN_PROGRESS`
3. **For existing tests that already cover an AC**, add the annotation. Do not rewrite working tests unnecessarily.
4. **For gap ACs (from the gap table)**, write new tests.

### Test Organization (follow existing patterns)

- **Unit tests:** `backend/tests/unit/services/engine/` — for pure logic (result parsing, evidence extraction)
- **Integration tests:** `backend/tests/integration/` — for API endpoint tests with real DB
- **Security tests:** `backend/tests/security/` — for auth/RBAC/encryption specs
- **Regression tests:** `backend/tests/regression/` — for specific bug fixes discovered during spec work
- Add `# Spec: specs/pipelines/scan-execution.spec.md` header comment to each test file to establish traceability

### Test Patterns (follow existing conventions)

```python
# Spec: specs/pipelines/scan-execution.spec.md

@pytest.mark.integration
async def test_start_scan_duplicate_prevention(client, auth_headers, host_with_active_scan):
    """AC-5: Duplicate scan request for host with active scan → 409."""
    response = await client.post(
        "/api/scans/kensa/start",
        json={"host_id": str(host_with_active_scan.id)},
        headers=auth_headers,
    )
    assert response.status_code == 409
    data = response.json()
    assert "SCAN_IN_PROGRESS" in str(data)
```

### Rules for This Phase

- **Tests must fail before implementation.** If a gap AC test passes immediately, either the behavior already exists (update the reverse spec) or the test is not specific enough.
- **Test the spec, not the implementation.** Tests assert on API responses, database state, and side effects — not on internal method calls or class structure.
- **Use existing test fixtures.** OpenWatch already has `conftest.py` with database setup, auth helpers, etc. Extend them, don't rebuild them.
- **Use the boundary extraction method** from Module 03, Chapter 1, Section 1.4 to identify edge cases. For the scan pipeline, key boundaries include: SSH connection timeout, Kensa returning zero results, Kensa returning partial results, concurrent scan attempts, host with no credentials.
- **Follow the pytest marker convention:** `@pytest.mark.unit`, `.integration`, `.security`, `.regression` — as defined in `context/TESTING_STRATEGY.md`.

---

## Phase 5: Establish the Going-Forward Contract

**Goal:** Transition the specced modules from code-first to spec-first workflow.

Once the target specs are approved and tests pass:

1. **Create `specs/SPEC_REGISTRY.md`** following the template in the transition plan (Phase 6). This is the master index.
2. **Update `CLAUDE.md`** (root and/or backend) to reference the spec directory and enforce the spec-first workflow for specced modules. Add a section like:
   ```markdown
   ## Spec-Driven Modules
   The following modules have formal specs in `specs/`. For these modules, the development workflow is:
   update spec → update tests → update code → validate.
   The spec is the SSOT. If spec and code disagree, the spec wins (once approved by a human).
   See specs/SPEC_REGISTRY.md for the full index.
   ```
3. **Update `context/TESTING_STRATEGY.md`** to add a "Spec-Derived Tests" section explaining the AC annotation convention and traceability requirements.
4. **Update `BACKLOG.md`** to reflect completed E5-G items satisfied by spec-derived tests.

---

## Guiding Principles (Refer to These Throughout)

1. **The spec is the SSOT.** If spec and code disagree, the spec wins (once approved by a human). See Module 01, Chapter 2.

2. **Archaeology before architecture.** Never propose target specs without first completing the archaeology and reverse spec for that module. See Module 05, Chapter 1, Section 1.2.

3. **One bounded slice at a time.** Complete one pipeline/module through all phases before moving to the next. The transition plan defines the priority order: scan pipeline → remediation → temporal compliance → auth/RBAC.

4. **Spec the behavior, not the implementation.** A spec says "MUST update scan status to FAILED when SSH connection times out." It does NOT say "call `scan_record.status = ScanStatus.FAILED` in the except block." See Module 01, Chapter 3, Section 3.7.

5. **Every AC must be testable.** If you cannot write a test for an acceptance criterion, the AC is too vague. Rewrite it. See Module 03, Chapter 1.

6. **Build on existing contracts.** OpenWatch already has Pydantic schemas, ORSA dataclasses, SQLAlchemy models, and rich context documentation. Specs extend these (adding behavioral rules, error handling, side effects) — they don't replace them.

7. **Mark what you don't touch.** Use the Scoping Matrix from Module 05, Chapter 1, Section 1.4. Explicitly list modules that are out of scope for each phase. The frontend is out of scope until backend specs are established.

8. **Human approval gates for high-risk changes.** Any spec AC that involves SSH operations on remote hosts, remediation execution, credential handling, or audit-facing compliance data must be reviewed and approved by a human before tests or code are written. See Module 05, Chapter 3.

9. **Respect the existing test infrastructure.** Follow pytest markers, use existing conftest fixtures, maintain the CI threshold (31% and rising). New spec-derived tests should increase coverage, not break the pipeline.

10. **Align with BACKLOG.md.** The SDD transition should satisfy existing backlog items where possible (E5-G3 through E5-G7). When spec work reveals new items, add them to BACKLOG.md using the existing priority scheme.

---

## Sequence Summary

```
Phase 1: Archaeology     → Produce Archaeology Report for scan execution pipeline
Phase 2: Reverse Specs   → Spec current behavior (as-is, including gaps)
Phase 3: Target Specs    → Spec desired behavior + gap analysis table
Phase 4: Spec Tests      → Write/annotate tests traced to spec ACs
Phase 5: Going-Forward   → Establish spec-first workflow + registry + update docs

Then repeat Phases 1-5 for the next priority module:
  → Remediation pipeline (SSH root operations, rollback, risk classification)
  → Temporal compliance engine (scores, drift, exceptions, edge cases)
  → Auth/RBAC (JWT lifecycle, authorization matrix, MFA)
  → High-impact API routes (output contracts, error taxonomy)
```

---

## Appendix: OpenWatch vs. JWTMS — Spec Strategy Comparison

| Dimension | JWTMS (Bookwell) | OpenWatch |
|-----------|-------------------|-----------|
| **Primary risk** | Financial loss, PHI exposure | Incorrect compliance posture, unsafe remediation |
| **Spec entry point** | Payment pipeline (money path) | Scan execution pipeline (core product) |
| **Existing schemas** | Prisma + Zod (input schemas) | SQLAlchemy + Pydantic (input/output schemas) |
| **Existing contracts** | None beyond schemas | ORSA v2.0 plugin interface (typed ABC + dataclasses) |
| **Test pattern** | Vitest integration + mocked Stripe | Pytest integration + real PostgreSQL + mocked SSH |
| **Spec granularity** | Per-API-route (135 routes, prioritized) | Pipeline-first, then per-service (50+ services) |
| **Unique SDD need** | Cross-module flow specs (booking, cancellation) | Multi-step async pipeline specs (scan → snapshot → drift → alert) |
| **Biggest gap** | No output contracts or side-effect specs | No pipeline behavioral specs, 32% test coverage |
| **Safety-critical path** | Stripe charges, PHI encryption | SSH root operations, remediation rollback |

Both projects benefit from the same SDD fundamentals (Context/Objective/Constraints, spec-to-test mapping, SSOT). The key difference: JWTMS needs flow-level specs for its money path; OpenWatch needs pipeline-level specs for its scan-to-remediation chain, with special attention to the SSH safety boundary.

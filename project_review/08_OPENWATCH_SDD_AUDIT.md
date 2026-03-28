# OpenWatch — SDD Readiness Audit

> Reviewed against the principles of Spec-Driven Development (SDD) Module 01: Foundations, Module 02: Architecture, and Module 03: Validation.

---

## 1. Project Summary

**OpenWatch** is an open-source (AGPLv3) continuous compliance platform for Linux infrastructure. It evaluates hosts against STIG, CIS, NIST 800-53, PCI-DSS, and FedRAMP frameworks via SSH-based scanning (no agents), provides temporal compliance posture queries, drift detection, exception governance, and automated remediation with rollback.

**Tech stack:** FastAPI (Python 3.12), React 19 + TypeScript (strict), PostgreSQL 15 via SQLAlchemy 2.0, Redis + Celery, Kensa v1.1.0 compliance engine (ORSA v2.0 plugin), Pydantic 2.x, Vite, MUI v7, Redux Toolkit, Playwright.

**Scale:** ~200+ Python files, 229 TypeScript/TSX files, 80+ API endpoints across 14 route packages, 50+ service modules across 20+ packages, 30+ SQLAlchemy models, 9 Pydantic schema files, 290+ backend tests (32% coverage), 246 E2E tests (Playwright), 88 frontend unit tests.

**Status:** Post-MVP. 7 production readiness epics complete (E0-E6). Active development on Kensa integration gaps and OpenWatch OS features.

---

## 2. What OpenWatch Already Does Well (SDD-Adjacent Strengths)

OpenWatch has an unusually strong documentation and architecture culture for a project at this stage. Several SDD principles are partially in place.

### 2.1 Pydantic Schemas as API Contracts

Nine dedicated schema files in `backend/app/schemas/` define structured request/response models using Pydantic 2.x with strict mode. This is textbook **Schema-First Design** (Module 02, Chapter 1) at the API boundary:

```python
class PostureQueryRequest(BaseModel):
    host_id: UUID
    as_of: Optional[date] = Field(None, description="Point-in-time query date")
    include_rule_states: bool = Field(False, description="Include per-rule state details")
```

The posture schemas are particularly well-structured — they define typed request models, response models with field descriptions, and nested dataclasses for complex structures like `DriftEvent`, `ValueDriftEvent`, and `GroupDriftRuleSummary`.

### 2.2 ORSA v2.0 as a Plugin Interface Contract

The ORSA (OpenWatch Remediation System Adapter) plugin architecture defines an abstract base class with typed dataclasses for all plugin interactions:

- `ORSAPlugin` — abstract interface with `check()`, `remediate()`, `rollback()` methods
- `CheckResult`, `RemediationResult`, `RollbackResult` — typed result dataclasses
- `CanonicalRule`, `PluginInfo`, `HostMetadata` — shared data contracts
- `Capability` enum — declares what a plugin can do

This is a well-designed **Component Contract** (Module 02, Chapter 2) — the plugin interface is the spec, and Kensa is an implementation.

### 2.3 SQLAlchemy Models as Data SSOT

`backend/app/models/sql_models.py` serves as the single source of truth for all data models. 30+ models with UUID primary keys, relationships, constraints, and enums. Alembic migrations are consolidated to a single head for clean schema evolution.

### 2.4 Rich AI Coding Context

Three separate CLAUDE.md files (root, backend, frontend) plus 13 modular context files in `context/` provide comprehensive AI development guidance:

- `ARCHITECTURE_OVERVIEW.md` — tech stack, SQL builders, directory structure
- `MODULE_BOUNDARIES.md` — 20+ service packages with ownership, import rules, dependency graph
- `TESTING_STRATEGY.md` — test pyramid, coverage targets, markers, patterns
- `SECURITY_BEST_PRACTICES.md` — input validation, encryption, JWT, audit
- `SECURITY_STANDARDS_COMPLIANCE.md` — OWASP, NIST, ISO, CMMC, FedRAMP mapping

This is mature **Automated Linting of Intent** (Module 03, Chapter 2) — the `context/` directory functions as a persistent spec constraint layer that survives across AI sessions.

### 2.5 PRD with Epic-Driven Planning

The `PRD/` directory contains a structured 14-week product roadmap with 7 epics, user stories, and status tracking. All 7 epics are complete, indicating disciplined planning and execution. This is closer to spec-driven planning than most projects achieve.

### 2.6 Structured Test Infrastructure

- **Backend:** Pytest with semantic markers (`@pytest.mark.unit`, `.integration`, `.regression`, `.slow`), organized into `tests/unit/`, `tests/integration/`, `tests/security/`, `tests/regression/`
- **Frontend:** Playwright E2E (246 tests) with fixtures, Vitest for unit tests
- **CI:** GitHub Actions with coverage enforcement, conditional E2E execution
- **Coverage thresholds:** Incrementally raised (31% current, 80% target)

### 2.7 Compliance Framework Mapping Files

Kensa's 338 YAML rules and 5 framework mapping files (CIS, STIG, NIST 800-53, PCI-DSS, FedRAMP) are themselves a form of declarative specification — each rule defines checks and expected states in a machine-readable format.

---

## 3. What's Missing: The SDD Gap Analysis

Despite strong foundations, OpenWatch has the same core gap as JWTMS: **behavior is defined by code, not by specs**. The documentation describes what was built, but there is no formal specification that says what should be built and why.

### 3.1 No Formal Spec Files (the SSOT Gap)

**The Problem:** There are no `.spec` files for any module. The "specification" for each feature is distributed across:
- `CLAUDE.md` files (AI guidance — how to write code)
- `context/` (coding standards, architecture patterns)
- `PRD/epics/` (product requirements — what to build)
- `docs/` (operational guides — how to deploy and run)
- Pydantic schemas (input/output shapes — what API accepts/returns)
- SQLAlchemy models (data shapes — what gets stored)
- The code itself (behavioral truth — what actually happens)

Example: If you ask "what should happen when a scan discovers a critical STIG failure on a host that already has an approved exception for that rule?", the answer is scattered across:
- `services/compliance/exceptions.py` (exception logic)
- `services/plugins/orsa/` (check result processing)
- `services/compliance/temporal.py` (posture snapshot creation)
- `services/compliance/alerts.py` (alert generation)
- `schemas/exception_schemas.py` (exception data shapes)
- `routes/compliance/` (API endpoints)

No single document owns that behavioral contract.

**SDD Principle Violated:** SSOT (Module 01, Chapter 2).

### 3.2 Tests Are Code-Derived, Not Spec-Derived

**The Problem:** The 290+ backend tests validate what the code does, but they don't trace back to a behavioral specification. For example, if there are tests for drift detection, they test the implementation's behavior — but there's no spec that says "drift MUST be detected when a rule transitions from pass to fail between snapshots" and "value-only drift MUST be reported separately from status drift."

If someone changes the drift detection algorithm, the tests would be updated to match the new code. There's no spec that anchors what "correct" means independently of the implementation.

**SDD Principle Violated:** Spec-to-Test Mapping (Module 03, Chapter 1).

### 3.3 Scan Pipeline Lacks a Behavioral Spec

**The Problem:** The scan execution pipeline is the core of the product — it flows through:
```
API request → Celery task → SSH connection → Kensa check() →
result parsing → evidence storage → posture snapshot →
drift detection → alert generation → notification
```

This multi-step pipeline spans 6+ service packages, involves async task execution, SSH connections to remote hosts, and database writes. But there's no single spec that defines:
- The complete sequence of operations
- What happens when each step fails (SSH timeout, Kensa error, DB failure)
- Idempotency guarantees (what if the same scan runs twice?)
- The exact data transformations between steps
- What constitutes a "complete" scan vs. a "partial" scan

### 3.4 RBAC Authorization Contracts Are Implicit

**The Problem:** OpenWatch has 6 roles (VIEWER through AUDITOR) with route-level `@require_role()` decorators. But there's no specification that defines:
- Which endpoints each role can access (the authorization matrix)
- What data filtering applies per role (e.g., can an ANALYST see all hosts or only assigned ones?)
- What happens on authorization failure (consistent error shape?)
- How dual-role users are handled

The authorization behavior is encoded in decorators scattered across 14 route packages.

### 3.5 Temporal Compliance Engine Lacks Edge Case Specs

**The Problem:** Temporal compliance queries ("what was compliance on Feb 1st?") are a key differentiator. The `TemporalComplianceService` and `PostureResponse` schemas define the API shape, but not:
- What happens when no snapshot exists for the requested date (nearest? error? interpolation?)
- How compliance scores are calculated (numerator/denominator definition)
- Precedence when a rule has both a scan result AND an approved exception
- Edge cases: first scan ever, host with zero applicable rules, framework with no mapped rules

### 3.6 Remediation Pipeline Needs Behavioral Specs

**The Problem:** The remediation workflow (K-2, K-3) is complete and handles real system changes on remote hosts — modifying GRUB configs, PAM settings, fstab entries. This is the highest-risk code path (SSH root operations with rollback). There's no spec that defines:
- The complete remediation lifecycle (request → approval → execute → verify → rollback if needed)
- Risk classification rules and approval gate thresholds (K-4 is not implemented)
- Rollback guarantees (what is a successful rollback? what if rollback fails?)
- Concurrent remediation handling (two remediations on the same host)

### 3.7 Encryption/PHI Handling Lacks Formal Spec

**The Problem:** OpenWatch handles credentials (SSH keys, passwords) and potentially sensitive compliance data. The `EncryptionService` uses AES-256-GCM, but there's no spec that defines:
- What fields are encrypted at rest
- Key management lifecycle (rotation, backup)
- Who can decrypt and under what authorization
- Audit requirements for encrypted data access

---

## 4. SDD Maturity Assessment

| Dimension | Current State | Level |
|-----------|--------------|-------|
| Data models (SQLAlchemy) | Schema-first, Alembic migrations | **Spec-Driven** |
| API input validation (Pydantic) | Schema-first at boundaries, 9 schema files | **Spec-Driven** |
| Plugin interface (ORSA v2.0) | Abstract class + typed dataclasses | **Spec-Driven** |
| Compliance rule definitions (Kensa YAML) | Declarative, machine-readable | **Spec-Driven** |
| API output contracts | Pydantic response models for some routes | **Spec-Aware** |
| Business logic (50+ services) | Code-first, documented in context files | **Pre-Spec** |
| Scan execution pipeline | Code-first, no behavioral spec | **Pre-Spec** |
| Remediation workflow | Code-first, implemented but unspecified | **Pre-Spec** |
| RBAC authorization | Decorator-based, no authorization matrix | **Pre-Spec** |
| Temporal compliance | Pydantic schemas + code, edge cases undefined | **Spec-Aware** |
| Drift detection | Recently implemented, no behavioral spec | **Pre-Spec** |
| Test derivation | Code-first, semantic markers, CI enforcement | **Spec-Aware** |
| AI coding constraints | Rich CLAUDE.md + 13 context files | **Spec-Aware** |
| Infrastructure (Docker, CI) | Declarative (Compose, GitHub Actions) | **Spec-Driven** |
| Encryption/credentials | Implemented, not formally specified | **Pre-Spec** |

**Overall: Spec-Aware** — strong documentation culture, good typing discipline, and excellent AI context infrastructure, but no formal spec-to-test pipeline. The project is well-documented but not spec-driven.

---

## 5. Key Insight: Where SDD Will Add the Most Value

OpenWatch has a characteristic that makes SDD adoption especially valuable: **it executes privileged operations on remote systems via SSH**. In the SDD risk framework, these are operations where ambiguity has the highest blast radius.

### 5.1 The Scan Pipeline (Correctness-Critical)

The flow from API request through Kensa check to posture snapshot is the core value proposition. A spec for this pipeline would:
- Define every state transition explicitly (scan states, finding states)
- Map every Kensa interaction to expected behavior
- Make evidence storage guarantees auditable
- Give you spec-derived tests for scan failure modes (SSH timeout, partial results, duplicate scans)

### 5.2 The Remediation Pipeline (Safety-Critical)

Remediation executes root-level changes on production Linux systems — GRUB, PAM, fstab, sysctl. A spec for this pipeline would:
- Define the complete lifecycle with explicit approval gates
- Classify remediation risk and map to approval requirements
- Specify rollback guarantees and failure handling
- Make concurrent remediation rules explicit
- Provide spec-derived tests for every risk level

### 5.3 The Temporal Compliance Engine (Accuracy-Critical)

Compliance posture queries drive audit reports and executive dashboards. Incorrect scores have regulatory consequences. A spec would:
- Define score calculation precisely (what counts, what doesn't)
- Specify exception interaction rules
- Define drift detection thresholds and classification
- Make temporal query edge cases (no data, first scan, framework changes) testable from the spec

### 5.4 The Auth/RBAC System (Security-Critical)

JWT RS256, Argon2id, TOTP 2FA, 6-role RBAC — this is security-critical code at 100% coverage target. A spec would:
- Define the authorization matrix (role × endpoint × action)
- Specify session lifecycle (token creation, refresh, revocation)
- Define MFA enrollment and verification flows
- Make credential handling rules auditable

These four areas are where SDD will deliver the highest ROI.

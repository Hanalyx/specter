# OpenWatch — SDD Transition Plan

> A phased approach to evolving OpenWatch from code-first to spec-driven, with spec-derived tests.

---

## Philosophy: Spec the Scan Pipeline First

Module 05, Chapter 1 says: "spec the seams where ambiguity causes the most damage." For OpenWatch, a compliance platform that executes SSH commands on production Linux systems, the highest-damage seams are:

1. **Scan execution pipeline** — Core product value; correctness determines compliance accuracy
2. **Remediation workflow** — SSH root operations; safety-critical with rollback requirements
3. **Temporal compliance engine** — Audit-facing data; regulatory accuracy requirements
4. **Auth/RBAC** — Security-critical; 100% coverage target already declared

Start with scanning. Every other feature (drift, posture, remediation, alerts) depends on scans producing correct, complete results.

---

## Proposed Spec Directory Structure

```
~/hanalyx/openwatch/
└── specs/
    ├── SPEC_REGISTRY.md                        # Master index of all specs

    ├── pipelines/                              # Multi-step workflow specs
    │   ├── scan-execution.spec.md              # API → Celery → SSH → Kensa → DB → snapshot
    │   ├── remediation-lifecycle.spec.md        # Request → approve → execute → verify → rollback
    │   └── drift-detection.spec.md             # Snapshot comparison → event classification → alert

    ├── services/                               # Business logic module specs
    │   ├── compliance/
    │   │   ├── temporal-compliance.spec.md      # Posture queries, score calculation, snapshot rules
    │   │   ├── exception-governance.spec.md     # Exception lifecycle, approval, expiry, rule interaction
    │   │   ├── alert-thresholds.spec.md         # Alert generation rules, severity, deduplication
    │   │   └── drift-analysis.spec.md           # Status drift, value drift, group drift rules
    │   ├── engine/
    │   │   ├── kensa-scan.spec.md               # Kensa check() invocation, result parsing, evidence storage
    │   │   └── scan-orchestration.spec.md       # Celery task lifecycle, concurrency, retry, timeout
    │   ├── remediation/
    │   │   ├── remediation-execution.spec.md    # Kensa remediate() invocation, step execution, rollback
    │   │   └── risk-classification.spec.md      # Risk levels, approval gates, auto-approve rules (K-4)
    │   ├── auth/
    │   │   ├── authentication.spec.md           # JWT lifecycle, Argon2id, token refresh, session mgmt
    │   │   ├── authorization-matrix.spec.md     # Role × endpoint × action matrix
    │   │   └── mfa.spec.md                      # TOTP enrollment, verification, backup codes
    │   ├── ssh/
    │   │   └── ssh-connection.spec.md           # Connection lifecycle, key validation, timeout, retry
    │   └── encryption/
    │       └── credential-encryption.spec.md    # AES-256-GCM, key mgmt, what is encrypted, audit

    ├── api/                                    # API route behavioral specs
    │   ├── scans/
    │   │   ├── start-kensa-scan.spec.md         # POST /api/scans/kensa/start
    │   │   └── scan-results.spec.md             # GET /api/scans/:id/results
    │   ├── compliance/
    │   │   ├── posture-query.spec.md            # GET /api/compliance/posture
    │   │   ├── drift-query.spec.md              # GET /api/compliance/drift
    │   │   └── exception-crud.spec.md           # /api/compliance/exceptions CRUD
    │   ├── remediation/
    │   │   ├── start-remediation.spec.md        # POST /api/remediation/start
    │   │   └── rollback.spec.md                 # POST /api/remediation/rollback
    │   └── auth/
    │       ├── login.spec.md                    # POST /api/auth/login
    │       └── mfa-verify.spec.md               # POST /api/auth/mfa/verify

    └── plugins/                                # Plugin interface specs
        └── orsa-v2.spec.md                     # ORSA plugin contract (already partially spec'd via ABC)
```

---

## Phase 1: Scan Execution Pipeline Specs (Start Here)

**Goal:** Spec the complete scan pipeline — from API request through Kensa check to posture snapshot creation.

**Why this first:** The scan pipeline is the foundation of every other feature. Drift detection, posture queries, alerts, and remediation all consume scan results. If scans are wrong, everything downstream is wrong.

**Target modules:**
- `backend/app/routes/scans/` (API endpoints)
- `backend/app/tasks/` (Celery scan tasks)
- `backend/app/services/engine/` (scan execution, SSH, result parsing)
- `backend/app/services/plugins/orsa/` (ORSA interface + Kensa bridge)
- `backend/app/services/compliance/temporal.py` (posture snapshot creation)

### 1.1 Micro-Spec: Start Kensa Scan

```markdown
# Spec: Start Kensa Scan

## Context

- **System:** OpenWatch compliance platform
- **Route:** POST /api/scans/kensa/start
- **Dependencies:** Celery task queue, SSHConnectionManager, Kensa ORSA plugin, TemporalComplianceService
- **Callers:** Frontend scan UI (ScanPanel), scheduled scan tasks (ComplianceSchedulerService)

## Objective

Initiate an asynchronous compliance scan of a single host using the Kensa ORSA plugin.

### Input Contract (Pydantic)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| host_id | UUID | yes | Target host to scan |
| framework | str | no | Specific framework to evaluate (default: all mapped) |

### Behavior

1. Validate the host exists and has stored credentials
2. Validate the requesting user has ANALYST or higher role
3. Check no scan is already in-progress for this host (prevent duplicate)
4. Create Scan record in database (status: QUEUED)
5. Dispatch Celery task for async execution
6. Return scan ID and status immediately (202 Accepted)

**Async execution (Celery task):**
7. Update Scan status to IN_PROGRESS
8. Establish SSH connection to host via SSHConnectionManager
9. Invoke Kensa plugin check() via ORSA interface
10. Parse CheckResult into scan findings with evidence
11. Store findings in scan_findings table (with evidence JSONB)
12. Create posture snapshot via TemporalComplianceService
13. Run drift detection against previous snapshot
14. Generate alerts if drift exceeds thresholds
15. Update Scan status to COMPLETED (or FAILED with error details)

### Output Contract

**Success (202):**
```json
{
  "scan_id": "uuid",
  "status": "QUEUED",
  "host_id": "uuid",
  "message": "Scan queued for execution"
}
```

**Error responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | VALIDATION_ERROR | Invalid input |
| 401 | UNAUTHORIZED | Not authenticated |
| 403 | FORBIDDEN | Role below ANALYST |
| 404 | HOST_NOT_FOUND | Host doesn't exist |
| 409 | SCAN_IN_PROGRESS | Host already has active scan |
| 500 | INTERNAL_ERROR | Unexpected failure |

### Acceptance Criteria

- [ ] AC-1: Authenticated ANALYST can start scan → 202 with scan_id
- [ ] AC-2: VIEWER role → 403 FORBIDDEN
- [ ] AC-3: Invalid host_id → 404 HOST_NOT_FOUND
- [ ] AC-4: Host without credentials → appropriate error (400 or 422)
- [ ] AC-5: Duplicate scan → 409 SCAN_IN_PROGRESS
- [ ] AC-6: Scan record created in DB with QUEUED status before task dispatch
- [ ] AC-7: Celery task updates status to IN_PROGRESS on start
- [ ] AC-8: SSH connection failure → Scan status FAILED with error detail
- [ ] AC-9: Kensa check() failure → Scan status FAILED, partial results NOT stored
- [ ] AC-10: Successful scan → all findings stored with evidence JSONB
- [ ] AC-11: Posture snapshot created after successful scan
- [ ] AC-12: Drift detection runs against previous snapshot
- [ ] AC-13: Alerts generated when drift exceeds configured thresholds
- [ ] AC-14: Scan status COMPLETED with timestamps on success
- [ ] AC-15: Scan timeout after configurable limit → FAILED status

## Constraints

- MUST use ORSA plugin interface (not Kensa directly) for all compliance checks
- MUST store evidence in scan_findings.evidence JSONB column
- MUST create posture snapshot even if some rules error (partial snapshot)
- MUST NOT allow concurrent scans on the same host
- MUST log scan start/completion to security audit log
- MUST use parameterized SQL via builders (no raw SQL)
- MUST handle SSH connection cleanup on failure (no leaked connections)
```

### 1.2 Additional Phase 1 Specs

Write similar micro-specs for:

1. **`scan-orchestration.spec.md`** — Celery task lifecycle: queuing, retry policy, timeout, cancellation, concurrent scan limits
2. **`kensa-scan.spec.md`** — Kensa ORSA plugin invocation: check() contract, result parsing, evidence extraction, error handling
3. **`scan-results.spec.md`** — GET scan results API: response shape, finding detail, evidence inclusion, pagination

### 1.3 Map Existing Tests to Spec ACs

The existing `test_scans.py` integration test and `test_executors.py` unit test should be annotated:

```python
# Spec: specs/api/scans/start-kensa-scan.spec.md
@pytest.mark.integration
async def test_start_scan_authenticated(client, auth_headers):
    """AC-1: Authenticated ANALYST can start scan."""
    ...

@pytest.mark.integration
async def test_start_scan_unauthorized(client):
    """AC-2: VIEWER role → 403 FORBIDDEN."""
    ...

# AC-5: Duplicate scan prevention — likely MISSING
# AC-8: SSH connection failure handling — likely MISSING
# AC-9: Kensa check() failure handling — likely MISSING
# AC-15: Scan timeout — likely MISSING
```

### 1.4 Estimated Gaps

Based on 32% backend coverage and the gap between 290 tests and 80+ endpoints + 50+ services:

- **AC-5 (Duplicate prevention)** — Concurrent scan guard
- **AC-8 (SSH failure)** — Connection failure → scan status update
- **AC-9 (Kensa failure)** — Partial result handling
- **AC-10 (Evidence storage)** — Verify evidence JSONB populated correctly
- **AC-12 (Drift detection trigger)** — Scan → snapshot → drift pipeline integration
- **AC-15 (Timeout)** — Scan timeout handling

---

## Phase 2: Remediation Pipeline Specs

**Goal:** Spec the remediation lifecycle — the highest-risk code path (SSH root operations on remote systems).

### 2.1 Remediation Lifecycle Spec

The complete remediation pipeline:
```
Request remediation → Validate host/rule/credentials →
Check risk level → [Approval gate for high-risk] →
SSH connect → Create pre-state snapshot →
Execute remediation steps via Kensa → Verify each step →
Store results → Verify post-state →
[Rollback if verification fails] → Update scan findings → Alert
```

### 2.2 Risk Classification Spec (K-4)

Kensa classifies remediation steps by risk (high/medium/low). K-4 is not yet implemented. The spec would define:
- Risk classification rules (what makes a remediation step "high risk"?)
- Approval gate thresholds (auto-approve low, require human for high)
- Which system areas are high-risk: GRUB, PAM, fstab, sysctl, SELinux, firewall

### 2.3 Rollback Guarantee Spec

- What constitutes a successful rollback
- What happens when rollback fails (escalation path)
- Pre-state snapshot retention rules (K-5: 7-day active / 90-day archive)

---

## Phase 3: Temporal Compliance Engine Specs

**Goal:** Spec the posture query system — audit-facing data where accuracy has regulatory implications.

### 3.1 Score Calculation Spec

- Numerator/denominator definition (what counts as "passed"?)
- How error, notapplicable, and exception statuses affect scores
- Per-severity breakdown rules
- Group-level aggregation rules

### 3.2 Temporal Query Edge Cases

- No snapshot for requested date (nearest-before? error?)
- First scan ever (no prior data)
- Host with zero applicable rules
- Framework with no mapped rules
- Exception approved mid-period (how does it affect historical scores?)

### 3.3 Drift Analysis Spec

- Status drift vs. value drift classification
- Drift magnitude thresholds (major, minor, improvement, stable)
- Group drift aggregation rules
- Drift event deduplication

---

## Phase 4: Auth/RBAC Specs

**Goal:** Spec the security-critical auth system — the 100% coverage target area.

### 4.1 Authentication Spec

- JWT RS256 lifecycle (creation, refresh, revocation, expiry)
- Argon2id password hashing parameters
- Rate limiting on login attempts
- Account lockout rules

### 4.2 Authorization Matrix Spec

A single table mapping: Role × Route Package × HTTP Method → Allow/Deny

This replaces the implicit `@require_role()` decorators with a single authoritative document. The decorators become the implementation; the matrix is the spec.

### 4.3 MFA Spec

- TOTP enrollment flow (secret generation, QR code, backup codes)
- Verification flow (time window, replay prevention)
- Recovery flow (backup code usage, limits)

---

## Phase 5: API Route Contract Specs

**Goal:** Spec the 10 highest-impact API routes with full input/output/error/side-effect contracts.

| Route | Risk | Existing Tests |
|-------|------|:-:|
| POST /api/scans/kensa/start | CRITICAL — core product | Yes (integration) |
| GET /api/compliance/posture | CRITICAL — audit-facing | Yes (integration) |
| POST /api/remediation/start | CRITICAL — SSH root ops | Unknown |
| POST /api/remediation/rollback | CRITICAL — recovery | Unknown |
| GET /api/compliance/drift | HIGH — drift analysis | Yes (integration) |
| POST /api/compliance/exceptions | HIGH — governance | Yes (integration) |
| POST /api/auth/login | HIGH — security | Yes (integration + security) |
| POST /api/auth/mfa/verify | HIGH — security | Yes (unit) |
| GET /api/hosts | MEDIUM — host inventory | Yes (integration) |
| POST /api/hosts/credentials | MEDIUM — credential storage | Unknown |

---

## Phase 6: Spec Registry and Maintenance

**Goal:** Create `specs/SPEC_REGISTRY.md` as the master index.

```markdown
# OpenWatch Spec Registry

## Pipeline Specs
| Spec | File | Tests | Status |
|------|------|-------|--------|
| Scan Execution | specs/pipelines/scan-execution.spec.md | test_scans.py, test_executors.py | Active |
| Remediation | specs/pipelines/remediation-lifecycle.spec.md | TBD | Active |

## Service Specs
| Spec | File | Tests | Status |
|------|------|-------|--------|
| Temporal Compliance | specs/services/compliance/temporal-compliance.spec.md | test_compliance_api.py | Active |

## Cross-Module Dependencies
- scan-execution.spec → kensa-scan.spec (Kensa invocation)
- scan-execution.spec → temporal-compliance.spec (snapshot creation)
- remediation-lifecycle.spec → risk-classification.spec (approval gates)
- drift-analysis.spec → alert-thresholds.spec (alert generation)
```

---

## Implementation Timeline

| Phase | Scope | Effort | Immediate Value |
|-------|-------|--------|-----------------|
| **Phase 1** | 4 scan pipeline specs | 2-3 sessions | Catches scan failure modes, makes evidence storage auditable |
| **Phase 2** | 3 remediation specs | 2-3 sessions | Safety specs for SSH root operations, rollback guarantees |
| **Phase 3** | 3 temporal compliance specs | 1-2 sessions | Audit-ready score calculation rules, drift classification |
| **Phase 4** | 3 auth/RBAC specs | 1-2 sessions | Authorization matrix as SSOT, MFA lifecycle |
| **Phase 5** | 10 API route specs | 2-3 sessions | Formalizes output contracts and error taxonomy |
| **Phase 6** | Spec registry | Ongoing | Living index of all specs |

**Recommended start:** Phase 1, specifically `scan-execution.spec.md` — it's the core product pipeline, already has some integration tests, and every other feature depends on it.

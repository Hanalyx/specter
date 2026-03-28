# JWTMS (Bookwell) — SDD Readiness Audit

> Reviewed against the principles of Spec-Driven Development (SDD) Module 01: Foundations and Module 02: Architecture.

---

## 1. Project Summary

**JWTMS** (codename Serenity, platform name Bookwell) is a white-label SaaS therapy booking and management platform. Single codebase template deploying as separate isolated instances per client (not multi-tenant).

**Tech stack:** Next.js 16, React 19, TypeScript (strict), MUI v7, PostgreSQL 16 via Prisma 6, NextAuth.js 5, Stripe (Platform + Connect), Vitest, Turborepo monorepo.

**Scale:** 557 TypeScript source files, 135+ API routes, 200+ React components, 40+ library modules, 30+ Prisma models, 333 tests (102 unit + 231 integration).

**Status:** MVP complete (48 P1 tasks done), 17 P2-P4 enhancements remaining.

---

## 2. What JWTMS Already Does Well (SDD-Adjacent Strengths)

JWTMS is a well-documented, thoughtfully-structured production application. Several SDD principles are already partially in place:

### 2.1 Zod Schemas as API Input Contracts

API routes consistently use Zod schemas for request validation, which is textbook **Schema-First Design** (Module 02, Chapter 1) at the API boundary:

```typescript
const createSchema = z.object({
  name: z.string().min(1),
  price: z.number().positive(),
});
type CreateInput = z.infer<typeof createSchema>;
```

This means every API route has a *machine-enforced input contract*. The shape is defined before the logic runs.

### 2.2 Prisma Schema as Data SSOT

The `packages/database/prisma/schema.prisma` (~1500+ lines) serves as the single source of truth for all data models. It defines 30+ models with relationships, constraints, enums, and defaults. Prisma generates the TypeScript client from this schema — a perfect example of **schema → code derivation**.

### 2.3 Rich Context Documentation

The `context/` directory contains 10+ markdown files that function as persistent AI coding constraints:
- `ARCHITECTURE_OVERVIEW.md` — system layers and data flow
- `CODING_STANDARDS.md` — TypeScript/React patterns with code examples
- `TESTING_STRATEGY.md` — test stack, conventions, priority matrix
- `SECURITY.md` — crypto standards, auth patterns
- `CRITICAL_MODULES.md` — protected files with risk tiers
- `GOTCHAS.md` — data structure pitfalls

This is early-stage **Automated Linting of Intent** (Module 03, Chapter 2) — the `context/` directory acts like a persistent spec constraint layer for AI development, similar to `.cursorrules` or `CLAUDE.md`.

### 2.4 Tiered Test Priority Matrix

`TESTING_STRATEGY.md` defines a 5-tier priority matrix for testing (CRITICAL → HIGH → AUTH → MEDIUM → CRUD), aligned with `CRITICAL_MODULES.md`. This is an intentional, risk-based approach to test coverage — not random.

### 2.5 Pre-Commit Quality Gates

Husky + lint-staged enforce ESLint, Prettier, and TypeScript type-checking on every commit. The CI pipeline adds build verification, unit tests, and integration tests with real PostgreSQL.

### 2.6 Platform Tier Feature Gating

The tier system (Basic/Professional) is implemented as a spec-like pattern: features are gated by `requireTier('PROFESSIONAL')`, and the tier definitions in `platform-tiers.ts` serve as a declarative feature manifest.

---

## 3. What's Missing: The SDD Gap Analysis

Despite strong foundations, JWTMS has the same core gap as most production applications: **behavior is defined by code, not by specs**. The documentation describes *what was built*, but there's no formal specification that says *what should be built and why*.

### 3.1 No Formal Spec Files (the SSOT Gap)

**The Problem:** There is no `.spec` file for any module. The "specification" for each feature is distributed across:
- `docs/` (UX descriptions — what the UI should look like)
- `context/` (coding standards — how to write code)
- `CLAUDE.md` (AI guidance — what to check before coding)
- Prisma schema (data shapes — what gets stored)
- Zod schemas (input validation — what API accepts)
- The code itself (behavioral truth — what actually happens)

Example: If you ask "what should happen when a customer cancels an appointment with a deposit?", the answer is scattered across:
- `docs/CUSTOMER_BOOKING_UX.md` (the UX flow)
- `lib/payment-policy/` (the business logic)
- `api/customer/appointments/cancel/route.ts` (the API behavior)
- `schema.prisma` (the Payment model and types)

No single document owns that behavioral contract.

**SDD Principle Violated:** SSOT (Module 01, Chapter 2) — "The spec is authoritative; code is derived from it."

### 3.2 Tests Are Code-Derived, Not Spec-Derived

**The Problem:** The 333 tests validate what the code does, but they don't trace back to a behavioral specification. For example, the buffer integration test (`buffer.integration.test.ts`) tests the fallback chain: service → therapist → business → default (15 min). This is excellent — but the fallback priority is defined *by the code*, not *by a spec*.

If someone changes the fallback order, the tests would be updated to match the new code. There's no spec that says "the buffer precedence MUST be: service > therapist > business > default 15 minutes."

**SDD Principle Violated:** Spec-to-Test Mapping (Module 03, Chapter 1).

### 3.3 API Route Contracts Are Implicit

**The Problem:** JWTMS has 135+ API routes. Each route has a Zod schema for *input* validation, but no formal contract for:
- **Output shape** — What JSON structure is returned on success? On error?
- **Side effects** — What database writes occur? What emails are sent? What Stripe calls are made?
- **Error taxonomy** — Which error codes can this endpoint return and under what conditions?
- **Authorization** — Which roles can access this endpoint? What happens for unauthorized access?

The `JWTMS_API_DOC.md` documents endpoints at a high level, but it's a reference document, not a behavioral spec with testable acceptance criteria.

### 3.4 Business Logic Modules Lack Behavioral Specs

**The Problem:** The 40+ library modules in `lib/` are the heart of the application — availability calculation, pricing, payment policy, booking orchestration, membership billing, gift card redemption. Each implements complex business rules, but the rules exist only in code.

High-risk examples:
- **Availability calculation** (`lib/availability/`): How are buffer times, blocked time, existing appointments, Google Calendar events, and operating hours combined to determine available slots? The algorithm is in the code.
- **Payment policy** (`lib/payment-policy/`): When is a deposit required? How is the balance collected? What happens with cancellation fees? What about no-show fees? The rules are in the code.
- **Pricing** (`lib/pricing/`): How do member discounts interact with discount codes? Are they stackable? Which takes precedence? The answer is in the code.

### 3.5 Component Contracts Are Absent

**The Problem:** The 200+ React components have TypeScript `interface` definitions for props, but no behavioral specifications. For example, the booking flow has components for service selection, therapist selection, time selection, and checkout — but there's no spec that says "the booking flow MUST enforce these validation rules at each step" or "the checkout component MUST handle these error states."

The `docs/CUSTOMER_BOOKING_UX.md` describes the UX *design*, but not the behavioral *contract* that the components must satisfy.

### 3.6 No Spec for the Stripe Integration Contract

**The Problem:** The Stripe integration is arguably the most critical subsystem — it handles real money. There are two separate Stripe clients (Platform and Connect), webhook handlers, payment intent creation, refund processing, and subscription billing. The `CRITICAL_MODULES.md` flags these files as highest-risk, but there's no behavioral spec that defines:
- What Stripe API calls are made during each user flow
- What happens when Stripe returns specific error codes
- How webhook idempotency is enforced
- What constitutes a "successful" vs "failed" payment flow

---

## 4. SDD Maturity Assessment

| Dimension | Current State | Level |
|-----------|--------------|-------|
| Data models (Prisma schema) | Schema-first, generated client | **Spec-Driven** |
| API input validation (Zod) | Schema-first at boundaries | **Spec-Driven** |
| API output contracts | Implicit in code | **Pre-Spec** |
| Business logic (40+ modules) | Code-first, documented retroactively | **Pre-Spec** |
| Component behavior | Props typed, behavior implicit | **Spec-Aware** |
| Test derivation | Code-first, priority matrix exists | **Spec-Aware** |
| AI coding constraints | Rich context/ directory | **Spec-Aware** |
| Feature gating (tiers) | Declarative tier definitions | **Spec-Driven** |
| Security (crypto, auth) | Documented standards, tested | **Spec-Aware** |
| UX flows | Rich design docs | **Spec-Aware** |
| Payment/Stripe contracts | CRITICAL tier flagged, partially tested | **Pre-Spec** |

**Overall: Spec-Aware** — strong documentation culture and good tooling, but no formal spec-to-test pipeline. The project is *well-documented* but not *spec-driven*.

---

## 5. Key Insight: Where SDD Will Add the Most Value

JWTMS has a specific characteristic that makes SDD adoption especially valuable: **it handles real money and protected health information (PHI)**. In the SDD risk framework, these are the places where ambiguity has the highest cost.

### 5.1 The Money Path

The flow from booking → payment intent → Stripe charge → receipt → potential refund/cancellation fee is a multi-step pipeline involving 6+ modules, 4+ API routes, and 2 Stripe accounts (Platform + Connect). A spec for this pipeline would:
- Define every state transition explicitly
- Map every Stripe API call to a business rule
- Make cancellation/refund/fee logic auditable
- Give you spec-derived tests for every financial edge case

### 5.2 The PHI Path

PHI encryption (`lib/crypto/`, `lib/soap-notes/`, intake forms) must comply with HIPAA. A spec for PHI handling would:
- Define what fields are encrypted and with what algorithm
- Define who can decrypt and under what authorization
- Map every access to an audit log entry
- Make compliance auditable from the spec, not just the code

### 5.3 The Availability Engine

The availability calculation combines operating hours, therapist schedules, blocked time, buffer time, existing appointments, and Google Calendar events. This is complex, correctness-critical logic. A spec would:
- Define the precedence rules explicitly
- Document every source of unavailability
- Make edge cases (midnight crossing, timezone boundaries, zero-buffer) testable from the spec

These three areas are where SDD will deliver the highest ROI.

# JWTMS — SDD Transition Plan

> A phased approach to evolving JWTMS from code-first to spec-driven, with spec-derived tests.

---

## Philosophy: Spec the Money Path First

Module 05, Chapter 1 says: "spec the seams where ambiguity causes the most damage." For JWTMS, a therapy SaaS handling real payments and PHI, the highest-damage seams are:

1. **Payment flows** — Real money, Stripe API interactions, refunds, fees
2. **Availability engine** — Booking correctness, scheduling conflicts
3. **PHI handling** — HIPAA compliance, encryption, access control
4. **Business logic modules** — Pricing, memberships, gift cards

Start with payments. A bug there costs you actual money and customer trust.

---

## Proposed Spec Directory Structure

```
~/projects/jwtms/
└── specs/
    ├── SPEC_REGISTRY.md                    # Master index of all specs
    │
    ├── api/                                # API route behavioral specs
    │   ├── appointments/
    │   │   ├── create-appointment.spec.md  # POST /api/appointments
    │   │   └── cancel-appointment.spec.md  # POST /api/customer/appointments/cancel
    │   ├── payments/
    │   │   ├── create-intent.spec.md       # POST /api/payments/create-intent
    │   │   └── confirm-payment.spec.md     # POST /api/payments/confirm
    │   └── webhooks/
    │       ├── connect-webhook.spec.md     # POST /api/connect/webhook
    │       └── platform-webhook.spec.md    # POST /api/platform/webhook
    │
    ├── lib/                                # Business logic module specs
    │   ├── availability/
    │   │   ├── buffer-resolution.spec.md   # Buffer time precedence
    │   │   └── slot-calculation.spec.md    # Available slot computation
    │   ├── pricing/
    │   │   └── pricing-calculation.spec.md # Price computation with discounts/memberships
    │   ├── payment-policy/
    │   │   └── payment-policy.spec.md      # Deposits, balance, fees, refunds
    │   ├── stripe/
    │   │   └── stripe-service.spec.md      # Stripe API interaction contracts
    │   ├── crypto/
    │   │   └── phi-encryption.spec.md      # PHI encryption/decryption contract
    │   └── gift-cards/
    │       └── gift-card-redemption.spec.md
    │
    └── flows/                              # Cross-module flow specs
        ├── booking-flow.spec.md            # End-to-end: select → book → pay → confirm
        ├── cancellation-flow.spec.md       # End-to-end: cancel → refund/fee → notify
        └── membership-billing.spec.md      # Recurring billing lifecycle
```

---

## Phase 1: Payment Pipeline Specs (Start Here)

**Goal:** Spec the money path — from payment intent creation through Stripe charge to receipt, including refunds, cancellation fees, and deposit/balance flows.

**Why this first:** Financial bugs have the highest blast radius. The payment code is flagged CRITICAL in `CRITICAL_MODULES.md`. Integration tests already exist for these routes, but they test *what the code does*, not *what the spec says it should do*.

### 1.1 Micro-Spec: Payment Intent Creation

```markdown
# Spec: Create Payment Intent

## Context

- **System:** JWTMS booking platform, `apps/web/src/app/api/payments/create-intent/route.ts`
- **Role:** Creates a Stripe PaymentIntent for an appointment booking
- **Dependencies:** Stripe SDK (Connected Account), Prisma, payment-policy service, pricing service
- **Callers:** Booking flow checkout step (client-side)

## Objective

Create a Stripe PaymentIntent on the connected account for the given appointment,
applying the business's payment policy (full upfront, deposit, or post-service).

### Input Contract (Zod Schema)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| appointmentId | string (cuid) | yes | The appointment to charge for |
| paymentMethodId | string | no | Stripe payment method (if paying now) |
| giftCardCode | string | no | Gift card to redeem against balance |

### Behavior

1. Validate the appointment exists and belongs to the authenticated customer
2. Determine payment type from PaymentPolicy:
   - FULL_UPFRONT → charge full amount
   - DEPOSIT_PLUS_BALANCE → charge deposit percentage
   - FULL_AFTER_SERVICE → create $0 intent (hold only)
3. Apply gift card balance (if provided):
   - Validate gift card is active and has sufficient balance
   - Reduce charge amount by gift card balance
   - Reserve gift card amount (deduct from balance)
4. Apply member pricing (if customer has active membership)
5. Create Stripe PaymentIntent on connected account
6. Create Payment record in database (status: PENDING)
7. Return client_secret for frontend confirmation

### Output Contract

**Success (200):**
```json
{
  "success": true,
  "clientSecret": "pi_xxx_secret_yyy",
  "paymentId": "clpay...",
  "amount": 12000,
  "paymentType": "FULL"
}
```

**Error responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | VALIDATION_ERROR | Invalid input |
| 401 | UNAUTHORIZED | Not authenticated |
| 404 | APPOINTMENT_NOT_FOUND | Appointment doesn't exist |
| 409 | ALREADY_PAID | Appointment already has a payment |
| 402 | GIFT_CARD_INSUFFICIENT | Gift card balance too low |
| 500 | STRIPE_ERROR | Stripe API failure |

### Acceptance Criteria

- [ ] AC-1: Full upfront payment → PaymentIntent for full amount
- [ ] AC-2: Deposit payment → PaymentIntent for deposit percentage only
- [ ] AC-3: Post-service payment → PaymentIntent for $0 (hold)
- [ ] AC-4: Gift card reduces charge amount correctly
- [ ] AC-5: Gift card balance reserved (deducted) on intent creation
- [ ] AC-6: Member pricing applied before Stripe charge
- [ ] AC-7: Payment record created with PENDING status
- [ ] AC-8: Connected account ID used (not platform account)
- [ ] AC-9: Idempotent — duplicate calls don't create duplicate intents
- [ ] AC-10: Stripe failure → no Payment record created, gift card not deducted
- [ ] AC-11: 401 returned for unauthenticated requests

## Constraints

- MUST use the connected Stripe account (not platform account) for all customer charges
- MUST create audit log entry for every payment action
- MUST NOT store raw card numbers — only Stripe token references
- MUST use Decimal.js for all financial calculations (no floating point)
- MUST handle Stripe API errors gracefully without exposing internal details
```

### 1.2 Additional Phase 1 Specs

Write similar micro-specs for:

1. **`confirm-payment.spec.md`** — Payment confirmation after frontend Stripe.js
2. **`connect-webhook.spec.md`** — Stripe Connect webhook handling (payment_intent.succeeded, charge.refunded, etc.)
3. **`payment-policy.spec.md`** — Business rules for deposit/balance/fee determination
4. **`stripe-service.spec.md`** — Stripe API wrapper contract (what calls are made, what errors are handled)

### 1.3 Map Existing Tests to Spec ACs

The existing integration tests for `create-intent` and `confirm` are solid. Annotate them:

```typescript
describe('Integration: /api/payments/create-intent', () => {
  it('should create payment intent for full amount', async () => {
    // AC-1: Full upfront payment → PaymentIntent for full amount
  });

  it('should create deposit intent when policy requires', async () => {
    // AC-2: Deposit payment → PaymentIntent for deposit percentage
  });

  // AC-10: Stripe failure scenario — likely MISSING
  // AC-9: Idempotency — likely MISSING
});
```

### 1.4 Estimated Gaps

Based on typical code-first test patterns, these ACs are likely untested:
- **AC-9 (Idempotency)** — Duplicate intent prevention
- **AC-10 (Atomicity)** — Stripe failure rollback behavior
- **AC-4/5 (Gift card interaction)** — Gift card + payment interaction edge cases
- **AC-6 (Member pricing + payment)** — Pricing integration with payment flow

---

## Phase 2: Availability Engine Specs

**Goal:** Spec the availability calculation — the second most critical business logic.

### 2.1 Buffer Resolution Spec

The buffer fallback chain already has excellent tests (`buffer.integration.test.ts`). Formalize it:

```markdown
# Spec: Buffer Time Resolution

## Objective

Determine the effective buffer time between appointments.

### Precedence (highest → lowest)

1. Service-specific buffer (`Service.bufferMinutes`) — if not null
2. Therapist default buffer (`TherapistProfile.defaultBufferMinutes`) — if profile exists and not null
3. Business default buffer (`BusinessSettings.defaultBufferMinutes`) — if configured
4. System default: 15 minutes

### Special Cases

- Explicit `0` is valid (no buffer) — MUST NOT fall through to next level
- `null` means "not configured" — MUST fall through to next level
```

The existing tests already cover this perfectly. The spec formalizes what the tests prove — making the tests *spec-derived* retroactively.

### 2.2 Slot Calculation Spec

The core availability algorithm needs a full spec:
- Input: date, therapist ID, service duration + buffer
- Sources of unavailability: operating hours, therapist schedule, blocked time, existing appointments, Google Calendar events
- Output: list of available time slots
- Edge cases: timezone boundaries, midnight-crossing appointments, back-to-back bookings

### 2.3 Spec-Derived Test Gaps

The availability engine likely has untested edge cases around:
- Timezone boundary appointments (11:30 PM booking for 90-min service)
- Google Calendar event blocking (partial overlap, all-day events)
- Concurrent booking attempts for the same slot

---

## Phase 3: API Route Contract Specs

**Goal:** Spec the 10 highest-traffic API routes with full input/output/error contracts.

### 3.1 Priority Routes

| Route | Risk | Existing Tests |
|-------|------|:----:|
| `POST /api/appointments` | CRITICAL — creates bookings | Yes |
| `POST /api/payments/create-intent` | CRITICAL — creates charges | Yes |
| `POST /api/payments/confirm` | CRITICAL — confirms charges | Yes |
| `POST /api/connect/webhook` | CRITICAL — Stripe webhooks | Yes |
| `POST /api/admin/refunds` | HIGH — processes refunds | Yes |
| `POST /api/gift-cards/redeem` | HIGH — deducts balances | Yes |
| `POST /api/customer/appointments/cancel` | HIGH — triggers fees/refunds | Needed |
| `POST /api/platform/webhook` | HIGH — subscription billing | Yes |
| `GET /api/availability/check` | MEDIUM — booking correctness | Needed |
| `POST /api/therapist/soap-notes` | MEDIUM — PHI write | Needed |

### 3.2 Spec Template for API Routes

Each API route spec should define:
1. **Input contract** (already exists as Zod schema — formalize it)
2. **Output contract** (JSON shape for success and each error code)
3. **Side effects** (DB writes, Stripe calls, emails sent, audit logs)
4. **Authorization** (which roles, what 401/403 behavior)
5. **Acceptance criteria** (testable assertions)
6. **Constraints** (security, performance, compliance)

### 3.3 Spec-Derived Output Validation

Current tests check status codes and some response fields. Specs would add:
```typescript
// Derived from create-appointment.spec.md, Output Contract
it('should return appointment with all required fields', async () => {
  const data = await response.json();
  // AC-12: Response MUST include these fields
  expect(data).toHaveProperty('success', true);
  expect(data).toHaveProperty('appointment.id');
  expect(data).toHaveProperty('appointment.status', 'PENDING');
  expect(data).toHaveProperty('appointment.totalPrice');
  expect(data).toHaveProperty('appointment.therapist.name');
  expect(data).toHaveProperty('appointment.service.name');
});
```

---

## Phase 4: PHI and Security Specs

**Goal:** Formalize the HIPAA-sensitive modules as specs for compliance auditing.

### 4.1 PHI Encryption Spec

- What fields are encrypted (SOAP notes, intake forms, specific patient data)
- What algorithm (AES-256-GCM)
- Key management (PLATFORM_DATA_ENCRYPTION_KEY)
- Who can decrypt (which roles, which routes)
- Audit logging requirements (every PHI access logged)

### 4.2 Authentication Spec

- Session lifecycle (JWT, 1-hour expiry)
- Role hierarchy (Customer, Therapist, Admin, dual-role)
- Account lockout rules (5 attempts, cooldown period)
- Password requirements (Argon2id parameters)
- 2FA flow (TOTP, backup codes)

Existing tests for `require-tier`, `password`, `totp`, `rate-limit` would be annotated with spec ACs.

---

## Phase 5: Cross-Module Flow Specs

**Goal:** Spec the end-to-end flows that span multiple modules.

### 5.1 Booking Flow Spec

The complete booking pipeline:
```
Select service → Select therapist → Check availability → Select time →
Enter customer info → Apply gift card/discount → Create appointment →
Create payment intent → Confirm payment → Send confirmation email →
Push to Google Calendar → Update availability
```

This flow touches 8+ modules and 4+ API routes. A single spec document that defines the entire chain, with acceptance criteria for each transition, would be invaluable.

### 5.2 Cancellation Flow Spec

```
Customer requests cancel → Check cancellation policy → Calculate fee →
Process refund (full or partial) → Update appointment status →
Send cancellation email → Remove from Google Calendar →
Restore availability → Credit gift card (if applicable)
```

### 5.3 Membership Billing Flow Spec

```
Customer enrolls → Create Stripe subscription → Webhook confirms →
Member pricing activates → Monthly billing cycles →
Usage tracking per session → Membership pause/cancel
```

---

## Phase 6: Spec Registry and Maintenance

**Goal:** Create `specs/SPEC_REGISTRY.md` indexing all specs, their test mappings, and dependencies.

```markdown
# JWTMS Spec Registry

## Payment Specs
| Spec | File | Tests | Status |
|------|------|-------|--------|
| Create Intent | specs/api/payments/create-intent.spec.md | route.integration.test.ts | Active |
| Payment Policy | specs/lib/payment-policy/payment-policy.spec.md | service.test.ts | Active |

## Availability Specs
| Spec | File | Tests | Status |
|------|------|-------|--------|
| Buffer Resolution | specs/lib/availability/buffer-resolution.spec.md | buffer.integration.test.ts | Active |

## Cross-Module Dependencies
- booking-flow.spec → create-intent.spec (payment step)
- booking-flow.spec → slot-calculation.spec (availability step)
- cancellation-flow.spec → payment-policy.spec (fee calculation)
```

---

## Implementation Timeline

| Phase | Scope | Effort | Immediate Value |
|-------|-------|--------|-----------------|
| **Phase 1** | 5 payment pipeline specs | 2-3 sessions | Catches financial edge cases, makes Stripe contract auditable |
| **Phase 2** | 2 availability specs | 1-2 sessions | Formalizes booking correctness, catches timezone/concurrency bugs |
| **Phase 3** | 10 API route specs | 2-3 sessions | Formalizes output contracts and error taxonomy |
| **Phase 4** | 2 PHI/security specs | 1-2 sessions | HIPAA audit trail from spec, not just code |
| **Phase 5** | 3 cross-module flow specs | 2-3 sessions | Documents end-to-end business workflows |
| **Phase 6** | Spec registry | Ongoing | Living index of all specs |

**Recommended start:** Phase 1, specifically the `create-intent.spec.md` — it's the most critical financial endpoint, already has integration tests to annotate, and the spec template above is ready to use.

---

## How JWTMS Differs from Kensa (Spec Strategy Comparison)

| Dimension | Kensa | JWTMS |
|-----------|-------|-------|
| **Primary risk** | Incorrect compliance checks | Financial loss, PHI exposure |
| **Spec entry point** | Handler contracts (check/remediation) | Payment pipeline + availability |
| **Existing schema** | JSON Schema for YAML rules | Prisma schema + Zod input schemas |
| **Test pattern** | MockSSHSession, pure functions | Real PostgreSQL + mocked Stripe |
| **Spec granularity** | Per-handler (21 + 23 handlers) | Per-API-route (135 routes, prioritized) |
| **Unique SDD need** | Behavioral spec for SSH command contracts | Cross-module flow specs (booking, cancellation) |
| **Biggest gap** | Tests don't trace to spec ACs | No output contracts or side-effect specs |

Both projects benefit from the same SDD fundamentals (Context/Objective/Constraints, spec-to-test mapping, SSOT). The difference is *where* the specs add the most value: Kensa needs handler-level behavioral specs; JWTMS needs flow-level pipeline specs for its money and PHI paths.

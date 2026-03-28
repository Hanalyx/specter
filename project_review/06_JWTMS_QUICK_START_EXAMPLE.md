# Quick Start: Your First Spec-Derived Test Cycle for JWTMS

> A concrete walkthrough applying SDD to JWTMS's payment intent creation endpoint.

---

## Why This Endpoint

`POST /api/payments/create-intent` is the gateway to every dollar flowing through the platform. It interacts with Stripe, gift cards, member pricing, and payment policy — 4 modules in one route. A spec here protects real money.

---

## Step 1: Write the Spec

Create `~/projects/jwtms/specs/api/payments/create-intent.spec.md` using the micro-spec template from the transition plan (Section 1.1). The spec defines:

- **Context:** What modules this route depends on (Stripe, pricing, payment-policy, gift-cards)
- **Objective:** What it does, with explicit acceptance criteria for every payment type
- **Constraints:** Connected account requirement, Decimal.js for math, no raw card storage

---

## Step 2: Map Existing Tests to Acceptance Criteria

Open `apps/web/src/app/api/payments/create-intent/route.integration.test.ts` and annotate:

```typescript
describe('Integration: /api/payments/create-intent', () => {
  describe('validation', () => {
    it('should return 400 for missing appointmentId', async () => {
      // AC-11: Input validation via Zod schema
    });
  });

  describe('full payment', () => {
    it('should create intent for full amount', async () => {
      // AC-1: Full upfront → PaymentIntent for full amount
    });
  });

  describe('deposit payment', () => {
    it('should create intent for deposit percentage', async () => {
      // AC-2: Deposit → PaymentIntent for deposit amount only
    });
  });

  describe('authorization', () => {
    it('should return 401 for unauthenticated request', async () => {
      // AC-11: Unauthenticated → 401
    });
  });
});
```

---

## Step 3: Identify Gaps

Compare spec acceptance criteria against existing tests:

| AC | Description | Test Exists? | Gap? |
|----|-------------|:------------:|:----:|
| AC-1 | Full upfront → full amount | Yes | — |
| AC-2 | Deposit → deposit percentage | Likely | Verify |
| AC-3 | Post-service → $0 hold | **Unknown** | **Check** |
| AC-4 | Gift card reduces amount | **Unknown** | **Likely GAP** |
| AC-5 | Gift card balance reserved | **Unknown** | **Likely GAP** |
| AC-6 | Member pricing applied | **Unknown** | **Likely GAP** |
| AC-7 | Payment record created (PENDING) | Likely | Verify |
| AC-8 | Connected account used | **Unknown** | **Likely GAP** |
| AC-9 | Idempotent (no duplicate) | **Unlikely** | **GAP** |
| AC-10 | Stripe failure → clean rollback | **Unlikely** | **GAP** |
| AC-11 | 401 for unauthenticated | Yes | — |

Conservatively, **4-6 untested financial edge cases** — each one a potential bug in production.

---

## Step 4: Write Spec-Derived Tests for the Gaps

```typescript
// specs/api/payments/create-intent.spec.md → AC-4, AC-5
describe('gift card interaction', () => {
  it('AC-4: should reduce payment amount by gift card balance', async () => {
    // Setup: Appointment for $100, gift card with $30 balance
    const giftCard = await createTestGiftCard({ balance: 3000 }); // cents
    const appointment = await createTestAppointment({ totalPrice: 10000 });

    const response = await POST(createRequest({
      appointmentId: appointment.id,
      giftCardCode: giftCard.code,
    }));

    const data = await response.json();
    expect(data.amount).toBe(7000); // $100 - $30 = $70
  });

  it('AC-5: should deduct gift card balance on intent creation', async () => {
    const giftCard = await createTestGiftCard({ balance: 3000 });
    const appointment = await createTestAppointment({ totalPrice: 10000 });

    await POST(createRequest({
      appointmentId: appointment.id,
      giftCardCode: giftCard.code,
    }));

    // Verify gift card balance was deducted
    const updated = await prisma.giftCard.findUnique({
      where: { id: giftCard.id },
    });
    expect(updated?.balance).toBe(0); // $30 fully applied
  });
});

// specs/api/payments/create-intent.spec.md → AC-10
describe('Stripe failure handling', () => {
  it('AC-10: should not create Payment record when Stripe fails', async () => {
    // Mock Stripe to throw an error
    vi.mocked(createConnectedPaymentIntent).mockRejectedValueOnce(
      new Error('Stripe API unavailable')
    );

    const appointment = await createTestAppointment({ totalPrice: 10000 });
    const response = await POST(createRequest({
      appointmentId: appointment.id,
    }));

    expect(response.status).toBe(500);

    // Verify no Payment record was created (atomicity)
    const payments = await prisma.payment.findMany({
      where: { appointmentId: appointment.id },
    });
    expect(payments).toHaveLength(0);
  });

  it('AC-10: should not deduct gift card when Stripe fails', async () => {
    vi.mocked(createConnectedPaymentIntent).mockRejectedValueOnce(
      new Error('Stripe API unavailable')
    );

    const giftCard = await createTestGiftCard({ balance: 3000 });
    const appointment = await createTestAppointment({ totalPrice: 10000 });

    await POST(createRequest({
      appointmentId: appointment.id,
      giftCardCode: giftCard.code,
    }));

    // Gift card balance should be unchanged (rollback)
    const updated = await prisma.giftCard.findUnique({
      where: { id: giftCard.id },
    });
    expect(updated?.balance).toBe(3000);
  });
});

// specs/api/payments/create-intent.spec.md → AC-9
describe('idempotency', () => {
  it('AC-9: duplicate calls should not create duplicate intents', async () => {
    const appointment = await createTestAppointment({ totalPrice: 10000 });
    const body = { appointmentId: appointment.id };

    const response1 = await POST(createRequest(body));
    const response2 = await POST(createRequest(body));

    // Second call should return existing intent, not create new one
    const data1 = await response1.json();
    const data2 = await response2.json();

    // Either: same client_secret (reuse), or 409 ALREADY_PAID
    expect(
      data1.clientSecret === data2.clientSecret ||
      response2.status === 409
    ).toBe(true);
  });
});
```

---

## Step 5: Run and Validate

```bash
# Start test database
docker compose -f docker-compose.test.yml up -d

# Run the spec-derived tests
DATABASE_URL=postgresql://test:test@localhost:5433/jwtms_test \
  npx vitest run --config vitest.integration.config.ts \
  src/app/api/payments/create-intent/route.integration.test.ts
```

If any new tests fail, you've found a **spec-code divergence** — the code doesn't match the intended behavior. This is the value of SDD: the spec-derived tests found financial edge cases that code-first testing missed.

---

## Step 6: Establish the Going-Forward Contract

From this point on, changes to the payment pipeline follow the SDD workflow:

```
BEFORE (code-first):
  feature request → write code → write tests → ship

AFTER (spec-first):
  feature request → update spec → update tests → update code → validate
```

When you or Claude need to add a new payment feature (e.g., "add tipping to payment intent"):

1. Update `create-intent.spec.md` — add AC for tip amount handling
2. Add tests derived from the new AC
3. Implement the code to pass the tests
4. The spec is the SSOT — if there's a conflict, the spec wins

---

## What You Just Accomplished

In one cycle, you:

1. Created a **behavioral specification** for the most critical financial endpoint
2. **Mapped 333 existing tests** (the relevant subset) to spec acceptance criteria
3. **Identified 4-6 untested financial edge cases** — gift card atomicity, Stripe failure rollback, idempotency
4. **Wrote spec-derived tests** that protect real money flows
5. Established the **template** for the other 134 API routes

---

## Next Steps

1. **Repeat** for `confirm-payment.spec.md` and `connect-webhook.spec.md` (complete Phase 1)
2. **Move to Phase 2** — spec the availability engine (buffer resolution + slot calculation)
3. **Move to Phase 5** — write the `booking-flow.spec.md` that ties everything together

The booking flow spec will be especially powerful because it traces the entire customer journey through 8+ modules — giving you a single document that defines the complete contract from service selection to confirmation email.

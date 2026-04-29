# SSRB-100: `spec.kind: audit-matrix` for cross-cutting specs

Status: REJECT
Decided: 2026-04-26
Source: GH #100

## 1. Request

Add a `spec.kind` discriminator to the spec schema with an initial value `audit-matrix` (default kept as the current shape). Audit-matrix specs would have rows of paths/transitions/actions, each with a `status: covered | gap | covered_needs_review` and a `spec_ref` pointing at the spec that owns the behavior. Intended for surfacing gaps across cross-cutting concerns (e.g., "does every payment completion path send a receipt?").

## 2. Origin

JWTMS had 5 specs that were explicit "coverage matrices" — each row was a path (`PATH-N`), transition (`T-N`), or action (`AUDIT-N`) with status + spec_ref. These were force-fit into `acceptance_criteria` (creating duplicate ACs) or lived outside Specter (`specs.legacy/flows/`). The cross-cutting pattern is real in regulated industries (HIPAA, SOC2, SOX, security reviews, accessibility coverage).

## 3. Universality

Verdict: UNCLEAR — the *pattern* is universal; the *proposed shape* is JWTMS-specific

Cross-cutting compliance/audit trackers exist across regulated industries. The need to enumerate concerns and track coverage across them is genuine. But JWTMS's specific row-shape (paths/transitions/actions with status + spec_ref) is one specific take on a broader pattern. Other domains might want different row shapes. Polymorphic `spec.kind` is a heavyweight commitment to support any of them.

## 4. Cost of acceptance

- The canonical schema definition: substantial — every existing required field becomes "required when kind=acceptance" or similar; new required fields appear when `kind=audit-matrix`.
- The in-memory type model: union or interface with per-kind variants.
- The JSON contract: editor extensions and CI tooling branch on `kind` for every operation.
- Reference documentation: doubles — each kind needs its own reference, examples, AI instruction templates.
- Existing user specs: no migration burden initially; all default to `kind=acceptance`.
- Editor surfaces: completion, hover, sidebar all need per-kind logic. A new spec creation flow asks "which kind?"
- Dogfooded specs: no immediate impact; could grow to use audit-matrix.

The deeper cost is forward: every future feature must answer "applies to which kinds?" — schema explosion compounds with the table of features × kinds.

## 5. Existing coverage

No direct mechanism for the cross-cutting view today. ACs are spec-internal; there's no built-in way to ask "show me every spec that touches `payment_completion` and whether the receipt action is covered."

But there are lighter mechanisms that approximate the cross-cutting view without a polymorphic schema:

- `tags` + queries (`specter ls --tag=audit-matrix-payment-receipt`) — already partially supported; needs tooling on the query side.
- Reverse linking via a future `governs:` field on AC — any AC can declare `governs: [AUDIT-3]` and Specter aggregates the cross-cutting view from existing acceptance criteria.
- External tracker file — JWTMS already does this via `specs.legacy/flows/`; a tolerable answer for trackers that don't fit AC shape.

## 6. Alternatives

- **Reverse linking via `governs:` annotation.** Any AC can declare `governs: [AUDIT-3]`. `specter ls --governs=AUDIT-3` aggregates the view from existing `acceptance_criteria`. No new spec shape, no schema bifurcation. Low-cost extension to the existing model.
- **Tags + queries.** `tags: [audit-matrix-payment-receipt]` + `specter ls --tag=audit-matrix-payment-receipt`. Already supported in concept; under-tooled on the query side.
- **External tracker file.** Cross-cutting matrices live in their own format (CSV, YAML, Markdown table) outside Specter. JWTMS already does this. Tolerable for cross-cutting trackers that don't fit AC shape and don't need machine-queryable integration.
- **Polymorphic `spec.kind`.** The proposed shape. Heaviest. Defers all the lighter mechanisms.

## 7. Decision

REJECT on solution shape, not problem validity. The cross-cutting pattern is real (§3) but polymorphic `spec.kind` is a heavyweight schema commitment (§4) when lighter mechanisms (§6) cover the use case at much lower cost. Reverse linking via `governs:` or tags + queries should land first; if those prove insufficient against a real second project's pain, revisit.

## 8. Reconsideration triggers

- A second unrelated project (regulated industry beyond JWTMS) reports the same need AND demonstrates that reverse linking / tags / external trackers cannot cover it.
- A specific cross-cutting query proves intractable without per-kind schema (e.g., type-checking the row shape).
- Reverse linking via `governs:` ships and adoption shows it's the wrong primitive.

## 9. References

- GH issue: https://github.com/Hanalyx/specter/issues/100 (closed not-planned)
- Related specs/code: `BACKLOG.md` "Unscheduled — design work needed first" (annotation-based source-file tracking, generalized provenance)

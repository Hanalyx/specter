# SSRB-098: AC-level lifecycle `status` field

Status: REJECT
Decided: 2026-04-26
Updated: 2026-04-29 (addendum §10 — broader per-AC lifecycle question)
Source: GH #98

## 1. Request

Add a `status` field to each acceptance criterion with the enum `passing | untested | pending | draft | acknowledged`. The intent is to track per-AC state along multiple dimensions: test coverage, product-stage, and human acknowledgment.

## 2. Origin

JWTMS used per-AC `status` in its legacy schema. During Specter migration, the values were stashed in `notes:` with a `JWTMS-status:` prefix — readable but not machine-queryable. Two underlying needs surfaced: (a) some statuses are about test coverage (`passing | untested`); (b) others are about product-stage state (`pending | draft | acknowledged`) — orthogonal to whether tests exist.

## 3. Universality

Verdict: SINGLE-PROJECT (for the proposed enum); UNCLEAR (for a hypothetical product-state-only variant)

The proposed enum is JWTMS's specific shape — five values mixing two distinct concepts. No other project has surfaced the same union. A narrower scope ("product-state-only") might be universal, but no second project has asked.

## 4. Cost of acceptance

- The canonical schema definition: new `status` field per AC with enum validation.
- The in-memory type model: new field on the AC struct.
- The JSON contract: editor extensions and CI tooling consume the new field.
- Reference documentation: schema reference + AI instruction templates updated.
- Existing user specs: every AC eligible for backfill; migration via `doctor --fix` non-trivial because the right value is human judgment.
- Editor surfaces: completion, hover, sidebar all want to display the field.
- Dogfooded specs: ~290 ACs across 15 specs would need backfill or accept a default.

## 5. Existing coverage

Yes — three of the proposed enum's five values map to existing mechanisms:

- `passing` / `untested` are coverage concepts: `specter coverage` (annotation count) and `coverage --strict` (mechanical via `.specter-results.json`) already answer "is this AC tested and passing." A separate field would create a second source of truth that can disagree with the toolchain — when `status: passing` but `coverage --strict` reports the AC as uncovered, the schema would need conflict-resolution rules that surprise users.
- `acknowledged` overlaps with the existing `approval_gate` (bool) and `approval_date` (ISO8601) fields, which already track human sign-off.
- `gap: true` is a degenerate one-value lifecycle ("declared but not implemented").
- `priority` is the existing risk dimension.

That leaves `pending` and `draft` as the only values without an existing home. Two values isn't an enum.

## 6. Alternatives

- **Use `notes:` prefix.** Status quo for JWTMS. Readable, not machine-queryable. Tolerable as a one-project workaround.
- **Use existing `tags`.** `tags: [pending]` is queryable via `specter ls --tag=pending` (when that lands) and doesn't require schema changes. Reasonable for project-specific lifecycle tracking.
- **Product-state-only enum (`pending | scheduled | acknowledged`)** as a fresh narrowed proposal — orthogonal to coverage. Plausibly universal but no second project has asked.
- **External tracker.** Per-AC product state lives in Jira / Linear / etc. Cross-link via `tags` or a custom field.

## 7. Decision

REJECT per §3 and §5. The five-value enum mixes coverage concerns (`passing | untested`) that the toolchain already answers mechanically with product-state concerns (`pending | draft | acknowledged`) that overlap with `approval_gate` / `tags`. Three overlapping fields disagreeing under common scenarios is a worse outcome than the current decomposition. JWTMS's migration can use `notes:` or `tags` until a second project surfaces the same pain in a narrower scope.

## 8. Reconsideration triggers

- A second unrelated project surfaces a need for *product-state-only* per-AC lifecycle (`pending | scheduled | acknowledged`) explicitly orthogonal to coverage.
- The `approval_gate` / `approval_date` shape proves insufficient for human sign-off in real workflows.
- A future Specter feature introduces per-AC operations that genuinely need a status field as input.

## 9. References

- GH issue: https://github.com/Hanalyx/specter/issues/98 (closed not-planned)
- Related specs/code: `spec-coverage` (mechanical state), `approval_gate` / `approval_date` fields, `gap: true`, `priority`

## 10. Addendum 2026-04-29 — broader per-AC lifecycle question

The original request (GH #98) was for a specific enum. A broader question came up later: *should ACs have any lifecycle dimension distinct from the spec's?*

Two camps:

**Per-spec (status quo).** The spec is the unit of decision. If parts of it are in flight while others are settled, that's a smell — split the spec, `depends_on` it, or bump the version. Long-lived specs grow in a changelog-traceable way; the spec's `status: draft | approved | deprecated` covers the lifecycle dimension.

**Per-AC.** Acknowledges that real specs stay together for cohesion but evolve at AC granularity. A spec for a billing flow might be "approved" overall, but a new "refunds for partial captures" AC is "draft" while the rest is settled.

The trade-off: per-AC granularity makes the YAML more honest about messy reality, but it also makes the YAML the place where lifecycle decisions get tracked — and YAML is a poor decision-history medium. Git history + changelog + spec-version are designed for that.

The existing per-AC fields (`description`, `inputs`, `expected_output`, `references_constraints`, `priority`, `gap`, `approval_gate`, `approval_date`) decompose lifecycle correctly:

- Coverage state — mechanical via annotations + ingest
- Approval state — explicit fields
- Risk — `priority`
- Implementation gap — `gap: true`
- Lifecycle — spec-level

Adding a per-AC lifecycle field would attract drift (anything hand-edited drifts) and would amplify the smell of long specs that should be split. The narrow case to revisit: a *strictly product-state-only* enum (e.g., `pending-review | scheduled | acknowledged-not-scheduled`) explicitly orthogonal to coverage. Until a second project surfaces that scope, the existing decomposition holds.

Decision unchanged. This addendum captures the broader reasoning for future requesters who ask the underlying lifecycle question rather than #98's specific enum.

# SSRB-106: settle the three inert tier mechanisms

Status: ACCEPT
Directed: 2026-08-22, founder, carrying `bugs/SP-SP-049`, `system.tier`, and `bugs/SP-SP-001` as one decision.
Source: the lexicon review of 2026-08-20 and the phase 4 review of v0.15 item 1D-a.

## 1. Request

The manifest carries three mechanisms that appear to set or change a spec's
tier. **None of them does anything.** Settle all three under one principle:

> **The spec declares its tier. The manifest may state a policy and be checked
> against it. The manifest may not silently change it.**

That principle resolves each of the three differently, which is why they belong
in one brief rather than three.

| Mechanism | Today | Decision |
|---|---|---|
| `domains.<name>.tier` | validated, inert, silent | **Becomes live as a checked assertion** |
| `system.tier` | validated, inert, silent | **Deprecated, removed at v1.0.0** |
| `settings.tier_overrides` | validated, inert, and the warning says it applied | **Deprecated, removed at v1.0.0. Message corrected now** |

`spec.tier` stays required and authoritative. No change to the spec schema.

## 2. Origin

Three findings, two of them from independent reviews.

**`domains.<name>.tier` and `system.tier` do nothing.** `ResolveTier` implements
a four-step inheritance cascade and nothing calls it. Both its call sites sit in
functions with no callers, and after `docs/ssrb/SSRB-105.md` retires the
registry it will have zero call sites of any kind. Every live consumer reads
`spec.Tier` raw at seven sites. Measured: a manifest declaring both
`system.tier: 1` and a domain at `tier: 1` listing a spec that declares
`tier: 3` still reports T3 everywhere. Nothing warns. `bugs/SP-SP-049`.

**`settings.tier_overrides` does nothing, and says it did.** Measured at
`5cf5b59`:

```
warn [tier_conflict] spec "lo" declares tier: 3 but specter.yaml tier_overrides assigns tier: 1 — using override (1)
info [orphan_constraint] lo C-01: ...
coverage: tier=3 threshold=50
```

The override is announced as applied. The orphan prints at `info`, which is
Tier 3 severity, and coverage reports T3. `bugs/SP-SP-001`.

**Both are advertised.** `specter init` writes `system.tier` and
`domains.default.tier` into every new workspace, and
`docs/GETTING_STARTED.md:158-164` shows them. A team following the
getting-started guide configures two settings that do nothing.

## 3. Universality

**Does the pain generalize?** Yes, and it is worst where the tool is most
useful. A single-spec project does not notice. A 249-spec workspace like jwtms
organizes specs into domains, sets a domain's risk level, and gets silence.

**Does the fix generalize?** The checked-assertion shape does. Any project with
domains benefits from knowing when a spec's declared tier contradicts the risk
level its domain asserts, and that is drift detection rather than a preference.

**The removals invert the question: does anyone depend on them?** They cannot.
Both are inert, so no behavior rests on either. The only dependency possible is
an operator's belief, and that belief is currently false.

## 4. Cost

Enumerated before scoping, per the schema-conservatism rule.

**Manifest schema.** `system.tier` and `settings.tier_overrides` become
accepted-with-warning in v0.15 and are removed at v1.0.0.
`domains.<name>.tier` keeps its shape and gains meaning.

**Spec schema.** None. `spec.tier` is unchanged, which is most of the point.

**Code.** A new domain-tier disagreement check. Deletion of `ResolveTier` and
`ResolveTierWithOverrides`. A one-line correction to the `tier_conflict`
message. `refresh.go:99`, which copies `System.Tier` into a synthesized default
domain, and `scaffold.go:33,65`, where `init` writes both inert fields.

**Spec.** `spec-manifest` C-14 covers `tier_conflict` and needs revising. A new
constraint and criteria for the domain check, and deprecation constraints for
the two removals.

**Docs.** `docs/GETTING_STARTED.md`, `docs/SPEC_SCHEMA_REFERENCE.md`, and
`docs/CLI_REFERENCE.md`, whose manifest-settings section omits `tier_overrides`
entirely, which is why an operator could not check the intended behavior against
a reference.

**Not affected.** `--scope`, which reads `domain.Specs` and never `domain.Tier`.
Coverage thresholds, orphan severity, and the Tier 1 evidence rule, which all
read `spec.Tier` and keep doing so.

## 5. Existing coverage

`spec-manifest` C-14 and its criteria cover `tier_conflict` as it exists: a
comparison against `tier_overrides` that reports and changes nothing. That
constraint is satisfied by the current code, which is why the defect has never
failed a gate.

No constraint covers domain-tier disagreement, because the cascade that would
have produced it was never reachable.

`bugs/SP-SP-002` records that `tier_conflict` never reaches `check --json`, so
JSON consumers including the VS Code extension have never seen it. The new
diagnostic must not repeat that, and fixing it for the existing one is in scope.

## 6. Alternatives

**Make the cascade live: relax `spec.tier` to optional so a spec inherits from
its domain.** This was the earlier recommendation in `SP-SP-049` and is
**rejected on measurement**. Relaxing the schema does not produce inheritance,
it produces three silent fallbacks: `CoverageThresholds()` populates keys 1, 2
and 3 only, so a tier-0 spec falls back to threshold **80**;
`orphanSeverityByTier[0]` returns the empty string and the guard turns it into
**warning**; and `BuildSummaryHeader` loops `for tier := 1; tier <= 3`, so the
spec **vanishes from the summary rollup**. It also makes
`tier_conflict.go:37`'s `spec.Tier == 0` branch reachable for the first time,
with a meaning chosen for a different feature.

The decisive argument is not cost. **The assertion is the safety half of
inheritance, and inheritance without it reproduces this bug one level up**:
`spec.tier: 3` inside `domains.payments.tier: 1` would resolve silently. So the
check must be built either way. Build it first, alone, and relax the schema
later if adopters ask, with the drift detection already in place.

What that gives up is the say-it-once ergonomic. A large workspace still writes
`tier` on every spec. That cost is not new, since `tier` is required today, and
it is recoverable later.

**Wire `tier_overrides` up so it does what it says.** Rejected. It contradicts
the principle in section 1 and the project's own SSOT rule. The spec is
authoritative; a manifest key that silently changes a spec's declared tier is
precisely the drift a type system for specs exists to detect.

**Leave any of the three inert.** Rejected. All three fail the v0.18 pre-lock
criterion that every schema field be consumed deterministically by at least one
command.

**Decide them separately.** Rejected, and this is why the brief exists. They are
three answers to one question, and settling one without the others risks a
manifest where a domain tier is checked, a system tier is ignored, and an
override still lies.

## 7. Decision

**ACCEPT.** Delivered in the SSRB-105 shape: behavior and code now, key removal
at v1.0.0.

### 7.1 `domains.<name>.tier` becomes a checked assertion

A domain's `tier` declares the risk level the domain asserts. A spec listed in
that domain whose declared `tier` disagrees produces a diagnostic naming the
spec, its declared tier, the domain, and the domain's tier.

The domain tier **does not change** the spec's effective tier. Nothing resolves.
`spec.Tier` remains what every consumer reads.

**Severity: warning.** It matches the `tier_conflict` precedent, and a
disagreement is not always an error: a mostly-Tier-1 domain may legitimately
hold one Tier 3 spec. `--strict` promotes it, which is the existing contract for
warnings and gives a project that wants it enforced a way to say so without a
new field.

**It must appear in `check --json`.** `bugs/SP-SP-002` records that
`tier_conflict` does not, because tier conflicts are computed after the JSON
branch returns. Fix that for both diagnostics in the same change, or the new one
ships with the defect already filed against the old one.

### 7.2 `system.tier` is deprecated

Accepted with a warning in v0.15, removed at v1.0.0. With domains carrying a
checked tier, a workspace-wide default adds a level and decides nothing.

`init` stops writing it. `refresh.go:99` stops copying it into a synthesized
domain.

### 7.3 `settings.tier_overrides` is deprecated, and its message is corrected now

Accepted with a warning in v0.15, removed at v1.0.0.

**The `using override (%d)` clause is corrected in v0.15, independently of the
removal.** `internal/manifest/tier_conflict.go:45` asserts something false on
every run today. That correction needs no schema change and should not wait for
the deprecation window to close.

This is not merely cosmetic. Correcting the public documentation on 2026-08-09
nearly shipped a second false claim, because the only evidence for the reading
that the override wins was this message. **A program that misdescribes itself
defeats the reviewer who checks the program.**

### 7.4 `ResolveTier` and `ResolveTierWithOverrides` are deleted

After SSRB-105 retires the registry, both have zero call sites. Neither is
needed: 7.1 compares, it does not resolve.

### 7.5 What ships in v0.15 versus v1.0.0

**v0.15.0**: the domain-tier check, the `tier_conflict` message correction, the
`check --json` fix, the two deprecation warnings, the two function deletions,
`init` and `refresh` corrections, and the doc updates.

**v1.0.0**: `system.tier` and `settings.tier_overrides` leave the schema,
alongside `settings.strictness` per SSRB-104 and `registry` per SSRB-105. One
migration for four removals.

## 8. Reconsideration triggers

- **Multiple unrelated adopters asking to omit `tier` and inherit it.** That is
  the say-it-once case section 6 defers, and the check from 7.1 is its
  prerequisite, so it would be an extension rather than a reversal.
- An adopter reporting they rely on `tier_overrides` in a way the current inert
  behavior somehow serves, which would mean the setting has a use nobody has
  described.
- Evidence that a warning is the wrong severity for 7.1, meaning a corpus where
  legitimate spec-versus-domain divergence is common enough that the diagnostic
  is noise.

## 9. References

- `bugs/SP-SP-049`, the dead cascade, carrying the Option A reassessment this
  brief adopts.
- `bugs/SP-SP-001`, `tier_overrides` parsed and never applied.
- `bugs/SP-SP-002`, `tier_conflict` absent from `check --json`. In scope for 7.1.
- `docs/ssrb/SSRB-105.md`, retiring the registry. Deletes one of the two
  `ResolveTier` call sites and shares the deprecate-then-remove shape.
- `docs/ssrb/SSRB-104.md`, whose v1.0.0 removal this joins.
- `docs/SPECTER_LEXICON.md`, `tier`, `tier override` and `tier conflict` entries.

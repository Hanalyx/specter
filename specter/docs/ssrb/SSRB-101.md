# SSRB-101: Source-file governance — annotation (F7) vs `governs:` list (F8)

Status: NEEDS-DESIGN
Decided: TBD (target: end of v0.16 cycle, before v0.17 opens)
Source: BACKLOG "Unscheduled — design work needed first" (annotation-based source-file tracking; generalize `generated_from` to `provenance` with `governs:` list)

## 1. Request

Two competing proposals answer the same question — *"how does Specter
know which source files are governed by a spec?"*:

- **F7 — Annotation-based.** Extend `// @spec <id>` annotations from
  test files to source files. New `specter specs governing <path>`
  command. Coverage output carries a derived `source_files: [...]`
  array per spec. Opt-in via `specter.yaml` setting. No `.spec.yaml`
  schema change.

- **F8 — Declarative `governs:` list.** Generalize `generated_from`
  to `provenance` with a `governs: [string]` field listing governed
  source files per spec. Source files have no annotations. New
  schema field.

The two are mutually exclusive defaults — projects shouldn't carry
both mechanisms answering the same question.

## 2. Origin

JWTMS migration surfaced the underlying need: a spec governs concrete
implementation files, and Specter today has no first-party way to
record that mapping (only `generated_from.test_files` for tests).
Reverse compiler produces drafts where the source-file relationship is
implicit.

The pain is real (one project asked); the proposed shape is two
distinct designs that need evaluation, not a single committed change.

## 3. Universality

**UNCLEAR** at evaluation start (2026-05-07). One project (JWTMS) has
asked. The evaluation window (v0.13–v0.16, ≈ 4–6 months) is intended
to surface or fail to surface a second unrelated request.

A "yes, ship one of them" decision requires evidence from at least
2 independent projects per the project's universality bar.

## 4. Cost of acceptance

### F7 (annotation-based)

| Surface | Impact |
|---|---|
| Canonical schema | None |
| In-memory type model | New derived field on coverage report |
| JSON contract | New `source_files: [...]` array in coverage output (v1.0 contract surface) |
| Reference docs | New annotation pattern documented; mirrors test annotation |
| User specs (migration) | None |
| Editor surfaces | New hover/jump-to-spec for source files |
| Dogfood | Specter must annotate its own `internal/**/*.go` |
| Performance | `coverage` scans every source file matching `tests_glob`-equivalent setting; large repos pay per-run cost |
| Drift modes | Net-new: spec-id in annotation that doesn't exist; spec renamed without annotation update |

### F8 (`governs:` list)

| Surface | Impact |
|---|---|
| Canonical schema | New field on every spec (or under existing `generated_from` / new `provenance` object) |
| In-memory type model | New required-or-optional field |
| JSON contract | Schema bump implication; `parse --json` carries new field |
| Reference docs | New schema field documented in `SPEC_SCHEMA_REFERENCE.md` |
| User specs (migration) | Pre-existing specs need to add `governs:` lists if they want governance enforced (or migration tool fills it) |
| Editor surfaces | Hover on `governs:` shows spec → file mapping |
| Dogfood | Specter must list source files in 15+ specs; maintenance burden |
| Performance | No scan; declared list is consumed directly |
| Drift modes | High: list goes stale when files move/rename/delete; new failure class for migrations |

F8 carries a schema commitment pre-1.0; F7 doesn't. F7 has performance
cost on large repos; F8 has drift cost on every repo.

## 5. Existing coverage

Today neither mechanism exists. `generated_from` (singular
`source_file`, optional) records *how* a spec was created (often via
`specter reverse`), not *what it governs*. SSRB-097 already rejected
the "make `generated_from.source_files` plural" path on
universality + symmetry-vs-friction grounds.

## 6. Alternatives

### Hybrid

Manifest-level `governs:` mapping (in `specter.yaml`, not in each
`.spec.yaml`). Centralizes the mapping; no schema change to specs;
no source annotations. Combines F7's no-schema-change with F8's
declarative shape.

Trade-off: manifest becomes the maintenance hot-spot; loses F7's
"truth lives next to code" property.

### Reverse linking via `depends_on`

Existing `depends_on` could carry source-file references if
generalized. Trade-off: muddles spec→spec dependency with spec→file
governance; SSRB-097 effectively rejected this direction.

### File-naming convention only

`payment-charge.spec.yaml` and `internal/payment/charge.go` are
related by name; no explicit mapping. Trade-off: works for greenfield
projects, fails for legacy, requires specific repo layout.

### Documented adapter pattern (no schema, no command)

Project-specific consumers wire their own governance check via the
existing `parse --json` output and project conventions. Trade-off:
no first-party support; each adopter rebuilds the same wheel.

## 7. Decision

NEEDS-DESIGN. Open during v0.13–v0.16. Decision criteria:

- If 0 additional projects request governance during the evaluation
  window AND the existing convention/adapter alternatives cover the
  use case → close not-planned (both F7 and F8 rejected).
- If 1+ additional projects request it AND the request can be served
  by F7's no-schema path → ACCEPT F7, ship in v0.17 or v0.18.
- If 1+ additional projects request it AND F7 alone fails (e.g.,
  generated/vendored code that can't be annotated) → consider hybrid
  (manifest-level `governs:`) before F8's per-spec schema field.
- F8's per-spec `governs:` field bar is highest: requires evidence
  that neither F7 nor a hybrid suffices.

## 8. Reconsideration triggers

- A second unrelated project (post-JWTMS) reports source-file
  governance pain that maps to either proposal.
- v0.13's `unreachable_annotation` (F3) ships and reveals patterns
  that inform either design.
- Performance benchmarks on a large repo (50K+ source files) tip the
  F7-vs-F8 trade-off.
- A v0.17/v0.18 contributor proposes the hybrid manifest-level
  variant with sufficient design.

## 9. References

- BACKLOG: "Unscheduled — design work needed first" (annotation-based
  source-file tracking; generalize `generated_from` to `provenance`
  with `governs:` list)
- Related SSRBs: SSRB-097 (`generated_from.source_files` plural —
  rejected), SSRB-100 (`spec.kind: audit-matrix` — rejected; touches
  governance edge)
- Related specs/code: `internal/parser/spec-schema.json`
  (`generated_from` shape), `internal/coverage/` (would gain
  `source_files` derived array under F7)

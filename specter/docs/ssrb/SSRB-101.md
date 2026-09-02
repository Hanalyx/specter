# SSRB-101: Source-file governance, annotation (F7) versus `governs:` list (F8)

Status: REJECT (closed not-planned)
Decided: 2026-08-16, ahead of the target window
Source: BACKLOG "Unscheduled, design work needed first" (annotation-based source-file tracking; generalize `generated_from` to `provenance` with `governs:` list)

## 1. Request

Two competing proposals answer the same question, *"how does Specter
know which source files are governed by a spec?"*:

- **F7, annotation-based.** Extend `// @spec <id>` annotations from
  test files to source files. New `specter specs governing <path>`
  command. Coverage output carries a derived `source_files: [...]`
  array per spec. Opt-in via `specter.yaml` setting. No `.spec.yaml`
  schema change.

- **F8, declarative `governs:` list.** Generalize `generated_from`
  to `provenance` with a `governs: [string]` field listing governed
  source files per spec. Source files have no annotations. New
  schema field.

The two are mutually exclusive defaults. Projects should not carry
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
asked. The evaluation window (v0.13 to v0.16, roughly 4 to 6 months) is intended
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
| Editor surfaces | Hover on `governs:` shows spec-to-file mapping |
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
generalized. Trade-off: muddles spec-to-spec dependency with spec-to-file
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

- BACKLOG: "Unscheduled, design work needed first" (annotation-based
  source-file tracking; generalize `generated_from` to `provenance`
  with `governs:` list)
- Related SSRBs: SSRB-097 (`generated_from.source_files` plural,
  rejected), SSRB-100 (`spec.kind: audit-matrix`, rejected; touches
  governance edge)
- Related specs/code: `internal/parser/spec-schema.json`
  (`generated_from` shape), `internal/coverage/` (would gain
  `source_files` derived array under F7)


## 10. Closure, 2026-08-16

**Both F7 and F8 are rejected. Closed not-planned, before the evaluation
window ended.**

Be precise about why, because the obvious reading is wrong. This did not close
because the clock ran out. The window ran v0.13 through v0.16 and the project
is at v0.14.1, so it is closing early, on the strength of a finding rather than
on the absence of requests.

### The finding

The orchestration flow assessment of 2026-08-16 examined a workflow that
appeared to need exactly this. Its stub-ban gate is described as a static scan
of every function tagged to an acceptance criterion, which presumes a
spec-to-source mapping. It looked like the second requester this brief was
waiting for.

It is not, and the reason generalizes past that one workflow. **An orchestrator
that freezes the tree between phases already knows the governed file set
exactly**, from a diff against the frozen reference. That is real provenance,
recorded by the act of freezing. Both candidates here are approximations of
provenance that a caller in that position already holds, and both carry drift
the diff does not: an annotation can name a spec that no longer exists or be
left behind by a rename, and a declared list goes stale on every file move.

So the most plausible future requester for source-file governance turns out to
be better served without it. That is a stronger reason to close than silence
would have been, and it does not depend on waiting.

### Section 3 supports the same conclusion, separately

Universality was recorded as UNCLEAR with one asking project. It stayed at one.
The workflow above does not raise it, on two independent grounds: it originates
from the project itself rather than an adopter, and section 8's trigger asks
specifically for a second **unrelated** project. Counting an internal brief
would be counting one vote twice, which is the failure the universality bar
exists to prevent.

A separate adopter data point cuts the same way. OpenWatch considered
annotation-based governance for their own use and declined it, using Specter's
own reasoning: an annotation on an implementation function has no runner-visible
counterpart, so it could only ever be an unverifiable claim. That is a
considered decline, and it should not be logged as demand.

### What this does not decide

The problem is real and remains unsolved. Nothing here says a project never
needs to know which files a spec governs. It says neither candidate in this
brief is the way to learn it, and that a caller who freezes a tree does not need
to.

### Reopen triggers, replacing section 8

- A second project, unrelated to the original requester and not the Specter
  project itself, reports source-file governance pain and states a workflow it
  blocks.
- A need for **function-level** rather than file-level granularity, meaning
  which function implements a named criterion rather than which files a phase
  touched. The frozen-reference diff serves the second and cannot serve the
  first. Nothing in the current orchestration work needs it.
- A workflow that cannot freeze, meaning phases running in a shared tree with no
  version-control reference, which removes the provenance this closure relies on.

### Downstream effects

- v0.17's headline is no longer conditional on this brief, and becomes the
  Python plugin under the 2026-08-16 roadmap reconciliation.
- The F7/F8 fallback slot in v0.18 empties.
- Two backlog entries are superseded: annotation-based source-file tracking, and
  generalizing `generated_from` to `provenance` with a `governs:` list. Both are
  marked in the 2026-08-16 grooming pass.

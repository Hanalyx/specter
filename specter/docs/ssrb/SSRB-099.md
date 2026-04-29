# SSRB-099: Coverage inference from `generated_from.test_files`

Status: REJECT
Decided: 2026-04-26
Source: GH #99

## 1. Request

When `specter coverage` runs and finds no `// @ac` annotations for a given AC, fall back to checking `generated_from.test_files`. If the listed test files exist and contain code matching the AC, count the AC as covered. Useful for newly-migrated specs that haven't been annotation-backfilled.

## 2. Origin

JWTMS migration. ~2,000 ACs whose tests existed and passed showed as "uncovered" until a separate annotation-backfill pass landed. Migrated specs declared `generated_from.test_files` listing exactly which tests covered them — that metadata existed but coverage ignored it.

## 3. Universality

Verdict: SINGLE-PROJECT

The reported friction surfaces only when migrating an existing codebase to Specter — newly-imported specs report 0% coverage until annotations are backfilled. Greenfield Specter projects write annotations from day one and never hit this state. The asymmetry isn't a Specter design gap; it's a one-time import affordance.

## 4. Cost of acceptance

- The canonical schema definition: minimal — no new fields.
- The in-memory type model: minimal.
- The JSON contract: coverage output gains an "inferred" flag per AC; consumers update.
- Reference documentation: explain when inference fires vs. annotation matching.
- Existing user specs: minimal direct impact, but the change of mental model is project-wide.
- Editor surfaces: coverage gutter would need to distinguish inferred from annotated.
- Dogfooded specs: minimal.

The bigger cost is conceptual, not surface-by-surface: it bifurcates the trust model. "Which kind of coverage am I looking at?" becomes a per-invocation question.

## 5. Existing coverage

Yes — annotation-based coverage is the canonical mechanism, and `coverage --strict` (v0.10) is the mechanical signal that closes three failure modes the soft signal has:

- A test with `it.skip(...)` + the annotation reads as "covered" — skipped tests claim coverage.
- A test that now fails but still has the annotation reads as "covered" — regressions slip through.
- A test referenced in `generated_from.test_files` that no longer exists, no longer runs, or no longer covers the AC reads as "covered" — drift slips through.

The v0.10 design call deliberately replaced filename-matching soft signals with annotations + `.specter-results.json`. Restoring filename inference as a fallback restores the failure modes v0.10 closed.

## 6. Alternatives

- **Migration tool (GH #96) backfills annotations on import.** When `specter migrate --from=<dialect>` translates a non-Specter dialect into Specter shape, it can also walk the listed `test_files` and insert `// @spec`/`// @ac` annotations in those test files as part of the import. After migration, ongoing coverage is annotation-based as designed; the migration tool absorbs the one-time cost.
- **One-shot annotation-backfill script.** Project-side tool that walks `generated_from.test_files` and inserts annotations. JWTMS already wrote one for its migration; the same approach generalizes.
- **Document the migration friction.** "Migrated specs report 0% coverage until annotations land" is a known transition state, not a Specter bug.

## 7. Decision

REJECT per §3 and §5. The pain is migration-only (greenfield projects don't hit it), and adding a coverage-inference fallback contradicts the v0.10 mechanical-coverage design call that deliberately replaced filename matching with annotations + `.specter-results.json`. The right place to address the import affordance is the migration tool itself (GH #96) — backfill annotations at import time, not soft-infer at coverage time.

## 8. Reconsideration triggers

- A non-migration scenario surfaces where `generated_from.test_files` is the ground truth and annotations cannot be added (e.g., generated tests in a read-only directory).
- The v0.10 mechanical-coverage model proves insufficient and a soft fallback re-emerges as a real need.
- GH #96 lands and explicitly does not backfill annotations, leaving the migration friction unsolved.

## 9. References

- GH issue: https://github.com/Hanalyx/specter/issues/99 (closed not-planned)
- Related SSRBs: SSRB-096 (when GH #96 lands)
- Related specs/code: `spec-coverage` v1.9.0+ (mechanical strict mode), `BACKLOG.md` v0.10 design call

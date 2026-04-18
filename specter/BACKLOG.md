# Specter Backlog

Forward-looking roadmap. Items are grouped by target release. Each item is a single sentence of intent plus a link to the design doc or discussion when one exists.

Current shipped version: **v0.6.9** (see [CHANGELOG](CHANGELOG.md) when created, or `git tag`).

---

## v0.7.0 — Schema hardening (in progress on `release/v0.7.0`)

Design rationale captured in local research notes (not shipped with the repo). See `CHANGELOG.md` for the public-facing summary.

- Tighten `context.additionalProperties` to `false` (breaking) — close the silent-data-loss gap between schema and `types.go`.
- Add `notes`, `approval_gate`, `approval_date` to `acceptance_criterion` — narrative and audit metadata per AC.
- Move `references_constraints` cross-reference validation to parse time — catch dangling refs before any downstream stage runs.
- Add optional `title` to `spec` — human-readable display name, defaults to `id` when absent.
- Internal Go enum validation helpers — protect internal code paths (reverse compiler, migration scripts) that skip JSON Schema.
- VS Code extension updates to render `title`, show `notes` in AC hover, surface `approval_gate` as a gutter indicator.

---

## v0.8.0 — Annotation-based source-file tracking (proposed)

Motivation: provide the "which specs govern this file?" use case from jwtms without introducing a `spec.source_files` schema field (which duplicates state and invites drift).

- Extend `@spec` annotation support from test files to source files. The existing extractor already scans for `@spec <id>` comments — broaden the scan from `tests_dir` to the full project (excluding build output, node_modules, etc.).
- New CLI command: `specter specs governing <path>` — reverse lookup given a file, returns specs that annotate it.
- Extend `specter coverage --json` output with a derived `source_files` array per spec, populated from annotations (not from a schema field).
- VS Code extension: gutter icon on source files with `@spec` annotations; hover shows spec summary and coverage status.
- Opt-in via `specter.yaml` setting (`scan_sources: true`) so teams that dislike metadata comments in production code aren't forced.
- Migration script for jwtms-style projects that already have `spec.source_files` lists: read the lists, insert `@spec` annotations into each target file, leave the schema untouched.

**Why annotations, not a schema field:** single source of truth (the annotation is IN the file, so rename/delete keeps them aligned), matches Specter's existing test-coverage model, zero drift class, no schema surface growth.

---

## v0.8+ / unscheduled — deferred from v0.7.0 proposal

Each needs its own design doc before scheduling:

- Generalize `generated_from` to `provenance` with a `governs: [string]` list — overlaps semantically with `depends_on`, needs careful design to avoid muddling "spec depends on spec" with "spec governs file." May be obsoleted by the v0.8 annotation-based approach.
- Optional `contracts` section for HTTP APIs — Specter's mission is framework-agnostic; HTTP specialization is a commitment. Better as an adapter/extension than core schema.
- Derived `callers` via `specter graph --callers-of <spec-id>` — no schema change; derivable from the existing `depends_on` graph. Low-cost feature.
- Per-rule narrowing of `constraint_validation.value` — constrain value type based on `rule` (e.g., `rule: "min"` implies numeric value). Field is write-only today; defer until someone consumes it.

---

## Open adoption-friction items (from v0.6.x review)

Not schema-scoped; move to a specific release when picked up:

- Zero-state and bare-command UX — `specter` with no args shows help; "no specs found" messages explain what was searched and suggest `init` / `reverse`.
- Parse-error hint map — common pattern violations include an example of the correct form.
- Reverse compiler handoff — success output points users at `specter explain <spec-id>` for gap triage.
- Docs consolidation — merge QUICKSTART into README, keep GETTING_STARTED as deep-dive, archive stale RELEASE_PLAN.

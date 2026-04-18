# Changelog

All notable changes to Specter (CLI + VS Code extension) documented here. The project is pre-1.0; breaking changes go in MINOR releases per semver conventions for 0.x.

---

## v0.7.0 — 2026-04-17 (unreleased, on `release/v0.7.0`)

### Breaking

- **`context.additionalProperties` tightened to `false`.** Unknown keys under `context` now fail `specter parse` with a named error. Previously they were silently dropped because the schema said "extras are allowed" but `SpecContext` (types.go) was a closed struct. This was the only silent-data-loss site in the schema. Users with `context.role`, `context.callers`, or similar custom keys must either rename to an existing field (e.g. move narrative into `context.description`) or open an issue to propose a new schema field.

- **`references_constraints` cross-reference validation moved to parse time.** An AC that references a constraint not declared in the same spec (e.g. `references_constraints: ["C-99"]` when only C-01 exists) now fails `specter parse` with a `dangling_reference` error. Previously this was caught later by `specter check` as a warning. No impact on specs with clean references.

### Added

- `acceptance_criterion.notes` — optional free-form narrative per AC. Complements the top-level `changelog` (which is version-over-version) with lifetime-of-the-AC annotation.
- `acceptance_criterion.approval_gate` (bool) and `approval_criterion.approval_date` (date) — optional audit metadata for regulated work. Specter does not enforce approval semantics; teams wire this into their own CI/PR gates.
- `spec.title` — optional human-readable display name. VS Code extension, tree views, `specter explain`, and PR renderings use this when present, falling back to `id`.
- Parse-time format validation for `date`-typed fields (`approval_date`, `changelog.date`, `generated_from.extraction_date`). Previously draft 2020-12's default was annotation-only; invalid dates slipped through.
- Internal `schema.ValidateEnums()` method and exported enum constants (`StatusApproved`, `EnforcementError`, etc.) for Go code that constructs specs without going through `ParseSpec` (reverse compiler, migration scripts).

### Changed

- VS Code extension now renders `spec.title` in the coverage tree view and `specter explain` output; falls back to `id` when absent.
- VS Code AC hover shows `notes` when present.
- `approval_gate: true` ACs get a subtle gutter indicator in the VS Code extension.

### Documentation

- `SPEC_SCHEMA_REFERENCE.md` — context extension escape hatch removed from docs; replaced with "propose a new schema field."
- `GOTCHAS.md` #14 added: documents the silent-context-drop trap and its v0.7.0 fix.
- `BACKLOG.md` introduced with v0.8.0 annotation-based source-file tracking and deferred items.

### Migration notes

- Specter's own dogfood: no changes needed. All 14 specs conform to the strict shape.
- External projects: run `specter parse` with v0.7.0 on your spec corpus. Any `context.*` unknown keys or dangling `references_constraints` will now surface as errors — fix them or propose new fields.
- CI consumers: pin `specter@v0.6.9` if you can't adopt v0.7.0 yet; otherwise update pin and fix surfaced errors.

---

## v0.6.9 — 2026-04-17

- VS Code: on activation, offer existing users the new **Specter: Add CLI to Shell PATH** command when the detected shell's rc file doesn't reference `~/.specter/bin` (dismissable with persistent "Don't show again").
- Docs: fixed broken install URLs across README, QUICKSTART, CLI_REFERENCE, GETTING_STARTED. Previously all used `uname`-based patterns that don't match goreleaser's lowercase `linux`/`amd64` naming — users got 9-byte "Not Found" files instead of binaries.
- QUICKSTART: fixed misplaced `gap: true` example (was at spec level; schema only allows on ACs) and wrong coverage example (T2 33% was shown as PASS; threshold is 80%).
- GOTCHAS #13 added: four-vocabulary arch/OS translation trap (uname / Node / VS Code runner / Go GOARCH).

## v0.6.8 — 2026-04-17

- VS Code: new **Specter: Add CLI to Shell PATH** command. Detects shell, appends idempotent export to the right rc file (`.bashrc`/`.bash_profile`/`.zshrc`/`config.fish`). Unknown shells get a clipboard fallback. 13 new unit tests.
- Extension README refreshed — commands table had been missing 5 commands.

## v0.6.7 — 2026-04-17

- VS Code: fixed arch mismatch that caused 404 on auto-download (`specter_0.6.6_linux_x64.tar.gz` → not found). `normaliseArch` now lowercases its input so `process.arch: "x64"` maps correctly to `amd64`.
- GOTCHAS #13 added.

## v0.6.6 — 2026-04-17

- VS Code: fixed release pipeline — `vsce package` was shipping stale `out/*.js` because it doesn't run the build. Added `vscode:prepublish` hook so builds always run before packaging.
- GOTCHAS.md introduced with 13 entries documenting traps hit during v0.6.x.

## v0.6.5 — 2026-04-17

- **Breaking**: `constraint.enforcement` now overrides tier-based severity in `specter check` diagnostics (previously parsed but unused).
- **Breaking**: `gap: true` ACs count as uncovered for threshold purposes. Previously a 100%-gap spec auto-passed threshold; this hid real coverage gaps.
- `trust_level` field removed from schema (was parsed but never enforced by any pipeline stage).
- `constraint.type` surfaces inline in `specter check` output: `warn [orphan_constraint] spec-auth C-04 (security): ...`.
- VS Code: validates resolved CLI binary regardless of source (cache/PATH/workspace-setting). Previously the validation was gated on `source === 'cache'`, so a corrupt binary on PATH slipped through. Output channel for errors, `Specter: Re-download CLI` recovery command, 30s timeout on downloads.

## Earlier versions

See git tags for v0.6.0–v0.6.4 and v0.3.0–v0.5.2. Tags: `git tag -l | sort -V`.

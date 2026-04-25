# v0.11.0 plan

Working document for the v0.11 release cycle. Delete after `release/v0.11` merges to `main` and the tag ships.

## Scope

Five features, ordered by dependency and blast radius:

1. **`specter explain` bundle** — `explain annotation`, `explain schema`, `explain <spec-id>` AC-less spec card. Terminal output only. No file writes.
2. **`specter check --test` / `-t`** — test-annotation cross-reference (`unknown_spec_ref`, `unknown_ac_ref`, `malformed_ac_id`).
3. **`specter init --install-hook`** — writes git pre-push hook blocking pushes lacking spec/ac annotations.
4. **`specter init --ai <tool>`** — writes AI instruction file for claude / cursor / copilot / codex / gemini. Design locked 2026-04-25.
5. **`settings.strictness`** — three-level strictness (`annotation` / `threshold` / `zero-tolerance`) in `specter.yaml`. Replaces BUG-3 part 2.

Deferred to v0.12: `--with-hooks` (Claude PreToolUse/PostToolUse compact templates), `unreachable_annotation` diagnostic. See BACKLOG `v0.12 — AI loop hard enforcement`.

## Ordering

Features 1 and 2 have no dependencies — run in parallel first. Feature 4 references `specter explain annotation` / `explain schema` in its instruction-file template, so Feature 1 must land before Feature 4. Features 3 and 4 both bump `spec-manifest` — bundle them into one spec commit to avoid two version bumps on the same spec in one cycle. Feature 5 has the largest design surface and resolves Agent 2's strictness exit-code finding; ship it last so earlier features validate the current `--strict` semantics before they change.

| Wave | Features | Gating |
|---|---|---|
| A | 1 (explain bundle), 2 (check --test) | independent — parallel PRs |
| B | 3 (init --install-hook) + 4 (init --ai) | bundled; depends on 1 landing |
| C | 5 (settings.strictness) | depends on B landing — last in cycle |

Release bump (`VERSION`, `package.json`, `CHANGELOG.md`) ships in a separate minimal `bump/v0.11.0` PR to `main` after `release/v0.11` merges, per the v0.10.1 / v0.10.2 pattern.

## Current spec state (baseline)

| Spec | Version | Cs | ACs |
|---|---|---|---|
| spec-explain | 1.0.0 | 8 | 6 |
| spec-check | 1.1.0 | 8 | 8 |
| spec-manifest (governs `init`) | 1.6.0 | 21 | 26 |
| spec-coverage | 1.10.0 | 23 | 26 |

---

## Feature 1 — `specter explain` bundle

### Spec (commit 1)

`spec-explain` 1.0.0 → 1.1.0 (additive, minor bump). New constraints and ACs:

- C-09: `specter explain annotation` prints the test-annotation reference (equivalent of `docs/TEST_ANNOTATION_REFERENCE.md`) to stdout.
- C-10: `specter explain schema` prints the full schema field reference to stdout.
- C-11: `specter explain schema <field-path>` (e.g. `spec.acceptance_criteria.items.approval_gate`) prints single-field detail: type, default, description, enum values.
- C-12: `specter explain <spec-id>` (no AC suffix) prints a human-readable spec card: tier, coverage %, test files per AC, uncovered ACs with descriptions.
- AC-07: `explain annotation` output contains both Convention A and Convention B sections.
- AC-08: `explain schema` output enumerates every field declared in `internal/parser/spec-schema.json`.
- AC-09: `explain schema <invalid-path>` exits non-zero with "unknown field path" and a `did you mean?` suggestion when possible.
- AC-10: `explain <spec-id>` (AC-less) renders tier, current coverage %, and per-AC test-file list.

### Tests (commit 2)

New test file `cmd/specter/explain_bundle_test.go`:
- `TestExplainAnnotation_PrintsReference` — `// @spec spec-explain`, `// @ac AC-07`.
- `TestExplainSchema_FullReference` — `// @ac AC-08`.
- `TestExplainSchema_FieldPath_Unknown` — `// @ac AC-09`.
- `TestExplainSpecCard_RendersTierAndCoverage` — `// @ac AC-10`.

All four wrap body in Convention A subtest: `t.Run("spec-explain/AC-NN description", ...)`.

### Implementation (commit 3)

- `cmd/specter/explain.go`: add `annotation` and `schema` subcommand branches. Embed `TEST_ANNOTATION_REFERENCE.md` via `//go:embed`.
- New `internal/explain/schema_walker.go`: walks the embedded JSON schema, emits the field table. Pure function; no I/O.
- `cmd/specter/explain.go`: AC-less `explain <spec-id>` calls existing `internal/coverage.BuildCoverageReport` and formats the spec card.
- `docs/CLI_REFERENCE.md`: add the three new forms to the `explain` table.

### Eval

`make dogfood-strict` green. `specter explain annotation`, `specter explain schema`, `specter explain schema spec.acceptance_criteria.items.approval_gate`, `specter explain spec-coverage` all print expected shapes. Docs review agent pass on `CLI_REFERENCE.md` delta (per root `CLAUDE.md` Docs Review Policy).

---

## Feature 2 — `specter check --test` / `-t`

### Spec (commit 1)

`spec-check` 1.1.0 → 1.2.0. Design decisions confirmed 2026-04-23 (see BACKLOG):

- C-09: `check --test` cross-references test-file `@spec` / `@ac` annotations against parsed specs. Opt-in in v0.11.
- AC-09: emits `unknown_spec_ref` when `// @spec foo` has no matching spec in workspace.
- AC-10: emits `unknown_ac_ref` when `// @ac AC-99` references an AC that does not exist in the named spec.
- AC-11: emits `malformed_ac_id` for non-zero-padded or wrong-case AC IDs (`AC-1`, `ac-01`).
- AC-12: `specter sync --strict` routes the flag through to the check phase; sync CI runs test-annotation checks when `--strict` is on.

### Tests (commit 2)

New `internal/checker/test_annotations_test.go`:
- `TestCheckTest_UnknownSpecRef` — `// @ac AC-09`.
- `TestCheckTest_UnknownAcRef` — `// @ac AC-10`.
- `TestCheckTest_MalformedAcId` — `// @ac AC-11`.

New CLI test in `cmd/specter/check_test.go`:
- `TestCheckTest_SyncStrictIntegration` — `// @ac AC-12`. Verifies `sync --strict` fails when a test annotation is broken.

### Implementation (commit 3)

- `internal/checker/test_annotations.go`: reads `@spec` / `@ac` from `*_test.go` and `*.test.ts`. Regex-level scan is sufficient for v0.11; full parser deferred to v0.12 for `unreachable_annotation`.
- `cmd/specter/check.go`: add `--test` / `-t` flag. Diagnostics mix into existing stream, new diagnostic kinds registered.
- `cmd/specter/sync.go`: propagate `--strict` to the check phase; add a corresponding `--check-tests` or reuse `--strict` — decide during impl.
- `docs/CLI_REFERENCE.md`: add `--test` / `-t` to the `check` flag table.

### Eval

`make dogfood-strict` green. `specter check --test` on a known-broken fixture (testdata file with `// @ac AC-999`) exits non-zero with all three diagnostic kinds at least once between fixtures. Docs review agent pass.

---

## Feature 3 + 4 — `init --install-hook` + `init --ai <tool>` (bundled)

### Spec (commit 1)

`spec-manifest` 1.6.0 → 1.7.0. Both features ship in one spec bump since both extend `init`:

**`--install-hook`:**
- C-22: `init --install-hook` writes `.git/hooks/pre-push` that blocks pushes where implementation files changed but no `@spec` / `@ac` was added or updated in the diff.
- AC-27: hook is executable after write.
- AC-28: hook exits non-zero on a diff with impl change + no annotation delta.
- AC-29: `git push --no-verify` bypass works (documented, discouraged).

**`--ai <tool>`:**
- C-23: `init --ai <tool>` writes a per-tool AI instruction file with a fenced `<!-- specter:begin v1 -->` / `<!-- specter:end -->` region.
- AC-30: `--ai claude` writes `CLAUDE.md`; if an `AGENTS.md` exists, body is `@AGENTS.md` + Claude-specific addenda, else inline body.
- AC-31: `--ai codex` writes `AGENTS.md`.
- AC-32: `--ai cursor` writes `.cursor/rules/specter.md`.
- AC-33: `--ai copilot` writes `.github/copilot-instructions.md`, body capped at 4KB.
- AC-34: `--ai gemini` writes `GEMINI.md`.
- AC-35: re-running `init --ai <tool>` replaces only the fenced region; out-of-fence content preserved byte-for-byte.
- AC-36: instruction body matches the v0.11 template (self-check preflight at top, Convention A good/bad examples, `make dogfood-strict` gate, on-demand reference to `specter explain`).

### Tests (commit 2)

- `cmd/specter/init_hook_test.go`: four tests (AC-27 through AC-29 + a no-op re-run case).
- `cmd/specter/init_ai_test.go`: six tests covering AC-30 through AC-36 (one per tool + one idempotency test).

All tests use Convention A subtest wrapping.

### Implementation (commit 3)

- `cmd/specter/init.go`: add `--install-hook` and `--ai <tool>` flags.
- `internal/init/hook_template.go`: embed the pre-push hook script via `//go:embed`.
- `internal/init/ai_template.go`: embed per-tool instruction templates; render with project-specific values (strictness level from `specter.yaml`, spec count, make target names).
- `internal/init/fenced_region.go`: pure-function fenced-region read/write/replace. Reusable between hook-file and instruction-file idempotency.
- `docs/CLI_REFERENCE.md`: add `--install-hook` and `--ai <tool>` to the `init` flag table.

### Eval

`make dogfood-strict` green. On a clean fixture workspace: `specter init --install-hook` produces executable pre-push hook; `specter init --ai claude` produces `CLAUDE.md` matching the v0.11 template. Re-running each command does not clobber manual edits outside the fenced region. Docs review agent pass on both `CLI_REFERENCE.md` delta and the generated-template content.

---

## Feature 5 — `settings.strictness`

### Spec (commit 1)

Two specs bump in one commit:

**`spec-manifest` 1.7.0 → 1.8.0:**
- C-24: `settings.strictness` field with enum `{annotation, threshold, zero-tolerance}`, default `threshold`.
- AC-37: parse accepts all three enum values; default applied when unset.
- AC-38: parse rejects invalid enum with clear error message.

**`spec-coverage` 1.10.0 → 1.11.0:**
- C-24: `strictness=annotation` rejects `--strict` CLI flag with clear error.
- C-25: `strictness=zero-tolerance` exits non-zero when any annotated AC has `status != passed`, regardless of tier threshold.
- C-26: `strictness=zero-tolerance` exits non-zero when any AC has `approval_gate: true` and `approval_date` unset.
- AC-27: `--strictness <level>` CLI flag overrides `specter.yaml` per-invocation.
- AC-28: `--strict` is preserved as a shortcut for `--strictness threshold` (backwards compatible).
- AC-29: exit-code contract — 0 for pass, 2 for strictness violation, 3 for approval-gate violation under zero-tolerance.

### Tests (commit 2)

- `internal/parser/strictness_test.go`: AC-37, AC-38.
- `internal/coverage/strictness_test.go`: AC-27 through AC-29, plus one test per `strictness` level verifying demotion semantics.
- `cmd/specter/coverage_strictness_test.go`: CLI-level — `--strictness zero-tolerance` on a fixture with one failing test exits code 2; same fixture under `threshold` exits 0 if tier threshold met.

### Implementation (commit 3)

- `internal/parser/spec-schema.json`: add `settings.strictness` enum. JSON-schema validation catches AC-38.
- `internal/coverage/coverage.go`: `BuildCoverageReportStrict` gains a `Strictness` field; exit-code contract lives in `cmd/specter/main.go`.
- `cmd/specter/main.go`: add `--strictness` flag; wire `--strict` as its shortcut.
- `docs/CLI_REFERENCE.md`: new section on strictness levels. `docs/SPEC_SCHEMA_REFERENCE.md`: add `settings.strictness` field row.

### Eval

`make dogfood-strict` green under default (`threshold`). Set `settings.strictness: zero-tolerance` in `specter.yaml` and re-run — confirm the gate still passes (all 15 specs at 100% avg coverage means zero failing tests). Create a deliberate failing-test fixture and confirm zero-tolerance exits 2 where threshold would exit 0. Two review agents (per root `CLAUDE.md` Docs Review Policy) verify `SPEC_SCHEMA_REFERENCE.md` and `CLI_REFERENCE.md` deltas match the embedded schema and code behavior.

---

## Release gate (before tagging v0.11.0)

`release/v0.11` working branch opens when Feature 1 begins. Each wave (A, B, C) merges to the working branch via PR. After Wave C lands:

1. `make check` + `make dogfood` + `make dogfood-strict` all green on CI.
2. Working branch merges to `main` via single PR. No bump in this merge.
3. Separate `bump/v0.11.0` PR to `main` — `VERSION`, `package.json`, `CHANGELOG.md`, nothing else.
4. Tag `v0.11.0` on new `main` HEAD.
5. VSCode extension: follow the `RELEASING.md` gate (install VSIX locally, test known-working and known-failing workspaces, exercise every changed code path, human sign-off). Publish `--pre-release` first for the `init --ai` and `settings.strictness` changes; promote to stable after a soak window.
6. Delete `release/v0.11` working branch after tag ships.

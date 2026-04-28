# Changelog

All notable changes to Specter (CLI + VS Code extension) documented here. The project is pre-1.0; breaking changes go in MINOR releases per semver conventions for 0.x.

---

## v0.11.1 — 2026-04-26

**Theme: post-v0.11.0 hotfix.** Two bugs reported within hours of v0.11.0 shipping; both fixed.

### Fixed

#### GH #94 — `strictness=zero-tolerance` + `approval_gate` report demotion

v0.11.0 fired exit code 3 when an AC carried `approval_gate: true` with unset `approval_date` under zero-tolerance, but the report cell continued to show the AC as PASS. Reporter expected the report to also reflect the demotion. v0.11.1 demotes such ACs in the report (moves them from `CoveredACs` to `UncoveredACs`, recomputes per-entry `CoveragePct` + `PassesThreshold`, recomputes `Summary.Passing` / `Summary.Failing`). Threshold mode unchanged — `approval_gate` stays metadata there per spec contract.

**Behavior change for `strictness: zero-tolerance` only:** a spec with an `approval_gate: true` AC and unset `approval_date` now shows as `NONE / uncovered` in the report (was `PASS` in v0.11.0). Exit code 3 unchanged.

#### GH #95 — `check --test` false positive on multi-`@spec` test files

Test files declaring two `@spec` headers at the top got the second header as the parent context for following `@ac` lines. An `@ac` legitimately in the FIRST declared spec was flagged `unknown_ac_ref`. Fix: each `@ac` is now validated against the union of declared specs in the file. Cross-cutting tests that bridge two specs work as expected.

### Internal

- `cmd/specter/main.go`: new `demoteApprovalGateViolations` post-processor.
- `internal/checker/test_annotations.go`: `scanFileAnnotations` tracks `declaredSpecs` (slice + dedupe set) instead of single `currentSpec`.
- 5 new regression tests across `cmd/specter/coverage_strictness_test.go` and `internal/checker/test_annotations_test.go`.

### No spec changes

Spec contracts unchanged from v0.11.0; the v0.11.1 fix aligns the implementation with the contracts already declared (spec-coverage AC-29 always implied report demotion, even though the explicit AC text only mentioned exit code).

---

## v0.11.0 — 2026-04-26

**Theme: AI loop discipline + adoption hardening.**

Five new features close order-of-operations gaps; four GH issues from real adopter friction are fixed (#75, #76, #78, #79 cheap fix). Full walkthrough in `docs/explainer/v0.11-ai-loop-discipline.md`.

### Added

#### `specter explain` — three new read-only surfaces

Stdout-only. File writes are scoped to `init --ai <tool>` (below).

- `specter explain annotation` — prints the test-annotation reference (Convention A title form, Convention B runtime-log form, source-comment annotations that pair with either).
- `specter explain schema` — prints the full schema field reference. Walks JSON `$ref` into `$defs` so nested fields under `spec.acceptance_criteria.items.*` and `spec.constraints.items.*` resolve.
- `specter explain schema <field-path>` — prints single-field detail (type, default, enum, description). Returns non-zero with a `did you mean?` suggestion within Levenshtein 3 on unknown paths.
- `specter explain <spec-id>` (no AC suffix) — now renders a spec card: tier, coverage %, per-AC test files. Previously showed only COVERED/UNCOVERED labels.

Spec: `spec-explain` 1.0.0 → 1.1.0 (C-09..C-12, AC-07..AC-10).

#### `specter check --test` (`-t`) — test-annotation cross-reference

Opt-in flag. Scans test files for `@spec` / `@ac` source comments and emits diagnostics for references that don't resolve against parsed specs. Three diagnostic kinds:

- `unknown_spec_ref` — `// @spec foo` where no spec with that id exists.
- `unknown_ac_ref` — `// @spec real-spec` + `// @ac AC-99` where the named spec doesn't declare that AC.
- `malformed_ac_id` — IDs failing `^AC-\d{2,}$` (`AC-1` not zero-padded, `ac-01` wrong case, `AC-1A` suffixed).

Skips lines inside multi-line string literals (TS template strings, Python triple-quoted strings). Cascade rule: when `@spec` is unknown, child `@ac` lines are not separately checked. `specter sync --strict` (and `settings.strict: true`) routes the check through.

Spec: `spec-check` 1.1.0 → 1.2.0 (C-09, AC-09..AC-12).

#### `specter init --install-hook` — git pre-push hook

Writes `.git/hooks/pre-push` (mode 0755) that blocks pushes whose diff changes implementation files but adds or updates no `@spec` / `@ac` annotations. Hook delegates to a hidden `specter pre-push-check` subcommand that reads git's pre-push stdin format, runs `git diff` per ref, and exits non-zero on impl-only diffs.

- Doc / spec / test changes always pass.
- Diffs touching impl AND adding an annotation delta in any test file pass.
- New branches without `origin/HEAD` merge-base are skipped (no "before" to compare against).
- `git push --no-verify` bypasses (git's behavior, not Specter's).

Hook script wrapped in shell-comment fenced markers (`# specter:begin v1` / `# specter:end`). Re-runs replace only the fenced region.

Spec: `spec-manifest` C-22, AC-27..AC-29.

#### `specter init --ai <tool>` — per-tool AI instruction file

Five tools, one command per tool:

| `--ai <tool>` | Target file |
|---|---|
| `claude` | `CLAUDE.md` |
| `codex` | `AGENTS.md` |
| `cursor` | `.cursor/rules/specter.md` (creates parent dir) |
| `copilot` | `.github/copilot-instructions.md` (capped at 4096 bytes) |
| `gemini` | `GEMINI.md` |

Body wrapped in `<!-- specter:begin v1 -->` / `<!-- specter:end -->` markers. Re-runs replace only the fenced region; out-of-fence content preserved byte-for-byte.

Body teaches the AI to (1) read the spec before writing code, (2) use Convention A annotations with good/bad examples, (3) run `make dogfood-strict` before declaring work done, (4) call `specter explain` for spec content on demand.

`--ai claude` checks for an existing `AGENTS.md` and writes `@AGENTS.md` import directive instead of inlining the body, avoiding duplication.

Spec: `spec-manifest` C-23, AC-30..AC-36.

#### `settings.strictness` — three-level coverage gate

```yaml
settings:
  strictness: threshold     # annotation | threshold (default) | zero-tolerance
```

| Level | Behavior |
|---|---|
| `annotation` | Pre-v0.10 behavior. `coverage --strict` rejected as incoherent. |
| `threshold` (default) | v0.10.x `--strict` behavior. Tier threshold applies after demotion. |
| `zero-tolerance` | Any non-passed annotated AC exits 2. `approval_gate: true` with unset `approval_date` exits 3 (distinct). |

Override per invocation with `--strictness <level>`. CLI flag and manifest field validated against the same enum; typos at either layer error with valid values listed.

Spec: `spec-manifest` C-24, AC-37..AC-38; `spec-coverage` C-24..C-26, AC-27..AC-29.

#### `settings.tests_glob` — default test-discovery pattern

```yaml
settings:
  tests_glob: "tests/**/*.py"           # string form
  # OR
  tests_glob:                           # list form
    - "backend/**/*_test.go"
    - "frontend/**/*.test.ts"
```

Used when `--tests` is unset. Supports `**` (recursive). List form unions matches.

Spec: `spec-manifest` C-25, AC-39. Closes GH #78.

### Fixed

#### GH #75 — silent 0% on empty test discovery (under `--strict`)

`specter coverage --strict` no longer falls through to `filepath.Walk(".")` when the configured glob matches zero files. Instead, it warns above the coverage table:

```
warn: no test files contained @spec/@ac annotations — coverage will report 0% for every spec
      set settings.tests_glob in specter.yaml or pass --tests <glob>
```

Under `strictness: zero-tolerance`, the warning escalates to a hard error.

Spec: `spec-coverage` C-27, AC-30.

#### GH #76 — `specter.yaml` settings block silently accepted unknown keys

`ParseManifest` now errors on unknown top-level and `settings:` keys with did-you-mean suggestions (Levenshtein ≤ 3) and the full list of valid keys. Existing manifests with typo'd keys will start failing on parse — fix the typo or remove the field.

Spec: `spec-manifest` C-26, AC-40.

#### GH #79 (cheap fix) — ingest body regex accepts `#` and `*`

`specter ingest`'s body-text annotation extractor (used to scan JUnit `<system-out>`) previously accepted `//` only. Now accepts `//`, `#`, `*` — the same three markers the source-file scanner already accepts. Cross-language Convention B output flows through ingest without language-specific kludges.

Pytest users can `print("# @spec my-spec")` from inside a test and ingest extracts identically to a Go test's `t.Log("// @spec my-spec")`.

The bigger fix (a `pytest-specter` plugin) is tracked for a follow-up.

Spec: `spec-ingest` 1.2.0 → 1.3.0 (C-12, AC-12).

### Security

A pre-release security review identified hardening opportunities across the VS Code extension, the CLI, and the build. All addressed in v0.11.0:

- **VS Code extension** — `specter.binaryPath` and `specter.version` are now declared `"scope": "machine"`, so workspace-level overrides are ignored. `specter.version` is additionally validated against strict semver (`^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$`) before being interpolated into download URLs. `package.json` declares `capabilities.untrustedWorkspaces` with `supported: "limited"`. The "View Diff" terminal command refuses paths containing shell metacharacters before reaching `terminal.sendText`. The `which` shim switched from `execSync` template strings to `execFileSync` array form.
- **CLI — pre-push hook** (new in v0.11.0): `ParsePushSpecs` validates `local_sha` and `remote_sha` against git's canonical 40-char hex form. `init --install-hook` uses `os.Lstat` and refuses to write through a `.git` symlink or worktree pointer file.
- **Build** — `go.mod` directive bumped from `1.25.8` to `1.25.9`, picking up five stdlib advisories (TLS, x509, archive/tar, html/template).

No CVEs were assigned; the findings were caught pre-release. The disclosure note in `docs/explainer/v0.11-ai-loop-discipline.md` covers the threat model and mitigations.

### Changed

- `loadManifest()` (internal) now returns an error when an existing `specter.yaml` fails to parse, rather than silently falling back to defaults. Library helpers (`noSpecsMessage`, `discoverSpecs`) tolerate missing manifests; RunE handlers fail-fast on invalid ones. Combined with the unknown-key rejection above, every typo in `specter.yaml` now surfaces at parse time.
- `discoverTestFiles` no longer falls through to walking `.` when an explicit glob matches zero files. The empty result is surfaced (and warned about) instead of hidden behind a noisy walk.
- `specter coverage --strict` exit codes: 0 (pass), 2 (strictness violation under zero-tolerance), 3 (approval-gate violation under zero-tolerance), errSilent (tier-threshold failure under threshold mode).

### Internal

- New package `internal/explain` — pure functions for schema walking and annotation reference rendering.
- New `internal/manifest/string_or_list.go` — custom `UnmarshalYAML` accepting scalar or sequence.
- New `internal/manifest/fenced.go` — `ReplaceFencedRegion` helper for idempotent file regions, used by both `init --install-hook` and `init --ai <tool>`.
- New `internal/manifest/{hook,ai_templates,prepush}.go` — pure logic for hook script, instruction templates, and pre-push diff classification.
- New `cmd/specter/glob.go` — `**`-aware glob matcher (no new dependency); used by `discoverTestFiles` when an explicit glob is provided.
- `internal/parser.SchemaBytes()` exposes the embedded JSON schema for consumers that need to walk it.

### Spec versions

| Spec | v0.10.2 | v0.11.0 |
|---|---|---|
| spec-explain | 1.0.0 | 1.1.0 |
| spec-check | 1.1.0 | 1.2.0 |
| spec-ingest | 1.2.0 | 1.3.0 |
| spec-manifest | 1.6.0 | 1.8.0 |
| spec-coverage | 1.10.0 | 1.11.0 |

### Migration

Most projects: drop-in. `settings.strictness` defaults to `threshold` (matches v0.10.x `--strict`); `tests_glob` is opt-in.

Projects with typos in `specter.yaml`: the parse error will surface on first invocation. Fix the offending key (the error names the closest valid key).

Python projects using pytest with `# @spec` source comments: `coverage` already worked via the source-file scanner. `coverage --strict` now also works if you wire the autouse-fixture pattern in `conftest.py` (documented in `docs/explainer/v0.11-ai-loop-discipline.md`).

---

## v0.10.2 — 2026-04-23

**Theme: docs/code parity fixes from jwtms Wave 0/1 integration.**

Two bugs surfaced during jwtms `--strict` rollout. Both are small; they ship together.

### Fixed

#### BUG-2 — `specter ingest --junit` and `--go-test` accept globs and repeated flags

The v0.10.0 CHANGELOG claimed `--junit <path>` supports globs. The code used `os.ReadFile` on a single path with `StringVar`, so:

- `specter ingest --junit 'test-results/*.xml'` failed with `open test-results/*.xml: no such file or directory`.
- `specter ingest --junit a.xml --junit b.xml` silently overwrote — only the last file's results made it into the output.

v0.10.2 implements the documented behavior:

- Paths containing `*`, `?`, or `[...]` are expanded via `filepath.Glob`.
- `--junit` and `--go-test` flags may be repeated; all specified files are read.
- Results from all files merge into one output via the existing worst-status-wins rule.
- A glob matching zero files is now a hard error (`--junit "no-such-*.xml": no files matched`) rather than a silent empty output.

Single-invocation CI patterns now work as documented:

```
specter ingest --junit 'test-results/*.xml' --output .specter-results.json
specter ingest --junit unit.xml --junit integration.xml
specter ingest --go-test 'go-*.json'
```

spec-ingest 1.1.0 → **1.2.0** (+C-11/AC-11 covering multi-file input).

#### BUG-3 part 1 — `approval_gate` docs parity

`docs/SPEC_SCHEMA_REFERENCE.md` claimed `specter coverage` demotes ACs with `approval_gate: true && approval_date == null`. The embedded JSON schema (the authoritative field definition) and the code both said the opposite — Specter does not enforce approval semantics; teams wire their own PR/CI gates. The human doc was the outlier.

v0.10.2 updates the human doc to match:

- `approval_gate` is metadata. `specter coverage` counts the AC as covered when a matching `@ac` annotation exists, regardless of `approval_gate` or `approval_date`.
- Teams wire enforcement into their own gates (example: pre-push hook rejecting diffs where any AC has `approval_gate: true && approval_date == null`).
- `approval_date` is metadata; Specter validates the ISO-8601 format at parse time but does not read the field at runtime.

No code or schema changes. The doc now states what the code actually does.

### Spec bumps

- `spec-ingest`: 1.1.0 → **1.2.0** (+C-11/AC-11 multi-file input).

### Release notes

No CLI behavior regressions from v0.10.1. `specter ingest`'s command surface gains documented behavior it was always supposed to have; no caller that worked under v0.10.1 breaks. VS Code extension runtime unchanged; bumped to 0.10.2 for version alignment.

---

## v0.10.1 — 2026-04-23

**Theme: Fix the docs that taught the wrong convention for `--strict`.**

v0.10.0 shipped `specter coverage --strict`, but every foundational guide still showed the v0.9-era source-only annotation form. Source comments (`// @spec` / `// @ac` above a test function) reach `specter coverage` but are invisible to `specter ingest`, so `--strict` demoted every test that only had source comments. A developer following the official guide wrote tests that demoted with no document to diagnose why. jwtms hit this on first `--strict` run. This patch fixes the foundation.

### Added

#### `docs/TEST_ANNOTATION_REFERENCE.md`

- Authoritative one-page reference for test annotations. The counterpart to `SPEC_SCHEMA_REFERENCE.md`.
- Two-channel rule (source comments count, runner-visible pair verifies).
- The extraction regex verbatim from `internal/ingest/annotations.go`.
- Per-runner sections: Go (`t.Run` subtests), TypeScript (jest/vitest with JUnit reporter), Python (known Convention A gap, Convention B as today's workaround), runtime-log form for any language.
- Parameterized tests per runner.
- Migration recipe from v0.9-style source-only, with file-atomic discipline.
- Common mistakes table (`AC-1` vs `AC-01`, `_` vs `/`, multi-AC tests).
- Troubleshooting keyed by symptom.

### Changed

- `docs/AI_PROMPTS.md` §3 (Spec → Tests) — teaches both source comments and runner-visible annotations. AI prompt block now asks for `[spec-id/AC-NN]` in test titles.
- `docs/GETTING_STARTED.md` Phase 4 — same update. TypeScript/Python/Go examples updated. Python encodes the pair in the function name; Go uses `t.Run` subtests.
- `docs/CLI_REFERENCE.md` coverage `--strict` section — adds the two-channel rule and the runner-visible format rules.
- `vscode-extension/walkthrough/step3.md` — onboarding walkthrough updated.
- All four docs cross-link into `TEST_ANNOTATION_REFERENCE.md` rather than duplicating the rules.

### BACKLOG

- Adds v0.11 bullet **Python Convention A gap** — pytest function names can't contain `/` or `:`, so the natural Convention A form doesn't match the ingest regex. Documents two candidate resolutions: docs-only (Python uses Convention B) or regex extension. Blocked on a design call.

### No code changes

CLI and extension runtime unchanged from v0.10.0. This is a docs release — the version bump exists to mark "v0.10.0 shipped with misleading guidance, v0.10.1 corrects it." Extension version bumped from 0.10.0 to 0.10.1 to match.

---

## v0.10.0 — 2026-04-22

**Theme: CI-gated coverage — test outcome is mechanical.**

v0.9.x made test existence mechanical (`coverage` counts annotated ACs). v0.10 makes test outcome mechanical: `coverage --strict` demotes any annotated AC whose test did not pass. See `docs/explainer/v0.10-ci-gated-coverage.md` for the design rationale.

### Added

#### `specter ingest` (new command)

- Converts test runner output into `.specter-results.json`, the canonical results file `coverage --strict` reads.
- Flags: `--junit <path>` (JUnit XML, glob supported), `--go-test <path>` (`go test -json` output), `--output <path>` (defaults to `.specter-results.json`).
- Flavor-specific parsing is isolated here; adding a new runner is a change to `ingest` only. `coverage --strict` stays runner-agnostic.
- Reads the `(spec_id, ac_id)` pair from runner-visible surfaces — subtest names (`t.Run("spec-foo/AC-03 ...", ...)`) or runtime logs (`t.Log("// @spec ...")` / `t.Log("// @ac ...")`). Source-comment annotations are invisible to `ingest` by design.

#### `specter coverage --strict`

- New flag. When passed, every annotated AC must have a `status: passed` entry in `.specter-results.json`. Anything else (`failed`, `skipped`, `errored`, or no entry) demotes the AC to uncovered.
- Demotion applies to **all tiers**, not only Tier 1.
- Missing or empty `.specter-results.json` is a hard error: `--strict requires .specter-results.json — run 'specter ingest' first`. Fails closed so the flag cannot silently degrade to annotation-only behavior.

#### `.specter-results.json` status enum

- Adds `status` field: `passed` | `failed` | `skipped` | `errored`.
- `errored` is distinct from `failed` — it means the framework itself failed (setup panic, compile error) rather than an assertion.
- Worst-status-wins when the same `(spec_id, ac_id)` is observed across multiple tests: `errored > failed > skipped > passed`.
- The boolean `passed` field is retained for pre-1.9.0 consumers; no forced migration.

#### VS Code extension: CLI auto-download defaults to matching version

- `specter.version` config default changed from `"latest"` to `""` (empty). With the empty default, `downloadBinary` reads `ctx.extension.packageJSON.version` and fetches the matching CLI — a v0.10.0 VSIX always pulls v0.10.0 CLI. `"latest"` remains available as an explicit opt-in; pinned semvers (e.g. `"0.9.2"`) still work as before.
- Why: the GoReleaser workflow creates a GitHub Release on tag push, so the v0.10.0 CLI archive was live on `/releases/latest` before any v0.10.0 extension shipped to the Marketplace. Any v0.9.x extension with `autoDownload: true` then pulled v0.10.0 CLI via the old `"latest"` default, producing split-brain installs (v0.9.2 extension + v0.10.0 CLI). Pinning to the extension's own version closes the gap.

#### Adoption affordances: empty-results warning, `--scope`, `--verbose`

Three diagnostic/staged-adoption features found during v0.10.0 shake-down on jwtms. Without them, `--strict` is technically functional but operationally unusable on a workspace that hasn't migrated every test to runner-visible annotations.

- **`specter coverage --strict` empty-results warning.** When `.specter-results.json` parses cleanly but contains zero entries, the command now emits a stderr warning BEFORE the demotion report: *"no (spec_id, ac_id) pairs were extracted from test output — tests likely don't carry runner-visible annotations"* with a pointer to `docs/explainer/v0.10-ci-gated-coverage.md` (Conventions A and B). Prior behavior silently demoted 100% of annotated ACs with no clue why. (Note: missing file — as opposed to empty file — still errors per the existing AC-20 contract.)
- **`specter coverage --strict --scope <domain>`.** Narrows `--strict`'s demand set to specs listed under the named domain in `specter.yaml`. Specs outside the domain fall back to v0.9 boolean-passed logic (annotation alone counts for tier 2/3). Enables staged adoption: enforce `--strict` on one domain per wave instead of rewriting every annotated test before CI can pass. `--scope` without `--strict` fails fast. Combines with `--tests` as AND.
- **`specter ingest` default summary + `--verbose`.** Every run now emits to stderr: *"Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation."* Replaces the terse `Wrote N result entries`. `--verbose` adds a per-case drop reason line for each skipped testcase — off by default to keep CI logs compact.

### Spec bumps

- `spec-coverage`: 1.8.0 → **1.10.0** (+C-19/AC-19 strict demotion all tiers; +C-20/AC-20 missing-results error; +C-21/AC-21/AC-22 status enum w/ back-compat; +C-22/AC-23 empty-results warning; +C-23/AC-24/AC-25/AC-26 `--scope` domain flag)
- `spec-ingest`: new spec at **1.1.0** (17 ACs covering JUnit/go-test parsing, status derivation, worst-status-wins, output contract; + C-09/AC-09 default scan summary; + C-10/AC-10 `--verbose` per-case drops)
- `spec-vscode`: 1.3.0 → **1.4.0** (+C-27/AC-50 covering the version-pinning default)

### Out of scope for v0.10

- Flake handling (planned: `status: flaky` + `--deny-flaky` in v0.11).
- Source-file tracking under `--strict`.
- VS Code red-dot rendering for failed annotated ACs (fast-follow, not this cut).

---

## v0.9.2 — 2026-04-20

**Theme: UX polish from jwtms migration testing.**

Two items surfaced when running v0.9.1 against the fully-migrated jwtms workspace (249 specs). Both are quality-of-life fixes; no security or correctness issues.

### Added

#### `specter coverage` visual redesign

- **Summary header** above the table:
  ```
  Spec Coverage Report — 249 specs · 97.2% avg coverage
    Tier 1: 32/34 passing (94%)
    Tier 2: 168/192 passing (88%)
    Tier 3: 11/23 passing (48%)
  ```
  Gives one-glance visibility into the overall shape before scanning the table. Tiers with zero specs are omitted.
- **Worst-first sort** in the default table: failing (below threshold) → partial (below 100% but passing threshold) → 100% covered. Within each bucket, tier descending (T1 > T2 > T3) so higher-risk work surfaces first.
- **`--failing` flag** filters the table to entries below 100% coverage. Summary header still reflects the full report. When every spec is at 100%, emits a single-line confirmation (`All N specs at 100% coverage.`) instead of an empty table.
- **Long spec ID truncation**: IDs over 40 characters are truncated with a trailing ellipsis (`…`) so the Tier column stays aligned. `--json` output is unaffected — it emits the full spec_id.

#### `specter init --refresh` for non-greenfield workspaces

- **`--refresh` flag**: updates only `domains.default.specs` in an existing `specter.yaml`. Preserves every other field — `settings`, `registry`, tier overrides, system metadata, and any custom domains the operator declared.
- **Smart diff**: specs on disk that are claimed by a non-default domain stay in that domain (aren't duplicated into `default`). Specs that used to be in `default.specs` but are no longer on disk are removed.
- **Summary line**: `updated specter.yaml: +A added, -B removed`.
- **`--dry-run` variant**: `specter init --refresh --dry-run` prints the proposed diff without writing the file. Matches `git add -p` / `terraform plan` discipline.
- **`--refresh` and `--force` mutually exclusive**: `--force` rewrites everything; `--refresh` preserves everything except `domains.default.specs`. Attempting both exits non-zero with a clear message.

### Spec bumps

- `spec-coverage`: 1.7.0 → **1.8.0** (+C-15/AC-15 sort, +C-16/AC-16 summary header, +C-17/AC-17 --failing, +C-18/AC-18 truncation)
- `spec-manifest`: 1.5.0 → **1.6.0** (+C-17/AC-23 through +C-21/AC-26 covering --refresh, --dry-run, custom domains, removed specs, flag conflict)

14 specs dogfood at 100% AC coverage. All Go + TS tests pass. No security changes.

---

## v0.9.1 — 2026-04-19

**Theme: post-ship audit fixes.**

Derived from `research/SPECTER_AUDIT_2026-04-19.md`. Five parallel audit agents reviewed the v0.9.0 codebase; findings were verified against live code before triage. This release ships the CRITICAL + BLOCKER + HIGH items; MEDIUM and LOW items are queued for v0.10.

### Fixed (CRITICAL)

- **Binary-download checksum verification is now mandatory.** Prior behavior: if `checksums.txt` was unreachable (404, timeout, MITM block), the extension silently fell back to installing the unverified binary. A MITM attacker with the ability to selectively block `checksums.txt` could deliver a tampered archive. Now: missing checksums file, missing entry for the specific archive, or hash mismatch all produce a modal error and refuse installation.

### Fixed (BLOCKER)

- **`specter.runReverse` is now registered.** The command was declared in `package.json` (including as the first step of the onboarding walkthrough) but had no handler in `extension.ts`. Invoking it produced "command not found." The handler opens the integrated terminal with `specter reverse` prefilled so the user can pick a source directory.
- **`specter.openQuickStart` orphan declaration removed.** Declared in `package.json` with no handler and no user-facing invocation. Removing the declaration is the honest move until an actual QuickStart walkthrough is designed.
- **Package.json ↔ extension.ts command parity is now CI-enforced.** A new `commands.test.ts` reads both sources and asserts set equality (minus `specter._`-prefixed internal commands, by convention). Prevents the declared-but-unregistered class that shipped three times in v0.9.0 (`specter.revealInTree`, `specter.runReverse`, `specter.openQuickStart`).

### Fixed (HIGH)

- **Fresh-install UX on new machines.** Extension activation now resolves the CLI binary (with auto-download, subject to `specter.autoDownload`) even when the current workspace contains no `.spec.yaml` files and no `specter.yaml`. Users who install the extension on a new machine and open a folder that isn't yet a Specter project can now invoke `specter.runReverse` and other commands without first having to manually trigger a download via the command palette.
- **Walkthrough reachable.** The `shouldShowWalkthrough` condition (no specs, no manifest) was mutually exclusive with the `shouldActivate` early-return that preceded it (has specs or manifest). The onboarding walkthrough that fires for empty workspaces could never run. Moved the check before the early-return.
- **`driftDecorationType` disposed on reload.** Created via `vscode.window.createTextEditorDecorationType` but never pushed to `ctx.subscriptions`; leaked across every Developer: Reload Window cycle. Now correctly disposed.
- **On-type parse errors and drift-scan failures route to the Output channel.** Three previously-silent `catch` sites (`catch { /* ignore parse failures */ }` and two `scanForDrift(...).catch(() => {})`) now log a timestamped entry to the Specter Output channel. Same discipline applied across v0.9.0 for coverage failures; caught these stragglers in the audit.
- **Nil slices in `CoverageReport` now marshal as `[]`, not `null`.** Go's zero-valued `[]string` previously marshalled to `null`, but TypeScript consumers declared `string[]` (non-nullable). Latent runtime-crash class for any future code trusting the type. Now consistent: fields without `omitempty` emit `[]`; fields with `omitempty` are absent.
- **`specter.insertAnnotation` renamed to `specter._insertAnnotation`.** VS Code community convention: internal commands (invoked programmatically from CodeActions / CodeLenses, never from the palette) use the `_` prefix and are exempt from the package.json declaration requirement.

### Spec bumps

- `spec-vscode`: 1.2.0 → **1.3.0** — adds C-22 through C-26 (parity, disposables, activation, checksum, error surfacing) and AC-41 through AC-49.
- `spec-coverage`: 1.6.0 → **1.7.0** — adds C-14 / AC-14 (empty array emits `[]`, never `null`).

All 14 specs dogfood at 100% AC coverage. 209 TypeScript tests pass. All Go tests pass under Go 1.25.8 + golangci-lint v2.6.2.

### Deferred to v0.10

From the audit's MEDIUM tier: HTTPS-redirect validation in `httpsGet`, cache-directory permission hardening (`mode: 0o700`), subprocess `maxBuffer` caps, tar-slip defenses via `node-tar`, YAML-bomb anchor limits, snake/camel conversion for `check --json` and `parse --json`, TOCTOU race on cache-path `exists()` check. Full list in BACKLOG.md.

---

## v0.9.0 — 2026-04-19

**Theme: coherent failure-handling and intelligent diagnosis.**

When specs fail to parse, every seam of the tool used to lie in a different way: the coverage command swallowed JSON output, the VS Code sidebar pointed at `specter init` (wrong state), the Insights panel claimed "All specs passing ✓" on top of 17 broken files, and `specter doctor` printed 20 identical error lines that together named a schema mismatch nobody could see. v0.9.0 fixes the whole pipeline end-to-end.

The trigger was a real workspace: `kensa-go` specs were written against the pre-v0.6.5 schema, and every tool in the suite disagreed about what that meant.

### Breaking changes

- **`specter coverage --json` now always emits a CoverageReport**, including when specs fail to parse. Exit code (not the presence/absence of JSON) signals pass/fail. Previous behavior: no JSON on parse error, tools had no structured data to work with. Any programmatic consumer that relied on "no JSON = failure" needs to check `exit_code` instead.

### Added

#### CLI (`cmd/specter`, `internal/coverage`)

- **`parse_errors` field** on `CoverageReport` — per-file schema violations (file, path, type, message, line, column).
- **`parse_error_patterns` field** — errors grouped by `(type, path)` sorted by count descending. Enables one-sentence drift diagnosis: "20 specs: missing `objective` at `spec.objective`" instead of 20 individual messages.
- **`spec_candidates_count` field** — count of `.spec.yaml` files on disk before any parse was attempted. Distinguishes "no specs exist" from "specs exist but drift."
- **`spec_file` field** on each entry — path to the source `.spec.yaml`. Populated by the CLI from discovery; previously not exposed.
- **`specter doctor` pattern analysis** — when the parse check fails, doctor prints a `Pattern analysis:` block that names schema version drift explicitly when every discovered spec hit the same error shape. Heterogeneous errors get a top-N list with counts.
- **`specter init` discovers existing specs** — scans `specs/`, populates `domains.default.specs` from parseable spec IDs, prints a warning with pattern analysis for any that fail. Always emits a `domains:` section with a placeholder default domain when empty (fixes a silent-exclusion footgun where an empty domains map caused `specter sync` to ignore every later spec).

#### VS Code extension

- **Parse errors populate the Problems panel** — each failing spec appears as a clickable `vscode.Diagnostic` entry at the reported line/column, prefixed with the error type (e.g. `[required] field is missing (at spec.objective)`).
- **Mixed-render Coverage sidebar** — passing specs and a "Failed to parse" group render in the same tree. Each failing file is a clickable leaf that opens the file at the reported line. Previously the sidebar was all-or-nothing: tree OR error banner.
- **Click-to-open on tree nodes** — spec nodes open their `.spec.yaml`, test-file leaves open the test file, failing spec leaves open the broken spec. Relative paths from the CLI are resolved against the workspace root.
- **Honest Insights panel** — renders a `Parse failures` section listing each broken file with its error, alongside the normal `Coverage gaps` section. Header reflects the true mixed state ("17 parse error(s), 4 spec(s) parsing cleanly"). The "All specs passing ✓" headline now appears only when it's literally true.
- **Clickable file-path headers** in Insights parse-error cards — webview posts an `{openFile, line}` message to the extension host, which opens the file.
- **`specter.revealInTree` command wired end-to-end** — takes the active editor's file and reveals the matching node in the Coverage sidebar. Previously declared in `package.json` but never registered, surfacing as "command 'specter.revealInTree' not found."
- **Honest `specter.runSync` completion toast** — info-level success vs warning-level "finished with errors in N folder(s)" with a "Show Output" button.
- **`@ac` hover populates covering files** from the live CoverageReport instead of always rendering as "uncovered" (latent UX regression).
- **Annotation extractor respects multi-line string literals** — `// @spec` inside a TypeScript template literal, Go raw string, or Python triple-quoted string is no longer treated as a real annotation.
- **Sidebar message names schema drift** when the pattern signature is unambiguous ("Every one of N .spec.yaml files hit the same failure: **required** at `spec.objective`").

### Fixed

- **Latent runtime bug: `entry.specID` was always undefined at runtime.** The VS Code types declared camelCase (`specID`, `coveragePct`, `parseErrors`) but the CLI emits snake_case JSON. A new `snakeToCamelCoverage` converter in the client layer handles the mapping; every downstream consumer now sees the shape its types promise.
- **Defensive guards against null arrays** — Go's `omitempty` emits `null` for empty slices, so `entry.coveredACs` could be `null` at runtime. Hardened every site that iterates entries/ACs/test files/parseErrors.
- **Insights panel crashed with `entries is not iterable`** when parses failed (`entries` was `null`).
- **Template-literal annotation bleed** — a `// @spec foo` mentioned inside a template literal (typical test-fixture content) no longer registers as a real annotation.
- **Annotation regex anchored to line start** — a prose comment that happened to quote `// @spec other-spec` no longer hijacked the surrounding `currentSpecID`. Caught when spec-coverage's own regression tests described string-literal handling.

### Spec bumps

- `spec-coverage`: 1.4.0 → **1.6.0** (C-10/AC-10 always-emit contract; C-11/AC-11 string-literal safety; C-12/AC-12 `spec_candidates_count`; C-13/AC-13 `parse_error_patterns`)
- `spec-doctor`: 1.0.0 → **1.1.0** (C-09/AC-09 pattern analysis + drift diagnosis)
- `spec-manifest`: 1.4.0 → **1.5.0** (C-16/AC-22 ScaffoldManifest always emits `domains:` section)
- `spec-vscode`: 1.1.0 → **1.2.0** (AC-29 rewritten; AC-30 no-specs-yet; AC-31 honest runSync toast; AC-32 hover populates coveredByFiles; AC-33 click-to-open; AC-34 Problems-panel plumbing; AC-35 drift diagnosis in sidebar; AC-36 mixed-render tree; AC-37 honest Insights; AC-38 revealInTree; AC-39 clickable Insights file headers)

All 14 specs dogfood at 100% AC coverage. 192 TypeScript tests pass. All Go tests pass under Go 1.25.8 + golangci-lint v2.6.2.

---

## v0.8.3 — 2026-04-18

### Fixed

- **`specter resolve --dot` and `specter resolve --mermaid` polluted stdout with a plain-English footer** (`No dependency issues found.`) after the structured output block. Piping to `dot -Tpng` or Mermaid renderers failed to parse. Fix: suppress the footer when `--dot`, `--mermaid`, or `--json` is set — the successful exit code already signals the no-issues status. Two regression tests added.

### Audit (no changes needed)

Full CLI audit performed, no other flag bugs found:
- `parse --json`, `check --json`, `coverage --json`, `sync --json` — all emit clean structured output, no trailing text
- Exit codes correct: unknown command / missing args / bad flag all exit 1
- `--version` works on root and via `-v`
- `sync --only <phase>` validates against the allowed set
- `init --template <name>` validates against the allowed set, errors on unknown
- `explain <unknown>` errors cleanly
- `diff` no-args errors cleanly (2 positional args required)

---

## v0.8.2 — 2026-04-18

### Fixed

- **Critical: extension passed CLI flags that don't exist.** `SpecterClient` called `specter parse --json --manifest <path>`, `specter check --json --manifest <path>`, `specter coverage --json --manifest <path>`, and `specter diff --json --base <ref> <file>`. None of the `--manifest`, `--spec`, `--base`, or `--json` (on diff) flags exist in the CLI. Every invocation threw "unknown flag" and the try/catch in `runCoverageForFolder` surfaced it as "No coverage data loaded yet" in the sidebar — so users following v0.8.1's fix for the manifest-discovery bug would reload, the extension would find specter.yaml correctly, then fail to run any specter command because of the flag mismatch.

  Fix: strip all fabricated flags. The CLI discovers `specter.yaml` by walking up from cwd, so `execFile` is now called with `cwd: path.dirname(manifestPath)`. Diff uses its actual positional `<path>[@<ref>]` syntax.

- **New integration test suite (`client.test.ts`) invokes the real built CLI binary** against a tmpdir workspace. Would have caught every one of the fabricated flags immediately. Previously all extension tests were unit-level against TypeScript mocks that described intent, not contract.

  GOTCHAS #17 documents the "mocks describe intent, not contract" lesson.

---

## v0.8.1 — 2026-04-18

### Fixed

- **Critical: "no specter.yaml found" when the file IS at the workspace root.** `resolveManifestPath` in the VS Code extension called `path.dirname()` on the workspace folder path before starting its search. `path.dirname("/home/user/project")` returns `/home/user` (the parent), so the resolver searched `/home/user/specter.yaml`, `/home/specter.yaml`, and so on — **never checking `/home/user/project/specter.yaml`** which is the canonical location the docs explicitly recommend. Affected every user since spec-vscode v1.0.

  Fix: `resolveManifestPath` now accepts an optional third argument `isDirectory` so the caller can say "this path IS the starting directory." The single runtime caller (`setupFolder`) supplies a real `statSync().isDirectory()` probe. Two regression tests pin both calling shapes.

  GOTCHAS #16 documents the trap.

  After updating, reload your VS Code window — the Coverage sidebar will populate.

---

## v0.8.0 — 2026-04-18

Followed the project's own SDD workflow: plan → specs first → failing tests → implement → validate → ship.

### Fixed

- **Wrong GitHub URL in `specter init` scaffold.** The header comment pointed at `github.com/Hanalyx/spec-dd` (wrong slug — that's the parent monorepo, not the Specter project). Now correctly emits `github.com/Hanalyx/specter`. spec-manifest C-15/AC-21 pin the canonical URL.

### Added

- **Coverage sidebar state messages.** When the Coverage tree has no data to display (report not yet loaded, or every spec failed parse), the panel now shows a synthetic node with a state explanation and a concrete next step. Previously the panel was silently empty — a dead-end UX. Two states distinguished:
  - *No coverage data loaded yet* — points at `specter init`, `specter reverse`, or `Specter: Run Sync`.
  - *All discovered specs failed to parse* — points at the Problems panel where the parse errors surface.

  spec-vscode C-21/AC-28/AC-29 pin the behavior. Pure `buildCoverageTreeRoot` function in `coverage.ts` carries the decision logic, unit-tested without VS Code mocks.

### Changed

- **Marketplace categories**: `Linters + Testing + Other` → `AI + Linters + Testing`. Drops the uninformative "Other" and adds "AI" to signal the AI-assistant integration use case (Specter's `Copy Spec Context for AI` command, spec-as-contract-for-AI workflow).

### Migration

- No schema changes; no breaking behavior changes. Upgrade is drop-in.

---

## v0.7.1 — 2026-04-18

### Fixed

- **Silent exit on unknown command or bad flag.** Typos like `specter covera` or `specter parse --wrong-flag` previously exited with code 1 and no output. Root cause: `SilenceErrors = true` on the root Cobra command was suppressing both our intentional silent-exit path AND Cobra's own usage errors. Now only the `errSilent` sentinel is truly silent; everything else prints the error message plus a pointer to `specter --help`. Cobra's "Did you mean?" suggestions now surface for near-miss typos.

### Changed

- **"No .spec.yaml files found" message** now explains where specter looked (the specs_dir from specter.yaml, or the default) and lists three concrete next steps (`specter reverse src/`, `specter init --template`, or editing specter.yaml). Previously it was a one-line dead-end.
- **`specter reverse` handoff.** Success output now concretely points at the first generated spec with a step-by-step triage walkthrough: `specter explain <spec-id>`, triage gaps, `specter parse`, `specter sync`. Previously it said "review each gap AC" without telling you where to start.
- **Parse-error hints refreshed.** Enum error messages for `status`, `constraint.type`, `constraint.enforcement`, `depends_on.relationship`, and `changelog.type` were out of date (listed old values or missing new ones). Added hints for `tier`, `constraint.validation.rule`, and a special case for `context.*` unknown-field errors that explains the v0.7.0 tightening and gives three remediation options.

### Docs

- **`docs/RELEASE_PLAN.md` archived** to `docs/archive/RELEASE_PLAN.md` with a prominent "stale" notice. Current release status → `CHANGELOG.md`, forward roadmap → `BACKLOG.md`.

---

## v0.7.0 — 2026-04-17

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

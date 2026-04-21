# Specter Backlog

Forward-looking roadmap. Items are grouped by target release. Each item is a single sentence of intent plus a link to the design doc or discussion when one exists.

Current shipped version: **v0.9.2** (published to VS Code Marketplace as stable 2026-04-21).

---

## v0.9.2 — UX polish (shipped)

Published 2026-04-21. Two items from jwtms migration testing; no security or correctness issues.

- **`specter coverage` redesign**: summary header with per-tier breakdown; worst-first sort (failing → partial → 100%, tier desc within each bucket); `--failing` flag to filter the table to sub-100% entries; 40-char truncation for long spec IDs (JSON output unchanged).
- **`specter init --refresh`**: non-destructive manifest update. Refreshes `domains.default.specs` only; preserves `settings`, `registry`, custom domains, tier overrides. `--dry-run` variant. Mutually exclusive with `--force`.
- **Marketplace metadata**: added `Other` category and discovery keywords (`spec`, `sdd`, `spec-driven-development`, ...).

Spec bumps: `spec-coverage` 1.7.0→1.8.0, `spec-manifest` 1.5.0→1.6.0.

---

## v0.9.1 — Post-audit fixes (shipped)

Published 2026-04-19. Derived from `research/SPECTER_AUDIT_2026-04-19.md`.

- **CRITICAL**: mandatory SHA256 checksum verification on binary download (no silent fallback).
- **BLOCKERS**: register `specter.runReverse`, remove `specter.openQuickStart` orphan declaration, CI-enforced package.json ↔ extension.ts command parity test.
- **HIGH**: fresh-install binary resolution, reachable walkthrough, `driftDecorationType` disposal, on-type + drift-scan error surfacing, Go `[]` not `null` emission.
- **Internal**: `specter.insertAnnotation` → `specter._insertAnnotation` (VS Code community convention for internal commands).

Spec bumps: `spec-coverage` 1.6.0→1.7.0, `spec-vscode` 1.2.0→1.3.0.

---

## v0.9.0 — Coherent failure-handling & intelligent diagnosis (shipped)

Published to Marketplace 2026-04-19 as stable. Covers:

- **B1 fix**: `specter coverage --json` always emits a CoverageReport, including on parse failure (new `parse_errors` field). Extension reads this reliably in every state.
- **H1 fix**: VS Code `specter.runSync` emits an honest completion toast that reflects success vs failure, no more unconditional "Specter sync complete."
- **H3 fix**: `@ac` hovers in test files populate `coveredByFiles` from the live CoverageReport instead of always rendering as "uncovered."
- **M8 fix**: annotation extractor respects multi-line string literals (backtick, triple-quote) so `// @spec` inside a template literal is no longer hijacked.
- **Intelligent drift diagnosis**: `parse_error_patterns` + `spec_candidates_count` let consumers name "every discovered spec hit the same `required` error at `spec.objective`" as schema drift in one sentence. Surfaced in `specter doctor` output and the VS Code sidebar message.
- **`specter init` discovers existing specs**: populates `domains` from parseable specs; always emits a `domains:` section (fixes silent-exclusion footgun); prints parse-error pattern analysis when specs fail.
- **VS Code Problems panel plumbing**: parse errors pushed as per-file `vscode.Diagnostic` entries — clickable, positioned at line/column.
- **Sidebar mixed-render**: passing specs and a "Failed to parse" group render together. Each failing file is a clickable leaf. Previously all-or-nothing.
- **Click-to-open**: spec nodes and test-file leaves open the underlying file at the reported line.
- **Honest Insights panel**: parse-failures section + coverage-gaps section; header text reflects true mixed state; file-path headers in parse cards are clickable.
- **`specter.revealInTree`**: wired end-to-end (previously declared in `package.json` but never registered).
- **snake_case → camelCase shape conversion**: latent runtime bug where `entry.specID` returned undefined — the VS Code types declared camelCase but the CLI emits snake_case.

Spec bumps: `spec-coverage` 1.4.0→1.6.0, `spec-doctor` 1.0.0→1.1.0, `spec-manifest` 1.4.0→1.5.0, `spec-vscode` 1.1.0→1.2.0.

---

## v0.8.x prerequisites / blocking future releases

- **`@vscode/test-electron` headless integration tests.** The release-gate currently relies on a human operator reproducing changes in a live VS Code window. Automating that via `@vscode/test-electron` would let CI spawn a real VS Code instance with the extension loaded against fixture workspaces and assert the sidebar / status bar / output channel behave as expected. Backstops the human gate; does not replace it. About a day of setup.
- **Go toolchain bump (1.22 → 1.23+).** ✅ Done in v0.8.3. Clears 5 stdlib CVEs under `govulncheck`. Now at Go 1.25.8 + golangci-lint v2.6.2.

---

## v0.10 — Migration tooling + CI-gated coverage quality (candidate)

The v0.9.0 work made schema drift *visible* via intelligent diagnosis. v0.10 should make it *fixable* without hand-editing, and make the coverage gate resistant to two failure modes currently silent: skipped tests counting as covered, and failing-but-annotated tests counting as covered.

### Migration tooling

- **`specter migrate` command.** Given specs from an older schema version, apply known-safe rewrites: strip removed fields (`trust_level`), rename renamed fields, update enum values, move root-level blocks under `spec:` (jwtms pattern). Dry-run by default; `--apply` writes changes. Seed with the v0.6.5 `trust_level` removal, the v0.7.0 field renames, and the jwtms v1 shape. See `research/JWTMS_SPECTER_REASSESSMENT_V0.9.md` for the driving design case.
- **VS Code quick-fix for removed fields.** Lightbulb action on a parse error like `Unknown field 'trust_level'` → "Remove deprecated field." Applies to the one file; `Fix all in workspace` batches across every failing spec. Pairs with `specter migrate` for the CLI path.
- **Schema-version metadata.** Record the schema version in each spec (`spec.schema_version`) so `specter migrate` can target known old versions instead of inferring from failure patterns. Optional field with sensible default.
- **`specter show <spec-id>`** — human-readable spec card assembled from existing coverage JSON. Shows tier, coverage %, test files covering each AC, uncovered ACs with descriptions. Closes the "where do I look to verify this spec?" gap for test files without waiting on source-annotation scanning. No new data collection — pure presentation over `specter coverage --json`. Small scope, ~2-3h.

### CI-gated coverage quality (test-results ingestion)

Today, `specter coverage` counts an AC as "covered" if any test file has a `// @ac AC-NN` annotation for it. This silently mis-reports in three shapes:
- A test with `it.skip(...)` + the annotation reads as "covered" — skipped tests claim coverage.
- A test that now fails but still has the annotation reads as "covered" — regressions slip through the gate.
- A flaky test that's intermittently failing reads as "covered on runs where it passed, uncovered on runs where it failed" — noise in the gate.

v0.4 shipped pass-rate-aware coverage for Tier 1 via a `.specter-results.json` file — but writing that file is manual, so adoption is near-zero. v0.10 makes CI-gated coverage quality a first-class, runner-agnostic feature.

**Design — two-stage ingest:**

- **`specter ingest` (new command).** Consumes CI-native test output formats and writes `.specter-results.json`. Initial flavors: JUnit XML (vitest / jest / pytest / playwright), `go test -json`. Fast-follow: TAP.
  - `specter ingest --junit 'test-results/*.xml' --output .specter-results.json`
  - `specter ingest --go-test test-output.json --output .specter-results.json`
  - Keeps JUnit parsing out of `specter coverage`'s hot path — coverage stays fast and deterministic.
- **Extended `.specter-results.json` schema.** Status enum: `passed | failed | skipped | errored | flaky`. Reserves space for flake handling without retrofitting.
- **`specter coverage --strict` (new flag).** Treats any non-`passed` annotated AC as uncovered, regardless of tier. `--strict` with no results file is a hard fail ("no test results found, can't verify coverage"). Non-strict coverage keeps today's behavior: annotation-only for Tier 2/3, pass-rate-aware for Tier 1 when results exist.

**CI wiring — downstream job consumes test artifacts:**

```yaml
specter-coverage:
  needs: [test, integration-test, specter]
  steps:
    - uses: actions/download-artifact@v4
      with: { pattern: test-results-*, merge-multiple: true, path: test-results/ }
    - run: specter ingest --junit 'test-results/*.xml' --output .specter-results.json
    - run: specter coverage --results .specter-results.json --strict
```

Blocks on test completion (~30s cost for jwtms's 250s integration suite). Unit + integration jobs emit JUnit via `--reporter=junit`; Specter reads the merged artifact.

**Open design question — flakes.** The proposal to "add `--retry 2` on test jobs" is a workaround that hides legitimate regressions. Better answer (deferred to v0.11): test runners distinguish flakes from deterministic failures in the results file; `--strict` tolerates `flaky`, `--deny-flaky` fails hard on them. Ship v0.10 with only `passed/failed/skipped/errored`; revisit flake handling when real patterns surface from v0.10 usage.

**Design discussion**: see the thread in session notes (2026-04-20) — the three design tradeoffs (two-stage vs one-stage ingest, JUnit flavor handling, missing-results behavior under `--strict`) are resolved in the bullets above. Flake handling deferred.

**Scope**: ~2 days for the `specter ingest` command with JUnit + go test flavors, `--strict` semantics on coverage, extended results-file schema. Spec bumps: new `spec-ingest`, `spec-coverage` 1.8.0 → 1.9.0.

---

## v0.11 — AI loop enforcement (candidate)

The CI gate (`specter sync`) already enforces annotated tests must exist. This phase makes the loop *proactive* rather than reactive — close the spec → test → implement → eval cycle for AI coding assistants. Items retrieved from the pre-v0.3 `docs/IMPROVEMENT_ROADMAP.md` (local, gitignored); they were Phase 5 in that doc and remain unshipped.

- **`specter context`** — generates AI-tool-specific instruction files from current specs so the AI reads and respects the spec before generating code:
  - `specter context --format claude` → updates/creates `CLAUDE.md` with current spec summaries, AC list, tier constraints
  - `specter context --format cursor` → writes `.cursor/rules` with spec constraints formatted as Cursor rule blocks
  - `specter context --format copilot` → writes `.github/copilot-instructions.md`
  - `specter context --format all` — one-pass generation
  - `specter context --spec <id>` — scope to a single spec for focused AI sessions
  - Output covers tier, objective, constraints, ACs with descriptions, current coverage status, uncovered ACs highlighted
  - Idempotent: re-running updates the context section without clobbering manual additions
  - `specter sync --update-context` flag regenerates context files as part of the sync pipeline

- **Pre-push hook integration** — `specter hook install` writes a git pre-push hook that:
  - Blocks pushes where implementation files changed but no corresponding `@spec`/`@ac` annotation was added or updated in the diff
  - Reports which specs are affected and which ACs have no test annotation in the changeset
  - Bypass with `git push --no-verify` (documented, discouraged)

- **`.specter-results.json` test runner adapters** — first-party adapters that write pass/fail results automatically so the pass-rate-aware coverage loop (already implemented for Tier 1 in v0.4) closes end-to-end without manual results-file maintenance:
  - Go: `go test -json | specter results ingest`
  - pytest: `pytest --specter` plugin
  - Jest: `jest-specter` reporter

---

## Audit items still pending (from `research/SPECTER_QUALITY_AUDIT.md`)

- **H4 — Status-bar error differentiation.** Today `Specter: error` says the same thing for "CLI not found," "17 parse errors," "coverage below threshold." Split these into distinct status-bar text + tooltip. Low effort, modest polish.
- **H5 — `specter reverse --dry-run` has no CLI-level test.** Add `TestReverse_DryRun_PrintsWithoutWriting` in `cmd/specter/reverse_test.go`.
- **M1/M3** — `spec-sync` phase-result assertions are too loose.
- **M2** — `spec-resolve` AC-08 Mermaid output tested at CLI layer only.
- **M4** — `spec-doctor` C-08 vs skip-coverage-on-parse-error conflict.
- **M5** — `spec-explain` annotation examples use inconsistent naming across languages.
- **M6** — `spec-check` AC-03 structural-conflict detection uses fragile keyword matching.
- **M7** — `spec-coverage` AC-01 float assertion looser than rounding contract.
- **LOW-tier** — several test-fidelity gaps where tests check *that* something happened but not *what*. Batch into a "test hardening" PR.

---

## v0.8+ / unscheduled — deferred from earlier proposals

Each needs its own design doc before scheduling:

- **Annotation-based source-file tracking.** Extend `@spec` annotations from test files to source files; new `specter specs governing <path>` command; coverage output carries a derived `source_files` array. Opt-in via `specter.yaml` setting. Rationale: single source of truth, matches existing test-coverage model, zero drift class.
- **Generalize `generated_from` to `provenance` with a `governs: [string]` list** — overlaps semantically with `depends_on`, needs careful design to avoid muddling "spec depends on spec" with "spec governs file." May be obsoleted by the annotation-based approach.
- **Optional `contracts` section for HTTP APIs** — Specter's mission is framework-agnostic; HTTP specialization is a commitment. Better as an adapter/extension than core schema.
- **Derived `callers` via `specter graph --callers-of <spec-id>`** — no schema change; derivable from existing `depends_on` graph. Low-cost feature.
- **Per-rule narrowing of `constraint_validation.value`** — constrain value type based on `rule` (e.g., `rule: "min"` implies numeric value). Field is write-only today; defer until someone consumes it.

---

## Open adoption-friction items

Not schema-scoped; move to a specific release when picked up:

- **Zero-state and bare-command UX** — `specter` with no args shows help; "no specs found" messages explain what was searched and suggest `init` / `reverse`. (v0.9.0 improved this for the sidebar; CLI still has gaps.)
- **Parse-error hint map** — common pattern violations include an example of the correct form. Partially addressed in v0.9.0 via drift-pattern detection; per-error hints still missing.
- **Reverse compiler handoff** — success output points users at `specter explain <spec-id>` for gap triage.
- **Docs consolidation** — merge QUICKSTART into README, keep GETTING_STARTED as deep-dive, archive stale RELEASE_PLAN.

# Specter Backlog

Forward-looking roadmap. Items are grouped by target release. Each item is a single sentence of intent plus a link to the design doc or discussion when one exists.

Current shipped version: **v0.8.3**. Pre-release in flight: **v0.9.0-dev.0** on `release/v0.9.0`.

---

## v0.9.0 — Coherent failure-handling & intelligent diagnosis (pre-release)

Shipped on `release/v0.9.0`. Awaiting publish. Covers:

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

## v0.10 — Migration tooling (candidate)

The v0.9.0 work made schema drift *visible* via intelligent diagnosis. v0.10 should make it *fixable* without hand-editing:

- **`specter migrate` command.** Given specs from an older schema version, apply known-safe rewrites: strip removed fields (`trust_level`), rename renamed fields, update enum values. Dry-run by default; `--apply` writes changes. Seed with the v0.6.5 `trust_level` removal and the v0.7.0 field renames.
- **VS Code quick-fix for removed fields.** Lightbulb action on a parse error like `Unknown field 'trust_level'` → "Remove deprecated field." Applies to the one file; `Fix all in workspace` batches across every failing spec. Pairs with `specter migrate` for the CLI path.
- **Schema-version metadata.** Record the schema version in each spec (`spec.schema_version`) so `specter migrate` can target known old versions instead of inferring from failure patterns. Optional field with sensible default.

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

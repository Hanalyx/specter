# Specter Backlog

Forward-looking roadmap. Items are grouped by target release. Each item is a single sentence of intent plus a link to the design doc or discussion when one exists.

Current shipped version: **v0.12.0** (CLI tagged 2026-04-30 at commit `97df6e3`; VS Code extension VSIX built locally, awaiting manual Marketplace gate per `specter/CLAUDE.md`). Past release notes live in [CHANGELOG.md](CHANGELOG.md) — this file is forward-only.

Working branch: **none open yet**. The next cycle (`v0.12.1` or `v0.13`) will be cut from `main` when scope locks. Per `CONTRIBUTING.md` → Branch workflow, all feature / fix / doc PRs during a cycle target the working branch, not `main`.

The v0.12 cycle delivered: `doctor --fix` BETA migration tooling (7 SDD sub-cycles), `init` writes `schema_version: 1`, GH #77 language-aware `explain`, GH #80 source-only diagnostic hint, GH #93 `doctor` no-manifest discovery alignment, VS Code quick-fix for removed fields, full M-tier supply-chain hardening (M1/M2/M4/M5/M6/M7/M8), and the SSRB framework for schema-change decisions. Three release-infra hotfixes (#116, #117, #118) post-cycle to fix landmines in the M6/M7 first-run paths. v0.12.0 shipped 2026-04-30T02:24:21Z with 16 release artifacts including sigstore-signed checksums (keyless OIDC) and CycloneDX SBOMs.

---

## v0.12.1+ — TBD scope

Open candidates (not yet committed to a cycle):

- **GH #101 — `specter doctor --diff <baseline>`**. Deferred from v0.12; standalone feature work rather than bundled with `doctor --fix`. ~1 day of work; reads existing `doctor --json` output, emits set differences. Useful for iterative migration-script development.
- **Pre-flight gate (P1)** — see "Release-time pre-flight gate (P1)" section below; promoted from P3 after three landmines surfaced this cycle.
- **VSIX packaging hygiene** — `junit.xml` (jest-junit output, 42 KB) is being packed into the VSIX. Add to `.vscodeignore`. P3, low-effort.
- **Post-v0.12-review polish (P2/P3)** — see existing section below.

---

## v0.12 — shipped 2026-04-30 (archived for reference)

Two themes delivered: shipped the migration tooling parked since v0.10 (doctor --fix, schema_version, VS Code quick-fix) so JWTMS-style schema drift is fixable in-place without GH #96, and folded the M-tier security hardening into the release.

### CLI features

- **`specter doctor --fix`** (parked on `feat/doctor-fix`). Apply known-safe rewrites to specs from older schema versions: strip removed fields (`trust_level`), rename renamed fields, update enum values, canonicalize manifest. Dry-run by default; `--fix` writes changes. Seeds: v0.6.5 `trust_level` removal, v0.7.0 field renames, jwtms v1 shape. Pairs with GH #93 (no-manifest discovery alignment with `parse`) and GH #101 (`--diff <baseline>` for delta-only error reporting). `spec-doctor` 1.1.0 → 1.2.0.
- **`specter init` writes `spec.schema_version: 1`** (parked on `feat/schema-version-manifest`). Records schema version per project so `doctor --fix` can target known old versions instead of inferring from failure patterns. Activates the schema-stability policy below. `spec-manifest` 1.6.0 → 1.7.0.
- **GH #77 — language-aware `specter explain`**. When `discoverTestFiles` returns at least one `.py` file, the annotation example uses `# @spec`/`# @ac` (and the autouse-fixture pattern), not `// @spec`. `spec-explain` 1.1.0 → 1.2.0. Detail in `V0_12_PYTHON_FOLLOWUP_PLAN.md` Item 1.
- **GH #80 — source-only diagnostic hint under `--strict`**. When an annotated AC has source-file annotations but no matching `.specter-results.json` entry, emit a per-AC stderr hint pointing the reader at the missing-runtime-channel cause. `spec-coverage` 1.11.0 → 1.12.0. Detail in `V0_12_PYTHON_FOLLOWUP_PLAN.md` Item 2.
- **GH #101 — `specter doctor --diff <baseline>`**. Takes a previous `doctor --json` output and reports only the deltas. Universal iterative-DX feature; folds into the `feat/doctor-fix` PR.

### CLI fixes

- **GH #93 — `doctor` no-manifest discovery mismatch**. Pre-existing v0.9.0+. `specter doctor` returns "no specs" when run without `specter.yaml` even though `parse` discovers nested `*.spec.yaml` recursively. Align `doctor`'s no-manifest fallback. Folds into the `feat/doctor-fix` PR.
- **M1 — input size cap on `.specter-results.json`** (16 MiB) in `internal/coverage/results.go`. Pre-staged on `chore/v0.12-security-hardening`.
- **M2 — input size cap on `specter.yaml`** (64 KiB) in `internal/manifest/manifest.go`. Pre-staged on `chore/v0.12-security-hardening`.

### VS Code extension features

- **VS Code quick-fix for removed fields** (parked on `feat/vscode-quick-fix`). Lightbulb action on `Unknown field 'X'` parse errors → "Remove deprecated field" (per-file) and "Fix all in workspace" (batch). Pairs with `doctor --fix` for the CLI path. `spec-vscode` 1.3.0 → 1.4.0.

### VS Code extension fixes

- **M4 — webview CSP** with per-render nonce in `vscode-extension/src/extension.ts`. Defense-in-depth against future `escapeHtml` regressions. Pre-staged on `chore/v0.12-security-hardening`.
- **M8 — jest-junit ^16 → ^17** (dev dep). Existing inline reporter config means no breaking impact. Pre-staged on `chore/v0.12-security-hardening`.

### Release infrastructure

- **M5 — GHA SHA-pinning + dependabot config**. All workflow actions SHA-pinned with version-comment preservation; `dependabot.yml` resolves and bumps both. Pre-staged on `chore/v0.12-security-hardening`.
- **M6 — sigstore cosign keyless signing + CycloneDX SBOM** via goreleaser. `-trimpath` + reproducible `mod_timestamp`. `release.yml` installs `sigstore/cosign-installer` and `anchore/sbom-action/download-syft` (both SHA-pinned). Pre-staged on `chore/v0.12-security-hardening`.
- **M7 — `release.yml` chained on Pre-Release Test Suite** via `workflow_run`. Concurrency guard. `id-token: write` for sigstore OIDC. Drops the redundant test job. Pre-staged on `chore/v0.12-security-hardening`.

### Open scope decisions

Three items the BACKLOG previously listed as v0.12 candidates whose inclusion is not yet confirmed for this cycle. Recommend deferring all three to v0.13 to keep v0.12 focused on migration tooling + hardening.

- **`init --ai claude --with-hooks`** — `PreToolUse` matcher on `Edit`/`Write` for `internal/**/*.go` requiring matching spec was Read in the same session; `PostToolUse compact` checklist. Claude-only. Scope estimate: 3–4 days, mostly session-scoped tracker integration testing. Codex / Cursor / Gemini have no hook equivalent.
- **`unreachable_annotation` — source-only annotation detection**. Deferred from v0.11's `check --test`. Requires a real per-language test-file parser (line regex isn't enough) to correlate `// @spec`/`// @ac` source comments with runner-visible test names.
- **`pytest-specter` plugin** (V0_12_PYTHON_FOLLOWUP_PLAN.md item 3). Separate Python package on PyPI; ~3 days; recommend separate-repo cadence so PyPI release lifecycle doesn't couple to Specter's release cadence.

### Deferred to v0.13+

- **GH #96 — `specter migrate` for non-Specter dialects.** Pluggable `--from=<dialect>` registry + `map.yaml` field mapping. JWTMS migration is the driving case (1900 ACs, 515 constraints), but the framework generalizes — universal adoption tooling. Substantial; sized for its own cycle.

### Rejected (closed not-planned 2026-04-26)

Detailed rationale lives on each issue thread. Brief summary:

- **GH #97 — `generated_from.source_files` plural array.** Single-project pain (JWTMS migration), and "match `test_files` shape" is a symmetry argument, not user-friction. Migration use case absorbed by GH #96 when it lands.
- **GH #98 — AC-level lifecycle `status` field.** Proposed enum overlaps with existing `coverage` and `approval_gate` semantics, creating three competing answers. Product-state-only scope might warrant a fresh issue if a second project surfaces the same pain.
- **GH #99 — coverage inference from `generated_from.test_files`.** Migration-only pain that contradicts the v0.10 mechanical-coverage design call. Migration tool (GH #96) should backfill annotations on import, not coverage soft-infer.
- **GH #100 — `spec.kind: audit-matrix`.** Cross-cutting pattern is real, but polymorphic `spec.kind` is a heavyweight schema commitment when lighter mechanisms (reverse linking via `governs:`, tags + queries, external tracker files) cover the use case at much lower cost.

### Post-v0.12-review polish (P2/P3 follow-ups)

Surfaced by the 2-agent review of the v0.12 cycle (2026-04-28) and the re-review (2026-04-29). Severity-tagged. None block merge — the merge-blocking P1s (block-scalar corruption, AC-52 grep regex, BETA-gate non-TTY refusal, goreleaser SBOM template field) shipped on `feat/doctor-fix-v2` 1.8.0, `feat/vscode-quick-fix-v2` 1.6.0, and `chore/v0.12-security-hardening`. The items below are remaining spec-test discipline tightening that can ride a follow-up patch or land before v0.12 tag if convenient.

Closed by the 2026-04-29 fix pass:
- ~~AC-16 ParseManifest assertion~~ → closed in `2474dec` on `feat/doctor-fix-v2`.
- ~~AC-31 ordering check~~ → closed in `1742cfe` on `feat/coverage-source-only-hint` with `hintIdx < tableIdx` strings.Index comparison.
- ~~AC-43 byte-unchanged claim~~ → closed in `fc807ae` on `feat/schema-version-manifest-v2` with parameterized (1, 7, 42) sub-tests asserting verbatim line preservation.
- ~~M1/M2/M4 aspirational test coverage~~ → closed in `6813c67` on `chore/v0.12-security-hardening` (size-cap tests for results.json + manifest, source-grep CSP test for the webview).
- ~~M5 ci.yml setup-node SHA pin~~ → closed in `640c995`.
- ~~M6 SBOM format mismatch~~ → closed in `b536cab` (CycloneDX) and `6debbbd` (correct `$document` shell-style syntax — `{{ .Document }}` is not a goreleaser template field and would crash at release time).
- ~~BETA gate non-TTY-with-content bypass~~ → closed in `391a759` via TTY-detection refusal with `os.Stdin.Stat() & os.ModeCharDevice` check before reading stdin content; spec-doctor 1.7.0 → 1.8.0 + AC-29.

Open:

- **P2 — AC-25 decline-aborts wiring is unverified end-to-end.** The cycle-7 refactor moved `confirmFixWithUser` to a unit-testable signature (`io.Reader`, `isTTY bool`, `io.Writer`) and tests AC-25 directly with `isTTY=true` + decline content. But the CLI integration that turns `proceed=false` into `Aborted. No files modified.` printed to stdout has no end-to-end test. Risk is low (the wiring is one `if !proceed { fmt.Println(...) }`) but the spec sentence is not mechanically enforced.
- **P3 — `--yes` / `-y` doc parity.** spec-doctor C-16 mandates both the long flag and the short alias. Consider a parity test (similar to commands.test.ts) that checks `cmd/specter/main.go`'s flag declaration matches `docs/CLI_REFERENCE.md` for the doctor command.
- **P3 — `internal/migrate/rewrite.go` package comment line says "C-10" should say "C-11"** (the rewrite-table constraint, not the discovery-fallback one). One-character fix.
- **P3 — `spec-doctor` 1.3.0 changelog narrative groups AC-14 under C-13**, but AC-14's `references_constraints` is `["C-07"]` (read-only-by-default regression guard, not the summary). Cosmetic mismatch.
- **P3 — `coverage --strict --json` exits 0 when uncovered**, but text mode exits 1 on the same input. Possibly intentional (json-as-data-extraction), but inconsistent and surprising for CI consumers. Pre-existing; verify intent and either align or document.
- **P3 — `.specter-results.json` accepts `"status": "pass"` (vs the canonical `"passed"`) and silently treats it as not-passed.** No diagnostic for the typo. Pre-existing footgun; add a strictness-mode warning when status values fall outside the documented enum.

### Release-time pre-flight gate (P1 — promoted from P3)

The v0.12.0 release surfaced **four release-infra landmines** end-to-end, all of which would have been caught by a single `goreleaser release --snapshot --skip=publish --clean` smoke job in CI:

1. **`cyclonedx-json={{ .Document }}` template field** — caught pre-merge by agent re-review (config inspection only). Goreleaser would have crashed at release time with `template: invalid: map has no entry for key "Document"`. Fixed in `6debbbd` before the tag push.
2. **`release.yml` `branches: [main]` filter excluded tag refs** — caught only after the tag push when `workflow_run` events list returned `[]`. M7 chain never fired. Fixed in PR #116.
3. **Cosign `--new-bundle-format=false` flag silently ignored** — caught only after firing release dispatch and observing the same error twice in a row. Cosign 2.5+ deprecated the legacy two-file output and the negation flag isn't honored. Superseded by #4.
4. **Migrate to new bundle format** — fixed by switching to `--bundle=${signature}` in PR #118, validated locally with `cosign sign-blob --key cosign.key --bundle ...`.

Cumulative cost: ~1 day of release-pipeline iteration that produced zero user-facing value. Each landmine was fully predictable from config inspection IF we'd been able to run the pipeline before the tag push.

#### Proposed workflow

A new `.github/workflows/release-snapshot.yml` triggered on PRs that touch `**/.goreleaser.y*ml`, `.github/workflows/release*.yml`, or `specter/cmd/specter/main.go` (anything that affects build output). Body:

```yaml
name: Release pipeline pre-flight (snapshot)
on:
  pull_request:
    paths:
      - '.github/workflows/release*.yml'
      - 'specter/.goreleaser.y*ml'
      - 'specter/Makefile'
      - 'specter/go.mod'
      - 'specter/go.sum'
defaults:
  run:
    working-directory: specter
permissions:
  contents: read
  id-token: write   # OIDC for cosign keyless
jobs:
  snapshot:
    name: Goreleaser snapshot (build + archive + SBOM + sign)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<sha>     # SHA-pin all
      - uses: actions/setup-go@<sha>
        with: { go-version-file: specter/go.mod }
      - uses: sigstore/cosign-installer@<sha>
      - uses: anchore/sbom-action/download-syft@<sha>
      - uses: goreleaser/goreleaser-action@<sha>
        with:
          distribution: goreleaser
          version: latest
          args: release --snapshot --skip=publish --clean
          workdir: specter
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - name: Verify signing artifact present
        run: test -f dist/checksums.txt.sigstore.json
      - name: Verify SBOMs present
        run: |
          ls dist/*.sbom.json | wc -l | grep -q '^5$' || (ls dist/ && exit 1)
```

#### Expected catch rate

Each of the 4 landmines from v0.12 would have failed this workflow:

| # | Landmine | Failure mode caught |
|---|---|---|
| 1 | `{{ .Document }}` | goreleaser tmpl error during `release --snapshot` |
| 2 | `branches: [main]` filter | N/A (release.yml not exercised by snapshot) — caught by separate `actionlint` job, see below |
| 3 | `--new-bundle-format=false` ignored | cosign step inside snapshot |
| 4 | Bundle format args | cosign step inside snapshot |

Items 1, 3, 4 caught directly. Item 2 is structural CI-config drift, not goreleaser drift — covered by a separate `actionlint`-style step. Together: 100% of v0.12.0's landmines would have failed CI before merge.

#### Open design questions

- **Cosign keyless on PR runs**: works for PRs from the same repo (OIDC token issued normally). Forks need fallback. Simplest: `--skip=sign` for PRs from forks; `id-token: write` permission auto-degrades.
- **Snapshot timing**: full snapshot is ~1 minute on ubuntu-latest. Doesn't add to PR critical path (parallel with existing CI).
- **Caching**: setup-go cache hit avoids rebuild if go.mod/go.sum unchanged. Snapshot itself rebuilds binaries — 5 platforms × ~5s each = ~25s.

#### Why P1

Landmine count over recent cycles: v0.10 (0), v0.11 (0), **v0.12 (4)**. The pattern correlates with introducing new release-infra features (M6 sigstore, M7 workflow_run chain). Future cycles WILL touch release infra (Homebrew tap is parked in `.goreleaser.yml`, future M-tier items). Without this gate, every such change risks repeating today's iteration cost.

#### Effort

One workflow file (~50 lines) + three SHA pins + one PR. Estimated 1-2 hours including local snapshot validation against current main HEAD to confirm all paths are exercised.

#### Acceptance

- Workflow file lands at `.github/workflows/release-snapshot.yml`.
- Triggered on changes to release-infra paths.
- Successfully runs `goreleaser release --snapshot --skip=publish --clean` against current main.
- A test PR that intentionally breaks `.goreleaser.yml` (e.g., reverts the `$document` fix) fails the workflow.
- BACKLOG entry moves from "candidate" to "shipped" reference once verified.

### Future paths for `doctor --fix` rewrite engine

When real adoption shows the `needs-manual-edit` path (spec-doctor C-15) is hit often, two upgrade routes:

- **Option C — yaml.v3-aware byte-range splice.** Use yaml.v3 to find the exact byte range of the targeted (key, value) entry — start at `keyNode.Line`/`keyNode.Column`, end at the next sibling's start (or document EOF for the last entry). Splice that byte range out of the original content; never call `yaml.Marshal`. Handles every YAML shape correctly AND preserves bytes outside the deletion. Requires ~40 lines of node-walking code and care around edge cases (last-entry-in-mapping, EOF without trailing newline, mixed line endings). Selectable per-rewrite — table entries opt into byte-range mode when their predicate matches a structurally complex value.
- **Option A+ — full `yaml.Marshal` round-trip with diff guard.** Round-trip the document through `*yaml.Node` (preserves comments via `Head/Line/Foot`), with `Encoder.SetIndent(matchOriginalIndent(content))`. After marshaling, diff against the original; if the diff has more changed lines than the deletion target, refuse and fall back to Unhandled. Less recommended than Option C — pays the marshal cost only to throw it away on style normalization.

### Cycle 6 — line-targeted deletion (deferred from feat/doctor-fix-v2)

Surfaced by the 2-agent review of the v0.12 line-span tightening (2026-04-28). One known data-loss bug class remains in the `--fix` rewrite path even with the v1.6.0 line-span check (AC-21/22/23) in place: the **predicate is yaml.v3 node-scoped, but the deletion is regex file-scoped**. Two corruption shapes leak:

- **Case A** — `trust_level:` mentioned inside a string literal (e.g., `summary: |` or `description: |` block scalars containing migration documentation that names the deprecated field). Predicate inspects only the legitimate `spec.trust_level` node; regex `^\s+trust_level\s*:\s*\S.*$` then strips the line inside the documentation string too. Silent corruption.
- **Case B** — `trust_level:` under a non-`spec` mapping (less plausible against current schema, but a hand-edited file with extra keys would over-delete).

Plus two latent concerns of much lower likelihood: `#`/`---`/`...` lines inside multi-line quoted scalars confuse the line-span helper; anchored values (`trust_level: &anchor high`) deleted without checking for `*anchor` references elsewhere.

**Recommended fix: cycle 6 with line-targeted deletion.** Estimated 30–45 minutes; spec-doctor 1.6.0 → 1.7.0 with one new constraint (or extension to C-15) and AC-24/AC-25 covering the two corruption shapes plus regression guards. Concretely:

1. Have `canSafelyStripTrustLevel` return both the verdict AND the list of `keyNode.Line` values for legitimate matches across all documents.
2. Replace the regex line-walk in `stripTrustLevel` with index-based deletion: strip exactly the source-line indices yaml.v3 reported for `spec.trust_level` keys, leave everything else byte-preserved.
3. Add an early refusal when `val.Anchor != ""` (closes the alias concern).
4. New tests: corruption-shape regressions for Case A (string-literal mention) and Case B (sibling mapping); regression guard that the legitimate single-key case still rewrites cleanly.

Until cycle 6 ships, `doctor --fix` is gated as **beta** (interactive y/N confirmation; `--yes` to bypass for CI; `--dry-run` exempt). The beta gate keeps the existing predicate's safety net visible to operators while the regex/predicate scope mismatch is being closed.

---

## Audit items still pending (from `research/SPECTER_QUALITY_AUDIT.md`)

- **H4 — Status-bar error differentiation.** Today `Specter: error` says the same thing for "CLI not found," "17 parse errors," "coverage below threshold." Split these into distinct status-bar text + tooltip. Low effort, modest polish.
- **H5 — `specter reverse --dry-run` has no CLI-level test.** Add `TestReverse_DryRun_PrintsWithoutWriting` in `cmd/specter/reverse_test.go`.
- **M1/M3** — `spec-sync` phase-result assertions are too loose.
- **M2** — `spec-resolve` AC-08 Mermaid output tested at CLI layer only.
- **M4** — `spec-doctor` C-08 vs skip-coverage-on-parse-error conflict.
- **M5** — `spec-explain` annotation examples use inconsistent naming across languages.
- **M6** — `spec-check` AC-03 structural-conflict detection uses fragile keyword matching.
- **M7** — `spec-coverage` AC-01 float assertion looser than the rounding contract.
- **LOW-tier** — several test-fidelity gaps where tests check *that* something happened but not *what*. Batch into a "test hardening" PR.

---

## Carry-overs from the pre-v0.3 roadmap (still unshipped)

Items from the local `docs/IMPROVEMENT_ROADMAP.md` that haven't landed yet:

- **Developer-friendly parse-error messages.** v0.9.0's pattern analysis names the *shape* of drift at the report level; per-error friendly messages (e.g., "Constraint ID 'c01' is invalid — must match pattern C-01, C-02, etc.") are still raw JSON-Schema paths.
- **Dangling-reference "did you mean?" suggestions.** `error: "handler-interface" does not exist` should include Levenshtein-distance closest match + a suggested fix path (file + `id:` to create). Kensa's original ask.
- **`specter reverse` summary report.** After `reverse` runs, print: *"Found 14 constraints, 23 assertions, 5 gaps. 3 files need your attention."* Today the output is raw YAML dumps.
- **Spec-writing guide links in error output.** Orphan-constraint and unmapped-AC errors should link to the annotation guide / relevant docs.

---

## Infrastructure follow-ups

- **`@vscode/test-electron` headless integration tests.** The release-gate currently relies on a human operator reproducing changes in a live VS Code window. Automating that via `@vscode/test-electron` would let CI spawn a real VS Code instance with the extension loaded against fixture workspaces and assert the sidebar / status bar / output channel behave as expected. Backstops the human gate; does not replace it. About a day of setup.
- **PR comment integration** (Phase 3 carry-over) — show spec coverage diff in PR comments (AC added/removed, coverage delta by tier). Pairs with the `specter-sync-action`.
- **Glob patterns in `settings.exclude`** — the exclude list currently matches by directory name only. Extend to support glob patterns so teams can write `- .claude/**` or `- **/worktrees` without enumerating every root-level directory.
- **CLI docs parity tests.** Three cases of "docs asserted behavior the code didn't implement" shipped during v0.10.x — BUG-2 (`--junit` glob claim) and BUG-3 (`approval_gate` enforcement claim) among them. Reviewer attention isn't sufficient; the reviewer shares the writer's mental model. Mechanize the check:
  - Parse the flag table in `docs/CLI_REFERENCE.md` for each command.
  - Compare against `cobra`'s registered flags on that command at test time.
  - Fail when they diverge — either the docs mention a flag not registered, or a registered flag isn't documented.
  - Same discipline for `docs/SPEC_SCHEMA_REFERENCE.md` vs `internal/parser/spec-schema.json` field descriptions.
  
  Matches the "parity tests over promises" principle in `specter/CLAUDE.md`. Complements the human docs-review policy in the root CLAUDE.md — policy catches authorial drift, parity tests catch mechanical drift.

---

## Policy decisions (activate when prerequisites ship)

### Schema stability policy

The spec schema is considered **draft during v0.x**. `schema_version: 1` is the placeholder value for pre-1.0 projects — the integer does not move during the v0.x series. Breaking schema changes are allowed under the pre-1.0 "no stability promise" convention, and `specter doctor --fix` absorbs them via inference over drift patterns.

**At Specter v1.0.0**, the then-current schema shape becomes the canonical `schema_version: 1` permanently. Subsequent breaking schema changes bump the integer (`2`, `3`, …) and MUST ship a migration path via `doctor --fix`.

Rationale: Specter is a type system for specs. Schema stability is a user-trust contract, and pre-1.0 is the window to iterate freely before making that contract. `schema_version` lives in `specter.yaml` (project-level), not in every spec file.

**Status**: activates with v0.12. `specter doctor --fix` (parked on `feat/doctor-fix`) and the `schema_version` manifest field (parked on `feat/schema-version-manifest`) both ship in the v0.12 cycle. After v0.12 release, this policy describes enforced behavior, not aspiration.

---

## Unscheduled — design work needed first

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

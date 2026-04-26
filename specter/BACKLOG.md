# Specter Backlog

Forward-looking roadmap. Items are grouped by target release. Each item is a single sentence of intent plus a link to the design doc or discussion when one exists.

Current shipped version: **v0.11.0** (CLI released to GitHub 2026-04-26; VS Code extension v0.11.0 VSIX built and pending Marketplace publish). Past release notes live in [CHANGELOG.md](CHANGELOG.md) — this file is forward-only.

Between releases. No working branch open. Per `CONTRIBUTING.md` → Branch workflow, PRs target `main` directly until the v0.12 cycle starts, at which point a new `release/v0.12` branch will open and this header will be updated to name it.

The v0.11 cycle delivered five features (explain bundle, check --test, init --install-hook + --ai, settings.strictness + tests_glob), four GH-issue closures (#75, #76, #78, #79), seven security hardening items folded in pre-release, and a CI gates pass. See `CHANGELOG.md` v0.11.0 entry and `docs/explainer/v0.11-ai-loop-discipline.md` for the full walkthrough.

The `chore/dogfood-strict` maintenance branch merged to `main` on 2026-04-24 (PR #66) — internal-only, no version bump. Specter now dogfoods `specter coverage --strict` on its own tests via `make dogfood-strict`: 15/15 specs mechanically verified across 214 (spec_id, ac_id) pairs from Go + TypeScript test runners.

---

## v0.10 — Migration tooling + CI-gated coverage quality (candidate)

The v0.9.0 work made schema drift *visible* via intelligent diagnosis. v0.10 should make it *fixable* without hand-editing, and make the coverage gate resistant to two failure modes currently silent: skipped tests counting as covered, and failing-but-annotated tests counting as covered.

### Migration tooling

**CLI surface discipline** (decided 2026-04-21): don't add `specter migrate` or `specter show` as new top-level verbs. Fold into existing commands — the CLI is already at 14 verbs. `doctor` diagnoses drift, `doctor --fix` repairs it. `explain <spec-id>:AC-NN` already renders an AC card; `explain <spec-id>` (no AC suffix) renders a whole-spec card.

- **`specter doctor --fix` (was `specter migrate`).** Given specs from an older schema version, apply known-safe rewrites: strip removed fields (`trust_level`), rename renamed fields, update enum values, move root-level blocks under `spec:` (jwtms pattern). Dry-run by default (current `doctor` behavior is read-only); `--fix` writes changes. Reuses `doctor`'s drift-pattern analysis for what to repair — the diagnose/repair pairing stays under one verb. Seed with the v0.6.5 `trust_level` removal, the v0.7.0 field renames, and the jwtms v1 shape. See `research/JWTMS_SPECTER_REASSESSMENT_V0.9.md` for the driving design case.
- **VS Code quick-fix for removed fields.** Lightbulb action on a parse error like `Unknown field 'trust_level'` → "Remove deprecated field." Applies to the one file; `Fix all in workspace` batches across every failing spec. Pairs with `specter doctor --fix` for the CLI path.
- **Schema-version metadata.** Record the schema version in each spec (`spec.schema_version`) so `specter doctor --fix` can target known old versions instead of inferring from failure patterns. Optional field with sensible default.
- **`specter explain <spec-id>` (was `specter show`).** AC-less invocation of `explain` renders a human-readable spec card: tier, coverage %, test files covering each AC, uncovered ACs with descriptions. Closes the "where do I look to verify this spec?" gap without waiting on source-annotation scanning. Pure presentation over `specter coverage --json`; no new data collection. Small scope, ~2-3h. Extends existing `explain <spec-id>:AC-NN` behavior — no new top-level verb.

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

**Design discussion**: the three design tradeoffs (two-stage vs one-stage ingest, JUnit flavor handling, missing-results behavior under `--strict`) are resolved in the bullets above. Flake handling deferred.

**Scope**: ~2 days for the `specter ingest` command with JUnit + go test flavors, `--strict` semantics on coverage, extended results-file schema, plus `doctor --fix` and AC-less `explain`. Spec bumps: new `spec-ingest`; `spec-coverage` 1.8.0 → 1.9.0; `spec-doctor` gets a `--fix` AC; `spec-explain` gets a spec-card-without-AC AC. Net CLI surface: +1 verb (`ingest`), not +3.

---

## v0.10.2 — Docs/code parity + `--junit` glob (candidate)

Bug-fix patch. Two real issues surfaced during jwtms Wave 0/1 `--strict` integration (2026-04-23); both are small and ship together.

- **BUG-2 — `specter ingest --junit` glob and multi-flag support.** `CHANGELOG.md` v0.10.0 claimed "glob supported" for `--junit`; the code (`cmd/specter/ingest.go`) uses `os.ReadFile` on a single path and declares `StringVar`, so globs don't expand and repeated `--junit` flags overwrite. Fix: expand paths with `filepath.Glob` when the input contains wildcards; switch to `StringArrayVar` so multiple `--junit` flags accumulate and merge into one results file. `spec-ingest` adds a constraint + AC covering multi-file input.

- **BUG-3 part 1 — `approval_gate` docs parity.** `docs/SPEC_SCHEMA_REFERENCE.md:220` claimed `specter coverage` demotes gated-but-undated ACs. The embedded JSON schema (`internal/parser/spec-schema.json:319`) says Specter does not enforce approval semantics. The code matches the JSON schema. The human doc is the outlier. Fix: update `SPEC_SCHEMA_REFERENCE.md` to match the JSON schema — `approval_gate` is a metadata field; teams wire it into their own PR/CI gates. Add a parity test that reads the JSON schema's field descriptions and asserts they match the human doc's table.

Scope: ~half a day. No CLI behavior change for end users except the `--junit` glob now working as documented. No spec semantic changes.

---

## v0.11 — AI loop enforcement (candidate)

The CI gate (`specter sync`) already enforces annotated tests must exist. This phase makes the loop *proactive* rather than reactive — close the spec → test → implement → eval cycle for AI coding assistants.

**CLI surface discipline**: no new top-level verbs. `specter context` folds into `explain --format`; `specter hook install` folds into `init --install-hook`.

- **`specter explain` v0.11 bundle (terminal-output only).** Extends the existing `explain <spec-id>` / `explain <spec-id>:AC-NN` verb with three new read-only surfaces. **`explain` writes to stdout only — never to files.** File-writing for AI tooling lives on `init --ai <tool>` (separate bullet below).

  **Verb discipline**: every `specter explain *` command prints to terminal. No `--format <tool>` flag that writes to disk. Developers who want to capture output redirect with shell (`> file.md`). Earlier drafts of this BACKLOG entry overloaded `explain` with file-writing — that violated the read-only semantics of the verb and the "one verb, one concern" CLI discipline. Reverted before implementation.

  **1. Schema reference** — `specter explain schema` prints the spec-file JSON Schema in human-readable form, generated from `internal/parser/spec-schema.json` (authoritative source). `specter explain schema --field <name>` (or `-f`) shows details for one field with examples. Supports dot-path lookup (`spec.acceptance_criteria.approval_gate`) and bare shorthand (`approval_gate`); ambiguous shorthand shows all matches. Generated-from-schema approach chosen over rendering `docs/SPEC_SCHEMA_REFERENCE.md` directly: the JSON Schema is the binary's authoritative enforcement source, so the CLI description is guaranteed to match `specter parse` behavior. BUG-3 part 1 was exactly this kind of drift. Over time, `SPEC_SCHEMA_REFERENCE.md` becomes a companion doc with examples and tutorials rather than the reference itself.

  **2. Annotation reference** — `specter explain annotation` prints `docs/TEST_ANNOTATION_REFERENCE.md` (embedded via `//go:embed`) through a markdown-to-terminal renderer with pagination for long output. No field-lookup subcommand — the doc is prose-first, not field-structured; developers who want a specific section pipe to `grep`. Ships the existing hand-written reference to the terminal unchanged.

  **3. AC-less spec card** — `specter explain <spec-id>` (no AC suffix) renders a human-readable spec card: tier, coverage %, test files covering each AC, uncovered ACs with descriptions. Closes the "where do I look to verify this spec?" gap. Pure presentation over `specter coverage --json`; no new data collection. Already implemented on parked branch `feat/explain-spec-card` (4 commits) — needs rebase + merge.

  **Parity test** (carries over from earlier draft): `specter explain schema` stdout for each field must include the field's JSON Schema `description` verbatim. CI fails if a refactor drops a field description. Complements the CLI-docs parity tests in Infrastructure follow-ups.

  **Scope estimate**: `explain annotation` is half a day (embed + markdown render). `explain schema` is 2-3 days (schema walker, type/default inference, field-path lookup, example rendering). AC-less spec card is parked, mostly done. Total ~3-4 days excluding the parked branch's rebase work.

- **`init` family additions — v0.11 scope.** Project-bootstrap commands that write project state. Both bullets below extend the existing `init` verb:

  **`specter init --install-hook`** — writes a git pre-push hook that:
  - Blocks pushes where implementation files changed but no corresponding `@spec`/`@ac` annotation was added or updated in the diff.
  - Reports which specs are affected and which ACs have no test annotation in the changeset.
  - Bypass with `git push --no-verify` (documented, discouraged).
  - Rationale: `init` is the project-bootstrap verb; hook install is one-shot bootstrap, same family as `init --refresh`.

  **`specter init --ai <tool>`** — writes a small (~30 line), stable AI-assistant instruction file telling the AI HOW to use this project's SDD discipline:

  | `--ai <tool>` | Target file |
  |---|---|
  | `claude` | `CLAUDE.md` |
  | `cursor` | `.cursor/rules/specter.md` |
  | `copilot` | `.github/copilot-instructions.md` |
  | `codex` | `AGENTS.md` |
  | `gemini` | `GEMINI.md` |

  One target per invocation. Teams using multiple AI tools run the command once per `<tool>`. **Implementation note**: target-file paths and instruction-file conventions for each tool are still stabilizing across the AI-tooling ecosystem; verify against each tool's current docs before hardcoding paths.

  **What the file contains** (intentionally NOT spec content): a project guide telling the AI how to interact with Specter — where specs live, how to read them on demand (`specter explain <spec-id>`, `specter explain schema`, `specter explain annotation`), the test-annotation conventions, the strictness level configured in `specter.yaml`, the make targets for the gate. Stable across spec edits — file changes only when project config changes (manifest edits, strictness change, etc.). Idempotent fenced `<!-- specter:begin --> ... <!-- specter:end -->` block preserves developer-authored content outside the fence.

  **Why it doesn't dump spec content**: a 249-spec workspace would produce a 10000-line CLAUDE.md, churn on every spec edit, and consume AI context budget reserved for the actual task. The AI doesn't need full spec content pre-loaded — it needs to know specs EXIST, WHERE they live, and HOW to read them on demand. The `specter explain <spec-id>` shell-out from inside an AI session is fast and produces current content, not a stale snapshot.

  **Design synthesis (3-agent research, 2026-04-25).** Findings on what makes instructions get followed, and how to keep them top of mind:

  **File targeting per tool.** Codex / Cursor / Gemini CLI / Copilot all read `AGENTS.md` (the cross-tool standard). Claude Code does not — it reads `CLAUDE.md` only. Copilot's code-review surface reads only the first 4KB of its instruction file. Practical layout:
  - `--ai codex` / `--ai cursor` / `--ai gemini` write `AGENTS.md` (single canonical body).
  - `--ai claude` writes `CLAUDE.md` whose body is `@AGENTS.md` (Claude's import directive) plus any Claude-specific addenda. One source of truth, no duplication. If the user's project has no `AGENTS.md` yet, `--ai claude` writes the body inline so the file stands alone.
  - `--ai copilot` writes `.github/copilot-instructions.md` capped at 4KB.

  **Length.** Cap each generated file at 80 lines. Instruction-following falls off above ~200 lines (Anthropic's CLAUDE.md guidance). HumanLayer's analysis of in-the-wild CLAUDE.md files recommends <60. The point is a project guide, not a manual — every detail link routes back to `specter explain`.

  **Phrasing.** Plain imperative voice ("Run X", "Read Y"). Reserve `MUST` / `NEVER` for the 1–3 highest-stakes rules — aggressive caps inflate triggering and degrade adherence on modern Claude (Anthropic Claude 4 prompting guide). Specificity beats generality: `specter explain <spec-id>` beats "read the spec." Prefer positive ("Annotate every test with `@spec`/`@ac`") over negative ("Don't write tests without annotations") — the Pink Elephant effect costs adherence. One good/bad code-example pair per critical rule (GitHub's analysis of 2,500+ AGENTS.md repos: "one real snippet beats three paragraphs"). Repeat the load-bearing rule first AND last (Lost-in-the-Middle: middle of the file is the lowest-attention region).

  **Top of mind under context pressure.** CLAUDE.md / AGENTS.md / GEMINI.md are loaded as user-prompt content, not system prompt — subject to drift, recency bias, and compaction loss. Two mitigations matter:
  - **Project-root placement**: Claude Code re-injects project-root `CLAUDE.md` after `/compact` (Anthropic doc, "Instructions seem lost after /compact"). Nested `CLAUDE.md` files do not auto re-inject. Always write to project root for the re-injection guarantee.
  - **Self-check at the top**: open the file with Anthropic's own preflight pattern adapted for Specter — "Before editing any file under `internal/`, identify the matching spec, run `specter explain <spec-id>`, then state in your reply which spec ID and AC IDs your change touches." Forces a written artifact before code, turning silent skipping into visible skipping.

  **Hard enforcement (Claude-only, future).** For load-bearing rules instructions cannot guarantee, hooks are the only deterministic mechanism. Two candidates for a future `init --ai claude --with-hooks` (or fold into `init --install-hook`):
  - `PreToolUse` matcher on `Edit|Write` for `internal/**/*.go` — block unless the matching `.spec.yaml` was Read in the same session.
  - `PostToolUse` matcher `compact` — echo a 5-line "Before You Ship" checklist after auto-compaction (the only documented escape hatch for "non-forgettable" rules in Claude Code).

  Codex / Cursor / Gemini have no hook equivalent — for those tools the file is the only persistence layer. Decide hook scope at implementation time; do not block v0.11 launch on it.

  **Idempotent fenced markers.** Wrap the generated body in `<!-- specter:begin v1 --> ... <!-- specter:end -->`. Future `init --ai <tool>` runs replace only the fenced region; user-authored content outside the fence is preserved. The version tag (`v1`) lets later Specter releases migrate the block format without ambiguity.

  **What stays out.** No spec content. No spec list. No AC enumeration. The file teaches the AI to ASK Specter for that data on demand (`specter explain <spec-id>`, `specter explain schema`, `specter explain annotation`), not to pre-load a snapshot that decays the moment a spec is edited.

  **Draft `--ai claude` body** (~50 lines, the canonical body that `--ai codex|cursor|gemini` would also emit, modulo wrapping markers):

  ```markdown
  <!-- specter:begin v1 -->
  # Specter project — read before writing code

  Specs in `specs/*.spec.yaml` are the Single Source of Truth. Code is
  derived from specs; when they disagree, the spec wins.

  ## Before you edit code
  1. Identify which spec governs the file (filename pattern: `specs/spec-<area>.spec.yaml`).
  2. Run `specter explain <spec-id>` and read the AC list.
  3. State the spec ID and AC IDs your change touches before producing code.

  ## Tests trace to ACs (Convention A)
  Every new test carries the spec ID and AC ID in its runner-visible name.

  Good (Go):
  ```go
  t.Run("spec-coverage/AC-19 failed result demotes all tiers", func(t *testing.T) {
      ...
  })
  ```

  Good (Jest / Vitest):
  ```ts
  describe("[spec-extension/AC-12] command registration", () => { ... })
  ```

  Bad — invisible to coverage, will fail the gate:
  ```go
  func TestFailedResult(t *testing.T) { ... }
  ```

  ## Validation
  Run `make dogfood-strict` before declaring work done. Exit 0 is the gate.
  Strictness level for this project is in `specter.yaml`.

  ## Boundaries
  - Do not edit `specs/*.spec.yaml` to make code pass. Update the code,
    or propose a spec change in your reply for human review.
  - If no spec covers your change, stop and ask which spec to read or create.

  ## On-demand reference
  - `specter explain <spec-id>` — canonical spec content. Read it; do not guess.
  - `specter explain schema` — schema field reference.
  - `specter explain annotation` — test-annotation reference.

  Reminder: read the spec before writing code. Tests without `@spec`/`@ac`
  annotations are invisible to `coverage --strict` and will fail the gate.
  <!-- specter:end -->
  ```

  **Scope estimate** (revised after research): `init --install-hook` 1-2 days. `init --ai <tool>` 2-3 days for v0.11 launch (template + per-tool path + idempotency markers + integration test verifying re-run preserves out-of-fence content). The Claude `PreToolUse` / `PostToolUse compact` hook templates are a v0.12 follow-up — they need a session-scoped "spec was Read" tracker in the hook script.

- **`.specter-results.json` test runner adapters** — first-party adapters that write pass/fail results automatically so the pass-rate-aware coverage loop closes end-to-end without manual results-file maintenance:
  - Go: `go test -json | specter results ingest`
  - pytest: `pytest --specter` plugin
  - Jest: `jest-specter` reporter

- **`specter check --test` / `-t`** — extend `check` to cross-reference test annotations against parsed specs. The test-side counterpart to today's spec-side cross-reference checks (`orphan_constraint`, `tier_conflict`). Catches the class of bug the v0.10.1 docs patch could only document, not enforce. Adds three diagnostic kinds:
  - `unknown_spec_ref` — `// @spec foo` in a test file where no spec with id `foo` exists in the workspace.
  - `unknown_ac_ref` — `// @ac AC-99` where the referenced spec has no AC-99.
  - `malformed_ac_id` — `// @ac AC-1` (not zero-padded) or `// @ac ac-01` (wrong case).

  Design decisions (confirmed 2026-04-23):
  - **Opt-in for v0.11.** `check` alone runs today's spec checks unchanged; `check --test` adds the test-annotation pass. Candidate for flipping to always-on in a later version once adoption is smooth.
  - **Short form `-t` is free** — no existing `check` flag declares a short form.
  - **One output stream.** Test diagnostics mix into the existing `check` diagnostic stream, differentiated by kind. Summary line aggregates across kinds.
  - **`specter sync` wiring.** Sync's check phase gets the matching flag so CI can run `sync --strict` including test-annotation checks.
  - **Spec bump**: `spec-check` gets a new constraint codifying the test-annotation cross-check plus one AC per diagnostic kind.

  Deferred to v0.12 or later: `unreachable_annotation` — detects source-only annotations in a test file whose functions don't carry runner-visible pairs (the jwtms-style situation that `--strict` demotes). Correlating a source-comment scan with test-title parsing requires a real test-file parser per language, not just line regex. Worth doing; not in v0.11 scope.

- **Flake handling** (deferred from v0.10) — `--deny-flaky` flag; runners emit `status: flaky`; `--strict` tolerates flakes by default. Ship when real patterns from v0.10 usage surface.

- **`settings.strictness` — first-class strictness level.** Resolves two pending design gaps on the same axis: (a) the exit-code semantics of `--strict` (chore/dogfood-strict Agent 2 finding — a single broken test on a 26-AC tier 2 spec demotes the AC but still passes the tier-80% threshold and exits 0, surprising operators who expect "strict" to mean zero-tolerance), and (b) BUG-3 part 2 (`approval_gate` enforcement — the same question of "should a declared-but-unsatisfied condition fail the build?").

  Today "strictness" is implicit and spread across three places: the `--strict` CLI flag, tier coverage thresholds, and `approval_gate`/`approval_date` metadata. Make it explicit in `specter.yaml`:

  ```yaml
  settings:
    strictness: threshold    # annotation | threshold | zero-tolerance (default: threshold)
  ```

  Three levels with defined semantics:
  - **`annotation`**: pre-v0.10 behavior. Count `// @ac` annotations only; ignore `.specter-results.json`. `--strict` CLI flag rejected with clear error. For new adopters mid-migration.
  - **`threshold`** (default, matches today's v0.10.x `--strict`): demote ACs whose tests didn't pass, then apply tier thresholds. Spec passes if above threshold after demotion. `approval_gate` is metadata, not enforced.
  - **`zero-tolerance`**: any annotated AC without `status: passed` causes non-zero exit, regardless of coverage percentage or tier threshold. `approval_gate: true && approval_date == null` also causes non-zero exit. For CI-strict adopters and mature codebases.

  **CLI interaction**: `--strict` remains a shortcut for `--strictness threshold` (today's meaning). A new `--strictness <level>` flag overrides the YAML per-invocation. Backwards-compatible.

  **Why this design shape rather than "just raise the threshold":** coverage threshold and strictness are semantically different. Coverage = "how much of the spec needs tests"; strictness = "how rigorously are those tests verified." A team can reasonably want 100% coverage with loose strictness (mid-migration) or 50% coverage with zero-tolerance (mature Tier 1 specs). Conflating them via threshold-only conflates two different intentions and doesn't express `approval_gate` at all.

  **Adoption ladder**: teams progress `annotation` → `threshold` → `zero-tolerance` as confidence grows. The level is explicit in `specter.yaml` so new contributors see what CI actually enforces.

  **Spec bumps**:
  - `spec-manifest` gains the `settings.strictness` field with enum validation (one new C + one AC).
  - `spec-coverage` adds constraints for each level's semantics plus exit-code contract (three new C + three new AC, roughly).
  - Replaces the "BUG-3 part 2 — approval_gate enforcement" entry that used to live here; both gaps fold under strictness.

  **Open questions to resolve in design doc, not backlog**: what to do with `coverage_threshold` overrides when strictness is `zero-tolerance` (does per-spec threshold still matter?); whether `--strict` CLI flag should be deprecated in favor of `--strictness`; whether `sync` phase pipes the strictness level through to `coverage` automatically.

- **Python Convention A gap.** `specter ingest`'s test-name regex `([a-z][a-z0-9-]*[a-z0-9])[/:](AC-\d+)` accepts only `/` or `:` as the separator between spec id and AC id. Python function names can't contain either, so the natural form `def test_user_create_AC_01_brief(...)` does not match — pytest emits the function name as the JUnit title, but ingest drops it. Today's Python users have to use Convention B (runtime `print('// @spec ...')` inside the test body) to get the pair into `.specter-results.json`. This is a real friction point — flagging it rather than leaving it buried in docs. Two directions, both viable, pick after real pytest migration friction surfaces:
  - **Docs only**: `TEST_ANNOTATION_REFERENCE.md` tells Python users to use Convention B. No code change. Penalty: Python is a second-class `--strict` citizen.
  - **Regex extension**: accept `_` as a separator, or a specific delimiter like `.` or `__`, so pytest function names can encode the pair directly. Non-trivial — `test_user_create_AC_01` has ambiguous spec-id boundary (`user_create` vs `user-create` vs partial-match). Needs a design doc. Candidate form: require spec-id to carry a `.` delimiter in Python titles (`def test_user_create.AC_01_brief` — invalid Python, so no) or use a class wrapper (`class Test_user_create: def test_AC_01(...)` → JUnit `Test_user_create.test_AC_01` — still no `/` or `:`).
  - **Status**: blocked pending P2 (`TEST_ANNOTATION_REFERENCE.md`) author's decision. If docs-only is chosen, close this item. If regex extension is chosen, spec-ingest 1.2.0 with C-09 and an AC for the new separator.

---

## v0.12 — AI loop hard enforcement (candidate)

Research framing carried from the v0.11 `init --ai <tool>` design synthesis (2026-04-25):

> Project-root `CLAUDE.md` is the only file across the major tools that's guaranteed re-injected after auto-compaction (Anthropic doc). All instruction files load as user-prompt content, not system prompt — they're high-priority but driftable. The two real top-of-mind levers are (1) keep the file at project root, (2) open it with a self-check / preflight prompt that forces the AI to write the spec ID before writing code. For hard guarantees, only Claude Code's `PreToolUse` / `PostToolUse compact` hooks are deterministic — those go in a v0.12 `--with-hooks` follow-up.

v0.11 delivers lever (1) and lever (2) via the instruction file. v0.12 delivers the deterministic backstop:

- **`specter init --ai claude --with-hooks`** — write hook templates into `.claude/settings.json` under a `<!-- specter:begin v1 -->` / `<!-- specter:end -->` fenced region. Two hooks:

  - **`PreToolUse` matcher on `Edit` / `Write` targeting `internal/**/*.go`** — block the edit unless the matching `specs/spec-<pkg>.spec.yaml` was Read in the same session. Implementation requires a session-scoped "spec-was-Read" tracker: a `SessionStart` hook seeds an empty set file in `/tmp`; a `PreToolUse` matcher on `Read` appends to it; the `Edit`/`Write` matcher consults it and exits code 2 on miss. The stderr message becomes feedback to Claude, prompting it to Read the spec first.

  - **`PostToolUse` matcher `compact`** — echo a 5-line "Before You Ship" checklist after auto-compaction. The only documented escape hatch for non-forgettable rules in Claude Code. Checklist content mirrors the load-bearing rules from the v0.11 instruction file (read the spec, annotate new tests with spec-id/AC-NN, run `make dogfood-strict`).

  Claude-only. Codex / Cursor / Gemini have no hook equivalent — for those tools the v0.11 instruction file is the only persistence layer.

  **Spec bump**: `spec-init` gains constraints for `--with-hooks` behavior, hook-file fenced-region idempotency, and SessionStart bootstrap. One C + one AC per hook (4 new ACs total) is the starting scope.

  **Scope estimate**: 3-4 days. Most of the cost is in the session-scoped tracker — the hook script itself is small, but validating it survives across sessions, works inside git worktrees, and doesn't leak `/tmp` state requires integration testing with a real Claude Code instance.

- **`unreachable_annotation` — source-only annotation detection.** Deferred from v0.11's `specter check --test` for the right reason: correlating `// @spec` / `// @ac` source comments with runner-visible test names requires a real per-language test-file parser, not line regex. Candidate for v0.12 once `check --test` has baked. Closes the last class of silent coverage miss the v0.10.1 docs patch could only warn about.

### Post-v0.11.0 triage (issues opened against v0.11.0)

- **GH #93 — `specter doctor` no-manifest discovery mismatch.** Pre-existing since v0.9.0. Fix in v0.12 alongside `feat/doctor-fix` (--fix canonicalization) so doctor's discovery path matches `specter parse`'s recursive-from-cwd fallback.
- **GH #94 — strictness=zero-tolerance + approval_gate enforcement.** Reporter claims AC-29 doesn't fire end-to-end despite the v0.11 unit test passing. Awaiting repro details. Potential v0.11.1 hotfix.
- **GH #95 — `check --test` false positive on multi-`@spec` test files.** Confirmed-class regression. Today's scanner treats latest-`@spec` as parent context for following `@ac` lines; should validate each `@ac` against the union of all declared specs in the file. Likely v0.11.1 hotfix candidate.
- **GH #77 — language-aware `specter explain`.** Per `V0_12_PYTHON_FOLLOWUP_PLAN.md` Item 1.
- **GH #80 — source-only diagnostic hint under `--strict`.** Per `V0_12_PYTHON_FOLLOWUP_PLAN.md` Item 2.

### Post-v0.11.0 feature requests (deferred to v0.13+)

- **GH #96 — `specter migrate` for non-Specter dialects.** Substantial feature; pluggable `--from=<dialect>` registry + `map.yaml` field mapping. JWTMS migration would be the driving case (1900 ACs, 515 constraints). v0.13+.
- **GH #97 — `generated_from.source_files` plural array.** Small schema enhancement matching the existing `test_files` shape. Doctor canonicalization for migration. v0.12 candidate.
- **GH #98 — AC-level lifecycle `status` field.** Schema addition for product-stage tracking. Overlaps with `approval_gate` semantics; needs design. v0.13+.
- **GH #99 — spec-level coverage inference from `generated_from.test_files`.** Adoption affordance for migrated specs that haven't been annotation-backfilled. Non-strict mode only; strict still requires annotations. v0.13.
- **GH #100 — `spec.kind: audit-matrix` for cross-cutting coverage specs.** Polymorphic spec shape; needs schema-stability work first. v0.14+.
- **GH #101 — `specter doctor --diff <baseline>`.** DX improvement for iterative migration work. Pairs with `feat/doctor-fix` in v0.12.

### Stable-pricing follow-ups deferred from the v0.11 cycle

- **VS Code extension v0.11.0 Marketplace publish.** VSIX is built at `vscode-extension/specter-vscode-0.11.0.vsix`; awaits manual `RELEASING.md` gate (install + verify + sign-off) and `vsce publish`. Recommend `--pre-release` first given the C1/C2/H1/H2 fixes are user-facing.
- **`chore/v0.12-security-hardening` pre-staged branch (local-only).** Bundles M1, M2, M4, M5, M6, M7, M8 from the v0.11 pre-release agent review: input size caps, webview CSP, GHA SHA pinning, sigstore signing + SBOM via goreleaser, `release.yml` Tier 3 gating, jest-junit ^17 bump. Will land as a single `chore/v0.12-security-hardening` PR (or split per-concern) when the v0.12 cycle starts.

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

**Status**: aspirational. Policy activates when `specter doctor --fix` (currently parked on `feat/doctor-fix`) and the `schema_version` manifest field (parked on `feat/schema-version-manifest`) ship. Candidate for v0.11 scope. Until both ship, this policy describes intent, not enforced behavior. Extracted from the `docs/schema-stability-policy` branch before that branch was deleted.

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

# Changelog

All notable changes to Specter (CLI + VS Code extension) documented here. The project is pre-1.0; breaking changes go in MINOR releases per semver conventions for 0.x.

Unreleased changes accumulate under `## Unreleased`. Every user-visible change adds its entry in the same pull request. At release the heading becomes `## vX.Y.Z - YYYY-MM-DD` and a fresh empty `## Unreleased` opens above it.

---

## Unreleased

### Added

- **`domains.<name>.tier` is now a checked assertion.** A domain's `tier` declares the risk level it asserts, and a spec listed in that domain whose declared `tier` disagrees produces a `domain_tier_conflict` warning naming both values. It does **not** change the spec's tier: the declared tier still governs. The warning appears in both text and `--json` output. **Action:** none required. If you had set a domain tier expecting it to apply, it never did, and now it says so.

- **`settings.annotation` in `specter.yaml`.** A new manifest block carrying one sub-key, `permissive` (boolean, default `false`), which warns where the same configuration would otherwise fail. Declaring the block is what counts: `annotation: {}` and a bare `annotation:` both register as declared. A manifest carrying no `annotation` key behaves exactly as it did in v0.14, so nothing changes on upgrade unless you opt in. **Action:** none required. This is the replacement for `settings.strictness`, deprecated below, which keeps working until v1.0.0.

  Two behaviors worth knowing before you declare the block. When a manifest carries **both** `settings.strictness` and an `annotation` block, the block wins and `check`, `coverage` and `sync` each warn on stderr naming the ignored key; the exit code is unchanged by the warning. And a declared block currently resolves to the strict path, so a workspace on `settings.strictness: annotation` that declares one moves from the lenient path to the strict one and may go red. That is visible rather than silent, and it is the safer of the two available defaults: the alternative would silently drop the strict path for the far more common `settings.strictness: threshold` case.

  **The rule it enables now ships too.** With a block declared, every acceptance criterion must have a test, and the tier threshold does not excuse one that has none. A criterion with no test is listed on a `no test:` line, appears in `coverage --json` as `no_test_acs`, and exits **2**. A pass rate below the tier threshold exits **1**. Those are different failures and the strictness ladder had one code for both. **`specter sync` returns the same code and names the same cause**, because it is the CI entry point and a code that fires on `coverage` alone would tell CI a different story from a developer running locally. **Action:** if you declare the block, expect criteria with no test to fail where they previously passed on tier arithmetic. Set `permissive: true` to warn instead while you close the gap.

  `settings.annotation.scope` is **not** accepted. `docs/ssrb/SSRB-104.md` names `scope: test | all` in its target shape and only test scope is implemented, so the key is rejected with a message saying so rather than one that reads as a typo.

- **Documentation style gate.** The shared Hanalyx checker (version 6) is vendored at `scripts/check-doc-style.py` and runs on every commit through the pre-commit hook. It enforces a grade 10 reading level, US English, no em dashes, no emojis, and no AI speak, across Markdown and whole-line code comments. Run it directly with `make doc-style-changed` (files changed against `main`), `make doc-style` (whole tree), or `make doc-style-grades` (the reading-level distribution). The failing gate is 11.0 against a writing target of 10.0. **Action:** run `make install-hooks` to pick up the new check. Existing debt does not block you; the gate applies to files you touch.

### Added

- **`specter check --concrete`.** Reports every acceptance criterion carrying neither `inputs` nor `expected_output`, as a `vague_criterion` diagnostic: error at Tier 1, warning at Tier 2, info at Tier 3. Those fields already existed and no stage of the pipeline read them. The rule checks presence only; whether a criterion's wording is testable is a judgment Specter does not make. **Action:** none unless you want the rule. It is opt-in and `--strict` does not enable it, because both fields are optional in the schema, so a criterion without them is valid and failing a build on it would be failing on correct input.

- **`specter diff` compares the fields that make an acceptance criterion concrete.** It compared id and description only, so inverting an `expected_output` from `[1, 2, 3]` to `[3, 2, 1]`, flipping `approval_gate` from true to false, changing `priority`, or rewriting `inputs`, `error_cases` or `references_constraints` all reported `no changes` and exited 0. Each of those changes the contract a criterion's tests were written against, and each now reports and classifies as breaking. A priority change counts in both directions: a downgrade says a criterion matters less without touching a word of it. **Action:** a diff that reported no changes may now report a breaking one. That is the change being seen, not a new failure.

- **`specter diff --exit-code`.** Opt-in. With the flag, a breaking change exits `10`; without it, `diff` exits 0 as before, because it is a diagnostic surface rather than a gate. Code `10` is the first allocation from the orchestration band. **Action:** none unless you want the gate. Add the flag where you want a breaking change to fail a pipeline.

### Fixed

- **`specter diff` printed the same description twice on a contract-only change.** A criterion whose `expected_output` changed but whose wording did not rendered `~AC-01: same text → same text` under a `[breaking]` header. The line now names the fields that differ: `~AC-01: expected_output changed (description unchanged)`. A criterion that changed both shows the transition and names the fields.

- **The `structural_conflict` diagnostic named the wrong spec's constraint.** The header reads `<spec> <constraint-id>`, the spec was the downstream one, and the constraint id belonged to the upstream spec, so looking that id up in the named spec found nothing or found a different constraint sharing it. The id is now qualified with its owner: `spec-app spec-core/C-01`. **Action:** if you parse `constraint_id` from `check --json`, it now carries `<spec>/<id>` for this diagnostic kind.

- **Structural-conflict detection skipped `extends` and `conflicts_with` edges.** It scanned only `requires`, so the one relationship where a contradiction check has the strongest reason to look was the one place it did not look. It now scans every dependency edge. **Action:** none. No workspace in the corpus declares such an edge, so no existing output changes.

- **`specter check` no longer fails a build on a structural conflict.** The `structural_conflict` diagnostic is a lexical heuristic: it fires when a downstream criterion mentions an upstream constraint's subject near a word like `absent` or `without`. It cannot tell a genuine contradiction from a criterion that tests the constraint being enforced, because `Process checkout when email is absent` and `Registration fails when email is absent` differ only in the outcome verb, which the rule never looks at. It now reports at `info`, is not raised by the constraint's `enforcement` field, is not promoted by `--strict`, and never contributes to a non-zero exit. The diagnostic still appears and still names a real tension worth reading. **Action:** a build that failed on `structural_conflict` now passes. If you were suppressing it by rewording criteria, you can stop. If you relied on it as a gate, nothing replaces it, and `docs/SPECTER_LEXICON.md` explains why a lexical rule cannot be one.

- **`specter coverage --json` exited 0 on a workspace `specter coverage` failed.** With `settings.annotation` declared and a criterion carrying no test, text mode exits 2. Under `--json` the same workspace exited 0 whenever the tier threshold was otherwise met, so a CI job reading `--json` got a green build. It also stayed silent: the warning naming the criteria went to the text path only. Both surfaces now run one shared gate sequence, so a gate cannot reach one and miss the other. **Action:** a CI job gating on `coverage --json` may start failing where it passed. That is the workspace failing, not the tool; run `coverage` for the criteria involved.

- **The zero-tolerance approval-gate demotion left three things wrong in the report it edited.** The demotion moved a criterion out of the covered list after the report was built, and three of the builder's own guarantees did not survive the edit. A demoted criterion was appended to the end of the uncovered list instead of appearing in declaration order. An entry whose every covered criterion was demoted reported `covered_acs` as `null` rather than an empty array, against a JSON contract that declares it non-nullable. And `fully_covered` and `partially_covered` were never recomputed, so one document could report a spec as fully covered while showing it at 50 percent beside that count. Classification now happens once, so the report is built with the demotion rather than edited afterward. **Action:** none, unless you parse `covered_acs` and depended on the null, or on the append ordering. Both were defects.

- **`specter reverse --json` reported specs it never wrote.** The JSON branch returned before the write loop, so `--json` was an unconditional dry run and `--output` was accepted and ignored. The command reported `SpecsGenerated: 43`, wrote nothing, and exited 0. `--json` now selects the report format only, and `--dry-run` is the only flag that stops files being written. Under `--json`, stdout carries exactly one JSON document and the per-spec `GENERATED` and `SKIPPED` lines go to stderr, so machine-readable output stays parseable. A write that fails exits non-zero without emitting JSON. **Action:** if you used `--json` as a report-only mode, add `--dry-run`. This includes runs with no `--output`, which now write to the default `specs` directory.

- **`specter reverse` told you to run a command that does not do what it said.** The handoff line read `Run 'specter explain <id>' to triage gaps in each generated draft`, and `specter explain` has no gap handling. The line now says review the criteria, which is what `explain` does: it lists every acceptance criterion with its coverage status. **Action:** none.

- **The gap count included criteria that are not gaps.** When extraction found nothing for a spec, `reverse` synthesized a placeholder acceptance criterion and flagged it `gap: true`, which the summary counted alongside real findings. `go-chi/chi` reported `0 constraints, 21 gaps`. The placeholder no longer carries the flag. Across a 12-repository corpus the reported total dropped from 7,311 to 7,149, and every repository now reports gaps at or below its constraint count, where six previously reported more gaps than constraints. **Action:** none. **Note:** the remaining count is still not a measure of test coverage. A constraint counts as covered only when an assertion's text contains the constraint's field name, and real test names rarely do.

- **Under `--group-by file`, `specter reverse` put a test file in its own spec instead of with the source it tests.** Every file was its own group, so `user.go` and `user_test.go` became two specs. Gap detection compares a group's constraints against that group's assertions, so a source file grouped apart from its test was compared against nothing and every constraint it carried was reported `UNTESTED`, whether or not a test existed. A test file now joins the source it names, when the adapter can name it and that source is present in the run. A test whose source is absent, or that the adapter cannot map (a Python `conftest.py`, for example), keeps its own spec as before. `--group-by directory` is unchanged. **Action:** a regenerated workspace has fewer specs, and criteria derived from a test now sit in the same spec as the constraints from its source.

  **This does not repair gap detection on its own.** Measured on a 12-repository corpus, the fold merged 86 name-paired files and removed 49 false gaps out of 7,360, under one percent. The larger cause is untouched: a constraint counts as tested only when an assertion's text mentions the constraint's field name, and real test names rarely do. In `go-playground/validator`, 86 constraints and 105 assertions still produce 86 gaps, because its validate tags and its tests live in files that were never name-paired. Grouping is what lets the comparison happen; it is not what makes it succeed.

- **`specter reverse` generated colliding spec ids, and `specter resolve` rejected the result.** Ids came from the file basename, with the parent directory prepended only when the basename was one of 14 hardcoded generic names. Two files called `coverage.go` in different packages both became `coverage`, with no warning, and the next command in the pipeline refused the workspace. Ids are now unique within a run: every member of a colliding set gains parent directory segments, one at a time, until the set is distinct, and each rename is reported as an `id_collision` warning naming the old id and the file. **Action:** ids in a regenerated workspace may differ from ids you generated earlier. The warning names every one that changed. If you pinned a generated id somewhere, check it against the new output.

  The same change ends a silent loss of generated specs. Two colliding ids produced the same output filename, so the second was skipped under a message saying the file already existed, when the file had been written seconds earlier by the same run. `reverse` reported 43 specs for one repository and wrote 37. Reported and written counts now agree.

  Measured on a 12-repository public corpus, before and after: workspaces the pipeline accepts went from 0 to 12, duplicate id diagnostics from 1,452 to 0, across 4,619 generated specs.

- **`specter reverse` generated specs that `specter check` rejected.** It wrote `enforcement: error` on every constraint it produced, including the placeholder it synthesizes when extraction finds nothing to constrain. That field overrides the tier-based severity `check` gives an unreferenced constraint, so every generated constraint no acceptance criterion happened to reference failed the build at error severity. Generated specs are `status: draft` and `tier: 3`, whose default for that diagnostic is `info`. The field is now omitted, and the tier default applies. **Action:** regenerate to pick this up, or delete the `enforcement:` line from constraints in specs you generated earlier. Specs you wrote by hand are unaffected; `enforcement` still means what it always did when you set it yourself.

  Measured on a 12-repository public corpus. Before the fix, both repositories whose generated specs got as far as `check` failed it: 37 and 61 diagnostics, every one an `orphan_constraint`, none at `info`. After, both pass. The other ten never reach `check`, because their generated spec ids collide and `resolve` rejects them first; that defect is separate and still open.

- **`tier_conflict` asserted something false on every run.** The warning ended `using override (N)` while nothing applied the override, so `check` claimed one tier was in use while `coverage` reported another for the same spec on the same run. It now states that the declared tier governs. This is not cosmetic: correcting the public documentation in August nearly shipped a second false claim, because the only evidence for the reading that the override wins was this message.

- **`tier_conflict` never reached `specter check --json`.** It was computed after the JSON branch returned and counted separately in the text summary, so no JSON consumer had ever seen it, including the VS Code extension, and the two output modes disagreed on the warning count for the same workspace. Both `tier_conflict` and the new `domain_tier_conflict` now go through one diagnostic list, so the counts agree by construction. **Action:** if you parse `check --json` and had compensated for the missing diagnostic, remove the workaround.

- **32 Dependabot alerts closed in the VS Code extension's dependency tree.** A lockfile-only refresh with no version-range changes. **No release was affected:** the extension declares zero runtime dependencies, and the published VSIX contains zero `node_modules` entries, so none of the flagged packages has ever shipped to a user. The exposure was to CI and developer machines, where these run during `npm ci`, `tsc`, `jest` and `vsce package`. Nine of the eleven packages arrived through `@vscode/vsce` alone. **Action:** none. Run `npm ci` in `vscode-extension/` if you build the extension locally.

- **`specter check` documentation described a diagnostic that does not exist.** The README and `docs/CLI_REFERENCE.md` said `tier_conflict` catches "a Tier 1 spec depends on a Tier 3 spec", and the README printed it as an ERROR. It fires only when a spec's declared `tier:` disagrees with an entry in `settings.tier_overrides`. It is a warning, `--strict` does not escalate it, and it does not appear in `--json` output. `settings.tier_overrides` does not change a spec's effective tier. **Action:** if you set `tier_overrides` expecting a stricter or looser gate, it is not in effect. Set `tier:` in the spec instead.
- **The pre-commit hook ran no checks once installed.** It resolved the Go module path relative to its own location, which is `.git/hooks` after `make install-hooks`, so `gofmt` and `go vet` were skipped on every commit. **Action:** run `make install-hooks`, then `make check` on any branch you have in flight.

### Removed

- **The tier inheritance cascade.** `ResolveTier` implemented explicit tier, then domain tier, then `system.tier`, then a default of 2. Nothing called it, and the last three steps were unreachable regardless, because the schema makes `tier` required. Three acceptance criteria asserted that inheritance and passed only because their tests called the function directly with a tier of zero, an input no parsed spec can produce. **Action:** none. No shipped behavior changes, because the behavior never existed. `docs/ssrb/SSRB-106.md`.

### Deprecated

Four manifest surfaces are on a removal path for v1.0.0. **Three of them do nothing today**, so removing them changes no workspace's behavior; an operator who configured one was already getting nothing. Only `settings.strictness` carries real behavior and needs a migration.

- **`system.tier` and `settings.tier_overrides` now warn on every run**, name themselves, and state that they are removed at v1.0.0. Both were previously validated, range-checked, and read by nothing, so an operator who set one got silence. The warning changes no exit code. **Action:** set `tier:` in the `.spec.yaml` file, the only place that has ever governed a spec's tier.

- **The `registry` section is retired.** Its key is still accepted so an existing manifest parses, and its value is discarded. The key is removed at v1.0.0. Nothing ever read or wrote it: the constraint mandating an auto-update was never implemented. **Action:** delete the block. Nothing is lost, because nothing was ever stored there. `docs/ssrb/SSRB-105.md`.

- **`settings.strictness` and `--strictness` will be retired at v1.0.0.** Both keep their current behavior unchanged for every release before then, so nothing in your manifest or your CI stops working now. This is advance notice, not a migration you have to run today.

  The setting was created to answer one question: does an acceptance criterion have a test reference? What shipped answers a different one. The three levels form an evidence ladder deciding how much proof a criterion needs before it counts as covered, which is a coverage judgment rather than a check on whether a marker exists. The difference is visible on a spec whose second criterion has no marker anywhere: `specter check --test` reports all specs passing and exits 0, while `specter coverage` reports that criterion uncovered. `check` scans from tests to specs and cannot see a marker that is absent, while `coverage` scans from specs to tests and reports it.

  The replacement is `settings.annotation`, carrying two sub-keys: `scope`, which is `test` (markers required in test files) or `all` (test and source files), and `permissive`, which warns where the same configuration would otherwise fail. It is set in `specter.yaml` only. There is no `--annotation` flag and none is planned, so the flag and the manifest cannot disagree about it. Four rules govern it. Every acceptance criterion must have a test, and a criterion with no test fails regardless of any threshold. `state` sets the scope. The `settings.coverage.tier1`, `tier2` and `tier3` thresholds you already have set the allowed failure rate among criteria that do have tests, so `tier2: 80` means 80 percent must pass. `permissive` sets severity rather than scope.

  That separation is the point of the change. Today the coverage percentage cannot tell a criterion with no test from a criterion whose test failed: both report the same number, and they need different responses from you. `scope: all` does not ship in v0.15.0. The decision record is `docs/ssrb/SSRB-104.md` and the request is tracked as `features/SP-006`.

  **Action:** none required for this release. If you set `settings.strictness` today, keep it. When the replacement lands it will ship with a migration note naming the exact replacement for your current value, and both spellings will be accepted until v1.0.0.

  **Worth knowing now if you run `--strict`:** it is a separate setting from `--strictness` despite the names, it means something different on each of the three commands that accept it, and `make dogfood-strict` in this repository has never exercised the level its name claims. Those are tracked as `bugs/SP-SP-046` and `bugs/SP-SP-047` and are being fixed on their own track.

### Changed

- British spellings corrected in `README.md`, `GOTCHAS.md`, `docs/CLI_REFERENCE.md`, `vscode-extension/README.md`, and three VS Code extension source comments. One decorative symbol removed from `docs/explainer/v0.10-ci-gated-coverage.md`. No behavior change.

---

## v0.14.1 - 2026-06-13

**Theme: stable-channel promotion, no functional change from v0.14.0.**

The 0.13.0 → 0.14.0 line shipped to the VS Code Marketplace pre-release channel only; the last stable release users on the default channel received was v0.12.1. This patch carries no code change from v0.14.0, it exists solely to obtain a fresh version number, because the Marketplace forbids republishing an already-published version (0.14.0) without its pre-release flag. v0.14.1 is the first **stable** release carrying the full 0.13 + 0.14 body of work (strict-coverage routing, resolver all-cycles, sync test-file cap, VS Code per-folder coverage). The packaged VSIX is byte-for-byte equivalent to the published 0.14.0 pre-release apart from the version string.

This also re-anchors release traceability: the `v0.14.0` tag was cut before #147 (`fix(vscode): exclude jest-junit output from the VSIX`); the `v0.14.1` tag points at the current `main` HEAD, which includes that fix.

### Changed

- Version bump only (`VERSION`, `vscode-extension/package.json`). No CLI, schema, or extension behavior change. See v0.14.0 below for the substantive release notes.

---

## v0.14.0 - 2026-06-11

**Theme: strictness enforcement parity, a correctness bundle from the 2026-06-11 six-finding review.**

An adversarially-verified review of the toolchain surfaced six findings; all six were confirmed against the code and fixed across four PRs (#140, #141, #144, #145). Per the pre-1.0 critical-issue handling, this minor is themed around the correctness bundle; the originally planned v0.14 headline feature moves to the next minor. This ships as a MINOR (not a patch) because the default behavior of `specter coverage` changes.

### Changed (action may be required)

- **`coverage --strictness threshold` / `zero-tolerance` now route through the same strict path as `--strict`** (spec-coverage 1.14.0 → 1.15.0, C-31/C-32 + AC-36/AC-37; #140). Previously the strict path keyed solely on the `--strict` boolean: `coverage --strictness threshold`, and plain `specter coverage` under the manifest default `threshold`, silently tolerated a missing `.specter-results.json` and counted failed Tier 2/3 annotated ACs as covered, while `sync` failed the same workspace. Now an effective strictness of `threshold` or `zero-tolerance` (flag or manifest) requires `.specter-results.json` and demotes non-passed annotated ACs across all tiers, matching `sync`'s routing since v0.13. When the strict mode comes from `--strictness` or the manifest (not the `--strict` flag), a missing results file fails with a mode-aware message: `strictness "threshold" requires .specter-results.json, run 'specter ingest' first, or use --strictness annotation for structural coverage`. **Migration:** pipelines running plain `specter coverage` without a results file should run `specter ingest` first (outcome-gated coverage), pass `--strictness annotation`, or set `settings.strictness: annotation` (structural coverage). The repo's own lightweight CI gates and `make dogfood` switched to `--strictness annotation`; `--strict` behavior is unchanged.
- **The VS Code extension pins `--strictness annotation` on its `coverage --json` calls** (#140), preserving the sidebar's structural coverage view and its "JSON document on every run" contract byte-for-byte under the new default.

### Fixed

- **`sync --strictness zero-tolerance` false-green** (spec-sync 1.3.0 → 1.4.0, C-09 + AC-12/AC-13; #140). Sync routed zero-tolerance through the strict report path but gated only on tier thresholds: a Tier 3 spec with one passing and one failing annotated AC passed sync at 50% while `coverage --strictness zero-tolerance` exited 2 on the same workspace, and `approval_gate` violations were never checked in sync at all. Sync's coverage phase now enforces both zero-tolerance gates with the same exit codes as `coverage` (2 = non-passed annotated AC, 3 = `approval_gate: true` with unset `approval_date`), demotes approval-gate violations in its report, and applies both identically under `--json`. The counting/demotion logic is shared (`internal/coverage/zero_tolerance.go`) so the parity is mechanical, not a review-time promise.
- **Resolver now reports ALL overlapping cycles** (spec-resolve 1.2.0 → 1.3.0, strengthened C-03 + AC-15; #141). The white/gray/black DFS silently skipped edges into fully-processed nodes, so two cycles sharing an edge (theta graph) dropped one cycle nondeterministically (~60% of runs, map-iteration-order dependent). `findCycles` is now Tarjan SCC decomposition + Johnson's simple-cycle enumeration with deterministic canonical output: each cycle starts at its lexicographically smallest spec ID and the set is sorted. Enumeration is capped at 1000 simple cycles (documented in C-03). Diagnostic kind and message format unchanged.
- **`sync` applies the 4 MiB test-file cap** (spec-sync 1.4.0 → 1.5.0, C-10 + AC-14; #144). Sync read every discovered test file with an unguarded `os.ReadFile`, bypassing the `MaxTestFileBytes` cap that `check --test`, `coverage`, and `explain` apply, the v0.13 H5 hardening commit claimed the sync site but never touched it, so a single large test artifact could OOM the primary CI command. Sync now stat-guards before reading (oversized files skip with a stderr warning, never failing the run) and unreadable test files warn + skip instead of silently becoming empty content.
- **VS Code: per-folder coverage reports + on-save whole-report refresh** (spec-vscode 1.6.0 → 1.7.0, amended AC-22/AC-33, new AC-54/AC-55; #145). Two extension-state bugs: (a) multi-root workspaces kept clients and diagnostic collections per folder but stored ONE module-global coverage report (last folder wins) and resolved CLI-relative paths against `workspaceFolders[0]`, so folder B's parse-error diagnostics pointed into folder A; (b) the on-save handler regex-scanned saved spec YAML for `// @spec` slash comments and spliced the fresh report's `entries[0]` into the matched spec's slot, corrupting coverage/status/notifications whenever the saved spec wasn't the report's first entry. Reports are now stored per folder with paths normalized to absolute against the owning folder's CLI cwd at ingestion time; the status bar/sidebar/Insights read a merged view (identity-returning for single-root); saving a spec re-runs coverage for its folder through the same path activation uses. Side effects: on-save now actually refreshes coverage (the old trigger almost never fired on real spec files), and sidebar tooltips show absolute paths.

### Docs

- `docs/CLI_REFERENCE.md`: coverage `--strictness` row documents the strict-path routing and the strict-by-default consequence; coverage exit codes 2/3 documented; sync synopsis gains `--strictness`; `--strict` correctly described as NOT overriding a manifest-set strictness level. Verified against the binary by an independent review pass per the Docs Review Policy.
- `docs/explainer/v0.13-sync-strict-coverage.md`: corrected the overstated historical claim that v0.12 `coverage` fully honored `strictness` (it honored only the zero-tolerance exit codes; strict report routing landed in this release).

---

## v0.13.3 - 2026-06-10

**Theme: DX patch, a non-misleading `sync` missing-results message.**

External adopter (Kensa) feedback on the v0.13 strict-coverage behavior. Under a strict mode, `specter sync`'s coverage phase fails when `.specter-results.json` is absent, but the error reused `coverage --strict`'s wording (`--strict requires .specter-results.json …`), naming a `--strict` flag the sync operator usually never passed. Under `sync` the strict mode comes from the manifest default (`threshold`), not an explicit flag, so the message pointed at the wrong lever. Patch-trigger: documentation/UX correction that materially misleads users.

### Fixed

- **`sync` missing-results message now names the active strictness mode and offers both remedies** (spec-sync 1.2.0 → 1.3.0, C-08/AC-10; `internal/sync/sync.go`). Under `threshold` / `zero-tolerance` with no `.specter-results.json`, sync now emits `strictness "threshold" requires .specter-results.json, run 'specter ingest' first, or use --strictness annotation for structural coverage`. The rewrite is gated on `errors.Is(err, coverage.ErrMissingResults)`, so **`coverage --strict`'s own message is unchanged**. Also corrects spec-sync C-06's stale parenthetical: the manifest default is `threshold` (per spec-manifest C-24), not `annotation`.

### Added

- **`docs/explainer/v0.13-sync-strict-coverage.md`.** A developer explainer for the v0.13 `sync` strict-coverage results-file requirement: the intent (v0.13 closed the gap where `sync` ignored `strictness` while `coverage` honored it), the local-vs-CI hazard during a partial-version upgrade, and three remedies (ingest before `sync`, split a structural `sync --strictness annotation` gate from a separate `coverage --strict` gate, or set `strictness: annotation`).

---

## v0.13.2 - 2026-05-18

**Theme: post-ship review patch, close two remaining spec-vs-code drift instances + one user-facing docs gap.**

A four-agent post-ship review of v0.13.0 + v0.13.1 surfaced 22 findings. Three meet the CLAUDE.md patch-trigger criteria ("documentation correction that materially misleads users"); they ship in this patch. The remaining 19 are cosmetic, design-pending, or in-scope for v0.14.

### Fixed

- **`approval_gate` schema description corrected** (`internal/parser/spec-schema.json:319`). The embedded schema claimed *"Specter does not enforce approval semantics, teams wire this into their own PR/CI gates."* Since v0.11.1 (GH #94 hotfix), `coverage` actually enforces `approval_gate` under zero-tolerance, an AC with `approval_gate: true` and unset `approval_date` is demoted and exits with code 3. `SPEC_SCHEMA_REFERENCE.md:221` already carried the correct text; the embedded schema was missed by the v0.11.1 update. Same shape as the v0.13.1 fix for `spec.status`, completes the b0ba292 sweep across all schema-field descriptions. `specter explain schema spec.acceptance_criteria.items.approval_gate` and the VS Code schema tooltip now match the documented behavior.

- **`specter diff spec <a> <b>` ghost form retracted** (spec-diff 2.1.0 → 2.2.0, AC-12 removed; `cmd/specter/main.go:3029-3046`). v0.13.0 promised an explicit `diff spec <a> <b>` form parallel to the implicit `diff <a> <b>`. The form was never implemented: cobra `ExactArgs(2)` rejects 3-arg invocations, and 2-arg `diff spec foo.yaml` misroutes `args[0]="spec"` into `readSpecAtRef` (which fails with "no such file"). The implicit form (AC-11) is the only invocation users have been using and is sufficient. Retracting the false promise is preferred over adding code because re-introducing a `spec` subcommand would collide with the polymorphic subcommand-dispatch pattern (where `coverage` is the registered kind). This is the third instance of the same drift class F3 surfaced in this cycle, spec-vs-code parity is now closed across every v0.13 spec change.

### Added

- **`// @reachable manual` documented in user-facing docs** (`docs/TEST_ANNOTATION_REFERENCE.md`, `internal/explain/annotation_reference.md`, `docs/CLI_REFERENCE.md`). v0.13.0 shipped the file-level off-switch for `unreachable_annotation` but the marker was only documented inside the spec, a user encountering the diagnostic in CI had no docs path to the escape hatch. The reference doc now carries: (a) a new "Suppressing `unreachable_annotation` per-file" section with the marker syntax per language family, scope semantics, when to use vs. when not to use, and strictness routing; (b) two new "Troubleshooting" entries for both `unreachable_annotation` and `unreachable_annotation_unknown`. The embedded mirror used by `specter explain annotation` is byte-for-byte updated via the existing parity test. The `check` command's Diagnostics table in CLI_REFERENCE also gains rows for both new diagnostic kinds (plus the pre-existing-but-undocumented `unknown_spec_ref` / `unknown_ac_ref` under `--test`).

### Not in this patch (tracked for v0.14)

The audit also identified items requiring design discussion rather than patching:

- `settings.exclude` glob patterns don't apply to test-file discovery (`discoverTestFiles` ignores `m.Settings.Exclude`). Spec C-29 frames broad coverage; code applies narrowly.
- `coverage_threshold: 0` silently falls through to the tier default (Yoke bug reporter's secondary issue). Decision pending: `*int` with explicit-presence detection vs. schema `minimum: 1` vs. docs-only.
- `parse` and `doctor` silently no-op when ParseManifest rejects a manifest (H1 error swallow at `main.go:292`). Security holds; UX is inconsistent.

### Audit context

Post-ship review used four parallel general-purpose agents covering: (1) feature behavior end-to-end, (2) spec-vs-code drift hunt, (3) docs parity, (4) bug/security fix boundary verification. No critical findings; two High (both addressed in this patch); four Medium (three addressed, four deferred); fourteen Low / Informational. The full finding set is captured in conversation history; a future schema-docs parity test (scheduled v0.17 D1) will mechanically catch the b0ba292-class drift going forward.

---

## v0.13.1 - 2026-05-17

**Theme: hotfix, finish the v0.12.1 status-claim docs/code parity work.**

The v0.12.1 cycle (commit `b0ba292`, 2026-05-07) updated `SPEC_SCHEMA_REFERENCE.md` and `FAQ.md` to drop the misleading claim that "only 'approved' specs are enforced by spec-sync", Specter's actual behavior is that every parseable spec is checked. That fix landed on the human-facing docs but **missed the embedded JSON schema description at `internal/parser/spec-schema.json:39`**, which is the source `specter explain schema spec.status` and the VS Code extension's schema tooltip read from.

Bug report from a third-party adopter (the Yoke project) running v0.13.0 surfaced the gap: `specter explain schema spec.status` still output the misleading claim, and operators reading it assumed `coverage --strict` would honor `status: draft`. It doesn't (and never did).

### Fixed

- **Schema description text** (`internal/parser/spec-schema.json:39`). Updated to match the v0.12.1 canonical text from `SPEC_SCHEMA_REFERENCE.md`: *"Lifecycle status. Specter parses and checks all discovered specs; status is informational. Use settings.warn_on_draft and settings.strict when draft specs should block release gates."* Closes the docs-vs-code drift class instance that `b0ba292` left incomplete. The VS Code extension picks this up via the next binary refresh.

### Not in this patch

The bug reporter's secondary ask, a way to have `coverage --strict` *actually* exempt draft specs from CI gates ("scaffolding lands first, per-area implementation is staged over weeks or months"), is a legitimate adoption-time feature request, but not a docs bug. Status-aware gating (e.g., `coverage --enforce-status approved` or `--exclude-status draft`) is a feature design discussion for a future minor release, gated on the feature-universality test from the project memory (does this generalize beyond one adopter?). Tracked as a follow-up; not blocking the v0.13.1 docs correction.

---

## v0.13.0 - 2026-05-16

**Theme: unreachable_annotation diagnostic + cycle cleanup.**

Headline feature: F3, `unreachable_annotation` diagnostic. When a test file carries `// @ac AC-NN` but the test produces no runner-visible spec-id/AC-NN token (neither in the subtest title nor as a runtime print of `// @spec` / `// @ac`), `specter check --test` now emits the diagnostic naming `file:line`. Pre-v0.13 the annotation would silently demote under `coverage --strict` with no signal, the same docs-vs-code drift class the v0.10.x CHANGELOG flagged. Covers Go (via `go/ast` + `ast.CommentMap`), TypeScript / Jest / Vitest (regex + block-comment state machine), and Python (regex + indentation). File-level off-switch: `// @reachable manual` (`# @reachable manual` for Python) suppresses both `unreachable_annotation` and `unreachable_annotation_unknown` for every `@ac` in the file.

### Added

#### F3, unreachable_annotation diagnostic (spec-check 1.3.0 → 1.5.0, C-10/C-11/C-12, AC-13..AC-20)

- `specter check --test` now runs the language-aware reachability scanner. Severity routes per `settings.strictness`: `annotation` suppresses; `threshold` (default) emits a warning (exit 0); `zero-tolerance` emits an error (exit non-zero).
- `unreachable_annotation_unknown` (always a warning, never an error) fires when the test shape is not recognized by any language-aware parser, operator may add `// @reachable manual` to assert manual verification.
- New CLI integration tests (`cmd/specter/check_test.go::TestCheckTest_UnreachableAnnotationFiresFromCLI` + `TestCheckTest_ReachableManualSuppressesFromCLI`) pin the wiring so future refactors can't re-orphan the diagnostic.

#### `specter diff coverage <baseline.json> <current.json>` (spec-diff 1.x → 2.1.0)

Polymorphic `diff` verb. First new kind is `coverage`, compares two `coverage --json` snapshots, emits per-spec AC delta (`+SpecID/AC-NN` gained, `-SpecID/AC-NN` lost, `~SpecID coverage_pct: X → Y`). Useful for tracking AC drift between CI runs. Backward compat preserved: `specter diff <path> <path>` continues to invoke the spec-comparison kind.

#### `specter resolve dependents <spec-id>` (spec-resolve)

Reverse dependency-graph query, "which specs depend on this one?" Companion to the existing `dependencies` operation.

#### Bundled enhancements

- `specter sync --strictness <annotation|threshold|zero-tolerance>`, explicit per-invocation override. Sync's strict-mode behavior now matches `coverage`'s; pre-v0.13 was a silent gap.
- `settings.exclude` accepts glob patterns (`.claude/**`, `**/worktrees`, `tests/fixtures/*`) in addition to bare directory names. (spec-manifest C-29)
- `specter reverse` emits a summary line and `specter explain` handoff at the end of a non-JSON run.
- `specter check --test` now lifts AC-14..AC-18 onto spec-parse (v0.7.0 parse behaviors that had test annotations but no spec ACs, closes the spec-vs-code drift in the opposite direction from F3).
- Auto-generated `specter help` subcommand suppressed (use `--help` or top-level `specter help`).
- CLI-flags ↔ `docs/CLI_REFERENCE.md` parity test catches the class where a flag is registered but undocumented.

### Fixed

- **D1, `doctor --fix` no longer corrupts description-block prose** (spec-doctor 1.9.0, C-17). The v0.12 rewrite used a content-pattern regex over every source line; a `description: |` block scalar mentioning `trust_level: high` in prose would be falsely matched and the doc line stripped. Now uses `yaml.v3 Node.Line` for exact-line deletion. The BETA warning's "Known limitation" claim removed; the BETA prompt itself is retained for v0.13 as soak time on a destructive operation.
- **D2, Unrecognized `status` value diagnostic** (spec-coverage 1.14.0, C-30, AC-35). A `.specter-results.json` entry with `status: "pass"` (typo missing `ed`) previously demoted the AC silently under `--strict` with no signal. `coverage` now emits a stderr warning per unique unrecognized value (`warning: .specter-results.json contains N entries with status="pass", not a recognized status (passed|failed|skipped|errored); treated as not-passed`) and exposes the data as a top-level `invalid_status_warnings` array under `--json`.
- **D3, `coverage --strict --json` exit-code parity** (spec-coverage 1.13.0, C-29, AC-34). Pre-v0.13, JSON mode exited 0 on zero-tolerance violations that text mode exited 2 (non-passed AC) or 3 (approval_gate unset). CI consumers reading `--json` could not gate on it. The JSON branch now runs the same exit checks text mode runs, after emitting the structured document.
- **D4, `internal/migrate/rewrite.go` C-10 → C-11 comment.** One-character documentation fix.

### Security

Three priority findings from the pre-release security audit closed this cycle (artifact at `docs/release-testing/v0.13.0-security.md`):

- **H1, `settings.specs_dir` workspace-scope** (spec-manifest 1.11.0, C-30, AC-46). `ParseManifest` refuses absolute paths, Windows drive-letter form (on every platform), `..` segments, and lexical-clean escapes. Closes the arbitrary-write path where a malicious workspace's `specter.yaml` with `settings.specs_dir: /home/victim` caused `filepath.Walk` and `doctor --fix` to operate outside the workspace.
- **H5, `MaxTestFileBytes` (4 MiB) cap on test files** (spec-check 1.5.0, C-12, AC-20). v0.13's F3 scanner reads every discovered test file via `os.ReadFile` and passes the full string to `go/parser.ParseFile` (Go), regex+state-machine (TS/JS), or regex+indentation (Python). A 4 GiB malicious test file would OOM the process; `go/parser` allocates an AST proportional to file size. The cap is enforced via `os.Stat` BEFORE `os.ReadFile`, so oversized files are never buffered into memory. Skip is per-file (not fatal); stderr warning names the file.
- **M3, `MaxCoverageReportBytes` (16 MiB) cap on `diff coverage` inputs** (spec-diff 2.1.0, C-12, AC-15). v0.13's new `diff coverage` subcommand previously did unbounded `os.ReadFile` followed by `json.Unmarshal`; multi-GiB JSON inputs would OOM CI. Now matches the existing `coverage.MaxResultsFileBytes` defense-in-depth pattern.

Defenses verified intact during the audit: GHA SHA pinning, BETA-gate TTY detection (not EOF-on-empty), pre-push hook SHA validation, manifest 64 KiB + results-file 16 MiB caps, XML DTD rejection, RE2 regex immunity to ReDoS across all 82 regex sites, webview CSP with per-render nonce, execFile (not exec), settings machine-scope, no extension telemetry.

Two HIGH findings remain as v0.14 follow-ups:
- **H3.** VS Code extension does not verify cosign signatures on the downloaded binary (signatures already published; consumer needs `sigstore-js` integration).
- **H4.** VS Code `capabilities.untrustedWorkspaces.supported: "limited"` declared in `package.json` but no code path checks `vscode.workspace.isTrusted` before downloading/executing the binary.

### Pre-release validation

- **Real-world test against 12 open-source repos** (`docs/release-testing/v0.13.0-realworld.md`): 0 crashes, 0 validation errors across 29,973 files / 4,156 generated specs / 32,752 extracted assertions. 5.5× the v0.2.x baseline of 5,434 files.
- **Feature smoke test** (`docs/release-testing/v0.13.0-smoke.md`): 16/16 pass across D1/D2/D3/C2/C4/C5+C6/B/F3/A4 surfaces. Reproducible harness at `scripts/smoketest_v013.sh`.
- **Spec coverage** via `make dogfood-strict`: 15/15 specs at 100% (spec-check 20/20 with the new AC-20, spec-diff 15/15 with AC-15, spec-manifest 45/46, only pre-existing AC-29 uncovered).

### Deferred to v0.14

E1 / E2 / E3, major-version migration PRs (`eslint` 8 → 10 flat-config, `typescript` 5 → 6, `jest` 29 → 30) bundled into v0.14 to keep v0.13 focused. v0.12.1 / v0.13 cumulatively absorbed 28 dependabot bumps; the three majors deserve dedicated revert paths.

---

## v0.12.1 - 2026-05-07

**Theme: release-infra hardening + user-facing docs parity.**

Patch cycle. No new CLI features. The v0.12.0 release surfaced four release-time landmines (cosign bundle format, `release.yml` `branches` filter excluding tag refs, `--new-bundle-format=false` silently ignored, goreleaser template field crash) that cumulatively cost ~1 day of post-tag iteration. v0.12.1 ships the pre-flight gate that catches that class pre-merge, plus brings user-facing docs into parity with v0.10–v0.12 shipped behavior.

### Added

#### Release-pipeline pre-flight gate

New CI workflow at `.github/workflows/release-snapshot.yml`. Triggers on PRs that touch release infrastructure (`release*.yml`, `.goreleaser.yml`, `Makefile`, `go.mod`, `go.sum`) and runs `goreleaser release --snapshot --skip=publish --clean` against the PR head. Catches:

- Goreleaser config crashes (template-field errors, malformed `signs:` / `sboms:` blocks).
- Cosign flag and bundle-format regressions (the cosign step actually executes on same-repo PRs).
- Missing or malformed artifacts (verify-outputs step asserts ≥5 archives, ≥5 SBOMs, and the `dist/checksums.txt.sigstore.json` cosign bundle on same-repo PRs).

Fork-PR handling: cosign keyless OIDC tokens are not issued for fork-PR runs, so fork PRs run with `--skip=sign`. Same-repo PRs run the full snapshot. This catches goreleaser config bugs from any contributor while accepting that cosign-flag changes are verified only on same-repo PRs (which is where signing changes actually land, fork PRs cannot push tags).

All actions SHA-pinned to the same versions as the production `release.yml` for consistency. Concurrency guard cancels superseded runs on the same PR.

### Documentation

#### v0.10–v0.12 user-facing docs parity

Six classes of doc-vs-code mismatches corrected in tracked user docs:

- **Status lifecycle**: `SPEC_SCHEMA_REFERENCE.md` and `FAQ.md` no longer claim "only `approved` specs are enforced by sync." All discovered specs are checked; `settings.warn_on_draft` + `settings.strict` gate release posture.
- **Approval gate**: `SPEC_SCHEMA_REFERENCE.md` now reflects the v0.11.1 enforcement contract, metadata under threshold, demoted under `strictness: zero-tolerance`.
- **CLI catch-up**: `CLI_REFERENCE.md` now documents `check --test`, `coverage --strictness/--quiet`, `init --install-hook`, `init --ai`, `doctor --fix/--dry-run/--yes`, and `ingest --junit/--go-test` glob + repeated-flag support, flags that shipped in v0.10–v0.12 but the docs lagged. Manifest example updated to v0.11+ shape (`schema_version`, nested `system`, `domains`, `settings.coverage`).
- **Test annotation realism for Python**: `GETTING_STARTED.md` teaches the runtime `print('// @spec ...')` form for pytest (function names cannot contain `/` or `:`) and shows the `pytest --junitxml -o junit_logging=all` invocation `ingest` actually requires.
- **Content-agnostic positioning**: README + FAQ frame `.spec.yaml` as a component-contract format covering behavior, data invariants, security, schema, and architecture rules, not API behavior alone.
- **Install snippet correctness**: root README now detects OS+arch and resolves the latest release tag instead of always grabbing `specter_Linux_x86_64.tar.gz`.

VS Code README clarifies `specter.binaryPath` and `specter.version` settings as machine-scoped per the v0.11 hardening; drops the de-listed `Open QuickStart` command row.

#### Reference-doc style discipline

Embedded `specter explain annotation` reference and the public `TEST_ANNOTATION_REFERENCE.md` no longer carry "is planned for a future release" promises. Reference docs state current behavior; planning lives elsewhere.

### Internal

#### Planning surface moved local-only

`BACKLOG.md` and `docs/roadmap/` (added this cycle) become maintainer-local working files. `CHANGELOG.md` (history), `docs/ssrb/` (formal schema decisions), `docs/CLI_REFERENCE.md` and `docs/SPEC_SCHEMA_REFERENCE.md` (reference docs), and `docs/TRIAGE_DISCIPLINE.md` (process) remain public. Forward planning now follows a 1-feature-per-minor cadence through v1.0.0 (the minor number rule applies pre-1.0 only; post-1.0 cadence is reactive).

22 tracked-file references to `BACKLOG.md` scrubbed across `CONTRIBUTING.md`, the embedded `specter explain` reference, three SSRBs, four `internal/` Go files, three `.spec.yaml` files, `cmd/specter/main.go` stderr text, and others. `CONTRIBUTING.md` now points contributors at GitHub's branch list (`gh api repos/Hanalyx/specter/branches`) for finding the active `release/*` branch.

Two stale planning docs deleted: `V0_11_PLAN.md` and `V0_12_PYTHON_FOLLOWUP_PLAN.md` (both explicitly marked themselves as temporary working documents to delete after their cycles shipped).

#### SSRB-101, source-file governance evaluation

Combined evaluation of two competing proposals for "how does Specter know which source files a spec governs", annotation-based (extend `@spec` to source files) and declarative (`governs: [string]` schema field). Status: `NEEDS-DESIGN`. Decision target: end of v0.16 cycle, with field evidence accumulated during v0.13–v0.16.

### Fixed

#### Spec narrative descriptions

`spec-coverage` 1.11.0 (one description), `spec-doctor` C-16/v1.7.0 (two descriptions), `spec-manifest` C-27/v1.7.0 (two descriptions) updated to drop external planning-doc references in favor of inline rationale. No AC or constraint shape changes; no spec version bump.

#### `internal/migrate/rewrite.go` package comment

Comment said "C-10" should say "C-11", the package implements the rewrite-table constraint, not the discovery-fallback one. One-character correction.

#### `doctor --fix` BETA warning text

Stderr text in `cmd/specter/main.go` no longer references an external planning doc. Operators see the same safety-relevant content (known string-literal corruption gap, recommendation to commit before running, `--dry-run` preview path) without an out-of-band pointer.

### Release notes

CLI behavior unchanged from v0.12.0. No new flags, no new commands, no schema changes. The pre-flight workflow runs only on PRs touching release infrastructure (no impact on regular feature/fix PRs). The VS Code extension VSIX is bumped to 0.12.1 for version alignment; no extension behavior changes.

The pre-flight gate's design was specified in BACKLOG before becoming local-only; the workflow file is the canonical artifact. A test PR that intentionally regresses `.goreleaser.yml` confirms the gate fails on bad config.

---

## v0.12.0 - 2026-04-29

**Theme: migration tooling + supply-chain hardening.** v0.12 ships the migration toolchain parked since v0.10 (`doctor --fix`, `schema_version` manifest field, VS Code quick-fix) so projects upgrading from older Specter releases can repair schema drift in-place. Paired with the M-tier security hardening bundle (size caps, webview CSP, SHA-pinned actions, sigstore signing, CycloneDX SBOM) so the supply chain catches up with the feature surface.

### Added

#### `specter doctor --fix`, table-driven rewrite engine (BETA)

Apply known-safe rewrites to spec files that fail parse against the current schema. v0.12 ships one rewrite (`strip-trust-level` for the v0.6.5-removed field) and the table is open for additions. Manifest canonicalization (`add-schema-version`) prepends `schema_version: 1` to a pre-v0.12 `specter.yaml`.

`--fix` is gated as **BETA** because the regex deletion does not yet handle string-literal mentions of the deprecated field. The gate emits a `[BETA]` warning naming the known corruption gap, prompts `Continue? (y/N): ` on stdin, and proceeds only on affirmative input. `--yes` (or `-y`) bypasses for CI; `--dry-run` is exempt (preview mode is read-only). Non-TTY stdin without `--yes` is refused via TTY detection (`os.Stdin.Stat()` character-device check) BEFORE stdin content is read, `echo y | specter doctor --fix` cannot bypass the gate.

The rewrite engine refuses structurally unsafe shapes via yaml.v3 inspection: block scalars, sequences, mapping values, anchored values, multi-line quoted scalars, and folded plain scalars all fall into the `needs-manual-edit` summary block rather than producing corrupted output.

spec-doctor 1.1.0 → 1.8.0 (C-10..C-16, AC-10..AC-29).

#### `specter init` writes `schema_version: 1`

`specter init` (scaffold mode) now emits `schema_version: 1` as the first field in the generated `specter.yaml`, ahead of `system:`. `init --refresh` preserves any existing `schema_version` value byte-for-byte (verified across `1`, `7`, `42`). Activates the schema-stability policy: at v1.0.0 the then-current schema becomes the canonical `schema_version: 1` permanently, and subsequent breaking changes bump the integer with `doctor --fix` migration paths.

spec-manifest 1.8.0 → 1.9.0 (C-27/28, AC-41/42/43).

#### GH #77, language-aware `specter explain`

When `discoverTestFiles` returns at least one `.py` file, `specter explain annotation` emits Python source-comment examples (`# @spec`, `# @ac`) and the autouse-fixture pattern alongside the JS/TS examples. Closes the Python-onboarding friction where users copying `// @spec` examples got "annotation not detected" despite the source comment being syntactically correct for their language.

spec-explain 1.1.0 → 1.2.0 (C-13, AC-11/12/13).

#### GH #80, source-only diagnostic hint under `--strict`

When an annotated AC has source-file `@ac` comments but no matching `.specter-results.json` entry, `coverage --strict` now emits a per-AC stderr hint above the table:

```
hint: my-spec/AC-01 has source annotation in tests/foo.py:13 but no
matching pass in .specter-results.json, did your test runner emit a
runner-visible annotation (Convention A: spec-id/AC-NN in the test
name; Convention B: print '// @spec'/'// @ac' from the test body)?
```

Limited to the first 5 affected pairs to keep CI logs compact. Suppressed by `--quiet`. Surfaced as a top-level `diagnostic_hints` array under `--json`. Closes the GH #80 confusion where Python users with source-only `# @ac` comments saw `--strict` exit non-zero with no signal about the missing runtime channel.

spec-coverage 1.11.0 → 1.12.0 (C-28, AC-31/32/33).

#### VS Code ("Remove deprecated field" quick-fix)

Lightbulb action on `Unknown field 'X'` parse errors offers "Remove deprecated field 'X'" when X is in the known-removed list (currently `trust_level`). The quick-fix performs YAML-shape inspection before rewriting, fields whose value spans multiple lines, lives inside a block scalar, or has unclosed quotes are silently refused (the parse error is still surfaced, just without a fix offer). Pairs with `doctor --fix` for the CLI path.

spec-vscode 1.4.0 → 1.6.0 (C-28/29, AC-51/52/53).

### Fixed

#### GH #93, `doctor` no-manifest discovery alignment

`specter doctor` returned "no specs" when run without `specter.yaml`, while `parse` discovered nested `*.spec.yaml` files recursively. The asymmetry confused first-run users. `doctor`'s no-manifest fallback now matches `parse`'s behavior, recursive discovery from the working directory.

spec-doctor C-10/AC-10/AC-11.

### Security

Supply-chain hardening bundle. None of these change user-facing behavior; all are defense-in-depth for the build/release path.

- **M1, 16 MiB cap on `.specter-results.json`.** Prevents memory exhaustion when a malicious CI runner commits a multi-GB results file. `internal/coverage/results.go` rejects oversized input before `json.Unmarshal` allocates. New `results_test.go` exercises both the rejection and the at-limit acceptance paths.
- **M2, 64 KiB cap on `specter.yaml`.** Same shape for the manifest path, caps a malicious specter.yaml before `yaml.Unmarshal` exposes the parser to billion-laughs / anchor-expansion. New `TestParseManifest_RejectsOversizedInput` enforces.
- **M4, Webview CSP with per-render nonce.** `vscode-extension/src/extension.ts`'s insights webview now serves with `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; script-src 'nonce-${nonce}';` and `localResourceRoots: []`. Nonce is `crypto.randomBytes(16)` per render. `csp.test.ts` enforces six source-level invariants (CSP meta tag presence, default-src 'none', script-src nonce template, inline `<script>` nonce attribute, randomBytes entropy, empty localResourceRoots) so a future regression that drops or weakens the CSP fails CI.
- **M5, GHA SHA-pinning + Dependabot config.** Every `uses:` line in every workflow is pinned to a 40-char SHA with the version comment preserved. `.github/dependabot.yml` resolves both the SHA and the version tag for ongoing maintenance.
- **M6, Sigstore cosign keyless signing + CycloneDX SBOM.** Releases now publish a `checksums.txt.sig` + `.pem` cert pair (verifiable with `cosign verify-blob --certificate-identity-regexp '...release.yml' --certificate-oidc-issuer https://token.actions.githubusercontent.com`) and a `<archive>.sbom.json` in CycloneDX format per release archive. `-trimpath` and reproducible `mod_timestamp` enabled in `.goreleaser.yml`.
- **M7, `release.yml` chained on Pre-Release Test Suite.** Releases trigger via `workflow_run` after the test suite completes successfully. Concurrency guard prevents overlapping releases. `id-token: write` declared for sigstore OIDC. The previously redundant in-release test job is removed.
- **M8, `jest-junit` ^16 → ^17.** Routine dev-dep bump; existing inline reporter config is forward-compatible.

### Process

- **Specter Schema Request Brief (SSRB).** Every schema-change request now gets a written brief documenting the decision and reasoning at `docs/ssrb/SSRB-NNN.md`. Backfilled briefs for the four post-v0.11 closures (GH #97/#98/#99/#100). Triage discipline (universality test, schema conservatism) extracted from internal CLAUDE.md to `docs/TRIAGE_DISCIPLINE.md`.

### Internal

- `internal/migrate/rewrite.go`, table-driven rewrite engine with yaml-aware safety predicates (`canSafelyStripTrustLevel`, `valueOccupiesOneSourceLine`, multi-doc iteration via `yaml.Decoder`).
- `cmd/specter/doctor_fix_test.go` + `init_schema_version_test.go` + `migrate/rewrite_test.go`, comprehensive test coverage for the new rewrite paths.
- `vscode-extension/src/quickFix.ts`, pure runtime-free helpers for the quick-fix YAML-shape inspection (16 chomp/indent variants tested).
- Spec/test parity tightening surfaced by the v0.12 review: AC-16 calls `ParseManifest`, AC-31 enforces `hintIdx < tableIdx` ordering, AC-43 parameterizes over `(1, 7, 42)` to assert byte-equality of the schema_version line.

### Behavior changes

None for greenfield projects on v0.11.x. Pre-v0.12 projects without `schema_version` in their `specter.yaml`: `doctor --fix` will offer the `add-schema-version` manifest canonicalization (gated behind the BETA prompt). The default value of `schema_version` is `1`, `ParseManifest` defaults to 1 when the field is absent, so v0.11 manifests parse cleanly under v0.12.

---

## v0.11.1 - 2026-04-26

**Theme: post-v0.11.0 hotfix.** Two bugs reported within hours of v0.11.0 shipping; both fixed.

### Fixed

#### GH #94, `strictness=zero-tolerance` + `approval_gate` report demotion

v0.11.0 fired exit code 3 when an AC carried `approval_gate: true` with unset `approval_date` under zero-tolerance, but the report cell continued to show the AC as PASS. Reporter expected the report to also reflect the demotion. v0.11.1 demotes such ACs in the report (moves them from `CoveredACs` to `UncoveredACs`, recomputes per-entry `CoveragePct` + `PassesThreshold`, recomputes `Summary.Passing` / `Summary.Failing`). Threshold mode unchanged, `approval_gate` stays metadata there per spec contract.

**Behavior change for `strictness: zero-tolerance` only:** a spec with an `approval_gate: true` AC and unset `approval_date` now shows as `NONE / uncovered` in the report (was `PASS` in v0.11.0). Exit code 3 unchanged.

#### GH #95, `check --test` false positive on multi-`@spec` test files

Test files declaring two `@spec` headers at the top got the second header as the parent context for following `@ac` lines. An `@ac` legitimately in the FIRST declared spec was flagged `unknown_ac_ref`. Fix: each `@ac` is now validated against the union of declared specs in the file. Cross-cutting tests that bridge two specs work as expected.

### Internal

- `cmd/specter/main.go`: new `demoteApprovalGateViolations` post-processor.
- `internal/checker/test_annotations.go`: `scanFileAnnotations` tracks `declaredSpecs` (slice + dedupe set) instead of single `currentSpec`.
- 5 new regression tests across `cmd/specter/coverage_strictness_test.go` and `internal/checker/test_annotations_test.go`.

### No spec changes

Spec contracts unchanged from v0.11.0; the v0.11.1 fix aligns the implementation with the contracts already declared (spec-coverage AC-29 always implied report demotion, even though the explicit AC text only mentioned exit code).

---

## v0.11.0 - 2026-04-26

**Theme: AI loop discipline + adoption hardening.**

Five new features close order-of-operations gaps; four GH issues from real adopter friction are fixed (#75, #76, #78, #79 cheap fix). Full walkthrough in `docs/explainer/v0.11-ai-loop-discipline.md`.

### Added

#### `specter explain`, three new read-only surfaces

Stdout-only. File writes are scoped to `init --ai <tool>` (below).

- `specter explain annotation`, prints the test-annotation reference (Convention A title form, Convention B runtime-log form, source-comment annotations that pair with either).
- `specter explain schema`, prints the full schema field reference. Walks JSON `$ref` into `$defs` so nested fields under `spec.acceptance_criteria.items.*` and `spec.constraints.items.*` resolve.
- `specter explain schema <field-path>`, prints single-field detail (type, default, enum, description). Returns non-zero with a `did you mean?` suggestion within Levenshtein 3 on unknown paths.
- `specter explain <spec-id>` (no AC suffix), now renders a spec card: tier, coverage %, per-AC test files. Previously showed only COVERED/UNCOVERED labels.

Spec: `spec-explain` 1.0.0 → 1.1.0 (C-09..C-12, AC-07..AC-10).

#### `specter check --test` (`-t`), test-annotation cross-reference

Opt-in flag. Scans test files for `@spec` / `@ac` source comments and emits diagnostics for references that don't resolve against parsed specs. Three diagnostic kinds:

- `unknown_spec_ref`, `// @spec foo` where no spec with that id exists.
- `unknown_ac_ref`, `// @spec real-spec` + `// @ac AC-99` where the named spec doesn't declare that AC.
- `malformed_ac_id`, IDs failing `^AC-\d{2,}$` (`AC-1` not zero-padded, `ac-01` wrong case, `AC-1A` suffixed).

Skips lines inside multi-line string literals (TS template strings, Python triple-quoted strings). Cascade rule: when `@spec` is unknown, child `@ac` lines are not separately checked. `specter sync --strict` (and `settings.strict: true`) routes the check through.

Spec: `spec-check` 1.1.0 → 1.2.0 (C-09, AC-09..AC-12).

#### `specter init --install-hook`, git pre-push hook

Writes `.git/hooks/pre-push` (mode 0755) that blocks pushes whose diff changes implementation files but adds or updates no `@spec` / `@ac` annotations. Hook delegates to a hidden `specter pre-push-check` subcommand that reads git's pre-push stdin format, runs `git diff` per ref, and exits non-zero on impl-only diffs.

- Doc / spec / test changes always pass.
- Diffs touching impl AND adding an annotation delta in any test file pass.
- New branches without `origin/HEAD` merge-base are skipped (no "before" to compare against).
- `git push --no-verify` bypasses (git's behavior, not Specter's).

Hook script wrapped in shell-comment fenced markers (`# specter:begin v1` / `# specter:end`). Re-runs replace only the fenced region.

Spec: `spec-manifest` C-22, AC-27..AC-29.

#### `specter init --ai <tool>`, per-tool AI instruction file

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

#### `settings.strictness`, three-level coverage gate

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

#### `settings.tests_glob`, default test-discovery pattern

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

#### GH #75, silent 0% on empty test discovery (under `--strict`)

`specter coverage --strict` no longer falls through to `filepath.Walk(".")` when the configured glob matches zero files. Instead, it warns above the coverage table:

```
warn: no test files contained @spec/@ac annotations, coverage will report 0% for every spec
      set settings.tests_glob in specter.yaml or pass --tests <glob>
```

Under `strictness: zero-tolerance`, the warning escalates to a hard error.

Spec: `spec-coverage` C-27, AC-30.

#### GH #76, `specter.yaml` settings block silently accepted unknown keys

`ParseManifest` now errors on unknown top-level and `settings:` keys with did-you-mean suggestions (Levenshtein ≤ 3) and the full list of valid keys. Existing manifests with typo'd keys will start failing on parse, fix the typo or remove the field.

Spec: `spec-manifest` C-26, AC-40.

#### GH #79 (cheap fix), ingest body regex accepts `#` and `*`

`specter ingest`'s body-text annotation extractor (used to scan JUnit `<system-out>`) previously accepted `//` only. Now accepts `//`, `#`, `*`, the same three markers the source-file scanner already accepts. Cross-language Convention B output flows through ingest without language-specific kludges.

Pytest users can `print("# @spec my-spec")` from inside a test and ingest extracts identically to a Go test's `t.Log("// @spec my-spec")`.

The bigger fix (a `pytest-specter` plugin) is tracked for a follow-up.

Spec: `spec-ingest` 1.2.0 → 1.3.0 (C-12, AC-12).

### Security

A pre-release security review identified hardening opportunities across the VS Code extension, the CLI, and the build. All addressed in v0.11.0:

- **VS Code extension.** `specter.binaryPath` and `specter.version` are now declared `"scope": "machine"`, so workspace-level overrides are ignored. `specter.version` is additionally validated against strict semver (`^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$`) before being interpolated into download URLs. `package.json` declares `capabilities.untrustedWorkspaces` with `supported: "limited"`. The "View Diff" terminal command refuses paths containing shell metacharacters before reaching `terminal.sendText`. The `which` shim switched from `execSync` template strings to `execFileSync` array form.
- **CLI, pre-push hook** (new in v0.11.0): `ParsePushSpecs` validates `local_sha` and `remote_sha` against git's canonical 40-char hex form. `init --install-hook` uses `os.Lstat` and refuses to write through a `.git` symlink or worktree pointer file.
- **Build.** `go.mod` directive bumped from `1.25.8` to `1.25.9`, picking up five stdlib advisories (TLS, x509, archive/tar, html/template).

No CVEs were assigned; the findings were caught pre-release. The disclosure note in `docs/explainer/v0.11-ai-loop-discipline.md` covers the threat model and mitigations.

### Changed

- `loadManifest()` (internal) now returns an error when an existing `specter.yaml` fails to parse, rather than silently falling back to defaults. Library helpers (`noSpecsMessage`, `discoverSpecs`) tolerate missing manifests; RunE handlers fail-fast on invalid ones. Combined with the unknown-key rejection above, every typo in `specter.yaml` now surfaces at parse time.
- `discoverTestFiles` no longer falls through to walking `.` when an explicit glob matches zero files. The empty result is surfaced (and warned about) instead of hidden behind a noisy walk.
- `specter coverage --strict` exit codes: 0 (pass), 2 (strictness violation under zero-tolerance), 3 (approval-gate violation under zero-tolerance), errSilent (tier-threshold failure under threshold mode).

### Internal

- New package `internal/explain`, pure functions for schema walking and annotation reference rendering.
- New `internal/manifest/string_or_list.go`, custom `UnmarshalYAML` accepting scalar or sequence.
- New `internal/manifest/fenced.go`, `ReplaceFencedRegion` helper for idempotent file regions, used by both `init --install-hook` and `init --ai <tool>`.
- New `internal/manifest/{hook,ai_templates,prepush}.go`, pure logic for hook script, instruction templates, and pre-push diff classification.
- New `cmd/specter/glob.go`, `**`-aware glob matcher (no new dependency); used by `discoverTestFiles` when an explicit glob is provided.
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

## v0.10.2 - 2026-04-23

**Theme: docs/code parity fixes from jwtms Wave 0/1 integration.**

Two bugs surfaced during jwtms `--strict` rollout. Both are small; they ship together.

### Fixed

#### BUG-2, `specter ingest --junit` and `--go-test` accept globs and repeated flags

The v0.10.0 CHANGELOG claimed `--junit <path>` supports globs. The code used `os.ReadFile` on a single path with `StringVar`, so:

- `specter ingest --junit 'test-results/*.xml'` failed with `open test-results/*.xml: no such file or directory`.
- `specter ingest --junit a.xml --junit b.xml` silently overwrote, only the last file's results made it into the output.

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

#### BUG-3 part 1, `approval_gate` docs parity

`docs/SPEC_SCHEMA_REFERENCE.md` claimed `specter coverage` demotes ACs with `approval_gate: true && approval_date == null`. The embedded JSON schema (the authoritative field definition) and the code both said the opposite, Specter does not enforce approval semantics; teams wire their own PR/CI gates. The human doc was the outlier.

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

## v0.10.1 - 2026-04-23

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

- `docs/AI_PROMPTS.md` §3 (Spec → Tests), teaches both source comments and runner-visible annotations. AI prompt block now asks for `[spec-id/AC-NN]` in test titles.
- `docs/GETTING_STARTED.md` Phase 4, same update. TypeScript/Python/Go examples updated. Python encodes the pair in the function name; Go uses `t.Run` subtests.
- `docs/CLI_REFERENCE.md` coverage `--strict` section, adds the two-channel rule and the runner-visible format rules.
- `vscode-extension/walkthrough/step3.md`, onboarding walkthrough updated.
- All four docs cross-link into `TEST_ANNOTATION_REFERENCE.md` rather than duplicating the rules.

### No code changes

CLI and extension runtime unchanged from v0.10.0. This is a docs release, the version bump exists to mark "v0.10.0 shipped with misleading guidance, v0.10.1 corrects it." Extension version bumped from 0.10.0 to 0.10.1 to match.

---

## v0.10.0 - 2026-04-22

**Theme: CI-gated coverage, test outcome is mechanical.**

v0.9.x made test existence mechanical (`coverage` counts annotated ACs). v0.10 makes test outcome mechanical: `coverage --strict` demotes any annotated AC whose test did not pass. See `docs/explainer/v0.10-ci-gated-coverage.md` for the design rationale.

### Added

#### `specter ingest` (new command)

- Converts test runner output into `.specter-results.json`, the canonical results file `coverage --strict` reads.
- Flags: `--junit <path>` (JUnit XML, glob supported), `--go-test <path>` (`go test -json` output), `--output <path>` (defaults to `.specter-results.json`).
- Flavor-specific parsing is isolated here; adding a new runner is a change to `ingest` only. `coverage --strict` stays runner-agnostic.
- Reads the `(spec_id, ac_id)` pair from runner-visible surfaces, subtest names (`t.Run("spec-foo/AC-03 ...", ...)`) or runtime logs (`t.Log("// @spec ...")` / `t.Log("// @ac ...")`). Source-comment annotations are invisible to `ingest` by design.

#### `specter coverage --strict`

- New flag. When passed, every annotated AC must have a `status: passed` entry in `.specter-results.json`. Anything else (`failed`, `skipped`, `errored`, or no entry) demotes the AC to uncovered.
- Demotion applies to **all tiers**, not only Tier 1.
- Missing or empty `.specter-results.json` is a hard error: `--strict requires .specter-results.json, run 'specter ingest' first`. Fails closed so the flag cannot silently degrade to annotation-only behavior.

#### `.specter-results.json` status enum

- Adds `status` field: `passed` | `failed` | `skipped` | `errored`.
- `errored` is distinct from `failed`, it means the framework itself failed (setup panic, compile error) rather than an assertion.
- Worst-status-wins when the same `(spec_id, ac_id)` is observed across multiple tests: `errored > failed > skipped > passed`.
- The boolean `passed` field is retained for pre-1.9.0 consumers; no forced migration.

#### VS Code extension: CLI auto-download defaults to matching version

- `specter.version` config default changed from `"latest"` to `""` (empty). With the empty default, `downloadBinary` reads `ctx.extension.packageJSON.version` and fetches the matching CLI, a v0.10.0 VSIX always pulls v0.10.0 CLI. `"latest"` remains available as an explicit opt-in; pinned semvers (e.g. `"0.9.2"`) still work as before.
- Why: the GoReleaser workflow creates a GitHub Release on tag push, so the v0.10.0 CLI archive was live on `/releases/latest` before any v0.10.0 extension shipped to the Marketplace. Any v0.9.x extension with `autoDownload: true` then pulled v0.10.0 CLI via the old `"latest"` default, producing split-brain installs (v0.9.2 extension + v0.10.0 CLI). Pinning to the extension's own version closes the gap.

#### Adoption affordances: empty-results warning, `--scope`, `--verbose`

Three diagnostic/staged-adoption features found during v0.10.0 shake-down on jwtms. Without them, `--strict` is technically functional but operationally unusable on a workspace that hasn't migrated every test to runner-visible annotations.

- **`specter coverage --strict` empty-results warning.** When `.specter-results.json` parses cleanly but contains zero entries, the command now emits a stderr warning BEFORE the demotion report: *"no (spec_id, ac_id) pairs were extracted from test output, tests likely don't carry runner-visible annotations"* with a pointer to `docs/explainer/v0.10-ci-gated-coverage.md` (Conventions A and B). Prior behavior silently demoted 100% of annotated ACs with no clue why. (Note: missing file, as opposed to empty file, still errors per the existing AC-20 contract.)
- **`specter coverage --strict --scope <domain>`.** Narrows `--strict`'s demand set to specs listed under the named domain in `specter.yaml`. Specs outside the domain fall back to v0.9 boolean-passed logic (annotation alone counts for tier 2/3). Enables staged adoption: enforce `--strict` on one domain per wave instead of rewriting every annotated test before CI can pass. `--scope` without `--strict` fails fast. Combines with `--tests` as AND.
- **`specter ingest` default summary + `--verbose`.** Every run now emits to stderr: *"Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation."* Replaces the terse `Wrote N result entries`. `--verbose` adds a per-case drop reason line for each skipped testcase, off by default to keep CI logs compact.

### Spec bumps

- `spec-coverage`: 1.8.0 → **1.10.0** (+C-19/AC-19 strict demotion all tiers; +C-20/AC-20 missing-results error; +C-21/AC-21/AC-22 status enum w/ back-compat; +C-22/AC-23 empty-results warning; +C-23/AC-24/AC-25/AC-26 `--scope` domain flag)
- `spec-ingest`: new spec at **1.1.0** (17 ACs covering JUnit/go-test parsing, status derivation, worst-status-wins, output contract; + C-09/AC-09 default scan summary; + C-10/AC-10 `--verbose` per-case drops)
- `spec-vscode`: 1.3.0 → **1.4.0** (+C-27/AC-50 covering the version-pinning default)

### Out of scope for v0.10

- Flake handling (planned: `status: flaky` + `--deny-flaky` in v0.11).
- Source-file tracking under `--strict`.
- VS Code red-dot rendering for failed annotated ACs (fast-follow, not this cut).

---

## v0.9.2 - 2026-04-20

**Theme: UX polish from jwtms migration testing.**

Two items surfaced when running v0.9.1 against the fully-migrated jwtms workspace (249 specs). Both are quality-of-life fixes; no security or correctness issues.

### Added

#### `specter coverage` visual redesign

- **Summary header** above the table:
  ```
  Spec Coverage Report, 249 specs · 97.2% avg coverage
    Tier 1: 32/34 passing (94%)
    Tier 2: 168/192 passing (88%)
    Tier 3: 11/23 passing (48%)
  ```
  Gives one-glance visibility into the overall shape before scanning the table. Tiers with zero specs are omitted.
- **Worst-first sort** in the default table: failing (below threshold) → partial (below 100% but passing threshold) → 100% covered. Within each bucket, tier descending (T1 > T2 > T3) so higher-risk work surfaces first.
- **`--failing` flag** filters the table to entries below 100% coverage. Summary header still reflects the full report. When every spec is at 100%, emits a single-line confirmation (`All N specs at 100% coverage.`) instead of an empty table.
- **Long spec ID truncation**: IDs over 40 characters are truncated with a trailing ellipsis (`…`) so the Tier column stays aligned. `--json` output is unaffected, it emits the full spec_id.

#### `specter init --refresh` for non-greenfield workspaces

- **`--refresh` flag**: updates only `domains.default.specs` in an existing `specter.yaml`. Preserves every other field, `settings`, `registry`, tier overrides, system metadata, and any custom domains the operator declared.
- **Smart diff**: specs on disk that are claimed by a non-default domain stay in that domain (aren't duplicated into `default`). Specs that used to be in `default.specs` but are no longer on disk are removed.
- **Summary line**: `updated specter.yaml: +A added, -B removed`.
- **`--dry-run` variant**: `specter init --refresh --dry-run` prints the proposed diff without writing the file. Matches `git add -p` / `terraform plan` discipline.
- **`--refresh` and `--force` mutually exclusive**: `--force` rewrites everything; `--refresh` preserves everything except `domains.default.specs`. Attempting both exits non-zero with a clear message.

### Spec bumps

- `spec-coverage`: 1.7.0 → **1.8.0** (+C-15/AC-15 sort, +C-16/AC-16 summary header, +C-17/AC-17 --failing, +C-18/AC-18 truncation)
- `spec-manifest`: 1.5.0 → **1.6.0** (+C-17/AC-23 through +C-21/AC-26 covering --refresh, --dry-run, custom domains, removed specs, flag conflict)

14 specs dogfood at 100% AC coverage. All Go + TS tests pass. No security changes.

---

## v0.9.1 - 2026-04-19

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

- `spec-vscode`: 1.2.0 → **1.3.0**, adds C-22 through C-26 (parity, disposables, activation, checksum, error surfacing) and AC-41 through AC-49.
- `spec-coverage`: 1.6.0 → **1.7.0**, adds C-14 / AC-14 (empty array emits `[]`, never `null`).

All 14 specs dogfood at 100% AC coverage. 209 TypeScript tests pass. All Go tests pass under Go 1.25.8 + golangci-lint v2.6.2.

### Deferred to v0.10

From the audit's MEDIUM tier: HTTPS-redirect validation in `httpsGet`, cache-directory permission hardening (`mode: 0o700`), subprocess `maxBuffer` caps, tar-slip defenses via `node-tar`, YAML-bomb anchor limits, snake/camel conversion for `check --json` and `parse --json`, TOCTOU race on cache-path `exists()` check.

---

## v0.9.0 - 2026-04-19

**Theme: coherent failure-handling and intelligent diagnosis.**

When specs fail to parse, every seam of the tool used to lie in a different way: the coverage command swallowed JSON output, the VS Code sidebar pointed at `specter init` (wrong state), the Insights panel claimed "All specs passing ✓" on top of 17 broken files, and `specter doctor` printed 20 identical error lines that together named a schema mismatch nobody could see. v0.9.0 fixes the whole pipeline end-to-end. <!-- doc-style: allow --><!-- quotes the literal string vscode-extension/src/insights.ts renders -->

The trigger was a real workspace: `kensa-go` specs were written against the pre-v0.6.5 schema, and every tool in the suite disagreed about what that meant.

### Breaking changes

- **`specter coverage --json` now always emits a CoverageReport**, including when specs fail to parse. Exit code (not the presence/absence of JSON) signals pass/fail. Previous behavior: no JSON on parse error, tools had no structured data to work with. Any programmatic consumer that relied on "no JSON = failure" needs to check `exit_code` instead.

### Added

#### CLI (`cmd/specter`, `internal/coverage`)

- **`parse_errors` field** on `CoverageReport`, per-file schema violations (file, path, type, message, line, column).
- **`parse_error_patterns` field.** Errors grouped by `(type, path)` sorted by count descending. Enables one-sentence drift diagnosis: "20 specs: missing `objective` at `spec.objective`" instead of 20 individual messages.
- **`spec_candidates_count` field.** Count of `.spec.yaml` files on disk before any parse was attempted. Distinguishes "no specs exist" from "specs exist but drift."
- **`spec_file` field** on each entry, path to the source `.spec.yaml`. Populated by the CLI from discovery; previously not exposed.
- **`specter doctor` pattern analysis.** When the parse check fails, doctor prints a `Pattern analysis:` block that names schema version drift explicitly when every discovered spec hit the same error shape. Heterogeneous errors get a top-N list with counts.
- **`specter init` discovers existing specs.** Scans `specs/`, populates `domains.default.specs` from parseable spec IDs, prints a warning with pattern analysis for any that fail. Always emits a `domains:` section with a placeholder default domain when empty (fixes a silent-exclusion footgun where an empty domains map caused `specter sync` to ignore every later spec).

#### VS Code extension

- **Parse errors populate the Problems panel.** Each failing spec appears as a clickable `vscode.Diagnostic` entry at the reported line/column, prefixed with the error type (e.g. `[required] field is missing (at spec.objective)`).
- **Mixed-render Coverage sidebar.** Passing specs and a "Failed to parse" group render in the same tree. Each failing file is a clickable leaf that opens the file at the reported line. Previously the sidebar was all-or-nothing: tree OR error banner.
- **Click-to-open on tree nodes.** Spec nodes open their `.spec.yaml`, test-file leaves open the test file, failing spec leaves open the broken spec. Relative paths from the CLI are resolved against the workspace root.
- **Honest Insights panel.** Renders a `Parse failures` section listing each broken file with its error, alongside the normal `Coverage gaps` section. Header reflects the true mixed state ("17 parse error(s), 4 spec(s) parsing cleanly"). The "All specs passing ✓" headline now appears only when it's literally true. <!-- doc-style: allow --><!-- quotes the literal string vscode-extension/src/insights.ts renders -->
- **Clickable file-path headers** in Insights parse-error cards, webview posts an `{openFile, line}` message to the extension host, which opens the file.
- **`specter.revealInTree` command wired end-to-end.** Takes the active editor's file and reveals the matching node in the Coverage sidebar. Previously declared in `package.json` but never registered, surfacing as "command 'specter.revealInTree' not found."
- **Honest `specter.runSync` completion toast.** Info-level success vs warning-level "finished with errors in N folder(s)" with a "Show Output" button.
- **`@ac` hover populates covering files** from the live CoverageReport instead of always rendering as "uncovered" (latent UX regression).
- **Annotation extractor respects multi-line string literals.** `// @spec` inside a TypeScript template literal, Go raw string, or Python triple-quoted string is no longer treated as a real annotation.
- **Sidebar message names schema drift** when the pattern signature is unambiguous ("Every one of N .spec.yaml files hit the same failure: **required** at `spec.objective`").

### Fixed

- **Latent runtime bug: `entry.specID` was always undefined at runtime.** The VS Code types declared camelCase (`specID`, `coveragePct`, `parseErrors`) but the CLI emits snake_case JSON. A new `snakeToCamelCoverage` converter in the client layer handles the mapping; every downstream consumer now sees the shape its types promise.
- **Defensive guards against null arrays.** Go's `omitempty` emits `null` for empty slices, so `entry.coveredACs` could be `null` at runtime. Hardened every site that iterates entries/ACs/test files/parseErrors.
- **Insights panel crashed with `entries is not iterable`** when parses failed (`entries` was `null`).
- **Template-literal annotation bleed.** A `// @spec foo` mentioned inside a template literal (typical test-fixture content) no longer registers as a real annotation.
- **Annotation regex anchored to line start.** A prose comment that happened to quote `// @spec other-spec` no longer hijacked the surrounding `currentSpecID`. Caught when spec-coverage's own regression tests described string-literal handling.

### Spec bumps

- `spec-coverage`: 1.4.0 → **1.6.0** (C-10/AC-10 always-emit contract; C-11/AC-11 string-literal safety; C-12/AC-12 `spec_candidates_count`; C-13/AC-13 `parse_error_patterns`)
- `spec-doctor`: 1.0.0 → **1.1.0** (C-09/AC-09 pattern analysis + drift diagnosis)
- `spec-manifest`: 1.4.0 → **1.5.0** (C-16/AC-22 ScaffoldManifest always emits `domains:` section)
- `spec-vscode`: 1.1.0 → **1.2.0** (AC-29 rewritten; AC-30 no-specs-yet; AC-31 honest runSync toast; AC-32 hover populates coveredByFiles; AC-33 click-to-open; AC-34 Problems-panel plumbing; AC-35 drift diagnosis in sidebar; AC-36 mixed-render tree; AC-37 honest Insights; AC-38 revealInTree; AC-39 clickable Insights file headers)

All 14 specs dogfood at 100% AC coverage. 192 TypeScript tests pass. All Go tests pass under Go 1.25.8 + golangci-lint v2.6.2.

---

## v0.8.3 - 2026-04-18

### Fixed

- **`specter resolve --dot` and `specter resolve --mermaid` polluted stdout with a plain-English footer** (`No dependency issues found.`) after the structured output block. Piping to `dot -Tpng` or Mermaid renderers failed to parse. Fix: suppress the footer when `--dot`, `--mermaid`, or `--json` is set, the successful exit code already signals the no-issues status. Two regression tests added.

### Audit (no changes needed)

Full CLI audit performed, no other flag bugs found:
- `parse --json`, `check --json`, `coverage --json`, `sync --json`, all emit clean structured output, no trailing text
- Exit codes correct: unknown command / missing args / bad flag all exit 1
- `--version` works on root and via `-v`
- `sync --only <phase>` validates against the allowed set
- `init --template <name>` validates against the allowed set, errors on unknown
- `explain <unknown>` errors cleanly
- `diff` no-args errors cleanly (2 positional args required)

---

## v0.8.2 - 2026-04-18

### Fixed

- **Critical: extension passed CLI flags that don't exist.** `SpecterClient` called `specter parse --json --manifest <path>`, `specter check --json --manifest <path>`, `specter coverage --json --manifest <path>`, and `specter diff --json --base <ref> <file>`. None of the `--manifest`, `--spec`, `--base`, or `--json` (on diff) flags exist in the CLI. Every invocation threw "unknown flag" and the try/catch in `runCoverageForFolder` surfaced it as "No coverage data loaded yet" in the sidebar, so users following v0.8.1's fix for the manifest-discovery bug would reload, the extension would find specter.yaml correctly, then fail to run any specter command because of the flag mismatch.

  Fix: strip all fabricated flags. The CLI discovers `specter.yaml` by walking up from cwd, so `execFile` is now called with `cwd: path.dirname(manifestPath)`. Diff uses its actual positional `<path>[@<ref>]` syntax.

- **New integration test suite (`client.test.ts`) invokes the real built CLI binary** against a tmpdir workspace. Would have caught every one of the fabricated flags immediately. Previously all extension tests were unit-level against TypeScript mocks that described intent, not contract.

  GOTCHAS #17 documents the "mocks describe intent, not contract" lesson.

---

## v0.8.1 - 2026-04-18

### Fixed

- **Critical: "no specter.yaml found" when the file IS at the workspace root.** `resolveManifestPath` in the VS Code extension called `path.dirname()` on the workspace folder path before starting its search. `path.dirname("/home/user/project")` returns `/home/user` (the parent), so the resolver searched `/home/user/specter.yaml`, `/home/specter.yaml`, and so on, **never checking `/home/user/project/specter.yaml`** which is the canonical location the docs explicitly recommend. Affected every user since spec-vscode v1.0.

  Fix: `resolveManifestPath` now accepts an optional third argument `isDirectory` so the caller can say "this path IS the starting directory." The single runtime caller (`setupFolder`) supplies a real `statSync().isDirectory()` probe. Two regression tests pin both calling shapes.

  GOTCHAS #16 documents the trap.

  After updating, reload your VS Code window, the Coverage sidebar will populate.

---

## v0.8.0 - 2026-04-18

Followed the project's own SDD workflow: plan → specs first → failing tests → implement → validate → ship.

### Fixed

- **Wrong GitHub URL in `specter init` scaffold.** The header comment pointed at `github.com/Hanalyx/spec-dd` (wrong slug, that's the parent monorepo, not the Specter project). Now correctly emits `github.com/Hanalyx/specter`. spec-manifest C-15/AC-21 pin the canonical URL.

### Added

- **Coverage sidebar state messages.** When the Coverage tree has no data to display (report not yet loaded, or every spec failed parse), the panel now shows a synthetic node with a state explanation and a concrete next step. Previously the panel was silently empty, a dead-end UX. Two states distinguished:
  - *No coverage data loaded yet*, points at `specter init`, `specter reverse`, or `Specter: Run Sync`.
  - *All discovered specs failed to parse*, points at the Problems panel where the parse errors surface.

  spec-vscode C-21/AC-28/AC-29 pin the behavior. Pure `buildCoverageTreeRoot` function in `coverage.ts` carries the decision logic, unit-tested without VS Code mocks.

### Changed

- **Marketplace categories**: `Linters + Testing + Other` → `AI + Linters + Testing`. Drops the uninformative "Other" and adds "AI" to signal the AI-assistant integration use case (Specter's `Copy Spec Context for AI` command, spec-as-contract-for-AI workflow).

### Migration

- No schema changes; no breaking behavior changes. Upgrade is drop-in.

---

## v0.7.1 - 2026-04-18

### Fixed

- **Silent exit on unknown command or bad flag.** Typos like `specter covera` or `specter parse --wrong-flag` previously exited with code 1 and no output. Root cause: `SilenceErrors = true` on the root Cobra command was suppressing both our intentional silent-exit path AND Cobra's own usage errors. Now only the `errSilent` sentinel is truly silent; everything else prints the error message plus a pointer to `specter --help`. Cobra's "Did you mean?" suggestions now surface for near-miss typos.

### Changed

- **"No .spec.yaml files found" message** now explains where specter looked (the specs_dir from specter.yaml, or the default) and lists three concrete next steps (`specter reverse src/`, `specter init --template`, or editing specter.yaml). Previously it was a one-line dead-end.
- **`specter reverse` handoff.** Success output now concretely points at the first generated spec with a step-by-step triage walkthrough: `specter explain <spec-id>`, triage gaps, `specter parse`, `specter sync`. Previously it said "review each gap AC" without telling you where to start.
- **Parse-error hints refreshed.** Enum error messages for `status`, `constraint.type`, `constraint.enforcement`, `depends_on.relationship`, and `changelog.type` were out of date (listed old values or missing new ones). Added hints for `tier`, `constraint.validation.rule`, and a special case for `context.*` unknown-field errors that explains the v0.7.0 tightening and gives three remediation options.

### Docs

- **`docs/RELEASE_PLAN.md` archived** to `docs/archive/RELEASE_PLAN.md` with a prominent "stale" notice. Current release status → `CHANGELOG.md`.

---

## v0.7.0 - 2026-04-17

### Breaking

- **`context.additionalProperties` tightened to `false`.** Unknown keys under `context` now fail `specter parse` with a named error. Previously they were silently dropped because the schema said "extras are allowed" but `SpecContext` (types.go) was a closed struct. This was the only silent-data-loss site in the schema. Users with `context.role`, `context.callers`, or similar custom keys must either rename to an existing field (e.g. move narrative into `context.description`) or open an issue to propose a new schema field.

- **`references_constraints` cross-reference validation moved to parse time.** An AC that references a constraint not declared in the same spec (e.g. `references_constraints: ["C-99"]` when only C-01 exists) now fails `specter parse` with a `dangling_reference` error. Previously this was caught later by `specter check` as a warning. No impact on specs with clean references.

### Added

- `acceptance_criterion.notes`, optional free-form narrative per AC. Complements the top-level `changelog` (which is version-over-version) with lifetime-of-the-AC annotation.
- `acceptance_criterion.approval_gate` (bool) and `approval_criterion.approval_date` (date), optional audit metadata for regulated work. Specter does not enforce approval semantics; teams wire this into their own CI/PR gates.
- `spec.title`, optional human-readable display name. VS Code extension, tree views, `specter explain`, and PR renderings use this when present, falling back to `id`.
- Parse-time format validation for `date`-typed fields (`approval_date`, `changelog.date`, `generated_from.extraction_date`). Previously draft 2020-12's default was annotation-only; invalid dates slipped through.
- Internal `schema.ValidateEnums()` method and exported enum constants (`StatusApproved`, `EnforcementError`, etc.) for Go code that constructs specs without going through `ParseSpec` (reverse compiler, migration scripts).

### Changed

- VS Code extension now renders `spec.title` in the coverage tree view and `specter explain` output; falls back to `id` when absent.
- VS Code AC hover shows `notes` when present.
- `approval_gate: true` ACs get a subtle gutter indicator in the VS Code extension.

### Documentation

- `SPEC_SCHEMA_REFERENCE.md`, context extension escape hatch removed from docs; replaced with "propose a new schema field."
- `GOTCHAS.md` #14 added: documents the silent-context-drop trap and its v0.7.0 fix.

### Migration notes

- Specter's own dogfood: no changes needed. All 14 specs conform to the strict shape.
- External projects: run `specter parse` with v0.7.0 on your spec corpus. Any `context.*` unknown keys or dangling `references_constraints` will now surface as errors, fix them or propose new fields.
- CI consumers: pin `specter@v0.6.9` if you can't adopt v0.7.0 yet; otherwise update pin and fix surfaced errors.

---

## v0.6.9 - 2026-04-17

- VS Code: on activation, offer existing users the new **Specter: Add CLI to Shell PATH** command when the detected shell's rc file doesn't reference `~/.specter/bin` (dismissable with persistent "Don't show again").
- Docs: fixed broken install URLs across README, QUICKSTART, CLI_REFERENCE, GETTING_STARTED. Previously all used `uname`-based patterns that don't match goreleaser's lowercase `linux`/`amd64` naming, users got 9-byte "Not Found" files instead of binaries.
- QUICKSTART: fixed misplaced `gap: true` example (was at spec level; schema only allows on ACs) and wrong coverage example (T2 33% was shown as PASS; threshold is 80%).
- GOTCHAS #13 added: four-vocabulary arch/OS translation trap (uname / Node / VS Code runner / Go GOARCH).

## v0.6.8 - 2026-04-17

- VS Code: new **Specter: Add CLI to Shell PATH** command. Detects shell, appends idempotent export to the right rc file (`.bashrc`/`.bash_profile`/`.zshrc`/`config.fish`). Unknown shells get a clipboard fallback. 13 new unit tests.
- Extension README refreshed, commands table had been missing 5 commands.

## v0.6.7 - 2026-04-17

- VS Code: fixed arch mismatch that caused 404 on auto-download (`specter_0.6.6_linux_x64.tar.gz` → not found). `normaliseArch` now lowercases its input so `process.arch: "x64"` maps correctly to `amd64`.
- GOTCHAS #13 added.

## v0.6.6 - 2026-04-17

- VS Code: fixed release pipeline, `vsce package` was shipping stale `out/*.js` because it doesn't run the build. Added `vscode:prepublish` hook so builds always run before packaging.
- GOTCHAS.md introduced with 13 entries documenting traps hit during v0.6.x.

## v0.6.5 - 2026-04-17

- **Breaking**: `constraint.enforcement` now overrides tier-based severity in `specter check` diagnostics (previously parsed but unused).
- **Breaking**: `gap: true` ACs count as uncovered for threshold purposes. Previously a 100%-gap spec auto-passed threshold; this hid real coverage gaps.
- `trust_level` field removed from schema (was parsed but never enforced by any pipeline stage).
- `constraint.type` surfaces inline in `specter check` output: `warn [orphan_constraint] spec-auth C-04 (security): ...`.
- VS Code: validates resolved CLI binary regardless of source (cache/PATH/workspace-setting). Previously the validation was gated on `source === 'cache'`, so a corrupt binary on PATH slipped through. Output channel for errors, `Specter: Re-download CLI` recovery command, 30s timeout on downloads.

## Earlier versions

See git tags for v0.6.0–v0.6.4 and v0.3.0–v0.5.2. Tags: `git tag -l | sort -V`.

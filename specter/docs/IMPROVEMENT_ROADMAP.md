# Specter Improvement Roadmap

**Mission:** "A type system for specs." The core pipeline (parse → resolve → check → coverage → sync) is the mission. Everything else serves adoption.

**Design principle:** Make it easy for developers to use. If Specter does its job well, developers will love it.

**Date:** 2026-04-14
**Based on:** Real-world testing against 12 open-source repos (5,434 files, 0 crashes, 27,012 assertions extracted) + Kensa team review (2026-04-14)

---

## The Mission Pipeline Today

```
parse → resolve → check → coverage → sync
  ✅       ✅        ✅       ✅        ✅
```

All 5 tools work. Dogfooding passes (7 specs, 85+ tests). The core engine has been validated against 12 real-world repos and one demanding external team (Kensa). No architectural gaps — remaining work is UX, CI integration, and authoring-loop friction.

---

## Three Improvement Tracks

### Track 1: Make the Core Pipeline Bulletproof

The pipeline works, but is it ready for a developer who just installed Specter and is writing their first spec?

| Gap | Why it matters | Status |
|-----|---------------|--------|
| ~~No `specter init` command~~ | ~~Developer downloads Specter, types `specter`... now what?~~ | ✅ v0.3.0 |
| Error messages are technical | Parse errors show JSON Schema paths like `spec.constraints[0].id`. A developer needs "Constraint ID 'c01' is invalid — must match pattern C-01, C-02, etc." | Open |
| Dangling-reference messages lack suggestions | `error: "handler-interface" does not exist` — no hint of what does exist or what to do | Open |
| No spec templates | Every spec starts from zero. A `specter init --template api-endpoint` would give them a starting point. | Open |
| `specter sync` only runs locally | No GitHub Action. The developer has to wire CI themselves. A `hanalyx/specter-sync-action` would make CI integration one line. | Open |
| Coverage thresholds are hardcoded | T1=100%, T2=80%, T3=50%. No way to configure per-project. A team might want T2=90%. | Open |
| `tier` vs `tier_overrides` conflict is silent | When a spec declares `tier: 1` and `specter.yaml` assigns a different tier via `tier_overrides`, no warning is emitted and precedence is undocumented | Open |

### Track 2: Fix What's Broken (P0/P1)

These are bugs that undermine trust. A developer tries Specter, it rejects valid output or silently drops data, and they walk away.

| Bug | Impact | Fix effort | Status |
|-----|--------|------------|--------|
| ~~`validation.rule` enum too narrow~~ | ~~18 rejections across 4 repos — Specter rejects its own output~~ | Small | ✅ v0.2.2 |
| ~~Missing `.spec.tsx`/`.spec.jsx` in IsTestFile~~ | ~~713 assertions silently lost in one repo~~ | Trivial | ✅ v0.2.2 |
| ~~Python false positive constraints from comments~~ | ~~`# isort MUST be provided` — nonsensical output~~ | Small | ✅ v0.2.2 |
| ~~Spec ID collision on generic filenames~~ | ~~54 specs all named `index` in one repo~~ | Medium | ✅ v0.2.2 |
| Test description truncation on embedded quotes | Garbled AC descriptions | Small | Open |

### Track 3: Developer Experience (the love factor)

This is what turns "useful tool" into "tool developers love."

| Feature | Why |
|---------|-----|
| Better CLI output | Progress indicators, color, summary tables. Right now it's raw text. |
| `specter reverse` summary report | After running reverse, show: "Found 14 constraints, 23 assertions, 5 gaps. 3 files need your attention." Not just raw YAML dumps. |
| `specter doctor` | Pre-flight check: "Your project has 0 spec files. Run `specter init` to get started. Your tests have 0 @spec annotations. See docs/GETTING_STARTED.md." |
| Spec-writing guide in error output | When `specter check` finds an orphan constraint, link to the doc that explains what to do about it. |
| VS Code extension | Syntax highlighting + validation for `.spec.yaml` files. Developers live in their editor. |

---

## Recommended Priority Order

### Phase 1 — Trust (v0.2.2) ✅ COMPLETE

Fix P0/P1 bugs. Specter must never reject its own valid output. This is table stakes.

- ✅ Map unknown `validation.rule` values to `"custom"` in core engine
- ✅ Add `.spec.tsx`, `.spec.jsx` to TypeScript adapter IsTestFile
- ✅ Fix Python adapter false positive constraints from comments
- ✅ Fix spec ID collision for generic filenames (route.ts, main.go, index.ts)
- Fix test description regex to handle embedded quotes

### Phase 2 — First-Run Experience (v0.3.x)

A developer should go from install to their first passing `specter sync` in under 5 minutes. Also: correctness fixes that surface during real-world authoring.

- ✅ `specter init` — scaffold a first spec from the manifest
- Spec templates (`api-endpoint`, `service`, `auth`, `data-model`)
- `specter doctor` — pre-flight check for project readiness
- **Human-readable error messages** — replace JSON Schema paths with developer-facing language; specifically:
  - Dangling-reference errors must include: existing spec IDs, Levenshtein-distance closest match, and a suggested file path + `id:` fix. Example:
    ```
    error [dangling_reference] Spec "engine-transaction" depends on "handler-interface" which does not exist
      searched: specs/**/*.spec.yaml
      existing specs: handler-file-permissions, engine-transaction
      did you mean: handler-file-permissions?
      fix: create specs/handler/interface.spec.yaml with `id: handler-interface`
    ```
  - Orphan constraint and unmapped AC errors must link to the annotation guide
- **`tier` vs `tier_overrides` conflict warning** — document that spec-level `tier` wins; emit a diagnostic when the two disagree:
  ```
  warn [tier_conflict] specs/engine/transaction.spec.yaml declares tier: 1
                      but specter.yaml tier_overrides assigns tier: 2 to specs/engine/
                      using tier: 1 (spec-level wins)
  ```

### Phase 3 — Authoring Loop & CI Integration (v0.4.0)

The authoring loop (draft → check → fix → recheck) is where developers spend most of their time. This phase makes that loop fast, informative, and CI-enforced.

**Authoring loop:**

- **`specter explain <spec-id>:<ac-id>`** — active diagnostic command for uncovered ACs. Shows what annotation pattern coverage looked for, which files were scanned, and a ready-to-paste example annotation. Example:
  ```
  $ specter explain engine-transaction:AC-07
  AC-07 is uncovered. Specter searched:
    tests/**/*_test.go with annotation pattern:
      // AC-07: ...   (Go comment)
      t.Run("AC-07/...", ...)  (subtest name)

    0 files matched. Suggested test location:
      tests/engine/transaction_test.go

    Example annotation:
      // AC-07: Concurrent Run calls against the same host serialize.
      func TestTransaction_AC07_PerHostSerialization(t *testing.T) { ... }
  ```
- **`specter watch`** — re-invoke the sync pipeline on filesystem change, 200–500ms loop. Same semantics as `tsc --watch`. Makes the draft → check → fix cycle interactive rather than manual.
- **`specter diff <spec>@<ref1> <spec>@<ref2>`** — semantic diff between two git revisions of a spec. Shows added/removed/changed ACs, constraints, and dependency version pins. More useful than YAML textual diff for PR review and release notes. Example:
  ```
  $ specter diff specs/engine/transaction.spec.yaml@HEAD~5 specs/engine/transaction.spec.yaml
  spec engine-transaction 0.1.0 → 0.2.0
    +constraint C-08 (security, error): "Host mutex must be released on panic"
    +ac AC-12: "Host mutex is released when Run panics"
    ~ac AC-02 priority: high → critical
    -ac AC-09: removed (superseded by evidence-envelope spec)
    ~depends_on handler-interface: any → ^1.0.0
  ```
- **`resolve --mermaid`** — Mermaid diagram output alongside existing `--dot`. Renders natively in GitHub PRs where reviewers actually look at graphs.
- **`specter sync --only <phase>`** — run a single pipeline phase without halting on prior-phase failures. Useful when you want to see all coverage gaps even with unresolved dangling references.

**CI integration:**

- **`specter check --strict`** — treat `enforcement: warning` diagnostics as errors; exit non-zero. Eliminates per-project CI shell gymnastics to re-exit on warnings.
- **`settings.strict: true`** — project-level equivalent of `--strict` in `specter.yaml`, so the policy is set once and applied everywhere.
- **`settings.warn_on_draft: true`** — emit a warning (or error under `--strict`) for any spec with `status: draft` encountered during sync. Prevents accidentally shipping unapproved specs in a release branch.
- **Pass-rate-aware coverage for Tier 1** — coverage currently counts an AC as covered if the annotation exists, regardless of whether the test passes. For Tier 1 specs a failing test is worse than no test. Implementation: adopt the `.specter-results.json` convention (language-agnostic; test infrastructure writes pass/fail results, coverage reads them). Tier 1 AC coverage requires both annotation presence and a passing result entry.
- **Configurable coverage thresholds** — per-project in `specter.yaml` (e.g. `thresholds.tier1: 100`, `thresholds.tier2: 90`). Per-spec override in the spec file for specs that need stricter or looser policy than the project default.
- **`hanalyx/specter-sync-action`** — GitHub Action for one-line CI setup. Runs the full sync pipeline and posts a coverage diff comment on PRs.
- **PR comment integration** — show spec coverage diff in PR comments (AC added/removed, coverage delta by tier).
- **Glob patterns in `settings.exclude`** — the exclude list currently matches by directory name only (e.g. `- .claude` skips the `.claude/` tree). Extend to support glob patterns so teams can write `- .claude/**` or `- **/worktrees` to express finer-grained exclusions without enumerating every root-level directory.
- **Dependency coverage warning** — when spec A `depends_on` spec B, warn if spec B has uncovered ACs at or above spec A's tier threshold. A joint resolver+coverage check: you cannot fully trust a dependency you haven't verified. Example:
  ```
  warn [dependency_coverage] engine-transaction depends on handler-interface (requires)
    handler-interface has 2 uncovered Tier 1 ACs: AC-03, AC-07
    engine-transaction is Tier 1 — all dependencies must meet the same coverage bar
    run: specter explain handler-interface:AC-03
  ```

### Phase 4 — Editor Experience & Schema Evolution (v0.5.0)

The "love" feature: real-time validation as developers write specs. Plus tooling for breaking schema changes.

- **VS Code extension** for `.spec.yaml` files
  - Syntax highlighting, schema validation, autocomplete
  - Inline diagnostics (orphan constraints, missing ACs)
  - Go-to-definition for `depends_on` references
- **`specter migrate v1→v2 specs/`** — scaffolded migration when `spec-schema.json` bumps a major version. Rewrites existing spec files to the new format and reports fields that require manual intervention. Necessary for adoption at scale — teams with hundreds of specs cannot migrate by hand.

---

## Real-World Test Results (evidence base)

Tested against 12 open-source repos on 2026-04-03:

| Repo | Language | Files | Specs | Assertions | Val. Failures |
|------|----------|-------|-------|------------|---------------|
| go-chi/chi | Go | 74 | 42 | 58 | 0 |
| gofiber/fiber | Go | 243 | 89 | 1,265 | 0 |
| go-playground/validator | Go | 78 | 28 | 105 | 6 |
| pydantic/pydantic | Python | 403 | 265 | 4,254 | 0 |
| fastapi/full-stack-fastapi-template | Python | 47 | 21 | 60 | 1 |
| django/django | Python | 2,888 | 868 | 17,496 | 0 |
| t3-oss/create-t3-app | TS/Next.js | 183 | 23 | 0 | 2 |
| payloadcms/payload | TS/Next.js | 641 | 46 | 424 | 0 |
| trpc/trpc | TypeScript | 578 | 231 | 1,144 | 0 |
| TanStack/router | TypeScript | 98 | 30 | 479 | 0 |
| refinedev/refine | React/TS | 330 | 73 | 407 | 0 |
| calcom/cal.com | TS/Next.js | 871 | 477 | 1,320 | 9 |
| **TOTAL** | | **5,434** | **1,822** | **27,012** | **18** |

**Key finding:** Zero crashes across 5,434 files. Core engine is solid. All 18 validation failures trace to the same root cause (validation.rule enum too narrow — fixed in v0.2.2).

### Kensa Team Review (2026-04-14)

External validation from the Kensa engineering team, who drove the full pipeline against a real spec graph with intentional dangling references and unmapped ACs. Key findings:

**What works:** C-NN / AC-NN cross-referencing, tier system with per-path overrides, status lifecycle, reverse with multi-language adapters (specifically: reverse-compiling from both Python and Go implementations to detect drift during a language migration), `resolve --dot` for design-review artifacts, strict schema rejection.

**What Kensa confirmed is architecturally correct:** "Seven improvements above are UX and CI-integration refinements, not architectural gaps."

**Kensa's 7 feedback items** are the direct source for the Phase 2 and Phase 3 additions above (dangling-ref suggestions, tier conflict warning, `specter explain`, `specter watch`, `specter diff`, `--strict`/`warn_on_draft`, pass-rate-aware coverage). All seven have been incorporated.

# Specter Improvement Roadmap

**Mission:** "A type system for specs." The core pipeline (parse → resolve → check → coverage → sync) is the mission. Everything else serves adoption.

**Design principle:** Make it easy for developers to use. If Specter does its job well, developers will love it.

**Date:** 2026-04-03
**Based on:** Real-world testing against 12 open-source repos (5,434 files, 0 crashes, 27,012 assertions extracted)

---

## The Mission Pipeline Today

```
parse → resolve → check → coverage → sync
  ✅       ✅        ✅       ✅        ✅
```

All 5 tools work. Dogfooding passes (6 specs, 85 tests). But they've only been validated against Specter's own specs. No external developer has gone through the full write-spec → run-sync → iterate loop. We don't actually know if the pipeline feels good to use.

---

## Three Improvement Tracks

### Track 1: Make the Core Pipeline Bulletproof

The pipeline works, but is it ready for a developer who just installed Specter and is writing their first spec?

| Gap | Why it matters |
|-----|---------------|
| No `specter init` command | Developer downloads Specter, types `specter`... now what? There's no scaffolding. They have to read docs and write YAML from scratch. |
| Error messages are technical | Parse errors show JSON Schema paths like `spec.constraints[0].id`. A developer needs "Constraint ID 'c01' is invalid — must match pattern C-01, C-02, etc." |
| No spec templates | Every spec starts from zero. A `specter init --template api-endpoint` would give them a starting point. |
| `specter sync` only runs locally | No GitHub Action. The developer has to wire CI themselves. A `hanalyx/specter-sync-action` would make CI integration one line. |
| Coverage thresholds are hardcoded | T1=100%, T2=80%, T3=50%. No way to configure per-project. A team might want T2=90%. |

### Track 2: Fix What's Broken (P0/P1)

These are bugs that undermine trust. A developer tries Specter, it rejects valid output or silently drops data, and they walk away.

| Bug | Impact | Fix effort |
|-----|--------|------------|
| `validation.rule` enum too narrow | 18 rejections across 4 repos — Specter rejects its own output | Small — map unknown rules to `"custom"` in core engine |
| Missing `.spec.tsx`/`.spec.jsx` in IsTestFile | 713 assertions silently lost in one repo | Trivial — add 2 strings |
| Test description truncation on embedded quotes | Garbled AC descriptions | Small — fix regex |
| Python false positive constraints from comments | `# isort MUST be provided` — nonsensical output | Small — skip comment lines |
| Spec ID collision on generic filenames | 54 specs all named `index` in one repo | Medium — incorporate parent dir |

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

### Phase 1 — Trust (v0.2.2)

Fix P0/P1 bugs. Specter must never reject its own valid output. This is table stakes.

- Map unknown `validation.rule` values to `"custom"` in core engine
- Add `.spec.tsx`, `.spec.jsx` to TypeScript adapter IsTestFile
- Fix test description regex to handle embedded quotes
- Fix Python adapter false positive constraints from comments
- Fix spec ID collision for generic filenames (route.ts, main.go, index.ts)

### Phase 2 — First-Run Experience (v0.3.0)

A developer should go from install to their first passing `specter sync` in under 5 minutes.

- `specter init` — scaffold a first spec with guided prompts
- Spec templates (api-endpoint, service, auth, data-model)
- Human-readable error messages (not JSON Schema paths)
- `specter doctor` — pre-flight check for project readiness

### Phase 3 — CI Integration (v0.3.x)

The mission becomes infrastructure — specs enforced on every PR, not just locally.

- `hanalyx/specter-sync-action` GitHub Action — one-line CI setup
- Configurable coverage thresholds per project (`.specter.yml`)
- PR comment integration — show spec coverage diff in PRs

### Phase 4 — Editor Experience (v0.4.0)

The "love" feature. Real-time validation as developers write specs.

- VS Code extension for `.spec.yaml` files
- Syntax highlighting, schema validation, autocomplete
- Inline diagnostics (orphan constraints, missing ACs)
- Go-to-definition for `depends_on` references

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

**Key finding:** Zero crashes across 5,434 files. Core engine is solid. All 18 validation failures trace to the same root cause (validation.rule enum too narrow).

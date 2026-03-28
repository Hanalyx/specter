# Specter: Build Decision

**Date:** 2026-03-28
**Decision:** APPROVED
**Confidence:** HIGH

---

## Synthesis of Agent Research

Four agents conducted independent research in parallel. Their findings converge.

### Agent 1: Market Research — MODERATE-TO-STRONG MOAT

The SDD tooling space has 6+ major players (GitHub Spec Kit, AWS Kiro, Tessl, OpenSpec, BMAD-METHOD, Intent). Every single one focuses on **spec-to-code generation** — helping AI write code from specs. None of them function as a **spec compiler** — a tool that treats specs as typed, interconnected artifacts subject to static analysis.

The competitive landscape map tells the story:

```
                    SPEC ANALYSIS DEPTH
                (structural -> semantic -> cross-spec)

    Shallow <------------------------------------> Deep

    | OpenSpec validate    |                    | [SPECTER]    |
    | Spectral             | Kiro validator     |              |
    | Spec Kit scoring     | Pact contracts     |              |
    |                      | Buf breaking       |              |
    |----------------------|--------------------|--------------+
    | Single-spec          | Code-vs-spec       | Spec-vs-spec |
    | structural lint      | drift detection    | type system  |
    |                      |                    |              |
    | CROWDED              | EMERGING           | EMPTY        |
```

**The "spec-vs-spec type system" quadrant is currently empty. That is the moat.**

The strongest architectural precedent is **Buf** (buf.build) — a compiler toolchain for protobuf that got significant funding doing essentially the same thing for a narrower domain.

**Risk:** GitHub (72.7k stars) and AWS Kiro could extend into this space. Window: 12-18 months.

### Agent 2: Technical Feasibility — CONDITIONALLY FEASIBLE

| Tool | Complexity | AI? | CI-Speed? | Effort |
|------|-----------|-----|-----------|--------|
| spec-parse | Low | No | Yes (ms) | 2-3 wk |
| spec-resolve | Medium | No | Yes (ms) | 2-3 wk |
| spec-check (structural) | Medium | No | Yes (sec) | 3-4 wk |
| spec-coverage | Medium | No | Yes (sec) | 2-3 wk |
| spec-sync | Low-Med | No | Yes (sec) | 1-2 wk |
| Reverse compiler | High | Partial | N/A | 6-9 wk |

**The deterministic core is 100% buildable with proven libraries** (Ajv, graphlib, ts-morph, semver). No novel engineering required. The AI-assisted layers (semantic conflict detection, gap analysis) are research-grade and deferred.

**Critical risk:** Schema design. Every tool depends on the canonical YAML schema. Get it wrong, everything needs rework.

**Total MVP effort:** 10-15 weeks for the deterministic pipeline. 20-30 weeks for the full toolchain.

### Agent 3: SDD Domain Expert — COMPLETE EXTRACTION

782 lines extracting every pattern from all 17 chapters:
- **12 sub-schemas** with complete field inventories (metadata, context, objective, constraints, testing, evolutionary, multi-agent, environment, migration, approval gate, confidence, reverse spec)
- **10 lifecycle phases** documented with rules
- **31 anti-patterns** the tool must detect (spec quality, drift, lifecycle)
- **4-level SDD maturity model** mapping toolchain role at each level
- **6 core + 8 supporting components** implied by the book
- **RFC 2119 vocabulary** (MUST/SHOULD/MAY) formalized as constraint language
- **Complete tier enforcement matrix** (Tier 1/2/3 with per-check severity)

Both JWTMS and OpenWatch assessed at Level 1 (Spec-Aware). The toolchain would move them to Level 2-3.

### Agent 4: Architecture/MVP — DESIGN COMPLETE

- **Product:** Specter — "A type system for specs"
- **Tech stack:** TypeScript, Node.js, pnpm, Commander.js, Vitest, Ajv, graphlib
- **MVP scope:** 4 tools (spec-parse, spec-resolve, spec-check, spec-coverage)
- **JSON Schema (draft 2020-12)** fully designed with constraint IDs (C-01), AC IDs (AC-01), `references_constraints` for orphan detection, tier enforcement, dependency references with semver ranges
- **Dogfooded specs:** spec-parse (6 constraints, 8 ACs) and spec-resolve (7+ ACs) written before implementation
- **6 milestones** over ~14 weeks
- **Core/CLI separation:** framework-agnostic core enables future IDE plugins

---

## Decision Matrix

| Criterion | Assessment | Weight | Score |
|-----------|-----------|--------|-------|
| Market moat | Moderate-to-strong. No existing spec compiler. | 25% | 8/10 |
| Technical feasibility | High for core, medium for reverse compiler | 25% | 8/10 |
| Alignment with SDD methodology | Perfect — dogfoods the entire book | 15% | 10/10 |
| MVP time-to-value | 10-15 weeks for deterministic core | 15% | 7/10 |
| Risk profile | Schema design is single point of failure | 10% | 6/10 |
| Competitive window | 12-18 months before GitHub/AWS could extend | 10% | 7/10 |
| **Weighted total** | | | **7.9/10** |

---

## Why APPROVED

1. **The moat is real.** No existing tool treats specs as typed artifacts in a dependency graph. The "spec compiler" concept is genuinely novel. The Buf precedent validates the business model.

2. **The core is buildable with known techniques.** spec-parse, spec-resolve, spec-check, and spec-coverage use proven libraries and textbook algorithms. No research-grade problems in the MVP.

3. **The sddbook IS the requirements spec.** 17 chapters of methodology, fully extracted into a 782-line domain document. This is the most thoroughly documented requirements base a tool project could have.

4. **Two real-world projects validate demand.** JWTMS (232 specs, 1,954 tests) and OpenWatch (200+ Python files, 229 TypeScript files) are both stuck at "Spec-Aware" despite strong engineering. The toolchain is exactly what they need to advance.

5. **Dogfooding is built in.** Specter's own specs are written in the format it validates. The tool proves itself by existing.

---

## Conditions and Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| Schema design is wrong | Validate schema against real specs from JWTMS/OpenWatch BEFORE building downstream tools. Iterate. |
| GitHub/AWS extends into this space | Ship MVP fast (14 weeks). Target regulated industries (they need this NOW for HIPAA/SOX/FedRAMP). |
| Adoption friction (no specs exist) | Reverse compiler bootstraps draft specs from existing code. Phase 6, not Phase 1. |
| False positives in semantic checks | Semantic checks are deferred to Phase 8. Core is 100% deterministic. No crying wolf. |
| Framework diversity in reverse compiler | Start TypeScript-only, Express/Fastify only. Flag others for manual review. |

---

## What Gets Built

### MVP (Milestones 1-4, ~14 weeks)

1. **M1:** Canonical JSON Schema + `spec-parse` CLI
2. **M2:** `spec-resolve` (dependency graph, cycle/dangling detection)
3. **M3:** `spec-check` (orphan constraints, version compat, duplicate IDs, structural conflicts)
4. **M4:** `spec-coverage` (traceability matrix: spec -> AC -> test)

### Post-MVP (Milestones 5-6)

5. **M5:** `spec-sync` (CI enforcement with tiered strictness)
6. **M6:** Reverse compiler (TypeScript structural extraction + AI gap-fill)

### Deferred (Phase 7-8)

7. Semantic conflict detection (AI-assisted)
8. Full gap analysis (AI-assisted)
9. Python target support
10. IDE plugins
11. Web dashboard

---

## Immediate Next Steps

1. Finalize the canonical `.spec.yaml` JSON Schema in `specter/src/core/schema/spec-schema.json`
2. Write Specter's own specs in `specter/specs/` (dogfooding)
3. Initialize the TypeScript project (`package.json`, `tsconfig.json`, `vitest.config.ts`)
4. Implement spec-parse (M1)
5. Write tests derived from the spec-parse spec's acceptance criteria

The SDD methodology says: specs first, then tests, then code. Specter follows its own rules.

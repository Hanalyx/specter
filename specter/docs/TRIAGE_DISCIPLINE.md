# Triage Discipline

How the Specter project decides which feature requests, schema changes, and design proposals get implemented vs. deferred vs. rejected.

## Two filters before scope

Before a feature request earns a release window, it passes two filters.

### Universality test

A feature must benefit *most* projects using Specter — or share the pain across multiple unrelated projects — to earn implementation. If a proposed feature only helps the project that asked for it, reject it.

- "JWTMS needs X" or "OpenWatch wants Y" alone is not sufficient. The pattern has to generalize.
- For pain that is real but only manifests in one project, prefer external tooling (a translator, an adapter, project-side scripts) or `specter migrate --from=<dialect>` over baking it into the core.
- For genuinely hard calls (clearly useful to one project, plausibly useful to others), surface explicitly rather than deciding alone. Either confirm a second project's use case or close until one surfaces.

This is a triage filter, not the only filter. Items that pass still have to clear mission focus and scope discipline.

### Schema conservatism

The spec schema is pre-1.0 (not locked), but schema changes are *not cheap*. Every delta forces downstream cost: docs rewritten, existing specs migrated, AI instruction templates updated, JSON consumers verified, dogfood specs touched. Pre-1.0 is permission to iterate, not encouragement to.

- Before recommending a schema change, look for non-schema alternatives: doctor canonicalization, `specter migrate --from=<dialect>` translators, tooling-side workarounds.
- "Match the existing X shape" or "fix the asymmetry" is *not* sufficient justification on its own. A documented user-friction case is required — and even then, ask whether the friction can be absorbed by tooling rather than the canonical schema.
- When a schema change is genuinely warranted, enumerate the full blast radius (parser, schema, type model, JSON output, doctor migration, editor surfaces, docs, dogfood specs, AI instruction templates) before proposing scope.
- Default framing for a schema-change idea: *"needs design discussion"* rather than *"vX.Y candidate"*. Don't pre-commit a release window before the design call happens.

## Specter Schema Request Brief (SSRB)

Each schema-change request — opened as a GitHub issue, raised internally, or surfaced during code review — gets a written brief documenting the decision and its reasoning. Briefs live at `docs/ssrb/SSRB-NNN.md`. The number matches the GitHub issue when one exists; otherwise sequential.

### When to write an SSRB

- Any field addition, removal, or shape change to the canonical spec schema
- Any change to the `acceptance_criteria` shape, `manifest` schema, or top-level metadata
- Cross-cutting design questions about the schema (e.g., "should ACs have a lifecycle?")
- Anything where future requesters will likely re-tread the same ground

### When NOT to write an SSRB

- Pure bug fixes that don't change schema
- Code refactors that don't surface to users
- Documentation improvements
- Tooling changes that don't alter the schema (CLI flags, output formats, etc.)

### Process

1. Copy `docs/ssrb/TEMPLATE.md` to `docs/ssrb/SSRB-NNN.md`.
2. Fill in §1 (Request) and §2 (Origin) immediately, before forming an opinion.
3. Apply the universality test (§3) and the cost analysis (§4).
4. Survey existing coverage (§5) and alternatives (§6).
5. Write §7 (Decision) only after the prior sections are complete. The reasoning must reference §3, §4, §5 explicitly.
6. Add §8 (Reconsideration triggers) — concrete criteria for revisiting.
7. Add a one-line entry to `docs/ssrb/INDEX.md`.
8. If the request originated as a GitHub issue, post a link to the SSRB on the issue thread when closing.

### Reusing prior reasoning

When a new request arrives that resembles a prior SSRB, link the new one to the existing brief. Either close the new request as "see SSRB-NNN" or write a fresh SSRB only if the new request differs in scope from the original. The catalog at `docs/ssrb/INDEX.md` is the entry point for "has this been asked before?"

### Why this exists

The v0.11.0 review cycle produced four schema-change requests (GH #97/#98/#99/#100), each requiring ~500 words of reasoning. Without a durable home, that reasoning would live only in scattered GitHub comments. SSRBs preserve the analysis, force consistent rigor on each new request, and give future requesters reference points so the project doesn't re-decide settled questions.

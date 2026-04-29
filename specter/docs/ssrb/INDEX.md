# Specter Schema Request Briefs (SSRB)

Each schema-change request gets a written brief documenting the decision and reasoning. See [`../TRIAGE_DISCIPLINE.md`](../TRIAGE_DISCIPLINE.md) for when an SSRB is required and how the process runs.

To start a new brief: copy [`TEMPLATE.md`](TEMPLATE.md) to `SSRB-NNN.md` (the number matches the GitHub issue when one exists, otherwise sequential).

## Catalog

| # | Title | Status | Decided |
|---|---|---|---|
| [097](SSRB-097.md) | `generated_from.source_files` plural array | REJECT | 2026-04-26 |
| [098](SSRB-098.md) | AC-level lifecycle `status` field | REJECT | 2026-04-26 |
| [099](SSRB-099.md) | Coverage inference from `generated_from.test_files` | REJECT | 2026-04-26 |
| [100](SSRB-100.md) | `spec.kind: audit-matrix` for cross-cutting specs | REJECT | 2026-04-26 |

## Status legend

- **ACCEPT** — change adopted; tracked into a release cycle
- **REJECT** — change declined; reasoning preserved here for future reference
- **DEFER (vN.M)** — accepted in principle, deferred to a future release
- **NEEDS-DESIGN** — requires a design call before scoping; held until the call happens

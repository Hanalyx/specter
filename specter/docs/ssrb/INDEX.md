# Specter Schema Request Briefs (SSRB)

Each schema-change request gets a written brief documenting the decision and reasoning. See [`../TRIAGE_DISCIPLINE.md`](../TRIAGE_DISCIPLINE.md) for when an SSRB is required and how the process runs.

To start a new brief: copy [`TEMPLATE.md`](TEMPLATE.md) to `SSRB-NNN.md` (the number matches the GitHub issue when one exists, otherwise sequential).

## Catalog

| # | Title | Status | Decided |
|---|---|---|---|
| [097](SSRB-097.md) | `generated_from.source_files` plural array | REJECT | 2026-04-26 |
| [098](SSRB-098.md) | AC-level lifecycle `status` field | NEEDS-DESIGN | 2026-04-26, reopened 2026-08-15 |
| [099](SSRB-099.md) | Coverage inference from `generated_from.test_files` | REJECT | 2026-04-26 |
| [100](SSRB-100.md) | `spec.kind: audit-matrix` for cross-cutting specs | REJECT | 2026-04-26 |
| [101](SSRB-101.md) | Source-file governance: annotation (F7) vs `governs:` list (F8) | REJECT | 2026-08-16 |
| [102](SSRB-102.md) | `settings.diagnostics`, per-rule severity in the manifest | NEEDS-DESIGN | TBD (with the SP-004 resolution) |
| [103](SSRB-103.md) | Multi-stream evidence in ingest and coverage | NEEDS-DESIGN | TBD |
| [104](SSRB-104.md) | Retire `settings.strictness` for `settings.annotation` | ACCEPT | 2026-08-19 |
| [105](SSRB-105.md) | Retire the manifest `registry` section | ACCEPT | 2026-08-22 |
| [106](SSRB-106.md) | Settle the three inert tier mechanisms | ACCEPT | 2026-08-22 |

## Status legend

- **ACCEPT**: change adopted; tracked into a release cycle
- **REJECT**: change declined; reasoning preserved here for future reference
- **DEFER (vN.M)**: accepted in principle, deferred to a future release
- **NEEDS-DESIGN**: requires a design call before scoping; held until the call happens

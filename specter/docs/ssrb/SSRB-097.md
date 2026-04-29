# SSRB-097: `generated_from.source_files` plural array

Status: REJECT
Decided: 2026-04-26
Source: GH #97

## 1. Request

Rename `generated_from.source_file` (singular string) to `generated_from.source_files` (plural array of strings) to match the existing `generated_from.test_files` shape. Backward-compat: accept `source_file` (singular) as shorthand for a one-element array.

## 2. Origin

JWTMS migration to Specter v1.0.0 schema (~250 specs). 46 of 249 specs legitimately listed >1 source file (max 19 — cross-cutting audit/flow specs). The workaround was to put the first path in `source_file` and the rest in `spec.tags` with a `src:` prefix, abusing `tags` for URI-shaped data and splitting one logical list across two fields.

## 3. Universality

Verdict: SINGLE-PROJECT

The singular/plural asymmetry between `source_file` and `test_files` has existed since v0.1. JWTMS is the first and only adopter to flag it. The reported friction is migration-specific (hand-edited spec content imported from a different schema dialect). No other Specter project has reported the same pain in the months since v0.1.

## 4. Cost of acceptance

- The canonical schema definition: rename + type change (string → array); add backward-compat alias.
- The in-memory type model: `SourceFile string` → `SourceFiles []string` (or both, with deprecation).
- The JSON contract: downstream consumers (VS Code extension, third-party tooling) verify the new shape.
- Reference documentation: rename across `docs/`.
- AI instruction templates: any schema fragments mention the new field.
- Existing user specs: `doctor --fix` canonicalization rule for the rename.
- Editor surfaces: minimal — no UI is keyed on this field today.
- Dogfooded specs: minimal — Specter's own specs are hand-authored, not reverse-compiled.

## 5. Existing coverage

The canonical schema does not currently express multi-file source provenance. The `tags` workaround is real; the proper answer is not to bend `tags`, but it doesn't follow that the schema must change.

## 6. Alternatives

- **Hand-edit `tags` workaround.** Status quo. Functional but opaque; tags isn't designed for URIs.
- **`specter migrate --from=jwtms` shape translator (GH #96).** When the migration tool lands, it absorbs JWTMS's flatter `spec.source_files` shape on import without relaxing the canonical schema. The asymmetry stays; the migration tool absorbs the cost.
- **`specter reverse` extension.** If `reverse` itself produces drafts where multi-source-file output is the natural shape, that's a different design call — but it hasn't surfaced.
- **Generalize `generated_from` to `provenance`.** Discussed in BACKLOG under "Unscheduled — design work needed first". Larger redesign; not the right scope for this request.

## 7. Decision

REJECT under §3 and §6. The singular/plural asymmetry is a real shape concern (§4 cost is non-trivial but tractable), but the only documented user-friction is JWTMS's, and the migration use case has a non-schema home in GH #96's `specter migrate --from=<dialect>` adapter. "Match the existing `test_files` shape" is a symmetry argument, not a user-friction argument; it doesn't meet the universality bar.

## 8. Reconsideration triggers

- A second unrelated project independently reports the same friction in non-migration use.
- `specter reverse` produces drafts where multi-source-file output is the natural shape and the singular form forces lossy emission.
- A future user surfaces a pain that GH #96's migration tool cannot absorb.

## 9. References

- GH issue: https://github.com/Hanalyx/specter/issues/97 (closed not-planned)
- Related SSRBs: SSRB-096 (when GH #96 lands)
- Related specs/code: `internal/parser/spec-schema.json` (GeneratedFrom shape), `BACKLOG.md` "Generalize `generated_from` to `provenance`"

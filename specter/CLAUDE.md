# CLAUDE.md — Specter

## What Is Specter

Specter is a **spec compiler toolchain** — "a type system for specs." It validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

**Core philosophy:** "Discipline can drift. Infrastructure cannot."

## SDD Rules — Specter Follows Its Own Methodology

1. **Specs first.** Every tool has a spec in `specs/` written BEFORE implementation. Read the spec before writing code.
2. **Tests derive from specs.** Every test traces to an AC in a spec. Use `// @spec <spec-id>` and `// @ac <AC-id>` annotations.
3. **Spec is SSOT.** When spec and code disagree, the spec is right. Change the spec first, then update code.
4. **No unspecced code.** New features require a spec. No exceptions.

## Project Structure

```
specter/
  specs/              # Specter's own specs (dogfooding)
  src/core/           # Framework-agnostic core (no CLI deps)
    schema/           # Canonical JSON Schema + validator
    parser/           # YAML -> SpecAST
    resolver/         # Dependency graph builder
    checker/rules/    # Individual check rules
    coverage/         # Traceability matrix
  src/cli/            # CLI layer (Commander.js)
  tests/              # Tests mirroring src/ structure
    fixtures/         # Valid and invalid spec files
  research/           # Agent research documents (read-only reference)
```

## Tech Stack

- TypeScript on Node.js 24+
- pnpm, tsup, Vitest, Commander.js
- Ajv (JSON Schema), @dagrejs/graphlib (dependency graph), semver, yaml (eemeli)

## Key Schema

The canonical spec schema lives at `src/core/schema/spec-schema.json`. This is the "type definition for the type system." All other tools depend on it.

Required fields: `id`, `version`, `status`, `tier`, `context`, `objective`, `constraints`, `acceptance_criteria`

## Conventions

- Constraint IDs: `C-01`, `C-02`, etc.
- AC IDs: `AC-01`, `AC-02`, etc.
- Spec IDs: kebab-case (`payment-create-intent`)
- Spec files: `{name}.spec.yaml`
- Core modules are pure functions — no I/O, no CLI deps
- Checker rules are individual files in `checker/rules/`

## Milestones

- **M1:** Schema + spec-parse (current)
- **M2:** spec-resolve (dependency graph)
- **M3:** spec-check (orphan + structural checks)
- **M4:** spec-coverage (traceability matrix)
- **M5:** spec-sync (CI enforcement)
- **M6:** Reverse compiler (code-to-spec)

## When Editing

- Read the relevant spec BEFORE writing or modifying any source file
- Maintain the core/CLI separation — core has zero CLI dependencies
- Run `npm run check` (typecheck + lint + test) before considering work done
- For SDD methodology reference, see `../sddbook/`

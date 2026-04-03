# Changelog

All notable changes to Specter will be documented in this file.

## [0.2.1] - 2026-04-03

### Fixed

- Fix null `validation.value` crash when Zod patterns have no extractable literal (`.email()`, `.url()`, `.optional()`, `.refine()`, `z.boolean()`, `z.array()`)
- Fix all Next.js App Router files generating the same spec ID `route` — now derives ID from route path (e.g., `/api/webhooks/stripe` → `webhooks-stripe`)
- Include source file path in `validation_failed` diagnostics for easier debugging
- Find `package.json`/`go.mod`/`pyproject.toml` in parent directories for system name inference

### Added

- TypeScript adapter: extract constraints from TypeScript enums, union types, and `as const` arrays
- TypeScript adapter: extract constraints from Prisma schema files (`.prisma`) — field types, `@unique`, `@db.VarChar(N)`, required/optional
- TypeScript adapter: extract role and status constraints from code patterns (e.g., `session.user.role === "ADMIN"`)
- TypeScript adapter: recognize `.url()`, `z.boolean()`, `z.array()` Zod patterns
- Zod enum now extracts values (e.g., `z.enum(["a", "b"])` → `Value: "a", "b"`)
- 85 tests (up from 77)

## [0.2.0] - 2026-04-03

### Added

- `specter reverse` command — reverse compiler extracts draft .spec.yaml from existing code
- Plugin adapter architecture with 3 built-in adapters: TypeScript, Python, Go
- TypeScript adapter: Zod schemas, Jest/Vitest tests, Next.js/Express routes
- Python adapter: Pydantic models, pytest tests, FastAPI/Django routes
- Go adapter: struct validate tags, table-driven tests, net/http/gin/chi routes
- Gap detection: flags constraints without test coverage
- Auto-detection of language adapter from file extensions
- spec-reverse.spec.yaml (10 constraints, 14 ACs)
- 77 total tests (up from 37)

## [0.1.0-alpha.2] - 2026-04-02

### Changed

- Migrated from TypeScript/Node.js to Go (single static binary, zero runtime dependencies)
- All 5 MVP tools rewritten: parse, resolve, check, coverage, sync
- Cross-platform binary distribution via goreleaser (linux, darwin, windows)
- DEB package available for Debian/Ubuntu

### Added

- GitHub Actions release workflow (tag-triggered)
- Community files: issue templates, PR template, CODEOWNERS
- CHANGELOG.md

## [0.1.0-alpha.1] - 2026-03-28

### Added

- Initial TypeScript implementation of Specter
- MVP tools: parse, resolve, check, coverage, sync
- Canonical spec schema (JSON Schema draft 2020-12)
- Dogfooding: Specter validates its own specs

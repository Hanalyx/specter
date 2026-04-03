# Changelog

All notable changes to Specter will be documented in this file.

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

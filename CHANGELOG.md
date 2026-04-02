# Changelog

All notable changes to Specter will be documented in this file.

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

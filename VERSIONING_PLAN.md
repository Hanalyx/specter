# Specter Versioning Plan

**Document Version**: v0.1.0
**Created**: 2026-04-16
**Status**: Active

---

## Overview

Specter uses **Semantic Versioning 2.0.0** (SemVer) with a single source of truth (`VERSION` file) and codenames for MINOR milestones. Specter ships two artifacts — the **CLI binary** and the **VS Code extension** — which share the same version number and are always released together.

---

## Version Format

```
MAJOR.MINOR.PATCH[-PRERELEASE]
```

| Component      | When to Increment                                                       |
|----------------|-------------------------------------------------------------------------|
| **MAJOR**      | Breaking changes to the spec schema, CLI flags, or output format        |
| **MINOR**      | New commands, new adapters, new pipeline stages (backward-compatible)   |
| **PATCH**      | Bug fixes, security patches, doc updates (no behavior change)           |
| **PRERELEASE** | `alpha.N`, `beta.N`, `rc.N` for testing phases before a release         |

**Examples**:
- `0.5.0` — new feature release
- `0.5.1` — bug fix on top of 0.5.0
- `0.6.0-beta.1` — beta for the next feature release
- `1.0.0` — first production-stable release

---

## Single Source of Truth

### `VERSION` File

The canonical version lives in `specter/VERSION`. All other references derive from it.

```
0.5.0
```

The file contains only the version string, no trailing newline.

### Version Propagation

| Artifact                          | How Updated                                      |
|-----------------------------------|--------------------------------------------------|
| `specter/VERSION`                 | **Manual edit** — this is the SSOT               |
| CLI binary (`main.version`)       | `make build` injects via `-ldflags "-X main.version=$(VERSION)"` |
| VS Code extension (`package.json`)| `make version-sync` copies from VERSION          |
| Git tag                           | Created from VERSION content during release      |
| GitHub Release                    | Created from the git tag                         |

### Reading the Version in the Makefile

```makefile
VERSION := $(shell cat VERSION)
```

### Keeping the VS Code Extension in Sync

```bash
make version-sync   # copies VERSION into vscode-extension/package.json
```

---

## Codenames

Codenames are assigned per MINOR version milestone and use a **supernatural / investigator** theme (fitting for a tool named Specter).

| Version | Codename     | Theme                           | Status          |
|---------|--------------|---------------------------------|-----------------|
| 0.1.x   | **Wisp**     | First light, early signal       | Released        |
| 0.2.x   | **Shade**    | Taking shape                    | Released        |
| 0.3.x   | **Wraith**   | Gaining form and power          | Released        |
| 0.4.x   | **Phantom**  | Mature, invisible enforcement   | Released        |
| 0.5.x   | **Specter**  | Named release — fully realized  | **Current**     |
| 0.6.x   | **Revenant** | Returns stronger after feedback | Planned         |
| 1.0.0   | **Sentinel** | Standing guard — production ready| Future         |

**Codename guidelines:**
- One word, easy to pronounce
- Supernatural / investigator theme
- Alphabetical progression preferred but not required
- Used in release announcements and changelogs, not in the binary itself

---

## Pre-release Phases

| Phase        | Purpose                        | Audience                  | Example          |
|--------------|--------------------------------|---------------------------|------------------|
| `alpha.N`    | Feature incomplete, unstable   | Core contributors only    | `0.6.0-alpha.1`  |
| `beta.N`     | Feature complete, bugs expected| Early adopters            | `0.6.0-beta.1`   |
| `rc.N`       | Production-ready candidate     | All willing testers       | `0.6.0-rc.1`     |

**Progression**:
```
0.5.0 (stable)
  ↓
0.6.0-alpha.1 → alpha.2
  ↓
0.6.0-beta.1 → beta.2
  ↓
0.6.0-rc.1
  ↓
0.6.0 (stable)
```

Pre-releases are published as **GitHub pre-releases** and as **VS Code extension pre-releases** (VSIX only, not marketplace).

---

## Version Lifecycle

### Development Phase (0.x.x)

Current phase. Indicates:
- Active feature development
- Spec schema may gain new optional fields between MINOR versions
- **No breaking changes within a MINOR** (patch releases are always safe to apply)
- Breaking changes are allowed between MINOR versions with a migration note in the changelog
- No long-term support commitment

### Production Phase (1.x.x+)

Triggered when:
- All six pipeline stages are stable and dogfooded
- VS Code extension has 100+ real-world users
- Spec schema is considered frozen (only additive changes in MINOR)

Production rules:
- **MAJOR**: Breaking schema or CLI changes (with migration guide)
- **MINOR**: New commands, new adapters (backward-compatible)
- **PATCH**: Bug fixes only — no behavior changes
- Security patches backported to the previous MINOR

---

## Breaking Change Policy

### What Constitutes a Breaking Change for Specter

- `.spec.yaml` schema field removal or rename
- CLI flag or subcommand removal or rename
- Changes to `--json` output structure (machine-readable output)
- Changes to exit codes
- Spec ID or AC ID format changes
- `specter.yaml` manifest key removal or rename

### What is NOT a Breaking Change

- Adding new optional fields to the spec schema
- Adding new subcommands
- Adding new flags to existing subcommands
- Output formatting changes (non-JSON output)
- Improvements to error messages

### Deprecation Process

1. **Announce**: Mark deprecated in release notes and `specter doctor` output
2. **Warn**: Print a deprecation warning when the deprecated feature is used
3. **Duration**: Minimum 2 MINOR versions before removal
4. **Remove**: Only in the next MAJOR version

```
v0.5.0 — Field X deprecated (specter doctor warns)
v0.6.0 — Field X still works (warning continues)
v1.0.0 — Field X removed
```

---

## Release Process

### Standard Release (MINOR or PATCH)

```bash
# 1. Update VERSION
echo "0.5.0" > specter/VERSION

# 2. Sync VS Code extension version
cd specter && make version-sync

# 3. Update CHANGELOG.md

# 4. Run full pre-release gate
make prerelease

# 5. Commit and tag
git add specter/VERSION specter/vscode-extension/package.json CHANGELOG.md
git commit -m "chore: release v0.5.0"
git tag -a "v0.5.0" -m "Release v0.5.0 \"Specter\""
git push origin main --tags

# 6. GitHub Actions publishes the release automatically on tag push
```

### Hotfix Process (PATCH)

```bash
# Branch from the release tag
git checkout -b hotfix/0.5.1 v0.5.0

# Apply the fix, then bump patch
echo "0.5.1" > specter/VERSION
make version-sync

git commit -am "fix: <description>"
git tag -a "v0.5.1" -m "Hotfix v0.5.1"

# Merge back to main
git checkout main
git merge hotfix/0.5.1
git push origin main --tags
```

### Commit Message Conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

| Type       | Version bump | Example                                        |
|------------|-------------|------------------------------------------------|
| `feat`     | MINOR       | `feat: add specter lint command`               |
| `fix`      | PATCH       | `fix: resolve false positive in coverage`      |
| `feat!`    | MAJOR       | `feat!: rename @ac to @criteria`               |
| `docs`     | none        | `docs: update watch command examples`          |
| `chore`    | none        | `chore: update dependencies`                   |
| `ci`       | none        | `ci: add arm64 build target`                   |
| `refactor` | none        | `refactor: extract adapter interface`          |

---

## Multi-Artifact Release Alignment

Specter ships two artifacts that must always be released at the **same version**:

| Artifact              | Registry              | Version source               |
|-----------------------|-----------------------|------------------------------|
| CLI binary            | GitHub Releases       | `specter/VERSION`            |
| VS Code extension     | VS Code Marketplace   | `vscode-extension/package.json` (synced from VERSION) |

**Rule:** A GitHub Release and a VS Code extension publish must happen together in the same release. Never release one without the other.

---

## Planned Releases

| Version | Codename    | Target  | Key Focus                                                          |
|---------|-------------|---------|--------------------------------------------------------------------|
| 0.5.0   | **Specter** | 2026-Q2 | VS Code extension GA, DX tooling, quality gates, bug reporting     |
| 0.6.0   | **Revenant**| 2026-Q3 | Language server protocol (LSP) support, spec linting               |
| 1.0.0   | **Sentinel**| TBD     | Stable schema, production SLA, VS Code Marketplace featured listing |

---

## Compatibility Matrix

| Specter | Go    | VS Code  | Spec Schema |
|---------|-------|----------|-------------|
| 0.4.x   | 1.22+ | 1.85+    | v1          |
| 0.5.x   | 1.22+ | 1.85+    | v1          |
| 1.0.x   | TBD   | TBD      | v2 (TBD)    |

---

## Version History

| Version      | Codename  | Date       | Notes                                                           |
|--------------|-----------|------------|-----------------------------------------------------------------|
| 0.1.0-alpha.2| Wisp      | 2026-01    | Early alpha                                                     |
| 0.2.0–0.2.4  | Shade     | 2026-01-02 | Core pipeline, multiple patch fixes                             |
| 0.3.0–0.3.1  | Wraith    | 2026-02-03 | Reverse compiler Phase 1                                        |
| 0.4.0–0.4.1  | Phantom   | 2026-03-04 | Phase 2 (doctor, explain, watch), Kensa UX improvements         |
| 0.5.0        | Specter   | 2026-04+   | VS Code extension GA, DX tooling, bug reporting *(in progress)* |

---

## References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [VS Code Extension Versioning](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#prerelease-extensions)

---

## Document History

| Version | Date       | Author | Changes                         |
|---------|------------|--------|---------------------------------|
| v0.1.0  | 2026-04-16 | Claude | Initial versioning plan         |

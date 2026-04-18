> **⚠ Archived — stale content.** This document was written pre-v0.3.0 and was not maintained past that version. Current release status lives in [CHANGELOG.md](../../CHANGELOG.md); forward-looking roadmap lives in [BACKLOG.md](../../BACKLOG.md). Kept here for historical reference only.

---

# Specter: Open Source Release Plan

## 1. Pre-Release Checklist

### Code Readiness

- [x] MVP complete (M1-M5): parse, resolve, check, coverage, sync
- [x] 41 tests, 33 ACs, 100% spec coverage
- [x] All checks pass: typecheck, lint, format, test, build
- [x] Specter validates its own specs (dogfooding proof)
- [x] Go 1.22+ target, go vet, go test
- [ ] Security audit: `go vet ./...` and `govulncheck ./...` pass with 0 vulnerabilities
- [x] License file (MIT) at repo root
- [x] CONTRIBUTING.md with contribution guidelines
- [ ] CODE_OF_CONDUCT.md
- [x] Issue templates (bug report, feature request)
- [x] PR template

### Documentation Readiness

- [x] README.md with quick start, examples, and CLI overview
- [x] docs/GETTING_STARTED.md
- [x] docs/SPEC_SCHEMA_REFERENCE.md
- [x] docs/CLI_REFERENCE.md
- [x] docs/FAQ.md
- [x] docs/MVP_VALUE_PROPOSITION.md
- [x] CHANGELOG.md
- [ ] docs/CONTRIBUTING_SPECS.md (how to write specs for contributions)

### Repository Setup

- [ ] GitHub repo description and topics (`sdd`, `spec-driven-development`, `yaml`, `validation`, `cli`, `golang`)
- [ ] GitHub Actions CI workflow verified on GitHub (not just local)
- [ ] Branch protection on `main` (require CI pass, require review)
- [x] `.github/CODEOWNERS` file
- [x] goreleaser configuration for cross-platform binary builds

---

## 2. Versioning Strategy

### Semantic Versioning (SemVer)

Specter follows strict semver:

- **MAJOR** (1.0.0 -> 2.0.0): Breaking changes to the spec schema, CLI interface, or core API
- **MINOR** (1.0.0 -> 1.1.0): New features, new check rules, new CLI commands
- **PATCH** (1.0.0 -> 1.0.1): Bug fixes, documentation updates, performance improvements

### Version Milestones

| Version | What It Represents |
|---------|-------------------|
| `0.1.0` | Current MVP (M1-M5). Pre-release. Schema may change. |
| `0.2.0` | M6 (reverse compiler). Schema stable candidate. |
| `0.3.0` | AI-assisted checks (semantic conflicts, gap detection). |
| `1.0.0` | Stable release. Schema frozen. Public API guaranteed. |
| `1.x.x` | Post-1.0 features without breaking changes. |

### What Constitutes a Breaking Change

**Schema changes (affects all users):**
- Removing a required field from `spec-schema.json`
- Changing field types or validation patterns
- Changing the meaning of existing fields
- Removing an enum value from `status`, `tier`, etc.

**CLI changes (affects CI integrations):**
- Removing a command or option
- Changing exit code semantics
- Changing output format of `--json` mode

**NOT breaking:**
- Adding new optional fields to the schema
- Adding new check rules (new warnings/errors)
- Adding new CLI commands
- Adding new output formats

---

## 3. Auto-Versioning with Conventional Commits

### Commit Convention

All commits follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**

| Type | Version Bump | Example |
|------|-------------|---------|
| `feat` | MINOR | `feat(check): add duplicate constraint ID detection` |
| `fix` | PATCH | `fix(parse): handle YAML tabs correctly` |
| `feat!` or `BREAKING CHANGE:` | MAJOR | `feat!(schema): remove deprecated trust_level field` |
| `docs` | none | `docs: update CLI reference for resolve --dot` |
| `chore` | none | `chore: update dependencies` |
| `test` | none | `test(coverage): add edge case for empty test files` |
| `refactor` | none | `refactor(resolver): simplify cycle detection` |
| `ci` | none | `ci: add Go 1.22 to test matrix` |
| `perf` | PATCH | `perf(parse): cache compiled JSON Schema validator` |

### Scopes

| Scope | What It Covers |
|-------|---------------|
| `schema` | Changes to `spec-schema.json` or schema types |
| `parse` | spec-parse (M1) |
| `resolve` | spec-resolve (M2) |
| `check` | spec-check (M3) |
| `coverage` | spec-coverage (M4) |
| `sync` | spec-sync (M5) |
| `reverse` | Reverse compiler (M6) |
| `cli` | CLI commands and output formatting |
| `docs` | Documentation |
| `deps` | Dependency updates |

### Tooling: release-please (deferred)

> **Note:** For alpha releases, Specter uses manual tagging with goreleaser. release-please will be adopted post-alpha when the release cadence justifies automation.

[release-please](https://github.com/googleapis/release-please) (Google's release automation) is planned for future auto-versioning:

1. Parses conventional commit messages since last release
2. Determines version bump (major/minor/patch)
3. Generates CHANGELOG.md entries
4. Creates a release PR with version bump
5. On merge: creates GitHub Release + git tag
6. Triggers goreleaser to build and publish binaries

**Why release-please over alternatives:**
- `semantic-release`: More opinionated, harder to configure for monorepo
- `changesets`: Requires manual changeset files per PR
- `release-please`: Fully automated from commit messages, supports monorepo, maintained by Google

### GitHub Actions: Release Workflow

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-latest
    outputs:
      release_created: ${{ steps.release.outputs.release_created }}
      tag_name: ${{ steps.release.outputs.tag_name }}
    steps:
      - uses: googleapis/release-please-action@v4
        id: release
        with:
          release-type: go
          path: specter
          package-name: specter

  goreleaser:
    needs: release-please
    if: ${{ needs.release-please.outputs.release_created }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Configuration: release-please-config.json

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    "specter": {
      "release-type": "go",
      "package-name": "specter",
      "changelog-path": "CHANGELOG.md",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "draft": false,
      "prerelease": false
    }
  }
}
```

**`bump-minor-pre-major: true`**: While version is 0.x.x, `feat` commits bump patch instead of minor (avoids rapid 0.x version inflation during pre-1.0 development).

---

## 4. Binary Distribution Strategy

### Distribution Channels

Specter is distributed as a single static binary with zero runtime dependencies.

**Homebrew (macOS and Linux):**
```bash
brew install hanalyx/tap/specter
```

**DEB package (Debian/Ubuntu):**
```bash
# Download from GitHub Releases, then:
sudo dpkg -i specter_*.deb
```

**GitHub Releases (all platforms):**

Download pre-built binaries for Linux, macOS (Intel and Apple Silicon), and Windows from the [GitHub Releases](https://github.com/Hanalyx/specter/releases) page. goreleaser produces archives for each platform on every tagged release.

> **Note:** `go install github.com/Hanalyx/specter/cmd/specter@latest` does not currently work because `go.mod` lives in the `specter/` subdirectory rather than the repo root. Use binary downloads or DEB packages instead. This will be addressed in a future repo restructure.

### Build Configuration

The `go.mod` file at the repository root defines the module path and dependencies. The `Makefile` provides build targets:

```bash
make build       # Build for current platform -> bin/specter
make build-all   # Cross-compile for linux/darwin/windows
make clean       # Remove built binaries
```

### Install Experience (Goal)

```bash
# Homebrew
brew install hanalyx/tap/specter
specter sync

# Go install
go install github.com/Hanalyx/specter/cmd/specter@latest
specter sync

# Direct download (example for Linux amd64)
curl -Lo specter.tar.gz https://github.com/Hanalyx/specter/releases/latest/download/specter_Linux_x86_64.tar.gz
tar xzf specter.tar.gz
./specter sync
```

---

## 5. Release Phases

### Phase 1: Soft Launch (v0.1.0)

**Goal:** Get early feedback from SDD practitioners.

- [ ] Push to GitHub as public repo
- [ ] Write announcement post (dev.to, Reddit r/programming, HN)
- [ ] Share with SDD community (GitHub Spec Kit, Kiro, OpenSpec users)
- [ ] Publish binaries via goreleaser as `v0.1.0-beta.1`
- [ ] Collect feedback via GitHub Issues

**Success metric:** 10+ GitHub stars, 3+ external issues filed.

### Phase 2: Schema Stabilization (v0.2.0)

**Goal:** Lock down the spec schema based on real-world usage.

- [ ] M6 (reverse compiler) ships -- solves cold-start problem
- [ ] Schema changes from user feedback incorporated
- [ ] Migration guide for any schema changes
- [ ] Publish `0.2.0`

**Success metric:** 3+ external projects using Specter specs.

### Phase 3: Stable Release (v1.0.0)

**Goal:** Spec schema frozen. Public API guaranteed.

- [ ] Schema declared stable (no breaking changes without major bump)
- [ ] CLI interface declared stable
- [ ] Full documentation review
- [ ] Publish `1.0.0`

**Success metric:** GitHub release downloads > 100/week. GitHub stars > 500.

---

## 6. Community Infrastructure

### GitHub Templates

**Bug Report:**
```markdown
**Specter version:** (output of `specter --version`)
**Go version:** (output of `go version`)
**OS:**

**Steps to reproduce:**
1.
2.

**Expected behavior:**

**Actual behavior:**

**Spec file (if relevant):**
```yaml
```
```

**Feature Request:**
```markdown
**Is this related to a problem?**

**Proposed solution:**

**Spec (if you've written one):**
```yaml
```

**Alternatives considered:**
```

**Spec Proposal (for new check rules, schema fields, etc.):**
```markdown
**What should Specter detect/validate?**

**Example of a spec that should PASS:**
```yaml
```

**Example of a spec that should FAIL:**
```yaml
```

**Which tier(s) should this affect?**

**Proposed severity:** error / warning / info
```

### Labels

| Label | Color | Usage |
|-------|-------|-------|
| `schema` | red | Changes to spec-schema.json |
| `check-rule` | blue | New or modified check rules |
| `cli` | green | CLI interface changes |
| `docs` | grey | Documentation |
| `good-first-issue` | purple | Easy entry point for contributors |
| `breaking` | red | Would require major version bump |
| `reverse-compiler` | orange | M6 features |

---

## 7. Immediate Next Steps

1. **Create LICENSE file** (MIT)
2. **Create CONTRIBUTING.md** (fork, branch, spec-first, PR)
3. **Set up release-please** (GitHub Action + config)
4. **Configure goreleaser** (`.goreleaser.yaml` with cross-compile targets)
5. **Push to Hanalyx/specter and verify CI**
6. **Tag v0.1.0**

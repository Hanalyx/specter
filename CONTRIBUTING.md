# Contributing to Specter

Specter follows Spec-Driven Development. Contributions follow the same methodology.

## The Rule

**Specs first, tests second, code third.** No exceptions.

## How to Contribute

### 1. Fork and Branch

```bash
git clone https://github.com/YOUR_USERNAME/specter.git
cd specter/specter
go build ./cmd/specter/
git checkout -b feat/your-feature
```

Target the **current working branch**, not `main`. See [Branch workflow](#branch-workflow) below.

### 2. Write (or Update) the Spec

Every change needs a spec. If you're adding a new check rule, write `specs/your-rule.spec.yaml` first. If you're modifying existing behavior, update the relevant spec in `specs/`.

Use the canonical schema. Validate your spec:

```bash
./specter parse specs/your-rule.spec.yaml
```

### 3. Write Tests from ACs

Every acceptance criterion in your spec becomes a test. Annotate tests with `@spec` and `@ac`:

```go
// @spec your-rule
package yourrule

// @ac AC-01
func TestDoesTheThing(t *testing.T) {
    // ...
}
```

### 4. Implement

Now write the code. The spec tells you what to build. The tests tell you when you're done.

### 5. Verify

```bash
make check   # go vet + go test + go build
make dogfood # specter validates its own specs
```

All must pass.

### 6. Commit with Conventional Commits

```
feat(check): add duplicate constraint ID detection
fix(parse): handle YAML tabs correctly
docs: update CLI reference
```

See [RELEASE_PLAN.md](specter/docs/RELEASE_PLAN.md) for the full commit convention.

### 7. Open a PR

- Title: conventional commit format
- Body: reference the spec and which ACs are covered
- **Base branch: the current working branch** (see below), not `main`, unless this is a hotfix for a bug in the shipped release
- All CI checks must pass

```bash
# Target the current working branch explicitly — do not rely on the repo default.
gh pr create --base release/v0.10 --title "..."
```

## Branch workflow

Specter uses a **release-working-branch** model. Each release cycle has one long-lived branch, typically `release/vX.Y.Z`, that accumulates every feature/fix/doc change until ship time. `main` receives one merge per release.

```
main
 │
 ├── release/v0.10  ← every PR targets this while v0.10 is in flight
 │    ├── feat/specter-migrate
 │    ├── fix/some-bug
 │    └── docs/something
 │
 └── hotfix/v0.9.3  ← hotfixes branch from main, merge to main, skip the working branch
```

### Rules

1. **All feature, fix, and doc PRs target the current working branch.** Not `main`. The current branch name is in [BACKLOG.md](specter/BACKLOG.md)'s header.
2. **The working branch is named per release** — `release/v0.10`, `release/v0.11`, etc. Created at cycle start; deleted after it merges to `main` and the release is tagged.
3. **Hotfixes are the exception.** A bug in the shipped release on `main` gets fixed via `hotfix/v0.9.3` (or similar) branched off `main`, PR'd to `main`, merged, and tagged. The in-flight working branch later merges `main` forward to absorb the hotfix.
4. **Tags happen on `main`** after the working branch merges, never on the working branch itself. A tag = "this commit shipped to users."
5. **`--base` is explicit** when opening a PR. GitHub's default-base-branch setting stays on `main`; contributors pass `--base release/vX.Y.Z` (or the current working branch name). An incorrectly-targeted PR is the one failure mode this discipline can't catch on its own.

### Why

The v0.9.0 / v0.9.1 / v0.9.2 releases each landed on `main` through ~5 separate PRs apiece. That works, but `git log main` becomes a development log, not a release history — every WIP commit and test iteration is mixed in with the ships. Routing WIP through a working branch keeps `main` a clean trunk of releases, makes "what's in the next release?" answerable from branch state, and reduces force-push risk on shared history.

### When no working branch exists

Between releases — after `main` catches the ship merge and before the next cycle's working branch is created — PRs that can't wait (hotfixes, urgent doc fixes) target `main` directly. This is the only time main accepts a non-working-branch PR.

## What Makes a Good Contribution

- **New check rules** -- detect new classes of spec errors (see `internal/checker/`)
- **Schema improvements** -- new optional fields that improve spec expressiveness
- **Bug fixes** -- with a test that reproduces the bug
- **Documentation** -- especially real-world examples of specs

## What to Avoid

- Code without a spec
- Tests without `@spec`/`@ac` annotations
- Changes to `spec-schema.json` without a discussion/issue first
- Breaking changes without a migration plan

## Architecture Notes

- `internal/` packages are **pure function libraries** -- no I/O, no CLI, no side effects
- `cmd/specter/` is the **thin CLI wrapper** (Cobra) -- reads files, calls internal packages, formats output
- Checker rules live in `internal/checker/` -- add new rules without touching the orchestrator
- Tests are colocated with source (`internal/parser/parse_test.go` next to `parse.go`)
- Test fixtures live in `testdata/`

## Questions?

Open a [GitHub Issue](https://github.com/Hanalyx/specter/issues) or start a [Discussion](https://github.com/Hanalyx/specter/discussions).

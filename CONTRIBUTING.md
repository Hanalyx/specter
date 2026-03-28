# Contributing to Specter

Specter follows Spec-Driven Development. Contributions follow the same methodology.

## The Rule

**Specs first, tests second, code third.** No exceptions.

## How to Contribute

### 1. Fork and Branch

```bash
git clone https://github.com/YOUR_USERNAME/specter.git
cd specter/specter
npm install
git checkout -b feat/your-feature
```

### 2. Write (or Update) the Spec

Every change needs a spec. If you're adding a new check rule, write `specs/your-rule.spec.yaml` first. If you're modifying existing behavior, update the relevant spec in `specs/`.

Use the canonical schema. Validate your spec:

```bash
npm run build && node dist/index.js parse specs/your-rule.spec.yaml
```

### 3. Write Tests from ACs

Every acceptance criterion in your spec becomes a test. Annotate tests:

```typescript
/**
 * @spec your-rule
 */
describe('your-rule', () => {
  // @ac AC-01
  it('AC-01: does the thing', () => {
    // ...
  });
});
```

### 4. Implement

Now write the code. The spec tells you what to build. The tests tell you when you're done.

### 5. Verify

```bash
npm run check          # typecheck + lint + test
npm run format:check   # formatting
npm run build          # builds
node dist/index.js sync  # Specter validates itself
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
- All CI checks must pass

## What Makes a Good Contribution

- **New check rules** -- detect new classes of spec errors (see `src/core/checker/rules/`)
- **Schema improvements** -- new optional fields that improve spec expressiveness
- **Bug fixes** -- with a test that reproduces the bug
- **Documentation** -- especially real-world examples of specs

## What to Avoid

- Code without a spec
- Tests without `@spec`/`@ac` annotations
- Changes to `spec-schema.json` without a discussion/issue first
- Breaking changes without a migration plan

## Architecture Notes

- `src/core/` is a **pure function library** -- no I/O, no CLI, no side effects
- `src/cli/` is the **thin CLI wrapper** -- reads files, calls core, formats output
- Checker rules are **individual files** in `src/core/checker/rules/` -- add new rules without touching the orchestrator
- Tests live in `tests/unit/` mirroring the `src/` structure

## Questions?

Open a [GitHub Issue](https://github.com/Hanalyx/specter/issues) or start a [Discussion](https://github.com/Hanalyx/specter/discussions).

# Specter Spec Registry

> Master index of all specs in the Specter project.
> Maintained per MODULE_03 CH03 Registry Pattern.

## Core Toolchain Specs

| ID | Version | Tier | Status | Path | Dependencies |
|----|---------|------|--------|------|-------------|
| spec-parse | 1.0.0 | 1 | approved | specs/spec-parse.spec.yaml | none |
| spec-resolve | 1.0.0 | 1 | approved | specs/spec-resolve.spec.yaml | spec-parse |
| spec-check | 1.0.0 | 1 | approved | specs/spec-check.spec.yaml | spec-parse, spec-resolve |
| spec-coverage | 1.0.0 | 2 | approved | specs/spec-coverage.spec.yaml | spec-parse |

## Dependency Graph

```
spec-parse (foundation)
  |
  +-- spec-resolve (depends on spec-parse)
  |     |
  |     +-- spec-check (depends on spec-parse, spec-resolve)
  |
  +-- spec-coverage (depends on spec-parse)
```

## Tier Summary

- **Tier 1 (3 specs):** spec-parse, spec-resolve, spec-check
- **Tier 2 (1 spec):** spec-coverage

## Coverage Targets

| Tier | Target | Current |
|------|--------|---------|
| 1 | 100% | 0% (pre-implementation) |
| 2 | 80% | 0% (pre-implementation) |

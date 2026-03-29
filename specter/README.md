# Specter

**A type system for specs.** Validates, links, and type-checks `.spec.yaml` files.

## Install

```bash
# From source
go build -o bin/specter ./cmd/specter/

# Or use Make
make build
```

Produces a single binary with zero runtime dependencies.

## Usage

```bash
# Validate specs against the canonical schema
specter parse specs/*.spec.yaml

# Build and validate the dependency graph
specter resolve

# Run structural checks (orphan constraints, conflicts, breaking changes)
specter check

# Generate spec-to-test traceability matrix
specter coverage

# Run the full pipeline (parse + resolve + check + coverage)
specter sync
```

## Write a Spec

```yaml
spec:
  id: user-registration
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: Auth service

  objective:
    summary: Register a new user with email and password.

  constraints:
    - id: C-01
      description: "email MUST be a valid RFC 5322 address"
    - id: C-02
      description: "password MUST be at least 8 characters"

  acceptance_criteria:
    - id: AC-01
      description: "Returns 201 with JWT on success"
      references_constraints: ["C-01", "C-02"]
    - id: AC-02
      description: "Returns 400 when email is invalid"
      references_constraints: ["C-01"]
```

## Project Structure

```
specter/
  cmd/specter/       CLI entry point (Cobra)
  internal/
    parser/          M1: YAML -> validated SpecAST
    resolver/        M2: Dependency graph, cycle detection
    checker/         M3: Orphan constraints, structural conflicts
    coverage/        M4: Spec-to-test traceability matrix
    sync/            M5: CI pipeline orchestrator
    schema/          Canonical types + JSON Schema
  specs/             Specter's own specs (dogfooding)
  testdata/          Test fixtures
  docs/              User documentation
```

## Development

```bash
make check      # go vet + go test + go build
make dogfood    # run specter against its own specs
make build-all  # cross-compile for linux/darwin/windows
```

## Documentation

- [Getting Started](docs/GETTING_STARTED.md)
- [Spec Schema Reference](docs/SPEC_SCHEMA_REFERENCE.md)
- [CLI Reference](docs/CLI_REFERENCE.md)
- [FAQ](docs/FAQ.md)

## Dogfooding

Specter validates its own 5 specs, resolves its own dependency graph, and checks for orphan constraints. The tool proves itself by existing.

```
$ specter sync

Specter Sync

  PASS parse: 5 spec(s) parsed successfully
  PASS resolve: 5 specs, 8 dependencies resolved
  PASS check: 0 warning(s), 0 info
  PASS coverage: 5 spec(s) meet coverage thresholds

All checks passed.
```

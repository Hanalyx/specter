# Specter

**A type system for specs.** Validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

```
$ specter sync

Specter Sync

  PASS parse: 5 spec(s) parsed successfully
  PASS resolve: 5 specs, 8 dependencies resolved
  PASS check: 0 warning(s), 0 info
  PASS coverage: 5 spec(s) meet coverage thresholds

All checks passed.
```

## The Problem

AI coding tools generate code from specifications. But nobody validates the specifications themselves. Specs are untyped YAML documents -- they can contradict each other, have orphaned constraints with no test coverage, reference dependencies that don't exist, and silently rot as code evolves.

**Specter treats specs as typed artifacts in a dependency graph, subject to static analysis.** It catches spec errors before code is ever generated.

## What Specter Does

| Command | What It Catches |
|---------|-----------------|
| `specter parse` | Malformed specs, missing fields, invalid IDs, wrong types -- like a compiler catching syntax errors |
| `specter resolve` | Circular dependencies, dangling references, version mismatches -- like a linker catching undefined symbols |
| `specter check` | Orphan constraints (no AC references them), structural conflicts between specs -- like a type checker catching errors |
| `specter coverage` | Specs without tests, ACs without coverage, below-threshold modules -- like code coverage but for specifications |
| `specter sync` | Runs the full pipeline in CI. Blocks the merge if specs are broken. |

## Quick Start

### Install

**Download binary** from [GitHub Releases](https://github.com/Hanalyx/spec-dd/releases):

```bash
# Linux (amd64)
curl -Lo specter.tar.gz https://github.com/Hanalyx/spec-dd/releases/latest/download/specter_Linux_x86_64.tar.gz
tar xzf specter.tar.gz
sudo mv specter /usr/local/bin/

# Verify
specter --version
```

**DEB package** (Debian/Ubuntu):

```bash
curl -Lo specter.deb https://github.com/Hanalyx/spec-dd/releases/latest/download/specter_amd64.deb
sudo dpkg -i specter.deb
```

**Build from source:**

```bash
git clone https://github.com/Hanalyx/spec-dd.git
cd spec-dd/specter
make build
bin/specter --version
```

### Use

```bash
# Validate your specs
specter parse specs/*.spec.yaml

# Run the full pipeline
specter sync
```

See the [Getting Started guide](specter/docs/GETTING_STARTED.md) for a complete walkthrough.

## Write Your First Spec

```yaml
spec:
  id: user-registration
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: Auth service
    description: Handles new user account creation

  objective:
    summary: >
      Register a new user with email and password.
      Return a JWT token on success.

  constraints:
    - id: C-01
      description: "email MUST be a valid RFC 5322 address"
      type: technical
      enforcement: error
    - id: C-02
      description: "password MUST be at least 8 characters"
      type: security
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Returns 201 with JWT when registration succeeds"
      references_constraints: ["C-01", "C-02"]
    - id: AC-02
      description: "Returns 400 when email is invalid"
      references_constraints: ["C-01"]
    - id: AC-03
      description: "Returns 400 when password is too short"
      references_constraints: ["C-02"]
    - id: AC-04
      description: "Returns 409 when email already registered"
      references_constraints: ["C-01"]
```

Then validate it:

```bash
specter parse user-registration.spec.yaml
# PASS user-registration.spec.yaml -- user-registration@1.0.0
```

## How It Works

Specter implements the **Spec-Driven Development (SDD)** methodology as infrastructure:

```
.spec.yaml files
      |
  [spec-parse]     Validate YAML against canonical JSON Schema
      |
  [spec-resolve]   Build dependency graph, detect cycles and broken refs
      |
  [spec-check]     Find orphan constraints, structural conflicts, breaking changes
      |
  [spec-coverage]  Map specs to tests, enforce tier-based coverage thresholds
      |
  [spec-sync]      Gate CI on all of the above
```

The core insight: **specs should work like a type system.** Constraints are type definitions. ACs are function signatures. `depends_on` is an import statement. An orphaned constraint is an unused variable. A missing AC is a missing null check.

## The Spec Type System Analogy

| Programming Concept | Spec Equivalent |
|---|---|
| Type definition | Constraint -- defines what's allowed |
| Function signature | AC -- defines input to output |
| Import statement | `depends_on` -- creates a contract between specs |
| Type error | Spec conflict -- caught before tests run |
| Unused variable | Orphan constraint -- no AC references it |
| Missing null check | Spec gap -- a path with no AC coverage |

## Tier-Based Enforcement

Not all specs need the same rigor:

| Tier | Risk Level | Examples | Coverage Target |
|------|-----------|---------|-----------------|
| **1** | Security / Money | Auth, payments, encryption | 100% |
| **2** | Business Logic | Booking flow, pricing | 80% |
| **3** | Utility | Helpers, formatters | 50% |

Specter adjusts enforcement severity per tier. A Tier 1 orphan constraint is an error. A Tier 3 orphan is informational.

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](specter/docs/GETTING_STARTED.md) | Write and validate your first spec in 5 minutes |
| [Spec Schema Reference](specter/docs/SPEC_SCHEMA_REFERENCE.md) | Every field in the `.spec.yaml` format |
| [CLI Reference](specter/docs/CLI_REFERENCE.md) | All commands, options, and exit codes |
| [FAQ](specter/docs/FAQ.md) | Common questions about SDD and Specter |

## Background: Spec-Driven Development

Specter is the tooling implementation of the SDD methodology taught in [Mastering Spec-Driven Development](sddbook/README.md) -- a 17-chapter course covering the full lifecycle from writing specs to multi-agent orchestration.

## Dogfooding

Specter validates its own specs. The tool has 5 specs with 33 acceptance criteria, 37 tests, and passes its own structural checks. Every feature was specified before it was implemented.

```
$ specter coverage

Spec ID                 Tier  ACs     Covered  Coverage  Status
-----------------------------------------------------------------
spec-check              T1    6       6        100%      PASS
spec-coverage           T2    5       5        100%      PASS
spec-parse              T1    10      10       100%      PASS
spec-resolve            T1    7       7        100%      PASS
spec-sync               T2    5       5        100%      PASS

5 specs: 5 passing, 0 failing
```

## Tech Stack

- Go (single binary, zero runtime dependencies, cross-compiles to all platforms)
- santhosh-tekuri/jsonschema v6 (JSON Schema validation, draft 2020-12)
- Masterminds/semver v3 (dependency version matching)
- Cobra (CLI)
- gopkg.in/yaml.v3 (YAML parsing)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT

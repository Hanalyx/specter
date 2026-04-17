# Specter

**A type system for specs.** Validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

Specs without validation are just documents. They can contradict each other, reference dependencies that don't exist, have constraints that no test ever covers, and silently rot as code evolves. Specter treats specs as typed artifacts in a dependency graph, subject to the same static analysis you apply to code.

```
$ specter sync

  PASS  parse     5 spec(s) parsed — no schema violations
  PASS  resolve   5 specs, 8 dependencies — no cycles or broken refs
  PASS  check     0 errors, 0 orphan constraints
  PASS  coverage  5 spec(s) meet coverage thresholds

All checks passed.
```

---

## Human Intent, AI Execution

Specter's schema is deliberately detailed — constraints, acceptance criteria, tiers, provenance, coverage thresholds. Writing all of that by hand for every module would be impractical, and that was never the intention.

The intended workflow is a collaboration between you and your AI coding assistant:

1. **You provide intent** — a brief description of what a module should do, its key constraints, and any non-obvious judgement calls or trade-offs
2. **The AI writes the spec** — translating your intent into a fully structured `.spec.yaml` file with constraints, ACs, and tier assignments
3. **The AI writes the tests** — derived directly from the ACs in the spec
4. **You review** — the spec and tests are the approval gate; you validate that the AI correctly captured your intent before any implementation begins
5. **The AI implements** — with the spec as the contract and the tests as the verification

Specter enforces the discipline at every step: the spec must exist before code, tests must trace to ACs, and coverage must meet the tier threshold before `specter sync` passes. It makes the process infrastructure, not a suggestion.

**The core mission: guide your AI coding assistant through spec → test → implement → eval in the right order, every time, with your intent preserved throughout.**

---

## Install

### VS Code extension (recommended for most users)

Search **Specter SDD** in the Extensions panel. The extension auto-downloads the CLI binary matching the host's OS and architecture, installs it under `~/.specter/bin/`, and wires up the integrated terminal so `specter` works without further setup. To call `specter` from external terminals, run **Specter: Add CLI to Shell PATH** from the command palette once.

### CLI, Linux / macOS (tar.gz)

Asset names follow Go's `GOOS`/`GOARCH` conventions (lowercase `linux`/`darwin`, `amd64`/`arm64`) — not `uname`'s `Linux`/`x86_64`. This snippet translates and picks the latest version automatically:

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_${OS}_${ARCH}.tar.gz"
tar xzf "specter_${VERSION}_${OS}_${ARCH}.tar.gz"
sudo mv specter /usr/local/bin/
specter --version
```

### CLI, Debian / Ubuntu (.deb)

```bash
ARCH=$(dpkg --print-architecture)   # amd64 or arm64
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_linux_${ARCH}.deb"
sudo dpkg -i "specter_${VERSION}_linux_${ARCH}.deb"
```

### CLI, Fedora / RHEL / openSUSE (.rpm)

```bash
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_linux_${ARCH}.rpm"
sudo rpm -i "specter_${VERSION}_linux_${ARCH}.rpm"
```

### CLI, Windows (PowerShell)

```powershell
$version = (Invoke-RestMethod https://api.github.com/repos/Hanalyx/specter/releases/latest).tag_name -replace '^v',''
$asset = "specter_${version}_windows_amd64.zip"
Invoke-WebRequest -Uri "https://github.com/Hanalyx/specter/releases/download/v${version}/${asset}" -OutFile specter.zip
Expand-Archive specter.zip -DestinationPath "$env:USERPROFILE\.specter\bin"
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\.specter\bin", "User")
specter --version  # restart terminal first, or reload $env:Path
```

### Build from source

```bash
git clone https://github.com/Hanalyx/specter.git
cd specter/specter
make build
./bin/specter --version
```

### Manual download

If you prefer clicking, every asset is listed on the [Releases page](https://github.com/Hanalyx/specter/releases/latest). Naming pattern: `specter_<version>_<os>_<arch>.<ext>` — lowercase OS, `amd64`/`arm64` arch.

---

## The Pipeline

Specter runs five stages in sequence. Each stage catches a different class of problem:

```
.spec.yaml files
      │
   [parse]      Schema validation — missing fields, invalid IDs, wrong types
      │
  [resolve]     Dependency graph — cycles, dangling refs, version mismatches
      │
   [check]      Structural analysis — orphan constraints, spec conflicts
      │
  [coverage]    Traceability — ACs without tests, below-threshold tiers
      │
   [sync]       CI gate — runs all four, exits non-zero on any failure
```

### What each stage catches

**`specter parse`** — Catches malformed specs before anything else runs. Missing required fields, IDs that don't match the allowed pattern, invalid enum values, wrong types. Like a compiler catching syntax errors.

```bash
specter parse specs/auth.spec.yaml

# ERROR: spec-auth.spec.yaml [required] missing required field: 'acceptance_criteria'
# ERROR: spec-auth.spec.yaml [pattern]  constraint ID 'constraint-1' does not match C-NN format
```

**`specter resolve`** — Builds the dependency graph across all specs and validates it. Catches circular dependencies and references to specs that don't exist.

```bash
specter resolve

# ERROR: circular dependency: spec-a → spec-b → spec-a
# ERROR: spec-auth depends on spec-session@^1.0.0 but spec-session is not found
```

**`specter check`** — Finds structural problems within and between specs. An orphan constraint — one that no acceptance criterion references — is a constraint that can never be tested. A tier conflict catches when a Tier 1 spec depends on a Tier 3 spec.

```bash
specter check

# WARN: spec-auth [orphan_constraint] C-04 is not referenced by any AC
# ERROR: spec-payments [tier_conflict] Tier 1 spec depends on Tier 3 spec-util
```

**`specter coverage`** — Reads `@spec` and `@ac` annotations from your test files and produces a traceability matrix. Enforces tier-based coverage thresholds.

```bash
specter coverage

# Spec ID          Tier  ACs  Covered  Coverage  Status
# ─────────────────────────────────────────────────────
# spec-auth        T1    6    4        67%       FAIL  ← below 100% threshold
# spec-payments    T2    5    5        100%      PASS
```

**`specter sync`** — Runs all four stages and exits 0 only when everything passes. Put this in CI.

---

## Write a Spec

```yaml
spec:
  id: user-registration
  version: "1.0.0"
  status: approved
  tier: 1

  context:
    system: Auth service
    description: >
      Handles new user account creation. Email is the primary identifier.
      Passwords are hashed with bcrypt before storage — never stored in plaintext.

  objective:
    summary: >
      Register a new user with email and password.
      Return a session token on success.
    scope:
      excludes:
        - "Social login (OAuth) — separate spec"
        - "Email verification — handled post-registration"

  constraints:
    - id: C-01
      description: "Email MUST be a valid RFC 5322 address"
      type: technical
      enforcement: error
    - id: C-02
      description: "Password MUST be at least 12 characters"
      type: security
      enforcement: error
    - id: C-03
      description: "Passwords MUST be hashed with bcrypt, cost factor ≥ 12"
      type: security
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Returns 201 with session token when registration succeeds"
      references_constraints: ["C-01", "C-02", "C-03"]
    - id: AC-02
      description: "Returns 422 when email format is invalid"
      references_constraints: ["C-01"]
    - id: AC-03
      description: "Returns 422 when password is shorter than 12 characters"
      references_constraints: ["C-02"]
    - id: AC-04
      description: "Returns 409 when email is already registered"
      references_constraints: ["C-01"]
```

Validate it:

```bash
specter parse user-registration.spec.yaml
# PASS user-registration.spec.yaml — user-registration@1.0.0
```

---

## Annotate Tests

Link test functions to acceptance criteria with two comment lines. Specter reads these annotations to build the traceability matrix.

```go
// @spec user-registration
// @ac AC-01
func TestRegistration_ValidEmailAndPassword_Returns201(t *testing.T) {
    // ...
}

// @spec user-registration
// @ac AC-02
func TestRegistration_InvalidEmail_Returns422(t *testing.T) {
    // ...
}
```

Works in any language — the annotations are plain comments.

---

## Tier-Based Enforcement

Coverage thresholds scale with risk:

| Tier | Examples | Coverage required |
|---|---|---|
| **T1** — Security / Money | Auth, payments, encryption | 100% |
| **T2** — Business logic | Booking flow, pricing rules | 80% |
| **T3** — Utility | Formatters, helpers | 50% |

A Tier 1 spec below threshold is a CI failure. A Tier 3 spec below threshold is a warning.

---

## The Type System Analogy

Specs map to programming type concepts one-for-one:

| Type system | Specter equivalent |
|---|---|
| Type definition | Constraint — defines what's allowed |
| Function signature | Acceptance criterion — input → expected output |
| Import statement | `depends_on` — formal contract between specs |
| Type error | Spec conflict — caught before code runs |
| Unused variable | Orphan constraint — no AC references it |
| Missing null check | Coverage gap — an AC with no test |

---

## Project Structure

```
specter/
  cmd/specter/       CLI entry point (Cobra)
  internal/
    parser/          M1: YAML → validated SpecAST
    resolver/        M2: Dependency graph, cycle detection
    checker/         M3: Orphan constraints, structural conflicts
    coverage/        M4: Spec-to-test traceability matrix
    sync/            M5: CI pipeline orchestrator
    reverse/         M6: Reverse-compile specs from existing code
    schema/          Canonical types + embedded JSON Schema
  specs/             Specter's own specs (dogfooding)
  testdata/          Test fixtures
  docs/              User documentation
```

---

## Development

```bash
make check      # go vet + go test + go build — the CI gate
make dogfood    # run specter against its own specs
make build-all  # cross-compile for linux/darwin/windows
```

Every package in `internal/` is a pure function — no I/O, no CLI dependencies.

---

## Dogfooding

Specter validates its own specs. The tool has 5 specs covering its own pipeline, 33 acceptance criteria, and 37 annotated tests. Every feature was specified before it was implemented.

```
$ specter coverage

Spec ID          Tier  ACs  Covered  Coverage  Status
─────────────────────────────────────────────────────
spec-check       T1    6    6        100%      PASS
spec-coverage    T2    5    5        100%      PASS
spec-parse       T1    10   10       100%      PASS
spec-resolve     T1    7    7        100%      PASS
spec-sync        T2    5    5        100%      PASS

5 specs: 5 passing, 0 failing
```

---

## Documentation

| | |
|---|---|
| [Getting Started](docs/GETTING_STARTED.md) | Write and validate your first spec |
| [AI Prompts](docs/AI_PROMPTS.md) | Ready-to-use prompts for every stage of the SDD loop |
| [Spec Schema Reference](docs/SPEC_SCHEMA_REFERENCE.md) | Every field in the `.spec.yaml` format |
| [CLI Reference](docs/CLI_REFERENCE.md) | All commands, flags, and exit codes |
| [FAQ](docs/FAQ.md) | Common questions about SDD and Specter |

---

## License

MIT

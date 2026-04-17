# Specter CLI Reference

Specter is a spec compiler toolchain — "a type system for specs." It validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

---

## Installation

Install the VS Code extension for the smoothest path — it auto-downloads the CLI and sets PATH. For CLI-only installs (tar.gz, `.deb`, `.rpm`, Windows zip, or build from source), see the [Install section in the root README](../README.md#install). Asset naming pattern: `specter_<version>_<os>_<arch>.<ext>` with lowercase `linux`/`darwin`/`windows` and `amd64`/`arm64`.

---

## Global Options

```
specter --version             Print the Specter version
specter --help                Show top-level help
specter <command> --help      Show help for a specific command
```

---

## Commands

### `specter parse`

Parse and validate `.spec.yaml` files against the canonical JSON Schema.

**Synopsis:**

```
specter parse [files...] [--json]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `files...` | One or more `.spec.yaml` file paths. If omitted, discovers all `*.spec.yaml` files recursively from the current directory (or `specs_dir` from `specter.yaml`), skipping `testdata/` and configured excludes. |

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output results as JSON instead of human-readable text. |

**Examples:**

```
$ specter parse
PASS specs/auth.spec.yaml — spec-auth@1.0.0
PASS specs/payments.spec.yaml — spec-payments@2.1.0

$ specter parse specs/auth.spec.yaml --json
{
  "file": "specs/auth.spec.yaml",
  "ok": true,
  "value": { "id": "spec-auth", "version": "1.0.0", ... }
}

$ specter parse broken.spec.yaml
FAIL broken.spec.yaml
  error [required] spec.id: must have required property 'id'
  error [pattern] spec.constraints[0].id: must match pattern "^C-\d{2,}$"
```

**Exit codes:** `0` = all files valid. `1` = one or more errors, or no files found.

---

### `specter resolve`

Build and validate the spec dependency graph. Constructs a directed acyclic graph from `depends_on` declarations and detects structural graph issues.

**Synopsis:**

```
specter resolve [--json] [--dot] [--mermaid]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output the graph and diagnostics as JSON. |
| `--dot` | Output the dependency graph in DOT format (for Graphviz). |
| `--mermaid` | Output the dependency graph in Mermaid format (renders natively in GitHub PRs). |

**Diagnostics:**

| Diagnostic | Description |
|------------|-------------|
| `circular_dependency` | Two or more specs form a cycle. Reports the full cycle path. |
| `dangling_reference` | A `depends_on.spec_id` does not match any discovered spec. Suggests similar IDs and a fix path. |
| `version_mismatch` | A `depends_on.version_range` is not satisfied by the target spec's actual version. |
| `duplicate_id` | Two spec files declare the same `id`. |

**Example:**

```
$ specter resolve
Spec Graph: 4 specs, 4 dependencies

Resolution order:
  spec-parse@1.0.0
  spec-resolve@1.0.0 -> spec-parse
  spec-check@1.0.0 -> spec-parse, spec-resolve
  spec-coverage@1.0.0 -> spec-parse

No dependency issues found.

$ specter resolve --mermaid
graph BT
    spec-parse["spec-parse@1.0.0"]
    spec-resolve["spec-resolve@1.0.0"]
    spec-resolve -->|"^1.0.0"| spec-parse
```

**Exit codes:** `0` = no issues. `1` = one or more errors.

---

### `specter check`

Run structural type-checking rules across the spec dependency graph. Detects semantic consistency issues between connected specs.

**Synopsis:**

```
specter check [--json] [--tier <n>] [--strict]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output diagnostics as JSON. |
| `--tier <n>` | Override the tier enforcement level for all specs (1, 2, or 3). |
| `--strict` | Treat warnings as errors. Also configurable via `settings.strict` in `specter.yaml`. |

**Diagnostics:**

| Diagnostic | Severity by tier | Description |
|------------|-----------------|-------------|
| `orphan_constraint` | T1=error, T2=warning, T3=info | A constraint is not referenced by any acceptance criterion. Individual constraints may override severity via `constraint.enforcement`. |
| `structural_conflict` | error (override via `constraint.enforcement`) | An upstream constraint requires something that a downstream AC handles as absent. |
| `tier_conflict` | warning | A higher-tier spec depends on a lower-tier spec (e.g., Tier 1 depends on Tier 3). |

When a constraint has a `type` (e.g. `security`, `performance`), it appears in parentheses after the constraint ID so diagnostics can be grouped by category.

**Example:**

```
$ specter check
warn [orphan_constraint] spec-auth C-04 (security): C-04 is not referenced by any AC
error [tier_conflict] spec-payments: Tier 1 spec depends on Tier 3 spec-util

1 error(s), 1 warning(s), 0 info

$ specter check --strict
# Warnings are now treated as errors — exits 1
```

**Exit codes:** `0` = no errors (warnings allowed unless `--strict`). `1` = one or more errors.

---

### `specter coverage`

Generate a spec-to-test traceability matrix. Scans test files for `@spec` and `@ac` annotations and maps them to spec acceptance criteria. Enforces tier-based coverage thresholds.

**Synopsis:**

```
specter coverage [--json] [--tests <glob>]
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--json` | — | Output the coverage report as JSON. |
| `--tests <glob>` | auto-discover | Glob pattern for test files. Default discovers `*.test.ts`, `*.test.js`, `*.test.py`, `*_test.go`, `*_test.py`. |

**Annotation format:**

```typescript
// @spec user-registration
// @ac AC-01
test('valid registration returns 201', () => { ... });
```

```python
# @spec user-registration
# @ac AC-01
def test_valid_registration():
    ...
```

```go
// @spec user-registration
// @ac AC-01
func TestValidRegistration(t *testing.T) { ... }
```

**Coverage thresholds by tier:**

| Tier | Required Coverage |
|------|-------------------|
| 1 (Security / Money) | 100% |
| 2 (Core Business Logic) | 80% |
| 3 (Utility / Internal) | 50% |

**Example:**

```
$ specter coverage

Spec Coverage Report

Spec ID                  Tier   ACs      Covered   Coverage   Status
-----------------------------------------------------------------
spec-auth                T1     6        4         67%        FAIL
spec-payments            T2     5        5         100%       PASS
  uncovered: AC-01, AC-03

2 specs: 1 passing, 1 failing
```

**Exit codes:** `0` = all specs meet thresholds. `1` = one or more below threshold.

---

### `specter sync`

Run the full validation pipeline: parse → resolve → check → coverage. Exits non-zero on any failure. This is the CI gate command.

**Synopsis:**

```
specter sync [--json] [--tests <glob>] [--only <phase>] [--strict]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output the pipeline result as JSON. |
| `--tests <glob>` | Glob pattern for test files. |
| `--only <phase>` | Run only one phase: `parse`, `resolve`, `check`, or `coverage`. Prerequisites run without halting on failure. |
| `--strict` | Treat warnings as errors. |

**Example:**

```
$ specter sync

Specter Sync

  PASS parse: 5 spec(s) parsed — no schema violations
  PASS resolve: 5 specs, 8 dependencies — no cycles or broken refs
  PASS check: 0 errors, 0 orphan constraints
  PASS coverage: 5 spec(s) meet coverage thresholds

All checks passed.
```

**CI integration (GitHub Actions):**

```yaml
- name: Validate specs
  run: specter sync
```

**Exit codes:** `0` = all phases pass. `1` = any phase fails.

---

### `specter reverse`

Extract draft `.spec.yaml` files from existing source code. Analyzes source and test files using language-specific adapters to extract constraints from validation schemas and acceptance criteria from test assertions.

**Synopsis:**

```
specter reverse [path] [--adapter <lang>] [--output <dir>] [--group-by <strategy>]
                [--dry-run] [--overwrite] [--exclude <pattern>] [--json]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `path` | Directory to analyze (default: `.`). |

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--adapter <lang>` | auto | Language adapter: `typescript`, `python`, `go`. Auto-detects from file extensions if omitted. |
| `--output <dir>` / `-o` | `specs` | Output directory for generated `.spec.yaml` files. |
| `--group-by <strategy>` | `file` | Grouping strategy: `file` (one spec per source file) or `directory` (one spec per directory). |
| `--dry-run` | false | Preview generated YAML to stdout without writing files. |
| `--overwrite` | false | Overwrite existing spec files. Default skips files that already exist. |
| `--exclude <pattern>` | — | Exclude paths matching pattern. Can be repeated. |
| `--json` | false | Output results as JSON. |

**Example:**

```
$ specter reverse src/auth --output specs/auth
GENERATED specs/auth/auth-service.spec.yaml — auth-service@1.0.0 (3 constraints, 5 ACs)
  warning: AC-03 description is a gap — review and complete manually

Summary: 1 spec(s) generated, 3 constraint(s), 4 assertion(s), 1 gap(s)

DRAFT: 1 AC(s) require manual review — specter reverse can extract structure but not intent.
       Review each gap and fill in description, inputs, and expected_output.

$ specter reverse --dry-run  # Preview without writing
```

**Supported languages:** TypeScript, Python, Go. Extracts constraints from Zod/Yup schemas (TypeScript), Pydantic models (Python), and validation logic (Go).

**Exit codes:** `0` = one or more specs generated. `1` = no specs generated.

---

### `specter init`

Initialize a `specter.yaml` project manifest, or scaffold a draft `.spec.yaml` from a template.

**Synopsis:**

```
specter init [--name <name>] [--force] [--template <type>]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--name <name>` | System name for the manifest. Defaults to the current directory name. |
| `--force` | Overwrite an existing `specter.yaml`. |
| `--template <type>` | Create a draft `.spec.yaml` from a template instead of a manifest. Types: `api-endpoint`, `service`, `auth`, `data-model`. |

**Example:**

```
$ specter init
Created specter.yaml with 5 spec(s) in system "my-project"

$ specter init --template api-endpoint
Created api-endpoint.spec.yaml (template: api-endpoint)
Edit the file to replace placeholder values, then run: specter sync
```

---

### `specter doctor`

Run pre-flight project health checks before the full pipeline. Reports `PASS`, `WARN`, or `FAIL` for each check.

**Synopsis:**

```
specter doctor
```

**Checks performed:**

| Check | PASS | WARN | FAIL |
|-------|------|------|------|
| `manifest` | `specter.yaml` found | No `specter.yaml` (optional) | — |
| `spec-files` | ≥1 `.spec.yaml` found | — | No spec files found |
| `parse` | All specs parse cleanly | — | Parse errors in ≥1 spec |
| `annotations` | `@spec`/`@ac` annotations found in tests | No annotations found | — |
| `coverage` | All specs meet tier thresholds | — | ≥1 spec below threshold |

**Example:**

```
$ specter doctor

specter doctor

  manifest     [PASS]  specter.yaml found at specter.yaml
  spec-files   [PASS]  5 spec file(s) discovered
  parse        [PASS]  All specs parse cleanly
  annotations  [WARN]  No @spec/@ac annotations found in test files
  coverage     [WARN]  No specs to check coverage for

Result: OK — project is ready for `specter sync`
```

**Exit codes:** `0` = all checks PASS or WARN. `1` = any check FAIL.

---

### `specter explain`

Show coverage status and annotation examples for a spec's acceptance criteria.

**Synopsis:**

```
specter explain <spec-id>[:<ac-id>]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `<spec-id>` | The spec ID to explain. Lists all ACs with COVERED/UNCOVERED status. |
| `<spec-id>:<ac-id>` | Show full details and annotation example for one AC. |

**Example:**

```
$ specter explain user-registration

specter explain user-registration

  Status   AC        Description
  ------------------------------------------------------------
  COVERED  AC-01     Valid email and password creates user and...
  UNCOVERED AC-02    Invalid email format returns 400...
  UNCOVERED AC-03    Weak password returns 400...

  Scanned 12 test file(s)
  Run `specter explain user-registration:<ac-id>` for annotation examples

$ specter explain user-registration:AC-02

specter explain user-registration:AC-02

  Spec:   user-registration (v1.0.0, tier 1)
  AC-02:  Invalid email format returns 400 with field-level error
  Status: UNCOVERED

  To cover this AC, add annotations in your test file:

  TypeScript / JavaScript:
    // @spec user-registration
    // @ac AC-02
    it('AC-02: Invalid email format returns 400 with field-level error', () => {
      // test implementation
    });
```

---

### `specter watch`

Re-run the full sync pipeline whenever spec or test files change. Uses `fsnotify` with a 150ms debounce to coalesce rapid saves.

**Synopsis:**

```
specter watch
```

Runs once immediately on startup, then re-runs on every `.spec.yaml` or test file change. Press `Ctrl+C` to stop.

**Example:**

```
$ specter watch

specter watch

  Watching: specs, test files
  Press Ctrl+C to stop

[14:32:01] PASS  5 spec(s)  33/33 ACs covered  (5 passing, 0 failing)
[14:32:15] FAIL  parse
[14:32:22] PASS  5 spec(s)  33/33 ACs covered  (5 passing, 0 failing)
```

---

### `specter diff`

Show a semantic diff of a spec between two git revisions (or between any two versions on disk). Classifies the overall change as `breaking`, `additive`, `patch`, or `unchanged`.

**Synopsis:**

```
specter diff <path>[@<ref>] <path>[@<ref>]
```

Each argument is `path` (read from disk) or `path@ref` (read from git).

**Change classes:**

| Class | Meaning |
|-------|---------|
| `breaking` | ACs or constraints removed, or descriptions changed in a way that narrows the contract. Requires a MAJOR version bump. |
| `additive` | New ACs or constraints added. Requires a MINOR version bump. |
| `patch` | Wording-only changes that don't alter meaning. PATCH version bump. |
| `unchanged` | No changes detected. |

**Example:**

```
$ specter diff specs/auth.spec.yaml@HEAD~3 specs/auth.spec.yaml

spec spec-auth 1.0.0 → 1.1.0 [additive]

  +AC-05: Returns 401 when token is expired
  ~C-02: MUST require 8-character passwords → MUST require 12-character passwords

$ specter diff specs/auth.spec.yaml specs/auth.spec.yaml
spec spec-auth 1.1.0 → 1.1.0: no changes
```

---

## The `specter.yaml` Manifest

An optional `specter.yaml` file at the project root configures discovery, thresholds, and settings. Specter searches upward from the current directory to find it.

```yaml
system: my-project

specs_dir: specs       # Where to look for .spec.yaml files (default: .)

settings:
  strict: false        # Treat warnings as errors
  warn_on_draft: false # Warn when draft specs are found

coverage_thresholds:   # Override default tier thresholds
  1: 100
  2: 80
  3: 50

exclude:               # Directory names to skip during discovery
  - testdata
  - node_modules
  - dist
```

---

## `.specterignore` File

An optional `.specterignore` file in the project root controls which paths are skipped during spec discovery. Follows `.gitignore` conventions.

```
# Ignore test fixtures
testdata/

# Ignore generated specs
specs/generated/
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed. |
| `1` | One or more errors, or no spec files found. |

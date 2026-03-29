# Specter CLI Reference

**Version:** 0.1.0
**Status:** M1 (Schema + Parse)

Specter is a spec compiler toolchain -- "a type system for specs." It validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

---

## Installation

```bash
# From the specter directory
make build

# Run directly
bin/specter <command>

# Or build with Go and install to $GOPATH/bin
go build -o bin/specter ./cmd/specter/
```

Requires Go 1.22 or later.

---

## Global Options

```
specter --version    Print the Specter version
specter --help       Show top-level help
specter <command> --help   Show help for a specific command
```

---

## Commands

### `specter parse`

**Status:** Available (M1)

Parse and validate `.spec.yaml` files against the canonical JSON Schema.

**Synopsis:**

```
specter parse [files...] [--json]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `files...` | No | One or more `.spec.yaml` file paths. If omitted, discovers all `*.spec.yaml` files in the current directory tree (excluding `.git/` and `testdata/`). |

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output results as JSON instead of human-readable text. |

**Examples:**

Parse all spec files in the current directory:

```
$ specter parse
PASS specs/spec-parse.spec.yaml -- spec-parse@1.0.0
PASS specs/spec-resolve.spec.yaml -- spec-resolve@1.0.0
PASS specs/spec-check.spec.yaml -- spec-check@1.0.0
PASS specs/spec-coverage.spec.yaml -- spec-coverage@1.0.0
```

Parse a single file:

```
$ specter parse specs/spec-parse.spec.yaml
PASS specs/spec-parse.spec.yaml -- spec-parse@1.0.0
```

Parse a file with validation errors:

```
$ specter parse broken.spec.yaml
FAIL broken.spec.yaml
  error [required] spec.id: must have required property 'id'
  error [additionalProperties] spec.extra_field:3: must NOT have additional properties
```

JSON output for CI integration:

```
$ specter parse specs/spec-parse.spec.yaml --json
{
  "file": "specs/spec-parse.spec.yaml",
  "ok": true,
  "value": {
    "id": "spec-parse",
    "version": "1.0.0",
    "status": "approved",
    "tier": 1,
    ...
  }
}
```

JSON output for a failing file:

```
$ specter parse broken.spec.yaml --json
{
  "file": "broken.spec.yaml",
  "ok": false,
  "errors": [
    {
      "type": "required",
      "path": "spec.id",
      "message": "must have required property 'id'"
    }
  ]
}
```

**Behavior:**

- Validates each file against the canonical JSON Schema (`internal/parser/spec-schema.json`).
- Reports errors with the YAML line number and JSON field path when available.
- Collects all validation errors before returning (does not fail-fast on the first error).
- Rejects unknown fields (`additionalProperties` enforcement).
- Supports YAML anchors and aliases (`&anchor` / `*alias`).

---

### `specter resolve`

**Status:** Available

Build and validate the spec dependency graph. Discovers all `.spec.yaml` files in the project, parses them, and constructs a directed acyclic graph based on `depends_on` declarations. Detects structural graph issues that would cause downstream tools to produce incorrect results.

**Synopsis:**

```
specter resolve [--json] [--dot]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output the graph and diagnostics as JSON. |
| `--dot` | Output the dependency graph in DOT format (for Graphviz). |

**Diagnostics:**

| Diagnostic | Description |
|------------|-------------|
| `circular_dependency` | Two or more specs form a cycle (e.g., A depends on B, B depends on A). Reports the full cycle path. |
| `dangling_reference` | A `depends_on.spec_id` does not match any discovered spec. |
| `version_mismatch` | A `depends_on.version_range` is not satisfied by the target spec's actual version (semver). |
| `duplicate_id` | Two spec files declare the same `id`. |

**Example:**

```
$ specter resolve
Spec Graph: 4 specs, 4 dependencies

Resolution order:
  spec-parse@1.0.0
  spec-resolve@1.0.0 -> spec-parse
  spec-coverage@1.0.0 -> spec-parse
  spec-check@1.0.0 -> spec-parse, spec-resolve

No dependency issues found.
```

**Behavior:**

- Discovers `.spec.yaml` files recursively from the project root.
- Respects `.specterignore` patterns (see below).
- Produces a typed `SpecGraph` with nodes, edges, and topological ordering.
- Reports all circular dependencies, not just the first one found.

---

### `specter check`

**Status:** Coming in M3

Run structural type-checking rules across the spec dependency graph. Detects semantic consistency issues between connected specs.

**Synopsis:**

```
specter check [--json] [--tier <tier>]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output diagnostics as JSON. |
| `--tier <tier>` | Override the tier enforcement level (1, 2, or 3). |

**Planned diagnostics:**

| Diagnostic | Description |
|------------|-------------|
| `orphan_constraint` | A constraint is not referenced by any acceptance criterion. Severity depends on tier. |
| `structural_conflict` | A downstream spec contradicts an upstream constraint (e.g., upstream says field MUST exist, downstream handles its absence). |
| `breaking_change` | A field was removed between spec versions (requires major version bump). |
| `additive_change` | An optional field was added between spec versions (requires minor version bump). |

**Tier-aware severity:**

| Tier | Orphan constraint severity | Coverage expectation |
|------|---------------------------|---------------------|
| 1 (Critical) | Error | Strict |
| 2 (Standard) | Warning | Standard |
| 3 (Advisory) | Info | Relaxed |

---

### `specter coverage`

**Status:** Coming in M4

Generate a spec-to-test traceability matrix. Scans test files for `@spec` and `@ac` annotations and maps them back to spec acceptance criteria.

**Synopsis:**

```
specter coverage [--json] [--tests <glob>]
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--json` | -- | Output the coverage report as JSON. |
| `--tests <glob>` | `**/*.test.{ts,js,py}` | Glob pattern for discovering test files. |

**Annotation format:**

Specter coverage relies on explicit annotations in test files:

```typescript
// @spec user-auth
// @ac AC-01
test('valid credentials return a session token', () => {
  // ...
});

// @ac AC-02
test('expired credentials are rejected', () => {
  // ...
});
```

```python
# @spec user-auth
# @ac AC-01
def test_valid_credentials():
    ...
```

Both `//` (Go, JavaScript, TypeScript) and `#` (Python, Ruby, Shell) comment styles are recognized. Go tests use the `//` style.

**Planned behavior:**

- Maps `@spec` and `@ac` annotations to parsed specs.
- Calculates coverage percentage per spec: `(covered ACs / total ACs) * 100`.
- Identifies uncovered ACs and specs with zero test coverage.
- Enforces tier-aware coverage thresholds:

| Tier | Required Coverage |
|------|-------------------|
| 1 (Critical) | 100% |
| 2 (Standard) | 80% |
| 3 (Advisory) | 50% |

---

### `specter init`

**Status:** Coming in a future milestone

Scaffold a new `.spec.yaml` file with the canonical structure.

**Synopsis:**

```
specter init <name> [--tier <tier>]
```

**Arguments:**

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Spec name in kebab-case (e.g., `user-auth`). |

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--tier <tier>` | `2` | Risk tier: 1 (critical), 2 (standard), or 3 (advisory). |

**Planned behavior:**

- Creates a `<name>.spec.yaml` file with all required fields pre-populated.
- Sets the initial version to `1.0.0` and status to `draft`.
- Includes placeholder sections for context, objective, constraints, and acceptance criteria.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All files valid, no diagnostics with error severity. |
| `1` | One or more validation errors or failing diagnostics found. Also returned when no spec files are found. |

---

## `.specterignore` File

Specter respects a `.specterignore` file in the project root. The format follows `.gitignore` conventions: one glob pattern per line, with `#` for comments.

**Example:**

```
# Ignore test fixtures -- these are for specter's own tests, not real specs
testdata/

# Ignore build output
bin/
```

The `.specterignore` file is used by `specter resolve` during recursive file discovery. The `specter parse` command uses its own ignore list (`.git/`, `testdata/`) when no explicit files are provided.

---

## Milestone Roadmap

| Milestone | Command | Status |
|-----------|---------|--------|
| M1 | `specter parse` | Available |
| M2 | `specter resolve` | Available |
| M3 | `specter check` | Planned |
| M4 | `specter coverage` | Planned |
| M5 | `spec-sync` (CI enforcement) | Planned |
| M6 | Reverse compiler (code-to-spec) | Planned |

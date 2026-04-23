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
specter coverage [--json] [--failing] [--strict] [--scope <domain>] [--tests <glob>]
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--json` | — | Output the coverage report as JSON. |
| `--failing` | — | Show only specs below 100% coverage in the table. Summary header still reflects the full report. When all specs are at 100%, emits a single-line confirmation instead of an empty table. Added in v0.9.2. |
| `--strict` | — | Require `.specter-results.json` and treat any annotated AC whose status is not `passed` as uncovered, across **all tiers**. Missing file is a hard failure; empty file emits a warning and proceeds. Pairs with `specter ingest`. Added in v0.10. |
| `--scope <domain>` | — | Narrow `--strict`'s demand to ACs of specs in the named `specter.yaml` domain. Specs outside the domain fall back to v0.9 boolean-passed logic. Enables staged adoption. Requires `--strict`; unknown domain fails fast. Added in v0.10. |
| `--tests <glob>` | auto-discover | Glob pattern for test files. Default discovers `*.test.ts`, `*.test.js`, `*.test.py`, `*_test.go`, `*_test.py`. |

**Annotation format:**

Specter reads annotations from two places.

1. **Source comments** above the test function: `// @spec <id>` and `// @ac AC-NN`. `specter coverage` counts these.
2. **Test title or runtime log** carrying `<spec-id>/AC-NN`. `specter ingest` reads this. `specter coverage --strict` requires it.

Source comments alone: `coverage` counts it, `--strict` demotes it. Write both forms.

```typescript
// @spec user-registration
// @ac AC-01
test('[user-registration/AC-01] valid registration returns 201', () => { ... });
```

```python
# @spec user-registration
# @ac AC-01
def test_user_registration_AC_01_valid_returns_201():
    ...
```

```go
// @spec user-registration
// @ac AC-01
func TestUserRegistration(t *testing.T) {
    t.Run("user-registration/AC-01 valid returns 201", func(t *testing.T) {
        // ...
    })
}
```

**Rules for runner-visible annotations:**

- Spec id is kebab-case, lowercase: `[a-z][a-z0-9-]*[a-z0-9]`.
- AC id is zero-padded: `AC-01`, not `AC-1`. Must match the spec's AC id exactly.
- Separator between spec id and AC id is `/` or `:`.
- One test (or subtest) covers one `(spec-id, AC-NN)` pair. Do not put two ACs in one test.

**Alternate form — runtime log.** When you can't rename titles (shared naming, snapshot tests, external contracts), emit the pair from inside the test body:

```typescript
test('rejects zero amount', () => {
  console.log('// @spec payment-charge');
  console.log('// @ac AC-03');
  // assertions
});
```

```go
func TestCharge_ZeroAmount(t *testing.T) {
    t.Log("// @spec payment-charge")
    t.Log("// @ac AC-03")
    // assertions
}
```

Pick one form per file. Do not mix title-based and runtime-log forms in the same file.

**Coverage thresholds by tier:**

| Tier | Required Coverage |
|------|-------------------|
| 1 (Security / Money) | 100% |
| 2 (Core Business Logic) | 80% |
| 3 (Utility / Internal) | 50% |

**Example (table output, v0.9.2+):**

```
$ specter coverage

Spec Coverage Report — 2 specs · 83% avg coverage
  Tier 1: 0/1 passing (0%)
  Tier 2: 1/1 passing (100%)

Spec ID                                   Tier   ACs      Covered   Coverage   Status
----------------------------------------------------------------------------------
spec-auth                                 T1     6        4         67%        FAIL
  uncovered: AC-01, AC-03
spec-payments                             T2     5        5         100%       PASS

2 specs: 1 passing, 1 failing
```

**Table output shape (since v0.9.2):**

- A **summary header** precedes the table: total-specs count, arithmetic-mean coverage, and per-tier breakdown (`Tier K: X/Y passing (Z%)`). Tiers with zero specs in the workspace are omitted.
- Entries are **sorted worst-first**: failing (below threshold) → partial (below 100% but passing threshold) → 100% covered. Within each bucket, tier descending (T1 before T2 before T3) so higher-risk specs surface first.
- Spec IDs longer than 40 characters are **truncated** in the table with a trailing ellipsis (`…`). This keeps column alignment on workspaces with long path-derived IDs. The `--json` output is unaffected — it emits the full spec_id.

**Example (`--failing`, v0.9.2+):**

```
$ specter coverage --failing

Spec Coverage Report — 2 specs · 83% avg coverage
  Tier 1: 0/1 passing (0%)
  Tier 2: 1/1 passing (100%)

Spec ID                                   Tier   ACs      Covered   Coverage   Status
----------------------------------------------------------------------------------
spec-auth                                 T1     6        4         67%        FAIL
  uncovered: AC-01, AC-03

2 specs: 1 passing, 1 failing
```

When every spec is at 100%, `--failing` emits a single-line confirmation in place of the empty table:

```
$ specter coverage --failing

Spec Coverage Report — 14 specs · 100% avg coverage
  Tier 1: 3/3 passing (100%)
  Tier 2: 9/9 passing (100%)
  Tier 3: 2/2 passing (100%)

All 14 specs at 100% coverage.
```

**Example (`--json`):**

Since v0.9.0, `--json` **always emits a CoverageReport JSON document to stdout**, including when one or more spec files fail to parse. The process exit code signals pass/fail; the presence of JSON does not. This is a breaking change from earlier versions which emitted no JSON on parse failure.

```json
{
  "entries": [
    {
      "spec_id": "spec-auth",
      "tier": 1,
      "total_acs": 6,
      "covered_acs": ["AC-01", "AC-02", "AC-03", "AC-04"],
      "uncovered_acs": ["AC-05", "AC-06"],
      "coverage_pct": 66.7,
      "threshold": 100,
      "passes_threshold": false,
      "test_files": ["tests/auth/login.test.ts"],
      "spec_file": "specs/spec-auth.spec.yaml"
    }
  ],
  "summary": {
    "total_specs": 1,
    "fully_covered": 0,
    "partially_covered": 1,
    "uncovered": 0,
    "passing": 0,
    "failing": 1
  },
  "spec_candidates_count": 1
}
```

When specs fail to parse, the report carries a `parse_errors` array and a grouped `parse_error_patterns` summary:

```json
{
  "entries": [],
  "summary": { "total_specs": 0, "passing": 0, "failing": 0, ... },
  "parse_errors": [
    {
      "file": "specs/broken.spec.yaml",
      "path": "spec.objective",
      "type": "required",
      "message": "Missing required field 'objective'",
      "line": 12,
      "column": 3
    }
  ],
  "parse_error_patterns": [
    {
      "type": "required",
      "path": "spec.objective",
      "count": 20,
      "example_file": "specs/auth.spec.yaml",
      "files": ["specs/auth.spec.yaml", "specs/payments.spec.yaml", ...]
    }
  ],
  "spec_candidates_count": 22
}
```

**Report field reference:**

| Top-level field | Since | Description |
|---|---|---|
| `entries[]` | v1.0 | One per parseable spec. Always present; may be empty. |
| `summary` | v1.0 | Roll-up counts. |
| `parse_errors` | v0.9.0 | Per-file schema violations. Absent or `null` when every spec parsed cleanly. |
| `parse_error_patterns` | v0.9.0 | `parse_errors` grouped by `(type, path)` sorted by count desc. Useful for naming schema drift ("20 specs missing `objective`"). |
| `spec_candidates_count` | v0.9.0 | Number of `.spec.yaml` files discovered on disk before parsing. Distinguishes "no specs exist" (count 0) from "specs exist but failed to parse" (count > 0, entries empty). |

| Entry field | Since | Description |
|---|---|---|
| `spec_file` | v0.9.0 | Path to the source `.spec.yaml` for this entry. Lets downstream consumers open the file. |

**Exit codes:**
- `0` — all specs parsed AND all meet their coverage thresholds
- `1` — one or more specs failed to parse, OR one or more specs are below threshold

**Consuming the JSON programmatically:**

Since v0.9.0 emits JSON in every state, the pattern for scripting is:

```bash
specter coverage --json > /tmp/cov.json
rc=$?
if [ "$(jq '.parse_errors | length' /tmp/cov.json)" -gt 0 ]; then
  echo "Parse errors — fix spec files first"
  exit 2
elif [ $rc -ne 0 ]; then
  echo "Coverage below threshold"
  exit 1
fi
```

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
specter init --refresh [--dry-run]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--name <name>` | System name for the manifest. Defaults to the current directory name. |
| `--force` | Overwrite an existing `specter.yaml`. Mutually exclusive with `--refresh`. |
| `--template <type>` | Create a draft `.spec.yaml` from a template instead of a manifest. Types: `api-endpoint`, `service`, `auth`, `data-model`. |
| `--refresh` | Update only `domains.default.specs` in an existing `specter.yaml`. Preserves every other field — `settings`, `registry`, tier overrides, custom domains. Added in v0.9.2. |
| `--dry-run` | Used with `--refresh`: print the proposed diff to stdout without writing the file. Added in v0.9.2. |

**Behaviour (v0.9.0+):**

`specter init` scans the workspace's `specs/` directory and populates the manifest's default domain based on what it finds.

- **Greenfield workspace (no spec files):** emits a manifest with an empty `domains.default` entry whose description invites you to add spec IDs as you author them.
- **Workspace with parseable specs:** reads each one, extracts its `spec.id`, and populates `domains.default.specs: [...]`.
- **Workspace with specs that fail to parse:** still writes the manifest (with an explanatory placeholder default domain) and prints a warning that includes a **Pattern analysis** block naming the shape of the failure — if every discovered spec hit the same error, init calls out schema version drift and points at `specter doctor` for deeper diagnosis.

**Important (v0.9.0+):** init always emits a `domains:` section, even in the greenfield case. Previous versions omitted `domains:` entirely when no spec IDs were discovered, which caused later `specter sync` runs to silently skip every spec the user added afterward — a silent-exclusion footgun now eliminated.

**Example (greenfield):**

```
$ specter init
Created specter.yaml with 0 spec(s) in system "my-project"
```

**Example (existing parseable specs):**

```
$ specter init
Created specter.yaml with 14 spec(s) in system "specter"
```

**Example (existing specs with schema drift):**

```
$ specter init
Created specter.yaml with 0 spec(s) in system "my-project"

Warning: 22 spec file(s) were discovered but could not be parsed:
  Every failing spec hit the same error: [additionalProperties] at "spec".
  This is the signature of schema version drift — the specs may
  have been written against an older Specter schema. Run `specter
  doctor` for a full report, then fix the specs and re-run
  `specter init --force` to populate the manifest.

The manifest was still written with an empty default domain as a
placeholder. Add your spec IDs under `domains.default.specs` once
the parse errors are resolved.
```

**Refresh mode (v0.9.2+):**

`specter init --refresh` is the non-destructive counterpart to `--force`. It reads the existing `specter.yaml`, rescans `settings.specs_dir` (or default `specs/`), and updates **only** `domains.default.specs` with the current on-disk spec set. Every other field is preserved — `settings`, `registry`, system metadata, and any custom domains declared under `domains.<name>` (anything that isn't `default`).

Specs claimed by a custom domain (listed under a non-default `domains.<name>.specs`) stay in that domain and are **not** migrated into `default`. A spec belongs to exactly one domain.

Specs that were previously listed in `domains.default.specs` but are no longer discoverable on disk (deleted, renamed, or now failing to parse) are removed from the list. The summary line reports the change counts.

**Example (`--refresh`):**

```
$ specter init --refresh
updated specter.yaml: +1 added, -0 removed
```

**Example (`--refresh --dry-run`):**

Prints the proposed diff without writing. The file on disk is byte-identical before and after. Useful for review before committing.

```
$ specter init --refresh --dry-run
Dry run — no changes will be written.

Proposed changes to domains.default.specs:
  + spec-payments
  - spec-legacy-auth

Run `specter init --refresh` (without --dry-run) to apply.
```

**Flag conflict:** `--refresh` and `--force` are mutually exclusive. `--force` rewrites the entire manifest; `--refresh` is surgical. Combining them exits non-zero with a clear error.

**Example (template):**

```
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

**Example (happy path):**

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

**Pattern analysis on parse failure (v0.9.0+):**

When the parse check fails, `specter doctor` prints a **Pattern analysis** block that groups errors by `(type, path)`. If every discovered spec hit the same pattern, doctor names it explicitly as the signature of schema version drift — a common shape for projects whose specs predate the current schema.

```
$ specter doctor

  manifest     [PASS]  specter.yaml found at specter.yaml
  spec-files   [PASS]  22 spec file(s) discovered
    specs/auth.spec.yaml: Unknown field 'trust_level'. Remove it or check for a typo in the field name.
    specs/payments.spec.yaml: Unknown field 'trust_level'. Remove it or check for a typo in the field name.
    ...
  parse        [FAIL]  22 spec file(s) have parse errors (see above)

  Pattern analysis:
    Every 22 discovered spec hit the same failure: [additionalProperties] at "spec".
    This pattern is the signature of schema version drift —
    your specs may have been written against an older Specter
    schema. Check the spec-parse changelog and migrate each file.

  annotations  [PASS]  8 annotation(s) found across 45 test file(s)
  coverage     [WARN]  Skipping coverage check — specs have parse errors

Result: FAIL — fix the issues above before running `specter sync`
```

When errors are heterogeneous (multiple distinct failure shapes), doctor lists the top patterns with counts instead of claiming drift:

```
  Pattern analysis:
    [required] at "spec.objective" — 12 occurrence(s) across 12 file(s)
    [enum] at "spec.status" — 3 occurrence(s) across 3 file(s)
    [additionalProperties] at "spec" — 2 occurrence(s) across 2 file(s)
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

### `specter ingest`

Convert CI-native test output (JUnit XML, `go test -json`) into `.specter-results.json`, the file `specter coverage --strict` reads to demote annotated-but-failing ACs. Added in v0.10.

**Synopsis:**

```
specter ingest [--junit <path>] [--go-test <path>] [--output <path>] [--verbose]
```

At least one of `--junit` or `--go-test` is required. Multiple sources can be combined in one invocation — results are merged (worst status wins per AC).

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--junit <path>` | — | JUnit XML file (vitest, jest, pytest, playwright). |
| `--go-test <path>` | — | Newline-delimited JSON from `go test -json`. |
| `--output <path>` | `.specter-results.json` | Where to write the merged results. |
| `--verbose` | — | Emit one stderr line per dropped testcase (testcases without a recognizable `(spec_id, ac_id)` annotation). Off by default; the summary line is always emitted. |

**Diagnostics:** every run writes to stderr a summary line:

```
Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation.
```

If `M` is 0 despite `N` being non-zero, your tests carry annotations only in source comments — those are invisible to `ingest` by design. See the explainer's Conventions A (test title) and B (runtime `t.Log`) for migrating.

**Annotation extraction:**

Each test needs a discoverable `(spec_id, ac_id)` pair or it's dropped silently. Sources in order of preference:

1. **Test name** — `spec-id/AC-NN` or `spec-id:AC-NN` embedded in the test case name.
2. **Classname** — same pattern, parsed from the JUnit `classname` attribute.
3. **Test body** — `// @spec <id>` and `// @ac <AC-id>` comments surfaced via `system-out` (JUnit) or `output`-action lines (go test -json).

**Status mapping:**

| Source | Maps to |
|--------|---------|
| JUnit `<testcase>` with no children | `passed` |
| JUnit `<failure>` child | `failed` |
| JUnit `<skipped>` child | `skipped` |
| JUnit `<error>` child | `errored` |
| go test `{"Action":"pass"}` | `passed` |
| go test `{"Action":"fail"}` | `failed` |
| go test `{"Action":"skip"}` | `skipped` |

**Worst-status rule:** when the same `(spec_id, ac_id)` is hit by multiple tests, the emitted entry uses the worst observed status: `errored > failed > skipped > passed`. One failing test is sufficient to demote an AC.

**Example (CI):**

```yaml
# run tests, emit JUnit
- run: pytest --junitxml=test-results/pytest.xml
- run: vitest run --reporter=junit > test-results/vitest.xml

# ingest, then gate
- run: specter ingest --junit 'test-results/*.xml' --output .specter-results.json
- run: specter coverage --strict
```

**Example (local):**

```bash
$ go test -json ./... > /tmp/go-test.json
$ specter ingest --go-test /tmp/go-test.json
Wrote 34 result entries to .specter-results.json

$ specter coverage --strict
Spec Coverage Report — 14 specs · 98% avg coverage
  Tier 1: 4/4 passing (100%)
  Tier 2: 9/10 passing (90%)
...
```

Pairs with `specter coverage --strict`. Without `ingest`, `--strict` fails with `--strict requires .specter-results.json — run 'specter ingest' first`.

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

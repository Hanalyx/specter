# Specter CLI Reference

Specter is a spec compiler toolchain, "a type system for specs." It validates, links, and type-checks `.spec.yaml` files the way `tsc` validates `.ts` files.

---

## Installation

Install the VS Code extension for the smoothest path: it auto-downloads the CLI and sets PATH. For CLI-only installs (tar.gz, `.deb`, `.rpm`, Windows zip, or build from source), see the [Install section in the Specter README](../README.md#install). Asset naming pattern: `specter_<version>_<os>_<arch>.<ext>` with lowercase `linux`/`darwin`/`windows` and `amd64`/`arm64`.

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

#### `specter resolve dependents <spec-id>`

Reverse traversal of the dependency graph: returns all specs whose `depends_on` includes the given spec id (direct dependents only). Bare `specter resolve` keeps its build-and-validate behavior; this sub-subcommand switches to query mode.

Exit code 0 even when no dependents exist (an empty set is a valid result); non-zero only when the spec id does not exist in the resolved graph.

**Synopsis:**

```
specter resolve dependents <spec-id> [--json]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output as JSON: `{"spec_id": "<id>", "dependents": ["<id1>", ...]}` |

**Example:**

```
$ specter resolve dependents spec-parse
spec-check
spec-coverage
spec-doctor
spec-explain
spec-manifest
spec-resolve
spec-reverse
spec-sync
spec-vscode

$ specter resolve dependents leaf-spec     # no dependents → empty, exit 0
$ echo $?
0

$ specter resolve dependents unknown-spec  # not in graph → error, exit 1
error: spec "unknown-spec" not found in graph
```

Future operations (`dependencies` for forward traversal, `cycles` for cycle enumeration, etc.) follow the same `specter resolve <op>` pattern.

---

### `specter check`

Run structural type-checking rules across the spec dependency graph. Detects semantic consistency issues between connected specs.

**Synopsis:**

```
specter check [--json] [--tier <n>] [--strict] [--test] [--concrete]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output diagnostics as JSON. |
| `--tier <n>` | Override the tier enforcement level for all specs (1, 2, or 3). |
| `--strict` | Treat warnings as errors. Also configurable via `settings.strict` in `specter.yaml`. |
| `--test`, `-t` | Cross-reference test-file `@spec` / `@ac` annotations against parsed specs. |
| `--concrete` | Report acceptance criteria carrying neither `inputs` nor `expected_output`. Opt-in: both fields are optional in the schema, so a criterion without them is valid and the rule does not run unless asked. `--strict` does not enable it. |

**Diagnostics:**

| Diagnostic | Severity by tier | Description |
|------------|-----------------|-------------|
| `orphan_constraint` | T1=error, T2=warning, T3=info | A constraint is not referenced by any acceptance criterion. Individual constraints may override severity via `constraint.enforcement`. |
| `structural_conflict` | info, always | An upstream constraint requires something a downstream AC handles as absent. **Advisory since v0.15**: it never fails a build, `--strict` does not raise it, and `constraint.enforcement` does not override it. Detection binds the absence expression to the required subject rather than testing whether both appear in the same sentence. It still cannot separate a contradiction from a criterion testing the constraint being enforced, because those differ only in the outcome verb, which is why it is advisory. |
| `draft_spec` | warning | A spec has `status: draft`. Emitted only when `settings.warn_on_draft` is set. |
| `malformed_ac_id` | error (under `--test`) | A test annotation names an AC id that does not match `^AC-\d{2,}$`, for example `AC-1` or `ac-01`. Emitted only under `--test`. |
| `duplicate_ac_id` | error, all tiers | Two or more acceptance criteria in one spec share an id. Names the id, the count, and the positions. Severity does not vary by tier: a repeated id makes coverage overstate itself, because one annotation satisfies every entry sharing the id. |
| `vague_criterion` | T1=error, T2=warning, T3=info | An acceptance criterion carries neither `inputs` nor `expected_output`, so nothing states what it asserts. Emitted only under `--concrete`. |
| `tier_conflict` | warning | A higher-tier spec depends on a lower-tier spec (e.g., Tier 1 depends on Tier 3). |
| `unknown_spec_ref` | error (under `--test`) | A test annotates `@spec <id>` but no spec with that ID was parsed. Emitted only under `--test`. |
| `unknown_ac_ref` | error (under `--test`) | A test annotates `@ac AC-NN` but the spec has no AC with that ID. Emitted only under `--test`. |
| `unreachable_annotation` | by `settings.strictness`: annotation→suppressed, threshold→warning, zero-tolerance→error | Source-comment `@ac` whose enclosing test produces no runner-visible `<spec-id>/AC-NN` token (Convention A) and no runtime print (Convention B). Such annotations would silently demote under `coverage --strict`. Per-file off-switch: `// @reachable manual` (`# @reachable manual` for Python). Added in v0.13.0. |
| `unreachable_annotation_unknown` | warning (regardless of strictness) | The reachability scanner could not recognize the test shape (custom helper, non-Go/TS/Python language, dynamically-generated tests). Soft form of the above; never fails a gate. Same off-switch suppresses it. Added in v0.13.0. |

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
specter coverage [--json] [--failing] [--strict] [--scope <domain>] [--tests <glob>] [--strictness <level>] [--quiet]
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--json` | none | Output the coverage report as JSON. |
| `--failing` | none | Show only specs below 100% coverage in the table. Summary header still reflects the full report. When all specs are at 100%, emits a single-line confirmation instead of an empty table. Added in v0.9.2. |
| `--strict` | none | Require `.specter-results.json` and treat any annotated AC whose status is not `passed` as uncovered, across **all tiers**. Missing file is a hard failure; empty file emits a warning and proceeds. Pairs with `specter ingest`. Added in v0.10. |
| `--scope <domain>` | none | Narrow `--strict`'s demand to ACs of specs in the named `specter.yaml` domain. Specs outside the domain fall back to v0.9 boolean-passed logic. Enables staged adoption. Requires `--strict`; unknown domain fails fast. Added in v0.10. |
| `--strictness <level>` | manifest setting | Override `settings.strictness`. Values: `annotation`, `threshold`, `zero-tolerance`. `threshold` and `zero-tolerance` route through the same strict path as `--strict`: `.specter-results.json` is required (the error names the active mode and offers both remedies) and non-passed annotated ACs demote across all tiers. Because the manifest default is `threshold`, plain `specter coverage` behaves strictly unless `settings.strictness: annotation` is set. `annotation` keeps structural annotation counting. `--strict` enables the same strict path and is equivalent to `--strictness threshold` under the default manifest strictness; it does not override a manifest-set level (a `zero-tolerance` manifest keeps its zero-tolerance gates under `--strict`, and `--strict` with `strictness: annotation` is an error). |
| `--tests <glob>` | auto-discover | Glob pattern for test files. Default discovers `*.test.ts`, `*.test.js`, `*.test.py`, `*_test.go`, `*_test.py`. |
| `--quiet` | none | Suppress per-AC source-only hints under `--strict`. JSON output still includes `diagnostic_hints`. |

**Annotation format:**

Specter reads annotations from two places.

1. **Source comments** above the test function: `// @spec <id>` and `// @ac AC-NN`. `specter coverage --strictness annotation` counts these structurally.
2. **Test title or runtime log** carrying `<spec-id>/AC-NN`. `specter ingest` reads this. The strict path (`--strict`, or the default `threshold`/`zero-tolerance` strictness) requires it.

Source comments alone: `--strictness annotation` counts it; the strict path demotes it, and that includes plain `specter coverage` under the manifest default `threshold`. Write both forms.

For the full rules (regex contract, zero-padding, one-AC-per-test, per-runner examples, parameterized tests, Python limitation, migration recipe, troubleshooting), see [`TEST_ANNOTATION_REFERENCE.md`](TEST_ANNOTATION_REFERENCE.md).

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

**Alternate form, runtime log.** When you can't rename titles (shared naming, snapshot tests, external contracts), emit the pair from inside the test body:

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
- Spec IDs longer than 40 characters are **truncated** in the table with a trailing ellipsis (`…`). This keeps column alignment on workspaces with long path-derived IDs. The `--json` output is unaffected. It emits the full spec_id.

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

Spec Coverage Report — 2 specs · 100% avg coverage
  Tier 1: 1/1 passing (100%)
  Tier 2: 1/1 passing (100%)

All 2 specs at 100% coverage.
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
- `0`: all specs parsed AND all meet their coverage thresholds
- `1`: one or more specs failed to parse, OR one or more specs are below threshold, OR `.specter-results.json` is missing under a strict mode (`--strict`, or effective strictness `threshold`/`zero-tolerance`)
- `2`: under `zero-tolerance` strictness, an annotated AC has a results-file status other than `passed`. **Under a declared `settings.annotation` block, code 2 means something narrower**: an acceptance criterion has no test at all. The two triggers do not overlap, because a criterion with no test has no results-file status to be wrong
- `3`: an AC carries `approval_gate: true` with an unset `approval_date`

**Codes 1 and 2 answer different questions under `settings.annotation`.** Code 2
is the annotation rule: a criterion has no test, and the tier threshold does not
excuse it. Code 1 is the pass rate: among criteria that do have tests, the share
passing is below the tier threshold. The strictness ladder had one code for
both. `docs/ssrb/SSRB-104.md` section 7.5 records the split.

Criteria with no test are listed on a `no test:` line, distinct from the
`uncovered:` line, and appear in `coverage --json` as `no_test_acs`. A criterion
can be uncovered without being on the `no test:` line: it has an annotation that
produced no passing result, which is a rate failure rather than a missing test.

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
specter sync [--json] [--tests <glob>] [--only <phase>] [--strict] [--strictness <level>]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--json` | Output the pipeline result as JSON. |
| `--tests <glob>` | Glob pattern for test files. |
| `--only <phase>` | Run only one phase: `parse`, `resolve`, `check`, or `coverage`. Prerequisites run without halting on failure. |
| `--strict` | Treat warnings as errors. Alias for `--strictness zero-tolerance` when `--strictness` is not set. |
| `--strictness <level>` | Override `settings.strictness` for the coverage phase. Values: `annotation`, `threshold`, `zero-tolerance`. Matches `coverage --strictness` semantics exactly. Sync's coverage phase delegates to the strict path so demotions match. When both `--strict` and `--strictness` are passed, `--strictness` wins. |

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

**Exit codes:** `0` = all phases pass. `1` = any phase fails. Under an effective `zero-tolerance` strictness (the `--strictness` flag, the `--strict` alias, or the manifest setting), the coverage phase refines the failure exit to match `coverage`: `2` = an annotated AC did not pass, `3` = `approval_gate: true` with unset `approval_date`.

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
| `--dry-run` | false | Preview generated YAML to stdout without writing files. This is the only flag that stops files being written. |
| `--overwrite` | false | Overwrite existing spec files. Default skips files that already exist. |
| `--exclude <pattern>` | none | Exclude paths matching pattern. Can be repeated. |
| `--json` | false | Report the run as JSON on stdout. Selects the output format only: files are still written to `--output`. Combine with `--dry-run` for a report without writes. Under `--json`, stdout carries exactly one JSON document and the per-spec `GENERATED` and `SKIPPED` lines go to stderr. |

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
specter init --install-hook
specter init --ai <tool>
```

**Options:**

| Option | Description |
|--------|-------------|
| `--name <name>` | System name for the manifest. Defaults to the current directory name. |
| `--force` | Overwrite an existing `specter.yaml`. Mutually exclusive with `--refresh`. |
| `--template <type>` | Create a draft `.spec.yaml` from a template instead of a manifest. Types: `api-endpoint`, `service`, `auth`, `data-model`. |
| `--refresh` | Update only `domains.default.specs` in an existing `specter.yaml`. Preserves every other field: `settings`, `registry`, tier overrides, custom domains. Added in v0.9.2. |
| `--dry-run` | Used with `--refresh`: print the proposed diff to stdout without writing the file. Added in v0.9.2. |
| `--install-hook` | Install a git pre-push hook that blocks implementation-only pushes with no `@spec` / `@ac` annotation delta. |
| `--ai <tool>` | Write an AI assistant instruction file. Values: `claude`, `codex`, `cursor`, `copilot`, `gemini`. |

**Behavior (v0.9.0+):**

`specter init` scans the workspace's `specs/` directory and populates the manifest's default domain based on what it finds.

- **Greenfield workspace (no spec files):** emits a manifest with an empty `domains.default` entry whose description invites you to add spec IDs as you author them.
- **Workspace with parseable specs:** reads each one, extracts its `spec.id`, and populates `domains.default.specs: [...]`.
- **Workspace with specs that fail to parse:** still writes the manifest (with an explanatory placeholder default domain) and prints a warning that includes a **Pattern analysis** block naming the shape of the failure. If every discovered spec hit the same error, init calls out schema version drift and points at `specter doctor` for deeper diagnosis.

**Important (v0.9.0+):** init always emits a `domains:` section, even in the greenfield case. Previous versions omitted `domains:` entirely when no spec IDs were discovered, which caused later `specter sync` runs to silently skip every spec the user added afterward, a silent-exclusion footgun now eliminated.

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

`specter init --refresh` is the non-destructive counterpart to `--force`. It reads the existing `specter.yaml`, rescans `settings.specs_dir` (or default `specs/`), and updates **only** `domains.default.specs` with the current on-disk spec set. Every other field is preserved: `settings`, `registry`, system metadata, and any custom domains declared under `domains.<name>` (anything that isn't `default`).

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
specter doctor [--fix] [--dry-run] [--yes]
```

**Options:**

| Option | Description |
|--------|-------------|
| `--fix` | Apply known-safe schema-drift rewrites in place. Currently beta-gated. |
| `--dry-run` | With `--fix`, print what would change without writing files. Skips the beta prompt. |
| `--yes`, `-y` | Skip the `--fix` beta confirmation prompt for non-interactive use. |

**Checks performed:**

| Check | PASS | WARN | FAIL |
|-------|------|------|------|
| `manifest` | `specter.yaml` found | No `specter.yaml` (optional) | n/a |
| `spec-files` | ≥1 `.spec.yaml` found | n/a | No spec files found |
| `parse` | All specs parse cleanly | n/a | Parse errors in ≥1 spec |
| `annotations` | `@spec`/`@ac` annotations found in tests | No annotations found | n/a |
| `coverage` | All specs meet tier thresholds | n/a | ≥1 spec below threshold |

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

When the parse check fails, `specter doctor` prints a **Pattern analysis** block that groups errors by `(type, path)`. If every discovered spec hit the same pattern, doctor names it explicitly as the signature of schema version drift, a common shape for projects whose specs predate the current schema.

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

Polymorphic diff verb. The single command for diffing any Specter artifact. Dispatches on an optional first `<kind>` argument; defaults to the `spec` kind for backward compat with v1.x.

**Synopsis:**

```
specter diff <path>[@<ref>] <path>[@<ref>]              # spec kind (implicit; backward compat)
specter diff spec <path>[@<ref>] <path>[@<ref>]         # spec kind (explicit)
specter diff coverage <baseline.json> <current.json>    # coverage kind
```

**Kinds:**

| Kind | Purpose |
|------|---------|
| `spec` | Semantic diff between two spec versions (v1.x behavior; default when no kind argument is present). Classifies as `breaking`, `additive`, `patch`, or `unchanged`. |
| `coverage` | Per-spec AC delta between two `coverage --json` snapshots. Useful for tracking coverage drift across CI runs. |

Future cycles add more kinds (e.g., `ingest`, `check`) under the same `specter diff <kind>` grammar. New diffable artifacts MUST NOT introduce a per-subcommand `--diff` flag. They land as kinds here.

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--exit-code` | false | Exit with code 10 when the change is breaking. Without it `diff` always exits 0, because it is a diagnostic surface rather than a gate. Code 10 is in the orchestration band; see [EXIT_CODES.md](EXIT_CODES.md). |

**spec kind, change classes:**

| Class | Meaning |
|-------|---------|
| `breaking` | An AC or constraint removed, or a criterion's contract changed: its `inputs`, `expected_output`, `error_cases`, `references_constraints`, `priority` or `approval_gate`. Requires a MAJOR version bump. |
| `additive` | New ACs or constraints added. Requires a MINOR version bump. |
| `patch` | Wording-only changes that don't alter meaning. PATCH version bump. |
| `unchanged` | No changes detected. |

**A criterion changes in two ways and the report says which.** When the
description changed, the line shows the transition. When only the contract
changed, the description is identical on both sides, so the line names the
fields that differ instead:

```
  ~AC-01: expected_output changed (description unchanged)
  ~AC-02: old wording → new wording (also priority, approval_gate)
```

**Example, spec kind:**

```
$ specter diff specs/auth.spec.yaml@HEAD~3 specs/auth.spec.yaml

spec spec-auth 1.0.0 → 1.1.0 [additive]

  +AC-05: Returns 401 when token is expired
  ~C-02: MUST require 8-character passwords → MUST require 12-character passwords

$ specter diff specs/auth.spec.yaml specs/auth.spec.yaml
spec spec-auth 1.1.0 → 1.1.0: no changes
```

**Example, coverage kind:**

```
$ specter coverage --json > baseline.json   # later, after changes:
$ specter coverage --json > current.json
$ specter diff coverage baseline.json current.json

+spec spec-new-feature
+spec-auth/AC-05
-spec-auth/AC-03
~spec-auth coverage_pct: 80.0 → 90.0 (passes_threshold: true → true)
```

Exit code is always 0 for both kinds. Diff is a diagnostic surface, not a gate.

---

### `specter ingest`

Convert CI-native test output (JUnit XML, `go test -json`) into `.specter-results.json`, the file `specter coverage --strict` reads to demote annotated-but-failing ACs. Added in v0.10.

**Synopsis:**

```
specter ingest [--junit <path>] [--go-test <path>] [--output <path>] [--verbose]
```

At least one of `--junit` or `--go-test` is required. Both flags accept glob patterns and may be repeated. Multiple sources can be combined in one invocation; results are merged by the worst-status-wins rule.

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--junit <path>` | none | JUnit XML file (vitest, jest, pytest, playwright). Accepts glob patterns; may be repeated. |
| `--go-test <path>` | none | Newline-delimited JSON from `go test -json`. Accepts glob patterns; may be repeated. |
| `--output <path>` | `.specter-results.json` | Where to write the merged results. |
| `--verbose` | none | Emit one stderr line per dropped testcase (testcases without a recognizable `(spec_id, ac_id)` annotation). Off by default; the summary line is always emitted. |

**Diagnostics:** every run writes to stderr a summary line:

```
Scanned N test cases; extracted M (spec_id, ac_id) pairs; dropped K with no runner-visible annotation.
```

If `M` is 0 despite `N` being non-zero, your tests carry annotations only in source comments, and those are invisible to `ingest` by design. See the explainer's Conventions A (test title) and B (runtime `t.Log`) for migrating.

**Annotation extraction:**

Each test needs a discoverable `(spec_id, ac_id)` pair or it's dropped silently. Sources in order of preference:

1. **Test name.** `spec-id/AC-NN` or `spec-id:AC-NN` embedded in the test case name.
2. **Classname.** same pattern, parsed from the JUnit `classname` attribute.
3. **Test body.** `// @spec <id>` and `// @ac <AC-id>` comments surfaced via `system-out` (JUnit) or `output`-action lines (go test -json).

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
Spec Coverage Report — 15 specs · 99% avg coverage
  Tier 1: 4/4 passing (100%)
  Tier 2: 9/9 passing (100%)
  Tier 3: 2/2 passing (100%)
...
```

Pairs with `specter coverage --strict`. Without `ingest`, `--strict` fails with `--strict requires .specter-results.json — run 'specter ingest' first`.

---

## The `specter.yaml` Manifest

An optional `specter.yaml` file at the project root configures discovery, thresholds, and settings. Specter searches upward from the current directory to find it.

```yaml
schema_version: 1
system:
  name: my-project

domains:
  default:
    description: Default domain for my-project specs
    specs:
      - user-create

settings:
  specs_dir: specs
  strict: false
  warn_on_draft: false
  strictness: threshold
  annotation:
    permissive: false
  tests_glob:
    - "**/*_test.go"
    - "**/*.test.ts"
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
  exclude:
    - node_modules
    - dist
    - .git
    - vendor
```

### `settings.annotation`

Added in v0.15.0. A block carrying one sub-key.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `permissive` | bool | `false` | Warns where the same configuration would otherwise fail |

Declaring the block is what matters, not its contents. `annotation: {}` and a
bare `annotation:` both count as declared and both set `permissive` to `false`.
A manifest carrying no `annotation` key behaves exactly as it did in v0.14.

**`settings.annotation` and `settings.strictness` cannot both govern.** When a
manifest declares both, the `annotation` block takes precedence and `check`,
`coverage` and `sync` each write a warning to stderr naming the ignored
`settings.strictness`. The exit code is not changed by the warning.

**`settings.annotation.scope` is not accepted.** `docs/ssrb/SSRB-104.md` names
`scope: test | all` in its target shape, and only test scope is implemented, so
the key is rejected with a message that says so rather than reading as a typo.
Annotation scope is test-only.

### Tier: what governs, and what is deprecated

**The tier declared in the `.spec.yaml` file governs.** It sets the coverage
threshold the spec is held to, the severity of its orphan constraints, and the
tier reported by `coverage --json`. Nothing resolves or overrides it.

| Field | State |
|---|---|
| `spec.tier` | **Authoritative.** Required, one of 1, 2, 3 |
| `domains.<name>.tier` | A **checked assertion**. It declares the risk level the domain asserts, and a spec listed in that domain whose tier disagrees produces a `domain_tier_conflict` warning. It does not change the spec's tier |
| `system.tier` | **Deprecated.** Warns on every run, has no effect, removed at v1.0.0 |
| `settings.tier_overrides` | **Deprecated.** Warns on every run, has no effect, removed at v1.0.0. The `tier_conflict` warning it produces now states that the declared tier governs |
| `registry` | **Retired.** The key is still accepted so an existing manifest parses, and its value is discarded. Removed at v1.0.0 |

A manifest declaring a deprecated key gets one stderr line per key. The warning
changes no exit code.

Both `tier_conflict` and `domain_tier_conflict` appear in `specter check --json`
as well as in the text output, with the same warning count in both.

**What changed in v0.15.0.** `system.tier`, `domains.<name>.tier` and
`settings.tier_overrides` were previously validated, range-checked, and read by
nothing, so an operator who set one got silence. `settings.tier_overrides` was
worse than silent: its warning ended `using override (N)` while nothing applied
the override. Decisions in `docs/ssrb/SSRB-105.md` and `docs/ssrb/SSRB-106.md`.

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

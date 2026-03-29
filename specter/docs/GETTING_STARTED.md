# Getting Started with Specter

## What is Specter?

Specter is a **type system for specs**. It validates, links, and type-checks `.spec.yaml` files the same way `tsc` validates `.ts` files. If you write specifications to drive AI-assisted development, Specter ensures those specifications are structurally sound, internally consistent, and correctly connected to each other.

The problem Specter solves is simple: specs drift. A developer writes a constraint that no acceptance criterion references. A spec declares a dependency on another spec that does not exist. A version bump introduces a breaking change that downstream specs never account for. These are the same categories of bugs that type systems catch in code -- missing fields, broken references, incompatible interfaces -- but until now, no tool applied that rigor to specifications.

Specter's core philosophy: **"Discipline can drift. Infrastructure cannot."** Rather than relying on reviewers to catch structural problems in specs, Specter catches them automatically, in CI, every time.

## Prerequisites

- **Go 1.22 or later** -- Specter is built in Go and distributed as a single binary.

Verify your setup:

```bash
go version   # Should print go1.22.x or higher
```

## Installation

### From source (current method)

Clone the repository and build:

```bash
git clone <repository-url>
cd specter
make build
```

This produces a binary at `bin/specter`. You can also build directly with Go:

```bash
go build -o bin/specter ./cmd/specter/
```

After building, you can run Specter directly:

```bash
bin/specter parse specs/
```

Or install it to your `$GOPATH/bin` for global access:

```bash
go install github.com/Hanalyx/specter/cmd/specter@latest
specter parse specs/
```

### Verify installation

```bash
specter --version
# 0.1.0

specter --help
# Usage: specter [options] [command]
#
# A type system for specs. Validates, links, and type-checks .spec.yaml files.
```

## Your First Spec

Specter specs are YAML files with the `.spec.yaml` extension. Each spec describes a single feature, endpoint, or component using a structured format called a **Micro-Spec** -- a specification built on three pillars: Context (where does this live?), Objective (what should change?), and Constraints (what are the hard boundaries?).

Let's write a spec for a user registration endpoint. Create a file called `user-registration.spec.yaml`:

```yaml
spec:
  id: user-registration
  version: "1.0.0"
  status: draft
  tier: 1

  context:
    system: User service
    feature: Registration endpoint
    description: >
      REST endpoint that creates a new user account. Accepts email and
      password, validates inputs, hashes the password, stores the record,
      and returns a JWT.
    assumptions:
      - "PostgreSQL is the backing store"
      - "Passwords are hashed with bcrypt (cost factor 12)"

  objective:
    summary: >
      Create a POST /users/register endpoint that accepts email and password,
      validates both, creates the user record, and returns a signed JWT.
    scope:
      includes:
        - "Input validation (email format, password strength)"
        - "Password hashing"
        - "User record creation"
        - "JWT generation and return"
      excludes:
        - "Email verification flow"
        - "Rate limiting (handled by API gateway)"
        - "OAuth/social login"

  constraints:
    - id: C-01
      description: "MUST validate email format per RFC 5322"
      type: technical
      enforcement: error

    - id: C-02
      description: "MUST require passwords of at least 12 characters"
      type: security
      enforcement: error

    - id: C-03
      description: "MUST hash passwords with bcrypt before storage"
      type: security
      enforcement: error

    - id: C-04
      description: "MUST return 409 Conflict if email already exists"
      type: business
      enforcement: error

    - id: C-05
      description: "MUST NOT include the password hash in any response body"
      type: security
      enforcement: error

  acceptance_criteria:
    - id: AC-01
      description: "Valid email and strong password creates user and returns 201 with JWT"
      inputs:
        email: "alice@example.com"
        password: "correct-horse-battery"
      expected_output:
        status: 201
        body_contains: "token"
      references_constraints: ["C-01", "C-02", "C-03"]
      priority: critical

    - id: AC-02
      description: "Invalid email format returns 400 with field-level error"
      inputs:
        email: "not-an-email"
        password: "correct-horse-battery"
      expected_output:
        status: 400
        error_field: "email"
      references_constraints: ["C-01"]
      priority: high

    - id: AC-03
      description: "Weak password returns 400 with password policy error"
      inputs:
        email: "alice@example.com"
        password: "short"
      expected_output:
        status: 400
        error_field: "password"
      references_constraints: ["C-02"]
      priority: high

    - id: AC-04
      description: "Duplicate email returns 409 Conflict"
      inputs:
        email: "existing@example.com"
        password: "correct-horse-battery"
      expected_output:
        status: 409
      references_constraints: ["C-04"]
      priority: high

    - id: AC-05
      description: "Response body never contains password hash"
      inputs:
        email: "alice@example.com"
        password: "correct-horse-battery"
      expected_output:
        body_must_not_contain: "password_hash"
      references_constraints: ["C-05"]
      priority: critical
```

Let's walk through the key sections.

### `id`

A unique, kebab-case identifier for this spec. Other specs reference it by this ID in their `depends_on` fields, and test files link to it via `@spec user-registration` annotations.

### `version`

Semantic version string (`MAJOR.MINOR.PATCH`). Specter uses this for dependency resolution and breaking change detection. Must be quoted in YAML to avoid interpretation as a number.

### `status`

Lifecycle stage: `draft`, `review`, `approved`, `deprecated`, or `removed`. Only `approved` specs are enforced by CI tooling.

### `tier`

Risk tier from 1 to 3. This controls how strictly Specter enforces rules:

| Tier | Category | Coverage threshold | Orphan severity |
|------|----------|-------------------|-----------------|
| 1 | Security / Money | 100% | error |
| 2 | Core Business Logic | 80% | warning |
| 3 | Utility / Internal | 50% | info |

### `context`

Where this spec lives. The `system` field is required. Everything else -- `feature`, `description`, `dependencies`, `assumptions` -- is optional but recommended.

### `objective`

What should change. The `summary` is required. The `scope` block with `includes` and `excludes` lists prevents scope creep -- especially important when AI agents are generating implementation code.

### `constraints`

Hard boundaries on the solution. Each constraint has:
- An `id` in the format `C-01`, `C-02`, etc.
- A `description` using RFC 2119 language (MUST, MUST NOT, SHOULD, MAY)
- An optional `type` (technical, security, performance, accessibility, business)
- An optional `enforcement` level (error, warning, info)

### `acceptance_criteria`

Testable conditions that define "done." Each AC has:
- An `id` in the format `AC-01`, `AC-02`, etc.
- A `description` of expected behavior
- Optional `inputs`, `expected_output`, and `error_cases`
- A `references_constraints` array linking back to the constraints it validates

This linkage is how Specter detects **orphan constraints** -- constraints that no AC covers.

## Validating Specs

Run `specter parse` to validate your spec against the canonical schema:

```bash
specter parse user-registration.spec.yaml
```

### Successful output

```
PASS user-registration.spec.yaml — user-registration@1.0.0
```

### Parsing all specs in a directory

Run `specter parse` with no arguments to validate every `.spec.yaml` file found recursively:

```bash
specter parse
```

```
PASS specs/user-registration.spec.yaml — user-registration@1.0.0
PASS specs/payment-create-intent.spec.yaml — payment-create-intent@1.0.0
PASS specs/auth-jwt-validation.spec.yaml — auth-jwt-validation@2.1.0
```

### Failed output

If a spec has problems, Specter prints each error with its type, field path, and (when possible) a line number:

```
FAIL specs/user-registration.spec.yaml
  error [required] spec/id: must have required property 'id'
  error [pattern] spec/version: must match pattern "^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9.]+)?$"
```

### JSON output

Use `--json` for machine-readable output (useful in CI pipelines):

```bash
specter parse --json user-registration.spec.yaml
```

## Understanding Errors

When validation fails, each error includes a `type` that tells you what went wrong.

| Error type | Meaning | Example cause |
|-----------|---------|---------------|
| `required` | A required field is missing | Omitting `id`, `version`, `constraints`, or any other required field |
| `pattern` | A string value does not match the expected format | Version `"v1.0"` instead of `"1.0.0"`, constraint ID `"c1"` instead of `"C-01"` |
| `additionalProperties` | The spec contains a field not defined in the schema | Typo like `contstraints` or adding an invented field like `priority` at the top level |
| `yaml_syntax` | The YAML itself is malformed | Bad indentation, missing colon, unclosed quotes |
| `enum` | A value is not one of the allowed options | Status `"active"` instead of `"approved"`, tier `4` instead of `1`, `2`, or `3` |
| `type` | A value has the wrong data type | Tier as a string `"1"` instead of integer `1`, constraints as an object instead of an array |
| `minItems` | An array has fewer items than required | Empty `constraints: []` or `acceptance_criteria: []` (at least one of each is required) |

### Common mistakes and fixes

**Version must be quoted:**

```yaml
# Wrong -- YAML interprets 1.0.0 as a string "1.0.0" in some cases,
# but 1.0 becomes the float 1.0, which fails the pattern check.
version: 1.0.0

# Correct
version: "1.0.0"
```

**Constraint IDs must match `C-XX` format:**

```yaml
# Wrong
- id: c1
- id: C1
- id: constraint-01

# Correct
- id: C-01
```

**AC IDs must match `AC-XX` format:**

```yaml
# Wrong
- id: ac1
- id: AC1

# Correct
- id: AC-01
```

**Spec ID must be lowercase kebab-case:**

```yaml
# Wrong
id: UserRegistration
id: user_registration
id: 1-user-reg

# Correct
id: user-registration
```

## Spec Schema Reference

### Required fields

| Field | Type | Description |
|-------|------|-------------|
| `spec.id` | string | Unique kebab-case identifier (`^[a-z][a-z0-9-]*$`) |
| `spec.version` | string | Semantic version (`MAJOR.MINOR.PATCH`, optional pre-release tag) |
| `spec.status` | string | One of: `draft`, `review`, `approved`, `deprecated`, `removed` |
| `spec.tier` | integer | Risk tier: `1` (Security/Money), `2` (Core Business), `3` (Utility) |
| `spec.context` | object | Must contain at least `system` (string) |
| `spec.objective` | object | Must contain at least `summary` (string) |
| `spec.constraints` | array | At least one constraint. Each needs `id` (C-XX) and `description` |
| `spec.acceptance_criteria` | array | At least one AC. Each needs `id` (AC-XX) and `description` |

### Optional fields

| Field | Type | Description |
|-------|------|-------------|
| `spec.depends_on` | array | References to other specs this one depends on |
| `spec.trust_level` | string | AI autonomy: `full_auto`, `auto_with_review`, `human_required` |
| `spec.environment` | object | `required_vars` and `deployment_targets` |
| `spec.tags` | array | Free-form strings for categorization |
| `spec.changelog` | array | Version history entries |
| `spec.generated_from` | object | Provenance for reverse-compiled specs |

### Constraint fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Format: `C-01`, `C-02`, ... (`^C-\d{2,}$`) |
| `description` | Yes | string | Use RFC 2119 language (MUST, SHOULD, MAY) |
| `type` | No | string | `technical`, `security`, `performance`, `accessibility`, `business` |
| `enforcement` | No | string | `error` (default), `warning`, `info` |
| `validation` | No | object | Machine-readable rule with `field`, `rule`, and `value` |

### Acceptance criterion fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | Yes | string | Format: `AC-01`, `AC-02`, ... (`^AC-\d{2,}$`) |
| `description` | Yes | string | Human-readable expected behavior |
| `inputs` | No | object | Input values or conditions |
| `expected_output` | No | object | Expected results |
| `error_cases` | No | array | Error conditions and expected handling |
| `references_constraints` | No | array | Constraint IDs this AC validates (e.g., `["C-01", "C-03"]`) |
| `gap` | No | boolean | `true` if identified by gap analysis |
| `priority` | No | string | `critical`, `high`, `medium`, `low` |

## A Minimal Valid Spec

Not every spec needs the full structure above. Here is the smallest spec that passes validation:

```yaml
spec:
  id: my-feature
  version: "1.0.0"
  status: draft
  tier: 3

  context:
    system: My service

  objective:
    summary: A short description of what this feature does.

  constraints:
    - id: C-01
      description: "MUST do the thing correctly"

  acceptance_criteria:
    - id: AC-01
      description: "The thing works as expected"
```

This is enough for `specter parse` to pass. As your spec matures, add scope boundaries, constraint types, AC inputs/outputs, and `references_constraints` linkages.

## Project Structure Conventions

### Where to put specs

Keep all spec files in a `specs/` directory at the root of your project:

```
my-project/
  specs/
    user-registration.spec.yaml
    payment-create-intent.spec.yaml
    auth-jwt-validation.spec.yaml
  src/
    ...
  tests/
    ...
```

### Naming conventions

- **Files:** `{feature-name}.spec.yaml` (kebab-case, always ends in `.spec.yaml`)
- **Spec IDs:** Match the filename without the extension (e.g., file `user-registration.spec.yaml` has `id: user-registration`)
- **Constraint IDs:** `C-01`, `C-02`, ... sequentially within each spec
- **AC IDs:** `AC-01`, `AC-02`, ... sequentially within each spec

### Linking tests to specs

Specter's coverage tool (coming in a future release) scans test files for annotations that trace back to specs:

```typescript
// @spec user-registration
// @ac AC-01
test('valid registration returns 201 with JWT', async () => {
  // ...
});

// @spec user-registration
// @ac AC-04
test('duplicate email returns 409', async () => {
  // ...
});
```

Python tests use the same pattern with `#` comments:

```python
# @spec user-registration
# @ac AC-02
def test_invalid_email_returns_400():
    ...
```

### Organizing larger projects

For projects with many specs, group them into subdirectories by domain:

```
specs/
  auth/
    jwt-validation.spec.yaml
    oauth-flow.spec.yaml
  payments/
    create-intent.spec.yaml
    refund-process.spec.yaml
  users/
    registration.spec.yaml
    profile-update.spec.yaml
```

Specter discovers `.spec.yaml` files recursively, so subdirectories work without any configuration.

## Next Steps

- **[Spec Schema Reference](../internal/parser/spec-schema.json)** -- The canonical JSON Schema that defines every field, type, and constraint. This is the source of truth.
- **[Specter's own specs](../specs/)** -- Specter dogfoods its own format. Read `spec-parse.spec.yaml` for a real-world example of a production spec.
- **CLI Commands** -- Beyond `parse`, Specter includes `resolve` (dependency graph), `check` (type-checking), and `coverage` (traceability matrix). Run `specter --help` for the full list.

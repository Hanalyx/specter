# Getting Started with Specter

This guide takes you from **zero specs to full coverage** — step by step, with every command, every AI prompt, and a complete VS Code workspace walkthrough.

If you just want to see it work in 5 minutes, start with the [QuickStart](QUICKSTART.md) instead.

---

## Before You Begin

### What Specter is

Specter is a **type system for specs**. It validates, links, and checks `.spec.yaml` files the same way `tsc` validates `.ts` files. The core idea: your specification is the source of truth — not the code, not the tests, and not the AI output. Specter enforces that.

### What Specter is not

Specter is **not** a general-purpose spec format reader. If you already use specs — Gherkin/Cucumber `.feature` files, OpenAPI `.yaml`, Notion docs, Confluence pages — Specter does not read those formats. It uses its own structured schema (see [Schema Reference](SPEC_SCHEMA_REFERENCE.md)).

**If you have existing specs in another format**, you have two options:

| Option | When to use |
|--------|-------------|
| **Run `specter reverse`** on your source code, then discard your old specs | Your existing specs are high-level or informal — the code is the real source of truth |
| **Migrate your existing specs** into Specter's schema manually or with AI | Your existing specs are detailed and authoritative — you want to preserve them |

For migration, use this AI prompt:

```
I have specifications in [Gherkin / OpenAPI / plain text / other format].
I want to migrate them to Specter's .spec.yaml format.

Specter's schema requires these top-level fields:
  spec.id — lowercase letters, digits, hyphens only, must start with a letter
            (e.g. "user-create", "payment-process"). No uppercase, no underscores.
  spec.version — quoted semver string: "1.0.0" (must be quoted or YAML parsing fails)
  spec.status — one of: draft | review | approved | deprecated | removed
  spec.tier — integer only, no quotes: 1 (Security/Money), 2 (Core Business), 3 (Utility)
  spec.context.system — string (name of the system or service)
  spec.objective.summary — string inside an objective object, not a bare string
  spec.constraints — array of objects. Required fields per item: id (format: C-01, C-02...),
                     description. Optional fields: type (technical | security | performance |
                     accessibility | business), enforcement (error | warning | info).
                     No other fields are allowed.
  spec.acceptance_criteria — array of objects. Required fields per item: id (format: AC-01,
                     AC-02...), description. Optional fields: references_constraints (array
                     of C-XX strings), inputs, expected_output, priority (critical | high |
                     medium | low). No other fields are allowed.

Here is my existing spec:
[paste your spec]

Please convert it to Specter's .spec.yaml format. Keep all the intent,
rewrite constraints to use MUST/MUST NOT/SHOULD language (RFC 2119),
and break acceptance criteria into individually testable AC-XX items.
```

---

## Phase 1 — Install

### CLI

Release assets follow the goreleaser convention `specter_<version>_<os>_<arch>.<ext>` — lowercase `linux`/`darwin`/`windows`, Go's `amd64`/`arm64` (not `uname`'s `x86_64`/`aarch64`). Each snippet below translates and picks the latest version automatically.

**macOS / Linux (tar.gz):**
```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_${OS}_${ARCH}.tar.gz"
tar xzf "specter_${VERSION}_${OS}_${ARCH}.tar.gz" && sudo mv specter /usr/local/bin/
specter --version
```

**DEB package (Ubuntu/Debian):**
```bash
ARCH=$(dpkg --print-architecture)
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_linux_${ARCH}.deb"
sudo dpkg -i "specter_${VERSION}_linux_${ARCH}.deb"
```

**RPM package (Fedora/RHEL):**
```bash
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
VERSION=$(curl -sL https://api.github.com/repos/Hanalyx/specter/releases/latest | grep '"tag_name"' | head -n1 | cut -d'"' -f4 | sed 's/^v//')
curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_linux_${ARCH}.rpm"
sudo rpm -i "specter_${VERSION}_linux_${ARCH}.rpm"
```

**Windows (PowerShell):**
```powershell
$v = (Invoke-RestMethod https://api.github.com/repos/Hanalyx/specter/releases/latest).tag_name -replace '^v',''
Invoke-WebRequest -Uri "https://github.com/Hanalyx/specter/releases/download/v${v}/specter_${v}_windows_amd64.zip" -OutFile specter.zip
Expand-Archive specter.zip -DestinationPath "$env:USERPROFILE\.specter\bin"
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:USERPROFILE\.specter\bin", "User")
```

**Build from source (Go 1.22+):**
```bash
git clone https://github.com/Hanalyx/specter.git
cd specter/specter && make build
sudo mv bin/specter /usr/local/bin/
```

Verify (restart terminal first on Windows so `PATH` picks up):
```bash
specter --version
specter --help
```

### VS Code Extension

1. Open VS Code
2. Press `Ctrl+Shift+X` (Extensions panel)
3. Search `Specter SDD`
4. Click **Install**

The extension activates automatically once `specter.yaml` exists in your workspace (Phase 2, Step 3).

---

## Phase 2 — Bootstrap Your Specs

### Step 1 — Run the reverse compiler

Point Specter at your source directory. It analyzes your code and generates draft specs automatically:

```bash
specter reverse src/        # TypeScript / JavaScript / Next.js
specter reverse app/        # Python / Django / FastAPI
specter reverse ./          # Go
specter reverse packages/   # monorepo — point at the package root
```

What gets created:
```
specs/
  user-create.spec.yaml
  payment-process.spec.yaml
  auth-jwt.spec.yaml
  ...
```

Each spec reflects the structure Specter found in your code — routes, models, validation rules, constraints. Every spec will have `gap: true` at this stage. That is expected.

> **`gap: true` means:** Specter extracted the structure but could not infer the *intent*. A human (or AI) needs to complete it before it becomes authoritative.

### Step 2 — Initialize the workspace manifest

```bash
specter init
```

Creates `specter.yaml`:
```yaml
specs_dir: specs
tests_dir: .
exclude:
  - node_modules
  - .git
  - dist
  - .next
```

This file is required for:
- The VS Code extension to activate
- `specter sync` to know where to look
- The `--exclude` patterns to work

Commit this file. It belongs in source control.

### Step 3 — Check the VS Code extension activated

Open VS Code in your project folder. Look at the activity bar (left sidebar) for the **Sp** icon. Click it — you should see the **Specter: Coverage** panel listing your specs with their current coverage percentages.

If you see: `Specter: no specter.yaml found in this workspace` — run `specter init` first (Step 2).

---

## Phase 3 — Close the Gaps

This is the most important phase. You are turning AI-extracted drafts into authoritative specifications.

### Step 1 — Run `specter check` to see the landscape

```bash
specter check
```

This reports structural issues: orphaned constraints (no AC references them), broken dependencies, missing required fields. Fix every `error` before moving on. `warn` items can be addressed incrementally.

### Step 2 — Review each spec with AI

Open a generated spec. It looks like this:

```yaml
spec:
  id: user-create
  version: "1.0.0"
  status: draft
  tier: 2
  gap: true
  context:
    system: User service
    description: "Handles user account creation"
  objective:
    summary: "Create a new user account"
  constraints:
    - id: C-01
      description: "POST /users accepts email and password"
  acceptance_criteria:
    - id: AC-01
      description: ""
    - id: AC-02
      description: ""
```

**AI prompt — fill the gaps:**

```
Here is a draft spec generated by Specter's reverse compiler for my [language/framework] codebase.

[paste the spec]

Please complete this spec:
1. Fill each empty acceptance_criteria description with a specific, testable behavior
   (e.g., "Valid email + password creates user and returns 201 with JWT token")
2. Add any obviously missing constraints based on the context
3. Add `references_constraints` arrays to each AC linking back to the C-XX IDs it validates
4. Set `status: draft` (leave it — we'll promote later)
5. Remove `gap: true` once all gaps are filled
6. Keep all existing IDs (C-01, AC-01, etc.) unchanged — do not renumber
7. Use MUST/MUST NOT/SHOULD language in constraint descriptions (RFC 2119)

Return only the completed YAML.
```

**AI prompt — add missing constraints:**

```
Review this Specter spec for [feature name]. Based on the context and objective,
what constraints are likely missing?

[paste the spec]

For each gap you identify, add a new constraint with:
- Sequential ID (next after the last C-XX)
- A MUST/MUST NOT description
- type: technical | security | performance | business
- enforcement: error | warning

Also add a corresponding AC that references it.
Return the updated YAML only.
```

### Step 3 — Validate each spec

After AI edits, validate before moving on:

```bash
specter parse specs/user-create.spec.yaml
```

```
PASS specs/user-create.spec.yaml — user-create@1.0.0
```

If it fails, Specter tells you exactly what's wrong:

```
FAIL specs/user-create.spec.yaml
  error [pattern] spec/constraints/0/id: must match "^C-\d{2,}$"
```

Common fixes:
- IDs must be `C-01` not `c1`, `C1`, or `constraint-1`
- Version must be quoted: `"1.0.0"` not `1.0.0`
- `tier` must be an integer: `2` not `"2"`
- `status` must be one of: `draft`, `review`, `approved`, `deprecated`, `removed`

### Step 4 — Repeat for all specs

```bash
specter parse    # validates all specs at once
specter check    # checks structural relationships
```

When both commands exit cleanly with no errors, Phase 3 is complete.

---

## Phase 4 — Write Tests Against the Specs

Specter reads test annotations from two places. Both matter.

1. **Source comments**: `// @spec <spec-id>` and `// @ac AC-NN` above the test function. Read by `specter coverage`.
2. **Runner-visible annotation**: the `<spec-id>/AC-NN` pair in the test title, or a `console.log('// @spec ...')` inside the test body. Read by `specter ingest` into `.specter-results.json`. Required by `specter coverage --strict`.

Source comments alone: `coverage` counts it, `--strict` demotes it. Write both forms and both commands work.

### Step 1 — See what's uncovered

```bash
specter coverage
```

```
Spec Coverage Report

Spec ID        Tier   ACs   Covered   Coverage   Status
------------------------------------------------------------
user-create    T2     4     0         0%         PASS
payment        T1     6     0         0%         FAIL  ← tier 1 needs 100%
auth-jwt       T2     5     0         0%         PASS
```

### Step 2 — Get AI to write annotated tests

Use `specter explain` to get a ready-to-copy annotation example for any AC:

```bash
specter explain user-create
```

Then pass the spec to your AI assistant:

**AI prompt — write tests for a spec:**

```
Here is a Specter spec:

[paste the spec]

Write tests in [TypeScript/Jest | Python/pytest | Go testing].

Rules:
1. One test per AC.
2. The test title carries [spec-id/AC-NN]:
   TypeScript:  it('[spec-id/AC-NN] brief description', () => { ... })
   Python:      def test_spec_id_AC_NN_brief(...): ...
   Go:          t.Run("spec-id/AC-NN brief description", ...)
   AC-NN is zero-padded: AC-01, not AC-1.
3. Above each test, add:
   // @spec [spec-id]
   // @ac [AC-NN]
4. Use [your test framework/mocks].
5. Tests are runnable. Use realistic inputs from the spec's `inputs` fields.

Return only the test code.
```

**TypeScript/Jest:**
```typescript
// @spec user-create
// @ac AC-01
test('[user-create/AC-01] valid email and password creates user and returns 201 with JWT', async () => {
  const res = await request(app).post('/users').send({
    email: 'alice@example.com',
    password: 'correct-horse-battery',
  });
  expect(res.status).toBe(201);
  expect(res.body).toHaveProperty('token');
});

// @spec user-create
// @ac AC-02
test('[user-create/AC-02] invalid email format returns 400', async () => {
  const res = await request(app).post('/users').send({
    email: 'not-an-email',
    password: 'correct-horse-battery',
  });
  expect(res.status).toBe(400);
  expect(res.body.error).toContain('email');
});
```

**Python/pytest** — Python function names can't contain `/` or `[`. Encode the pair in the function name. `specter ingest` reads the function name as the test title.
```python
# @spec user-create
# @ac AC-01
def test_user_create_AC_01_valid_registration_returns_201(client):
    response = client.post('/users', json={
        'email': 'alice@example.com',
        'password': 'correct-horse-battery'
    })
    assert response.status_code == 201
    assert 'token' in response.json()

# @spec user-create
# @ac AC-02
def test_user_create_AC_02_invalid_email_returns_400(client):
    response = client.post('/users', json={
        'email': 'not-an-email',
        'password': 'correct-horse-battery'
    })
    assert response.status_code == 400
```

**Go** — use `t.Run` so each AC has its own runner-visible subtest title. `specter ingest` reads subtest names from `go test -json` output.
```go
// @spec user-create
// @ac AC-01
// @ac AC-02
func TestCreateUser(t *testing.T) {
    t.Run("user-create/AC-01 valid credentials returns 201 with JWT", func(t *testing.T) {
        body := `{"email":"alice@example.com","password":"correct-horse-battery"}`
        rec := httptest.NewRecorder()
        req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
        handler.ServeHTTP(rec, req)
        assert.Equal(t, http.StatusCreated, rec.Code)
        assert.Contains(t, rec.Body.String(), "token")
    })

    t.Run("user-create/AC-02 invalid email returns 400", func(t *testing.T) {
        body := `{"email":"not-an-email","password":"correct-horse-battery"}`
        rec := httptest.NewRecorder()
        req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
        handler.ServeHTTP(rec, req)
        assert.Equal(t, http.StatusBadRequest, rec.Code)
    })
}
```

### Step 3 — Check coverage after each test file

```bash
specter coverage
```

```
user-create    T2    4 ACs    2 covered    50%    PASS
```

Repeat until all tier 1 specs hit 100% and tier 2 specs hit 80%.

---

## Phase 5 — VS Code Workspace Walkthrough

With `specter.yaml` in place and specs annotated, the VS Code extension gives you real-time feedback as you write code.

### Coverage panel

Click the **Sp** icon in the activity bar. The **Specter: Coverage** panel shows every spec with its current coverage percentage. Red means below threshold. Click a spec to open it.

### Inline diagnostics

The extension underlines `@ac` annotations in test files when the referenced AC does not exist in any spec. This catches typos and stale references immediately — before CI.

### Run Sync from VS Code

Open the Command Palette (`Ctrl+Shift+P`), type **Specter: Run Sync**. This runs the full `specter sync` pipeline and reports results in the Output panel without leaving VS Code.

### Drift detection

When a spec changes (you edit a constraint or AC description), the extension highlights test files that reference that spec. This is the **intent drift** warning — your tests may no longer match the updated specification.

---

## Phase 6 — Lock It Into CI

Once `specter sync` passes locally, add it to your CI pipeline. This is the gate that prevents specs and tests from drifting apart on every PR.

**GitHub Actions (composite action, pinned version — preferred):**
```yaml
- uses: hanalyx/specter-sync-action@v1
  with:
    version: 0.9.2
```

**Or inline download (if you can't use the composite action):**
```yaml
- name: Install specter
  shell: bash
  run: |
    OS=$(echo "${{ runner.os }}" | tr '[:upper:]' '[:lower:]')
    case "${{ runner.arch }}" in X64) ARCH=amd64 ;; ARM64) ARCH=arm64 ;; esac
    VERSION=0.9.2   # pin a version; don't rely on "latest" in CI
    curl -LO "https://github.com/Hanalyx/specter/releases/download/v${VERSION}/specter_${VERSION}_${OS}_${ARCH}.tar.gz"
    tar xzf "specter_${VERSION}_${OS}_${ARCH}.tar.gz" && sudo mv specter /usr/local/bin/
- name: Specter sync
  run: specter sync
```

---

## Phase 7 — Promote Specs to Approved

When a spec is fully covered and reviewed by your team, promote it:

```yaml
spec:
  id: user-create
  status: approved    # ← was draft
```

`Approved` specs are enforced more strictly by CI. Constraints become `error` by default and coverage thresholds are non-negotiable.

**AI prompt — review a spec before promotion:**

```
Review this Specter spec before we promote it from draft to approved:

[paste the spec]

Check for:
1. All constraints use RFC 2119 language (MUST/MUST NOT/SHOULD/MAY)
2. Every constraint is referenced by at least one AC
3. Every AC has a specific, testable description (not vague)
4. The objective scope clearly states what is excluded
5. Tier assignment is appropriate (1=Security/Money, 2=Core Business, 3=Utility)

Flag any issues. If it looks good, say so and I'll promote it.
```

---

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| `Specter: no specter.yaml found` | Manifest missing | Run `specter init` |
| `error [required] spec/id` | Missing required field | Add the field; see [Schema Reference](SPEC_SCHEMA_REFERENCE.md) |
| `error [pattern] spec/constraints/0/id` | Wrong ID format | Must be `C-01`, `C-02`, etc. |
| AC shows 0% after annotating tests | Annotation not found | Check `@spec` ID matches `spec.id` exactly; check `tests_dir` in `specter.yaml` |
| `specter reverse` generates too many specs | Large codebase | Use `--exclude` flag or add patterns to `specter.yaml` |
| Coverage drops after refactor | Tests deleted | Re-annotate new tests; run `specter coverage` to find the gap |

---

## Reference

- **[QuickStart](QUICKSTART.md)** — 5-minute path if you just want to see it work
- **[Spec Schema Reference](SPEC_SCHEMA_REFERENCE.md)** — every field, type, and validation rule
- **[CLI Reference](CLI_REFERENCE.md)** — all commands and flags
- **[AI Prompts](AI_PROMPTS.md)** — ready-to-use prompts for the full SDD loop
- **[Specter's own specs](../specs/)** — production specs from Specter itself
- **[FAQ](FAQ.md)** — common questions about SDD and Specter

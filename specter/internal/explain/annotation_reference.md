# Test Annotation Reference

How to annotate tests so `specter coverage` counts them and `specter coverage --strict` verifies them.

This is the counterpart to `SPEC_SCHEMA_REFERENCE.md`. The spec reference defines the schema for `.spec.yaml` files. This reference defines the schema for test annotations.

---

## What Specter reads

Specter reads annotations from two places. They serve different purposes and both exist for a reason.

| Channel | Source | Read by | Purpose |
|---|---|---|---|
| 1 | `// @spec <id>` and `// @ac AC-NN` comments above the test function | `specter coverage` | Counts which ACs have annotated tests. |
| 2 | `<spec-id>/AC-NN` in the test's runner-visible output (test title or runtime log) | `specter ingest` | Records pass/fail per AC in `.specter-results.json`. Required by `specter coverage --strict`. |

**Write both.** A test with only channel 1 is counted but not verified. Under `--strict`, counted-but-not-verified equals uncovered, and the AC demotes.

---

## The rules

1. **Source comment format.** Above every test function:
   ```
   // @spec <spec-id>
   // @ac AC-NN
   ```
   One `// @spec` per test. One `// @ac` per AC the test covers. Languages other than C-family use their own comment character: `#` for Python, `--` for SQL, etc.

2. **Runner-visible format.** The `(spec-id, AC-NN)` pair appears in one of:
   - The test title: `<spec-id>/AC-NN` or `<spec-id>:AC-NN` somewhere in the name.
   - The test body, printed at runtime: `// @spec <spec-id>` and `// @ac AC-NN` on separate lines.

3. **Spec id format.** Lowercase kebab-case. Matches the regex `[a-z][a-z0-9-]*[a-z0-9]`. Starts with a letter. Ends with a letter or digit. No underscores, no uppercase, no leading or trailing dash.

4. **AC id format.** Zero-padded two-digit minimum: `AC-01`, `AC-02`, `AC-12`, `AC-100`. **`AC-1` does not match `AC-01`.** The regex accepts `AC-\d+` so single-digit forms extract as `AC-1` — but the coverage gate compares by string equality against the spec, and the spec uses `AC-01`.

5. **One AC per test.** Each test function or subtest covers exactly one `(spec-id, AC-NN)` pair. A JUnit `<testcase>` entry or a `go test -json` test event carries one title, so `specter ingest` assigns one pair per entry. A multi-AC test loses ACs under `--strict`.

6. **One convention per file.** Ingest accepts both forms. Mixing them in one file is legal but error-prone during migration. Pick one form per file.

7. **The extraction regex** (from `specter/internal/ingest/annotations.go`):
   ```
   ([a-z][a-z0-9-]*[a-z0-9])[/:](AC-\d+)
   ```
   The separator between spec id and AC id is `/` or `:`. Nothing else. `_`, `-`, `.`, and whitespace do not work.

---

## By runner and language

### Go (`go test -json`)

Use `t.Run` so each AC has its own subtest and its own runner-visible entry.

```go
// @spec user-create
// @ac AC-01
// @ac AC-02
func TestCreateUser(t *testing.T) {
    t.Run("user-create/AC-01 valid credentials returns 201", func(t *testing.T) {
        // assertions
    })
    t.Run("user-create/AC-02 invalid email returns 400", func(t *testing.T) {
        // assertions
    })
}
```

`go test -json` emits events with `Test: "TestCreateUser/user-create/AC-01 valid credentials returns 201"`. The regex matches `user-create/AC-01`.

### TypeScript / Jest / Vitest (JUnit reporter)

Put the pair in each `it` or `test` title.

```typescript
// @spec user-create
// @ac AC-01
test('[user-create/AC-01] valid email and password creates user and returns 201 with JWT', () => {
    // assertions
});

// @spec user-create
// @ac AC-02
test('[user-create/AC-02] invalid email format returns 400', () => {
    // assertions
});
```

Run with JUnit output:
- Jest: `jest --reporters=jest-junit`
- Vitest: `vitest run --reporter=junit --outputFile=test-results.xml`

JUnit `<testcase name="...">` carries the full title. The regex matches the pair inside the brackets.

### Python / pytest (known limitation)

Python function names cannot contain `/` or `:`. Convention A (title-based) does not work for pytest by default. **Use Convention B (runtime log) for Python.**

```python
# @spec user-create
# @ac AC-01
def test_valid_registration_returns_201(client):
    print('// @spec user-create')
    print('// @ac AC-01')
    response = client.post('/users', json={...})
    assert response.status_code == 201
```

Run pytest with JUnit output:
```
pytest --junitxml=test-results.xml -o junit_logging=all -o junit_log_passing_tests=True
```

`-o junit_logging=all` captures `print()` output into `<system-out>` for every test case. `specter ingest` reads `<system-out>` and matches the body regex `//\s*@spec\s+([a-z][a-z0-9-]*[a-z0-9])` and `//\s*@ac\s+(AC-\d+)`.

**Why not function names.** `def test_user_create_AC_01_valid_returns_201` emits the JUnit title `test_user_create_AC_01_valid_returns_201`. The regex requires `/` or `:` between `user-create` and `AC-01`. `_` does not match. See the BACKLOG entry "Python Convention A gap" — this may change in a future release.

### Rust / `cargo test`

No first-party ingest flavor today. Work around by emitting Convention B to stdout and parsing manually, or wait for a TAP flavor. Track progress in the BACKLOG.

### Runner-log form — Convention B

Works in every language. Use it when you cannot rename test titles (shared naming contract, snapshot tests, external expectations, Python function names).

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
```python
def test_rejects_zero_amount(client):
    print('// @spec payment-charge')
    print('// @ac AC-03')
    # assertions
```

---

## Parameterized tests

A parameterized test produces one JUnit entry per case. Each case carries its own title, so each case needs its own `(spec-id, AC-NN)`.

### Vitest `test.each`

```typescript
// @spec payment-charge
// @ac AC-04
test.each([
    { amount: 0,  ac: 'AC-04', desc: 'rejects zero' },
    { amount: -1, ac: 'AC-04', desc: 'rejects negative' },
])('[payment-charge/$ac] $desc', ({ amount }) => {
    // assertions
});
```

Each case emits a title like `[payment-charge/AC-04] rejects zero`. The regex matches.

### pytest `@pytest.mark.parametrize`

Use Convention B inside the test body — titles are parameter-suffixed function names, which again don't contain `/` or `:`.

```python
# @spec payment-charge
# @ac AC-04
@pytest.mark.parametrize('amount', [0, -1])
def test_rejects_invalid_amount(amount):
    print('// @spec payment-charge')
    print('// @ac AC-04')
    # assertions
```

### Go table tests

```go
// @spec payment-charge
// @ac AC-04
func TestReject(t *testing.T) {
    cases := []struct{ name string; amount int }{
        {"payment-charge/AC-04 zero",     0},
        {"payment-charge/AC-04 negative", -1},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            // assertions
        })
    }
}
```

Both subtests emit the same `(spec-id, AC-NN)`. `specter ingest` merges by worst-status (errored > failed > skipped > passed), so one failing case demotes the AC.

---

## Migrating from v0.9-style source-only

v0.9 and earlier taught source-only annotations: `// @spec` / `// @ac` above the function, no runner-visible form. Those tests work with `specter coverage` (annotation counting) but demote under `--strict` (no results entry).

### Migration recipe

1. **Add `--reporter=junit`** (or `go test -json`) to the CI test command. Reporter output is additive; keep the existing reporters.
2. **Rename test titles** file by file. Inside each file, add `<spec-id>/AC-NN` to every test title. Keep the source comments.
3. **Wire ingest + strict**:
   ```
   specter ingest --junit 'test-results/*.xml'
   specter coverage --strict
   ```
4. **Use `--scope <domain>` for staged rollout.** Enforce `--strict` on one domain at a time. Specs outside the scoped domain keep v0.9 annotation-counting behavior. See `CLI_REFERENCE.md` → `specter coverage` → `--scope`.

### File-atomic discipline

Migrate whole files at once. A half-migrated file (some tests renamed, some not) under `--strict` will demote the unrenamed tests even though the renamed ones pass. `--tests <glob>` scopes by path; it cannot scope by test-title form within a file.

---

## Common mistakes

**`AC-1` instead of `AC-01`.** The regex accepts single-digit; the coverage gate compares against the spec, which uses `AC-01`. Zero-pad always.

**`_` between spec id and AC id.** Python users hit this. `_` is not in the regex. Use Convention B or wait for the Python-separator resolution (BACKLOG).

**Underscore in spec id.** Spec ids are kebab-case. `user_create/AC-01` does not match; `user-create/AC-01` does.

**Uppercase in spec id.** `User-create/AC-01` does not match. The spec id matches `[a-z][a-z0-9-]*[a-z0-9]`.

**Two ACs in one test.** Under `--strict`, `specter ingest` assigns one pair per runner entry. `test('[spec-foo/AC-01 AC-02] two things', ...)` captures only `spec-foo/AC-01`. Split into two tests.

**Source-only annotations in a migrated file.** `specter coverage` will count them; `specter ingest` will drop them from the results; `specter coverage --strict` will demote them. Check `ingest`'s summary line (`Scanned N; extracted M; dropped K`) — K should be zero for fully-migrated files.

**Mixed Convention A and B in one file.** Ingest handles both, but the mix is a migration smell. Pick one.

**Reporter not wired.** `ingest --junit` against a file that doesn't exist is a hard error. `--junit 'test-results/*.xml'` against an empty glob produces zero entries; `--strict` then demotes everything and emits the empty-results warning.

---

## Troubleshooting

**Symptom**: `specter ingest` reports `Scanned N; extracted 0; dropped N`.
**Cause**: Test titles don't match the regex. Usually missing `/` or `:`, or missing the spec id entirely.
**Check**: `specter ingest --junit <path> --verbose` lists every dropped test name. Scan for the pattern you expected.

**Symptom**: `specter coverage --strict` demotes every annotated AC.
**Cause**: `.specter-results.json` has zero entries, or no entry matches the annotated `(spec-id, AC-NN)`.
**Check**: The empty-results warning fires before the demotion report. Read `.specter-results.json`; it should have one entry per AC your tests cover.

**Symptom**: The AC number in the test title is `AC-1`, the spec has `AC-01`, and `--strict` demotes.
**Cause**: String-equality mismatch. Zero-pad the test title.

**Symptom**: pytest tests don't produce annotation entries.
**Cause**: pytest isn't capturing `print()` output in the JUnit XML by default.
**Fix**: `pytest --junitxml=out.xml -o junit_logging=all -o junit_log_passing_tests=True`.

---

## See also

- `CLI_REFERENCE.md` → `specter coverage` (the `--strict`, `--scope`, `--tests` flags)
- `CLI_REFERENCE.md` → `specter ingest` (JUnit and `go test -json` flavors, `--verbose`)
- `docs/explainer/v0.10-ci-gated-coverage.md` (design rationale for the two-channel split)
- `BACKLOG.md` → "Python Convention A gap" (current limitation and candidate resolutions)

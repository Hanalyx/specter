# Annotate your tests and check coverage

## Add annotations to your tests

Annotate each test with two things:

1. **Source comments** above the test: `// @spec <id>` and `// @ac AC-NN`. `specter coverage` counts these.
2. **`<spec-id>/AC-NN` in the test title**. `specter ingest` reads this. `specter coverage --strict` requires it.

Write both. `coverage` works with source comments alone; `--strict` does not.

```typescript
// @spec user-create
// @ac AC-01
test('[user-create/AC-01] valid email and password creates user and returns 201', async () => {
  // ...
});
```

```python
# @spec user-create
# @ac AC-01
def test_user_create_AC_01_valid_registration_returns_201(client):
    # ...
```

```go
// @spec user-create
// @ac AC-01
func TestCreateUser(t *testing.T) {
    t.Run("user-create/AC-01 valid credentials returns 201", func(t *testing.T) {
        // ...
    })
}
```

AC numbers are zero-padded: `AC-01`, not `AC-1`. One test per AC. Full rules are in `docs/TEST_ANNOTATION_REFERENCE.md`.

**Completions are automatic** — type `// @spec ` and Specter suggests IDs; type `// @ac ` and completions are scoped to the spec above.

## Check your coverage

```bash
specter coverage
```

```
user-create    T2    4 ACs    1 covered    25%    PASS
```

Each `@ac` annotation you add moves the percentage up. Tier 1 specs need 100%. Tier 2 specs need 80%.

## Run the full pipeline

```bash
specter sync
```

```
PASS parse:    all specs valid
PASS resolve:  no dependency issues
PASS check:    0 errors
PASS coverage: thresholds met

All checks passed.
```

**Add this to CI** — it exits non-zero if any Tier 1 or Tier 2 spec falls below its coverage threshold.

```yaml
# GitHub Actions
- name: Specter sync
  run: specter sync
```

> **You're done.** The Coverage panel in the sidebar shows real-time status. Gutter icons turn green as you annotate. The status bar shows aggregate coverage at all times.

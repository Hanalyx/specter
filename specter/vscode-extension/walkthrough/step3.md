# Annotate your tests and check coverage

## Add annotations to your tests

Link test functions to specs with two comment lines — no library, no framework, any language:

```typescript
// @spec user-create
// @ac AC-01
test('valid email and password creates user and returns 201', async () => {
  // ...
});
```

```python
# @spec user-create
# @ac AC-01
def test_valid_registration_returns_201(client):
    # ...
```

```go
// @spec user-create
// @ac AC-01
func TestCreateUser_ValidCredentials_Returns201(t *testing.T) {
    // ...
}
```

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

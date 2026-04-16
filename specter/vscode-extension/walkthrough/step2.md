# Annotate your first test

Once you have a spec, link your test functions to it using `@spec` and `@ac` annotations:

```typescript
// @spec payment-create-intent
// @ac AC-01
function testValidCurrencyCreatesIntent() {
  // ...
}
```

**How to get completions:**

1. Type `// @spec ` in a test file — Specter suggests spec IDs ranked by proximity to your file
2. Type `// @ac ` on the next line — Specter scopes suggestions to the spec above

**No annotation yet?** Look for the lightbulb (💡) CodeLens above unannotated test functions — Specter suggests the most relevant ACs using offline tf-idf scoring against your spec corpus. Click to insert.

> Annotations are plain comments — they work in any language and require no build step.

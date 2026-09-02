# Writing acceptance criteria that Specter can verify

An acceptance criterion is the unit Specter tracks. Constraints say what must be
true. Acceptance criteria say how you would know. Every coverage number Specter
reports and every annotation it matches counts acceptance criteria, not
constraints and not specs. Constraints can still fail the pipeline on their own,
through the orphan check in Rule 4, so a spec at 100% coverage is not
automatically a passing spec.

That makes the AC the place where a spec either becomes checkable or stays
decorative. This guide covers how to write one that holds up.

## The shape

Two fields are required. The rest are optional.

```yaml
acceptance_criteria:
  - id: AC-01
    description: Charging an order with zero line items returns 422
    inputs:
      order: valid order, line_items = []
    expected_output:
      status: 422
      error_code: order.empty
    references_constraints:
      - C-03
    priority: high
```

This and every other snippet below shows the acceptance criteria block alone.
In a real file it sits under a top-level `spec:` key, alongside the other
required fields: `id`, `version`, `status`, `tier`, `context`, `objective`, and
`constraints`.

The schema sets `additionalProperties: false`, so a typo in a field name is a
parse error rather than a silently ignored key. That is deliberate. You find out
at parse time, not when a gate mysteriously passes. The exception is a duplicated
AC ID, which nothing checks.

## Rule 1: one criterion, one observable behavior

An AC that describes two behaviors cannot be covered by one test, and Specter's
annotation model expects one test per `(spec-id, AC-NN)` pair. Splitting later
is expensive, because every annotation that referenced the old ID has to move.

Split this:

```yaml
- id: AC-04
  description: Charge validates the amount and sends a receipt email
```

Into this:

```yaml
- id: AC-04
  description: Charging a negative amount returns 400 with error_code amount.invalid
- id: AC-05
  description: A successful charge enqueues a receipt email to the payer address
```

The test for a well-formed AC is whether you can name the assertion that would
fail. If you cannot, the criterion is a summary, not a criterion.

## Rule 2: describe the outcome, not the implementation

A criterion outlives the code that satisfies it. Write what an observer would
see, not how the code gets there.

| Avoid | Prefer |
|---|---|
| Calls `validateAmount()` before persisting | Rejects a negative amount with 400 before any row is written |
| Uses a bcrypt cost factor of 12 | A stored password is not recoverable from the database dump |
| Loops over line items | Total equals the sum of line item amounts times quantities |

The left column breaks when you refactor. The right column breaks only when the
behavior actually changes, which is the signal you want.

## Rule 3: zero-pad the ID

AC IDs must match `^AC-\d{2,}$`. `AC-01` is valid, `AC-1` is not, and neither
are `ac-01` or `AC_01`.

This is stricter than it looks because annotations match on the exact string.
`specter check --test` emits `malformed_ac_id` for a near miss and
`unknown_ac_ref` when the ID is well formed but the spec does not declare it.
Both are errors.

Note which commands run them. You get these diagnostics from `specter check
--test` and from `specter sync --strict`. A plain `specter check` or `specter
sync` does not cross-reference annotations, so a typo passes the check phase
there and silently drops the coverage it was meant to claim. If you want typos
to fail CI, run one of the two forms that checks them.

Numbers do not have to be contiguous, and they should never be reused. Deleting
`AC-07` and giving the number to a new criterion silently repoints every
existing annotation at different behavior.

## Rule 4: wire `references_constraints`

Orphan detection depends on this field. A constraint that no AC references is
reported as `orphan_constraint`, at error severity for Tier 1, warning for Tier
2, and info for Tier 3. A constraint can override that default with its own
`enforcement` field, which wins over the tier.

Referencing a constraint that the spec does not declare is a parse error, not a
check error. Specter reports `dangling_reference` and names the constraints that
are declared, so the fix is usually visible in the message.

An AC may reference several constraints, and several ACs may reference the same
constraint. What you want to avoid is a constraint with no referencing AC, which
means you wrote a requirement and no way to tell whether it holds.

## Rule 5: annotate in both channels

Specter reads annotations from two places, and they are not interchangeable.

1. **Source comments** above the test: `// @spec <id>` and `// @ac AC-NN`.
2. **A runner-visible token** carrying `<spec-id>/AC-NN` in the test title or a
   runtime log line, which `specter ingest` extracts from your runner's output.

The two channels do different jobs, which is why you need both. The source
comment establishes which `(spec, AC)` pairs a test file claims. The ingested
result decides whether each claim passed.

Under `--strictness annotation` the comment alone is enough, because nothing
checks outcomes. Under `threshold` and `zero-tolerance`, coverage is the
intersection: an AC counts only when a discovered test file claims it in a
comment and a result records it as passed. Drop either side and the AC reports
uncovered. A passing result with no source comment scores zero, the same as a
comment with no result.

The manifest default strictness is `threshold`, so this is the default behavior,
not an opt-in. A plain `specter coverage` with no `.specter-results.json` in the
workspace does not fall back to counting comments. It exits 1 and tells you to
run `specter ingest` first. Write both forms, and run `ingest` before `coverage`
in CI.

```go
// @spec payment-charge
// @ac AC-04
func TestCharge_NegativeAmount(t *testing.T) {
    t.Run("payment-charge/AC-04 negative amount returns 400", func(t *testing.T) {
        // assertions
    })
}
```

The Go example works because the subtest title carries `payment-charge/AC-04`.
The separator has to be `/` or `:`. An underscore does not match, so encoding
the pair in a function name as `..._AC_04_...` extracts nothing.

That rules out the title channel for pytest, whose test names are function
names. Python uses the log-line form instead:

```python
# @spec payment-charge
# @ac AC-04
def test_charge_negative_amount_returns_400():
    print("# @spec payment-charge")
    print("# @ac AC-04")
    # assertions
```

The log line only reaches `ingest` if pytest is told to keep stdout for tests
that pass. Without these two options, the JUnit file has no `<system-out>` and
the annotation is dropped silently:

```
pytest --junitxml=results.xml -o junit_logging=all -o junit_log_passing_tests=True
```

Pick one form per file and do not mix them.

**Name Python test files `charge_test.py`.** Specter discovers `.test.ts`,
`.test.js`, `.test.py`, `_test.go`, and `_test.py`. Pytest's usual
`test_charge.py` prefix matches none of them, so the file is never scanned and
the source-comment channel reports nothing. `ingest` still works, because it
reads your runner's output and never looks at source files, but by the
intersection rule above the AC reports uncovered no matter how clean the ingest
was. Pytest collects `charge_test.py` without configuration, so that one name
satisfies both tools.

Do not reach for `charge.test.py`. Specter discovers it, but under pytest's
default import mode the dot makes it an invalid module name, so pytest never
collects the test. It can be forced to run with `--import-mode=importlib` plus a
`python_files` override, which is two non-default settings to buy what
`charge_test.py` gives you for free. Until you set both, this is the worst
outcome available: Specter discovers the file, `--strictness annotation` reports
a confident 100%, `check --test` exits 0, and no test ever ran.

`settings.tests_glob` and `--tests` point discovery somewhere else if your layout
differs. Note two limits. They **replace** the built-in list rather than adding
to it, so a glob added to rescue Python will silently drop every Go file to 0%
unless you enumerate each language yourself:

```yaml
settings:
  tests_glob: ["tests/test_*.py", "*_test.go"]
```

They also apply to `coverage` only. `check --test` always uses the built-in list
and takes no `--tests` flag, so a glob that rescues your coverage numbers leaves
`unreachable_annotation` switched off for the files it redirected to.

Since v0.13.0, `specter check --test` reports `unreachable_annotation` when a
source comment has no runner-visible counterpart, because such an annotation
would demote under a strict coverage run. Plain `specter check` does not run it.
For tests that genuinely cannot carry a token, `// @reachable manual` suppresses
it, and `# @reachable manual` does the same for Python. The directive may sit on
any line and its scope is the whole file.

## Rule 6: pick the tier deliberately

The tier on the spec sets the coverage threshold its ACs must meet.

| Tier | Default threshold | Orphan constraint severity |
|---|---|---|
| 1 | 100% | error |
| 2 | 80% | warning |
| 3 | 50% | info |

Tier is a claim about how much verification the behavior deserves. Marking
everything Tier 1 produces a wall of failures nobody can act on. Marking
everything Tier 3 produces a green build that proves very little. The defaults
are overridable per spec with `coverage_threshold`.

One trap in that override. `coverage_threshold: 0` is schema-valid but does not
mean zero. The code reads it as unset and falls back to the tier default, so a
Tier 1 spec set to `0` gets 100 instead of the loosest possible setting. A
negative value is rejected at parse, so `0` is the only value that surprises you.
If you want a low bar, write `1`.

## Rule 7: know what `approval_gate` does and when

`approval_gate: true` marks an AC as needing a human sign-off recorded in
`approval_date`. Its effect depends entirely on strictness.

- Under `annotation` and `threshold`, it is metadata. Nothing enforces it.
- Under `zero-tolerance`, an AC with `approval_gate: true` and no
  `approval_date` is demoted in the report, and `coverage` exits 3.

Setting the flag and running at `threshold` gives you a documented intention and
no gate. That is a reasonable choice, as long as nobody believes otherwise.

## On `inputs`, `expected_output`, and `error_cases`

These three fields let you state a criterion concretely instead of in prose:

```yaml
- id: AC-09
  description: A charge above the account ceiling is rejected
  inputs:
    account_ceiling: 50000
    charge_amount: 50001
  expected_output:
    status: 409
    error_code: charge.exceeds_ceiling
  error_cases:
    - condition: ceiling is unset on the account
      expected_behavior: fall back to the plan default, do not reject
```

`error_cases` entries require both `condition` and `expected_behavior`. The other
two are free-form objects, so you choose the keys.

**Be clear about what these fields do today: no stage of the pipeline acts on
them.** Parse validates their shape and stops there. Nothing in resolve, check,
coverage, or sync consults their contents, and no gate passes or fails because
of what they say. Two specs identical apart from these fields produce the same
diagnostics, the same coverage report, the same exit codes, and `specter diff`
reports no change between them. The one place they surface is `parse --json` and
`resolve --json`, which echo the parsed document verbatim.

`specter reverse` does not generate them for you, with one narrow exception. It
emits `error_cases` from Python sources when it finds a `pytest.raises` block.
It never emits `inputs` or `expected_output` in any language. If you want a
concrete criterion, you write it.

They still earn their place for two reasons:

- A reviewer can see whether a criterion was loosened. Prose can be reworded
  without the change being obvious. A status code cannot.
- A concrete criterion is the only kind you can check a test against by reading
  both.

Write literals where the criterion has them. Specter's own specs often put prose
inside these keys, which reads well but gives a value-matching tool nothing to
match on.

## Anti-patterns

**The criterion that restates the constraint.** If C-03 says an email must be
unique and AC-03 says email uniqueness is enforced, you have written the same
sentence twice. The AC should say what happens on a duplicate: which status,
which error code, whether the first record survives.

**The criterion nobody can fail.** "The API is performant" and "errors are
handled gracefully" cannot be tested, so any test annotated to them counts as
coverage while proving nothing.

**The bundled test.** One test carrying two `@ac` lines counts both ACs under
`--strictness annotation`, so you cannot tell which behavior broke when it fails.
Under the default `threshold` the failure runs the other way. `ingest` takes the
first `(spec-id, AC-NN)` pair it finds per test case and ignores the rest, so the
second AC records no result and demotes to uncovered. You get a coverage failure
naming an AC you believe is tested.

**Renumbering during a refactor.** IDs are the join key between spec and test.
Treat them as permanent. Add new numbers, retire old ones, and never recycle.

**The criterion added to fix a coverage number.** An AC written to describe what
a test already does inverts the direction of authority. The spec is meant to
constrain the code. Backfilling it from the test makes it a transcript.

## A checklist

Before committing a new acceptance criterion:

- One behavior, and you can name the assertion that would fail it.
- ID zero-padded, unique in the spec, and never previously used. Nothing enforces
  uniqueness, so a duplicated ID parses and checks cleanly while one test credits
  both rows. Check this one by eye.
- Description states an observable outcome, not an implementation step.
- `references_constraints` names at least one declared constraint, unless the
  criterion genuinely stands alone.
- Concrete values in `inputs` and `expected_output` where the behavior has them.
- A test exists carrying both the source comment and a runner-visible token.
- The tier on the spec matches how much verification this behavior deserves.
- `specter ingest` has run, then `specter sync` passes.

## Related reading

- `docs/SPEC_SCHEMA_REFERENCE.md` for the full field reference.
- `docs/TEST_ANNOTATION_REFERENCE.md` for the annotation contract, including
  parameterized tests and the Python limitation.
- `docs/CLI_REFERENCE.md` for strictness levels and exit codes.

# Specter Lexicon

This document defines the terms that belong to Specter specifically. It exists
because several of them mean two things today, and at least one of those splits
has shipped as a defect.

The one to read first is `strict` against `strictness`. Two settings, four
letters apart, governing unrelated things: one is a severity switch on the
checker, the other decides when a criterion counts as covered. Confusing them is
not a parse error, because both are valid keys. That collision is the spine of
Part 1 and the root of most of what follows.

Verified against `release/v0.15.0` at `d85a77e`, using a binary built with
`make build` from `specter/`. No code or spec changed between `d85a77e` and this
document's own first commit, so that reference still holds. Every claim about
current behavior was checked by running `bin/specter` or by reading source.
[Appendix A](#appendix-a-how-each-claim-was-checked) says which, claim by claim.

## Scope

This is not a glossary of spec-driven development. The general methodology terms
(Micro-Spec, SSOT, Intent Drift, Approval Gate, Spec Coverage, the Three Eras)
are defined in `CLAUDE.md` and are not repeated here. Entries below reference
them where a Specter term depends on one.

What belongs here is narrower: a word Specter's own code, specs, or CLI uses in
a load-bearing way, where getting it wrong changes a gate result.

## How to read an entry

Each entry separates three things that are routinely conflated.

**Meaning.** The concept, stated without reference to any one command's
implementation.

**Surfaces.** What each command, spec, flag, or document actually does with the
term today. Where surfaces disagree, both readings are stated. Neither is quietly
promoted to the correct one.

**Standing.** Settled, or open. A term the project has not decided says so.

That separation is the point. A lexicon that records only behavior goes stale the
moment behavior changes. A lexicon that records only intent hides the divergences
that intent was supposed to prevent.

---

---

# Part 1: `strict` and `strictness`

Specter has two settings whose names differ by four letters. They govern
unrelated things. One is a severity switch on the checker. The other is a ladder
deciding when a criterion counts as covered.

Almost every confusion recorded in this document starts there, including
`bugs/SP-SP-046`, which is why the document was commissioned. Read this part in
order. The collision comes first, because the flag entries do not make sense
without it.

## The collision, stated first

Two manifest keys, both accepted, both listed in `validSettingsKeys` at
`internal/manifest/manifest.go:25-26`:

| Key | Type | Constraint | Governs |
|---|---|---|---|
| `settings.strict` | bool | spec-manifest C-11 | Whether checker warnings become errors |
| `settings.strictness` | enum | spec-manifest C-24 | When a criterion counts as covered |

They share no code path. `settings.strict` never affects a coverage number.
`settings.strictness` never changes a diagnostic's severity, except for the one
diagnostic named in the `strictness` entry below.

The practical hazard: a manifest that means one and writes the other gets no
error, because both are valid keys. Writing `strict: true` when you meant
`strictness` leaves the ladder at its default. Writing `strictness: threshold`
when you meant `strict` leaves warnings as warnings. Neither typo is detectable
by the parser, because neither is a typo.

Nothing in the repository proposes renaming either key. Both appear in shipped
manifests. This entry exists to keep them apart, not to argue for a change.

## `strict`

**Meaning.** Not one thing. `strict` names **two** behaviors in the code today,
and was **intended** as a third that was never built. Any single-sentence
definition of it is currently false.

First, severity promotion. Every warning and info diagnostic is reported as an
error, so the command exits non-zero. Measured on a Tier 2 spec with one orphan
constraint: `check` reports `0 error(s), 1 warning(s)` and exits 0, while
`check --strict` reports `1 error(s), 0 warning(s)` and exits 1.

Second, scan gating. On `sync`, it decides whether the test-annotation
cross-reference runs **at all** (`main.go:1347`). Measured, no flag in either
run, only the manifest key changed, with one test file carrying a stale
`@spec bogus-spec`:

```
settings.strict: true     FAIL check: 1 error(s)       exit 1
settings.strict absent    PASS check / PASS coverage   exit 0
```

The diagnostic is `unknown_spec_ref`, hardcoded at error severity
(`internal/checker/test_annotations.go:119`), so no promotion is involved. A
severity switch cannot change which defects are discovered. This one does.

Third, the intended behavior, recorded because it explains the name. `strict`
was meant as a **coverage reporting override**: when a spec clears its tier
threshold, `coverage` reports it as passed, and `settings.strict: true` was to
override that and surface every failed spec and criterion anyway. Nothing in the
tree implements this. `coverage` never reads the key, and severity has no
meaning in a coverage report, which carries percentages rather than diagnostics.

**Surfaces, and the scope is not uniform.** Set by `settings.strict`
(`internal/manifest/types.go:49`) or by a `--strict` flag. The two spellings
apply to **different sets of commands**:

| Command | Accepts `--strict` | Reads `settings.strict` |
|---|---|---|
| `check` | yes | yes, `main.go:711` |
| `sync` | yes | yes, `main.go:1313` and `:1347` |
| `coverage` | yes | **no** |
| `watch` | **no** | yes, `main.go:3003` |

Union of four, intersection of two. Verified with the C-24 guard, which exists
to reject strict combined with `strictness: annotation`: on a manifest holding
both `strict: true` and `strictness: annotation`, plain `coverage` exits 0 and
`coverage --strict` exits 1. The guard cannot see the key it is meant to police.

**Three further defects, all verified.** The key and the flag are not
interchangeable on `sync`: `main.go:1313` combines them, `:1323` reads the flag
alone, so `settings.strict: true` reaches the severity switch and never reaches
the ladder. Promotion is incomplete: it runs inside `CheckSpecs`
(`internal/checker/check.go:150`), and `main.go:744` and `:773` append the
test-annotation families afterward, so those never pass through it. And
spec-manifest C-11 says strict upgrades "warning-severity" diagnostics while the
code upgrades warning **and** info.

**Standing. Open, and the entry above supersedes an earlier claim in this
document that it was settled.** That claim was written from a citation rather
than a measurement: the site at `main.go:1347` was correctly listed as a
consumer and never exercised, so the scan-gating behavior went unrecorded while
the line that proves it sat in the entry.

Treat that as the standing lesson for this column. **A term is settled when
someone has run the cases that would distinguish it, and the entry names those
cases.** A citation shows a term is used somewhere. Only a measurement shows
what it decides.

Filed as `bugs/SP-SP-047` (the three defects) and as a feature request to make
the setting coherent.

## `strictness`

**Meaning as implemented.** A three-value ladder naming how much evidence
Specter demands before it calls an acceptance criterion covered. The values are
`annotation`, `threshold`, and `zero-tolerance`, in increasing order of demand.

**Meaning as intended, which the implementation does not match.** The setting
was meant to answer one question: **does this acceptance criterion have a test
reference?** The goal was confidence that a testable criterion carries a marker
tying it to a test that either passed or failed. The staging was deliberate:
markers in test files first, markers in source files in a later release.

The gap between those two readings is the whole story of this entry. What
shipped is an evidence ladder, which is a coverage-judgment concept. What was
wanted is marker enforcement, which is a checker concept. The ladder can be used
to approximate the goal, because an unannotated criterion counts as uncovered,
but it approximates it as a percentage rather than naming the criterion that
lacks a marker.

**The measurement that shows the gap.** A spec with two criteria, only the first
annotated, no marker anywhere for the second:

```
check --test    All 1 specs passed structural checks    exit 0
coverage        ma-spec  T2  2  1  50%  FAIL   uncovered: AC-02
```

`check` cannot see the missing marker. It scans test to spec, validating the
markers it finds, so a criterion with no marker is invisible to it. `coverage`
scans spec to test, so it is the only command that knows. **The fact the setting
was created to enforce is computed in the command that reports percentages, and
is absent from the command that reports diagnostics.**

**Surfaces.** The enum is declared once, at `internal/manifest/manifest.go:30`.
Both `settings.strictness` and the `--strictness` flag validate against it. A
value outside the enum is rejected at parse for the manifest and at the flag
layer for the CLI, exit 1 either way.

Only two commands accept a `--strictness` flag. The binary ships 14 commands, so
the claim is rested on the source rather than on a sweep of help output: a grep
of `cmd/` for the literal flag names returns five registrations and no others,
all in `main.go`.

| Command | `--strict` | `--strictness` | Registration |
|---|---|---|---|
| `check` | yes | **no** | `:837` |
| `coverage` | yes | yes | `:1239`, `:1241` |
| `sync` | yes | yes | `:1411`, `:1412` |
| the other 11 | no | no | none |

The other 11 are `completion`, `diff`, `doctor`, `explain`, `feedback`,
`ingest`, `init`, `parse`, `resolve`, `reverse`, and `watch`. `watch` is the
one worth naming: it accepts neither flag and still reads `settings.strict` at
`main.go:3003`.

That asymmetry matters more than it looks. `check` accepts `--strict` and has no
ladder at all. Any proposal that defines `--strict` in terms of a strictness
level is therefore undefined on `check`, not merely contested there.

Three surfaces read a strictness level:

| Surface | What the level controls |
|---|---|
| `coverage` | Which report path runs, and which exit-code gates are armed |
| `sync` | The same, inside the coverage phase |
| `check` | The severity of the `unreachable_annotation` diagnostic, and nothing else |

`check` reads the level from `settings.strictness` only, since it has no flag for
it. The word "only" in that last row was checked: a grep of `internal/checker/`
for `strictness` returns hits in exactly three files, all in the
`unreachable_annotation` family, and every one routes through `routeSeverity`.

**Standing. Retiring.** The enum and its behavior are settled as a description,
and the setting is being replaced. See the next section. Nothing in this entry
is disputed; what is decided is that the concept it names is not the concept the
project wants.

## RETIRING: `strictness` becomes `annotation`

Direction set by the founder, 2026-08-18. Recorded here so the deprecation has
one description rather than several. The replacement's shape is a feature
request, `features/SP-006`, and is not settled by this entry.

### The plan

`settings.strictness` and `--strictness` are **preserved until v1.0.0 and
removed there**. Both keep their current behavior for the whole window, so no
existing workspace changes meaning on upgrade.

The replacement is `settings.annotation` and `--annotation`, a single axis
governing **where `@spec` and `@ac` markers must appear**, with three positions:

| Value | Scans | On a missing marker |
|---|---|---|
| `permissive` | test and source | warn |
| `default` | test only | fail |
| `full` | test and source | fail |

Coverage strictness moves to the existing `settings.coverage.tier1`, `tier2` and
`tier3` thresholds. All three at 100 is the strict posture.

### Why this is a replacement rather than a rename

Nothing in the tree governs where markers must appear today. `tests_glob`
decides which files are **scanned**, not where markers must **exist**, and no
diagnostic reports a criterion that has no marker. So `annotation` is a new
capability, and the retirement of `strictness` is a separate act from adding it.
They should be priced separately.

### Which commands this can reach

Verified by which commands discover test files at all:

| Command | Discovers test files | Can enforce markers |
|---|---|---|
| `parse` | no | **No.** Nothing to scan; a flag there would be inert |
| `resolve` | no | **No.** Same |
| `check` | yes, `main.go:723` | Has the files, scans the wrong direction. Needs new spec-side code |
| `coverage` | yes, `main.go:886` | Yes, and already computes the fact |
| `sync` | yes, `main.go:1275` | Passthrough to its phases only |
| `doctor` | yes, `main.go:2223` | Undecided, and not yet considered |

### Open questions this entry does not answer

Four, recorded so they are decided rather than inherited.

**The value names collide with a value being retired.** `annotation` is today a
value of `settings.strictness`. Both keys are accepted until v1.0.0, so a
manifest may legally carry `strictness: annotation` and `annotation: default`
together, meaning different things. That is the shape this document exists to
end.

**The ladder is not monotonic.** `permissive` scans test and source; `default`
scans test only. Moving from the first to the second **narrows** scope while
widening severity, so source findings visible under `permissive` disappear under
`default` and return as errors under `full`. Information vanishes at the step
every adopter takes.

**One corner of the grid is missing.** Scope and severity form a two-by-two and
three corners are used. Test-only-with-warnings, the natural first rung for a
project not yet ready to decide about source markers, has no value.

**`default` names a config position rather than a behavior**, so the name lies if
the default ever moves.

### The overlap to settle before implementing

`annotation: default` says fail when a criterion has no test marker.
`coverage.tierN: 100` says every criterion must be covered, which under
annotation-evidence means annotated. **Those are the same assertion**, and
coverage already enforces it.

They differ usefully: coverage yields a percentage where annotation would yield
a per-criterion diagnostic with file and line; coverage is per-tier where
annotation is global; and coverage cannot reach source files at all.

The interaction needs a stated rule. With `annotation: default` in force, any
tier below 100 can never fail on marker grounds, because the marker check fires
first.

`full` is F7, rejected in `docs/ssrb/SSRB-101.md` on 2026-08-16. It requires a
founder override reopening that brief, and it is the only value that needs one.

## the three levels

**`annotation`.** A source-comment `@ac` counts as coverage. No test result is
consulted. A missing `.specter-results.json` is not an error. This is the
migration mode for a workspace that has not yet made its annotations visible to a
test runner.

**`threshold`.** An annotated criterion counts as covered only when
`.specter-results.json` carries a `status: passed` entry for its
`(spec_id, ac_id)` pair. A missing results file is a hard failure. A spec passes
when its resulting coverage percentage clears its effective threshold. This is
the default.

**`zero-tolerance`.** Everything `threshold` demands, plus two gates that ignore
thresholds entirely. Any annotated criterion whose resolved status is not
`passed` exits 2 (spec-coverage C-25). Any criterion carrying
`approval_gate: true` with an unset `approval_date` exits 3 (spec-coverage C-26).
A spec at 100 percent of its threshold still fails if one annotated criterion
failed.

**Standing.** Settled as concepts. One asymmetry is easy to misread:
`threshold` and `zero-tolerance` run the same report path, so the difference
between them is not what gets demoted. It is which exit-code gates are armed
after demotion.

## the strict path

**Meaning.** The internal routing that `BuildCoverageReportStrict` represents, as
distinct from the weaker `BuildCoverageReportWithResults`. On the strict path, a
missing results file is fatal, annotated criteria without a passing entry demote
across all tiers, and the strict diagnostics apply.

This is a third concept, and it needs its own name. It is neither the severity
switch nor the ladder. It is what `coverage --strict` actually selects, and it is
reachable from two ladder positions as well.

**Surfaces.** One rule, stated in spec-coverage C-31 and spec-sync C-07 and
implemented at `main.go:958`: the strict path runs when effective strictness is
`threshold` or `zero-tolerance`, or when the `--strict` boolean is set. Only
`annotation` stays off it.

Because the manifest default is `threshold`, the strict path is on by default.
That has a consequence measured below: the `--strict` boolean almost never
selects anything that was not already selected.

**Standing.** Settled. Both specs state the rule and the code matches. The term
itself appears in no user-facing output, so it is an internal name.

## effective strictness

**Meaning.** The ladder position in force for one run, after the flag, the
manifest, and the built-in default have been resolved against each other.

**Surfaces.** The two commands resolve it differently, and that is `SP-046`.

`coverage` (`cmd/specter/main.go:932-938`) resolves the flag over the manifest
over a built-in `threshold`. The `--strict` boolean does not participate at all.
It is a separate variable feeding the strict path.

`sync` (`cmd/specter/main.go:1321-1328`) resolves the flag first, then a bare
`--strict` to `zero-tolerance`, then the manifest.

One workspace makes it visible. A Tier 3 spec, two annotated criteria, a results
file marking `AC-02` as `failed`, and `settings.strictness: threshold`. All ten
invocations run in one directory:

| Invocation | Exit |
|---|---|
| `coverage` | 0 |
| `coverage --strict` | 0 |
| `coverage --strictness threshold` | 0 |
| `coverage --strictness zero-tolerance` | 2 |
| `coverage --strictness annotation` | 0 |
| `sync` | 0 |
| `sync --strict` | 2 |
| `sync --strictness threshold` | 0 |
| `sync --strictness zero-tolerance` | 2 |
| `sync --strictness annotation` | 0 |

Every row agrees except the two `--strict` rows, which disagree with each other.
`sync --strict` reaches the ladder. `coverage --strict` does not.

`docs/EXIT_CODES.md` defines effective strictness as the result of resolving
"`--strictness`, the `--strict` alias, and `settings.strictness`". That asserts a
single resolution rule and calls `--strict` an alias. No command implements that
rule as written, and the alias claim is false on `coverage` and undefined on
`check`. It must change alongside the code.

**Standing.** Open, on the `--strict` input only. The flag-over-manifest-over-
default order is settled and both commands implement it.

## `--strict`: three flags sharing a name

**Meaning.** None that holds across the CLI. `--strict` is registered three
times, with three help strings and three behaviors. It is not one flag resolved
by divergent rules. It is three flags that happen to be spelled the same.

**Surfaces.**

| Site | Command | Help text says | What it does |
|---|---|---|---|
| `main.go:837` | `check` | "Treat warnings as errors (also set via settings.strict in specter.yaml)" | The severity switch, and nothing else |
| `main.go:1239` | `coverage` | "Require .specter-results.json and treat any non-passed annotated AC as uncovered (all tiers)" | Forces the strict path at `:958`. Not severity, not the ladder |
| `main.go:1411` | `sync` | "Treat warnings as errors (also set via settings.strict in specter.yaml). Alias for --strictness zero-tolerance when --strictness is not set." | Severity, plus test-annotation checking, plus the ladder |

Only `sync` touches the ladder. Only `check` and `sync` touch severity. `coverage`
touches neither.

**`sync --strict` does three separate things**, and no single document says so.
It sets the checker's `Strict` field (`main.go:1313`), so the check phase
upgrades warnings to errors. It sets `CheckTestAnnotations` (`main.go:1347`), so
the check phase cross-references test annotations against specs. And it sets
effective strictness to `zero-tolerance` (`main.go:1321-1328`).

Measured on a workspace with one orphan constraint in a Tier 2 spec:
`sync --strict` reports `FAIL check: 1 error(s)`, while
`sync --strictness annotation` reports `PASS check: 1 warning(s)`. The two are
not interchangeable even in the check phase, which reads no coverage strictness.

**A fourth description exists.** `coverage --strictness`'s own help at
`main.go:1241` describes `--strict` a fourth way:

```
threshold and zero-tolerance route through the same strict path as --strict; --strict is equivalent to --strictness threshold under the default manifest strictness and does not override a manifest-set level.
```

Against `sync --strict`'s help at `:1411`, which calls it an alias for
`zero-tolerance`. Each string is accurate about its own command. Read together
they define one flag name four ways.

### `coverage --strict` never changes a coverage verdict

Measured, one fixture, the manifest edited between runs:

| `settings.strictness` | `coverage` | `coverage --strict` |
|---|---|---|
| `annotation` | 0 | 1 |
| `threshold` | 0 | 0 |
| `zero-tolerance` | 2 | 2 |
| no manifest at all | 0 | 0 |

The single non-matching cell is not a coverage judgment. It is spec-coverage
C-24 refusing to run: `--strict requires settings.strictness >= threshold`.

The reason is the default. The strict path is already on at `threshold`, and
`threshold` is what an absent or silent manifest resolves to, so the only ladder
position where `--strict` would have something to force is `annotation`, where it
errors instead. Two behaviors on `coverage` still key on the literal flag:

- **spec-coverage C-24**, the refusal above.
- **spec-coverage C-23**, which makes `--scope <domain>` require `--strict`.
  `coverage --scope foo --strictness zero-tolerance` is refused with
  `error: --scope requires --strict`, even though it is the stricter mode.
  Verified by running.

`sync` has no equivalent of C-24. `sync --strict --strictness annotation` passes
silently where the same combination on `coverage` is a hard error. Verified by
running both.

**Standing.** Open. This is the term the project must decide.

## PROPOSED: retire the overload

**This section is a proposal. A human signs off before `SP-046` is implemented.**

The earlier framing of this document offered two readings of `--strict` as a
single concept. That framing was wrong and has been withdrawn. `check` accepts
`--strict` and has no `--strictness` flag, so a definition of `--strict` in terms
of a ladder position is not merely contested on `check`. There is no ladder there
to alias.

### The proposal

**`--strict` means the severity switch, on every command that has one. The ladder
is reachable only through `--strictness` and `settings.strictness`.**

This is what the flag's name says, what `settings.strict` already means, and what
`check --strict` already does. It gives one word one job.

Concretely: `sync --strict` keeps its severity effects and stops implying
`zero-tolerance`. `coverage --strict` has no severity work to do, because
`coverage` emits no checker diagnostics, so it is removed or deprecated rather
than redefined. The measurement above is what makes that affordable: the flag
never changed a coverage verdict, so nothing that was failing starts passing.

### Why this is not a loosening of a gate

It is worth stating plainly, because the change removes `zero-tolerance` from a
flag that currently implies it.

`--strictness zero-tolerance` and `settings.strictness: zero-tolerance` both stay
exactly as they are. Every gate remains reachable. What goes away is one
spelling that reached a gate by implication rather than by request. This
loosens a conflation, not an enforcement level.

That distinction has a limit worth naming. A CI job running `sync --strict`
today and relying on the implied `zero-tolerance` loses that gate on upgrade,
silently, unless the change ships with a deprecation cycle and a release note.
The gate is still reachable; that job is not reaching it any more. This is the
proposal's real cost and it should not be minimized into the paragraph above.

### What the proposal costs

**spec-sync C-06 must be rewritten.** It states plainly that `--strict` "remains
as an alias for `--strictness zero-tolerance` for backward compatibility". That
sentence is the thing being retired.

**spec-coverage C-23 must be reworded.** `--scope` currently requires the literal
`--strict`. With that flag gone from `coverage`, the prerequisite has to be
restated against the strict path, which means `--strictness threshold` or higher.
That is closer to what staged adoption wants anyway, since `--scope` exists to
let a workspace enforce one domain per wave.

**spec-coverage C-24 loses its subject.** Its guard against
`--strict` with `strictness: annotation` has nothing to guard once `coverage`
has no `--strict`. The underlying misconfiguration it caught, an operator asking
for strict behavior in a workspace configured not to have it, does not go away.
Whether it is worth re-expressing against `--strictness` is a real question and
this proposal does not answer it.

**spec-coverage C-31 survives.** Its clause that `--strict` does not override a
manifest-set level is consistent with the proposal, since under it `--strict`
never touches the level at all.

**`sync --strict`'s third effect is still unresolved.** It also sets
`CheckTestAnnotations` (`main.go:1347`), which is neither severity nor ladder.
The proposal settles the ladder question and says nothing about this one.
Whatever is decided, the fix must state what happens to all three effects, or the
next reader inherits the same problem one layer down.

**`make dogfood-strict` must change or be renamed.** `SP-046` records that it
runs `coverage --strict` in a tree with no root `specter.yaml`, so it resolves to
`threshold` and codes 2 and 3 have never fired in Specter's own dogfooding. Under
this proposal the flag it invokes stops existing on that command. The target
should call `--strictness zero-tolerance` and mean its name, or be renamed to
match what it measures.

### The alternative, and why it is weaker

The narrow fix is to leave the overload in place and make `coverage --strict` and
`sync --strict` agree, which is what `SP-046` literally asks for. It is a smaller
change and it closes the reported divergence.

It cannot be made coherent, though. Whichever behavior wins, `check --strict`
still means something else, because `check` has no ladder. The result is a flag
that means one thing on one command and another thing on two others, now with a
specification asserting they agree. That is the condition this document was
commissioned to end rather than to document more precisely.

### What must be fixed under any decision

`docs/EXIT_CODES.md` describes a single resolution rule over three inputs and
calls `--strict` an alias. No command implements that rule in full, and the alias
claim is undefined on `check`. It must be rewritten alongside the code, and it is
listed here because this document does not edit it.

# Part 2: coverage terms

## covered, uncovered, demoted

**Meaning.** An acceptance criterion is **covered** when the active strictness
level's evidence requirement is met for it. **Uncovered** is the negation.
**Demoted** is the transition: a criterion that has an annotation, and would be
covered under `annotation`, but is reported uncovered because the strict path
found no passing result.

**Surfaces.** The report labels only two states. There is no third column for a
demoted criterion. It appears on the `uncovered:` line exactly as an un-annotated
criterion does.

Verified by running, on the fixture above. Under `--strictness annotation` the
spec reports 2 covered of 2, at 100 percent, with no `uncovered:` line. Under
`--strictness threshold` the same spec reports 1 covered of 2, at 50 percent,
with `uncovered: AC-02`. Nothing in the table says that AC-02 is annotated and
failing rather than simply untested. The only difference an operator sees between
the two runs is the number.

The distinction survives in the diagnostics rather than in the table.
Spec-coverage C-28 requires a per-criterion hint when a criterion has a source
annotation but no matching pass, which is the demotion case specifically.

**Standing.** Settled in behavior. The word "demote" appears throughout the specs
and in no user-facing output, so it is an internal term. That is worth knowing
before writing it into a user document.

## spec coverage against test coverage

**Meaning.** **Spec coverage** is the fraction of a spec's acceptance criteria
that have satisfying evidence. **Test coverage**, in the ordinary sense of lines
or branches executed, is not something Specter measures at all.

**Surfaces.** Every percentage Specter prints is spec coverage. The tool never
reads a coverage profile. `.specter-results.json` carries pass and fail status
per criterion, not execution data.

**Standing.** Settled. Recorded here only because the two are named similarly and
the confusion is expensive: an operator who reads a Specter percentage as line
coverage draws the wrong conclusion in both directions.

## results file

**Meaning.** `.specter-results.json`, the file mapping `(spec_id, ac_id)` pairs
to test statuses. It is the evidence channel that the strict path requires.

**Surfaces.** Produced by `specter ingest`. Consumed by `coverage` and by
`sync`'s coverage phase.

The path is asymmetric, and this is a trap worth naming. `ingest` accepts
`--output <path>`, defaulting to `.specter-results.json`. Both readers open the
literal string `.specter-results.json` in the working directory
(`main.go:993` and `:1331`) and have no flag to point elsewhere. So
`ingest --output results/run.json` succeeds, and every later strict run reports
the file as missing.

Status values are the enum `passed`, `failed`, `skipped`, `errored`
(spec-coverage C-21). A value outside that enum ranks as failed and never as
passed (C-33), and produces a warning naming the unrecognized value (C-30).

When several entries name one pair, the worst status wins (C-33). Row order in
the file must not change any reported value.

**Standing.** Settled. The fixed path is a real constraint that nothing documents
as a decision, but nothing contests it either.

## annotation, and its two channels

**Meaning.** A marker in a test that names the criterion the test covers. Specter
recognizes two channels for it, and they are not interchangeable.

**Surfaces.**

The **source channel** is a comment in the test file: `// @spec <id>` and
`// @ac AC-NN`. It is found by reading the file. It is what
`--strictness annotation` counts, and it is what `check --test` cross-references.

The **runner-visible channel** is a `<spec-id>/AC-NN` token that appears in the
test runner's own output. `docs/TEST_ANNOTATION_REFERENCE.md` names two ways to
produce it: Convention A puts the token in the test title, Convention B prints it
from the test body. Python cannot use Convention A, because function names cannot
contain a slash.

The distinction is the single most expensive one in this document. A source
annotation alone satisfies `annotation` and satisfies nothing above it, because
`ingest` never sees it and so no results entry exists for the pair. That is the
demotion case, and it is what `unreachable_annotation` warns about before it
happens.

**Standing.** Settled. Both channels are specified and both are implemented.

## `unreachable_annotation`

**Meaning.** A `check` diagnostic naming a source annotation whose test produces
no runner-visible token, so it would demote under the strict path.

**Surfaces.** Severity routes through the strictness level: suppressed under
`annotation`, warning under `threshold`, error under `zero-tolerance`. Suppressed
per file by `// @reachable manual`, or `# @reachable manual` in Python.

A softer sibling, `unreachable_annotation_unknown`, fires when the scanner cannot
recognize the test shape at all. It is always a warning and never fails a gate.

**Standing.** Settled.

## `coverage_threshold` and the effective threshold

**Meaning.** The percentage a spec's coverage must reach to pass. The
**effective** threshold is the one actually compared, after the spec value, the
manifest tier value, and the built-in tier default are resolved.

**Surfaces.** Precedence is spec value, then `settings.coverage.tierN`, then the
built-in defaults of 100, 80, and 50 for Tiers 1, 2, and 3. A declared 0 means
zero and not "unset", per spec-coverage C-36, at both override layers.

The effective number is emitted as the `threshold` field of `coverage --json`, so
it can be read rather than inferred. Verified by running: the fixture reports
`"tier": 3, "threshold": 50`.

The comparison is greater-than-or-equal against the stored `coverage_pct`, which
carries one decimal place. The human-readable column shows the integer floor of
that value (spec-coverage C-35). A threshold set to the integer the tool just
printed always passes.

**Standing.** Settled, and recently. C-35 and C-36 both landed to close defects
where displayed and compared numbers disagreed.

---

# Part 3: structural terms

## tier

**Meaning.** An integer from 1 to 3 declaring how much rigor a spec is held to.
Tier 1 is the strictest. It sets the default coverage threshold and the default
severity of several diagnostics.

**Surfaces.** Declared per spec as `tier:`. Resolved by `ResolveTier` in
`internal/manifest/tier.go` through a four-step cascade, read from the function
body rather than from its comment: an explicit spec `tier:` greater than 0, then
the tier of a domain in `specter.yaml` that lists the spec, then `system.tier`,
then a hardcoded default of 2.

**Standing.** Settled.

## tier override

**Meaning.** `settings.tier_overrides` in the manifest, a per-spec map that is
supposed to replace a spec's declared tier.

**Surfaces.** Parsed at `internal/manifest/types.go:51`. A resolver exists,
`ResolveTierWithOverrides` at `types.go:159`, and it correctly applies the
override.

Nothing calls it. A repository-wide grep for `ResolveTierWithOverrides` returns
only its own definition. This is `bugs/SP-SP-001`.

Verified by running. A manifest declaring `tier_overrides: {lo: 1}` against a
spec declaring `tier: 3` produces this warning:

```
warn [tier_conflict] spec "lo" declares tier: 3 but specter.yaml tier_overrides assigns tier: 1 — using override (1)
```

and then `coverage --json` reports `"tier": 3, "threshold": 50` for that same
spec. The message says the override is in use. It is not.

**Standing.** Open as a defect, not as a definition. The intended meaning is not
in dispute. The implementation does not deliver it, and the warning text claims
otherwise. This is the case `CLAUDE.md` cites: a program's own log line is a
claim by an author, not evidence.

## tier conflict

**Meaning.** Two incompatible things, depending on which surface you read.

**Surfaces.** The only producer of the string `tier_conflict` is
`cmd/specter/main.go:804`, which prints the warnings from
`manifest.CheckTierConflicts`. That function compares a spec's declared tier
against its `settings.tier_overrides` entry. Verified by running, in the fixture
above.

`docs/CLI_REFERENCE.md:190` documents `tier_conflict` under `specter check` as
"A higher-tier spec depends on a lower-tier spec (e.g., Tier 1 depends on Tier
3)". Verified by running that this does not fire: the same fixture has a Tier 1
spec depending on a Tier 3 spec, and the only `tier_conflict` emitted is the
override one.

The documented meaning has no producer anywhere in the tree.

**Standing.** Open. The definition is settled by the code (an override
mismatch). The documentation states a different definition for a check that does
not exist. This belongs in a bug and is listed in
[Part 5](#part-5-where-two-surfaces-disagree).

## orphan constraint

**Meaning.** A constraint that no acceptance criterion references. It is a spec
that states a rule and never says how the rule is verified.

**Surfaces.** Detected by `check`. Default severity by tier: Tier 1 error, Tier 2
warning, Tier 3 info. A constraint may override its own severity with
`enforcement:`.

`check --strict` upgrades all of these to error. Verified by running: a Tier 2
orphan reports `0 error(s), 1 warning(s)` under `check` and `1 error(s), 0
warning(s)` under `check --strict`.

`specs/spec-check.spec.yaml` states the Tier 3 severity twice and inconsistently.
Constraint C-02 says `info`, which matches the code. The objective scope on line
35 says `warning`. This is `bugs/SP-SP-003`.

**Standing.** Settled in code and in C-02. Open as a specification defect,
because the spec contradicts itself and Specter's own methodology says the spec
decides.

## structural conflict

**Meaning.** A downstream spec's acceptance criterion contradicts an upstream
spec's constraint. The canonical shape: an upstream constraint requires a field,
and a downstream criterion handles that field being absent.

**Surfaces.** Detected by `check` across the dependency graph. Default severity
error, overridable per constraint via `enforcement:`.

**Standing.** Settled as a definition. `bugs/SP-SP-004` records false positives
against spec-check C-05, which requires a zero false-positive rate, and
`bugs/SP-SP-014` records that the diagnostic names two specs as one. Both are
implementation defects rather than disputes about what the term means. Neither
was re-verified for this document.

## dangling reference

**Meaning.** Two unrelated defects share this name in the shipped output.

**Surfaces.** Both were reproduced by running, on the same spec file.

The **parse** meaning: an acceptance criterion's `references_constraints` names a
constraint the spec does not declare. Emitted as a `ParseError.Type` from
`internal/parser/parse.go:189`, per spec-parse C-10. Observed message:

```
  error [dangling_reference] spec.acceptance_criteria[AC-01].references_constraints: references constraint "C-99" which is not declared in this spec (declared constraints: [C-01])
```

The **resolve** meaning: a `depends_on.spec_id` names no discovered spec. Emitted
as a `Diagnostic.Kind` from `internal/resolver/resolve.go:106`, per spec-resolve
C-05. Observed message:

```
error [dangling_reference] Spec "alpha" depends on "nosuchspec" which does not exist
```

`docs/CLI_REFERENCE.md:94` documents only the second, under `specter resolve`.
That entry is correct for its command. Nothing documents that the same string
also names a parse error about constraint references.

**Standing.** Open, but low cost. The two never appear in the same command's
output, because parse errors block resolution. The collision is a reader problem
and a machine-consumer problem, not a gate problem. A consumer filtering
diagnostics by kind gets two defect classes in one bucket.

## gap

**Meaning.** Three uses, one of them real.

**Surfaces.** The shipped meaning is a field: `gap: true` on a generated
acceptance criterion. `specter reverse` sets it on placeholder criteria it
generates for constraints it could not match to existing tests
(`internal/reverse/gap.go:24`). It is declared in the schema at
`internal/parser/spec-schema.json:302`.

The second use is informal. "Coverage gap" appears in prose meaning an uncovered
criterion. It has no field, no diagnostic, and no code.

The third is unshipped. `specs/spec-check.spec.yaml:43` lists "Gap detection
(uncovered input paths, Phase 8)" in its objective scope. Phase 8 has not
shipped.

**Standing.** The field is settled. The prose use should be avoided in technical
documents, because it collides with a schema field that means something narrower.

---

# Part 4: reserved, but not defined

These names appear in Specter documents without a definition anywhere. They are
listed so nobody mistakes a reservation for a settled term.

**evidence stream.** `docs/EXIT_CODES.md:181` reserves exit codes 20 to 29 for
"Evidence stream validation". No code in that band is allocated, no command
emits one, and no spec defines what an evidence stream is. The band is an
allocation, not a feature.

**deferred criterion.** `docs/EXIT_CODES.md:18` describes a prototype in which a
failing test was misreported as a "stale deferral marker". No shipped schema
field, diagnostic, or command uses the term. There is no way to defer a criterion
in Specter today.

**baseline.** Used in exactly one place with a precise meaning: the first
positional argument to `specter diff coverage <baseline.json> <current.json>`,
which is a `coverage --json` report from an earlier run. It carries no broader
sense. It is not a stored artifact, and Specter does not manage one.

---

# Part 5: where two surfaces disagree

An index of every divergence found while writing this document. Each is stated
above with its evidence.

| Term | Surface A | Surface B | Filed |
|---|---|---|---|
| `--strict` on `check` | The severity switch. `check` has no `--strictness` flag at all | On `sync` the same flag sets a ladder position | Not filed |
| `--strict` on `coverage` | Forces the strict path, never touches the level | `sync` maps it to `zero-tolerance` | `SP-SP-046` |
| `--strict` in help text | `coverage --strictness` help at `main.go:1241`: "equivalent to `--strictness threshold` ... does not override a manifest-set level" | `sync --strict` help at `main.go:1411`: "Alias for `--strictness zero-tolerance`" | `SP-SP-046` |
| `--strict` descriptions | Four exist: `main.go:837`, `:1239`, `:1411`, and `:1241` | No two of the four are equivalent. Each states or omits behavior another asserts | `SP-SP-046` |
| effective strictness | `EXIT_CODES.md` states one resolution rule with `--strict` as an alias | Neither command implements that rule in full, and it is undefined on `check` | `SP-SP-046` |
| `settings.strict` and `settings.strictness` | Both valid keys at `manifest.go:25-26`, four letters apart | One is checker severity, the other is coverage judgment. Confusing them is not a parse error | Not filed |
| results file path | `ingest --output` writes anywhere | `coverage` and `sync` read only `./.specter-results.json` | Not filed |
| `--strict` with `strictness: annotation` | `coverage` rejects it, per C-24 | `sync` accepts it silently | Not filed |
| `--scope` prerequisite | Requires the literal `--strict` flag | `--strictness zero-tolerance` is refused despite being stricter | Not filed |
| `tier_conflict` | Code: a `tier_overrides` mismatch | `CLI_REFERENCE.md:190`: a high-tier spec depending on a low-tier one | Not filed |
| `tier_overrides` | Warning says "using override" | No caller applies it | `SP-SP-001` |
| `dangling_reference` | Parse: an undeclared constraint reference | Resolve: an unknown `depends_on` target | Not filed |
| Tier 3 orphan severity | `spec-check` C-02 and the code: `info` | `spec-check` objective scope, line 35: `warning` | `SP-SP-003` |
| sync's default strictness | Code comment at `main.go:1318`: "ultimate default is `annotation`" | Behavior: `threshold`, because the manifest loader fills it in | Not filed |

---

# Appendix A: how each claim was checked

Claims about current behavior fall into two groups. Errors concentrate in the
second, so the split is recorded rather than summarized.

## Verified by running `bin/specter`

The binary was built with `make build` from `specter/` at `d85a77e` and reports
`specter version 0.14.1`. Fixtures were written to a scratch directory outside
the repository.

- The ten-row strictness table, on a Tier 3 spec with two annotated criteria, a
  results file marking `AC-02` failed, and `settings.strictness: threshold`.
- The same table with `specter.yaml` deleted. Results were identical, which is
  what confirms the manifest loader supplies `threshold` rather than leaving the
  value empty.
- `coverage` and `sync` on a workspace with no manifest and no results file. Both
  exit 1 with the same message naming `strictness "threshold"`, which is how
  sync's effective default was established as `threshold` and not `annotation`.
- `coverage --strict --strictness annotation` rejected with the C-24 message, and
  the same combination on `sync` passing.
- `coverage --scope foo` and `coverage --scope foo --strictness zero-tolerance`,
  both refused with `error: --scope requires --strict`.
- Both `dangling_reference` messages, on one spec file carrying both defects.
- `tier_conflict` firing for a `tier_overrides` mismatch, and not firing for a
  Tier 1 spec depending on a Tier 3 spec, in one fixture that had both.
- `coverage --json` reporting `"tier": 3, "threshold": 50` for a spec whose
  override warning had just claimed tier 1 was in use.
- A Tier 2 orphan constraint reported as a warning by `check` and as an error by
  `check --strict`.
- `sync --strict` failing the check phase on that orphan, and
  `sync --strictness annotation` passing it.
- `coverage --strictness annotation` reporting the fixture at 100 percent with no
  `uncovered:` line, against `--strictness threshold` reporting 50 percent with
  `uncovered: AC-02`.
- `specter ingest --help`, which is where `--output` was found.
- `specter --help`, for the command list. There are 14.
- `--help` on 10 of those 14, to read their flag sets. The claim that only
  `check`, `coverage`, and `sync` accept `--strict` does **not** rest on that
  sweep, because it did not cover `completion`, `feedback`, `init`, or
  `reverse`. It rests on the source grep recorded below, which covers all 14.
- The four-row table showing `coverage --strict` never changes a coverage
  verdict, run on one fixture with `settings.strictness` edited between runs and
  once with the manifest deleted.

Program output quoted in fenced blocks is reproduced exactly, em dashes
included. The style checker does not scan fenced blocks, confirmed with a scratch
file, so no character in a quoted message was changed to satisfy the writing
standard.

## Verified by reading source or specs

- The three `--strict` flag registrations at `main.go:837`, `:1239`, and `:1411`,
  and their help strings.
- The three effects of `sync --strict`, at `main.go:1313`, `:1347`, and
  `:1321-1328`.
- The strictness resolution blocks at `main.go:932-938` and `:1321-1328`, and the
  stale comment above the second one at `:1318`.
- The strict-path routing decision at `main.go:958`.
- `validateStrictness` at `internal/manifest/manifest.go:126-130`, which fills an
  empty strictness with `threshold`, and `Defaults()` at `:282`, which does the
  same when no manifest exists. `loadManifest` at `main.go:1671-1685` returns one
  or the other, so the field is never empty downstream.
- `ResolveTierWithOverrides` at `internal/manifest/types.go:159` having no caller.
- The two `dangling_reference` emit sites, `internal/parser/parse.go:189` and
  `internal/resolver/resolve.go:106`.
- The `settings.strict` consumers at `main.go:711`, `:1313`, and `:3003`.
- `Gap` at `internal/schema/types.go:82` and `internal/reverse/gap.go:24`.
- `ResolveTier` in `internal/manifest/tier.go`, read as a function body and not
  as its comment, for the four-step tier cascade.
- The hardcoded `.specter-results.json` reads at `main.go:993` and `:1331`, and
  the absence of any path flag on `coverage` or `sync`.
- Every hit for `strictness` under `internal/checker/`, which lands only in
  `unreachable_annotation.go`, `unreachable_ts.go`, and `unreachable_py.go`.
- All four descriptions of `--strict`, at `main.go:837`, `:1239`, `:1411`, and
  `:1241`.
- A grep of `cmd/` for the literal strings `"strict"` and `"strictness"`,
  excluding tests. It returns five flag registrations, all in `main.go`: three
  for `--strict` (`:837`, `:1239`, `:1411`) and two for `--strictness` (`:1241`,
  `:1412`). This is the evidence for which commands accept which flag, and it is
  stronger than a help sweep because it cannot miss a command.
- `validSettingsKeys` at `internal/manifest/manifest.go:25-26`, which is what
  establishes that `strict` and `strictness` are both accepted keys and that
  writing one for the other is not a parse error.
- spec-manifest C-11 (`settings.strict`) and C-24 (`settings.strictness`).
- Constraint text: spec-coverage C-19 through C-36, spec-sync C-06 through C-09,
  spec-check C-01 through C-05, spec-parse C-10.
- Documentation text: `docs/EXIT_CODES.md` Terms section and line 181,
  `docs/CLI_REFERENCE.md` lines 94, 188 to 193, and
  `docs/TEST_ANNOTATION_REFERENCE.md` on Conventions A and B.

## Not verified for this document

- `bugs/SP-SP-004` (structural conflict false positives) and `bugs/SP-SP-014`
  (the diagnostic naming two specs as one) are cited from their bug reports. The
  structural conflict entry above states the term's meaning, which those bugs do
  not contest, and does not restate their findings as measurements.
- The `SP-046` claim that `make dogfood-strict` has never exercised codes 2 or 3
  is cited from the bug. The mechanism was confirmed here (a `--strict` run with
  no root manifest resolves to `threshold`), but the dogfood target itself was
  not run.

# Specter Lexicon

This document defines the terms that belong to Specter specifically. It exists
because several of them mean two things today, and at least one of those splits
has shipped as a defect.

Verified against `release/v0.15.0` at `d85a77e`, using a binary built with
`make build` from `specter/`. Every claim about current behavior was checked by
running `bin/specter` or by reading source. [Appendix A](#appendix-a-how-each-claim-was-checked)
says which, claim by claim.

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

# Part 1: strictness, and the word `strict`

These four entries are why the document was commissioned. `bugs/SP-SP-046`
records that `--strict` and `--strictness zero-tolerance` are not the same thing,
and that `coverage` and `sync` resolve `--strict` by different rules. The fix for
that bug is blocked on the definition proposed at the end of this part.

## strictness level

**Meaning.** A three-value enum naming how much evidence Specter demands before
it calls an acceptance criterion covered. The three values are `annotation`,
`threshold`, and `zero-tolerance`, in increasing order of demand.

**Surfaces.**

The enum is declared in one place, `internal/manifest/manifest.go:30`, and both
the manifest key `settings.strictness` and the `--strictness` flag validate
against it. A value outside the enum is rejected at parse for the manifest and at
the flag layer for the CLI. Rejection is exit 1.

`coverage`, `sync`, and `check` all read a strictness level, for different
purposes:

| Surface | What the level controls |
|---|---|
| `coverage` | Which report path runs, and which exit-code gates are armed |
| `sync` | The same, inside the coverage phase |
| `check` | The severity of the `unreachable_annotation` diagnostic only |

`check` is worth naming because it is easy to miss. Under `annotation` the
diagnostic is suppressed, under `threshold` it is a warning, under
`zero-tolerance` it is an error. That routing is documented at
`docs/CLI_REFERENCE.md:193`.

The word "only" in that table is load-bearing and was checked. A grep of
`internal/checker/` for `strictness` returns hits in exactly three files, all in
the `unreachable_annotation` family (`unreachable_annotation.go`,
`unreachable_ts.go`, `unreachable_py.go`), and every one of them routes through
`routeSeverity`. No other checker behavior reads the level.

**Standing.** Settled. The enum, its three values, and their order are not in
dispute anywhere in the repository.

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

**Standing.** Settled as concepts. Note one asymmetry that is easy to misread:
`threshold` and `zero-tolerance` both run the same report path, so the difference
between them is not what gets demoted. It is which additional exit-code gates are
armed after demotion.

## effective strictness

**Meaning.** The level in force for one run, after the flag, the manifest, and
the built-in default have been resolved against each other.

**Surfaces.** This is where the two commands disagree, and the disagreement is
`SP-046`.

`coverage` (`cmd/specter/main.go:932-938`) resolves the flag over the manifest
over a built-in `threshold`. The `--strict` boolean does not participate. It is a
separate variable.

`sync` (`cmd/specter/main.go:1321-1328`) resolves the flag first, then a bare
`--strict` to `zero-tolerance`, then the manifest.

One workspace makes the divergence visible. A Tier 3 spec, two annotated
criteria, a results file marking `AC-02` as `failed`, and
`settings.strictness: threshold`. All ten invocations were run in that one
directory:

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

The two commands state the divergence in their own help text, in opposite
directions. From `main.go:1241`, on `coverage --strictness`:

```
--strict is equivalent to --strictness threshold under the default manifest strictness and does not override a manifest-set level.
```

From `main.go:1411`, on `sync --strict`:

```
Alias for --strictness zero-tolerance when --strictness is not set.
```

Each help string is accurate about its own command. Read together, they define
the same flag two ways.

A third document disagrees with both. `docs/EXIT_CODES.md` defines effective
strictness as the result of resolving "`--strictness`, the `--strict` alias, and
`settings.strictness`". That description asserts a single resolution rule and
calls `--strict` an alias. It is true of `sync` and false of `coverage`, and it
must change when `SP-046` is fixed.

**Standing.** Open. The definition below is proposed, not settled.

## `--strict`

**Meaning.** Currently none that holds across the CLI. The name is registered on
three commands with three different effects.

**Surfaces.** Three registrations, three help strings, verified in source and by
running:

| Command | Site | Effect |
|---|---|---|
| `check` | `main.go:837` | Upgrades every warning and info diagnostic to error |
| `coverage` | `main.go:1239` | Turns on the strict report path, as an independent boolean |
| `sync` | `main.go:1411` | All three of the effects below at once |

`sync --strict` does the most, and no single document says so. It sets the
checker's `Strict` field (`main.go:1313`), so the check phase upgrades warnings
to errors. It sets `CheckTestAnnotations` (`main.go:1347`), so the check phase
cross-references test annotations. And it sets effective strictness to
`zero-tolerance` (`main.go:1321-1328`).


Measured on a workspace with one orphan constraint in a Tier 2 spec:
`sync --strict` reports `FAIL check: 1 error(s)`, while
`sync --strictness annotation` reports `PASS check: 1 warning(s)`. The two flags
are not interchangeable even in the check phase, which reads no coverage
strictness at all.

Two further behaviors key on the `--strict` boolean rather than on the strict
path, and both differ between the commands:

`coverage --strict --strictness annotation` is a hard error, per spec-coverage
C-24: `--strict requires settings.strictness >= threshold`. The same combination
on `sync` passes silently, because `sync` has no equivalent coherence rule.
Verified by running both.

`coverage --scope <domain>` requires the literal `--strict` flag, per
spec-coverage C-23. Passing `--strictness zero-tolerance` instead is refused with
`error: --scope requires --strict`, even though it is the stricter mode. Verified
by running.

**Standing.** Open. This is the term the project must decide.

## `settings.strict` against `settings.strictness`

**Meaning.** Two distinct manifest keys whose names differ by three letters and
whose scopes do not overlap.

**Surfaces.** `settings.strict` is a boolean at `internal/manifest/types.go:49`.
It sets the checker's warnings-as-errors behavior and nothing else. It is read at
`main.go:711` (`check`), `main.go:1313` (`sync`), and `main.go:3003` (`watch`).

`settings.strictness` is the enum at `types.go:53`. It is read by the coverage
paths and by the `unreachable_annotation` severity routing.

`coverage` reads neither `settings.strict` nor the checker. A workspace that sets
`settings.strict: true` and expects coverage to tighten gets no change.

**Standing.** Settled behavior, poor naming. Nothing in the repository proposes
renaming either key, and both appear in shipped manifests, so this entry exists
to keep the two apart rather than to argue for a change.

## the strict path

**Meaning.** The internal routing that `BuildCoverageReportStrict` represents, as
distinct from the weaker `BuildCoverageReportWithResults`. On the strict path, a
missing results file is fatal, annotated criteria without a passing entry demote
across all tiers, and the strict diagnostics apply.

**Surfaces.** One rule, stated in spec-coverage C-31 and spec-sync C-07 and
implemented at `main.go:958`: an effective strictness of `threshold` or
`zero-tolerance` routes through the strict path, and so does a bare `--strict`.
Only `annotation` does not.

This is why the `SP-046` divergence is narrower than it first looks.
`coverage --strict` and `coverage --strictness zero-tolerance` produce the same
report. They differ only in whether the C-25 and C-26 exit gates are armed.

**Standing.** Settled. Both specs state it, and the code matches.

## PROPOSED: what `--strict` should mean

**This section is a proposal. A human signs off before `SP-046` is implemented.**

Two readings are live.

**Reading A, alias.** `--strict` is a spelling of `--strictness zero-tolerance`.
This is what `sync` does and what the name suggests to most readers.

**Reading B, axis.** `--strict` is an independent boolean that turns on the
strict report path, orthogonal to the level. This is what `coverage` does, and
spec-coverage C-24 has a coherence rule that only makes sense on this reading. A
flag that simply set the level would not need to reject a level.

### The recommendation

**Adopt Reading A.** The deciding argument is the direction in which each reading
fails.

Under Reading A, `coverage --strict` tightens. A workspace that passed goes red.
The operator sees a failure that was always real and had been hidden. This is
noisy, and it is fail-safe.

Under Reading B, `sync --strict` loosens. It drops from `zero-tolerance` to
whatever the manifest says, usually `threshold`. Gates that were red go green
with no message. That is the false-green class that spec-sync C-09 was written to
close and that `SP-046` was filed about. Choosing Reading B would reopen it
through the flag rather than through the report path.

The blast radius stated in `SP-046` points the same way. An adopter following the
documentation uses `--strict`, because that is the flag the docs and the Makefile
use. Under Reading A that adopter gets what the name promises.

### What Reading A costs

Naming the constraints that would have to change is the point of this section.
None of these are free.

**spec-coverage C-24 becomes a rewrite, not an edit.** C-24 makes
`--strict` with `settings.strictness: annotation` an error. Under Reading A the
combination is not incoherent, it is an override: `--strict` sets
`zero-tolerance`, and the manifest loses. Either C-24 is deleted, or it is
rewritten to say the flag wins. Deleting it removes a real guard against a
misconfiguration. Keeping the error contradicts the alias.

**spec-coverage C-31 loses a clause.** C-31 states that `--strict` enables the
strict path but does not override a manifest-set strictness level. Under Reading A
it does override it. That clause dies.

**spec-coverage C-23 needs a decision.** `--scope` currently requires the literal
`--strict`. Under Reading A that reads as "`--scope` requires zero-tolerance",
which is probably not what staged adoption wants, since `--scope` exists to let a
workspace enforce one domain at a time. C-23 likely wants rewording to "requires
the strict path", which is `--strictness threshold` or higher.

**`sync --strict` keeps a second job.** Reading A settles the coverage half.
It does not say what happens to the checker `Strict` and `CheckTestAnnotations`
effects at `main.go:1313` and `:1347`. Dropping them would loosen the check phase,
which is the same failure direction Reading A was chosen to avoid. Keeping them
means `--strict` is an alias for the level and also carries an unrelated effect,
which weakens the word "alias". This needs its own decision and is not resolved
here.

**`make dogfood-strict` starts failing, or starts being honest.** `SP-046`
records that the target runs `coverage --strict` in a tree with no root
`specter.yaml`, so today it resolves to `threshold`. Codes 2 and 3 have never
fired in Specter's own dogfooding. Under Reading A the target begins exercising
`zero-tolerance`. That is the point of the change, and it may turn the gate red on
first run.

### What Reading B costs

**The false-green returns.** `sync --strict` would stop meaning
`zero-tolerance`. Anyone relying on that today, including the workspace measured
above, silently loses a gate.

**spec-sync C-06 becomes wrong.** C-06 states plainly that `--strict` "remains as
an alias for `--strictness zero-tolerance`". It would have to be rewritten.

**The name keeps lying.** A flag called `--strict` that leaves the level at the
default is the reading that produced `SP-046` in the first place. Reading B is
defensible in code and hard to defend in documentation.

### What both readings must fix regardless

`docs/EXIT_CODES.md` describes a single resolution rule over three inputs and
calls `--strict` an alias. That is currently false for `coverage` under either
reading, because the document describes a resolution that no command implements
in full. It must be rewritten alongside the code.

---

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
| `--strict` on `coverage` | Independent boolean, level stays at manifest value | `sync` maps it to `zero-tolerance` | `SP-SP-046` |
| `--strict` in help text | `coverage` help at `main.go:1241`: "does not override a manifest-set level" | `sync` help at `main.go:1411`: "Alias for `--strictness zero-tolerance`" | `SP-SP-046` |
| effective strictness | `EXIT_CODES.md` states one resolution rule with `--strict` as an alias | Neither command implements that rule in full | `SP-SP-046` |
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
- The two conflicting help strings at `main.go:1241` and `:1411`.
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

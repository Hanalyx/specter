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
`make build` from `specter/`. Every commit since has touched documentation only,
confirmed by `git diff --stat d85a77e..HEAD`, so no code or spec has moved under
the measurements below.

Every claim about current behavior was checked by running `bin/specter` or by
reading source. [Appendix A](#appendix-a-how-each-claim-was-checked) says which,
claim by claim, **for the claims present at the second commit**.

**On the citations, and how to re-verify them.** This document carries 38
references into `cmd/specter/main.go` across 20 distinct line numbers, plus 17
others. That file is over 3,000 lines and roadmap phases 1B, 2C, 3A and 3C all
edit it. **A line number here is a claim with a shelf life.** Before trusting any
citation, check whether Go source has moved since the commit named above:
`git diff --stat <that commit>..HEAD -- '*.go'`. If it is non-empty, the
citations are unverified until someone re-runs them, and this document says
nothing about which ones moved. This document
has been amended six times since, and the appendix does not cover what those
amendments added. Those claims were independently verified on 2026-08-20 and the
appendix carries a list of them rather than a per-claim split.

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

**A Settled standing must rest on a measurement, and the entry must name the
cases that were run.** A citation shows a term is used somewhere; only a
measurement shows what it decides. This rule was learned twice on the `strict`
entry, which carried "Settled" while the line disproving it sat in the same
paragraph.

**The file did not meet its own rule, and now mostly does.** An audit on
2026-08-20 found that of the entries carrying a Settled standing, four named
distinguishing cases and about seven rested on citations. `tier` was falsified
in that audit. Two later passes measured ten more entries and falsified four:
the two annotation-channel entries, `the three levels`, and `the strict path`.

So the citation-resting set was not merely uninspected. **Five of the eleven
entries examined did not survive contact with a measurement.** What remains
uninspected is short and is listed at the end of Appendix D. Treat any Settled
standing that names no case as provisional, and promote it by running something
rather than by reading more.

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

First, severity promotion, **for the diagnostic families that reach the
promoter**. Those are reported as errors, so the command exits non-zero.
Measured on a Tier 2 spec with one orphan constraint: `check` reports
`0 error(s), 1 warning(s)` and exits 0, while `check --strict` reports
`1 error(s), 0 warning(s)` and exits 1.

The qualifier is load-bearing and is not a hedge. Measured on a workspace with
two `unreachable_annotation` warnings, `check --test --strict` reports
`0 error(s), 2 warning(s), 0 info` and exits 0. See "Promotion is incomplete"
below for why.

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
scans spec to test, so it knows.

**It is not the only command that knows.** An earlier draft said so, inferring
"only" from `check`'s blindness rather than checking the other commands.
`explain` computes the same fact per criterion at `cmd/specter/main.go:2706-2709`
and names it:

```
$ bin/specter explain mk
  COVERED   AC-01     This one is annotated in the test file    mk_test.go
  UNCOVERED AC-02     This one has no marker anywhere
```

`doctor` knows it in aggregate, reporting `mk: 50% coverage (T2 requires 80%)`
without naming the criterion.

**The fact the setting was created to enforce is computed in the commands that
report percentages, and is absent from the command that reports diagnostics.**
That conclusion is unchanged, because `explain` reports percentages too. What
changes is the scoping: `bugs/SP-SP-038` records that `explain` and `doctor`
ignore strictness, so an annotation rule landed in `coverage` alone would drift
from two commands computing the same fact independently.

**Surfaces.** A value outside the enum is rejected at parse for the manifest and
at the flag layer for the CLI, exit 1 either way. Measured, both layers:

```
$ bin/specter coverage --strictness bogus
error: --strictness "bogus" is not a valid value (allowed: annotation, threshold, zero-tolerance)
$ bin/specter coverage            # settings.strictness: bogus
error: invalid .../specter.yaml: settings.strictness: "bogus" is not a valid value ...
```

**The enum is declared three times, not once.** An earlier draft said the flag
and the key validate against a single declaration at
`internal/manifest/manifest.go:30`. They do not. That line declares
`validStrictnessValues` for the manifest. The flag validates against two
separately written map literals, `cmd/specter/main.go:924` for `coverage` and
`:1263` for `sync`.

The three agree today, which is why the behavior above is correct and the
mechanism claim was still wrong. It is latent rather than visible: add a fourth
level to `validStrictnessValues` and the manifest accepts it while both flags
reject it. That is the declaration-pair class `CLAUDE.md` says to close with a
parity test rather than reviewer attention, and it matters now because SSRB-104
retires both surfaces.

**Every `--strictness` row below is on a removal path.** The flag and the key are
accepted until v1.0.0 and removed there, per the RETIRING section above. The
tables describe what exists today, which is what a reader debugging today needs,
but nothing in them should be built on.

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
for `strictness` returns hits in five files, four excluding the test file:
`unreachable_annotation.go`, `unreachable_go.go`, `unreachable_ts.go`, and
`unreachable_py.go`. All four are in the `unreachable_annotation` family, and
every one routes through `routeSeverity`, which `unreachable_annotation.go:305`
defines. An earlier draft said three, which was true before `85dfd54` added the
Go reachability scanner. The conclusion the count supports is unaffected; the
number was stale, and a stale number inside a sentence whose purpose is to show
the work reads as evidence when it is not.

**Standing. Retiring.** The enum and its behavior are settled as a description,
and the setting is being replaced. See the next section. Nothing in this entry
is disputed; what is decided is that the concept it names is not the concept the
project wants.

## RETIRING: `strictness` becomes `annotation`

Direction set by the founder, 2026-08-18. Recorded here so the deprecation has
one description rather than several. **The shape was settled on 2026-08-19 and
is stated below**; this sentence previously said it was not settled by this
entry, which contradicted the four decisions 70 lines further down. The decision
record is `docs/ssrb/SSRB-104.md` and the request is `features/SP-006`.

### The plan

`settings.strictness` and `--strictness` are **preserved until v1.0.0 and
removed there**. Both keep their current behavior for the whole window, so no
existing workspace changes meaning on upgrade.

The replacement is `settings.annotation`. The v1.0.0 target carries two
sub-keys:

```yaml
settings:
  annotation:
    scope: test          # test | all
    permissive: false    # true warns where false fails
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
```

**What v0.15.0 actually ships is narrower**, by founder direction 2026-08-21 and
recorded as SSRB-104 section 7.7. Behavior is **test scope only**, and the
`scope` key **does not ship**:

```yaml
settings:
  annotation:
    permissive: false
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
```

`scope: all` requires reopening `docs/ssrb/SSRB-101.md`, which rejected
source-file annotation and has not been answered. With one of its two values
unimplemented, a `scope` key would decide nothing, and an inert manifest key is
the exact shape this document records three times in Part 5. Writing
`settings.annotation.scope` is rejected with a message naming the staging, not
with a bare unknown-field error, because SSRB-104 is public and names `all`.

Adding `scope` later is non-breaking: it arrives with default `test`.

**Four rules.**

1. **The annotation rule.** Every acceptance criterion must have a test. A
   criterion with no test fails, and the tier threshold does not excuse it.
2. **`scope` names which files must carry markers.** `test` requires them in
   test files. `all` requires them in test and source files.
3. **The tier thresholds set the allowed failure rate among criteria that do
   have tests.** `tier2: 80` means 80 percent must have a passing test.
4. **`permissive` sets severity, not scope.** It warns where the same
   configuration would otherwise fail.

**Manifest only. There is no `--annotation` flag and none is planned.**

`permissive` and `scope` are two axes rather than three points on one ladder.
An earlier draft of this section proposed the single ladder, and the sub-key
shape replaced it because a severity setting and a scope setting cannot be
positions on the same list.

**What the separation buys.** Today the coverage percentage cannot distinguish
a criterion with no test from a criterion whose test failed:

```
A: AC-04 has a test, and it failed     4 ACs  3 covered  75%  PASS
B: AC-04 has no test at all            4 ACs  3 covered  75%  PASS
```

Byte-identical for two problems that need different responses. Under the rules
above, A is a pass-rate question against the tier and B is a hard failure.

**`scope: all` does not ship in v0.15.0.** It requires reopening SSRB-101.

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

### The four questions, settled 2026-08-19

Recorded in short form. The reasoning is in `docs/ssrb/SSRB-104.md` section 7.

**The key keeps the name `annotation`,** with a conflict rule: when a manifest
carries both `strictness` and an `annotation` block during the window, the new
key wins and a warning names the ignored one. Renaming to dodge a temporary
collision would cost more than it saves, since `annotation` is the project's own
word for these markers.

**The value is `scope: test | all`.** `default` named a config position rather
than a behavior and would have become false the moment the default moved.

**Manifest only, no flag.** Flag and manifest cannot diverge if there is no
flag, which makes that class of divergence unreachable **for this setting**. It
does not resolve `bugs/SP-SP-046` or `bugs/SP-SP-047`, which are filed against
`--strict` and `settings.strict` and which SSRB-104 does not touch. Those stay
open and belong to `features/SP-005`. The cost is that no per-invocation
override exists.

**`permissive` is the severity mechanism.** None of the three existing patterns
fits. Tier routing would emit `info` for a Tier 3 criterion with no test, which
contradicts the rule that the tier does not excuse a missing test. Global
escalation belongs to `settings.strict`. Ladder routing is what retires.

### One consequence owed before v1.0.0

`vscode-extension/src/client.ts:180` passes `--strictness annotation` to
guarantee a parseable document on every run, because under a `threshold`
manifest plain `coverage` hard-fails without a results file and emits no JSON.
Manifest-only annotation gives it no replacement once `--strictness` is removed.

**The larger casualty is this repository's own build.** `Makefile:116-117` runs
`coverage --strictness annotation` and `sync --strictness annotation` for
`make dogfood`, and `Makefile:135` runs `coverage --strict` for
`make dogfood-strict`. There is no `specter.yaml` at the repository root. **Two
targets, one tree, no manifest, needing two different modes**, and
per-invocation control is the only mechanism that expresses that. SSRB-104
section 7.3 accepts "no per-invocation override" as the cost of manifest-only
without naming this as the thing it costs.

Other surfaces that instruct a user or an agent to pass the retiring flag, and
that outlive any deprecation window because they ship into other repositories:
`internal/manifest/ai_templates.go`, written by `init --ai` into an adopter's
tree; `internal/explain/annotation_reference.md`, embedded in the binary and
printed by `specter explain`; and `CHANGELOG.md`'s v0.13 migration instructions,
which are published and permanent and offer the flag as one of three remedies.

Three more that make removal a coordinated edit rather than a delete.
`docs/explainer/v0.13-sync-strict-coverage.md` carries 25 references and was
written for an external adopter, so its whole remedy set rests on the flag.
`scripts/smoketest_v013.sh` asserts that `--strictness` is registered.
`cmd/specter/cli_docs_parity_test.go` binds the `CLI_REFERENCE.md` flag tables to
the registered Cobra flags, so the docs and the code must move together or the
test fails.

**Checked and clean:** `.github/actions/specter-sync/action.yml` is a passthrough
that hardcodes no strictness, and `internal/manifest/hook.go` delegates to
`pre-push-check` and passes none.

The override exists to work around a contract violation. `spec-coverage` C-10
already claims `--json` emits a document in every state, and `bugs/SP-SP-032`
records that it does not. Making that claim true removes the need for the
override.

### One consequence settled, one proposed

**Exit codes need new triggers, and the replacement is proposed rather than
decided.** Codes 2 and 3 fire today only under `zero-tolerance` and go
unreachable when the ladder retires. `docs/ssrb/SSRB-104.md` section 7.5 sketches
exit 1 for a pass rate below the tier, exit 2 for a criterion with no test, and
exit 3 for an unmet approval gate, and ends that table with the sentence "This is
offered as an observation rather than a decision."

**An earlier draft of this entry listed that mapping as settled. It is not**, and
the error mattered: roadmap 1A4's parity test pins golden files per gate
combination, so a reader trusting a Settled label would have unblocked 1A4 and
pinned triggers nobody chose. `docs/EXIT_CODES.md` registers both codes as Stable
today, and under the sketch exit 2 stops meaning "an annotated criterion did not
pass" and starts meaning "a criterion has no test". An adopter branching on the
integer sees that change silently. That cost is unpriced in every document that
mentions it.

**Deferred criteria become a prerequisite.** The annotation rule fails a
criterion with no test, and Specter's own repository would fail three of fifteen
specs under it. `spec-manifest` AC-29 shows why that is not merely unfinished
work: it asserts that `git push --no-verify` bypasses the pre-push hook, a fact
about git rather than about Specter, so no honest test exists. Roadmap phase 3C
moves from optional to required, because the alternative recourse is a fake test.

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
A spec meeting its threshold still fails **the run** if one annotated criterion
failed, and the distinction matters to a machine consumer. Measured at exactly
50 against a threshold of 50 with one failed criterion: the process exits 2
while the row prints `PASS`, the summary prints `1 specs: 1 passing, 0 failing`,
and `--json` carries `"passes_threshold": true` and `"failing": 0`.

That is not a reporting defect. The failed criterion **was** demoted, which is
why the spec reads 50 percent rather than 100. It then cleared a threshold of
50 honestly. The exit code answers a different question from the report cell,
and both answers are correct. Read `passes_threshold` as "did this spec meet its
tier threshold", not as "did this run pass".

**Standing.** Settled as concepts, with one correction. An earlier draft said
that `threshold` and `zero-tolerance` "run the same report path, so the
difference between them is not what gets demoted. It is which exit-code gates
are armed after demotion." **The second sentence is false.** They share the
report path, and zero-tolerance additionally applies a demotion that threshold
does not: an approval-gate violation moves from covered to uncovered.

`DemoteApprovalGateViolations` runs only under zero-tolerance, gated at
`cmd/specter/main.go:1074-1076` and implemented at
`internal/coverage/zero_tolerance.go:71`. Measured on one fixture, one results
file marking all three criteria `passed`, with `AC-01` carrying
`approval_gate: true` and no `approval_date`:

```
$ bin/specter coverage
th-spec   T3   3   3   100%   PASS
exit 0

$ bin/specter coverage --strictness zero-tolerance
th-spec   T3   3   2   66%    PASS
  uncovered: AC-01
error: zero-tolerance strictness — 1 AC(s) carry approval_gate=true with unset approval_date
exit 3
```

Same inputs, different coverage percentage. So **both** what demotes and which
gates arm differ between the two levels. The comment at
`zero_tolerance.go:65-70` records why: v0.11.0 emitted the exit code while
leaving the report identical to threshold mode, so an operator saw PASS on a run
that exited 3. GH #94 was filed against that, and this function is the fix.

## the strict path

**Meaning.** The internal routing that `BuildCoverageReportStrict` represents, as
distinct from the weaker `BuildCoverageReportWithResults`. On the strict path, a
missing results file is fatal, annotated criteria without a passing entry demote
across all tiers, and the strict diagnostics apply.

This is a third concept, and it needs its own name. It is neither the severity
switch nor the ladder. It is what `coverage --strict` actually selects, and it is
reachable from two ladder positions as well.

**Surfaces.** The strict path runs when effective strictness is `threshold` or
`zero-tolerance`. **Nothing else selects it.**

An earlier draft added "or when the `--strict` boolean is set" and called that
one rule stated by both specs. **The disjunct is dead on both surfaces**, for
two different reasons.

On `coverage`, `main.go:958` really does read
`strict || threshold || zero-tolerance`. But the C-24 guard at `main.go:943`
tests the same `effectiveStrictness` and exits first, so the only configuration
where the disjunct could decide is refused before the line is reached.

On `sync`, `internal/sync/sync.go:217-218` consults the boolean only when
`input.Strictness == ""`, and `cmd/specter/main.go:1345` always passes a
non-empty value, because the manifest loader fills it. `sync --strict` alone
still reaches the strict path, through `main.go:1321-1328`, which resolves the
bare flag to `zero-tolerance` before `RunSync` is called.

Measured on a fixture with no results file, which makes the strict path fatal
and so discriminates:

```
coverage --strictness annotation             exit 0
coverage --strict --strictness annotation    exit 1   error: --strict requires settings.strictness >= threshold
coverage --strictness threshold              exit 1   strictness "threshold" requires .specter-results.json
sync --strictness annotation                 exit 0   PASS coverage
sync --strict --strictness annotation        exit 0   PASS coverage
sync --strict                                exit 1   FAIL coverage: strictness "zero-tolerance" requires .specter-results.json
sync --strictness threshold                  exit 1   FAIL coverage: strictness "threshold" requires .specter-results.json
```

Row 5 is the falsifier: the boolean is set and the strict path does not run.
Row 6 shows why `sync --strict` alone still works, and the error names
`strictness "zero-tolerance"` rather than the boolean.

This also supplies the mechanism behind the four-row table measured later in
this document. `coverage --strict` never changes a coverage verdict because the
branch that would make it matter is unreachable, not because the effects happen
to coincide.

Same dead-branch shape as `tier`: a disjunct that reads as a rule and decides
nothing.

**The specs were checked, because the earlier standing line claimed both stated
the rule.** Neither states it as written. `specs/spec-coverage.spec.yaml:192`
(C-31) states the two-disjunct rule for `coverage` and separately records that
`--strict` with `annotation` "remains a C-24 error", which is consistent with
the code. `specs/spec-sync.spec.yaml:72` (C-07) says only "Under any strict mode
(`threshold` or `zero-tolerance`)" and never mentions the boolean. So the
cross-command generalization was this document's own, and there is no
spec-versus-code disagreement to file.

**Standing. Open.** The routing rule holds in its corrected form. What was
falsified is the claim that the `--strict` boolean is a third way to select the
path. The term itself appears in no user-facing output, so it is an internal
name.

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

**Read it against the RETIRING section above, which is newer and which it
predates.** Every concrete remedy below is written in terms of `--strictness`
and `settings.strictness`, which `docs/ssrb/SSRB-104.md` retires at v1.0.0. The
proposal's core, that `--strict` means the severity switch and one word should
do one job, survives and is tracked as `features/SP-005`. Its mechanics do not:
where it says the ladder is reachable through `--strictness`, read that as the
ladder reachable through whatever survives the retirement. Where it tells
`make dogfood-strict` to call `--strictness zero-tolerance`, read the tier
thresholds instead.

Kept rather than deleted because the reasoning is still the reasoning, and
because a reader tracing how `SP-005` was arrived at needs it.

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

**The distinction survives nowhere.** An earlier draft of this entry claimed
that spec-coverage C-28's per-criterion hint separates the two cases. It does
not. C-28 fires when a criterion has a source annotation and **no matching
entry** in the results file (`specs/spec-coverage.spec.yaml:177`), and the code
agrees: the guard is `hasEntry[key]` at `internal/coverage/coverage.go:174`. A
criterion whose test ran and failed **has** an entry, so no hint fires. "No
matching pass" is the hint's message text, not its trigger.

Measured: two fixtures, one with a criterion whose test genuinely fails and one
with a criterion carrying no test at all, produce byte-identical stdout and
stderr, and neither emits a hint. The table conflates them and so do the
diagnostics. That is the gap `docs/ssrb/SSRB-104.md` exists to close.

**Standing.** Settled in behavior. The word "demote" appears throughout the specs
and in no user-facing output, so it is an internal term. That is worth knowing
before writing it into a user document.

## spec coverage against test coverage

**Meaning.** **Spec coverage** is the fraction of a spec's acceptance criteria
that have satisfying evidence. **Test coverage**, in the ordinary sense of lines
or branches executed, is not something Specter measures at all.

**Surfaces.** The tool never reads a coverage profile. `.specter-results.json`
carries pass and fail status per criterion, not execution data. No reader exists
for `coverprofile`, `lcov`, `cobertura`, or `coverage.out` anywhere in the Go or
TypeScript sources.

**Not every percentage Specter prints is spec coverage**, which an earlier draft
claimed. `BuildSummaryHeader` emits a per-tier rollup at
`internal/coverage/coverage.go:845` computing `t.passing / t.total`, the fraction
of specs in a tier that pass their threshold. That is a spec pass rate, over a
different denominator. One header carries both meanings at once:

```
Spec Coverage Report — 1 specs · 50% avg coverage
  Tier 2: 0/1 passing (0%)
```

50 percent is spec coverage. 0 percent is a pass rate over specs. Same block,
same run. The entry exists to stop an operator misreading a Specter percentage,
and the absolute claim licensed misreading the one printed directly above the
table.

**Standing. Settled on the distinction, corrected on the quantifier.** Recorded
here because the two are named similarly and
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

**Standing. Settled, and measured on four cases** rather than cited. A duplicate
pair with `passed` first then `failed`, and the same two rows reversed, both
reported `uncovered: AC-01`, so worst status wins and row order does not matter.
A `status: "flaky"` entry ranked as not-passed and produced:

```
warning: .specter-results.json contains 1 entries with status="flaky" — not a recognized status (passed|failed|skipped|errored); treated as not-passed
```

The fixed-path trap reproduced. `ingest --output results/run.json` wrote three
entries, and `coverage --strict` in the same directory answered:

```
error: --strict requires .specter-results.json — run 'specter ingest' first
```

The fixed path is a real constraint that nothing documents
as a decision, but nothing contests it either.

## annotation, and its two channels

**Meaning.** A marker in a test that names the criterion the test covers. Specter
recognizes two channels for it, and they are not interchangeable.

**Surfaces.**

The **source channel** is a comment in the test file: `// @spec <id>` and
`// @ac AC-NN`. It is found by reading the file. It is what
`--strictness annotation` counts, and it is what `check --test` cross-references.

The **runner-visible channel** is what `ingest` can extract from a test runner's
report. **It is defined by which field the marker sits in, not by whether the
marker is visible in the output.** An earlier draft of this entry said the
channel is "a `<spec-id>/AC-NN` token that appears in the test runner's own
output," and that description is falsified by measurement below.

Two forms, and they are not variants of one rule:

- **Convention A** puts a `<spec-id>/AC-NN` token in the **test name or the
  JUnit classname** (`internal/ingest/annotations.go:31-35`). Python cannot use
  it, because function names cannot contain a slash.
- **Convention B** prints `// @spec <id>` **and** `// @ac AC-NN` from the test
  body. **Both markers are required.** `internal/ingest/junit.go:111-113` drops
  the case when either is empty, and `internal/ingest/gotest.go:89-93` takes the
  body pair only when both were seen.

A bare `<spec-id>/AC-NN` token printed into the body matches neither, and is
dropped. Measured, on a Go fixture with one test per form:

```
Scanned 5 test cases; extracted 2 (spec_id, ac_id) pairs; dropped 3 with no runner-visible annotation.
  dropped: TestConvA — no (spec_id, ac_id) pair found in name, classname, or output
  dropped: TestConvBBareToken — no (spec_id, ac_id) pair found in name, classname, or output
  dropped: TestSourceOnly — no (spec_id, ac_id) pair found in name, classname, or output
```

The token in `TestConvBBareToken` **is** in the runner's output, and `go test
-json` carries it in an Output event. It is dropped anyway, because `ingest`
applies the token regex only to the name and the classname. That is why the
visibility framing was wrong.

The distinction is the single most expensive one in this document. A source
annotation alone satisfies `annotation` and satisfies nothing above it, because
`ingest` never sees it and so no results entry exists for the pair. That is the
demotion case, and it is what `unreachable_annotation` is supposed to warn about
before it happens.

**Standing. Open.** Both channels are implemented, and the description above is
now measured rather than cited. What is open is that `check --test` accepts a
wider grammar than `ingest` does, so the warning does not cover every form that
demotes. See the next entry and `bugs/SP-SP-050`.

## `unreachable_annotation`

**Meaning.** A `check` diagnostic naming a source annotation whose test produces
no runner-visible token, so it would demote under the strict path.

**Surfaces.** Severity routes through the strictness level: suppressed under
`annotation`, warning under `threshold`, error under `zero-tolerance`. Suppressed
per file by `// @reachable manual`, or `# @reachable manual` in Python.

A softer sibling, `unreachable_annotation_unknown`, fires when the scanner cannot
recognize the test shape at all. It is always a warning and never fails a gate.

The severity routing was measured, on one workspace with only
`settings.strictness` edited between runs:

```
annotation       All 1 specs passed structural checks.        exit 0
threshold        0 error(s), 1 warning(s), 0 info             exit 0
zero-tolerance   1 error(s), 0 warning(s), 0 info             exit 1
```

**Standing. Open, on coverage rather than on severity.** The routing above holds
and the `@reachable manual` suppression works in both Go and Python. What is open
is that **the diagnostic misses forms that demote.** Its reachability scanner
accepts either marker alone (`unreachable_go.go:452-460`,
`unreachable_py.go:122-127`, `unreachable_ts.go:165-168`) where `ingest` requires
both, and accepts a bare pair in any string-literal argument
(`unreachable_go.go:441-446`) where `ingest` reads only the name and classname.

Measured: on a four-criterion fixture, `check --test` warned about `AC-04` and
stayed silent on `AC-03`, while `coverage --strictness threshold` reported
`uncovered: AC-03, AC-04`. Under `zero-tolerance` the diagnostic is an error, so
this is a gate staying silent on the input it exists to catch. Filed as
`bugs/SP-SP-050`.

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

**Standing. Settled, and measured across six configurations**, not the one case
quoted above. On a fixture at 66.7 percent with three criteria and two covered:
no manifest and no spec value resolves to 50 and passes; `tier3: 90` resolves to
90 and fails with exit 1, and `sync` fails its coverage phase identically;
`tier3: 0` resolves to 0 and passes, with `--json` reporting `"threshold": 0`; a
spec `coverage_threshold: 67` over `tier3: 0` resolves to 67 and fails; a spec
value of 66 passes, which is the floor rule; and a spec value of 0 over
`tier3: 90` resolves to 0 and passes. Both zero cases behave as C-36 requires at
both layers.

C-35 and C-36 both landed to close defects where displayed and compared numbers
disagreed. The display column showed 66 in every row while the stored value was
66.7.

---

# Part 3: structural terms

## tier

**Meaning.** An integer from 1 to 3 declaring how much rigor a spec is held to.
Tier 1 is the strictest.

A tier is read in three different registers, and the counts below are closed
lists **for the Go binary**. The VS Code extension reads a tier three more
times and is excluded on purpose, per this document's Scope: it changes no gate
result. Those three are named at the end of this entry so the exclusion is
visible rather than silent.

**Enforcement, exactly three.** The default coverage threshold, the severity of
one diagnostic (`orphan_constraint`), and the Tier 1 evidence rule that a
criterion needs a passing result and not only an annotation. The third is a
coverage rule, not a severity. Citations are in Surfaces below, kept in one
place so the line numbers have one place to drift.

**Emission, exactly one.** A tier decides whether `tier_conflict` exists at all.
`internal/manifest/tier_conflict.go:37` skips a spec when `spec.Tier == 0` or
when it equals the override, so the same manifest produces the warning against a
Tier 3 spec and nothing against a Tier 1 spec. This is not enforcement, because
it gates nothing: `check` exits 0 with the warning present, and `check --strict`
leaves it a warning while promoting the orphan, which is the promotion gap
recorded in Part 5. It is not presentation either. It is its own register, and
the two-bucket split an earlier draft used had no slot for it.

**Presentation, the rest.** A tier sorts entries within a coverage bucket
(`internal/coverage/coverage.go:792`), groups the summary rollup (`:826`),
prints the `T%-5d` column in the coverage table (`cmd/specter/main.go:1189`), is
printed twice by `explain` (`:2701`, `:2747`), and is printed by `doctor`
(`:2273-2275`), which re-derives the threshold from the same map only in order
to display it. None of these decide a verdict.

**Surfaces.** Declared per spec as `tier:`. **Nothing resolves it.** Every live
consumer reads `spec.Tier` raw. The complete list for the Go binary, which
matters because it is what anyone scoping `bugs/SP-SP-049` has to change:

| Site | Register |
|---|---|
| `internal/coverage/coverage.go:517` | the Tier 1 evidence rule |
| `internal/coverage/coverage.go:542` | the threshold lookup |
| `internal/checker/check.go:189` | orphan severity |
| `internal/manifest/tier_conflict.go:37,42,46` | whether `tier_conflict` is emitted |
| `internal/coverage/coverage.go:571` | the `tier` field in `--json` |
| `cmd/specter/main.go:2701`, `:2747` | `explain`, printed |
| `internal/schema/validate.go:93` | range validation, decides nothing |

An earlier draft gave the first three plus `:571` and called that "every live
consumer." It was an absolute quantifier over an incomplete list, which is the
worst shape a claim can take in a document meant for scoping. `explain` and the
conflict check were missing.

The only tier-keyed severity map in the tree is `orphanSeverityByTier` at
`check.go:50`. `duplicate_ac_id` refuses tier routing outright
(`internal/checker/duplicate_ac_id.go:12-13`).

**There is a resolver, and it is dead.** `ResolveTier` in
`internal/manifest/tier.go` implements a four-step cascade: an explicit spec
`tier:` greater than 0, then the tier of a domain in `specter.yaml` that lists
the spec, then `system.tier`, then a hardcoded default of 2. It has two call
sites and **neither is reachable**. `types.go:164` sits inside
`ResolveTierWithOverrides`, which has no caller. `registry.go:15` sits inside
`BuildRegistryFromSpecs`, whose only **non-test** caller is `UpdateRegistry`,
which has no caller at all, tests included. The qualifier matters in a document
whose thesis is that you must walk the call graph:
`internal/manifest/manifest_test.go:310` and `:334` both call
`BuildRegistryFromSpecs`, so "only caller" without it is false.
Confirmed by running: `sync` on a workspace whose `specter.yaml`
has no `registry:` block leaves the manifest byte-identical, because no registry
is ever regenerated.

So the cascade is dead in full, step 1 included. Two independent facts each make
a domain tier and `system.tier` inert, and it is worth keeping them apart:

1. **Nothing calls the cascade.** Even step 1 never runs.
2. **The schema would kill steps 2 to 4 anyway.** `tier` is a required property
   with enum `[1, 2, 3]` in `internal/parser/spec-schema.json`, so `specTier > 0`
   would hold for every spec that parses. Measured: a spec omitting `tier` fails
   with `Missing required field 'tier'`, which is a Go-side translation at
   `internal/parser/humanize.go:59`, and `tier: 0` fails the schema validator
   with `value must be one of 1, 2, 3`.

An earlier draft of this entry named only the second fact, and so described
`ResolveTier` as the resolver with three dead branches. It is not the resolver at
all.

That is not cosmetic. `specter init` writes both fields into every new workspace
and `docs/GETTING_STARTED.md` shows them, so a team setting tiers per domain gets
silence and no effect. The only full manifest example,
`testdata/manifests/valid/full.specter.yaml`, declares tier at four levels:
`system.tier`, three domain tiers, per-entry tiers in `registry`, and the specs
themselves. Three of the four decide nothing, and the `registry` block is
weaker still: it parses into `Manifest.Registry` and no command reads it or
regenerates it. It is the same shape as
`tier_overrides` in the next entry, which is filed as `bugs/SP-SP-001`, except
that this one emits no warning at all.

Scoped precisely: this is about tier **resolution**. Domains still drive
`--scope`, and that path reads `domain.Specs` (`cmd/specter/main.go:986-989`)
without ever reading `domain.Tier`. Measured: `coverage --strict --scope
payments --json` on a fixture whose `payments` domain is Tier 1 still reports
`"tier": 3, "threshold": 50` for a spec declaring `tier: 3`. The only other
reader of `domain.Tier` is `DomainCoverage` in `internal/manifest/domain.go:48`,
which also has no caller. `domain.Tier` is read nowhere live.

**Under consideration, and not current behavior.** `bugs/SP-SP-049` records the
defect and carries a recommendation: relax `spec.tier` to optional so a spec
inherits its domain's tier, remove `system.tier`, and report a spec whose
declared tier disagrees with its domain's. The argument is that per-domain tier
assignment is a capability the manifest already has a field for, and that
`settings.coverage.tierN` becomes the allowed failure rate among criteria that
have tests once `settings.annotation` lands.

**The recommendation is what is undecided here, not `settings.annotation`.** The
annotation model is settled and accepted (`docs/ssrb/SSRB-104.md`, status
ACCEPT, shape settled 2026-08-19) and Part 2 of this document states it. It is
not yet implemented: `internal/manifest/types.go` has no `Annotation` field, and
`annotation` exists today only as a `strictness` enum value at `manifest.go:30`.
The SP-SP-049 recommendation is a separate schema change that needs its own
SSRB, and that brief should settle `tier_overrides` in the same pass.

**Excluded on purpose: the VS Code extension.** Three more reads of a tier live
there, and this document's Scope covers terms where getting it wrong changes a
gate result. None of these do, so they are named here rather than counted above.
Confirmed by running the extension's own jest suite with `-t` filters, not by
driving a live VS Code window:

- `vscode-extension/src/coverage.ts:529`, `classifyNotification`. A Tier 3 spec
  dropping below threshold gets the status bar only. Tiers 1 and 2 get a toast.
- `vscode-extension/src/coverage.ts:586`, `buildFileDecoration`. A failing Tier 1
  spec renders red, a failing Tier 2 yellow.
- `vscode-extension/src/extension.ts:658-659`, `updateStatusBar`. The warning
  color trips only for a failing Tier 1 or Tier 2 spec, so a failing Tier 3 does
  not trip it.

**Standing. Open.** This entry has been wrong three times, in the same
direction, and the third is the one that best shows the shape.

The first draft marked it Settled after reading `ResolveTier`'s body. That read
was accurate about the function and wrong about **what the function can
receive**, because the schema constraint that kills three branches lives outside
the function.

The second draft corrected that and still called `ResolveTier` the resolver. That
was wrong about **whether the function runs at all**, because reachability lives
outside the function too. Both drafts cited a body and neither followed the call
graph. A citation shows a term is used somewhere; only a measurement shows what
it decides, and for a function the first measurement is whether anything calls
it.

The third draft fixed the mechanism and then wrote "every live consumer reads
`spec.Tier` raw" over a list of four, when there are seven. That is a different
error from the first two and a worse one for this document's purpose: an
absolute quantifier is what a reader scoping `bugs/SP-SP-049` would trust to be
closed. A list that is merely incomplete misleads only about itself. A list
labeled **every** misleads about the tree.

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

**Standing. Settled in code and in C-02, measured across all three tiers**, not
the Tier 2 pair alone. On three specs with one orphan constraint each, `check`
reported `error` for Tier 1, `warn` for Tier 2, and `info` for Tier 3.
`check --strict` reported all three as errors, which is the warning-and-info
promotion filed as `SP-SP-047`. The `enforcement:` override works in both
directions: a Tier 1 orphan with `enforcement: info` printed as info, and a Tier
3 orphan with `enforcement: error` printed as error.

Open as a specification defect,
because the spec contradicts itself and Specter's own methodology says the spec
decides.

## structural conflict

**Meaning.** A downstream spec's acceptance criterion contradicts an upstream
spec's constraint. The canonical shape: an upstream constraint requires a field,
and a downstream criterion handles that field being absent.

**Surfaces.** Detected by `check` across the dependency graph. Default severity
error, overridable by the **upstream** constraint's `enforcement:`.

Which constraint owns the override is easy to get wrong, because the diagnostic
is attributed to the downstream spec. `internal/checker/check.go:252-254` reads
`constraint.Enforcement` from the upstream loop. Measured: setting
`enforcement: warning` on the upstream constraint dropped the diagnostic to a
warning and the run to exit 0, while setting `enforcement: info` on a downstream
constraint changed nothing.

**Standing.** Settled as a definition, and one of its two cited defects is now
reproduced rather than cited.

`bugs/SP-SP-014`, the diagnostic naming two specs as one, **reproduces**. On a
fixture with deliberately distinct constraint IDs:

```
error [structural_conflict] down-spec C-07 (technical): Structural conflict: "up-spec" constraint C-07 requires "email" but "down-spec" AC-01 handles it as absent
```

The header reads `down-spec C-07`. `C-07` belongs to `up-spec`, and `down-spec`
has no `C-07` at all. `check.go:257-259` sets `SpecID` from the downstream spec
and `ConstraintID` from the upstream constraint.

`bugs/SP-SP-004`, the false positives against spec-check C-05, is **now measured
too**, on 2026-08-21. Crossing all 111 subject-bearing constraints against all
379 acceptance criteria in Specter's own corpus gives 31 fires over 42069 pairs,
an upper bound over any conceivable graph. **On the real graph the count is 0**,
and `check --json` carries no `structural_conflict` entry, because 22 of the 31
are same-spec pairs the resolver rejects as circular and the other 9 sit on
edges that do not exist.

All 31 were classified and none is a genuine conflict. The clearest case:
`spec-commits` C-06 reads "CI MUST validate the PR title", so the subject is the
two letters `CI`, and two of its fires match `ci` inside "produ**ci**ng" and
"exer**ci**ses". The rule is not doing word matching.

The reachable count being 0 does not make the defect theoretical. An
eleven-line two-spec fixture still fails a build at HEAD, and C-05 is scoped to
anyone's specs.

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

**Standing. Settled, and scheduled for deletion.** Roadmap 3C7 removes the field
this cycle with a migration drop rule.

An earlier draft of this entry summarized 3C7's grounds as "nothing reads it."
**That summary was wrong, and it was wrong about this document, not about the
plan.** The field has live in-process readers: `internal/reverse/reverse.go:235`
builds a per-spec warning from it and `:241` feeds `Summary.GapsDetected`,
printed at `cmd/specter/main.go:1617`. Measured on a zod fixture:

```
$ bin/specter reverse ts --dry-run --adapter typescript
          gap: true
  warning: 7 gap(s) detected — constraints without test coverage
Found 7 constraints, 1 assertion, 7 gaps across 2 files.
```

What is true is narrower: nothing reads a persisted `gap: true` back from a
parsed spec. No reader exists in `internal/coverage`, `internal/checker`, or
`internal/explain`. The roadmap entry already reflects this correctly. It prices
the reverse-compiler edits and records that the count survives because it can be
computed rather than persisted. Anyone scoping 3C7 should read the roadmap entry
rather than the one-line summary that used to sit here.

Noted in passing, and not part of any claim above: `reverse` tells the user to
run `specter explain` to triage gaps, and `explain` reads no gap data.

Do not write `gap: true` into a spec on the strength of the label above. The
prose use should be avoided in technical documents, because it collides with a
schema field that means something narrower.

---

# Part 4: reserved, but not defined

These names appear in Specter documents without a definition anywhere. They are
listed so nobody mistakes a reservation for a settled term.

**Two of the three are scheduled to become real this cycle**, and this section
said nothing about that until 2026-08-20. Roadmap phase 3A builds evidence
streams and phase 3C builds deferred criteria, which `docs/ssrb/SSRB-104.md`
section 7.6 promotes from optional to a prerequisite. A reader who took
"undefined" to mean "not coming" would be wrong on both. The entries below
describe what exists today, which is nothing, and each names the item that
changes it.

**evidence stream.** `docs/EXIT_CODES.md:181` reserves exit codes 20 to 29 for
"Evidence stream validation". No code in that band is allocated, no command
emits one, and no spec defines what an evidence stream is. The band is an
allocation, not a feature. **Roadmap 3A builds it**, and `docs/EXIT_CODES.md`
already maps roadmap 3B4 into the band, so the reservation has a scheduled
occupant.

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
| Convention B grammar | `check --test`: either `@spec` or `@ac` alone marks a test reachable, and a bare pair in any string-literal argument counts | `ingest`: both markers required, and a bare pair read only from the name or classname | `SP-SP-050` |
| `gap` readers | This document, earlier draft: "nothing reads it" | `reverse.go:235` and `:241` read it in process, and `main.go:1617` prints the count. Roadmap 3C7 had it right; the summary here did not | Not filed; a defect in this document |
| `tier_overrides` | Warning says "using override" | No caller applies it | `SP-SP-001` |
| tier cascade | `ResolveTier` inherits from a domain tier, then `system.tier`, then a default of 2 | Nothing calls it. Both call sites sit in functions with no callers, and every live consumer reads `spec.Tier` raw | `SP-SP-049` |
| `registry` block | Parsed into `Manifest.Registry`, and `full.specter.yaml` carries per-entry tiers | No command reads it or regenerates it | `SP-SP-049` |
| `dangling_reference` | Parse: an undeclared constraint reference | Resolve: an unknown `depends_on` target | Not filed |
| Tier 3 orphan severity | `spec-check` C-02 and the code: `info` | `spec-check` objective scope, line 36: `warning` | `SP-SP-003` |
| sync's default strictness | Code comment at `main.go:1318`: "ultimate default is `annotation`" | Behavior: `threshold`, because the manifest loader fills it in | Not filed |
| `settings.strict` against `--strict` on `sync` | The key reaches the severity switch at `main.go:1313` | It never reaches the ladder: `:1323` reads the flag alone, so key and flag are not interchangeable | `SP-SP-047` |
| `--strict` promotion coverage | Promotes the families `CheckSpecs` built, at `internal/checker/check.go:150` | Skips `taDiags` and `uaDiags`, appended after that returns at `main.go:744` and `:773` | `SP-SP-047` |
| spec-manifest C-11 against the code | C-11: strict upgrades "warning-severity" diagnostics | The code upgrades warning **and** info | `SP-SP-047` |

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
  as its comment, for the four-step tier cascade. **This is the entry that failed
  twice.** Reading the body established the cascade correctly and established
  nothing about whether the cascade runs. See Appendix B for the call-graph walk
  that settled it, and the `tier` entry for what the two failures have in common.
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

## Appendix B: claims added after the second commit

Appendix A covers the claims present at commit `e9d59e3`. This document has been
amended six times since, and those amendments added measured claims the appendix
does not list. An independent verification pass on 2026-08-20 checked each of the
following and found every one accurate. They are recorded here as a list rather
than a per-claim split, and the pass that confirmed them is named so the
provenance is traceable.

**Confirmed by running the binary:**

- The `settings.strict` scan-gating table in the `strict` entry.
- The `check --test` against `coverage` missing-marker pair in the `strictness`
  entry.
- The A/B conflation table. Byte-identity confirmed with `cmp` on both streams.
- The `SP-SP-032` consequence behind `client.ts:180`: under a threshold manifest
  with no results file, `coverage --json` writes zero bytes to stdout and exits 1,
  while `coverage --json --strictness annotation` writes a document.
- "Three of fifteen specs would fail" is exact: `spec-diff` AC-11,
  `spec-manifest` AC-29, `spec-reverse` AC-18, and no others.
- The `check --test --strict` promotion gap, previously listed in Appendix A as
  read-only.

**Confirmed by reading source:**

- The four discovery citations in the command-reach table: `main.go:723`,
  `:886`, `:1275`, `:2223`.
- `vscode-extension/src/client.ts:180`.

**Corrected by the same pass, and now fixed above:** the claim that
spec-coverage C-28's hint distinguishes a demoted criterion from an unannotated
one. C-28 fires on a missing **entry**, not a missing pass, so it does not.

## Appendix C: the tier pass, 2026-08-20

The `tier` entry was rewritten twice in one day and got a dedicated verification
pass. Recorded separately because the entry is the document's worked example of
its own rule, and because the first pass missed what the second one found.

**Confirmed by running the binary** (built with `make build`, `bin/specter`,
version 0.14.1, fixtures in a scratch directory):

- Both parse failures, as exact substrings. Omitting `tier`:
  `error [required] spec: Missing required field 'tier'`. Setting `tier: 0`:
  `error [validation] spec.tier: at '/spec/tier': value must be one of 1, 2, 3`.
  Same on `parse` and on `check`.
- A manifest declaring `system.tier: 1` **and** a domain at `tier: 1` listing the
  spec, against a spec declaring `tier: 3`. `coverage --strictness annotation
  --json` returns `"tier": 3, "threshold": 50`, and `check` reports the orphan at
  `info`, which is Tier 3 severity and so confirms independently of the JSON that
  the domain tier was **not** applied. An earlier wording of this bullet said it
  "confirms the domain tier", which reads as the opposite of what the fixture
  showed.
- No warning on any of `check`, `coverage`, or `sync`. Both streams carried
  nothing about the disagreement.
- `coverage --strict --scope payments --json` on that fixture, still reporting
  `"tier": 3, "threshold": 50` where a domain-tier read would have given 100.
- `specter init` in an empty directory, which writes `system.tier: 2` and
  `domains.default.tier: 2`.
- `sync` on a workspace whose `specter.yaml` has no `registry:` block, leaving
  the manifest byte-identical. This is what establishes that no command
  regenerates the registry.

**Confirmed by walking the call graph**, which is the step both earlier drafts
skipped:

- `ResolveTier` has two call sites, `registry.go:15` and `types.go:164`.
- `types.go:164` is inside `ResolveTierWithOverrides`, which has no caller.
- `registry.go:15` is inside `BuildRegistryFromSpecs`, whose only non-test
  caller is `UpdateRegistry`, which has no caller at all.
  `manifest_test.go:310` and `:334` call `BuildRegistryFromSpecs` directly.
- `DomainCoverage` at `domain.go:48`, the only other reader of `domain.Tier`,
  also has no caller.
- The live consumers read `spec.Tier` raw. The complete Go-binary list is in the
  entry itself and has seven rows, not the four an earlier draft gave.
  `explain` at `main.go:2701` and `:2747` and the conflict check at
  `tier_conflict.go:37` were the ones missing.
- `orphanSeverityByTier` at `check.go:50` is the only tier-keyed severity map in
  the tree. `duplicate_ac_id.go:12-13` refuses tier routing.

**Read as source, not measured, and flagged as such:** what a tier-0 spec would
do if the schema allowed one. `CoverageThresholds()` at `types.go:126-139`
populates keys 1, 2 and 3 only, so `thresholds[0]` misses and
`coverage.go:542-545` falls back to 80. `orphanSeverityByTier[0]` returns the
empty string and the guard at `check.go:189-192` turns it into `warning`. Both
are near-Tier-2 behavior. A tier-0 spec would also vanish from the summary
rollup, because `BuildSummaryHeader` loops `for tier := 1; tier <= 3` at
`coverage.go:838`. This could not be run, because the schema blocks
`tier: 0` at the CLI and running it would have meant editing the repository.
`bugs/SP-SP-049` carries the same flag, and it is the reason the recommendation
there costs more than it first appeared to.

## Appendix D: the Settled-entry pass, 2026-08-20

The document admitted, in the Scope section, that about seven entries carrying a
Settled standing rested on citations rather than on cases that were run. Two
independent passes inspected **ten** Settled entries, which is more than the
citation-resting set because some already named a case and were re-run anyway.
**Four were falsified or partially falsified.** With `tier` from the earlier
audit, five of eleven examined did not survive.

That ratio is the argument for the admission staying in Scope rather than being
quietly dropped.

**Falsified, and corrected above:**

- **The runner-visible channel was defined by output visibility.** It is defined
  by which field the marker sits in. A bare `<spec-id>/AC-NN` token printed into
  a test body appears in `go test -json` output and is dropped anyway. Convention
  B needs both markers, not either.
- **`threshold` and `zero-tolerance` were said to differ only in which gates
  arm.** Zero-tolerance also applies a demotion threshold does not. Same fixture,
  same results file, 100 percent against 66 percent.
- **This document said `gap` was scheduled for deletion "on the grounds that
  nothing reads it."** `reverse` reads it in process. The roadmap entry was
  already correct and prices the reverse-compiler edits; the one-line summary
  here was the defect. Recorded because the first correction written here
  over-reached and asserted the plan was under-scoped, which the roadmap
  disproves.
- **`the strict path` claimed the `--strict` boolean is a third way to select
  it.** The disjunct is dead on both surfaces: C-24 refuses the only
  `coverage` configuration where it would decide, and `sync` ignores the boolean
  whenever a strictness value is present, which is always. Measured with a
  seven-row table. Neither spec states the rule the entry attributed to both.

**Confirmed by running, and unchanged:** the `coverage_threshold` precedence
table across six configurations including both zero cases,
`unreachable_annotation` severity routing across all three levels and the
`@reachable manual` suppression in Go and Python, orphan severity by tier with
`enforcement:` overriding in both directions, and the results-file rules for
worst-status-wins, unrecognized statuses, and the fixed path.

**Checked by grep only, and labeled as such:** that no reader exists for
`coverprofile`, `lcov`, `cobertura`, or `coverage.out`, which is adequate for a
negative existence claim; and that "demoted" appears in no user-facing string.

**Measured in the second round, after this appendix was first written:** `the
strict path` (falsified, above), the `annotation` and `threshold` level prose
(holds, with the zero-tolerance clause reworded), `covered, uncovered, demoted`
on the `--json` surface as well as the text table (holds; no third list, no
demoted flag), and `structural conflict` (holds, with the upstream-versus-
downstream `enforcement:` refinement and `SP-SP-014` reproduced).

**What is still uninspected, which is now a short list:**

- `spec coverage against test coverage` was the last one, and it was measured
  during the quantifier sweep in Appendix E, which falsified its absolute claim.

`bugs/SP-SP-004`'s false-positive rate was measured on 2026-08-21 and is
recorded in the `structural conflict` entry. Nothing on the original
uninspected list remains.

Entries marked Retiring or Open were out of scope throughout.

**One process hazard, recorded because it nearly produced a wrong measurement.**
A fixture built in a shared scratch directory under a short name collided with
another agent's files, and the first run reported a `duplicate_ac_id` error for a
spec the reviewer never wrote. The measurement was redone in a uniquely named
directory after confirming its contents. A plausible number from a contaminated
fixture is this project's most repeated failure, and a short directory name is
enough to cause it.

**A second process hazard, which did produce a wrong measurement.** A reviewer
reported that `resolve` exits 0 on a parse error. It exits 1. The command was
`specter resolve 2>&1 | head -20; echo "EXIT=$?"`, and in bash `$?` after a
pipeline is the status of the **last** command in it. The reported number was
`head`'s.

```
no pipe:        $?=7
through head:   $?=0
PIPESTATUS[0]:  $?=7
with pipefail:  $?=7
```

This one is nastier than the contaminated fixture, because the output printed
above the number is genuine. Only the number is wrong, and nothing in the
transcript looks off. `set -o pipefail`, `${PIPESTATUS[0]}`, or not piping at all
when the exit code is the claim each avoid it.

**The general rule these hazards share.** This project has now been burned by a
stale binary, a contaminated fixture, and a pipeline-swallowed exit code. In all
three the output looked right and the conclusion was wrong. A measured number
needs the command that produced it inspected, not only the output it printed.

## Appendix E: the absolute-quantifier sweep, 2026-08-20

Two absolute quantifiers in this document had been examined and both were false,
so the class was swept deliberately rather than left to surface one at a time.
Twelve closed-set claims were tested. **Four failed.**

The test for each was the same: not a grep that confirms what the text says, but
the grep that would find a counterexample.

**Failed, and corrected above:** "`coverage` is the only command that knows"
(`explain` knows per criterion, `doctor` in aggregate); "the enum is declared
once" (three declarations, two of them map literals the flag validates against);
"exactly three files" (five, four excluding tests, stale since `85dfd54`); and
"every percentage Specter prints is spec coverage" (the per-tier rollup is a
spec pass rate).

**Held, each with the search that would have found a counterexample:** that a
parse-stage and a resolve-stage `dangling_reference` never appear in one run
(built a workspace with both defects in different files; blocking is global, not
per file); that `unreachable_annotation_unknown` never fails a gate (all seven
emit sites hardcode `warning` and none reach `routeSeverity`; `--strict` output
was byte-identical); that `main.go:804` is the only producer of `tier_conflict`;
that only two commands accept `--strictness` and there are five flag
registrations; that nothing reads a persisted `gap: true`, widened to the
extension; and that `coverage` never reads `settings.strict`, where the third
consumer at `main.go:3003` is `watch`.

**The pattern in the failures is worth more than the count.** Three of the four
are sentences whose **conclusion** is sound and whose **quantifier** was
decoration. The point about `check`'s blindness stands, the point about the
`unreachable_annotation` family stands, the point about rejection behavior
stands. The absolute was added for force, and it is the only part that broke.
Two of the four were true when written and went stale.

**One claim reported, not reproduced, and withdrawn.** The sweep noted in
passing that `resolve` exits 0 on a parse error. It exits 1. The cause was a
measurement defect rather than a tool defect, and it is recorded in Appendix D
because it is reusable. `bugs/SP-SP-044` covers `resolve --json` exiting zero on
a dependency error and is a different path, still open.

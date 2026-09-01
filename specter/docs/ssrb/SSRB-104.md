# SSRB-104: retire `settings.strictness` for `settings.annotation`

Status: ACCEPT. **Implemented for v0.15.0 on 2026-08-22: 1D-a, 1D-b and 1D6 all shipped.**
Directed: 2026-08-18, founder. Shape settled 2026-08-19.
**Delivery staged 2026-08-21, founder: v0.15.0 ships `scope: test` behavior only.
See section 7.7.**

**What has shipped.** The manifest surface, as roadmap item 1D-a:
`settings.annotation` with `permissive`, the staged `scope` rejection, and the
section 7.1 conflict rule across `check`, `coverage` and `sync`. Recorded in
`spec-manifest` 1.14.1 as C-32, C-33, C-34 and AC-52 through AC-59.

**1D-b shipped the same day.** `spec-coverage` C-38 applies the section 1 model
when a block is declared: rule 1 separated from rule 3, with exit 2 and exit 1
respectively per section 7.5, and `permissive` as the severity switch per
section 7.4. Section 7.9 was added to settle a question section 1 did not
address, whether a declared block requires a results file. It does.

**1D6 shipped too.** Specter carries a root `specter.yaml` at
`permissive: true`, so the feature runs on its own corpus. At
`permissive: false` three criteria fail, one of which asserts a fact about git
and can have no honest test.

**The two criteria owed in `bugs/SP-SP-053` landed**, `spec-manifest` AC-61 and
AC-62, before 1D-b as the roadmap required. Section 7.8's precedence answer is
now pinned by a criterion rather than only recorded here.

**What remains for v1.0.0:** removing `settings.strictness` and `--strictness`
from the schema and the CLI, alongside the three other removals in
`docs/roadmap/v1.0.0.md` section A5. `scope: all` stays unshipped and
conditional on SSRB-101 being reopened, per section 7.7.
Source: the strict/strictness consensus panel of 2026-08-17, and `features/SP-006`

## 1. Request

Retire `settings.strictness` and `--strictness`. Keep both accepted until
v1.0.0 with unchanged behavior, then remove them.

Replace with `settings.annotation`, carrying two sub-keys:

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

**Manifest only. There is no `--annotation` flag and none is planned.**

**As shipped in v0.15.0 the `scope` key does not exist**, because only one of
its two values is implemented. Section 7.7 records that decision and the
reasoning. The shape above is the v1.0.0 target, not the v0.15.0 surface.

**The model, in four rules.**

1. **The annotation rule.** Every acceptance criterion must have a test. A
   criterion with no test fails, and the tier threshold does not excuse it.
2. **`scope` names which files must carry markers.** `test` requires them in
   test files. `all` requires them in test and source files.
3. **The tier thresholds set the allowed failure rate among criteria that do
   have tests.** `tier2: 80` means 80 percent of criteria must have a passing
   test. `tier1: 100` means all of them must.
4. **`permissive` sets severity, not scope.** It warns where the same
   configuration would otherwise fail, and it applies to whichever `scope` is
   set.

So `permissive` and `scope` are two axes rather than three points on one ladder.
That resolves three open questions from an earlier draft of this brief and is
the reason for the sub-key shape.

**What the separation buys, measured.** Today the coverage percentage cannot
distinguish a criterion with no test from a criterion whose test failed:

```
A: AC-04 has a test, and it failed     4 ACs  3 covered  75%  PASS
B: AC-04 has no test at all            4 ACs  3 covered  75%  PASS
```

Byte-identical output for two problems that need different responses from a
developer. Under the rules above, A is a pass-rate question against the tier and
B is a hard failure of the annotation rule.

**`scope: all` does not ship in v0.15.0.** It requires reopening SSRB-101 and
is deliberately out of the first release.

## 2. Origin

`settings.strictness` was created to answer one question: does an acceptance
criterion have a test reference? The goal was confidence that a testable
criterion carries a marker tying it to a test that either passed or failed.
Markers in test files first, markers in source files later.

**What shipped answers a different question.** The three levels form an evidence
ladder deciding how much proof a criterion needs before it counts as covered.
That is a coverage-judgment concept. Marker enforcement is a checker concept.

The gap is measurable. One spec, two criteria, only the first annotated:

```
check --test    All 1 specs passed structural checks    exit 0
coverage        ma-spec  T2  2  1  50%  FAIL   uncovered: AC-02
```

`check` scans test to spec, validating the markers it finds, so a criterion with
no marker is invisible to it. `coverage` scans spec to test and is the only
command that knows. **The fact the setting exists to enforce is computed where
percentages are reported and absent where diagnostics are reported.**

A five-agent consensus panel ran on the naming question and found the term
incoherent by measurement rather than by opinion. **That panel's own record is
not in this repository.** It was written up in a reference document held out of
v0.15.0 for re-verification, and this brief is now the only shipped account of
it. Section 5's table carries the figures the panel produced, 492 lines
mentioning `strictness` against 34 for `settings.strict`, and the sections above
carry the argument. Read those as the evidence; nothing else here restates it.

## 3. Universality

**Satisfied, and on different grounds than a feature request.**

The usual bar asks whether three unrelated projects share a pain. That bar is
built for additions. This is a shipped manifest key in Specter's core
vocabulary, so it reaches every adopter by construction, and the question is not
whether others want it but whether the current state harms them.

Evidence that it does, from the repository's own history:

- **jwtms** ran a `--strict` rollout and filed two bugs against it
  (`CHANGELOG.md:558`, `:676`), and hit the source-only annotation trap on its
  first strict run (`:612`).
- **Kensa** drove the spec-sync C-08 message rewrite, because the old message
  named a flag the operator never passed (`CHANGELOG.md:69`).
- **Yoke** filed against `explain schema spec.status` for a misleading claim
  about what `coverage --strict` honors (`CHANGELOG.md:117`).

Three independent adopters confused by this family of settings. None confused by
anything else in the manifest.

## 4. Cost of acceptance

| Surface | Impact |
|---|---|
| Manifest schema | One key retired, one added. Both accepted through the window, so no manifest breaks before v1.0.0 |
| Spec constraints | `strictness` appears on 92 lines across five specs: spec-check, spec-manifest, spec-doctor, spec-coverage, spec-sync |
| Tree-wide references | 492 lines mention `strictness`, against 34 for `settings.strict` |
| CLI surface | Two `--strictness` registrations retire, `main.go:1241` and `:1412`. **Nothing replaces them.** The new setting is manifest only |
| VS Code extension | `client.ts:180` hardcodes `--strictness annotation` against a user-pinnable binary. It breaks in one direction or the other unless the old flag is accepted through the window, which this request requires |
| Exit codes | 2 and 3 fire only under `zero-tolerance` (`docs/EXIT_CODES.md`). Both go unreachable unless a new trigger is named |
| Docs | `CLI_REFERENCE.md` flag tables, `TEST_ANNOTATION_REFERENCE.md`, `EXIT_CODES.md` |
| Dogfood | `make dogfood-strict` already does not exercise the level it names (`bugs/SP-SP-046`) and needs retargeting regardless |
| Tests | `cmd/specter/cli_docs_parity_test.go` fails until the flag tables match |

**New implementation, not a rename.** Nothing in the tree governs where markers
must appear. `tests_glob` decides which files are scanned, not where markers must
exist, and no diagnostic reports a criterion with no marker. So the retirement
and the capability should be estimated separately.

**Command reach**, verified by which commands discover test files at all:

| Command | Discovers test files | Assessment |
|---|---|---|
| `parse` | no | Out. Nothing to scan |
| `resolve` | no | Out. Same |
| `check` | yes, `main.go:723` | Has the files, scans the wrong direction. Needs new spec-side traversal |
| `coverage` | yes, `main.go:886` | Already computes the fact |
| `sync` | yes, `main.go:1275` | Passthrough only |
| `doctor` | yes, `main.go:2223` | Not yet considered |

## 5. Existing coverage

Nothing else expresses marker enforcement. The closest mechanisms and why each
falls short:

- **`coverage` uncovered lists** name criteria without markers, but as a
  percentage against a tier threshold rather than as a per-criterion diagnostic
  with a file and line.
- **`check --test`** validates markers it finds. It cannot see one that is
  absent.
- **`unreachable_annotation`** reports a marker that will not reach the runner.
  It presumes a marker exists.
- **`tests_glob`** scopes discovery, not obligation.

## 6. Alternatives

**Keep `strictness` and fix its defects.** Cheaper, and it leaves the name
attached to an evidence ladder while the project wants marker enforcement. The
panel's finding was that the confusion is structural rather than a wording
problem.

**Rename only, keeping the ladder semantics.** Solves nothing. The complaint is
not the word, it is that the setting answers a different question than intended.

**Do nothing.** Defensible only if the marker-enforcement goal is abandoned.
That should be stated rather than reached by inaction.

## 7. Decision

**The retirement is accepted by founder direction, and the shape is settled**
as stated in section 1. Three questions from the first draft are resolved by the
two-key structure and are recorded here so nobody re-derives them.

**Resolved: the ladder is not monotonic.** It was, when `permissive`, `default`
and `full` were three points on one list, because `permissive` scanned more than
`default`. With `permissive` as a severity flag over whichever `state` is set,
the problem does not arise.

**Resolved: a missing corner of the grid.** Scope and severity are now two axes
with all four combinations expressible.

**Resolved: the overlap with tier thresholds.** They govern different things.
The annotation rule asks whether a test exists; the tier threshold asks what
share of existing tests must pass. A criterion with no test fails regardless of
tier.

**All four remaining questions were settled on 2026-08-19.** Each is recorded
with the reasoning, because three of the four were decided against the option
that looked more natural.

### 7.1 The key keeps the name `annotation`, with a conflict rule

`annotation` is today a value of `settings.strictness`, so during the window a
manifest can carry both. In YAML they are structurally distinct and no parser is
confused; the hazard is that the two say opposite things. `strictness:
annotation` means markers alone are sufficient evidence. An `annotation` block
means every criterion must have a test. That is a lenient posture and a strict
one in the same file.

**The decision is therefore a conflict rule rather than a name.** When both are
present, the new key wins and a warning names the ignored one. Silence would be
the worst option, because the manifest would read as though `strictness` still
applied.

Renaming to dodge the collision was rejected. `annotation` is the project's own
word for these markers, in the spec text, in `TEST_ANNOTATION_REFERENCE.md`, and
in the CLI. The collision expires at v1.0.0; a worse name would not.

### 7.2 The value is `scope: test | all`, not `state: default | full`

`default` names a config position rather than a behavior. It teaches a reader
nothing and becomes false the moment the default moves: if `all` ever became the
default, `state: default` would mean all.

`scope` names the axis, which `state` left unnamed. `test` and `all` are
parallel, where `test` and `full` are not: one names a scope and the other names
completeness.

### 7.3 Manifest only. No CLI flag, and none is planned

This is the decision with the largest consequences, and it goes further than the
question asked.

**What it buys.** Flag and manifest cannot diverge if there is no flag. That is
the entire bug class `bugs/SP-SP-046` and `bugs/SP-SP-047` document, made
unreachable by construction rather than by discipline. The precedence question
disappears with it, and so does the `Changed()` hazard: today `strict` combines
flag and manifest as `strict || m.Settings.Strict` at **three** sites,
`main.go:711`, `:1313` and `:1347`, so `--strict=false` cannot turn off a
manifest `true`, and **one flag uses `cmd.Flags().Changed()`**, `ingest --stream`, at two sites, added by the SSRB-103 work in this same cycle. A
fourth site, `:3003`, reads `m.Settings.Strict` alone, because `watch` accepts
no flag at all.

**What it costs.** No per-invocation override. A team that wants permissive
locally and enforcing in CI needs two manifests or a manifest edit, and
trialling `scope: all` in one job before committing is not possible.

**One consequence needs an answer before v1.0.0, and it is not blocking now.**
`vscode-extension/src/client.ts:180` invokes
`['coverage', '--json', '--strictness', 'annotation']`. Its own comment records
why, and the reason is stronger than a display preference: under a `threshold`
manifest, plain `coverage` hard-fails without a results file and **emits no JSON
at all**, so the flag is what guarantees a parseable document on every run.
Remove `--strictness` at v1.0.0 with no replacement and the extension has no way
to force that.

Worth naming precisely: **the extension's override exists to work around a
contract violation.** `spec-coverage` C-10 already claims `--json` emits a
document in every state, and `bugs/SP-SP-032` records that it does not. Make
that claim true and the extension needs no override. That is the cheaper fix and
it is owed anyway.

### 7.4 `permissive` is the severity mechanism. None of the three existing patterns applies

The question was which of three existing mechanisms should carry the
missing-test diagnostic's severity. The answer is none, and the mechanism was
already chosen when `permissive` became a sub-key.

**Tier routing is wrong, provably.** The existing pattern maps Tier 1 to error,
Tier 2 to warning, Tier 3 to info (`internal/checker/check.go:50-54`). Applied
to a missing test, a Tier 3 criterion with no test would emit `info`, which does
not fail. That contradicts rule 1, which says the tier does not excuse a missing
test.

**Global escalation is wrong.** `opts.Strict` belongs to `settings.strict`, a
different setting whose own definition is unsettled (`features/SP-005`).
Coupling the annotation rule to it rebuilds the bundling this work dismantles.

**Ladder routing is unavailable**, since the ladder is what retires.

So the diagnostic is `error` when `permissive: false` and `warning` when true,
decided by that setting and nothing else. That is a fourth pattern: a per-feature
severity switch.

**Consequence for SSRB-102.** It becomes deferred rather than unnecessary. Two
features will now each carry their own severity switch, `settings.strict` for
checker diagnostics and `annotation.permissive` for the marker rule. **A third
would be the point at which a general per-rule mechanism earns its keep**, and
that should be recorded in SSRB-102 as a concrete reconsideration trigger rather
than left as NEEDS-DESIGN with no threshold.

### 7.5 Exit codes get distinct triggers, which the retirement otherwise removed

Codes 2 and 3 fire today only under `zero-tolerance` and would go unreachable.
Under the model in section 1 each code gets a condition that does not depend on
a ladder:

| Code | Trigger |
|---|---|
| 1 | Pass rate below the tier threshold |
| 2 | A criterion has no test at all |
| 3 | Approval gate unmet |

**Promoted from observation to decision on 2026-08-22**, because roadmap item
1D-b cannot be specified without it. The annotation rule needs an exit code for
"a criterion has no test", and leaving codes 2 and 3 unreachable is the only
alternative, which contradicts `docs/EXIT_CODES.md` registering both as Stable.

The mapping above is adopted as written. Three consequences worth stating:

**Code 2 changes meaning, and the change is a narrowing.** Today it fires under
`zero-tolerance` when an annotated criterion did not pass. Under the model it
fires when a criterion has no test at all. A workspace whose criteria all have
tests, some failing, moves from code 2 to code 1. That is the intended
distinction: rule 1 and rule 3 are different failures and had one code between
them.

**Code 3 is unchanged.** The approval-gate trigger does not depend on the
ladder, so it survives the retirement untouched.

**This unblocks roadmap item 1A4**, the exit-code parity test, which was blocked
on precisely this question, and it settles the codes 2 and 3 trigger section that
`docs/EXIT_CODES.md` still owes.

### 7.9 An `annotation` block requires a results file

Decided 2026-08-22, surfaced while specifying 1D-b. Section 1 does not address
it and the answer is not derivable from the four rules.

Rule 1 is **structural**: whether a criterion has a test can be answered from
annotations alone, with no results file. Rule 3 is **outcome-based**: a pass rate
needs results. So a workspace declaring an `annotation` block with no
`.specter-results.json` can evaluate one rule and not the other.

**The block requires `.specter-results.json`, exactly as `threshold` does
today.** Both rules then always evaluate together.

The alternative, evaluating rule 1 and skipping rule 3 with a warning, was
rejected. The manifest default today is `threshold`, which requires results.
Declaring an `annotation` block would then silently drop that requirement, and a
workspace would move from an outcome-verified gate to a structural one by adding
a key that reads as *stricter*. That is the silent-weakening class this project
has spent the cycle removing.

It also keeps the interim behavior in section 7.7a and the final behavior
agreeing on this point, so 1D-b does not change what a missing results file
does.

### 7.6 Deferred criteria become a prerequisite, not an optional phase

The annotation rule fails a criterion that has no test. Specter's own repository
would fail three of fifteen specs under it:

```
spec-diff        92.9%   uncovered: AC-11
spec-manifest    98%     uncovered: AC-29
spec-reverse     94.7%   uncovered: AC-18
```

`spec-manifest` AC-29 shows why this is not simply unfinished work. It asserts
that `git push --no-verify` bypasses the pre-push hook, which is a fact about
git rather than about Specter, so no honest test exists.

A workspace adopting the annotation rule needs a way to say *this criterion is
deliberately untested, and here is why*. That is roadmap phase 3C, deferred
criteria (`SSRB-098`). **It moves from an optional phase-3 item to a
prerequisite**, because the alternative recourse is a fake test, which is worse
than no test and defeats the evidence the rule exists to capture.

**`scope: all` requires reopening SSRB-101.** That brief rejected source-file
annotation as F7 on 2026-08-16, arguing that an annotation on an implementation
function has no runner-visible counterpart and can only ever be an unverifiable
claim. That argument survives the intent clarification and should be answered
rather than bypassed. If `scope: all` does not land, `scope` carries a single
value and is inert until it does. Section 7.2's naming decision stands either
way.

### 7.7 v0.15.0 ships `scope: test` behavior, and no `scope` key

Directed 2026-08-21. The annotation model ships in v0.15.0 with **test-scope
behavior only**. `scope: all` does not ship.

**This is not an arbitrary narrowing.** Section 6 of this brief already records
that `scope: all` requires reopening `docs/ssrb/SSRB-101.md`, which rejected
source-file annotation as F7 on 2026-08-16, on the grounds that an annotation
on an implementation function has no runner-visible counterpart and can only
ever be an unverifiable claim. That argument has not been answered. Declining to
ship `all` is therefore consistent with SSRB-101 standing, not a departure from
it.

**The `scope` key does not ship in v0.15.0.** Section 6 anticipated this exact
case and named the consequence: "if `scope: all` does not land, `scope` carries
a single value and is inert until it does." A key with one legal value decides
nothing, and shipping one deliberately runs against two of this project's own
rules. The v0.18 pre-lock criterion requires that every schema field be consumed
deterministically by at least one command. And `projects/specter/MEMORY` carries
a triage filter written after three inert manifest surfaces were found in one
week (`bugs/SP-SP-001`, `bugs/SP-SP-049`, and the dead `--strict` disjunct): when
a document describes a fallback, check whether the case can occur.

So v0.15.0 ships `settings.annotation.permissive` and nothing else under that
key. Behavior is test-scope, unconditionally.

**One handling rule, because this brief is public and names `all`.** An adopter
who reads section 1 and writes `scope: all`, or `scope: test`, must not get a
bare unknown-field error. The manifest validator special-cases
`settings.annotation.scope` with a message naming the staging:

```
error: settings.annotation.scope is accepted in SSRB-104 and not implemented in
       v0.15.0. Annotation scope is test-only; remove the key.
```

That is a validation rule, not a schema field, so it adds nothing inert.

**Adding `scope` later is non-breaking.** It arrives with default `test`, so
every v0.15 manifest keeps its meaning. Section 7.2's naming decision stands
unchanged and is unaffected by the staging.

**Dogfooding consequence, which the roadmap did not surface.** Specter has **no
root `specter.yaml`**, so the annotation rule is inert on Specter's own corpus
until one is added. Shipping the feature without a manifest means shipping a
feature the project does not dogfood, which this project's own culture rejects.
Adding one at `permissive: false` fails three criteria: `spec-diff` AC-11,
`spec-manifest` AC-29, and `spec-reverse` AC-18, measured 2026-08-21. AC-29
asserts that `git push --no-verify` bypasses the pre-push hook, a fact about git
rather than about Specter, so no honest test exists for it.

**Recommendation: dogfood at `permissive: true` in v0.15.0.** That is the mode
built for a project not yet at the bar. It warns on all three, keeps
`make dogfood-strict` green, and demonstrates the feature honestly. Moving to
`permissive: false` needs SSRB-098's deferred criteria, which is roadmap item
3C, and that is a v0.16 decision rather than a v0.15 one.

### 7.7a The v0.15.0 interim behavior, and what it costs

Recorded 2026-08-22, after roadmap item 1D-a shipped the manifest surface.

Until 1D-b lands the rule that consumes `permissive`, a declared `annotation`
block resolves to **the strict path at `threshold`**, the C-24 default.

**A declared block is not inert, and an earlier claim that it was is withdrawn.**
The orchestrator's decision passed to the build phases said the interim was
"chosen so the block changes nothing observable beyond the warning". Measured on
2026-08-22, that is false:

```
strictness: annotation, annotated tests, no results file   coverage rc=0
same workspace + 'annotation: {}'                          coverage rc=1
  error: strictness "threshold" requires .specter-results.json
```

So an adopter on `settings.strictness: annotation` who declares the emptiest
possible block moves from the lenient path to the strict one and their build
goes red. The C-34 warning fires, so it is visible rather than silent.

**`threshold` was kept anyway, because the alternative is worse.** The interim
value is genuinely free: the full suite passes with `annotation` and with
`zero-tolerance` as well. Measured on the same workspace shape at
`settings.strictness: threshold`, which is the C-24 default and therefore the
common case:

| Adopter | interim `threshold` | interim `annotation` |
|---|---|---|
| `strictness: annotation` | rc 0 to 1, warned | unchanged |
| `strictness: threshold` | unchanged | rc 1 to **0**, strict path silently dropped |

A visible red build on an opt-in key beats a gate that quietly stops gating. The
second column is the P1 false-confidence class in this repository's priority
framework.

**1D-b should decide this explicitly rather than inherit it.** The value it
picks replaces the interim, and the tradeoff above is the one it is choosing
between, not a default it can accept without looking.

### 7.8 Open: `--strictness` against a declared `annotation` block

Surfaced 2026-08-22 by the 1D-a spec phase, which could not resolve it from this
brief and correctly declined to invent an answer.

Section 7.3 makes flag-and-manifest divergence unreachable **for the new key**,
by giving it no flag. It does not address the **legacy** flag. Section 1 keeps
`--strictness` working unchanged until v1.0.0, so during the whole deprecation
window this is reachable:

```
specter coverage --strictness zero-tolerance     # with an annotation block declared
```

`spec-manifest` C-34(d) pins that no observable result varies with the
**manifest** `strictness` value while the block is declared, and AC-59 pins that
the flag cannot trigger the conflict warning. Neither says what the flag does to
the block's behavior.

**Three candidates.**

1. **The flag still wins**, as it does today. Preserves the deprecation promise
   that `--strictness` behaves unchanged, and reintroduces exactly the
   divergence 7.3 was written to prevent, through the one door left open.
2. **The block wins and the flag is ignored, with a warning.** Consistent with
   7.1's treatment of the manifest key, and it breaks the deprecation promise
   for a workspace that has opted in.
3. **The combination is an error**, the way C-24 already refuses
   `--strict` with `strictness: annotation`. Cheapest to reason about, and it
   makes an adopter mid-migration fix their CI invocation before they are ready.

**Answered for v0.15.0 on 2026-08-22: option 1, the flag still wins.**

The question was raised while 1D-a was in flight and it did not wait for a
decision. The implementation took option 1, and phase 4 measured what shipped
rather than what its author intended:

```
# manifest declares an annotation block and no settings.strictness
coverage                          rc=1   block governs, resolves to threshold
coverage --strictness annotation  rc=0   flag wins, lenient path
```

Only `coverage` and `sync` are affected, since `check` has no `--strictness`
flag, and both are consistent.

**It is recorded here because it was decided in code and undecided on paper,
which is the worst of the two states.** Phase 4 confirmed the opposite reading
also passes the full suite: rewriting both call sites so the block beats the
flag is green. So the precedence is entirely unguarded, and a future change in
either direction would pass CI.

Option 1 is the right answer on its merits, not only by accident of what shipped.
Section 1 promises `--strictness` behaves unchanged until v1.0.0, and options 2
and 3 both break that promise for a workspace that has opted in to the new key.
The divergence 7.3 exists to prevent is between the **new** key and a flag it
does not have; the legacy flag was always going to keep working through the
window.

**Two things this still owes.**

A criterion pinning the precedence, so the opposite reading fails. Filed with
the other 1D-a coverage gap rather than left as a note.

A re-decision at 1D-b, which gives the block observable behavior and therefore
raises the stakes on what a flag can override. The answer may still be option 1;
it should be chosen again rather than inherited.

**It expires on its own** when `--strictness` is removed at v1.0.0, which is an
argument for option 3: the least code, on a path that is going away.

## 8. Reconsideration triggers

- Any answer to 7.1 through 7.4 that changes the value set materially, which
  makes this brief describe a different request.
- A decision to abandon marker enforcement, which turns this into a plain
  deprecation with no replacement.
- SSRB-101 reopened and F7 accepted, which promotes `scope: all` from
  conditional to ordinary and makes the `scope` key worth adding. This is the
  specific trigger for revisiting 7.7.
- Evidence that the deprecation window is too short, meaning adopters cannot
  migrate a manifest key inside one minor series.

## 9. References

- `features/SP-006`, the request this brief formalizes.
- `features/SP-005`, `settings.strict` coherence. A separate axis and a separate
  decision; the two should not be bundled.
- `docs/ssrb/SSRB-101.md`, which rejected source-file annotation.
- `docs/ssrb/SSRB-102.md`, per-rule severity, NEEDS-DESIGN.
- `bugs/SP-SP-038`, surfaces computing coverage from a different input than the
  gate. The replacement inherits it unless fixed.
- `bugs/SP-SP-046`, `bugs/SP-SP-047`, `bugs/SP-SP-048`.

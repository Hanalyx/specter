# SSRB-104: retire `settings.strictness` for `settings.annotation`

Status: ACCEPT in direction, NEEDS-DESIGN in shape
Directed: 2026-08-18, founder
Source: the strict/strictness consensus panel of 2026-08-17, and `features/SP-006`

## 1. Request

Retire `settings.strictness` and `--strictness`. Keep both accepted until
v1.0.0 with unchanged behavior, then remove them.

Replace with `settings.annotation`, carrying two sub-keys:

```yaml
settings:
  annotation:
    state: default       # default | full
    permissive: false    # true warns where false fails
  coverage:
    tier1: 100
    tier2: 80
    tier3: 50
```

**The model, in four rules.**

1. **The annotation rule.** Every acceptance criterion must have a test. A
   criterion with no test fails, and the tier threshold does not excuse it.
2. **`state` sets the scope.** `default` requires markers in test files. `full`
   requires them in test and source files.
3. **The tier thresholds set the allowed failure rate among criteria that do
   have tests.** `tier2: 80` means 80 percent of criteria must have a passing
   test. `tier1: 100` means all of them must.
4. **`permissive` sets severity, not scope.** It warns where the same
   configuration would otherwise fail, and it applies to whichever `state` is
   set.

So `permissive` and `state` are two axes rather than three points on one ladder.
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

**`full` does not ship in v0.15.0.** It requires reopening SSRB-101 and is
deliberately out of the first release.

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
incoherent by measurement rather than by opinion. That work is recorded in
`docs/SPECTER_LEXICON.md`.

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
| CLI surface | Two `--strictness` registrations retire, `main.go:1241` and `:1412`. New `--annotation` registrations on the commands that honor it |
| VS Code extension | `client.ts:180` hardcodes `--strictness annotation` against a user-pinnable binary. It breaks in one direction or the other unless the old flag is accepted through the window, which this request requires |
| Exit codes | 2 and 3 fire only under `zero-tolerance` (`docs/EXIT_CODES.md`). Both go unreachable unless a new trigger is named |
| Docs | `CLI_REFERENCE.md` flag tables, `TEST_ANNOTATION_REFERENCE.md`, `EXIT_CODES.md`, `SPECTER_LEXICON.md` |
| Dogfood | `make dogfood-strict` already does not exercise the level it names (`bugs/SP-SP-046`) and needs retargeting regardless |
| Tests | `cmd/specter/cli_docs_parity_test.go` fails until the flag tables match |

**New implementation, not a rename.** Nothing in the tree governs where markers
must appear. `tests_glob` decides which files are scanned, not where markers must
exist, and no diagnostic reports a criterion with no marker. So the retirement
and the capability should be estimated separately.

**Command reach**, verified by which commands discover test files at all:

| Command | Discovers test files | Assessment |
|---|---|---|
| `parse` | no | Out. A flag would be inert |
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

Three questions remain.

**7.1 The key name collides with a value being retired.** `annotation` is today
a value of `settings.strictness`. Both keys are accepted through the window, so
a manifest may legally carry `strictness: annotation` and an
`annotation.state` block together, meaning different things. That is the failure
this work exists to end, reappearing in the replacement.

**7.2 `default` names a config position rather than a behavior.** The name lies
if the default ever moves. `test` would describe the scope directly and pair
naturally with `full`.

**7.3 The CLI surface is unspecified.** The manifest carries two sub-keys and no
flag shape follows from that. `--annotation <state>` plus a separate
`--annotation-permissive`, a single flag carrying both, or state-only with
permissive left to the manifest are all open. Whichever is chosen, the
precedence rule between flag and manifest has to be stated per sub-key, because
this project has already shipped one setting where the flag and the key diverge
(`bugs/SP-SP-047`).

**A fourth question is mechanical rather than design.** A missing-test
diagnostic needs a severity, and three patterns exist in the tree: tier routing
as orphan constraints use, ladder routing as `unreachable_annotation` uses, and
global escalation through `opts.Strict`. `permissive` supplies the warn-or-fail
decision, so what remains is which mechanism carries it. The choice determines
whether `SSRB-102` becomes unnecessary or merely deferred.

### 7.4 Exit codes get distinct triggers, which the retirement otherwise removed

Codes 2 and 3 fire today only under `zero-tolerance` and would go unreachable.
Under the model in section 1 each code gets a condition that does not depend on
a ladder:

| Code | Trigger |
|---|---|
| 1 | Pass rate below the tier threshold |
| 2 | A criterion has no test at all |
| 3 | Approval gate unmet |

This is offered as an observation rather than a decision. It resolves the
contradiction between this brief and `docs/EXIT_CODES.md`, which registers both
codes as Stable.

### 7.5 Deferred criteria become a prerequisite, not an optional phase

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

**`full` requires reopening SSRB-101.** That brief rejected source-file
annotation as F7 on 2026-08-16, arguing that an annotation on an implementation
function has no runner-visible counterpart and can only ever be an unverifiable
claim. That argument survives the intent clarification and should be answered
rather than bypassed. If `full` does not land, `permissive | default` is a
two-value enum, which is a boolean wearing an enum's clothes.

## 8. Reconsideration triggers

- Any answer to 7.1 through 7.5 that changes the value set materially, which
  makes this brief describe a different request.
- A decision to abandon marker enforcement, which turns this into a plain
  deprecation with no replacement.
- SSRB-101 reopened and F7 accepted, which promotes `full` from conditional to
  ordinary.
- Evidence that the deprecation window is too short, meaning adopters cannot
  migrate a manifest key inside one minor series.

## 9. References

- `features/SP-006`, the request this brief formalizes.
- `features/SP-005`, `settings.strict` coherence. A separate axis and a separate
  decision; the two should not be bundled.
- `docs/SPECTER_LEXICON.md`, `strictness` entry and the RETIRING section.
- `docs/ssrb/SSRB-101.md`, which rejected source-file annotation.
- `docs/ssrb/SSRB-102.md`, per-rule severity, NEEDS-DESIGN.
- `bugs/SP-SP-038`, surfaces computing coverage from a different input than the
  gate. The replacement inherits it unless fixed.
- `bugs/SP-SP-046`, `bugs/SP-SP-047`, `bugs/SP-SP-048`.

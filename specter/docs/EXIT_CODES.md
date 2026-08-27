# Specter Exit Codes

This document is the single allocation authority for Specter process exit codes.
No command may emit a code that is not allocated here, and no design track may
pick a number without adding it here first.

Verified runtime behavior against `release/v0.15.0` at `6c82473`, using a binary
built with `make build` from `specter/`. Every claim in that sweep was checked by
reading `cmd/specter/main.go` or by running `bin/specter`, and
[Appendix A](#appendix-a-how-each-claim-was-checked) says which.

**Claims added after that sweep name their own verification, in the section that
makes them.** Code 10's behavior was measured on 2026-08-24. The SP-069 and
SP-070 enforcement claims were verified against `515e6df` on 2026-08-25. Code 20's
row, its band entry, and the precedence claim about it were measured against
`622a172` on 2026-08-26: a workspace carrying an inconsistent `streams` block and
nothing else exits 20 on `coverage`, `sync` and `sync --json`, and the same
workspace also below its tier threshold exits 1 on all three with the stream
violations still named. None of these is covered by the `6c82473` sweep, and the
header does not pretend otherwise: a document that re-dates its whole
verification on every edit stops recording when anything was actually checked.

## Why this document exists

Three design tracks began allocating exit codes at the same time, with no shared
list. That is the mechanism that double-books a number.

A prototype showed the failure is worse than a duplicate. A new gate ran ahead of
a shipped one, so a genuinely failing test was reported as a stale deferral
marker. Both the exit code and the error line named the wrong cause. An operator
reading either channel would have looked in the wrong place.

Three rules follow from that, and the rules matter more than the numbers:

1. **Allocation.** Band the space. Each track allocates inside its own range.
2. **Precedence.** A new code may never preempt a shipped one.
3. **Propagation.** A code that fires on `coverage` fires on `sync`.

## Terms

**Code.** The integer the process returns to its caller.

**Gate.** A check whose result can change the exit code.

**Shipped code.** A code some released command can emit today. Every shipped
code appears in the table below.

**Band.** A reserved range of integers. A band is an allocation, not a promise
that any number in it is in use.

**Effective strictness.** The strictness level in force for a run, after the
`--strictness` flag, the `--strict` alias, and `settings.strictness` in
`specter.yaml` are resolved. Several codes fire only under `zero-tolerance`.

## Section 1: the codes Specter emits today

Specter emits six codes. There is no seventh.

The count was five until code 20 shipped. It is stated as a number rather than
left to the table because a reader who skims the table and miscounts is exactly
who this document is for, and `spec-sync` AC-16 and AC-18 make the two agree in
both directions: every code the binary can emit has a row, and every code with a
Stable row is reachable.

| Code | Commands | Condition | Standing |
|---|---|---|---|
| `0` | all | The command completed and every gate it ran passed. | Stable |
| `1` | all | Everything else. See the breakdown below. | Shipped, overloaded, frozen |
| `2` | `coverage`, `sync` | Effective strictness `zero-tolerance`, and at least one annotated acceptance criterion has a results-file status other than `passed`. | Stable |
| `2` | `coverage`, `sync` | A `settings.annotation` block is declared with `permissive: false`, and at least one acceptance criterion has no test at all. | Stable |
| `2` | all | The process recovered a panic on the main goroutine. | Accidental collision |
| `3` | `coverage`, `sync` | At least one acceptance criterion carries `approval_gate: true` with an unset `approval_date`, under effective strictness `zero-tolerance` **or** under a declared `settings.annotation` block. | Stable |
| `10` | `diff` | `--exit-code` was passed and the change class is `breaking`. Without the flag, `diff` exits 0. | Stable |
| `20` | `coverage`, `sync` | The results file carries a `streams` block that breaks one of `spec-coverage` C-44's five consistency rules, and no gate that shipped earlier also failed. | Stable |

### Code 0

Returned when `RunE` returns `nil`. `--help` and `--version` also return 0.

`watch` returns 0 on SIGINT or SIGTERM once it has entered its event loop
(`cmd/specter/main.go:2933-2935`). It never returns a code of its own for a
failed cycle, because a watch cycle reports to the terminal rather than to the
caller.

Some zeros today are wrong. [Section 2](#section-2-where-the-code-and-the-output-disagree)
lists them. Do not read a 0 from this table as a contract that the work was done.

### Code 1

Code 1 is the unclassified failure. At least six unrelated conditions produce it,
which is `bugs/SP-SP-020`.

| Producer | Site | Example |
|---|---|---|
| Cobra usage error | `main.go:200-204` | unknown command, unknown flag, wrong argument count |
| No specs discovered | `main.go:427`, `:481`, `:1273` | an empty workspace |
| A spec failed to parse | `main.go:459`, `:687`, `:1118` | invalid YAML, schema violation |
| A dependency error | `main.go:570`, `:702` | dangling `depends_on` reference |
| A gate failed | `main.go:668-672`, `:1231`, `:1403` | check errors, coverage below threshold |
| A configuration error | `main.go:875`, `:927`, `:946`, `:2891` | rejected manifest, invalid `--strictness` value |

A caller reading only the integer cannot tell "the gate failed" from "the gate
never ran". Until code 1 is re-carved, a driver has to read stderr. Cobra usage
errors are the one class that is easy to separate: they print `error: ` followed
by a hint to run `specter --help`, and no gate does that.

**Code 1 is frozen.** Do not add a new condition to it. `bugs/SP-SP-020` defers
the re-carve to the v1.0 contract release, because moving conditions off 1 breaks
callers that read it today.

### Code 2 has two triggers, and they do not overlap

Added 2026-08-22, when `docs/ssrb/SSRB-104.md` section 7.5 was promoted from an
observation to a decision and roadmap item 1D-b shipped.

| Trigger | Condition |
|---|---|
| Ladder | Effective strictness `zero-tolerance`, and an **annotated** criterion has a results-file status other than `passed` |
| Annotation model | A `settings.annotation` block is declared with `permissive: false`, and a criterion has **no test at all** |

**The two cannot both describe one criterion**, because a criterion with no test
has no results-file status to be wrong. So a caller reading 2 knows the run
failed a contract gate and must read the message to know which, exactly as it
must today between the ladder's own gates.

**Why the model needed a second trigger rather than reusing code 1.** Under the
ladder, a criterion with no test and a criterion whose test failed were the same
failure: both counted as uncovered and both moved the percentage. The model
separates them, so a missing test exits 2 and a pass rate below the tier exits 1.
Collapsing them would have removed the distinction the model exists to draw.

**This resolves the contradiction this document previously carried.** Section
7.5 had noted that codes 2 and 3 fire only under `zero-tolerance` and would go
unreachable when the ladder retires at v1.0.0, while this document registers both
as Stable. Code 2 now has a trigger that survives the retirement. Code 3's
trigger never depended on the ladder.

### Code 2, the zero-tolerance non-passed gate

Emitted by `coverage` from `coverageExitGates` at `main.go:888`, which both the
text path and the `--json` branch call, and by `sync`. Both print the same
sentence to stderr before exiting:

```
error: zero-tolerance strictness — %d annotated AC(s) did not pass
```

That string is the golden value for the propagation parity test in roadmap item
1A4. It must stay identical wherever it is emitted, which is one place now:
`coverage.GateVerdict` builds it and both commands print what the verdict
returns.

### Code 2, the panic path

`main.go:152-160` recovers a panic on the main goroutine, prints a pre-filled bug
report link, and calls `os.Exit(2)`.

**This collides with the zero-tolerance gate.** A caller that sees 2 from
`coverage` or `sync` cannot tell a failing acceptance criterion from a crash. The
sentence on stderr differs, so the two are separable by message and not by code.

The collision is a defect, not an allocation. It belongs at `70` under
[Section 3](#section-3-band-allocation). This document does not fix it; the fix
needs its own bug and its own change.

A panic on any other goroutine is not caught by that `recover`, because a
deferred recover only covers its own goroutine. The Go runtime ends the process
instead. Specter starts goroutines for the spinner and the file watcher. Which
status the runtime produces in that case is not verified here.

### Code 3, the approval-gate violation

Emitted by `coverage` from the same `coverageExitGates`, and by `sync`.

The gate fires under both strictness models, which is what `spec-coverage`
C-40 requires and what `bugs/done/SP-SP-071` was filed for: counting the
violations inside the zero-tolerance branch alone made the gate silent for a
workspace that declared `settings.annotation`. So there are two sentences, one
per model, and a run prints the one that matches how it was configured.

Under the ladder:

```
error: zero-tolerance strictness — %d AC(s) carry approval_gate=true with unset approval_date
```

This is the second golden value for 1A4.

Under a declared `settings.annotation` block:

```
error: %d AC(s) carry approval_gate=true with unset approval_date. An approval gate is a human sign-off, so settings.annotation.permissive does not soften it
```

The second sentence names the setting the operator actually chose. Naming
zero-tolerance in a run that is not on the ladder would send them to a setting
they never set. `permissive` is named because an operator who set it will
reasonably expect it to soften this too, and it does not: an approval gate is a
human sign-off rather than a coverage measurement.

Measured 2026-08-26 at `986c228`, on a workspace declaring `settings.annotation`
with `permissive: true`, no `--strictness` flag, every criterion covered, and one
criterion carrying `approval_gate: true`: `coverage`, `coverage --json`, `sync`
and `sync --json` each exit 3 and print the second sentence.

### Code 10, the breaking-diff gate

Emitted by `diff` at `main.go:3233`, from a deferred call so the report prints
before the process leaves. Two conditions, both required: `--exit-code` was
passed, and the change class is `breaking`.

Without the flag, `diff` exits 0 on a breaking change, and that is deliberate.
`spec-diff` C-10 calls the command a diagnostic surface rather than a gate, and
changing a shipped exit code silently breaks every caller, so the failing exit
arrived as an opt-in flag on git's own pattern. `spec-diff` C-14 states the rule
and AC-19 asserts it. The history is `bugs/done/SP-SP-012`.

Measured on `release/v0.15.0`:

| Invocation | Code |
|---|---|
| breaking, with `--exit-code` | `10` |
| breaking, without the flag | `0` |
| unchanged, with `--exit-code` | `0` |

**This document registered code 10 in one place and contradicted it in five.
The five were repaired on 2026-08-24.** Section 3 named the allocation when
SP-SP-012 closed. The count opening Section 1, the table in Section 1, the 0-or-1
list below, the Section 2 row for `diff`, and the "nothing occupies 10 to 29"
line in Section 3 all kept describing a binary that no longer existed. `spec-sync` C-12 is the rule
this broke, and it was unenforced here. The parity test matched `os.Exit(`
followed by literal digits, and this site returns a named constant, so the test
stayed green over a registry contradicting the binary in five places.

**Enforced 2026-08-25**, as `bugs/done/SP-SP-069`. The scan now reads the syntax
tree, resolves a named constant through the `const` declarations in scope, and
refuses an argument it cannot resolve instead of skipping it. `spec-sync` AC-17
states the rule and pins the property that matters: the number of sites resolved
must equal the number of `os.Exit` calls in scope, because a scan that drops what
it cannot read reports the same result as a scan with nothing to drop.

**A second defect, and this document was the cause.** Renaming the code 10 row
left the check green, because it matched the code cell anywhere in the file and
the measurement table above holds `10` in its second column. A registry row now
has to start the line. Verified by mutation rather than by reading: renaming the
row for 1, 2, 3, or 10 now fails the test. Code 0 still survives a rename, correctly.
It is registered and it is not an `os.Exit` call, so only AC-16's converse could
reach it, and the converse exempts it for that reason.

**Both directions are enforced as of 2026-08-25**, the converse as
`bugs/done/SP-SP-070`. `spec-sync` AC-18 states the three rules it needs. Stable
is an exact match on the Standing cell, because `Shipped, overloaded, frozen` and
`Accidental collision` are the other values in use and neither is a weaker form
of stable. A code is required to be reachable when any one of its rows is Stable,
because standing belongs to a row while the claim is about a code, and code 2
carries three rows of differing standing. Code 0 is exempt, and the exemption is
guarded: the assertion fails if anything ever calls `os.Exit(0)`.

Adding a Stable row here for a code nothing emits now fails the build. That was
verified by doing it, not by reading the assertion.

### Commands that can only emit 0 or 1

`parse`, `resolve`, `resolve dependents`, `check`, `reverse`, `init`, `doctor`,
`explain`, `watch`, `diff coverage`, `ingest`, `feedback`, and `pre-push-check`.
`diff` is not on this list: it emits 10 under `--exit-code`, per the section
above. Verify with
`grep -rn "os.Exit" --include="*.go" cmd/ internal/ | grep -v _test.go`, which
finds exit sites in `main.go` only. No package under `internal/` calls
`os.Exit`, which is what keeps the internal packages usable as a library.

The `_test.go` filter is not cosmetic. Without it the command returns
`cli_test.go`, whose `TestMain` calls `os.Exit(0)` and `os.Exit(m.Run())`, plus
three test files that only mention the string. Those are outside the shipped
binary and outside the `spec-sync` AC-16 scan, which skips `_test.go` for the
same reason.

## Section 2: where the code and the output disagree

These are the cases the registry exists to make impossible. Each is a place where
the exit code says one thing and the document, the diagnostic, or the truth says
another. They are recorded as current behavior, not as allocations.

| Case | Behavior | Bug |
|---|---|---|
| `resolve --json` on a dependency error | The `dangling_reference` diagnostic appears in the document. Exit 0. Text mode exits 1. | Not filed as of 2026-08-17 |
| `diff` on a change it labels `[breaking]`, without `--exit-code` | The classification is printed and correct. Exit 0. Intended, per `spec-diff` C-10. With `--exit-code` the same run exits 10. | `bugs/done/SP-SP-012`, resolved in v0.15.0 |
| `diff coverage` on any delta | Exit 0 by design. Documented as a diagnostic surface, not a gate. | Intended |
| `check` and `coverage` on an unreadable specs directory | Reported as a clean workspace with zero specs. Exit 0. `sync` exits 1, but names the wrong cause. | `bugs/SP-SP-026`, open |
| `check --json` when a spec fails to parse, the resolver errors, or the manifest is rejected | Exit 1 with an empty stdout. No document at all. | `bugs/SP-SP-032`, open |
| `coverage --json` when the manifest is rejected | Exit 1 with an empty stdout. | `bugs/SP-SP-032`, open |
| A rejected `specter.yaml` | `check`, `coverage`, `sync`, `doctor`, and `watch` exit 1. `parse`, `resolve`, `resolve dependents`, and `explain` warn on stderr and exit 0, through `warnManifestRejected()` at `main.go:422`, `:476`, `:618`, and `:2612`. | `bugs/SP-SP-017`, open |

`check --json` was the same defect until this cycle. It was fixed by
`checkExitVerdict`, which both the text branch and the JSON branch now end on,
so the verdict cannot differ by rendering.

**`parse --json` was the third and last of the group, fixed 2026-08-25 as
`bugs/done/SP-SP-022`.** Its row is gone from the table above rather than marked
resolved, because Section 2 lists cases where the code and the output disagree
today. `hasErrors` was set inside the text branch and the JSON branch hit
`continue` above it, so the verdict now comes off the parse result before the
rendering branch. `spec-parse` C-11 states the rule and AC-19 pins both
directions, the failing workspace at 1 in both modes and the clean one at 0, so
parity cannot be reached by failing everything.

**`coverage` followed, and the second case is the sharper lesson.** Its `--json`
branch carried a private copy of the gate sequence plus a comment saying it
mirrored the text checks. That was true when written. `settings.annotation`
shipped later in the same cycle, went into the text path, and the comment
stayed, so a workspace whose tier threshold was met exited 2 in text and **0**
under `--json`. A CI job reading JSON got a green build on a workspace
`coverage` fails (`bugs/done/SP-SP-066`).

It was fixed by deleting the private copy rather than by adding the missing
check, and by a table test asserting the two surfaces agree across every
gate-relevant state. The three per-gate tests that preceded it all passed
throughout. **A per-gate test cannot fail when a new gate is added to one
surface; a table can.**

Those two functions are the pattern to copy. `parse` reaches the same result by
the other route: it still renders in a branch, but the verdict is taken off the
parse result above it, so the branch decides only the format. `resolve` and
`reverse` still decide inside theirs.

Two rules follow for anyone adding a gate:

- **A `--json` branch takes the same verdict as its text branch.** Route both
  through one function. Do not restate the rule in the branch.
- **An exit code and the emitted document must agree.** If a command cannot do
  the work, it says so in the document rather than writing nothing.

## Section 3: band allocation

Bands exist so a track can allocate a number without asking, and without
colliding. A planned gate takes a specific number in the planned-gate table when
the spec that scopes it lands, and takes a Section 1 row only in the commit that
makes it reachable. Recording the number early is what stops two tracks picking
the same one; withholding the Section 1 row is what keeps every Stable code
reachable, which `spec-sync` AC-18 enforces.

| Band | Purpose | Allocated to |
|---|---|---|
| `0` | Success | Shipped |
| `1` | Unclassified failure. Frozen. | Shipped |
| `2` to `9` | Spec and coverage contract | `2` and `3` shipped. `4` to `9` free. |
| `10` to `19` | Orchestration gates | `10` shipped (`diff --exit-code`). `11` to `19` free. |
| `20` to `29` | Evidence stream validation | `20` shipped (stream validation). `21` planned. `22` to `29` free. |
| `30` to `63` | Unallocated | Reserved for a future track |
| `64` to `78` | Usage, internal, and configuration errors, per `sysexits.h` | `70`, `78` and `64` planned, none emitted yet. Rest free. |
| `79` to `125` | Do not use | Reserved |
| `126` and above | Do not use | Owned by the shell and the operating system |

The band edges are the roadmap's proposal, and the shipped codes fit them without
change. Codes 2 and 3 both gate the spec and coverage contract, and both sit
inside `2` to `9`. Code 10 is the first allocation from the
orchestration band, taken by `specter diff --exit-code` on a breaking change.
Code 20 is the first allocation from the evidence band, taken by the
streams-block refusal `spec-coverage` C-44 defines.

### Three constraints on any number picked from a band

**An exit status is eight bits.** POSIX truncates the value a process returns to
its low byte. `os.Exit(256)` reports success. Never derive a code from a count.
This is not hypothetical: a mutant that replaced the check verdict with
`os.Exit(result.Summary.Errors)` survived the test suite, and is recorded in
`bugs/SP-SP-021`.

**Do not use 126 and above.** A shell reports 126 for a file that is not
executable, 127 for a command not found, and 128 plus the signal number for a
process killed by a signal. A binary that returns those numbers is
indistinguishable from a failure to run it at all.

**A band is not a promise, and a planned number is not an occupied one.** Two are
occupied today: 10 by `diff --exit-code` and 20 by the streams-block refusal.
Nothing occupies 11 to 19 or 21 to 29. Code 21 is planned in the table below and
emitted by nothing, which is why it has no Section 1 row: that row asserts a code
is reachable and `spec-sync` AC-18 fails the build when it is not. Anyone reading
21 from a released binary has found a bug or a stale build.

### Where the planned gates belong

Named here so no track has to guess. A number is recorded in this table during
the spec cycle that scopes its gate, and takes a Section 1 row only in the commit
that makes it reachable.

**The evidence band's first number is now emitted and its second is not.** Code 20
is `spec-coverage` C-44, the artifact-consistency refusal, and it took its
Section 1 row in the commit that made it reachable, which is the rule this
section states. Code 21 is reserved for the differential refusal when 3B ships
and has no Section 1 row yet, for the same reason: a row marked Stable asserts
the code is reachable, and `spec-sync` AC-18 fails the build when it is not.
Each row lands in the commit that makes its code reachable, so both directions
of C-12 hold at every commit rather than only at the end of a cycle.

| Planned gate | Roadmap | Band |
|---|---|---|
| Failing exit for `diff` on a breaking change | 2B3 | **Shipped as `10`** |
| Stream validation refusing an inconsistent artifact | 3A5 | **Shipped as `20`** |
| Stream validation refusing a differential | 3B4 | Evidence stream, **`21`** |
| The panic path, once it moves off 2 | Not scheduled | `sysexits.h`, `70` |
| Configuration errors, once they move off 1 | v1.0, per `bugs/SP-SP-020` | `sysexits.h`, `78` |
| Usage errors, once they move off 1 | v1.0, per `bugs/SP-SP-020` | `sysexits.h`, `64` |

## Section 4: precedence

**Rule. A new gate runs after every gate that already ships. A new code may never
preempt a shipped one.**

Precedence is an order over checks, not an order over integers. The shipped order
already proves that a numeric rule cannot work: within one `coverage` run, code 1
can fire before code 2 and also after code 3.

The verified order inside `coverage`, from the top of `coverageCmd` to its last
return, identical in text and JSON mode:

1. Flag validation and configuration errors. Code 1. `main.go:856`, `:873`, `:923`, `:943`.
2. No annotated test file under zero-tolerance. Code 1. `main.go:962-967`.
3. Unknown `--scope` domain. Code 1. `main.go:976-984`.
4. Missing results file under a strict mode. Code 1. `main.go:1013-1024`.
5. Spec parse errors. Code 1. `main.go:1094` in JSON mode, `main.go:1117` in text mode.
6. Annotation rule 1, a criterion with no test at all. Code 2 when
   `settings.annotation.permissive` is `false`. Under `permissive: true` it
   reports and decides nothing, so a later gate chooses the code.
7. Zero-tolerance non-passed. Code 2.
8. Approval gate. Code 3.
9. Coverage threshold. Code 1.
10. Streams-block validation. Code 20.

Items 6 through 10 carry no line reference per output mode, because there is no
longer one per mode. All five are decided by `coverage.GateVerdict`, which
`coverageExitGates` calls once and which the text path and the `--json` branch
both reach. The pairs this list used to name were the two copies that diverged
in `bugs/done/SP-SP-066`.

**The order of items 6 through 10 is the order `GateVerdict` appends them**, and
that is the whole mechanism behind the rule above: the verdict returns the first
non-zero code, so a gate appended later cannot preempt one appended earlier.
Adding a gate to the end is how a new code takes a number without changing what
an existing caller sees.

Steps 1 through 5 are workspace and invocation preconditions. All of them return
code 1, and all of them run before the two gates that have codes of their own. A
gate added at the end of this list preempts nothing.

`sync` reaches the same gates in the same relative order. It no longer carries
its own copy of them: `bugs/done/SP-SP-071` deleted that copy and routed both
commands through `coverage.GateVerdict`, so the order lives in one function and
`sync` inherits it rather than restating it.

### What this means for anyone adding a gate

A gate is only wired when it appears in four places. Both prototypes that shipped
the preemption bug had the first two:

1. The check itself.
2. The phase result, so the output names it.
3. The exit mapping, so the code names it.
4. **Its ordering against gates that already exist.**

The fourth is the one review misses, because from inside the change everything
looks right. `sync` still fails. It fails with the wrong number and the wrong
sentence.

So the parity test in 1A4 asserts over the emitted diagnostic and not only the
integer. Under the broken prototype, a workspace with a red test alongside a
stale deferral and a workspace with the stale deferral alone produced
byte-identical `sync` output. No assertion over integers can separate them,
because the integers agreed.

### The single-verdict rule

The zero-tolerance exit logic used to exist in three hand-maintained copies: the
`coverage` JSON branch, the `coverage` text branch, and a `sync` closure. They
agreed because someone kept them in step, which is not a mechanism, and twice
they stopped agreeing: `bugs/done/SP-SP-066` and `bugs/done/SP-SP-071`.

**There is one copy now.** `coverage.GateVerdict` is a pure function taking
counts and returning every violation in order plus the code, and both commands
build inputs, print what it returns and exit with what it decides. Neither orders
gates, words messages, or picks codes.

**A new gate is added to that function, not beside it.** `spec-coverage` C-44
states the rule for the gate 3A5 adds, and states it as a requirement rather than
a preference, because a private branch returning a code directly reintroduces
both failures at once: the drift between surfaces, and a new code preempting a
shipped one by running earlier than the function that orders them.

## Section 5: propagation

**Rule. Any code that fires on `coverage` fires identically on `sync`, with the
same integer and the same diagnostic, when `sync` reaches its coverage phase.**

`sync` is the CI entry point. A gate that fires on `coverage` and not on `sync`
is a gate that CI does not run.

The trailing clause is an amendment to the rule as the roadmap states it. The
unqualified form is false today, and the reason it is false is correct behavior
rather than a defect. [Section 5.2](#52-where-the-rule-fails-today) has the case.

### 5.1 Where the rule holds

Verified on a fixture workspace with one valid spec, one annotated test, and a
`.specter-results.json` entry whose status is `failed`:

| Invocation | Code |
|---|---|
| `coverage --strictness zero-tolerance` | 2 |
| `coverage --strictness zero-tolerance --json` | 2 |
| `sync --strictness zero-tolerance` | 2 |
| `sync --strictness zero-tolerance --json` | 2 |

All four print the same sentence to stderr. The approval-gate case behaves the
same way and returns 3 in all four invocations, with its own sentence identical
across all four.

Both codes propagate today, in both output formats, in code and in message.

### 5.2 Where the rule fails today

**Precedence beats propagation, and this is not smoothed over.**

`sync` halts at the first failing phase. A workspace whose `check` phase fails
never reaches the coverage phase, so codes 2 and 3 cannot fire there. Measured on
a Tier 1 spec with an orphan constraint and a failing annotated acceptance
criterion:

| Invocation | Code | Reported cause |
|---|---|---|
| `check` | 1 | `orphan_constraint` |
| `coverage --strictness zero-tolerance` | 2 | 1 annotated AC did not pass |
| `sync --strictness zero-tolerance` | 1 | Pipeline failed at check phase |

Same workspace, same strictness, two different codes.

This is the right verdict, because a run whose specs do not pass structural
checks has not earned a coverage verdict. It is the wrong rule as the roadmap
words it. The corrected rule is the one stated at the top of this section:
propagation applies from the point `sync` reaches the coverage phase, and an
earlier phase failure wins by precedence.

The 1A4 parity assertion must be written against the corrected rule. Written
against the unqualified one, it fails on a workspace that behaves correctly.

### 5.3 Same code, different diagnosis

`coverage` and `sync` can return the same integer for the same workspace while
naming different causes. Measured on a workspace with one spec, no annotated
tests, and an empty results file, under `zero-tolerance`:

| Invocation | Code | Message |
|---|---|---|
| `coverage --strictness zero-tolerance` | 1 | `error: zero-tolerance strictness requires at least one annotated test file` |
| `sync --strictness zero-tolerance` | 1 | `FAIL coverage: 1 spec(s) below coverage threshold` |

The integers agree and the diagnosis does not. A parity assertion that binds on
the message, which 1A4 requires, fails here today. Whether the fix is to give
`sync` the specific message or to accept a documented exception is not settled by
this document.

The same shape appears in `bugs/SP-SP-026`. An unreadable specs directory makes
`sync` exit 1 and report that no spec files were found, which is the message an
empty workspace produces. The code is right and the diagnosis points away from
the cause.

### 5.4 Two cases where propagation does not apply

**`sync --only`.** `--only parse`, `--only resolve`, and `--only check` return
before the coverage phase runs, so codes 2 and 3 cannot fire under them. Measured
on a workspace where full `sync` returns 2: `--only parse`, `--only resolve`, and
`--only check` each return 0, and `--only coverage` returns 2. This is the
documented purpose of `--only` and not a propagation failure.

**Flags `sync` does not have.** `coverage --scope` rejects an unknown domain with
code 1. `sync` has no `--scope` flag, so the condition has no `sync` surface.
Usage errors on a flag one command does not accept are outside the propagation
rule.

## Section 6: adding a gate

Follow this order.

1. **Pick a band, not a number.** Orchestration gates take `10` to `19`.
   Evidence stream validation takes `20` to `29`. Spec and coverage contract
   gates take `4` to `9`.
2. **Record the planned number in Section 3 when the spec lands, and add the
   Section 1 row in the same commit as the `os.Exit` that reaches it.** A number
   in the source and not in this document is the double-booking this document
   exists to prevent. A Section 1 row ahead of the code is the opposite failure
   and is now caught: that row asserts the code is Stable, `spec-sync` AC-18
   requires every Stable code to be reachable from some `os.Exit`, and the build
   fails while the gate is still being written. Both directions of `spec-sync`
   C-12 therefore hold at every commit rather than only at the end of a cycle.
3. **Write the check as a pure function** in the package that owns the data, so
   `coverage` and `sync` call the same code rather than copies of it.
4. **Run it after every shipped gate.** Section 4 gives the current order. Insert
   at the end of the relevant phase.
5. **Wire all four places.** The check, the phase result, the exit mapping, and
   the ordering. Missing the fourth is what shipped twice.
6. **Take one verdict for both output formats.** Copy the `checkExitVerdict`
   shape at `main.go:668-672`.
7. **Pin the diagnostic.** Record the exact sentence here. The 1A4 parity test
   asserts on it, and a code that collapses onto a shipped one leaves the message
   as the only channel that separates them.

## Appendix A: how each claim was checked

Every claim above is checkable. Build with `make build` from `specter/`, which
writes `bin/specter`. Do not run `./specter` at the repository root. That binary
is stale, and probing it has already produced one wrong bug severity and one
false alarm, which `bugs/SP-SP-026` records.

**Read from source.** The exit sites, the precedence order inside `coverage` and
`sync`, the panic path, the `watch` signal handler, and the absence of `os.Exit`
in `internal/`. Every one carries a `file:line` above.

**Run against a built binary.** Verified on `release/v0.15.0` at `6c82473` with
`bin/specter`:

- The codes 0, 1, 2, and 3, on a fixture workspace with one valid spec and one
  annotated test.
- `parse --json` returning 0 on a spec that fails to parse. **This result has
  since reversed.** It was fixed on 2026-08-25 as `bugs/done/SP-SP-022`, and
  `parse --json` now exits 1 and still writes its document. Recorded rather than
  deleted, because this appendix is a log of what was measured when, and a
  reader re-running it would otherwise get an answer it does not predict.
- `resolve --json` returning 0 on a dangling `depends_on` reference.
- `check --json` returning 1 with zero bytes on stdout, on a parse error and on a
  resolver error.
- Codes 2 and 3 on `coverage` and `sync`, in text and JSON mode, with their
  stderr sentences compared.
- The precedence case in Section 5.2.
- The message divergence in Section 5.3.
- The `--only` results in Section 5.4.
- The rejected-manifest sweep across nine commands, with the stderr warning
  captured for each of the four commands that continue.
- `watch` returning 0 on SIGINT in a clean workspace, and 1 on a rejected
  manifest before it enters the loop.

**Code 10, added to this document 2026-08-24**, verified on the same branch with
a rebuilt `bin/specter`. A spec pair differing by one removed acceptance
criterion and a major version bump returns 10 with `--exit-code`, 0 without the
flag, and 0 when the two files are identical. The earlier sweep predates the
code, which is why it says four.

**Not verified.** The status the Go runtime produces when a goroutine other than
the main one panics. It is stated as unverified in Section 1 rather than claimed.

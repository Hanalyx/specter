# Specter Exit Codes

This document is the single allocation authority for Specter process exit codes.
No command may emit a code that is not allocated here, and no design track may
pick a number without adding it here first.

Verified against `release/v0.15.0` at `6c82473`, using a binary built with
`make build` from `specter/`. Every claim about current behavior below was
checked by reading `cmd/specter/main.go` or by running `bin/specter`, and
[Appendix A](#appendix-a-how-each-claim-was-checked) says which.

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

Specter emits four codes. There is no fifth.

| Code | Commands | Condition | Standing |
|---|---|---|---|
| `0` | all | The command completed and every gate it ran passed. | Stable |
| `1` | all | Everything else. See the breakdown below. | Shipped, overloaded, frozen |
| `2` | `coverage`, `sync` | Effective strictness `zero-tolerance`, and at least one annotated acceptance criterion has a results-file status other than `passed`. | Stable |
| `2` | all | The process recovered a panic on the main goroutine. | Accidental collision |
| `3` | `coverage`, `sync` | Effective strictness `zero-tolerance`, and at least one acceptance criterion carries `approval_gate: true` with an unset `approval_date`. | Stable |

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

### Code 2, the zero-tolerance non-passed gate

Emitted by `coverage` at `main.go:1104` (JSON) and `main.go:1222` (text), and by
`sync` at `main.go:1358`. All three print the same sentence to stderr before
exiting:

```
error: zero-tolerance strictness — %d annotated AC(s) did not pass
```

That string is the golden value for the propagation parity test in roadmap item
1A4. It must stay identical across all three sites.

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

Emitted by `coverage` at `main.go:1108` (JSON) and `main.go:1226` (text), and by
`sync` at `main.go:1362`. All three print:

```
error: zero-tolerance strictness — %d AC(s) carry approval_gate=true with unset approval_date
```

This is the second golden value for 1A4.

### Commands that can only emit 0 or 1

`parse`, `resolve`, `resolve dependents`, `check`, `reverse`, `init`, `doctor`,
`explain`, `watch`, `diff`, `diff coverage`, `ingest`, `feedback`, and
`pre-push-check`. `grep -rn "os.Exit" --include="*.go" cmd/ internal/` finds exit
sites in `main.go` only, and no package under `internal/` calls `os.Exit`.

## Section 2: where the code and the output disagree

These are the cases the registry exists to make impossible. Each is a place where
the exit code says one thing and the document, the diagnostic, or the truth says
another. They are recorded as current behavior, not as allocations.

| Case | Behavior | Bug |
|---|---|---|
| `parse --json` on a spec that fails to parse | Errors appear in the document. Exit 0. Text mode exits 1. | `bugs/SP-SP-022`, open |
| `resolve --json` on a dependency error | The `dangling_reference` diagnostic appears in the document. Exit 0. Text mode exits 1. | Not filed as of 2026-08-17 |
| `diff` on a change it labels `[breaking]` | The classification is printed and correct. Exit 0. | `bugs/SP-SP-012`, open |
| `diff coverage` on any delta | Exit 0 by design. Documented as a diagnostic surface, not a gate. | Intended |
| `check` and `coverage` on an unreadable specs directory | Reported as a clean workspace with zero specs. Exit 0. `sync` exits 1, but names the wrong cause. | `bugs/SP-SP-026`, open |
| `check --json` when a spec fails to parse, the resolver errors, or the manifest is rejected | Exit 1 with an empty stdout. No document at all. | `bugs/SP-SP-032`, open |
| `coverage --json` when the manifest is rejected | Exit 1 with an empty stdout. | `bugs/SP-SP-032`, open |
| A rejected `specter.yaml` | `check`, `coverage`, `sync`, `doctor`, and `watch` exit 1. `parse`, `resolve`, `resolve dependents`, and `explain` warn on stderr and exit 0, through `warnManifestRejected()` at `main.go:422`, `:476`, `:618`, and `:2612`. | `bugs/SP-SP-017`, open |

`check --json` was the same defect until this cycle. It was fixed by
`checkExitVerdict` at `main.go:668-672`, which both the text branch and the JSON
branch now end on, so the verdict cannot differ by rendering. That function is
the pattern to copy.

Two rules follow for anyone adding a gate:

- **A `--json` branch takes the same verdict as its text branch.** Route both
  through one function. Do not restate the rule in the branch.
- **An exit code and the emitted document must agree.** If a command cannot do
  the work, it says so in the document rather than writing nothing.

## Section 3: band allocation

Bands exist so a track can allocate a number without asking, and without
colliding. Only shipped behavior gets a specific number. A planned gate gets a
band and nothing more.

| Band | Purpose | Allocated to |
|---|---|---|
| `0` | Success | Shipped |
| `1` | Unclassified failure. Frozen. | Shipped |
| `2` to `9` | Spec and coverage contract | `2` and `3` shipped. `4` to `9` free. |
| `10` to `19` | Orchestration gates | None allocated |
| `20` to `29` | Evidence stream validation | None allocated |
| `30` to `63` | Unallocated | Reserved for a future track |
| `64` to `78` | Usage, internal, and configuration errors, per `sysexits.h` | None allocated |
| `79` to `125` | Do not use | Reserved |
| `126` and above | Do not use | Owned by the shell and the operating system |

The band edges are the roadmap's proposal, and the shipped codes fit them without
change. Codes 2 and 3 both gate the spec and coverage contract, and both sit
inside `2` to `9`. No shipped code lands in the orchestration or evidence bands,
so neither band is contended.

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

**A band is not a promise.** Nothing occupies 10 to 29 today. Anyone reading a
code in that range from a released binary has found a bug or a stale build.

### Where the planned gates belong

Named here so no track has to guess, and no number is assigned until the gate
ships.

| Planned gate | Roadmap | Band |
|---|---|---|
| Failing exit for `diff` on a breaking change | 2B3 | Orchestration, `10` to `19` |
| Stream validation refusing a differential | 3B4 | Evidence stream, `20` to `29` |
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
6. Zero-tolerance non-passed. Code 2. `main.go:1102`, `main.go:1220`.
7. Approval gate. Code 3. `main.go:1106`, `main.go:1224`.
8. Coverage threshold. Code 1. `main.go:1111`, `main.go:1230`.

Steps 1 through 5 are workspace and invocation preconditions. All of them return
code 1, and all of them run before the two gates that have codes of their own. A
gate added at the end of this list preempts nothing.

`sync` runs the same two zero-tolerance gates in the same relative order, inside
its coverage phase, at `internal/sync/sync.go:274-293`. The threshold check
follows them at `:295`.

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

The zero-tolerance exit logic exists in three hand-maintained copies today: the
`coverage` JSON branch, the `coverage` text branch, and the `sync` closure at
`main.go:1355-1364`. They agree because someone kept them in step, which is not a
mechanism.

`checkExitVerdict` is the shape to copy: one function, both formats, no restated
rule. Roadmap item 1C2 does this for the per-criterion verdict, so `sync`
inherits classification rather than repeating it. **A new gate is written once as
a pure function and called from every surface.** Do not add a fourth copy.

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
2. **Add a row to Section 1 before writing the code.** A number in the source and
   not in this table is the double-booking this document exists to prevent.
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

- The four codes, on a fixture workspace with one valid spec and one annotated test.
- `parse --json` returning 0 on a spec that fails to parse.
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

**Not verified.** The status the Go runtime produces when a goroutine other than
the main one panics. It is stated as unverified in Section 1 rather than claimed.

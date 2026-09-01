# SSRB-103: multi-stream evidence in ingest and coverage

Status: ACCEPT. **The accepted foundation shipped 2026-08-27**: `ingest --stream` and `--merge`, the `streams` block, the merge key gaining `stream`, and exit code 20 for an inconsistent block. Sections 2 and 5 describe the gap as it stood at decision time, not as it stands now for the foundation; policies NEEDS-DESIGN.
Decided: 2026-08-24 for the foundation.
Source: `features/SP-002-ingest-live-host-verification` (Kensa, 2026-07-01), plus an unfiled OpenWatch request drafted 2026-08-14

**Decision summary.** The shared foundation is accepted: Specter results may
carry labeled evidence streams, with unlabeled legacy entries reading as the
`default` stream. `stream` is a label only. The artifact does not carry kind,
role, baseline/current, unit/live, mutation, or policy meaning. Producers set the
label, for example through `ingest --stream`, and consumers may choose roles by
invocation in later policy commands.

Conjunction policy and red-first differential policy remain NEEDS-DESIGN. This
brief accepts the multi-valued result model they both need; it does not decide
whether coverage may require a conjunction of streams or whether Specter may
compute a red-first verdict between streams.

## 1. Request

Two requests from unrelated projects, each asking coverage to see more than one
body of evidence.

**Kensa** wants a second *kind* of evidence. Unit results prove a test passed;
their ledger records that remediation and rollback were byte-perfect on a real
host. They propose `specter ingest --kind=verification <ledger.json>`, a coverage
column distinguishing `unit` from `live-verified`, and a tier policy able to
demand live evidence for Tier 1.

**OpenWatch** wants a second *run* of the same kind. They propose ingesting a
baseline run alongside the implementation run, so an acceptance criterion whose
test passes in both can be reported as vacuous, plus a `proven` strictness level
and per-AC exemption fields for criteria with no absent-implementation state.

## 2. Origin

Kensa's pain is that the strongest evidence they produce is invisible to the tool
that decides whether a spec is covered. The ledger exists and is gated
independently, so the gate that customers and auditors care about is not the gate
Specter runs.

OpenWatch's pain is that a test asserting nothing satisfies every gate they run,
and they run all of them. Their stated reason it became urgent is that one AI
session now writes the spec, the tests, and the implementation, so no part of the
pipeline is adversarial to any other part.

**Distinguish the pain from the shape, because the two shapes are not the same
operation.** Kensa wants a conjunction: an AC is adequately covered when two
kinds of evidence both exist. OpenWatch wants a differential: an AC is proven
when a result is present in one run and absent or failing in another. Conjunction
and differential are different policies and should not be conflated.

What they share is the foundation underneath. **The results model is
single-valued.** It records one status per `(spec_id, ac_id)` pair, and merging
collapses duplicates to the worst status. Neither policy can be expressed until a
pair can carry more than one labeled result. That foundation is the subject of
this brief. The two policies are named here, and deliberately not decided.

## 3. Universality

**UNIVERSAL for the foundation. UNCLEAR for each policy.**

Two unrelated projects arrived at the same structural need from opposite
directions, one wanting more kinds of evidence and one wanting more runs of one
kind. Neither request demonstrates universality alone, and each read on its own
looks like one project's domain leaking into the tool.

The generalization beyond both is immediate. Unit, integration, and end-to-end
are distinct evidence streams that most projects already produce and none can
currently describe to Specter.

Specter's own dogfood gate, which flattens Go and JavaScript runner output into
one stream, is the natural first test but not an independent universality vote.
The stronger in-product argument is orchestration: roadmap Phase 4 expects the
orchestrator to run `ingest` per mutation without new Specter code, and that is
not true while global duplicate collapse turns many runs into one
indistinguishable body of evidence. Per-run ingest needs the stream foundation.

This is the first of the currently open briefs whose section 3 is satisfied
rather than pending. SSRB-101 and SSRB-102 both wait on a second requester. This
one has two for the foundation.

The policies are a different matter. Conjunction has one requester and
differential has one requester, and each should meet the bar on its own before it
ships. Accepting the foundation does not accept either policy.

## 4. Cost of acceptance

| Surface | Impact |
|---|---|
| Canonical spec schema | None for the foundation. The differential policy separately proposes per-AC exemption fields, which is an AC shape change and needs its own decision. |
| In-memory type model | The result entry gains a stream label. The merge rule becomes per-stream rather than global. |
| JSON contract | The results file is a published artifact that consumers write. A label must be optional, with unlabeled entries reading as the default stream, or every existing producer breaks. |
| CLI surface | `ingest` gains `--stream` to label the entries one invocation produces, and `--merge` to build an output from only the named inputs. `--merge` is an exception to the accumulate rule `spec-ingest` C-11 requires today, so that constraint needs an amendment. |
| Manifest schema | Only if a policy ships. Requiring a stream, or adding a strictness value, is where enum pressure lands. The foundation alone needs no manifest change. |
| Reference documentation | The annotation and ingest references both describe a single evidence path throughout. |
| Existing user specs | None. |
| Editor surfaces | The coverage view shows one status per AC and would need to show which stream produced it. |
| Dogfooded specs | None, though the dogfood gate itself becomes a two-stream case and is the natural first test. |

Two costs are not surfaces.

**Evidence Specter cannot authenticate.** A consumer produces every stream.
Specter would gate on a file it cannot verify was generated by the run it claims
to describe. A second stream that silently failed to run must not read as
success, and the failure direction matters: a missing stream defaulting to
satisfied produces maximum confidence exactly when the evidence is most broken.
Any design needs a defined semantics for absence and a guard against a stream
that is empty or implausibly small.

**A coverage number that means different things per project.** Once coverage can
be computed against a configurable set of streams, two teams reporting 100% may
be making different claims. This is the same portability concern raised in
SSRB-102, and it argues for the stream set being visible in the report rather
than only in configuration.

## 5. Existing coverage

**No existing coverage.** The strictness ladder governs how hard a single stream
is judged, not how many exist. The `--scope` flag narrows which specs receive
strict treatment, not which evidence they are judged against. The `approval_gate`
fields record a human act rather than a machine-produced stream, and are the
closest existing thing only in that they are a second condition on the same AC.

The merge rule is the clearest evidence of the gap. It collapses duplicate
results to the worst status, which is correct within one stream and wrong across
two, because a failing integration result and a passing unit result are not a
contradiction to resolve. They are two facts.

## 6. Alternatives

**Multiple results files, labeled at the command line.** Accept
`--results unit=a.json --results live=b.json` and let the label come from the
invocation rather than the file. No format change and no producer breakage.
Trade-off: the labeling lives in CI configuration rather than in the artifact, so
a results file is no longer self-describing and two invocations can disagree
about what a file means. This is the strongest alternative and may be the answer.

**Corrected 2026-08-23.** This alternative was first written as letting each
stream stay "a file some existing tool already writes," and that was carried into
the Context Plane response to Kensa as the cheapest path to their item 1. It is
wrong on the filing's own evidence. Kensa's ledger is keyed
`rule_id` / `os` / `scope` / `host` / `verified_at`; the results artifact is
keyed `(spec_id, ac_id)` with a status per entry. Command-line labeling decides
which stream a file belongs to. It does not teach `ingest` to read a shape it has
no adapter for, so that ledger needs a translation step under either fork.

Two consequences, and the second is what unblocked this brief.

The alternative is still the strongest one, but on its own merits rather than on
zero cost to this requester. What it buys is no artifact change and no producer
breakage. What it does not buy is a requester keeping the file they have.

And **section 8's first reconsideration trigger cannot be met by asking Kensa.**
That trigger, and the first of the two questions put to them on 2026-08-15, both
turn on whether their records map one to one onto `(spec_id, ac_id)` pairs. They
do not, and the answer is the same under both branches, so it does not
discriminate. The fork is a Specter-internal question about whether a results
file must be self-describing, and it is decided here.

**Accepted 2026-08-24: self-describing artifact plus producer flag.** Result
entries may carry `stream`; missing `stream` means `default`. `ingest --stream`
writes that label when producing results. This keeps the artifact
self-describing without making the label a policy role. `stream` names a body of
evidence, not whether it is baseline, current, unit, live, mutation, or any other
kind.

**Where `--merge` comes from.** Neither request asked for it, and it appears in
section 7 as part of the accepted shape, so its origin belongs here.
`spec-ingest` C-11 requires repeated `--junit` and `--go-test` flags to
accumulate into an existing output. That is correct for assembling one run from
several files and wrong across runs. A criterion that passed in the previous run
and produced no entry in this one keeps its stale passing entry, which hides the
absence this foundation exists to make visible. `--merge` builds the output from
only the named inputs, so a re-run replaces a stream rather than layering on top
of it. It is a new CLI flag and an amendment to C-11, not a free consequence of
the label.

**Consumers gate separately, as today.** Kensa's current position. The ledger is
maintained and enforced outside Specter. Trade-off: the evidence never reaches
the coverage matrix, every project rebuilds the same wiring, and the claim
Specter reports is weaker than the claim the project can actually make.

**One stream, richer status values.** Extend the status enum to carry
`passed-live` or similar. Trade-off: the enum becomes a cross product of outcome
and provenance, and it grows every time a project has a new evidence kind. This
is the shape to avoid.

**Encode the stream in the identifiers.** Suffix the spec or AC id per stream.
Trade-off: breaks the join with annotations and corrupts the graph. Rejected.

**Do nothing until one policy is decided.** Trade-off: whichever policy is
designed first will shape the foundation around its own semantics, and the other
will fit badly. This is the outcome the brief exists to prevent, and it is the
reason the foundation is posed separately from the policies.

## 7. Decision

**ACCEPT for the foundation. Policies remain NEEDS-DESIGN.**

Section 3 is satisfied for the foundation. Two unrelated projects independently
require a multi-valued result model, and the generalization to unit,
integration, and end-to-end reaches most projects. Section 5 confirms nothing in
the toolchain expresses this, and that the existing merge rule actively destroys
the distinction.

The accepted foundation has this shape:

- Result entries MAY carry optional `stream`.
- Missing `stream` means `default`.
- The merge key becomes `(spec_id, ac_id, stream)`.
- The results artifact carries a top-level `streams` array so a stream that ran and
  produced zero results is distinguishable from one that never ran.
- `ingest --stream <name>` writes one stream label into produced entries.
- `ingest --merge <file>...` builds a new results file only from the named
  inputs and does not accumulate into an existing output.
- `stream` is a label only. The artifact MUST NOT encode a policy role or kind,
  such as baseline, current, unit, live, or mutation.
- Validation belongs in shared pure code so `coverage` and `sync` inherit the
  same failures.

**The verdict does not change in this cycle.** Section 5 objects that collapsing
to the worst status across two streams destroys two facts. The foundation fixes
the artifact and not the verdict. Entries keep both facts, because the merge key
is per stream, while coverage still fails a criterion when any stream that
reported on it reports a non-passing status. Splitting one body of evidence into
labeled streams must leave the report identical field for field. Deciding what
the verdict should do with two facts is the conjunction and differential work,
and it stays undecided here.

Absence is part of the accepted foundation. A criterion is covered only when at
least one stream reports an entry and every stream reporting an entry reports
`passed`. A criterion no stream reported on is not covered. A declared stream
with zero entries is visible in the `streams` array rather than disappearing into the
same shape as a stream that never ran.

Whether coverage may require a conjunction of streams, and whether Specter may
compute a red-first differential between streams, are separate questions with one
requester each. Each should clear the universality bar and amend the relevant
specs on its own terms. Deciding a policy here would repeat the error recorded in
SSRB-098, where a trigger written from one requester's shape could never fire for
the need underneath it.

## 8. Reconsideration triggers

- Evidence that self-describing stream labels in the artifact break existing
  producers or consumers in a way command-line labeling would not.
- A policy tries to encode role or kind into the `stream` field, which would make
  the accepted foundation carry policy meaning it deliberately excludes.
- Conjunction accumulates a second independent requester, which would let that
  policy be scoped on its own.
- Differential/red-first accumulates a second independent requester, which would
  let that policy be scoped on its own.
- The dogfood gate is converted to two streams and reveals a foundation
  requirement neither request anticipated.

## 9. References

- `features/SP-002-ingest-live-host-verification` in the Context Plane, held open
  and reframed on 2026-08-15 pending this brief.
- The OpenWatch request is drafted but not yet filed, so it carries no id. Its
  first ask, red-first as a second ingest, is the differential policy named here.
- `docs/design/2026-08-24-phase-3-consensus.md`, the panel recommendation that
  accepted the foundation shape and deferred both policy layers.
- Related SSRBs: SSRB-098 on posing a question from the need rather than from one
  requester's shape; SSRB-102 on configuration that makes a report mean different
  things in different projects.
- Related specs and code: the ingest package's result model and merge rule, the
  coverage strictness path, and the dogfood gate that already ingests two runners
  into one stream.

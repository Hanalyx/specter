# SSRB-102: `settings.diagnostics`, per-rule severity in the manifest

Status: NEEDS-DESIGN
Decided: TBD (target: alongside or after the SP-004 resolution)
Source: internal design discussion, 2026-08-15, arising from SSRB-101 Ask 1 analysis

## 1. Request

Add a `settings.diagnostics` map to the manifest, keyed by the diagnostic names
`check` already prints in brackets, valued by severity:

```yaml
settings:
  diagnostics:
    structural_conflict: warning
    non_concrete_ac: off
```

Precedence would be defined once: an explicit entry wins, then `settings.strict`,
then the per-tier default. The proposal is a severity layer, not a new rule.

## 2. Origin

Two unrelated threads converged on the same missing capability.

The first is SP-004. The structural conflict scan matches substrings rather than
meaning, and produces reproducible false errors on ordinary phrasing. It is an
error-severity diagnostic, so it fails the build. An adopter hitting it today has
no way to silence it. The only remedies are to patch the tool, reword a correct
spec to satisfy a string matcher, or stop running `check`.

The second is the concreteness gate proposed by an adopter for acceptance
criteria. That proposal carried its own two-axis severity configuration. Working
through it surfaced that the configuration was not really about concreteness. It
was a per-rule severity layer with one rule's name on it, and the next
opinionated rule would need to invent the same thing again.

Distinguish the pain from the shape. The pain is that a project cannot say
anything about a diagnostic as a class. The proposed shape is one of several ways
to give it that.

## 3. Universality

**UNCLEAR.**

The argument for universal is precedent. Every comparable tool ships per-rule
control, and teams reach for it as soon as a tool has opinions. Specter has also
begun inventing it ad hoc: the coverage strictness setting already modulates the
severity of exactly one diagnostic, which is per-rule severity arriving without
a general mechanism.

The argument against is that nobody has asked. Zero external requests. The
motivating defect was filed by the Specter agent against itself, and the adopter
whose proposal surfaced the gap did not request suppression and would not need it
at the tier their corpus occupies. A capability justified only by its author's
analysis has not met the bar this project applies to every other request.

A decision to ship requires the same evidence SSRB-101 waits on: a request from
a project that is not the filer.

## 4. Cost of acceptance

| Surface | Impact |
|---|---|
| Canonical spec schema | None. This is a manifest change only. |
| Manifest schema | New nested map, open-keyed by diagnostic name. Unknown-key validation must decide whether an unrecognized diagnostic name is an error or ignored. |
| In-memory type model | New settings field, plus a resolver that applies precedence. |
| JSON contract | Diagnostics carry a resolved severity today. Consumers would need to know severity is now project-configurable, not derivable from tier. |
| Reference documentation | A precedence table covering the interaction with the global warnings-as-errors setting and with tier. This is the expensive part, because it is a rule users must hold in their heads. |
| Existing user specs | None. Absent the setting, behavior is unchanged. |
| Editor surfaces | Completion for diagnostic names; severity no longer inferable client-side. |
| Dogfooded specs | None expected. |

Two costs are not surface costs and matter more.

**It creates a way to silence a true defect.** Every current diagnostic reports a
structural fault. A project that suppresses one to escape a false positive also
suppresses the true positives, with no signal that it did.

**It erodes portability of meaning.** The stated value of a tier is that it means
the same thing across products. A green `check` carries the same property. Once
severity is per-project configuration, two teams can both report a passing check
while enforcing different things, and the word stops being shared. This cuts
directly against the mission argument that justifies most of the toolchain, and
it is the strongest reason to be cautious.

## 5. Existing coverage

**Partial, and fragmented.** Three layers exist: a global warnings-as-errors
switch, a per-spec tier gradient, and a per-instance enforcement override on an
individual constraint. A fourth, coverage strictness, sets the severity of one
named diagnostic as a side effect of governing something unrelated.

So the toolchain already answers "how loud" at three scopes and answers it for
one specific rule by accident. What it does not answer is "how loud for this rule
across my project." The gap is real. The fragmentation is itself an argument that
the layer is being invented incrementally and would be better designed once.

## 6. Alternatives

**Fix the motivating defect directly.** SP-004 lists four remedies, the first
being a one-line severity downgrade so an unsound heuristic warns instead of
failing builds. This resolves the entire motivating case with no schema change,
and it is strictly better than letting every adopter configure around a defect
the tool should not have. Cost: does nothing for the general problem.

**Downgrade unsound checks as a standing policy.** Generalize the above: any
diagnostic that cannot promise a low false positive rate ships at warning, not
error. This is a claim-discipline fix rather than a feature, and it addresses the
root cause the request is a workaround for. Cost: does not help a project that
disagrees with a rule which is working correctly.

**A command-line flag rather than a manifest field.** Suppression as an invocation
detail keeps the manifest schema untouched. Cost: not versioned, not diffable, not
visible in review, and CI configuration drifts from what the repository declares.
The properties that make a spec field valuable are exactly the ones this loses.

**Do nothing pending a request.** Hold the analysis, fix defects at their source,
and wait for a project to state the pain. Cost: the next opinionated rule may
ship its own private configuration first, which is the outcome this brief exists
to prevent.

## 7. Decision

**NEEDS-DESIGN**, and explicitly not accepted on the evidence presented.

Section 3 is the blocker. The universality case rests on tooling precedent and on
the filer's own analysis, with zero requests from any project, which is the same
standard that holds SSRB-101 open. Section 5 confirms the gap is real and that the
layer is already being invented piecemeal, which is why this is held for design
rather than rejected. Section 4 records a cost that is not paid in surfaces: a
per-project severity map lets a project silence true defects and weakens the
claim that a passing `check` means the same thing everywhere, which is the
property most of the toolchain exists to protect.

Decisively, the motivating case does not require this change. SP-004 is better
served by fixing the unsound check than by shipping a mechanism for adopters to
configure around it. A defect should not become the justification for a schema
commitment when a direct remedy is one line. This brief should not be cited as
the reason to leave SP-004 unfixed.

## 8. Reconsideration triggers

- A project that is not the filer requests per-rule severity control, naming the
  diagnostic and the workflow it blocks.
- A second opinionated diagnostic, one that reports a preference rather than a
  defect, reaches the point of needing configuration. Two such rules make the
  general layer cheaper than two private ones.
- The concreteness rule ships and a project running the global warnings-as-errors
  setting reports being unable to adopt the release that introduces it.
- SP-004 is resolved in a way that leaves adopters needing suppression anyway.
- The post-v1.0 stability window opens, changing the cost calculus for manifest
  additions.

## 9. References

- `bugs/SP-SP-004-structural-conflict-scan-produces-false-positives-against-c-05`
  in the Context Plane; carried as open item 4 in the project memory.
- Related SSRBs: SSRB-101 (source-file governance, NEEDS-DESIGN on the same
  universality standard).
- Related specs and code: the check package's severity resolution, the manifest
  settings validation, and the coverage strictness setting that already modulates
  one diagnostic's severity.

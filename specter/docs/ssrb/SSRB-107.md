# SSRB-107: Keep, replace, or retire `constraint.validation`

Status: NEEDS-DESIGN  
Decided: Pending owner review  
Source: SP-117 design discussion

## 1. Request

Decide whether to keep, replace, or retire the optional `constraint.validation` object before the schema lock. The owner wants authors to be able to require or waive verification evidence with an optional boolean named `evidence_required`. An explicit waiver must carry a reason in `evidence_waiver_reason`. The field ships only with a report that reads it. A separate proposal would let a checker compare a criterion's example with a constraint it cites. The two serve different decisions and can coexist.

## 2. Origin

Specter's file-size cap illustrates the reviewer's problem. In `spec-check`, C-12 requires the CLI to check the size before reading a test file. AC-20 cites C-12 and also requires a warning with the path and byte count. The CLI test exercises the cap and checks warning text and continued scanning. Reading the file first and then skipping it could produce the same output. The at-cap subtest in `check_test.go` checks for the absence of a skip warning. A silently skipped file could pass that assertion. A reviewer needs to know which observations support each claim.

The reference gives `password / min / 8` and `yaml_features / enum / [anchors, aliases]` as examples of validation. The first leaves the unit open. The second accompanies a requirement to support both YAML features. An enum of allowed values cannot establish support for both. Typing the operand alone leaves both ambiguities unresolved.

## 3. Universality

Verdict: UNCLEAR for a new schema shape. Any project with mandatory constraints needs a way to review them. Specter supplies a real case for reviewing evidence. The integer example below illustrates a separate need for checking examples. Evidence of demand for that check across independent projects is still missing.

## 4. Cost of acceptance

The current schema accepts an optional object with three required members. The Go model preserves the operand as an open value. The reverse compiler emits the object, including numeric bounds captured as strings by the TypeScript adapter. A tighter rule would affect existing drafts, the parser, reverse output, serialized specs, editor tooling, the reference, AI templates, and fixtures. Adopter usage remains unknown.

For a replacement, compare representative old specs and generated drafts with the proposed reader. A legacy rule describes a predicate. Neither its presence nor its absence records an author's evidence policy. Migration must preserve an omitted `evidence_required` unless the author makes a choice. For removal, SSRB-105 offers a precedent for a warning period and migration guidance. A removal plan would also need to retire the writer and preserve the author's intent before dropping the field. Neither path has an approved schedule.

Adding `evidence_required` and `evidence_waiver_reason` reaches the canonical schema, the Go model, the parser's diagnostics, the reference, AI templates, and fixtures. It also needs the per-constraint report as its reader. A field with no reader would enter the locked schema inert, which is the defect this brief exists to fix.

## 5. Existing coverage

`references_constraints` links a criterion to a constraint. The checker uses those links to find orphans. Coverage classifies criteria using annotations and, where the mode requires it, test results. These records could support a report listing each constraint, its cited criteria, and their available results. A reviewer would still need to inspect whether the assertions expose each violation.

The stronger test for a file at the cap runs under `spec-sync/AC-14`. Its boundary file supplies the only annotation and the test requires the coverage phase to pass. A report that follows only references to `spec-check/C-12` would miss that evidence. Per-constraint evidence reporting belongs under SP-114.

C-12 also shows the limit of any link-based report. No test asserts the byte count in the warning, and no test observes whether an oversized file is read. Those gaps were found by inspecting the tests, not by following links. A report can list the criteria and records it found, and give a reviewer a place to start. It cannot show which obligation a test observed.

Specter already records human sign-off on a criterion through `approval_gate` and `approval_date`. SSRB-098 rejected a per-criterion `status` field partly because it overlapped with those fields. Human review of a constraint uses that route first. A criterion names the reviewed obligation, cites the constraint, and carries the approval fields. The report can then say that AC-20 cites C-12 and records human approval on a given date. That statement is traceable. It does not make all of C-12 verified, because the other obligations still need their own evidence. `evidence_required` records an evidence policy for a constraint, not a sign-off, so it does not overlap with the approval fields.

The spec parser already checks the shape of a validation declaration. It accepts several operand types without tying them to individual rules. The reference and schema description claim deterministic checking by `spec-check`. Correcting that claim is useful under every option.

## 6. Alternatives

**Declare whether evidence is required.** The agreed design uses an optional boolean at `spec.constraints[].evidence_required`. Its scope is verification of the implementation. The constraint keeps its stated force under each choice. For each mandatory constraint, the author should be able to describe an observation that would reveal a violation. If the author cannot identify one, clarify the constraint before deciding whether to require or waive evidence. One test may supply evidence for several obligations. A review or another check can also supply evidence if it could detect the violation.

| Author's declaration | Meaning for review |
|---|---|
| `evidence_required: true` | The author requires evidence. Surface missing evidence when the reviewer has no observation that supports the obligation. |
| `evidence_required: false` | The author explicitly waives the evidence requirement. Show the waiver and exclude the waived constraint from the verified count. A mandatory constraint still binds the implementation. |
| Omitted | The author made no explicit choice. Preserve that distinction when reading and writing existing specs. The default policy for newly authored specs is open. |

**Waiver reason.** When `evidence_required` is `false`, `evidence_waiver_reason` is required and must contain a non-whitespace character. When the policy is `true` or omitted, a reason is rejected, so changing the policy cannot leave a stale waiver behind. The parser checks that a reason exists. The reviewer decides whether it justifies the waiver.

Merge the following members into the existing constraint definition. Add the two property declarations alongside the existing properties. The condition checks key presence so older specs can omit the policy. The reason pattern uses the Unicode White_Space character set. Specter's Go validator accepts a reason consisting of a single nonbreaking space with the shorter `\S` pattern.

```json
{
  "properties": {
    "evidence_required": { "type": "boolean" },
    "evidence_waiver_reason": {
      "type": "string",
      "pattern": "[^\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]"
    }
  },
  "if": {
    "required": ["evidence_required"],
    "properties": { "evidence_required": { "const": false } }
  },
  "then": {
    "required": ["evidence_waiver_reason"]
  },
  "else": { "not": { "required": ["evidence_waiver_reason"] } }
}
```

**What the report must show.** These rows are candidate criteria for SP-114.

| Declaration | What the report shows |
|---|---|
| `false` with a reason | Waived, with the reason visible. The constraint is excluded from the verified count. |
| Omitted | No policy declared. The constraint counts as neither verified nor waived. Linked criteria and their results still appear. |
| `true` | Show the linked criteria, available run results and dated criterion approvals. Identify missing records. The reviewer judges which obligations those records support. |

For example, waiving runtime evidence for C-02's maximum of 20 would still leave a declared example of 22 in conflict with C-02. A schema-level example check would apply independently of the waiver. The existing `enforcement` field governs diagnostic severity. It cannot express the author's choice about evidence.

**Keep the object as descriptive metadata.** Preserve existing drafts and describe exactly what the parser accepts. Identify who uses the structured metadata and what they can decide from it before locking in that meaning.

**Retire the object.** Authors can state observable obligations in constraints and link focused criteria to them. A criterion can describe evidence such as a byte count in a warning or a read that must never occur. This approach relies on review to catch contradictions in examples.

**Bind a typed predicate to a named example value.** Consider the predicate and the example it reads in the same decision. The existing `inputs` and `expected_output` maps can already contain integers. One candidate is to type only a value targeted by a predicate. Requiring a type declaration for every example key would have a wider migration cost.

| Candidate contract element | Requirement and rationale |
|---|---|
| Target | Identify the criterion side (input or expected output) and key. This gives the checker one value to inspect in a citing example. |
| Operation and operand | Define the type, comparison, inclusivity, and unit where relevant. An inclusive integer maximum accepts an integer bound. String length would require its own operation and unit. |
| Check result | Distinguish a satisfied comparison, a violated comparison, and an example the checker cannot evaluate. A missing target or wrong type cannot establish satisfaction. Decide how examples that intentionally describe error outcomes declare applicability. |

Worked example for review: a criterion promises integer `y = x + x`; C-01 requires `y >= 1` and C-02 requires `y <= 20` for successful outputs. For `x = 10, y = 20`, both comparisons pass. For `x = 11, y = 22`, the proposed checker would identify the criterion, C-02, the output key, the observed 22, and the inclusive maximum of 20. For `x = 3, y = 8`, both comparisons pass even though the arithmetic is wrong. The feature would check supplied example values. The author still defines the input domain and the behavior outside it. Application tests supply runtime evidence.

## 7. Decision

NEEDS-DESIGN. Ship the policy with or after the SP-114 report. The release test starts with an existing constraint, shows “No policy declared,” adds a waiver and checks its displayed reason, then switches to required evidence and removes the reason. With no linked result or dated approval, the final report must identify the missing evidence. Section 3 leaves demand for typed comparisons open, and section 5 describes the limits of evidence links. Before adopting a schema change, settle the default for new specs, test the migration cases in section 4, and review the typed comparison before choosing the fate of `validation`.

## 8. Reconsideration triggers

- An adopter provides a spec where a typed example check catches a meaningful error and identifies who acts on the diagnostic.
- A consumer demonstrates a decision it makes from the current validation object and supplies a fixture for compatibility review.
- A trial report using existing criterion links and results shows which evidence a reviewer can identify and which obligations still need a separate observation.

## 9. References

- [Constraint validation examples](../SPEC_SCHEMA_REFERENCE.md#constraint-validation-object) and [canonical schema](../../internal/parser/spec-schema.json)
- [Checker constraints and criteria](../../specs/spec-check.spec.yaml), [CLI assertions for AC-20](../../cmd/specter/check_test.go), and [sync assertions for AC-14](../../cmd/specter/sync_test.go)
- [Criterion verdicts](../../internal/coverage/verdict.go) and [orphan detection](../../internal/checker/check.go)
- [Reverse compiler](../../internal/reverse/reverse.go) and [TypeScript adapter](../../internal/reverse/adapter_typescript.go)
- [SSRB-105, registry retirement](SSRB-105.md), [SSRB-098, per-criterion status](SSRB-098.md), and [schema triage policy](../TRIAGE_DISCIPLINE.md)

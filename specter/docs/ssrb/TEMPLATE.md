# SSRB-NNN: <Title>

Status: ACCEPT | REJECT | DEFER (vN.M) | NEEDS-DESIGN
Decided: YYYY-MM-DD
Source: <GH issue, internal design call, etc.>

## 1. Request

The change as proposed, in the requester's terms. Quote the concrete proposal if one was offered. One paragraph.

## 2. Origin

The real-world pain that drove the request, and where it surfaced. Distinguish the pain from the proposed shape.

## 3. Universality

Pain shared across projects, or specific to one? Would 3+ unrelated projects find this useful?

Verdict: UNIVERSAL | SINGLE-PROJECT | UNCLEAR

## 4. Cost of acceptance

Surfaces a schema change reaches:

- The canonical schema definition
- The in-memory type model
- The JSON contract consumed by editor extensions and third-party tools
- Reference documentation and instructional templates
- Existing user specs (migration burden)
- Editor surfaces — completion, hover, sidebar
- Dogfooded specs

Note significant impact per surface; default is "minimal" if unstated.

## 5. Existing coverage

Does the toolchain already answer this through another mechanism? Common answers: coverage tooling, approval-gate fields, priority, tags, reverse linking, migration adapter. Name the mechanism if yes; state "no existing coverage" if not.

## 6. Alternatives

Non-schema paths to the same pain. List candidates and the trade-off each carries. Explain which was chosen and why.

## 7. Decision

The verdict and the reasoning. Reference §3, §4, §5 explicitly. Three to five sentences.

## 8. Reconsideration triggers

Concrete criteria that would prompt a fresh review (e.g., a second unrelated project surfacing the same pain; a prerequisite tool landing; the post-v1.0 schema-stability window opening).

## 9. References

- Source issue or thread
- Related SSRBs
- Related specs and code

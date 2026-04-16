# AI Prompts for Specter

Specter's schema is detailed by design — but you are not meant to write specs by hand. The intended workflow is a collaboration: **you provide intent, your AI coding assistant translates it into a full spec and tests, you review and approve.**

These prompts are ready to paste into Claude, Cursor, Copilot, or any AI coding assistant. Use them in order for a new feature, or individually when you need a specific step.

---

## 1. Intent → Spec

The starting point for every feature. You describe what you want; the AI produces the full `.spec.yaml`.

```
I want to build [module name]. Here is my intent:

[2-3 sentences describing what it does]

Key constraints I care about:
- [constraint 1]
- [constraint 2]

Non-obvious decisions / trade-offs:
- [anything the AI shouldn't guess about]

Generate a complete `.spec.yaml` for this using Specter's schema.
- id: [kebab-case name]
- status: draft
- tier: [1 = security/money, 2 = business logic, 3 = utility]
- Use MUST / MUST NOT language for constraints
- Generate ACs that cover each constraint, including error cases
- Do not invent requirements I haven't mentioned
```

---

## 2. Review a Generated Spec

Run this before approving any spec. The spec is the approval gate — problems caught here cost nothing. Problems caught after implementation are expensive.

```
Review this Specter spec before I approve it.

Check for:
- Are all constraints actually testable?
- Are there obvious missing edge cases in the ACs?
- Does the objective accurately match the constraints?
- Is the tier appropriate for this module's criticality?
- Are any constraints redundant or contradictory?
- Would anything here surprise a developer implementing it?

Be direct — flag problems, don't just validate.

[paste spec]
```

---

## 3. Spec → Tests

Once the spec is reviewed and approved. Tests are derived directly from the ACs — no guessing, no scope creep.

```
Using this Specter spec as the contract, write [Go/Python/TypeScript] tests
for every acceptance criterion.

Rules:
- Each test function must have // @spec [spec-id] and // @ac [AC-id] annotations
- One test function per AC minimum
- Tests must be executable — no pseudocode, no TODOs
- Cover both the happy path and the error cases defined in the spec
- Do not test anything not described in the spec

[paste spec]
```

---

## 4. Spec → Implementation

The AI implements against the spec as a contract. The tests already exist and define what done looks like.

```
Implement [module name] using this Specter spec as the contract.

Rules:
- Every C-NN constraint must be satisfied — do not skip or soften any
- Do not add behavior not described in the spec
- If the spec is ambiguous on something, ask before assuming
- The tests already exist — your implementation must make them pass

[paste spec]
```

---

## 5. Clean Up a Reverse-Generated Spec

After running `specter reverse` on an existing codebase, use this to turn raw extracted drafts into meaningful specs worth approving.

```
I ran `specter reverse` on my codebase. These are draft specs extracted from
existing code. Improve them:

- Replace `gap: true` ACs with real descriptions based on the constraints
- Improve the objective summary to describe intent, not just structure
- Make constraint descriptions precise using MUST / MUST NOT language
- Suggest the correct tier (1/2/3) based on what the module does
- Remove placeholder text like "auto-generated placeholder"
- Keep status: draft — I will promote to approved after review

[paste specs]
```

---

## 6. Full Loop (new feature end-to-end)

For when you want to run the complete workflow in a single session. The pause after step 1 is non-negotiable — never let the AI write tests or code against an unreviewed spec.

```
I want to build [feature]. My intent:

[brief description]
[key constraints]
[non-obvious decisions]

Do the following in order:
1. Write a complete `.spec.yaml` (status: draft)
2. Write [language] tests for every AC with @spec/@ac annotations
3. Implement the feature so the tests pass

After step 1, pause and show me the spec. I will review it before you proceed
to step 2. Do not write tests or code until I approve the spec.
```

---

## The Order Matters

These prompts follow the SDD loop in sequence:

```
Intent → Spec → [Review] → Tests → Implementation → specter sync
```

Skipping the review step defeats the purpose. The spec is where your intent is captured — if the AI misunderstood something, that is the cheapest place to catch it.

Run `specter sync` after the implementation step. If it passes, the feature is done. If it fails, the spec tells you exactly what is missing.

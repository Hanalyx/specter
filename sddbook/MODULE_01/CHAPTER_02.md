# Chapter 2: The Single Source of Truth (SSOT)

## MODULE 01 — Foundations: The "Contract" Mindset

---

### Lecture Preamble

*Welcome back. Last time we talked about why natural language fails as a specification medium and how the industry evolved from vibe coding to spec-driven development. Today we are going to tackle what I consider the single most important architectural decision in SDD: establishing the spec as the Single Source of Truth.*

*This concept might sound abstract at first. "Source of truth" is one of those phrases that gets thrown around in engineering so often that it has lost some of its meaning. So let me be very precise about what I mean, and why it matters.*

*Grab your notebooks. This is the chapter where the philosophy becomes architecture.*

---

## 2.1 What "Source of Truth" Actually Means

In any system where the same information exists in multiple places, you have a potential for conflict. The database says the user's name is "Alice." The cache says it is "Alce" (a stale entry with a typo). The UI is displaying "Alice Smith" because it combined the cached first name with a fresh last name. Which one is correct?

The answer depends on which one is the **source of truth.** If the database is the source of truth, then the database is always right, and everything else is a derivative that should be updated to match it. If the cache disagrees with the database, the cache is wrong.

This is a foundational concept in software architecture, and it applies directly to the relationship between specs and code in SDD:

> **The `.spec` file is the source of truth. The `.code` file is a derivative.**

This means:

- When the spec and the code disagree, **the spec is right and the code is wrong.**
- When you want to understand what a feature does, **you read the spec, not the code.**
- When you want to change a feature, **you change the spec first, then regenerate or update the code.**
- When you want to review a feature, **you review the spec for correctness of intent, then review the code for correctness of implementation.**

This is a profound shift in how most developers think about their codebase. For decades, the code has been the ultimate authority. "Read the source" is one of the oldest pieces of programming advice. Documentation is perpetually out of date. Comments lie. The code is the only thing that tells you what the program *actually does.*

SDD does not dispute that the code tells you what the program does. But it introduces a distinction:

- **The code tells you WHAT the program does.** (implementation)
- **The spec tells you what the program SHOULD do.** (intent)

Both of these are necessary. Neither is sufficient alone. But when they conflict, intent governs. The spec is the authority.

> **Professor's Aside:** I know what some of you are thinking: "But the code is what actually runs. The spec is just a document. How can a document be more authoritative than running software?" Here is how: the code is a *translation* of the spec. If the translation is wrong, you fix the translation, not the original text. If a French translation of a novel changes the ending, you do not say "well, the French version is what actually got printed, so that is the real ending." You say the translator made an error. The spec is the novel. The code is the translation.

---

## 2.2 The Spec-Code Relationship

Let me formalize the relationship between specs and code. Understanding this relationship is essential to everything else in SDD.

### The Spec is the "What and Why"

The spec answers these questions:

- **What** should this component/feature/module do?
- **Why** does it exist? What problem does it solve?
- **What** are the constraints and boundaries?
- **What** are the acceptance criteria?
- **What** is the context in which it operates?

The spec deliberately does NOT answer:

- **How** should the feature be implemented?
- What specific algorithms should be used?
- What variable names should be chosen?
- How should the code be structured internally?

### The Code is the "How"

The code answers these questions:

- **How** is the specified behavior achieved?
- What data structures are used?
- What is the control flow?
- How are edge cases handled at the implementation level?

The code should NOT contain:

- Information about *why* design decisions were made
- Business context that is not evident from the code itself
- Constraints that are not enforced mechanically

### A Concrete Example

Let me show you this relationship with a real example. Here is a spec for a search feature:

```yaml
# search-bar.spec.yaml
kind: Component
metadata:
  name: SearchBar
  module: search
  version: 1.2.0

context:
  description: >
    The application has a product catalog with ~50,000 items.
    The existing search API is at GET /api/search?q={query}&limit={n}.
    The API supports debouncing on the server side but we want
    client-side debouncing to reduce unnecessary network requests.
    Current implementation has no search — users browse by category only.

objective:
  summary: >
    Add a search bar to the main navigation that provides real-time
    search suggestions as the user types.
  acceptance_criteria:
    - Search input is always visible in the top navigation bar
    - Suggestions appear after the user has typed at least 2 characters
    - Suggestions are debounced at 300ms to avoid excessive API calls
    - Maximum of 8 suggestions displayed at a time
    - Each suggestion shows product name and category
    - Clicking a suggestion navigates to the product detail page
    - Pressing Enter searches with the full query text
    - Pressing Escape closes the suggestion dropdown
    - The search bar is accessible (ARIA combobox pattern)

constraints:
  - MUST debounce on the client side at 300ms minimum
  - MUST NOT make API calls for queries shorter than 2 characters
  - MUST NOT display more than 8 suggestions
  - MUST cancel in-flight requests when a new query is typed
  - MUST handle API errors gracefully (no uncaught exceptions)
  - MUST NOT block the main thread during search
```

And here is what the generated code might look like (simplified):

```typescript
// SearchBar.tsx — Generated from search-bar.spec.yaml v1.2.0

import { useState, useCallback, useRef, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { useDebounce } from "@/hooks/useDebounce";
import { searchProducts } from "@/api/search";

interface SearchSuggestion {
  id: string;
  name: string;
  category: string;
}

export function SearchBar() {
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  // Debounce at 300ms per spec constraint
  const debouncedQuery = useDebounce(query, 300);

  // Only search when query is 2+ characters per spec constraint
  const shouldSearch = debouncedQuery.length >= 2;

  const { data: suggestions = [], isError } = useQuery({
    queryKey: ["search", debouncedQuery],
    queryFn: () => searchProducts(debouncedQuery, 8), // max 8 per spec
    enabled: shouldSearch,
    staleTime: 60_000,
  });

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      switch (e.key) {
        case "Escape":
          setIsOpen(false);
          inputRef.current?.blur();
          break;
        case "Enter":
          if (activeIndex >= 0 && suggestions[activeIndex]) {
            navigate(`/products/${suggestions[activeIndex].id}`);
          } else if (query.length >= 2) {
            navigate(`/search?q=${encodeURIComponent(query)}`);
          }
          setIsOpen(false);
          break;
        case "ArrowDown":
          e.preventDefault();
          setActiveIndex((prev) =>
            prev < suggestions.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
          e.preventDefault();
          setActiveIndex((prev) => (prev > 0 ? prev - 1 : -1));
          break;
      }
    },
    [activeIndex, suggestions, query, navigate]
  );

  return (
    <div role="combobox" aria-expanded={isOpen} aria-haspopup="listbox">
      <input
        ref={inputRef}
        type="search"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setIsOpen(true);
          setActiveIndex(-1);
        }}
        onFocus={() => shouldSearch && setIsOpen(true)}
        onKeyDown={handleKeyDown}
        aria-autocomplete="list"
        aria-controls="search-suggestions"
        aria-activedescendant={
          activeIndex >= 0 ? `suggestion-${activeIndex}` : undefined
        }
        placeholder="Search products..."
      />
      {isOpen && shouldSearch && !isError && suggestions.length > 0 && (
        <ul id="search-suggestions" role="listbox">
          {suggestions.map((item, index) => (
            <li
              key={item.id}
              id={`suggestion-${index}`}
              role="option"
              aria-selected={index === activeIndex}
              onClick={() => {
                navigate(`/products/${item.id}`);
                setIsOpen(false);
              }}
            >
              <span>{item.name}</span>
              <span>{item.category}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

Now, look at the relationship between spec and code:

| Spec Says | Code Implements |
|---|---|
| "debounced at 300ms" | `useDebounce(query, 300)` |
| "at least 2 characters" | `debouncedQuery.length >= 2` |
| "Maximum of 8 suggestions" | `searchProducts(debouncedQuery, 8)` |
| "Pressing Escape closes" | `case "Escape": setIsOpen(false)` |
| "ARIA combobox pattern" | `role="combobox"`, `aria-expanded`, etc. |
| "Cancel in-flight requests" | React Query handles this automatically |

Every line of code traces back to a spec requirement. Every spec requirement has a corresponding implementation. This traceability is the hallmark of spec-driven code.

---

## 2.3 Why Not Just Use Code as the Source of Truth?

This is the most common pushback I get from experienced developers, and it deserves a thorough answer.

The argument goes like this: "Code is already precise. Code is already structured. Code is already version-controlled. Why add another layer? Why not just use the code itself as the source of truth?"

Here are five reasons.

### Reason 1: Code Captures "How", Not "Why"

Look at this line of code:

```typescript
const debouncedQuery = useDebounce(query, 300);
```

The code tells you that the debounce interval is 300ms. But it does not tell you *why* it is 300ms. Is it 300ms because that is the UX standard? Because the backend cannot handle more than 3 requests per second? Because user testing showed that shorter intervals felt "jittery"? Because the API has a rate limit?

The spec captures this context:

```yaml
constraints:
  - MUST debounce on the client side at 300ms minimum
  # Rationale: Our API rate limit is 10 req/s per user.
  # With 300ms debounce, even fast typists generate at most
  # ~3 req/s, leaving headroom for other API calls.
```

When a future developer wonders "can I reduce this to 100ms?", the code gives them no answer. The spec tells them exactly why the value was chosen and what would break if they changed it.

### Reason 2: Code Cannot Express Intent for Things That Are NOT There

One of the most powerful features of a spec is its ability to express what should NOT happen and what is NOT in scope. Code cannot do this. Code can only express what IS.

Consider this spec section:

```yaml
constraints:
  - MUST NOT store search queries in localStorage
  - MUST NOT send search queries to analytics without user consent
  - MUST NOT cache search results across sessions

scope:
  excludes:
    - Search history / recent searches feature
    - Voice search
    - Image search
    - Search filters / faceted search
```

None of this information can exist in the code. There is no way to write code that says "I deliberately chose not to implement search history." The absence of a feature in code is indistinguishable from the feature not having been considered.

> **Professor's Aside:** This point is subtle but profound. In code, absence is ambiguous. The feature might not be there because it was deliberately excluded, because it was forgotten, because it is planned for later, or because the AI was not prompted to include it. In a spec, absence from the `includes` and presence in the `excludes` is *meaningful*. It communicates "we thought about this and decided not to do it." This is information that code literally cannot express.

### Reason 3: Code Is Too Detailed to Review for Intent

Imagine you are reviewing a pull request that adds a new feature. The PR contains 500 lines of TypeScript. To understand the developer's intent, you have to:

1. Read all 500 lines
2. Build a mental model of what the code does
3. Infer the developer's intent from the implementation
4. Determine whether the implementation matches your understanding of the requirements
5. Identify whether anything is missing (the hardest part)

Now imagine the same PR includes a 50-line spec. You read the spec first. You understand the intent in two minutes. Now when you read the code, you are *validating implementation against intent* — a much easier cognitive task than inferring intent from implementation.

Specs make code review faster and more effective by separating the "is this the right thing?" question from the "is this done right?" question.

### Reason 4: Code Is Model-Specific; Specs Are Model-Agnostic

Here is a practical consideration. If you generated your code with Claude 3.5 Sonnet and tomorrow you want to use GPT-4o, or Gemini 2 Flash, or the latest Llama model, your code is tied to the patterns and conventions that Claude chose. But your spec is not.

A spec is a model-agnostic description of what you want. You can feed the same spec to any AI model (or to a human developer) and get a conformant implementation. The spec decouples your *intent* from any specific *implementation tool.*

This is not theoretical. Teams regularly switch between AI models as new versions are released. Teams that have specs can switch easily — they just regenerate from the spec. Teams that do not have specs are stuck with whatever the previous model produced.

### Reason 5: Code Cannot Drive Validation

A spec contains testable acceptance criteria:

```yaml
acceptance_criteria:
  - Suggestions appear after the user has typed at least 2 characters
  - Suggestions are debounced at 300ms
  - Maximum of 8 suggestions displayed
```

These criteria can be mechanically transformed into tests. In fact, one of the more powerful SDD workflows is to have the AI generate tests *from the spec* before generating the implementation, ensuring that the tests are driven by intent rather than by implementation.

Code, by itself, can only be tested against itself. You can verify that functions return what they return. But you cannot verify that they return what they *should* return without an external definition of "should." The spec provides that external definition.

---

## 2.4 The SSOT Principle in Industry

The idea that structured, authoritative documents should govern code is not new. SDD is drawing on a long lineage of engineering practices. Let me trace some of the key influences.

### Google's Design Docs

Google's engineering culture has long required design documents before implementation. A Google design doc typically contains:

- **Context:** What problem are we solving? What is the current state?
- **Goals and Non-Goals:** What will this project accomplish, and what is explicitly out of scope?
- **Design:** How will the system work at a high level?
- **Alternatives Considered:** What other approaches were evaluated and why were they rejected?
- **Security/Privacy Considerations:** What are the implications?

Sound familiar? The structure of a Google design doc maps almost directly to an SDD spec:

| Google Design Doc | SDD Spec |
|---|---|
| Context | `context` section |
| Goals | `objective.acceptance_criteria` |
| Non-Goals | `scope.excludes`, `constraints` |
| Design | Deliberately omitted (left to implementation) |
| Alternatives Considered | Optional `rationale` section |

The key difference is that a Google design doc is a one-time artifact — it is written, reviewed, and then often becomes stale as implementation diverges. An SDD spec is a *living* artifact — it is maintained alongside the code and updated when requirements change.

Google's Gemini team and DeepMind have reportedly extended the design doc concept into their AI-assisted development workflows, using structured documents to guide code generation with their internal tools. The pattern is clear: when you have AI generating code, you need a formal document that governs what gets generated.

### Anthropic's Constitutional AI

Anthropic's approach to AI safety provides a fascinating parallel. In Constitutional AI, the model's behavior is governed by a set of explicit principles — a "constitution" — that specifies what the model should and should not do. The model is trained to follow these principles, and the principles serve as the source of truth for evaluating whether the model's behavior is correct.

This is structurally identical to how a spec governs code in SDD:

| Constitutional AI | SDD |
|---|---|
| Constitution | Spec |
| Model behavior | Generated code |
| Principle evaluation | Acceptance criteria validation |
| Red-teaming | Constraint testing |

The constitution is the source of truth for model behavior, just as the spec is the source of truth for code behavior. When the model violates a constitutional principle, the constitution is not wrong — the model is wrong. When generated code violates a spec constraint, the spec is not wrong — the code is wrong.

Anthropic's system prompts for Claude are another manifestation of this principle. A system prompt is a specification for how the model should behave in a given context. It is the source of truth for that interaction. The model's responses are derivatives of the system prompt, evaluated against it.

> **Professor's Aside:** I want you to appreciate the elegance of this parallel. Anthropic uses specifications (constitutions, system prompts) to control AI behavior. We use specifications (SDD specs) to control AI-generated code. The principle is the same at both levels: explicit, structured declarations of intent produce more reliable outcomes than implicit expectations.

### OpenAI's Schema-Driven APIs

OpenAI's evolution toward structured outputs is another manifestation of the SSOT principle. When you define a JSON Schema for structured outputs:

```typescript
const schema = {
  type: "object",
  properties: {
    name: { type: "string" },
    age: { type: "number", minimum: 0 },
    email: { type: "string", format: "email" }
  },
  required: ["name", "email"],
  additionalProperties: false
};
```

The schema is the source of truth. The model's output must conform to it. If the output does not match the schema, the output is wrong — not the schema.

OpenAI's function calling definitions work the same way. The function schema is the source of truth for how the model should invoke the function. The model's function call is a derivative of the schema. If the call does not match the schema, the call is wrong.

This is SSOT at the API level. SDD extends the same principle to the application level.

---

## 2.5 The Spec File and the Code File: A Practical Architecture

Let me get concrete about how specs and code coexist in a project.

### File Structure

A typical SDD project has a parallel structure where every code file has a corresponding spec file:

```
project/
  specs/
    system.spec.yaml           # Global system context
    features/
      auth/
        login.spec.yaml        # Spec for login feature
        signup.spec.yaml       # Spec for signup feature
        password-reset.spec.yaml
      search/
        search-bar.spec.yaml   # Spec for search bar
        search-results.spec.yaml
      users/
        user-list.spec.yaml
        user-profile.spec.yaml
  src/
    features/
      auth/
        components/
          LoginForm.tsx         # Code generated from login.spec.yaml
          SignupForm.tsx
          PasswordResetForm.tsx
        hooks/
          useLogin.ts
          useSignup.ts
      search/
        components/
          SearchBar.tsx         # Code generated from search-bar.spec.yaml
          SearchResults.tsx
        hooks/
          useSearch.ts
      users/
        components/
          UserList.tsx
          UserProfile.tsx
```

### The System Spec

The `system.spec.yaml` is a special spec that provides global context. It does not generate code directly. Instead, it is referenced by every feature spec. Think of it as the "constitution" for your entire application:

```yaml
# system.spec.yaml
kind: SystemContext
metadata:
  name: acme-saas
  version: 3.1.0
  updated: 2026-02-15

technology:
  language: TypeScript 5.4
  runtime: Node.js 22 LTS
  framework: React 19
  styling: Tailwind CSS 4.0
  state:
    global: Zustand 5
    server: "@tanstack/react-query v5"
    forms: "React Hook Form v7"
  routing: "React Router v7"
  testing:
    unit: Vitest 2
    component: React Testing Library
    e2e: Playwright

architecture:
  pattern: Feature-based modules
  api_style: REST with OpenAPI 3.1 schemas
  authentication: JWT with httpOnly cookies
  authorization: Role-based (RBAC)

conventions:
  naming:
    components: PascalCase
    hooks: camelCase with "use" prefix
    files: kebab-case
    types: PascalCase with no "I" prefix
  error_handling: >
    All API errors are caught by React Query.
    Components display ErrorBanner for fetch failures.
    Form validation errors are handled by React Hook Form.
    Unexpected errors are caught by ErrorBoundary at route level.
  data_fetching: >
    All server data access goes through React Query hooks.
    Custom hooks are in /features/{domain}/hooks/.
    No direct fetch() calls in components.
  accessibility: >
    All interactive components meet WCAG 2.1 AA.
    Use semantic HTML elements. No div-based buttons.
    All images have alt text. All form inputs have labels.

security:
  - All user input is sanitized before rendering (DOMPurify)
  - No dangerouslySetInnerHTML without explicit spec approval
  - All API calls include CSRF token from cookie
  - No sensitive data in localStorage (use httpOnly cookies)
  - No console.log in production builds
```

This system spec answers the questions that every feature spec would otherwise need to repeat: What is the tech stack? What are the conventions? What are the global constraints? By extracting this into a shared document, we ensure consistency across the entire application.

### The Feature Spec

A feature spec references the system spec and adds feature-specific context, objectives, and constraints:

```yaml
# login.spec.yaml
kind: Feature
metadata:
  name: Login
  module: auth
  version: 2.0.0
  system_spec: system.spec.yaml@3.1.0

context:
  description: >
    The application currently has a JWT-based authentication system.
    The login API endpoint is POST /api/auth/login, which accepts
    { email, password } and returns { accessToken, refreshToken }.
    Tokens are set as httpOnly cookies by the API (not returned in body).
    The current login page is a basic form with no OAuth support.
    This spec adds OAuth support while maintaining email/password login.
  related_specs:
    - signup.spec.yaml  # Signup flow shares the auth layout
    - password-reset.spec.yaml  # Linked from login form

objective:
  summary: >
    Enhance the login page to support OAuth (Google, GitHub) alongside
    existing email/password authentication.
  acceptance_criteria:
    - Email/password login continues to work as before
    - Google OAuth button initiates Google OAuth flow
    - GitHub OAuth button initiates GitHub OAuth flow
    - OAuth callback handles success and error states
    - Failed login shows specific error message (not generic)
    - Successful login redirects to the page the user came from
    - Login form validates email format and password presence
    - Rate limiting feedback shown after 5 failed attempts

constraints:
  - MUST NOT store OAuth tokens on the client
  - MUST NOT send credentials over non-HTTPS connections
  - MUST NOT expose error details that reveal whether email exists
  - MUST use the existing AuthLayout component for page structure
  - MUST use React Hook Form for form handling (per system spec)
  - OAuth popup MUST close automatically after callback

testing:
  unit:
    - Email/password form validates required fields
    - Form submits correctly formatted data to API
    - Error messages display for invalid credentials
    - Rate limit message appears after 5 failures
  integration:
    - Google OAuth flow completes successfully
    - GitHub OAuth flow completes successfully
    - Redirect after login goes to original destination
  accessibility:
    - Form navigable by keyboard alone
    - Error messages announced by screen readers
    - OAuth buttons have descriptive labels
```

Notice how the feature spec references the system spec (`system_spec: system.spec.yaml@3.1.0`). This creates a formal dependency chain. The AI, when generating code from this spec, should also read the system spec to understand the global conventions.

---

## 2.6 Version Control for Specs vs. Version Control for Code

One question that comes up immediately when you start treating specs as engineering artifacts is: how do you version-control them?

The answer is: the same way you version-control code. Specs live in the same repository as the code they govern. They are committed, branched, merged, and reviewed using the same Git workflow.

But there are some important nuances.

### Spec Changes Precede Code Changes

In a typical SDD workflow, a change starts with a spec modification:

```
1. Developer modifies the spec (e.g., adds a new acceptance criterion)
2. Spec change is reviewed (PR review)
3. After spec is approved, code is regenerated or updated
4. Code change is reviewed (PR review)
5. Both spec and code changes are merged together
```

This means that spec changes and code changes are typically in the same commit or PR. You can see the spec diff alongside the code diff, making it clear *why* the code changed.

### The Spec Has Its Own Version

Each spec includes a version number in its metadata:

```yaml
metadata:
  name: SearchBar
  version: 1.2.0
```

This version is independent of the Git commit hash. It represents the *semantic version* of the spec itself:

- **Major version** (1.x.x to 2.0.0): Breaking change to the feature's behavior or API
- **Minor version** (1.2.x to 1.3.0): New capability added to the feature
- **Patch version** (1.2.0 to 1.2.1): Clarification or correction that does not change behavior

The generated code should reference the spec version it was generated from:

```typescript
// SearchBar.tsx — Generated from search-bar.spec.yaml v1.2.0
```

This creates traceability. If you find a bug, you can check which version of the spec the code was generated from, and whether the spec has been updated since then.

### Diffing Specs

One of the most powerful aspects of version-controlled specs is that you can *diff* them. A spec diff tells you, in human-readable terms, exactly what changed about the feature's requirements:

```diff
  acceptance_criteria:
    - Search input is always visible in the top navigation bar
    - Suggestions appear after the user has typed at least 2 characters
-   - Suggestions are debounced at 300ms
+   - Suggestions are debounced at 200ms
    - Maximum of 8 suggestions displayed at a time
+   - Suggestions are grouped by category
+   - Each group shows a maximum of 3 suggestions
    - Each suggestion shows product name and category
```

Compare this to trying to understand a code diff that changes the debounce interval, restructures the suggestion rendering, adds category grouping logic, and modifies the API call — all in a 200-line diff of TypeScript. The spec diff communicates the *intent* of the change immediately. The code diff shows the *implementation* of the change.

> **Professor's Aside:** This is one of the things I love most about SDD in practice. Spec diffs are *readable.* They tell a story. "We reduced the debounce to 200ms and added category grouping." You can read a spec diff and understand the change in seconds. Code diffs, by contrast, often require deep reading to understand what changed and why. Spec diffs are the executive summary; code diffs are the detailed report.

---

## 2.7 What Happens When Spec and Code Diverge

Divergence is the cancer of SSOT. It happens when the code no longer matches the spec. And it will happen. Here are the common causes and how to handle them.

### Cause 1: Someone Edits the Code Without Updating the Spec

This is the most common cause of divergence, and it usually happens for understandable reasons. A developer finds a bug. The fix is simple — change one line of code. They fix the code, commit, and move on. They do not update the spec.

Now the code does something that the spec does not describe. The spec says debounce is 300ms. The code says 250ms because someone found that 300ms felt sluggish on mobile. The spec is now inaccurate.

**Prevention:**

1. **Code review discipline.** Every PR that modifies code governed by a spec must also modify the spec if behavior changes. This should be a required checklist item in your PR template.

2. **Spec-code sync checks.** Automated checks in CI that compare the spec version referenced in the code against the latest spec version. If they diverge, the check fails.

3. **Generated code comments.** Include a comment in generated code files indicating the spec version:

```typescript
/**
 * @generated from search-bar.spec.yaml v1.2.0
 * @warning Manual edits to this file will diverge from the spec.
 * Update the spec and regenerate instead.
 */
```

### Cause 2: The Spec Is Updated but the Code Is Not Regenerated

A product manager updates the spec to add a new requirement. The spec is merged. But no one regenerates the code or updates the implementation to match.

**Prevention:**

1. **Spec change triggers code task.** In your project management workflow, a spec change automatically creates a task to update the corresponding code.

2. **CI validation.** A CI step that compares the spec version with the version embedded in the generated code. If the spec is newer, the build warns (or fails).

3. **Workflow automation.** Some teams automate this: when a spec file changes in a PR, a CI job regenerates the code and adds it to the PR automatically.

### Cause 3: The AI Generates Code That Does Not Match the Spec

Sometimes the AI gets it wrong. Despite a clear spec, the generated code does not meet all the acceptance criteria. Maybe it misses an edge case. Maybe it uses the wrong library. Maybe it partially implements a constraint.

**Prevention:**

1. **Spec-derived tests.** Generate tests from the spec *before* generating the implementation. The tests codify the spec requirements as executable checks. If the generated code does not pass the tests, you know immediately.

2. **Acceptance criteria as a checklist.** After generating code, mechanically walk through each acceptance criterion and verify that the code addresses it.

3. **Constraint verification.** For each constraint in the spec, verify that the code either enforces the constraint or makes the constraint enforceable through configuration.

Here is what spec-derived testing looks like in practice:

```typescript
// SearchBar.test.tsx — Generated from search-bar.spec.yaml v1.2.0

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SearchBar } from "./SearchBar";
import { server } from "@/test/mocks/server";
import { http, HttpResponse } from "msw";

// Acceptance Criterion: "Suggestions appear after the user has
// typed at least 2 characters"
describe("minimum query length", () => {
  it("does not show suggestions for single character input", async () => {
    const user = userEvent.setup();
    render(<SearchBar />);

    await user.type(screen.getByRole("searchbox"), "a");

    await waitFor(() => {
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    });
  });

  it("shows suggestions for two character input", async () => {
    const user = userEvent.setup();
    render(<SearchBar />);

    await user.type(screen.getByRole("searchbox"), "ab");

    await waitFor(() => {
      expect(screen.getByRole("listbox")).toBeInTheDocument();
    });
  });
});

// Constraint: "MUST NOT display more than 8 suggestions"
describe("suggestion limit", () => {
  it("displays at most 8 suggestions even when API returns more", async () => {
    server.use(
      http.get("/api/search", () => {
        return HttpResponse.json({
          results: Array.from({ length: 15 }, (_, i) => ({
            id: String(i),
            name: `Product ${i}`,
            category: "Test",
          })),
        });
      })
    );

    const user = userEvent.setup();
    render(<SearchBar />);

    await user.type(screen.getByRole("searchbox"), "test");

    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options.length).toBeLessThanOrEqual(8);
    });
  });
});

// Constraint: "MUST NOT make API calls for queries shorter
// than 2 characters"
describe("API call prevention", () => {
  it("does not call search API for single character queries", async () => {
    let apiCallCount = 0;
    server.use(
      http.get("/api/search", () => {
        apiCallCount++;
        return HttpResponse.json({ results: [] });
      })
    );

    const user = userEvent.setup();
    render(<SearchBar />);

    await user.type(screen.getByRole("searchbox"), "a");

    // Wait enough time for a debounced request to fire
    await new Promise((r) => setTimeout(r, 500));

    expect(apiCallCount).toBe(0);
  });
});
```

Notice how every test is annotated with the spec requirement it validates. This creates a direct, traceable link from spec to test to code. If a test fails, you know exactly which spec requirement is not met. If a spec requirement changes, you know exactly which tests need to be updated.

---

## 2.8 The Spec as a Communication Layer

I want to zoom out and talk about the spec's role in team communication.

In traditional development, the communication chain looks like this:

```
Product Manager → (conversation/ticket) → Developer → (code) → Reviewer
```

The "conversation/ticket" step is where intent gets lost. Jira tickets are written in prose. Conversations are ephemeral. By the time the reviewer sees the code, the original intent has been filtered through two translations (PM to developer, developer to code).

In SDD, the communication chain looks like this:

```
Product Manager → Spec ← Developer → Code ← Reviewer
```

The spec is the *shared artifact* that both the PM and the developer contribute to. The PM defines the objectives and constraints. The developer adds technical context and validation criteria. The reviewer reads the spec to understand intent and the code to validate implementation.

This has several benefits:

### Benefit 1: The PM Can Review Specs

Product managers are typically not qualified to review TypeScript code. But they are perfectly qualified to review a spec. They can verify that the acceptance criteria match their intent. They can check that the constraints are correct. They can validate the scope.

```yaml
# A PM can read this and verify it matches their intent:
acceptance_criteria:
  - Search input is always visible in the top navigation bar
  - Suggestions appear after the user has typed at least 2 characters
  - Maximum of 8 suggestions displayed at a time
```

This is a massive improvement over the traditional workflow, where the PM writes a ticket, the developer interprets it, and the PM only sees the result in a staging environment days or weeks later.

### Benefit 2: New Team Members Onboard Faster

When a new developer joins the team, they can read the specs to understand what each feature does and why. Specs are more readable than code, more current than documentation (because they are maintained as part of the development workflow), and more structured than chat histories or meeting notes.

### Benefit 3: The AI Becomes a Team Member

Here is a subtle but important point: the spec makes the AI a predictable team member.

Without specs, the AI is like a brilliant but erratic contractor who might build exactly what you want or might go off on a tangent. You never know what you are going to get. Each interaction is a negotiation.

With specs, the AI is like a reliable contractor who works from blueprints. The blueprints (specs) define what gets built. The contractor (AI) implements the blueprints. If the result does not match the blueprints, you refine the blueprints and try again. The process is predictable, repeatable, and debuggable.

### Benefit 4: Cross-Team Coordination

When Feature A needs to integrate with Feature B, the teams can share specs rather than code. Team A reads Team B's spec to understand the interface. They do not need to understand Team B's implementation.

This is the same principle as API documentation, but applied to the entire feature surface. The spec is the *contract* between teams, just as an API schema is the contract between services.

---

## 2.9 The Living Spec vs. The Dead Document

I want to address an objection that experienced developers will be screaming internally right now:

*"We have tried this before. It was called 'requirements documents.' They were always out of date. Nobody maintained them. This will be the same."*

This is a legitimate concern, and it is worth addressing directly.

Traditional requirements documents died because they existed *outside* the development workflow. They were written in Word or Confluence, separate from the code. Updating them required switching tools, navigating to a different location, editing a different artifact. It was friction, and developers rationally avoided it.

SDD specs are different for several reasons:

### Reason 1: Specs Live in the Repository

Specs are in the same Git repository as the code. They are in the same PR. They are diffed alongside the code. Updating a spec is the same workflow as updating code: edit a file, commit, push.

### Reason 2: Specs Are Functional, Not Documentary

A requirements document is descriptive — it describes what should be built. An SDD spec is *functional* — it is the input to the AI code generator. If you do not maintain it, you cannot generate code from it. This creates a natural incentive to keep it current.

Think of it like a Dockerfile. Nobody writes a Dockerfile and then ignores it, because the Dockerfile is how you build the container. If the Dockerfile is wrong, the build fails. Similarly, if the spec is wrong, the code generation is wrong. The spec has functional consequences.

### Reason 3: Specs Are Validated

Traditional requirements documents had no validation mechanism. Nobody could tell if the document accurately described the system. Specs, by contrast, have acceptance criteria that can be tested. You can mechanically verify whether the code matches the spec.

### Reason 4: Specs Are Minimal

A requirements document tries to capture everything about a feature in prose. An SDD spec is structured and focused: context, objective, constraints, tests. It does not include narrative, justification, meeting notes, or historical discussion. It is lean. It takes minutes to write, not hours.

### Reason 5: The Spec-Code Loop Is Tight

In traditional development, the feedback loop from "requirements change" to "code changes" could be weeks. In SDD, the loop is minutes. You update the spec, regenerate the code, validate. The tight loop makes it practical to keep spec and code in sync.

> **Professor's Aside:** I have seen teams adopt SDD and initially treat specs as "extra work." Within two weeks, they universally report that specs *save* time. The upfront investment in writing the spec is repaid several times over in reduced re-prompting, reduced code review time, reduced onboarding time, and reduced debugging time. The spec is not overhead. It is the highest-leverage artifact in your development workflow.

---

## 2.10 The SSOT Contract: Rules of Engagement

Let me formalize the rules that govern the SSOT relationship between spec and code. These are the "rules of engagement" for any SDD team.

### Rule 1: Spec Writes First

No code is written (or generated) without a spec. The spec does not have to be perfect — it can start as a draft and be refined. But it must exist before code generation begins.

### Rule 2: Code References Spec

Every generated code file includes a reference to the spec it was generated from, including the spec version:

```typescript
/**
 * @spec search-bar.spec.yaml
 * @spec-version 1.2.0
 * @generated 2026-02-24
 */
```

### Rule 3: Spec Changes Trigger Code Updates

When a spec is modified, the corresponding code must be updated to match. This can be automated (regeneration) or manual (targeted edits), but the code must reflect the current spec.

### Rule 4: Code Changes May Trigger Spec Updates

If a developer discovers during implementation that the spec is incomplete or incorrect, they update the spec *first*, then update the code. Code does not drive spec; but code can *inform* spec.

### Rule 5: Divergence Is a Bug

If the code does not match the spec, that is a bug — regardless of whether the code "works." A feature that works but does not match its spec is not correct. Correctness is defined by the spec, not by observed behavior.

### Rule 6: Tests Validate Spec Compliance

The test suite validates that the code conforms to the spec. Tests are derived from the spec's acceptance criteria and constraints. Passing tests mean the code matches the spec. Failing tests mean divergence.

### Rule 7: Specs Are Reviewed

Specs undergo review just like code. A spec PR is reviewed for clarity, completeness, correctness, and consistency with the system spec. Spec reviews should involve both technical and product stakeholders.

---

## 2.11 Practical Workflow: A Day in the Life

Let me walk you through a realistic day of SDD development to make this concrete.

### Morning: New Feature Request

The PM creates a draft spec for a new "favorites" feature. The spec includes business context, acceptance criteria, and scope:

```yaml
# favorites.spec.yaml (draft)
kind: Feature
metadata:
  name: Favorites
  module: catalog
  version: 0.1.0
  status: draft
  system_spec: system.spec.yaml@3.1.0

context:
  description: >
    Users have been requesting the ability to save products
    for later. Analytics show 60% of users visit the same
    product pages multiple times before purchasing. A favorites
    feature would reduce friction in the purchase funnel.

objective:
  summary: >
    Allow users to mark products as favorites and view a
    list of their favorite products.
  acceptance_criteria:
    - Heart icon on each product card toggles favorite status
    - Favorites are persisted to the user's account
    - A "My Favorites" page shows all favorited products
    - Favorites count shown in the navigation bar
    - Removing a favorite from the list updates immediately (optimistic)
```

### Mid-Morning: Spec Review

The developer reviews the draft spec and adds technical context and constraints:

```yaml
# favorites.spec.yaml (reviewed)
# ... (previous sections unchanged)

context:
  description: >
    Users have been requesting the ability to save products
    for later. Analytics show 60% of users visit the same
    product pages multiple times before purchasing. A favorites
    feature would reduce friction in the purchase funnel.
  technical_context: >
    The favorites API is being built by the backend team and will
    be available at:
      POST /api/favorites/:productId (add favorite)
      DELETE /api/favorites/:productId (remove favorite)
      GET /api/favorites (list all favorites)
    The API uses the standard auth middleware (JWT from cookie).
    Product cards are rendered by the existing ProductCard component.

constraints:
  - MUST use optimistic updates for toggle (immediate UI feedback)
  - MUST invalidate product list queries when favorites change
  - MUST handle offline/network error gracefully (revert optimistic update)
  - MUST NOT allow unauthenticated users to favorite (show login prompt)
  - MUST limit favorites display to 50 per page with pagination
  - MUST NOT refetch all favorites on every toggle (surgical cache update)

scope:
  excludes:
    - Sharing favorites with other users
    - Organizing favorites into collections/folders
    - Favorites import/export
    - Favorite notifications (e.g., price drop alerts)

testing:
  unit:
    - Heart icon toggles visual state immediately
    - Optimistic update reverts on API error
    - Unauthenticated users see login prompt on toggle
    - Favorites count in nav updates on toggle
  integration:
    - Full add/remove/list cycle works end-to-end
    - Pagination works for users with many favorites
```

### Afternoon: Code Generation

The developer feeds the reviewed spec to the AI. The AI generates:

- `FavoriteButton.tsx` — The heart icon toggle component
- `FavoritesList.tsx` — The favorites page component
- `useFavorites.ts` — React Query hooks for the favorites API
- `FavoriteButton.test.tsx` — Tests derived from spec
- `FavoritesList.test.tsx` — Tests derived from spec

The developer validates the generated code against each acceptance criterion and constraint. They find that the AI did not handle the "unauthenticated user" case. They add a clarifying note to the spec and regenerate the relevant component.

### Late Afternoon: PR and Review

The developer creates a PR that includes both the spec and the generated code. The reviewer reads the spec first, then reviews the code. The review is focused on:

1. Does the spec accurately capture the requirements?
2. Does the code implement the spec correctly?
3. Are the tests comprehensive enough to validate spec compliance?

The reviewer suggests an additional constraint: "MUST animate the heart icon on toggle." The developer adds this to the spec, regenerates the component, and updates the PR.

### End of Day: Merge

The spec and code are merged together. The spec version is updated from 0.1.0 to 1.0.0. The codebase now has a clear, versioned, reviewable record of what the favorites feature does and why.

---

## 2.12 Common Anti-Patterns

Let me close this chapter with some anti-patterns — things I see teams do that undermine the SSOT principle.

### Anti-Pattern 1: The Retroactive Spec

Writing the spec *after* the code is written. This is like writing a blueprint after the building is constructed. The spec becomes documentation, not a source of truth. It describes what was built rather than governing what should be built.

**Fix:** Spec first, always. Even if the spec is rough. Even if it changes during implementation. The act of thinking through the spec before writing code is where most of the value comes from.

### Anti-Pattern 2: The Orphan Spec

A spec that exists but is never referenced. No code references it. No tests derive from it. It sits in a directory, slowly becoming stale.

**Fix:** Generated code must reference its spec. CI checks should verify that every spec has corresponding code and that the versions match.

### Anti-Pattern 3: The Kitchen Sink Spec

A spec that tries to capture everything — implementation details, UI pixel specifications, database schemas, API response formats, deployment configuration. An overly detailed spec is as bad as no spec, because it takes so long to write and maintain that the team abandons it.

**Fix:** Specs capture *what* and *why*, not *how*. If you find yourself specifying variable names or CSS pixel values, you have gone too far. The spec should be at the level of abstraction where a competent developer (human or AI) could implement it without further guidance.

### Anti-Pattern 4: The Immutable Spec

Treating the spec as a sacred document that cannot be changed once written. Requirements change. Understanding deepens. The spec must evolve.

**Fix:** Specs are living documents. They have version numbers precisely because they change. The goal is not perfection on the first draft; the goal is a document that always reflects the current intent.

### Anti-Pattern 5: The Ignored Spec

Everyone writes specs, but nobody reads them. Code is generated without consulting the spec. Reviews are done without referencing the spec. The spec exists, but it has no authority.

**Fix:** This is a cultural problem, not a technical one. The team must commit to the principle that the spec is the source of truth. This means spec review is mandatory, spec-code divergence is treated as a bug, and spec references are required in code.

---

## Chapter Summary

| Concept | Key Takeaway |
|---|---|
| Source of Truth | The spec file is the ultimate authority. When spec and code disagree, the spec is right. |
| Spec = What + Why | The spec captures intent and constraints. The code captures implementation. |
| Industry Parallels | Google's design docs, Anthropic's constitutional AI, and OpenAI's structured outputs all embody the SSOT principle. |
| Version Control | Specs are version-controlled alongside code. Spec diffs communicate intent changes; code diffs communicate implementation changes. |
| Divergence Prevention | Code reviews, CI checks, spec-derived tests, and cultural discipline prevent spec-code divergence. |
| Communication Layer | Specs serve as the shared artifact between PMs, developers, reviewers, and AI. |
| Living Document | Unlike traditional requirements docs, specs are functional, minimal, validated, and tightly coupled to the code they govern. |

---

## Discussion Questions

1. Your team currently uses Jira tickets for requirements. How would you introduce specs alongside (or instead of) tickets? What resistance would you expect?

2. Consider the "code drives spec" vs. "spec drives code" debate. Are there situations where it makes sense for the code to be the source of truth? When?

3. How would you handle a situation where a senior developer disagrees with a spec? Who has authority — the spec author, the code author, or the team?

4. Think about the system spec concept. What would your current project's system spec look like? What patterns and conventions would it capture?

5. The chapter argues that spec-code divergence is a "bug." Do you agree? How strict should this be in practice?

---

*Next: Chapter 3 — "The Anatomy of a Micro-Spec"*

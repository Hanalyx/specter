# Chapter 3: The Anatomy of a Micro-Spec

## MODULE 01 — Foundations: The "Contract" Mindset

---

### Lecture Preamble

*Today we dissect. In Chapter 1 we learned why specs matter. In Chapter 2 we learned that the spec is the source of truth. Now we learn what a spec actually looks like, piece by piece, section by section.*

*I call the fundamental unit of SDD a "micro-spec." Not because it is small — some micro-specs are quite detailed — but because it is focused. A micro-spec describes one component, one feature, one endpoint, one bounded unit of work. It is the atom of spec-driven development. Everything larger is composed of micro-specs.*

*By the end of this chapter, you will be able to look at any micro-spec and immediately identify its parts, evaluate its quality, and understand how it will guide the AI. More importantly, you will be able to write one from scratch.*

*Open your editors. We are going to build specs today.*

---

## 3.1 The Three Pillars

Every effective micro-spec is built on three pillars:

1. **Context** — What exists now?
2. **Objective** — What specific change should be made?
3. **Constraints** — What must NOT happen?

These three pillars correspond to the three fundamental questions that any intelligent agent — human or AI — needs answered before it can do useful work:

- **Where am I?** (Context)
- **Where am I going?** (Objective)
- **What landmines should I avoid?** (Constraints)

Miss any one of these, and the output degrades predictably:

| Missing Pillar | Result |
|---|---|
| No Context | The AI makes assumptions about the current system state, technology stack, and existing patterns. These assumptions are usually wrong. |
| No Objective | The AI does not know when it is done. It either under-delivers (misses requirements) or over-delivers (adds features you did not ask for). |
| No Constraints | The AI takes the path of least resistance, which often involves insecure defaults, inappropriate dependencies, or patterns that conflict with the existing codebase. |

Let me go through each pillar in detail.

---

## 3.2 Pillar 1: Context

Context answers the question: **What is the current state of the world?**

An AI model, no matter how capable, starts every interaction with zero knowledge of your specific project. It does not know what framework you use. It does not know what database you have. It does not know what APIs exist. It does not know what patterns your team follows. It does not know what version of React you are on.

Without context, the AI falls back on its training data — which is a statistical average of all the code it has ever seen. This means it will generate code that is generically reasonable but specifically wrong for your project.

Context has several sub-components. Let me walk through each one.

### 3.2.1 System Context

System context is the global information that applies to your entire project. This typically lives in the system spec (which we discussed in Chapter 2) and is referenced by every micro-spec. It includes:

**Technology Stack:**

```yaml
context:
  system:
    language: TypeScript 5.4
    runtime: Node.js 22
    framework: React 19
    styling: Tailwind CSS 4.0
    state_management: Zustand 5
    data_fetching: "@tanstack/react-query v5"
    testing: Vitest + React Testing Library
    package_manager: pnpm
```

Why does this matter? Consider what happens without explicit stack context. You ask the AI to "create a component that fetches user data." The AI might:

- Use JavaScript instead of TypeScript
- Use class components instead of functional components
- Use `axios` instead of your preferred HTTP client
- Use Redux instead of Zustand
- Use Jest instead of Vitest
- Use `npm` syntax in its instructions instead of `pnpm`

Every one of these is a reasonable default. Every one of them is wrong for your project. Explicit stack context eliminates these mismatches.

**Existing Patterns:**

```yaml
context:
  patterns:
    data_fetching: >
      All data fetching is done through custom React Query hooks
      located in /features/{domain}/hooks/. Hooks follow the
      naming convention use{Domain}{Action}, e.g., useUsersFetch,
      useUsersCreate. Each hook returns the React Query result
      object directly.
    error_handling: >
      API errors are handled by React Query's error state.
      Components check query.isError and display the shared
      ErrorBanner component. No try/catch blocks in components.
      Unexpected errors are caught by the ErrorBoundary at the
      route level.
    component_structure: >
      Components are functional, using TypeScript interfaces for
      props. Props interfaces are defined in the same file as the
      component and exported. Components are in /features/{domain}/
      components/. One component per file.
```

Patterns are critical. They are the *institutional knowledge* of your codebase — the conventions that make the code consistent and maintainable. Without explicit patterns, the AI will generate code that *works* but does not *fit.* It will be the odd one out, the component that handles errors differently, the hook that is structured differently, the file that is in the wrong place.

> **Professor's Aside:** I have a rule of thumb: if a new team member would need to be *told* about a convention (because they could not infer it from the code alone), that convention belongs in the context section. Examples: "We use barrel exports from each feature directory." "We prefix all test IDs with `data-testid`." "We do not use default exports." These are the kinds of conventions that AI models will not discover on their own.

**Existing Infrastructure:**

```yaml
context:
  infrastructure:
    api_base: "https://api.acme.com/v2"
    auth_mechanism: "JWT in httpOnly cookies, auto-refreshed by interceptor"
    api_documentation: "OpenAPI spec at /docs/api/openapi.yaml"
    existing_endpoints:
      - "GET /api/users — List all users (paginated)"
      - "GET /api/users/:id — Get single user"
      - "POST /api/users — Create user"
      - "PUT /api/users/:id — Update user"
      - "DELETE /api/users/:id — Soft-delete user"
```

This tells the AI what already exists in the system. Without this, the AI might create new API endpoints that duplicate existing ones, or it might generate mock APIs that conflict with real ones.

### 3.2.2 Feature Context

Feature context is specific to the particular micro-spec. It describes what exists in the immediate area of the feature being built:

```yaml
context:
  feature:
    description: >
      The product catalog currently displays products in a grid view
      using the ProductCard component. Each ProductCard shows the
      product image, name, price, and an "Add to Cart" button.
      There is no search functionality — users browse by category
      using the sidebar navigation.
    current_state:
      - ProductCard component exists at /features/catalog/components/ProductCard.tsx
      - Category sidebar exists at /features/catalog/components/CategorySidebar.tsx
      - Product data is fetched by useProductsFetch hook
      - Product type is defined in /features/catalog/types/Product.ts
    related_features:
      - "Cart feature (cart.spec.yaml) — ProductCard's 'Add to Cart' button
         dispatches addToCart action"
      - "Category feature (categories.spec.yaml) — Sidebar navigation
         filters products by category"
```

Feature context tells the AI what it is working *next to.* This is how you prevent the AI from duplicating existing work, conflicting with adjacent features, or making assumptions about what is and is not available.

### 3.2.3 The Context Completeness Test

How do you know if your context is sufficient? Apply this test:

**Could a competent developer who has never seen your codebase implement this feature correctly using only the information in the spec?**

If the answer is no — if they would need to explore the codebase, ask a teammate, or make assumptions — then your context is incomplete.

This does not mean the context needs to be exhaustive. You do not need to describe every file in the repository. You need to describe:

1. The technology stack (so they know what tools to use)
2. The existing patterns (so they know how to structure the code)
3. The immediate neighborhood (so they know what already exists)
4. The integration points (so they know what their code needs to connect to)

Think of it like giving directions. You do not need to describe every building in the city. You need to describe the starting location, the destination, the major landmarks along the way, and the roads to avoid.

---

## 3.3 Pillar 2: Objective

The objective answers the question: **What specific change should be made?**

Notice the word "specific." The objective is not a mission statement. It is not a user story. It is not a vague description of desired functionality. It is a precise, testable description of the *delta* — the difference between the current state (described in the context) and the desired future state.

### 3.3.1 The Summary

Every objective starts with a summary — a one-to-three-sentence description of the change:

```yaml
objective:
  summary: >
    Add a search bar to the main navigation that provides real-time
    search suggestions as the user types. When a suggestion is selected,
    navigate to the product detail page.
```

The summary should be understandable by anyone — PM, developer, designer, QA. It is the "elevator pitch" for the feature.

### 3.3.2 Acceptance Criteria

The acceptance criteria are the heart of the objective. They are the specific, testable conditions that must be true when the feature is complete:

```yaml
objective:
  acceptance_criteria:
    - Search input is visible in the top navigation bar on all pages
    - Typing in the search input triggers a search after 2+ characters
    - Search results appear in a dropdown below the input
    - Each result shows the product name and category
    - Results are limited to 8 items maximum
    - Clicking a result navigates to /products/:id
    - Pressing Enter with text submits a full search to /search?q=...
    - Pressing Escape closes the dropdown
    - The dropdown closes when clicking outside it
    - The search input is cleared after navigating to a result
```

Good acceptance criteria have the following properties:

**Testable:** Each criterion can be verified mechanically. "The search bar looks good" is not testable. "The search bar is visible in the top navigation bar on all pages" is testable.

**Independent:** Each criterion stands alone. You should be able to test any criterion without relying on the order in which they are listed.

**Complete:** Together, the criteria fully describe the feature. If all criteria pass, the feature is done. There should be no "implied" requirements that are not listed.

**Unambiguous:** Each criterion has only one possible interpretation. "Results are limited" is ambiguous (limited how? in time? in number? in size?). "Results are limited to 8 items maximum" is unambiguous.

> **Professor's Aside:** Writing good acceptance criteria is a skill, and like all skills, it improves with practice. A useful exercise is to write your acceptance criteria and then ask a colleague to read them without any other context. If they can tell you exactly what the feature does from the criteria alone, your criteria are good. If they have questions, your criteria are incomplete or ambiguous.

### 3.3.3 Scope Definition

The scope section explicitly defines what is and is not included in the feature:

```yaml
objective:
  scope:
    includes:
      - Search bar component in the navigation
      - Search suggestions dropdown
      - Navigation to product detail on selection
      - Navigation to search results page on Enter
      - Keyboard navigation within suggestions
    excludes:
      - Search results page (separate spec)
      - Advanced search / filters
      - Search history / recent searches
      - Voice search
      - Search analytics tracking
```

The `excludes` section is not busywork. It is one of the most important parts of the spec. Remember Failure 3 from Chapter 1 — the notification system scope creep? The developer wanted a toast message and got a 15,000-line notification platform. An explicit `excludes` section prevents this.

When the AI sees `excludes: Search history / recent searches`, it knows not to add a "recent searches" feature, even though its training data includes thousands of search bar implementations that have one. The excludes section is a *boundary*, and boundaries are essential for controlled code generation.

### 3.3.4 The Delta Principle

I want to emphasize a concept I call the **Delta Principle**: a micro-spec describes a *change*, not a *state.*

Wrong (describes a state):

```yaml
objective:
  summary: The application has a search feature that...
```

Right (describes a change):

```yaml
objective:
  summary: Add a search bar to the existing navigation bar that...
```

The delta framing matters because it tells the AI what to *create* versus what already *exists.* If you describe a state, the AI might try to regenerate things that already exist. If you describe a delta, the AI knows to create only the new parts and integrate with the existing parts.

This is particularly important in large codebases. You do not want the AI to regenerate your entire navigation bar when you just want to add a search input to it. The delta framing, combined with context about what already exists, keeps the scope tight.

---

## 3.4 Pillar 3: Constraints

Constraints answer the question: **What must NOT happen?**

If the objective is the accelerator, constraints are the brakes. They bound the solution space. They prevent the AI from taking shortcuts, making insecure choices, or deviating from project standards.

I call constraints the "never-evers" because they express inviolable rules. They are not preferences or suggestions. They are hard boundaries.

### 3.4.1 Types of Constraints

Constraints fall into several categories:

**Technical Constraints:**

```yaml
constraints:
  technical:
    - MUST use React Query for data fetching (no raw fetch in components)
    - MUST NOT introduce new npm dependencies
    - MUST NOT use inline styles (Tailwind only)
    - MUST be compatible with React 19 strict mode
    - MUST NOT use deprecated React APIs (no componentWillMount, etc.)
```

Technical constraints ensure the generated code fits the existing codebase. Without them, the AI will cheerfully import `lodash` to use `_.debounce` when your project has a custom `useDebounce` hook, or it will use `styled-components` when your project uses Tailwind.

**Security Constraints:**

```yaml
constraints:
  security:
    - MUST NOT store tokens in localStorage or sessionStorage
    - MUST NOT use dangerouslySetInnerHTML
    - MUST sanitize all user input before rendering
    - MUST NOT log sensitive data (tokens, passwords, PII)
    - MUST NOT expose internal API paths in client-side code
    - MUST include CSRF token in all mutating API calls
```

Security constraints are arguably the most important constraints in any spec. AI models have a tendency to generate *functional* code that is *insecure,* because their training data contains a mix of secure and insecure patterns, and they optimize for "works" not "safe."

The authentication hallucination from Chapter 1 — where the AI hardcoded `"secret"` as the JWT key — would have been prevented by a single constraint: "MUST load JWT secret from environment variable."

**Performance Constraints:**

```yaml
constraints:
  performance:
    - MUST debounce search input at 300ms minimum
    - MUST NOT re-render the entire product list on search input change
    - MUST cancel in-flight requests when new search query is entered
    - MUST lazy-load the search suggestions component
    - MUST NOT block the main thread during search processing
```

Performance constraints prevent the AI from generating code that works but performs poorly. Without them, the AI might generate a search feature that fires an API request on every keystroke, or a list component that re-renders all 1,000 items when one item changes.

**Accessibility Constraints:**

```yaml
constraints:
  accessibility:
    - MUST implement ARIA combobox pattern for search
    - MUST support keyboard navigation (arrow keys, Enter, Escape)
    - MUST announce search results to screen readers via aria-live
    - MUST NOT rely solely on color to communicate state
    - MUST have minimum 4.5:1 color contrast ratio
    - MUST have visible focus indicators on all interactive elements
```

Accessibility is an area where AI models are particularly inconsistent. Some models generate reasonably accessible code by default; others generate code that is completely inaccessible. Explicit accessibility constraints ensure consistent, accessible output regardless of which model you use.

**Business Constraints:**

```yaml
constraints:
  business:
    - MUST NOT show product prices to unauthenticated users
    - MUST NOT display out-of-stock products in search results
    - MUST respect user's region for currency formatting
    - MUST comply with GDPR (no tracking without consent)
```

Business constraints encode rules that come from product, legal, or compliance requirements. These are rules the AI could never infer from technical context alone.

### 3.4.2 The MUST / MUST NOT Convention

You will notice that every constraint starts with either `MUST` or `MUST NOT`. This is deliberate. It comes from RFC 2119, which defines key words for use in specifications:

- **MUST:** This is an absolute requirement. Violation is a defect.
- **MUST NOT:** This is an absolute prohibition. Violation is a defect.
- **SHOULD:** This is a strong recommendation. Violation requires justification.
- **SHOULD NOT:** This is a strong discouragement. Violation requires justification.
- **MAY:** This is truly optional.

In SDD, we primarily use MUST and MUST NOT for constraints. SHOULD and MAY are used sparingly, typically in the objective section for nice-to-have features.

This vocabulary has two benefits:

1. **Clarity for the AI.** Models are trained on RFCs and technical specifications. They understand the weight of MUST vs. SHOULD. Using this vocabulary increases the likelihood that the AI will treat constraints as inviolable.

2. **Clarity for humans.** During spec review, MUST and MUST NOT stand out visually and semantically. A reviewer can quickly scan the constraints section and understand the hard boundaries.

### 3.4.3 The Constraint Completeness Test

How do you know if your constraints are sufficient? Ask yourself:

**If the AI produced code that technically meets all acceptance criteria but does so in the worst possible way, what would go wrong?**

For example, acceptance criteria say "fetches user data." Without constraints, the AI might:

- Fetch all 50,000 users at once (no pagination)
- Store users in a global variable (no state management)
- Cache users indefinitely (stale data)
- Fetch users on every render (performance disaster)
- Log the API response to console (data exposure)

Each of these "worst case" scenarios suggests a constraint:

```yaml
constraints:
  - MUST paginate results (maximum 50 per request)
  - MUST use React Query for state management
  - MUST set staleTime to 5 minutes maximum
  - MUST deduplicate requests (React Query handles this)
  - MUST NOT log API responses containing user data
```

> **Professor's Aside:** I sometimes call this the "malicious compliance" test. Imagine the AI is a malicious genie that will grant your wish in the worst possible way unless you constrain it precisely. What guardrails do you need? This mental model is useful because it forces you to think about edge cases and failure modes *before* code is generated, not after.

---

## 3.5 The Complete Micro-Spec: An Annotated Walkthrough

Let me now show you a complete micro-spec with detailed annotations explaining why each section exists and what it communicates to the AI.

```yaml
# ============================================================
# METADATA
# ============================================================
# Metadata identifies the spec. It answers: What is this spec
# for, and where does it fit in the system?
# ============================================================
kind: Component                          # [1]
metadata:
  name: NotificationBell                 # [2]
  module: notifications                  # [3]
  version: 1.0.0                         # [4]
  status: approved                       # [5]
  owner: team-engagement                 # [6]
  system_spec: system.spec.yaml@3.1.0    # [7]
  created: 2026-02-20                    # [8]
  updated: 2026-02-24                    # [8]

# [1] kind: What type of thing is this? Component, Feature,
#     Endpoint, Migration, etc. Tells the AI what kind of
#     code to generate.
# [2] name: The specific name. Will be used for the component/
#     function/class name in generated code.
# [3] module: Which feature domain this belongs to. Determines
#     file placement.
# [4] version: Semantic version of this spec. Incremented
#     when the spec changes.
# [5] status: draft | review | approved | deprecated.
#     Only approved specs should drive code generation.
# [6] owner: Which team is responsible. Useful for review
#     routing and ownership.
# [7] system_spec: Reference to the global system spec,
#     pinned to a specific version.
# [8] Timestamps for auditability.


# ============================================================
# CONTEXT
# ============================================================
# Context answers: What is the current state of the world?
# This is the AI's "briefing" before it starts work.
# ============================================================
context:
  description: >                         # [9]
    The application currently has no notification system.
    The backend team has built a notifications API that
    provides unread notification count and notification
    list. We need a UI component that shows the count
    and allows the user to view their notifications.

  technical_context: >                   # [10]
    Notifications API:
      GET /api/notifications/count → { count: number }
      GET /api/notifications → { notifications: Notification[], total: number }
      PUT /api/notifications/:id/read → { success: boolean }

    Notification type (from backend):
      { id: string, title: string, body: string,
        type: "info" | "warning" | "error",
        read: boolean, createdAt: string }

    The app header is rendered by the HeaderBar component at
    /features/layout/components/HeaderBar.tsx. It has a
    designated slot for notification UI (a div with
    id="notification-slot").

  related_specs:                         # [11]
    - notification-panel.spec.yaml  # Full notification list panel
    - notification-settings.spec.yaml  # User notification preferences

  assumptions:                           # [12]
    - Backend API is stable and deployed
    - Notification count endpoint is lightweight (< 50ms response)
    - Maximum 99+ notifications displayed in badge


# [9]  description: Plain-language overview of why this spec
#      exists. Provides business context.
# [10] technical_context: Detailed technical information the AI
#      needs — API endpoints, data types, existing components.
# [11] related_specs: Other specs that interact with this one.
#      Helps the AI understand the boundaries.
# [12] assumptions: Things we are taking as given. If an
#      assumption is wrong, the spec may need revision.


# ============================================================
# OBJECTIVE
# ============================================================
# Objective answers: What specific change should be made?
# This is the AI's "mission brief."
# ============================================================
objective:
  summary: >                             # [13]
    Create a notification bell icon component for the app
    header that shows the unread notification count and
    opens a notification dropdown when clicked.

  acceptance_criteria:                   # [14]
    - Bell icon is rendered inside the #notification-slot in HeaderBar
    - Badge shows unread count (number) when count > 0
    - Badge shows "99+" when count exceeds 99
    - Badge is hidden when count is 0
    - Clicking the bell toggles a dropdown panel
    - Dropdown shows the 5 most recent notifications
    - Each notification shows title, body preview (50 chars), and time ago
    - Clicking a notification marks it as read and navigates to its target
    - "Mark all as read" button at the bottom of the dropdown
    - Unread count refreshes every 30 seconds via polling
    - Dropdown closes when clicking outside it

  scope:                                 # [15]
    includes:
      - Bell icon with badge
      - Notification dropdown (compact, 5 items)
      - Mark-as-read functionality
      - Polling for count updates
    excludes:
      - Full notification list/page (see notification-panel.spec)
      - Notification settings (see notification-settings.spec)
      - Push notifications / WebSocket real-time updates
      - Notification sounds or browser notifications
      - Notification grouping or categorization


# [13] summary: One clear sentence describing the delta.
#      "Create a..." tells the AI this is new, not a modification.
# [14] acceptance_criteria: Testable conditions. Each one maps
#      to at least one test case.
# [15] scope: Explicit boundaries. The excludes section prevents
#      the AI from building adjacent features.


# ============================================================
# CONSTRAINTS
# ============================================================
# Constraints answer: What must NOT happen?
# These are the guardrails that prevent bad AI decisions.
# ============================================================
constraints:                             # [16]
  # Technical
  - MUST use React Query for fetching notification count and list
  - MUST use the existing usePolling hook for count refresh
  - MUST NOT introduce new npm dependencies
  - MUST NOT use CSS animations (use Tailwind transition utilities)
  - MUST use React Portal for the dropdown (to escape header overflow)

  # Performance
  - MUST NOT fetch full notification list until dropdown is opened
  - MUST NOT re-render HeaderBar when notification count changes
  - Polling interval MUST be 30 seconds (not shorter)
  - MUST abort in-flight requests when dropdown closes

  # Security
  - MUST NOT include notification content in document.title
  - MUST NOT store notification data in localStorage
  - MUST sanitize notification body before rendering (DOMPurify)

  # Accessibility
  - Bell button MUST have aria-label "Notifications"
  - Badge count MUST be announced to screen readers via aria-live="polite"
  - Dropdown MUST trap focus when open
  - MUST support Escape key to close dropdown
  - Notifications MUST be navigable by keyboard (arrow keys)

  # UX
  - Dropdown MUST NOT cover more than 50% of viewport on mobile
  - MUST show loading skeleton in dropdown while fetching
  - MUST show "No notifications" message when list is empty


# [16] Constraints are organized by category for readability.
#      Each constraint uses MUST or MUST NOT for unambiguous
#      requirement level.


# ============================================================
# TESTING
# ============================================================
# Testing defines how we validate that code matches the spec.
# These are the spec's "teeth" — without tests, constraints
# are just suggestions.
# ============================================================
testing:                                 # [17]
  unit:
    - Bell icon renders without errors
    - Badge shows correct count for counts 1-99
    - Badge shows "99+" for counts > 99
    - Badge is hidden when count is 0
    - Dropdown opens on bell click
    - Dropdown closes on outside click
    - Dropdown closes on Escape key
    - Notifications render with title and truncated body
    - "Mark all as read" calls PUT endpoint for each notification
    - Clicking notification triggers navigation

  integration:
    - Count polling updates badge every 30 seconds
    - Opening dropdown fetches notification list
    - Marking notification as read decrements badge count
    - Dropdown position is correct in various viewport sizes

  accessibility:
    - Bell button has aria-label "Notifications"
    - Badge count is announced via aria-live
    - Dropdown traps focus when open
    - All notifications reachable via keyboard
    - Focus returns to bell button when dropdown closes


# [17] Tests are organized by type. Unit tests validate individual
#      behaviors. Integration tests validate interactions between
#      parts. Accessibility tests validate WCAG compliance.
#      Each test maps to one or more acceptance criteria or
#      constraints.
```

That is a complete micro-spec. Let me highlight the key properties:

1. **Self-contained.** Everything the AI needs to generate this component is in this document (plus the referenced system spec). No ambiguity, no implied requirements, no "you know what I mean."

2. **Structured.** Each section has a clear purpose. A reviewer can skip to the section they care about — PMs read the objective, security engineers read the constraints, QA reads the testing section.

3. **Traceable.** Every acceptance criterion can be traced to a test. Every constraint can be verified. Every piece of context can be validated against the actual codebase.

4. **Bounded.** The scope section explicitly says what is and is not included. The AI cannot add push notifications or notification sounds because the spec explicitly excludes them.

---

## 3.6 How Each Section Maps to AI Behavior

Let me be explicit about what each section of the micro-spec does to the AI's behavior. Understanding this mapping will help you write better specs.

### Context → Reduces Hallucination

Without context, the AI fills in blanks from its training data. With context, it has specific, grounded information to work with.

**Without context:**

```
Create a notification bell component.
```

The AI might:
- Invent its own API endpoints
- Use Socket.io for real-time updates
- Create a full notification model from scratch
- Choose a random icon library
- Guess at the data structure

**With context:**

```yaml
context:
  technical_context: >
    GET /api/notifications/count → { count: number }
    GET /api/notifications → { notifications: Notification[], total: number }
    Notification: { id: string, title: string, body: string, ... }
```

Now the AI uses the exact endpoints and data types you specified. No invention needed.

Here is a TypeScript example of what the AI generates with proper context:

```typescript
// With context: AI uses the exact API and types from the spec

interface Notification {
  id: string;
  title: string;
  body: string;
  type: "info" | "warning" | "error";
  read: boolean;
  createdAt: string;
}

interface NotificationCountResponse {
  count: number;
}

interface NotificationListResponse {
  notifications: Notification[];
  total: number;
}

function useNotificationCount() {
  return useQuery({
    queryKey: ["notifications", "count"],
    queryFn: async (): Promise<NotificationCountResponse> => {
      const response = await fetch("/api/notifications/count");
      return response.json();
    },
    refetchInterval: 30_000, // 30 seconds per spec
  });
}
```

Compare this to what the AI might generate without context:

```typescript
// Without context: AI invents everything

interface Notification {
  _id: string;          // MongoDB-style ID (your project uses UUID)
  message: string;      // Different field name than your API
  isRead: boolean;      // Different naming convention
  timestamp: number;    // Unix timestamp, but your API sends ISO string
  priority: number;     // Field that does not exist in your API
}

// Uses axios (not in your project's dependencies)
const fetchNotifications = async () => {
  const { data } = await axios.get("/notifications");  // Wrong URL path
  return data;
};
```

The difference is stark. Context does not just "help." It fundamentally changes the quality of the output.

### Objective → Defines Completeness

The acceptance criteria tell the AI *when it is done.* Without them, the AI uses its own judgment about what a "notification bell" includes. With them, it has a checklist.

This matters because AI models have a bias toward completeness — they tend to generate more rather than less. A detailed set of acceptance criteria *focuses* the AI on exactly what is needed:

```typescript
// The AI generates exactly these behaviors, no more, no less:

export function NotificationBell() {
  const { data: countData } = useNotificationCount();
  const [isOpen, setIsOpen] = useState(false);
  const count = countData?.count ?? 0;

  return (
    <div id="notification-slot">
      <button
        onClick={() => setIsOpen(!isOpen)}
        aria-label="Notifications"
      >
        <BellIcon />
        {/* Criterion: Badge shows count when > 0 */}
        {/* Criterion: Badge shows "99+" when count > 99 */}
        {/* Criterion: Badge is hidden when count is 0 */}
        {count > 0 && (
          <span aria-live="polite">
            {count > 99 ? "99+" : count}
          </span>
        )}
      </button>

      {/* Criterion: Clicking the bell toggles dropdown */}
      {isOpen && (
        <NotificationDropdown
          onClose={() => setIsOpen(false)}
        />
      )}
    </div>
  );
}
```

Each piece of the generated code traces to a specific acceptance criterion. Nothing extra. Nothing missing.

### Constraints → Prevents Bad Decisions

Constraints are the most direct form of AI control. They are explicit prohibitions and requirements that override the AI's default behavior.

Let me show you the difference constraints make with a real example:

**Without security constraints:**

```typescript
// AI's default: functional but insecure
function NotificationDropdown({ notifications }) {
  return (
    <div>
      {notifications.map((n) => (
        <div
          key={n.id}
          // DANGER: renders raw HTML from API without sanitization
          dangerouslySetInnerHTML={{ __html: n.body }}
        />
      ))}
    </div>
  );
}
```

**With constraint "MUST sanitize notification body before rendering":**

```typescript
// AI respects the constraint: sanitized input, safe preview
import DOMPurify from "dompurify";

function NotificationDropdown({ notifications }) {
  return (
    <div>
      {notifications.map((n) => {
        // Strip all tags to derive a plain-text preview, then truncate.
        // React then auto-escapes when rendering the preview as text.
        const preview = DOMPurify.sanitize(n.body, { ALLOWED_TAGS: [] }).slice(0, 50);
        return (
          <div key={n.id}>
            <p>{n.title}</p>
            <p>{preview}</p>
          </div>
        );
      })}
    </div>
  );
}
```

The constraint transformed the output from insecure to secure. That one line in the spec — `MUST sanitize notification body before rendering` — prevented a potential XSS vulnerability.

Two things worth naming in the fixed version. First, DOMPurify has to do real work: slicing a sanitized *HTML* string as if it were text can cut inside a tag and reintroduce the problem, so we strip tags (`ALLOWED_TAGS: []`) *before* truncating — slicing plain text is safe. Second, the preview is rendered inside `{...}` rather than `dangerouslySetInnerHTML`, so React's built-in escaping is the final line of defense. The AI's "bad" default was structurally unsafe; the "good" version is safe by construction.

---

## 3.7 Common Mistakes: Over-Specifying vs. Under-Specifying

There is a sweet spot for spec detail. Too little, and the AI fills in blanks incorrectly. Too much, and the spec becomes as complex as the code itself, defeating the purpose.

### The Under-Specified Spec

```yaml
# BAD: Under-specified
kind: Component
metadata:
  name: NotificationBell

objective:
  summary: Show notifications to the user.
```

This gives the AI almost nothing to work with. It will make dozens of implicit decisions about API endpoints, data structures, behavior, styling, accessibility, and scope. The result will be functional but will almost certainly not match your intent.

### The Over-Specified Spec

```yaml
# BAD: Over-specified
kind: Component
metadata:
  name: NotificationBell

objective:
  acceptance_criteria:
    - Bell icon MUST be 24x24 pixels
    - Bell icon MUST use SVG path "M12 22c1.1 0 2-.9 2-2h-4c0..."
    - Badge MUST be positioned top: -4px, right: -4px
    - Badge MUST use background-color: #EF4444
    - Badge MUST use font-size: 10px
    - Badge MUST use border-radius: 9999px
    - Dropdown MUST be positioned 8px below the bell
    - Dropdown MUST have width: 320px
    - Dropdown MUST have max-height: 400px
    - Dropdown MUST have box-shadow: 0 4px 6px rgba(0,0,0,0.1)
    - Each notification row MUST have padding: 12px 16px
    - Each notification title MUST use font-weight: 600
    - Each notification body MUST use font-size: 14px
    - Each notification body MUST use color: #6B7280
    - Time ago MUST use font-size: 12px
    - Time ago MUST use color: #9CA3AF
    - Use useState for isOpen with initial value false
    - Use useQuery with queryKey ["notifications", "count"]
    - Use useQuery with queryFn that calls fetch("/api/notifications/count")
    - Set refetchInterval to 30000
    ...
```

This spec is more detailed than the code itself. It micromanages the AI, specifying pixel values, colors, CSS properties, and even variable names. If you are going to be this specific, you might as well write the code yourself.

### The Sweet Spot

```yaml
# GOOD: Right level of detail
kind: Component
metadata:
  name: NotificationBell

context:
  technical_context: >
    API: GET /api/notifications/count → { count: number }
    API: GET /api/notifications → { notifications: Notification[] }
    Place in #notification-slot in HeaderBar component.

objective:
  summary: >
    Create a notification bell icon with unread count badge
    that opens a dropdown showing recent notifications.
  acceptance_criteria:
    - Badge shows count when > 0, "99+" when > 99, hidden when 0
    - Clicking bell toggles a dropdown with 5 most recent notifications
    - Each notification shows title, body preview, and time ago
    - Clicking a notification marks it as read
    - Count refreshes via polling every 30 seconds

constraints:
  - MUST use React Query for all data fetching
  - MUST sanitize notification body with DOMPurify
  - MUST support keyboard navigation and screen readers
  - MUST NOT fetch notifications until dropdown opens
  - MUST NOT introduce new dependencies
```

This spec tells the AI *what to build* (objective), *in what context* (context), and *what guardrails to respect* (constraints). It does not tell the AI *how to build it* — the variable names, the CSS values, the component structure. Those are implementation decisions that the AI is well-equipped to make.

> **Professor's Aside:** A good rule of thumb: if changing a detail in your spec would NOT change the user-visible behavior or the architectural qualities (security, performance, accessibility) of the feature, that detail is probably too specific. The badge being `#EF4444` vs. `#DC2626` does not change behavior. The debounce being 300ms vs. 100ms DOES change performance characteristics. Spec the latter, not the former.

---

## 3.8 The Micro-Spec and Industry Parallels

The micro-spec structure I have described — Context, Objective, Constraints — is not an arbitrary invention. It mirrors patterns that have emerged independently across the AI industry.

### OpenAI's Function Calling Schema

When you define a function for OpenAI's API, you provide:

```typescript
{
  name: "search_products",                    // Identity (metadata)
  description: "Search the product catalog",  // Objective (what it does)
  parameters: {                               // Constraints (valid inputs)
    type: "object",
    properties: {
      query: { type: "string", minLength: 2 },    // Constraint
      category: { type: "string", enum: [...] },   // Constraint
      maxResults: { type: "integer", maximum: 50 } // Constraint
    },
    required: ["query"]                             // Constraint
  }
}
```

This is a micro-spec for a function. It has identity (name), objective (description), and constraints (parameter schema with types, enums, min/max values, and required fields). The structure is different, but the pillars are the same.

### Anthropic's Tool Use Definitions

Anthropic's tool use format for Claude follows the same pattern:

```typescript
{
  name: "get_stock_price",                    // Identity
  description:                                 // Objective + Context
    "Get the current stock price for a given ticker symbol. " +
    "Returns price in USD. Only works for US-listed stocks.",
  input_schema: {                             // Constraints
    type: "object",
    properties: {
      ticker: {
        type: "string",
        description: "Stock ticker symbol (e.g., 'AAPL', 'GOOGL')",
        pattern: "^[A-Z]{1,5}$"              // Constraint: format
      }
    },
    required: ["ticker"]                      // Constraint: required
  }
}
```

Again: identity, objective, constraints. The description provides context ("only works for US-listed stocks") and objective ("get the current stock price"). The schema provides constraints (type, pattern, required).

### Google's Vertex AI Function Declarations

Google's approach with Gemini and Vertex AI is structurally identical:

```typescript
{
  name: "control_light",                      // Identity
  description:                                 // Objective
    "Set the brightness and color of a room light.",
  parameters: {                               // Constraints
    type: "object",
    properties: {
      brightness: {
        type: "number",
        description: "Light level from 0 to 100",
        minimum: 0,                           // Constraint
        maximum: 100                          // Constraint
      },
      colorTemperature: {
        type: "string",
        enum: ["daylight", "cool", "warm"],   // Constraint
        description: "Color temperature of the light"
      }
    },
    required: ["brightness"]                  // Constraint
  }
}
```

### The Pattern Is Universal

The convergence is striking. Three different companies, building three different AI systems, independently arrived at the same pattern: **structured identity + structured objective + structured constraints produces reliable AI behavior.**

SDD takes this pattern — which these companies apply at the function/tool level — and applies it at the feature level. A micro-spec is a function definition for a feature. It tells the AI what the feature is (identity/metadata), what it should do (objective/acceptance criteria), and what guardrails apply (constraints).

The reason this pattern works is not specific to any company's implementation. It works because it maps to the fundamental structure of intelligent task execution: know where you are, know where you are going, know what to avoid. This is true for AI models, for human developers, and for any agent executing in a complex environment.

---

## 3.9 Building a Micro-Spec: Step by Step

Let me walk you through the process of building a micro-spec from scratch. We will take a real feature request and construct the spec methodically.

**Feature request (from a PM, in Slack):**

> "We need to add a dark mode toggle to the app. Users have been asking for it."

### Step 1: Identify the Kind

What type of thing is this? A dark mode toggle is a UI component, but it also involves theme state management and CSS changes. Let us spec it as a Feature (which may produce multiple components).

```yaml
kind: Feature
metadata:
  name: DarkMode
  module: theme
  version: 1.0.0
```

### Step 2: Gather Context

Before writing the spec, we need to understand the current state. Ask yourself (or investigate the codebase):

- What styling system does the app use? (Tailwind CSS 4.0)
- Is there any existing theme system? (No)
- How is global state managed? (Zustand)
- Where should the toggle go? (HeaderBar, next to user avatar)
- How are preferences stored? (User preferences API at /api/preferences)

```yaml
context:
  description: >
    Users have requested dark mode support. The app currently has
    no theme system — all styles use Tailwind's default (light) theme.
    User preferences are stored via the preferences API.
  technical_context: >
    Styling: Tailwind CSS 4.0 with dark mode class strategy,
    configured in CSS via @custom-variant dark (&:where(.dark, .dark *)).
    State: Zustand for global state.
    Preferences API:
      GET /api/preferences → { theme: "light" | "dark" | "system", ... }
      PUT /api/preferences → updates preference
    The toggle should go in the HeaderBar component, next to
    the existing UserAvatar component.
    Tailwind's dark: variant is already available but unused.
```

### Step 3: Define the Objective

What is the specific change? Be precise about the delta.

```yaml
objective:
  summary: >
    Add dark mode support with a toggle in the header bar that
    allows users to switch between light, dark, and system-default
    themes. Persist the preference to the user's account.
  acceptance_criteria:
    - Toggle in HeaderBar shows current theme mode (icon changes)
    - Three options: Light, Dark, System (follows OS preference)
    - Selecting a theme immediately applies it (no page reload)
    - Theme preference is persisted to the user's account via API
    - On first load, theme is read from user preference (or system default)
    - Dark mode applies appropriate Tailwind dark: styles globally
    - System mode responds to OS theme changes in real-time
    - Unauthenticated users can use the toggle (stored in localStorage)
    - When user logs in, localStorage preference syncs to account
  scope:
    includes:
      - Theme toggle component in header
      - Theme state management (Zustand store)
      - Theme persistence (API for authenticated, localStorage for anonymous)
      - Dark class management on document root
      - OS theme detection via matchMedia
    excludes:
      - Redesigning existing components for dark mode (separate task)
      - Custom theme colors (only light/dark, using Tailwind defaults)
      - Theme scheduling (auto-switch at certain times)
      - Per-page or per-component theme overrides
```

### Step 4: Define Constraints

What could go wrong? What must not happen?

```yaml
constraints:
  - MUST NOT cause flash of unstyled content (FOUC) on page load
  - MUST apply theme class to <html> element before React hydration
  - MUST use Tailwind's dark: variant (no custom CSS variables for theme)
  - MUST NOT store theme preference in cookies (use API or localStorage)
  - MUST debounce theme toggle to prevent rapid API calls
  - MUST handle API failure gracefully (apply theme locally even if save fails)
  - MUST NOT introduce new dependencies (no theme libraries)
  - MUST respect prefers-reduced-motion for theme transitions
  - MUST ensure all existing Tailwind classes work correctly in dark mode
  - Toggle MUST be accessible (keyboard operable, screen reader labels)
```

### Step 5: Define Testing

What tests validate that the implementation matches the spec?

```yaml
testing:
  unit:
    - Toggle renders with correct icon for current theme
    - Toggle cycles through light → dark → system options
    - Theme class is applied to document.documentElement
    - System theme matches OS preference via matchMedia
    - Preference is saved to API on change (authenticated user)
    - Preference is saved to localStorage on change (anonymous user)
  integration:
    - Full cycle: change theme → reload page → theme persists
    - System mode responds to OS theme change
    - Login syncs localStorage preference to API
  visual:
    - No FOUC on initial page load
    - Smooth transition between themes (if motion allowed)
  accessibility:
    - Toggle is keyboard operable
    - Current theme is announced to screen readers
```

### The Complete Spec

Putting it all together, we have a comprehensive micro-spec that any AI model can use to generate a consistent, high-quality dark mode implementation. The spec is roughly 80 lines. The generated code might be 200-400 lines. But those 80 lines of spec ensure that the 200-400 lines of code are *correct* — that they match the team's intent, respect the project's conventions, and avoid common pitfalls.

---

## 3.10 Spec Quality Evaluation

How do you know if a spec is good? Here is a rubric you can use to evaluate any micro-spec:

### Completeness (0-10)

- Does the context include the technology stack?
- Does the context describe existing patterns and conventions?
- Does the context identify integration points?
- Does the objective have a clear summary?
- Does the objective have testable acceptance criteria?
- Does the scope explicitly include AND exclude items?
- Are there technical constraints?
- Are there security constraints?
- Are there accessibility constraints?
- Are there tests defined for each acceptance criterion?

### Clarity (0-5)

- Is each acceptance criterion unambiguous?
- Do constraints use MUST/MUST NOT vocabulary?
- Could a developer who has never seen the codebase implement this from the spec alone?
- Are there any vague terms ("fast," "good," "modern," "clean")?
- Is the scope boundary clear?

### Appropriateness of Detail (0-5)

- Does the spec avoid implementation details (variable names, CSS values)?
- Does the spec focus on behavior, not structure?
- Is the spec significantly shorter than the expected code?
- Could the implementation change without the spec changing?
- Are constraints focused on qualities (security, performance) rather than implementation?

A score of 15+ out of 20 indicates a high-quality spec. Below 10, the spec needs revision before code generation.

---

## 3.11 Templates and Starting Points

To make spec writing practical, here are template starting points for common spec types.

### Component Spec Template

```yaml
kind: Component
metadata:
  name: [ComponentName]
  module: [feature-domain]
  version: 1.0.0
  system_spec: system.spec.yaml@[version]

context:
  description: >
    [Why does this component need to exist? What problem does it solve?]
  technical_context: >
    [What APIs, types, and existing components does it interact with?]

objective:
  summary: >
    [One sentence: Create/Add/Modify [what] that [purpose].]
  acceptance_criteria:
    - [Testable criterion 1]
    - [Testable criterion 2]
    - [...]
  scope:
    includes:
      - [Explicit capability 1]
    excludes:
      - [Explicit exclusion 1]

constraints:
  - MUST [technical requirement]
  - MUST NOT [technical prohibition]
  - MUST [security requirement]
  - MUST [accessibility requirement]

testing:
  unit:
    - [Test case derived from acceptance criterion]
  accessibility:
    - [Accessibility test case]
```

### API Endpoint Spec Template

```yaml
kind: Endpoint
metadata:
  name: [EndpointName]
  module: [feature-domain]
  version: 1.0.0
  system_spec: system.spec.yaml@[version]

context:
  description: >
    [Why does this endpoint need to exist?]
  technical_context: >
    [Database tables, existing services, authentication requirements]

endpoint:
  method: [GET|POST|PUT|DELETE|PATCH]
  path: /api/[resource]
  authentication: [required|optional|none]
  authorization: [roles that can access]

request:
  params:
    - name: [param]
      type: [string|number|boolean]
      required: [true|false]
      description: [what it is]
      validation: [rules]
  body:
    type: object
    properties:
      [field]:
        type: [type]
        required: [true|false]
        validation: [rules]

response:
  success:
    status: [200|201|204]
    body:
      type: object
      properties:
        [field]:
          type: [type]
  errors:
    - status: 400
      condition: [when this error occurs]
      body: { error: "[message]" }
    - status: 401
      condition: Unauthenticated request
    - status: 403
      condition: Insufficient permissions
    - status: 404
      condition: Resource not found

constraints:
  - MUST validate all input before processing
  - MUST NOT expose internal error details
  - MUST [rate limiting requirement]
  - MUST [data access constraint]

testing:
  unit:
    - [Test case for success path]
    - [Test case for each error condition]
  integration:
    - [Test case for database interaction]
  security:
    - [Test for authentication requirement]
    - [Test for authorization requirement]
    - [Test for input validation]
```

These templates are starting points, not rigid forms. Adapt them to your project's needs. The important thing is that every spec covers the three pillars — Context, Objective, Constraints — and that each pillar is detailed enough to guide AI code generation.

---

## Chapter Summary

| Concept | Key Takeaway |
|---|---|
| Three Pillars | Every micro-spec has Context (what exists), Objective (what should change), and Constraints (what must not happen). |
| Context Reduces Hallucination | Explicit context gives the AI grounded information instead of training data averages. |
| Objective Defines Completeness | Acceptance criteria tell the AI when it is done. Scope defines boundaries. |
| Constraints Prevent Bad Decisions | MUST/MUST NOT rules override the AI's default behaviors and prevent security, performance, and consistency issues. |
| Delta Principle | Specs describe changes, not states. This keeps the AI focused on what is new. |
| Sweet Spot | Spec what and why, not how. Avoid both under-specifying (too vague) and over-specifying (too detailed). |
| Industry Parallels | OpenAI function schemas, Anthropic tool definitions, and Google function declarations all follow the Context-Objective-Constraint pattern. |
| Quality Rubric | Evaluate specs on Completeness (0-10), Clarity (0-5), and Appropriateness of Detail (0-5). |

---

## Discussion Questions

1. Take a feature you recently built and write the Context section of its micro-spec. What information would the AI need that you take for granted because you are familiar with the codebase?

2. Look at the "over-specified" example in Section 3.7. Why is specifying CSS pixel values in the spec a problem? Where should visual design details live if not in the spec?

3. The chapter argues that constraints are "the most important part of the spec." Do you agree? Can you think of a constraint that, if missing, would lead to a serious defect?

4. Compare the micro-spec structure (Context, Objective, Constraints) to a Jira ticket. What does the micro-spec capture that a Jira ticket typically does not? What does a Jira ticket capture that a micro-spec typically does not?

---

*Next: Chapter 4 — "Practice: From Vibe to Spec"*

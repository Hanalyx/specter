# Chapter 2: The Component Contract

## MODULE 02 — Defining the Architecture (The "How") | Intermediate Level

---

## Lecture Preamble

*The professor pulls up a design mockup on the projector — a search bar with autocomplete suggestions, a loading spinner, and a "clear" button. Then the professor closes the mockup and opens a blank code editor.*

Forget the mockup. I do not care what that search bar looks like right now. I care about what it **does**. I care about what data it accepts, what state it manages, what events it emits, and what external systems it touches. I care about its **contract**.

This is one of the hardest lessons for frontend developers to internalize: the visual design of a component is the **least important** part of its specification. Pixels can change overnight. Colors are subjective. Spacing is debatable. But the contract — the props, state, and side effects — that is the architecture. That is what makes or breaks your application.

Today we are going to learn how to write **component contracts**: specifications that fully define a UI component's behavior without describing a single pixel of its appearance. And when we hand these contracts to an AI, the AI will produce components that work correctly every time — even if the styling needs adjustment later.

---

## 2.1 What Is a Component Contract?

A component contract is a formal specification that defines three things about a UI component:

1. **Props** — the inputs the component accepts from its parent
2. **State** — the internal data the component manages
3. **Side Effects** — the external systems the component interacts with

Notice what is NOT in that list: colors, fonts, spacing, animations, border radius, box shadows, or any other visual property. Those belong in a **visual spec**, which is a separate concern.

```typescript
// THIS is a component contract:
interface SearchBarContract {
  // Props (inputs from parent)
  props: {
    placeholder: string;
    initialValue?: string;
    maxLength?: number;
    onSearch: (query: string) => void;
    onClear: () => void;
    suggestions: Suggestion[];
    isLoading: boolean;
    disabled?: boolean;
  };

  // Internal state
  state: {
    inputValue: string;
    isFocused: boolean;
    selectedSuggestionIndex: number;  // -1 means none selected
    showSuggestions: boolean;
  };

  // Side effects
  sideEffects: {
    debounce: "Debounce input changes by 300ms before calling onSearch";
    focus: "Auto-focus input on mount if no initialValue provided";
    keyboard: "Handle ArrowUp, ArrowDown, Enter, Escape for suggestion navigation";
    clickOutside: "Close suggestions dropdown when clicking outside component";
  };
}

interface Suggestion {
  id: string;
  text: string;
  category?: string;
  icon?: string;  // Icon identifier, not a component
}
```

That contract tells a developer (or an AI) everything needed to implement the SearchBar. It says nothing about whether the suggestions appear in a dropdown or a modal, whether the loading spinner is a circle or a bar, or whether the component uses Tailwind or CSS modules. Those are implementation details that belong elsewhere.

> **Professor's Aside:** I have a rule in my own projects: if you cannot describe a component's contract without mentioning a CSS property, you are conflating behavior with appearance. Separate them ruthlessly. A button's contract is about what happens when you click it, not what color it turns.

---

## 2.2 Separation of Concerns: Behavior Spec vs. Visual Spec

The distinction between behavior specification and visual specification is fundamental to SDD component design. Let me make it crystal clear.

### The Behavior Spec (Component Contract)

The behavior spec answers these questions:

- What data does this component need?
- What can the user do with this component?
- What happens in response to user actions?
- What external systems does this component touch?
- What are the edge cases and error states?

```typescript
// Behavior spec for a QuantitySelector component
interface QuantitySelectorContract {
  props: {
    value: number;
    min: number;
    max: number;
    step: number;
    onChange: (newValue: number) => void;
    disabled?: boolean;
  };

  behavior: {
    increment: "Increase value by step, capped at max";
    decrement: "Decrease value by step, floored at min";
    directInput: "Allow typing a number; validate on blur";
    invalidInput: "If user types non-numeric or out-of-range, revert to previous valid value";
    disabledState: "All interactions disabled; visual indication of disabled state";
  };

  edgeCases: {
    "value exceeds max on mount": "Clamp to max";
    "value below min on mount": "Clamp to min";
    "step causes value to exceed max": "Set to max instead of max + remainder";
    "min equals max": "Both buttons disabled; input is read-only";
    "negative step": "Rejected — step must be positive";
  };
}
```

### The Visual Spec (Design System)

The visual spec answers different questions:

- What does this component look like?
- What are the colors, sizes, and spacing?
- How does it animate?
- How does it respond to different screen sizes?

```
Visual Spec: QuantitySelector
- Height: 40px
- Button width: 40px each (square)
- Input width: 60px, centered between buttons
- Font: 14px, medium weight
- Border: 1px solid gray-300, rounded-md
- Hover state: border-gray-400
- Focus state: ring-2 ring-blue-500
- Disabled state: opacity-50, cursor-not-allowed
- Button icons: minus (left), plus (right)
- Mobile: full width of container
```

### Why Separate Them?

Three reasons:

**1. AI generates better code when concerns are separated.**

When you mix behavior and visual specs in a single prompt, AI models often sacrifice behavioral correctness for visual accuracy. They will get the colors right but miss an edge case. By separating the specs, you can validate behavior independently of appearance.

**2. Behavior changes less frequently than visuals.**

A SearchBar's contract (accept query, show suggestions, handle keyboard nav) is stable for years. Its visual design changes with every rebrand. Separating the specs means you can update the visual layer without touching the behavioral contract.

**3. Different people own different specs.**

Engineers own the behavior spec. Designers own the visual spec. When they are in the same document, you get merge conflicts, confusion about authority, and design-by-committee paralysis.

> **Professor's Aside:** Meta's React team learned this lesson the hard way during the development of their internal component library. Early versions of their components mixed behavior and styling so tightly that changing a button's appearance required understanding its state machine. The refactoring to separate concerns — behavior hooks on one side, visual primitives on the other — is what eventually became the pattern we now see in Headless UI libraries like Radix, React Aria, and Headless UI itself.

---

## 2.3 The Props Interface as a Contract

In component-based frameworks (React, Vue, Svelte, Angular), the props interface is the **primary contract** between a parent component and a child component. It is the API surface of your component.

### Designing Props as Contracts

Good props design follows the same principles as good API design:

```typescript
// ===================================================
// MODAL DIALOG — COMPONENT CONTRACT
// ===================================================

interface ModalProps {
  // --- Required Props ---
  /** Whether the modal is currently open */
  isOpen: boolean;

  /** Called when the modal requests to be closed */
  onClose: () => void;

  /** The main content of the modal */
  children: React.ReactNode;

  // --- Content Props ---
  /** Title displayed in the modal header */
  title: string;

  /** Optional description below the title */
  description?: string;

  // --- Behavior Props ---
  /** Whether clicking the overlay closes the modal. Default: true */
  closeOnOverlayClick?: boolean;

  /** Whether pressing Escape closes the modal. Default: true */
  closeOnEscape?: boolean;

  /** Whether to show the X close button. Default: true */
  showCloseButton?: boolean;

  /** Whether the modal can be dismissed at all. Default: true.
   *  When false: no X button, no overlay click, no Escape.
   *  Used for mandatory confirmations. */
  dismissible?: boolean;

  // --- Lifecycle Props ---
  /** Called after the modal open animation completes */
  onAfterOpen?: () => void;

  /** Called after the modal close animation completes */
  onAfterClose?: () => void;

  // --- Accessibility Props ---
  /** ARIA label for the modal (defaults to title) */
  ariaLabel?: string;

  /** ID of the element that describes the modal */
  ariaDescribedBy?: string;

  /** Where to return focus when modal closes. Default: trigger element */
  returnFocusTo?: HTMLElement | null;

  // --- Layout Props (behavior, not styling) ---
  /** Size preset affecting max-width. Default: "medium" */
  size?: "small" | "medium" | "large" | "fullscreen";

  /** Whether the modal content is scrollable. Default: true */
  scrollable?: boolean;
}
```

Notice several important principles at work:

**Principle 1: Boolean props have sensible defaults.** `closeOnOverlayClick` defaults to `true`. The most common usage does not need to specify it.

**Principle 2: Callback props use the `on` prefix.** `onClose`, `onAfterOpen`, `onAfterClose`. This is a universal convention that AI models understand perfectly.

**Principle 3: Accessibility is part of the contract, not an afterthought.** `ariaLabel`, `ariaDescribedBy`, and `returnFocusTo` are first-class props.

**Principle 4: The `dismissible` prop is a composite behavior.** Instead of making the consumer set three booleans (`closeOnOverlayClick={false} closeOnEscape={false} showCloseButton={false}`), we provide a single semantic prop that controls all three. This is contract-level thinking.

**Principle 5: `size` is a behavior prop, not a style prop.** It affects max-width constraints, not colors or fonts. The visual spec determines what "medium" means in pixels.

### Props as Documentation

When you write a thorough props interface with JSDoc comments, you have simultaneously created:

- A TypeScript contract (compile-time enforcement)
- Component documentation (hoverable in VS Code)
- An AI prompt (the AI can read the interface and understand the component)
- A test specification (each prop suggests tests to write)

### The "Controlled vs. Uncontrolled" Decision

One of the most important decisions in a component contract is whether state is controlled or uncontrolled:

```typescript
// CONTROLLED: Parent owns the state
interface ControlledInputProps {
  value: string;
  onChange: (value: string) => void;
}

// UNCONTROLLED: Component owns the state
interface UncontrolledInputProps {
  defaultValue?: string;
  onChangeComplete?: (value: string) => void;
}

// HYBRID: Supports both patterns
interface FlexibleInputProps {
  value?: string;            // If provided, component is controlled
  defaultValue?: string;     // Used only if value is undefined
  onChange?: (value: string) => void;
}
```

In your component contract, explicitly state which pattern is used:

```typescript
interface DatePickerContract {
  // This is a CONTROLLED component.
  // The parent must manage the selected date.
  props: {
    selectedDate: Date | null;        // Controlled value
    onChange: (date: Date | null) => void;  // Required callback
    minDate?: Date;
    maxDate?: Date;
    disabled?: boolean;
    disabledDates?: Date[];           // Specific dates to disable
  };

  reasoning: {
    whyControlled:
      "Date selection typically needs to coordinate with other form fields " +
      "(e.g., check-in/check-out dates). Controlled pattern makes this " +
      "coordination explicit and predictable.";
  };
}
```

> **Professor's Aside:** The controlled vs. uncontrolled decision should be made at spec time, not implementation time. If you leave this ambiguous, the AI will guess — and it will guess differently in different files. I have seen codebases where the same DatePicker component is used in controlled mode in one form and uncontrolled mode in another, because nobody specified which pattern to use. The result was two different behaviors for the same component, and a week of debugging.

---

## 2.4 State Machines as Component Specs

One of the most powerful tools for specifying component behavior is the state machine. A state machine defines:

- All possible **states** a component can be in
- All possible **events** (triggers) that cause transitions
- Which **transitions** are valid (and which are not)
- What **actions** occur during transitions

This is the core philosophy behind XState, a popular state machine library for JavaScript. But you do not need to use XState to think in state machines — the concept is universal.

### Example: File Upload Component

Let us spec a file upload component using state machine thinking:

```typescript
// ===================================================
// FILE UPLOAD COMPONENT — STATE MACHINE SPEC
// ===================================================

// All possible states
type FileUploadState =
  | { status: "idle" }
  | { status: "dragover" }
  | { status: "validating"; file: File }
  | { status: "uploading"; file: File; progress: number }
  | { status: "processing"; file: File }  // Server is processing
  | { status: "success"; file: File; url: string }
  | { status: "error"; file: File | null; errorCode: UploadErrorCode; message: string };

type UploadErrorCode =
  | "FILE_TOO_LARGE"
  | "INVALID_TYPE"
  | "NETWORK_ERROR"
  | "SERVER_ERROR"
  | "QUOTA_EXCEEDED"
  | "TIMEOUT";

// All possible events
type FileUploadEvent =
  | { type: "DRAG_ENTER" }
  | { type: "DRAG_LEAVE" }
  | { type: "DROP"; file: File }
  | { type: "SELECT_FILE"; file: File }
  | { type: "VALIDATION_SUCCESS"; file: File }
  | { type: "VALIDATION_FAILURE"; errorCode: UploadErrorCode; message: string }
  | { type: "UPLOAD_PROGRESS"; progress: number }
  | { type: "UPLOAD_COMPLETE"; file: File }
  | { type: "PROCESSING_COMPLETE"; url: string }
  | { type: "ERROR"; errorCode: UploadErrorCode; message: string }
  | { type: "RETRY" }
  | { type: "CANCEL" }
  | { type: "RESET" };

// Transition table — the heart of the spec
interface FileUploadTransitions {
  idle: {
    DRAG_ENTER: "dragover";
    SELECT_FILE: "validating";
  };
  dragover: {
    DRAG_LEAVE: "idle";
    DROP: "validating";
  };
  validating: {
    VALIDATION_SUCCESS: "uploading";
    VALIDATION_FAILURE: "error";
  };
  uploading: {
    UPLOAD_PROGRESS: "uploading";     // Self-transition with updated progress
    UPLOAD_COMPLETE: "processing";
    ERROR: "error";
    CANCEL: "idle";
  };
  processing: {
    PROCESSING_COMPLETE: "success";
    ERROR: "error";
    // Note: CANCEL is NOT valid here — server is already processing
  };
  success: {
    RESET: "idle";
  };
  error: {
    RETRY: "uploading";  // Only if file exists in error state
    RESET: "idle";
  };
}

// Validation rules (run during "validating" state)
interface FileValidationRules {
  maxSizeBytes: number;          // e.g., 10 * 1024 * 1024 (10MB)
  allowedMimeTypes: string[];    // e.g., ["image/jpeg", "image/png", "application/pdf"]
  allowedExtensions: string[];   // e.g., [".jpg", ".jpeg", ".png", ".pdf"]
}
```

Now look at what this state machine spec gives us:

1. **Every possible state is enumerated.** There is no ambiguity about what states the component can be in.

2. **Every possible event is enumerated.** We know exactly what can happen.

3. **Invalid transitions are excluded.** You cannot CANCEL during the "processing" state. You cannot RETRY from the "idle" state. These constraints are explicit.

4. **The data associated with each state is defined.** The "uploading" state carries `progress`. The "error" state carries an `errorCode`. The "success" state carries a `url`.

5. **This is testable.** Every transition is a test case.

### Visualizing the State Machine

The spec above can be visualized as a state diagram:

```
                    +-----------+
                    |   idle    |<---------+----------+
                    +-----------+          |          |
                     |       |             |          |
              DRAG_ENTER  SELECT_FILE    RESET      CANCEL
                     |       |             |          |
                     v       |             |          |
                +-----------+|    +--------+-+   +----+-----+
                | dragover  ||    | success  |   | uploading|
                +-----------+|    +----------+   +----------+
                 |      |    |         ^              |    |
            DRAG_LEAVE  DROP |  PROCESSING_COMPLETE   |    |
                 |      |    |         |           ERROR  UPLOAD_COMPLETE
                 |      v    v         |              |    |
                 |  +------------+  +------------+    |    |
                 |  | validating |  | processing |<---+----+
                 |  +------------+  +------------+    |
                 |    |       |          |            |
                 | SUCCESS  FAILURE    ERROR          |
                 |    |       |          |            |
                 |    |       v          |            |
                 |    |    +--------+    |            |
                 |    +--->| error  |<---+            |
                 |         +--------+                 |
                 |           |    |                    |
                 |         RETRY  RESET               |
                 |           |    |                    |
                 |           +----+-----> idle         |
                 +------------------------------------+
```

> **Professor's Aside:** If you are using XState, you can literally take this spec and translate it into an XState machine definition. But even if you are not using XState, the state machine *thinking* is invaluable. It forces you to enumerate all states and transitions before writing code. I have seen this technique prevent more bugs than any other specification approach.

### State Machines for Form Components

Forms are particularly well-suited to state machine specs:

```typescript
type FormState =
  | { status: "pristine" }                         // No changes yet
  | { status: "dirty"; changedFields: string[] }   // User has made changes
  | { status: "validating" }                       // Async validation in progress
  | { status: "invalid"; errors: FieldError[] }    // Validation failed
  | { status: "submitting" }                       // Form is being submitted
  | { status: "submitted"; response: unknown }     // Submission succeeded
  | { status: "submitError"; error: string }       // Submission failed

interface FieldError {
  field: string;
  code: string;          // Machine-readable: "REQUIRED", "TOO_SHORT", etc.
  message: string;       // Human-readable: "Email is required"
}

type FormEvent =
  | { type: "FIELD_CHANGE"; field: string; value: unknown }
  | { type: "FIELD_BLUR"; field: string }
  | { type: "SUBMIT" }
  | { type: "VALIDATION_START" }
  | { type: "VALIDATION_COMPLETE"; errors: FieldError[] }
  | { type: "SUBMIT_SUCCESS"; response: unknown }
  | { type: "SUBMIT_FAILURE"; error: string }
  | { type: "RESET" };
```

---

## 2.5 Side Effects: What External Things Does This Component Touch?

Side effects are the most commonly under-specified part of a component. A side effect is any interaction with the world outside the component's own state. This includes:

- **API calls** — fetching or mutating data
- **Browser APIs** — localStorage, sessionStorage, clipboard, geolocation
- **Timers** — setTimeout, setInterval, debounce, throttle
- **DOM manipulation** — focus management, scroll position, resize observation
- **Event listeners** — keyboard shortcuts, click-outside, scroll, resize
- **Third-party services** — analytics, logging, error tracking
- **Navigation** — route changes, URL parameter updates

Every side effect in a component should be declared in the contract:

```typescript
// ===================================================
// AUTOCOMPLETE SEARCH — COMPLETE COMPONENT CONTRACT
// ===================================================

interface AutocompleteSearchContract {
  name: "AutocompleteSearch";
  description: "A search input with autocomplete suggestions fetched from an API";

  props: {
    /** API endpoint for fetching suggestions */
    endpoint: string;

    /** Minimum characters before triggering search */
    minChars: number;  // Default: 2

    /** Maximum number of suggestions to display */
    maxSuggestions: number;  // Default: 10

    /** Called when user selects a suggestion */
    onSelect: (suggestion: Suggestion) => void;

    /** Called when user submits the search (Enter without selecting) */
    onSubmit: (query: string) => void;

    /** Placeholder text */
    placeholder?: string;

    /** Whether the input is disabled */
    disabled?: boolean;

    /** Custom header to include in API requests (e.g., auth token) */
    requestHeaders?: Record<string, string>;
  };

  state: {
    query: string;
    suggestions: Suggestion[];
    isLoading: boolean;
    isFocused: boolean;
    isOpen: boolean;  // Whether suggestions dropdown is visible
    highlightedIndex: number;  // -1 = none highlighted
    error: string | null;
  };

  sideEffects: {
    apiCalls: {
      trigger: "query changes AND query.length >= minChars";
      endpoint: "GET ${props.endpoint}?q=${encodeURIComponent(query)}";
      debounce: "300ms";
      cancellation: "Cancel previous request when new one starts";
      errorHandling: "Set error state; do not throw; show inline error message";
      headers: "Include props.requestHeaders if provided";
    };

    keyboardListeners: {
      ArrowDown: "Highlight next suggestion (wrap to first after last)";
      ArrowUp: "Highlight previous suggestion (wrap to last before first)";
      Enter: "Select highlighted suggestion, or submit query if none highlighted";
      Escape: "Close suggestions dropdown; clear highlight";
      Tab: "Close suggestions dropdown; allow normal tab behavior";
    };

    clickOutside: {
      trigger: "Click anywhere outside the component";
      action: "Close suggestions dropdown";
      implementation: "useRef + document click listener";
    };

    focusManagement: {
      onMount: "Do NOT auto-focus (let parent decide)";
      onFocus: "Set isFocused=true; show suggestions if query.length >= minChars";
      onBlur: "Set isFocused=false; close suggestions after 150ms delay";
      blurDelay: "150ms delay allows click on suggestion to register before closing";
    };

    analytics: {
      trackSearch: "Fire analytics event when user submits search";
      trackSuggestionSelect: "Fire analytics event with suggestion.id when selected";
      trackNoResults: "Fire analytics event when API returns 0 suggestions";
    };
  };

  edgeCases: {
    "rapid typing": "Debounce ensures only final query triggers API call; previous requests cancelled";
    "empty response": "Show 'No results found' message; do not hide dropdown";
    "API error": "Show inline error; allow retry on next keystroke";
    "very long query": "Truncate display in input; send full query to API";
    "special characters": "URL-encode query before sending to API";
    "suggestion selected while loading": "Cancel pending request; use selected suggestion";
    "component unmount during request": "Cancel pending request; no state updates after unmount";
  };
}

interface Suggestion {
  id: string;
  label: string;         // Display text
  value: string;         // Actual value (may differ from label)
  category?: string;     // Optional grouping
  metadata?: Record<string, unknown>;  // Additional data for parent
}
```

That side effects section is dense. And it should be. Side effects are where bugs live. Let me call out the non-obvious decisions:

- **150ms blur delay**: Without this, clicking a suggestion triggers blur before the click registers, closing the dropdown and swallowing the click. This is a classic bug that a state machine spec catches.
- **Cancel previous request**: Without cancellation, a slow request for "hel" might resolve after a fast request for "hello", displaying wrong suggestions.
- **Component unmount during request**: Without cleanup, you get the React "Can't perform a state update on an unmounted component" warning.

Every one of these edge cases would require the AI (or a human developer) to discover independently if not specified. By including them in the contract, we ensure correct behavior on the first implementation.

---

## 2.6 How Design Systems Use Component Contracts

The largest tech companies in the world have learned — often through painful experience — that component contracts are essential for building consistent, reliable UI systems at scale.

### Google's Material Design Components

Google's Material Design system (now in its third major version, Material 3 / Material You) defines components through a layered specification:

1. **Anatomy** — the structural parts of a component (e.g., a dialog has a container, title, content area, action buttons)
2. **Behavior** — interaction patterns (e.g., how a dialog handles focus trap, Escape key, overlay clicks)
3. **States** — the possible states and transitions (e.g., a button can be enabled, disabled, focused, pressed, hovered)
4. **Accessibility** — ARIA roles, keyboard interactions, screen reader behavior

Material's component specs explicitly separate behavior from theming. The Material Web Components library provides behavior and structure; the theme layer provides colors, typography, and elevation.

This separation is why Material components can be "re-themed" for different Google products (Gmail, Docs, Calendar all use Material but look distinct) without changing behavioral contracts.

### Meta's React Component Ecosystem

Meta's internal React component library — which powers Facebook, Instagram, WhatsApp Web, and other products — uses a contract-driven approach. Each component has:

- **A TypeScript interface** defining all props
- **A behavior spec** defining interactions and state
- **Visual variants** defined through a design token system
- **Accessibility requirements** as part of the contract

Meta's approach to component design heavily influenced the broader React ecosystem. The patterns you see in libraries like Radix UI, Headless UI, and React Aria all trace back to lessons learned at Meta: **separate the behavior contract from the visual implementation.**

React Aria (by Adobe) takes this to the extreme. It provides hooks that implement component behavior (keyboard navigation, focus management, ARIA attributes) with zero styling. The contract is pure behavior:

```typescript
// React Aria's approach: behavior contract as a hook
import { useComboBox } from "react-aria";
import { useComboBoxState } from "react-stately";

// The state spec
const state = useComboBoxState({
  items: suggestions,
  onSelectionChange: (key) => onSelect(key),
  // ... behavioral configuration
});

// The behavior contract — returns props to spread onto DOM elements
const { inputProps, listBoxProps, labelProps } = useComboBox(
  {
    label: "Search",
    items: suggestions,
    onSelectionChange: (key) => onSelect(key),
  },
  state,
  inputRef
);

// You provide ALL the rendering:
return (
  <div>
    <label {...labelProps}>Search</label>
    <input {...inputProps} ref={inputRef} />
    <ul {...listBoxProps}>
      {/* Your custom rendering of suggestions */}
    </ul>
  </div>
);
```

The contract (what `useComboBox` expects and returns) is completely separate from the visual implementation (how you render the HTML).

### The Headless UI Pattern

This approach — providing behavior without opinions about rendering — has become known as the "headless UI" pattern. It is the purest expression of the component contract philosophy:

| Library       | Company | Approach                                      |
|---------------|---------|-----------------------------------------------|
| React Aria    | Adobe   | Hooks that return ARIA-compliant props         |
| Radix UI      | WorkOS  | Unstyled primitives with full behavior         |
| Headless UI   | Tailwind| Unstyled components for Tailwind users         |
| Downshift     | Paypal  | Headless combobox/select (Kent C. Dodds)       |
| TanStack Table| Tanner L.| Headless table with full feature set          |

All of these libraries are essentially **published component contracts** — they define the behavior, and you provide the visuals.

> **Professor's Aside:** Here is a prediction for this class: by the end of 2026, the dominant pattern for AI-generated UI will be headless-first. You will write a component contract (props, state, side effects), hand it to an AI, and the AI will generate a headless implementation with a styled wrapper. The contract stays stable; the styling layer can be regenerated for different design systems. This is already how the most sophisticated AI-assisted development teams work.

---

## 2.7 Real Example: Specifying a SearchBar Component

Let us go through a complete, production-grade example. We will specify a SearchBar component from scratch, covering every aspect of its contract.

### Step 1: Identify the Requirements

Before writing any interface, list what the SearchBar must do:

1. Accept text input from the user
2. Show autocomplete suggestions as the user types
3. Support keyboard navigation through suggestions
4. Allow the user to select a suggestion or submit a free-text query
5. Show a loading state while fetching suggestions
6. Support a "recent searches" feature
7. Allow the user to clear the input
8. Be fully accessible (screen readers, keyboard-only users)

### Step 2: Write the Props Contract

```typescript
// ===================================================
// SEARCHBAR COMPONENT — FULL SPECIFICATION
// ===================================================

// --- Props ---

interface SearchBarProps {
  // --- Core Behavior ---
  /** The current search query (controlled component) */
  value: string;

  /** Called on every input change */
  onChange: (value: string) => void;

  /** Called when the user submits a search (Enter key or submit button) */
  onSubmit: (query: string) => void;

  /** Called when the user selects a suggestion */
  onSuggestionSelect: (suggestion: SearchSuggestion) => void;

  /** Called when the user clears the input */
  onClear: () => void;

  // --- Data ---
  /** Autocomplete suggestions to display */
  suggestions: SearchSuggestion[];

  /** Recent searches to display when input is focused but empty */
  recentSearches: RecentSearch[];

  /** Whether suggestions are currently loading */
  isLoading: boolean;

  // --- Configuration ---
  /** Placeholder text for the input. Default: "Search..." */
  placeholder?: string;

  /** Maximum length of the search query. Default: 200 */
  maxLength?: number;

  /** Whether to show the recent searches section. Default: true */
  showRecentSearches?: boolean;

  /** Maximum number of suggestions to display. Default: 8 */
  maxSuggestions?: number;

  /** Maximum number of recent searches to display. Default: 5 */
  maxRecentSearches?: number;

  // --- State ---
  /** Whether the search bar is disabled */
  disabled?: boolean;

  /** Whether to auto-focus on mount. Default: false */
  autoFocus?: boolean;

  // --- Accessibility ---
  /** ARIA label for the search input */
  ariaLabel?: string;

  /** ID of the search input (for label association) */
  inputId?: string;

  // --- Callbacks ---
  /** Called when the input gains focus */
  onFocus?: () => void;

  /** Called when the input loses focus */
  onBlur?: () => void;

  /** Called when a recent search is removed */
  onRecentSearchRemove?: (searchId: string) => void;

  /** Called when all recent searches are cleared */
  onRecentSearchesClear?: () => void;
}

// --- Supporting Types ---

interface SearchSuggestion {
  id: string;
  type: "product" | "category" | "query" | "user";
  label: string;            // Display text
  sublabel?: string;        // Secondary text (e.g., category name)
  iconUrl?: string;         // Optional icon/thumbnail URL
  highlightRanges?: Array<{
    start: number;
    end: number;
  }>;  // Character ranges to bold in the label (matching the query)
}

interface RecentSearch {
  id: string;
  query: string;
  timestamp: string;  // ISO 8601
}
```

### Step 3: Write the State Contract

```typescript
// --- Internal State ---

interface SearchBarState {
  /** Whether the input is focused */
  isFocused: boolean;

  /** Whether the dropdown is open */
  isDropdownOpen: boolean;

  /** Which section of the dropdown is active */
  activeSection: "suggestions" | "recent" | null;

  /** Index of the currently highlighted item (-1 = none) */
  highlightedIndex: number;

  /** The composite list used for keyboard navigation.
   *  Merges suggestions and recent searches into a single navigable list. */
  navigableItems: NavigableItem[];
}

type NavigableItem =
  | { type: "suggestion"; index: number; data: SearchSuggestion }
  | { type: "recent"; index: number; data: RecentSearch }
  | { type: "action"; id: "clear-recent"; label: "Clear recent searches" };
```

### Step 4: Write the Behavior Contract

```typescript
// --- Behavior Specification ---

interface SearchBarBehavior {
  // --- Dropdown Visibility ---
  dropdown: {
    openWhen: [
      "Input is focused AND (suggestions.length > 0 OR recentSearches.length > 0)",
      "Input is focused AND isLoading is true",
    ];
    closeWhen: [
      "Input loses focus (after 150ms delay for click registration)",
      "User presses Escape",
      "User selects a suggestion",
      "User submits the search",
    ];
    content: {
      "when query is empty and showRecentSearches":
        "Show recent searches section";
      "when query is not empty and suggestions exist":
        "Show suggestions section";
      "when query is not empty and isLoading":
        "Show loading indicator";
      "when query is not empty and not loading and suggestions empty":
        "Show 'No results' message";
    };
  };

  // --- Keyboard Navigation ---
  keyboard: {
    ArrowDown: [
      "If dropdown closed: open dropdown",
      "If dropdown open: move highlight to next item (wrap from last to first)",
    ];
    ArrowUp: [
      "If dropdown open: move highlight to previous item (wrap from first to last)",
      "If at first item: remove highlight (return to input)",
    ];
    Enter: [
      "If item highlighted: select that item (suggestion or recent search)",
      "If no item highlighted: submit the current query via onSubmit",
    ];
    Escape: [
      "If dropdown open: close dropdown, clear highlight",
      "If dropdown closed: clear input value, call onClear",
    ];
    Tab: [
      "Close dropdown",
      "Allow normal tab behavior (move focus to next element)",
    ];
  };

  // --- Input Behavior ---
  input: {
    onChange: [
      "Update value via props.onChange",
      "Reset highlightedIndex to -1",
      "Open dropdown if it was closed",
    ];
    onFocus: [
      "Set isFocused to true",
      "Open dropdown if there are items to show",
      "Call props.onFocus if provided",
    ];
    onBlur: [
      "After 150ms delay:",
      "  Set isFocused to false",
      "  Close dropdown",
      "  Clear highlight",
      "  Call props.onBlur if provided",
    ];
  };

  // --- Clear Button ---
  clearButton: {
    visible: "When value.length > 0 AND not disabled";
    onClick: [
      "Clear input value via onClear callback",
      "Return focus to input",
    ];
  };

  // --- Recent Searches ---
  recentSearches: {
    itemClick: "Set input value to search.query, call onSubmit";
    removeClick: "Call onRecentSearchRemove with search.id; do NOT close dropdown";
    clearAll: "Call onRecentSearchesClear; close recent section";
  };
}
```

### Step 5: Write the Accessibility Contract

```typescript
// --- Accessibility Specification ---

interface SearchBarAccessibility {
  roles: {
    container: "role='combobox'";
    input: "role='searchbox' (implicit with type='search')";
    dropdown: "role='listbox'";
    suggestionItem: "role='option'";
    recentItem: "role='option'";
  };

  ariaAttributes: {
    input: {
      "aria-expanded": "true when dropdown is open, false otherwise";
      "aria-activedescendant": "ID of highlighted item, empty when none highlighted";
      "aria-autocomplete": "'list'";
      "aria-controls": "ID of the dropdown listbox";
      "aria-label": "props.ariaLabel or 'Search'";
    };
    dropdown: {
      "aria-label": "'Search suggestions' or 'Recent searches' based on content";
    };
    suggestionItem: {
      "aria-selected": "true when this item is highlighted";
    };
    clearButton: {
      "aria-label": "'Clear search'";
    };
    removeRecentButton: {
      "aria-label": "'Remove recent search: {query}'";
    };
  };

  focusManagement: {
    "keyboard navigation": "Focus stays on input; aria-activedescendant indicates visual highlight";
    "suggestion selection": "Focus returns to input after selection";
    "clear button": "Focus returns to input after clearing";
    "mount with autoFocus": "Input receives focus on mount";
  };

  screenReaderAnnouncements: {
    suggestionsLoaded: "'N suggestions available' announced via aria-live region";
    suggestionHighlighted: "Handled via aria-activedescendant (no extra announcement needed)";
    noResults: "'No results found' announced via aria-live region";
    searchCleared: "'Search cleared' announced via aria-live region";
  };
}
```

### Step 6: Write the Edge Cases

```typescript
// --- Edge Cases ---

interface SearchBarEdgeCases {
  "empty submit": "If value is empty and user presses Enter, do NOT call onSubmit";
  "whitespace-only submit": "Trim value; if empty after trim, do NOT call onSubmit";
  "disabled state": "All interactions disabled; dropdown never opens; clear button hidden";
  "suggestions update while navigating": "Reset highlightedIndex to -1; keep dropdown open";
  "value prop changes externally": "Update input display; if new value is empty, close suggestions";
  "very long suggestion label": "Truncate with ellipsis; full text in title attribute";
  "HTML in suggestion label": "Escape HTML; only render highlightRanges as bold";
  "duplicate suggestion IDs": "Use array index as fallback key; log warning in development";
  "rapid typing": "Parent is responsible for debouncing API calls; component just renders what it gets";
  "mobile behavior": "No hover states; tap to select; virtual keyboard should not obscure dropdown";
  "RTL languages": "Component must support right-to-left text direction";
}
```

This complete specification — props, state, behavior, accessibility, edge cases — is approximately 200 lines. An implementation will be 400-600 lines. But those 200 lines of spec ensure that the 400-600 lines of implementation are correct, consistent, and complete.

---

## 2.8 Anti-Patterns in Component Specification

### Anti-Pattern 1: Over-Specifying Visual Details

```typescript
// BAD: This is a visual spec pretending to be a behavior spec
interface ButtonContract {
  props: {
    label: string;
    onClick: () => void;
    // These are VISUAL concerns, not behavioral:
    backgroundColor: string;      // NO
    hoverBackgroundColor: string;  // NO
    borderRadius: number;          // NO
    fontSize: number;              // NO
    fontWeight: string;            // NO
    padding: string;               // NO
    boxShadow: string;             // NO
  };
}
```

**Fix:** Replace visual props with semantic variants:

```typescript
// GOOD: Behavior-focused with semantic variants
interface ButtonContract {
  props: {
    label: string;
    onClick: () => void;
    variant: "primary" | "secondary" | "destructive" | "ghost";
    size: "sm" | "md" | "lg";
    disabled?: boolean;
    loading?: boolean;
    iconBefore?: IconName;
    iconAfter?: IconName;
    fullWidth?: boolean;
  };
}
```

The visual meaning of "primary" or "destructive" is defined in the design system, not in the component contract.

### Anti-Pattern 2: Under-Specifying Behavior

```typescript
// BAD: Missing critical behavioral details
interface DropdownContract {
  props: {
    options: string[];
    onSelect: (option: string) => void;
  };
  // That is it? What about:
  // - Keyboard navigation?
  // - Multi-select?
  // - Search/filter?
  // - Accessibility?
  // - Loading state?
  // - Empty state?
  // - Positioning (what if it overflows the viewport)?
}
```

**Fix:** Enumerate all behaviors, even if some are "default":

```typescript
// GOOD: Complete behavioral specification
interface DropdownContract {
  props: {
    options: DropdownOption[];
    value: string | string[] | null;  // Current selection
    onChange: (value: string | string[]) => void;
    multiple?: boolean;               // Default: false
    searchable?: boolean;             // Default: false
    loading?: boolean;
    disabled?: boolean;
    placeholder?: string;
    emptyMessage?: string;            // Default: "No options available"
    maxHeight?: number;               // Max dropdown height in px
    placement?: "bottom" | "top" | "auto";  // Default: "auto"
  };

  behavior: {
    keyboard: { /* ... full keyboard spec ... */ };
    positioning: "Auto-flip to top if insufficient space below";
    virtualScroll: "If options > 100, use virtualized list";
    search: "Filter options client-side; highlight matching text";
  };

  accessibility: {
    role: "listbox";
    ariaAttributes: { /* ... */ };
  };
}
```

### Anti-Pattern 3: Implicit Side Effects

```typescript
// BAD: Side effects are hidden in the implementation
interface UserProfileContract {
  props: {
    userId: string;
  };
  // Where does the user data come from?
  // Is there a fetch on mount?
  // Does it cache? Refetch on window focus?
  // Does it update the document title?
  // Does it track a page view?
}
```

**Fix:** Declare all side effects explicitly:

```typescript
// GOOD: All side effects are visible in the contract
interface UserProfileContract {
  props: {
    userId: string;
    onError?: (error: ApiError) => void;
  };

  sideEffects: {
    dataFetching: {
      endpoint: "GET /api/users/${userId}";
      trigger: "On mount and when userId changes";
      caching: "Cache for 5 minutes; stale-while-revalidate";
      refetch: "On window focus; on network reconnect";
    };
    documentTitle: {
      action: "Set document.title to '${user.displayName} — Profile'";
      cleanup: "Restore previous title on unmount";
    };
    analytics: {
      pageView: "Track 'profile_view' event with userId on mount";
      condition: "Only track once per mount (not on refetch)";
    };
  };
}
```

### Anti-Pattern 4: Props Drilling Specification

```typescript
// BAD: Specifying props that are just passed through to children
interface PageLayoutContract {
  props: {
    // These are all just passed to child components:
    headerTitle: string;
    headerSubtitle: string;
    headerShowLogo: boolean;
    sidebarItems: MenuItem[];
    sidebarCollapsed: boolean;
    sidebarOnToggle: () => void;
    footerLinks: Link[];
    footerCopyright: string;
    // ... 20 more pass-through props
  };
}
```

**Fix:** Use composition and reference child contracts:

```typescript
// GOOD: Compose child contracts
interface PageLayoutContract {
  props: {
    header: React.ReactNode;     // Parent renders <Header> with its own props
    sidebar: React.ReactNode;    // Parent renders <Sidebar> with its own props
    footer: React.ReactNode;     // Parent renders <Footer> with its own props
    children: React.ReactNode;   // Main content area
  };

  behavior: {
    layout: "CSS Grid with header, sidebar, main, footer regions";
    responsive: "Sidebar collapses to overlay on screens < 768px";
    scrolling: "Main content scrolls independently; header is sticky";
  };
}
```

---

## 2.9 From Contract to Implementation: The Handoff

Once your component contract is complete, the handoff to implementation (whether by a human or an AI) becomes straightforward.

### Handoff to AI

Here is a template for handing a component contract to an AI:

```markdown
## Implementation Request

Implement the following React component based on this contract.

### Component: SearchBar

### Contract:
[paste full contract here]

### Implementation Requirements:
- React 19 with TypeScript
- Use `useState` and `useEffect` (no external state library)
- Use `useRef` for DOM references
- Use `useCallback` for stable callback references
- Tailwind CSS for styling (utility classes only)
- Follow the accessibility spec exactly
- Handle all listed edge cases
- Include JSDoc comments on the component and its props

### Testing Requirements:
- Write tests using Vitest + React Testing Library
- Test every keyboard interaction
- Test every edge case listed in the contract
- Test accessibility (use @testing-library/jest-dom matchers)
- Use `userEvent` instead of `fireEvent` for realistic interactions

### File Structure:
- `SearchBar.tsx` — the component
- `SearchBar.test.tsx` — the tests
- `SearchBar.types.ts` — the TypeScript interfaces (from the contract)
```

The AI now has everything it needs. The contract eliminates guesswork. The implementation requirements eliminate ambiguity about tooling. The testing requirements ensure the contract is verified.

### Handoff Between Teams

The same contract facilitates human-to-human handoffs:

- **Backend team** reads the props to understand what data shape the frontend needs
- **Design team** reads the behavior spec to understand interaction requirements
- **QA team** reads the edge cases as a test plan
- **Accessibility team** reads the accessibility spec as an audit checklist

The component contract is the single document that all stakeholders reference.

---

## 2.10 Exercises

### Exercise 1: Specify a DataTable Component

Write a complete component contract for a DataTable that supports:
- Column definitions (sortable, filterable, resizable)
- Pagination (client-side and server-side)
- Row selection (single and multi-select)
- Expandable rows
- Inline editing
- Column visibility toggling
- Export to CSV

Your contract must include: props, state, behavior, keyboard interactions, accessibility, and edge cases. Do NOT write any implementation — only the contract.

### Exercise 2: Specify a NotificationCenter Component

Write a component contract for a NotificationCenter (the bell icon + dropdown that shows notifications):
- Shows unread count badge
- Dropdown lists notifications (grouped by date)
- Support for different notification types (info, warning, error, success)
- Mark individual or all as read
- Click notification to navigate
- Real-time updates (new notifications appear without refresh)
- Empty state
- Loading state

### Exercise 3: Spec-to-Implementation Challenge

Take the SearchBar contract from section 2.7 and hand it to an AI assistant (Claude, GPT, Gemini). Then take only the natural-language description from section 2.7 Step 1 (the 8-item requirements list) and hand THAT to the same AI.

Compare the two implementations:
- Which has more complete keyboard navigation?
- Which handles more edge cases?
- Which has better accessibility?
- Which required more follow-up prompts to get right?

Document your findings. This exercise will demonstrate the concrete value of component contracts in AI-assisted development.

### Exercise 4: Refactor a God Component

Here is an over-specified component. Refactor it into a proper contract:

```typescript
// Refactor this into a proper component contract
interface BadModalProps {
  show: boolean;
  title: string;
  message: string;
  okText: string;
  cancelText: string;
  okColor: string;
  cancelColor: string;
  width: string;
  height: string;
  borderRadius: string;
  overlayColor: string;
  overlayOpacity: number;
  animationDuration: number;
  zIndex: number;
  onOk: () => void;
  onCancel: () => void;
  className: string;
  style: React.CSSProperties;
  headerClassName: string;
  bodyClassName: string;
  footerClassName: string;
}
```

Separate behavior props from visual props. Add missing behavioral concerns (keyboard handling, focus trap, accessibility). Remove visual props that belong in a design system.

---

## 2.11 Key Takeaways

1. **A component contract** defines props, state, and side effects — NOT visual styling.

2. **Separate behavior specs from visual specs.** They change at different rates, are owned by different people, and serve different purposes.

3. **Props are contracts** between parent and child. Design them like APIs: with clear types, sensible defaults, and explicit documentation.

4. **State machines** are the most rigorous way to specify component behavior. They enumerate all states, all events, and all valid transitions.

5. **Side effects must be declared explicitly.** API calls, timers, event listeners, analytics — if the component touches the outside world, it belongs in the contract.

6. **Major design systems** (Material, React Aria, Radix) all separate behavioral contracts from visual implementation. This is not accidental — it is the pattern that scales.

7. **Over-specifying visuals** and **under-specifying behavior** are the two most common anti-patterns. Err on the side of more behavioral detail.

8. **Component contracts are the best AI prompts.** A complete contract produces dramatically better AI-generated implementations than prose descriptions.

---

## Looking Ahead

In the next chapter, we move from individual components to the connections between them: **API Blueprinting**. You will learn how to specify the request/response contracts that connect your frontend components to your backend services. If component contracts are the organs, API blueprints are the circulatory system.

---

*End of Chapter 2 — The Component Contract*

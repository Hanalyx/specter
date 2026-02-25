# Chapter 3: The Context Window Strategy

## MODULE 03 — Validation & The Feedback Loop (Intermediate Level)

---

## Lecture Preamble

*The professor draws a large rectangle on the whiteboard and labels it "Context Window." Inside the rectangle, they begin filling it with smaller boxes: "System Prompt," "Spec A," "Spec B," "Spec C," "Code File 1," "Code File 2," "Conversation History." The rectangle fills up fast. The professor circles the remaining empty space -- a thin sliver at the bottom.*

So here's the thing nobody tells you when you start building real applications with AI assistance.

Your spec is 4,000 lines. Your codebase is 50,000 lines. Your conversation history is growing with every message. And the AI's context window -- the total amount of information it can "see" at once -- is finite.

*Points to the thin sliver of empty space on the whiteboard.*

That's how much room the AI has left to actually *think* about your request. Everything else is just... context. And here's the counterintuitive part: **more context often makes the AI perform worse, not better.**

Today we're going to learn how to be strategic about what goes into that window. We're going to build a "Registry of Specs" that lets you give the AI exactly the context it needs -- no more, no less. We're going to learn why Google invested heavily in giving Gemini a million-token context window, and why that doesn't solve the problem the way you might think. And we're going to learn the techniques that Anthropic's Claude Code and other AI development tools use to manage context in real-world codebases.

This chapter is about the engineering discipline of context management. It's not glamorous. But it's the difference between an AI that produces good work and an AI that drowns in irrelevant information.

Let's dive in.

---

## 3.1 Understanding Context Windows

### What Is a Context Window?

A context window is the total amount of text (measured in tokens) that a large language model can process in a single interaction. Think of it as the AI's working memory -- everything it can "see" at once.

A token is roughly 3/4 of a word in English, or about 4 characters. So:

| Context Window | Approximate Words | Approximate Pages |
|---------------|-------------------|-------------------|
| 8K tokens     | ~6,000 words      | ~12 pages         |
| 32K tokens    | ~24,000 words     | ~48 pages         |
| 128K tokens   | ~96,000 words     | ~192 pages        |
| 200K tokens   | ~150,000 words    | ~300 pages        |
| 1M tokens     | ~750,000 words    | ~1,500 pages      |
| 2M tokens     | ~1,500,000 words  | ~3,000 pages      |

### The Current Landscape (2025-2026)

| Provider | Model | Context Window | Notes |
|----------|-------|---------------|-------|
| Anthropic | Claude (Opus, Sonnet) | 200K tokens | Strong recall across the full window |
| Google | Gemini 1.5 Pro/2.0 | 1M-2M tokens | Largest publicly available context |
| OpenAI | GPT-4o/o1/o3 | 128K tokens | Varying by model variant |
| Meta | Llama 3.x | 128K tokens | Open-source, community-extended |
| Mistral | Mistral Large | 128K tokens | European-developed model |

### What Actually Goes Into the Context Window

When you interact with an AI coding assistant, the context window is consumed by:

```
+--------------------------------------------------+
|              CONTEXT WINDOW (200K)                |
|                                                   |
|  +--------------------------------------------+  |
|  | System Prompt / Instructions     (~2-5K)   |  |
|  +--------------------------------------------+  |
|  | Project Configuration            (~1-3K)   |  |
|  | (.cursorrules, CLAUDE.md, etc.)             |  |
|  +--------------------------------------------+  |
|  | Referenced Files                 (~5-50K)   |  |
|  | (specs, code files, configs)                |  |
|  +--------------------------------------------+  |
|  | Conversation History             (~5-100K)  |  |
|  | (all previous messages in session)          |  |
|  +--------------------------------------------+  |
|  | Current Request                  (~0.5-5K)  |  |
|  +--------------------------------------------+  |
|  | REMAINING SPACE FOR REASONING    (varies)   |  |
|  | AND RESPONSE GENERATION                     |  |
|  +--------------------------------------------+  |
+--------------------------------------------------+
```

> **Professor's Aside:** Here's a subtle but critical point. The "remaining space" isn't just for the response text. It's also the space the model uses for *reasoning* -- for figuring out what code to write, how to structure it, which patterns to apply. When you fill the context window to the brim with reference material, you're literally squeezing the AI's ability to think. It's like taking an open-book exam where you brought so many reference books that you can't spread out your scratch paper.

---

## 3.2 Why More Context Isn't Always Better

### The "Needle in a Haystack" Problem

This is one of the most well-studied phenomena in LLM research. The basic finding: as context length increases, the model's ability to find and use specific pieces of information *decreases* -- sometimes dramatically.

The metaphor: imagine you need to find a specific sentence (the needle) in a 500-page document (the haystack). Even if the model can technically "see" all 500 pages, its attention mechanism may not focus on the right sentence when it matters.

Research from multiple labs has consistently shown:

1. **Information in the middle of the context is recalled less reliably** than information at the beginning or end. This is called the "lost in the middle" effect.

2. **Relevant information surrounded by irrelevant information is harder to use** than the same information presented in a focused context.

3. **The model may blend or confuse similar-but-different pieces of information** when too many are present simultaneously.

### Practical Implications for SDD

This means that if you dump your entire spec (all 4,000 lines) into the context window along with 20 code files and a long conversation history, the AI may:

- **Miss critical constraints** buried in the middle of the spec
- **Confuse requirements from Spec A with requirements from Spec B**
- **Apply patterns from one part of the codebase to an inappropriate context**
- **Generate code that satisfies some spec requirements while violating others**

### The Google Gemini Paradox

Google's Gemini models offer context windows of 1 million tokens or more -- enough to hold an entire codebase. This is impressive engineering, and it's genuinely useful for certain tasks (like analyzing an entire repository or answering questions across many files).

But here's the paradox: **having a huge context window doesn't mean you should use all of it.**

Google's own documentation for Gemini Code Assist recommends providing focused, relevant context rather than dumping entire repositories. Their internal research has shown that targeted context selection produces better results than brute-force context inclusion, even when the model *can* handle the full input.

The analogy: a library has millions of books. You *could* read them all before writing your essay. But a skilled researcher selects the 5-10 most relevant sources and reads those carefully. That's what we need to do with AI context.

### Anthropic's Approach: Right-Sized Context

Anthropic's Claude Code takes an interesting approach. Rather than trying to load everything into context, it:

1. **Reads specific files on demand** -- using tools to look at exactly the files needed
2. **Uses project-level configuration** (CLAUDE.md) as persistent lightweight context
3. **Supports a file search and retrieval workflow** that lets the AI discover relevant files rather than preloading them
4. **Maintains conversation context** but encourages focused, task-oriented interactions

This is a "pull" model of context management: the AI pulls in what it needs when it needs it, rather than having everything pushed into its context upfront.

---

## 3.3 The Registry Pattern: A Master Index of Specs

### What Is a Spec Registry?

A spec registry is a master index file that catalogs all the specifications in your project. It's the table of contents for your spec-driven development system. The AI reads the registry to understand what specs exist and which ones are relevant to the current task.

### The Registry Structure

```markdown
# SPEC_REGISTRY.md

## Project: SaaS Dashboard v2

## Last Updated: 2026-02-20

## How to Use This Registry
This file lists all active specifications for the project.
When implementing a feature, find the relevant spec(s) below and
read them before writing any code.

---

## System-Level Specs

| ID | Name | Path | Status | Last Updated |
|----|------|------|--------|--------------|
| SYS-001 | Architecture Overview | `/specs/system/architecture.md` | Active | 2026-01-15 |
| SYS-002 | Authentication & Authorization | `/specs/system/auth.md` | Active | 2026-02-01 |
| SYS-003 | Database Schema | `/specs/system/database.md` | Active | 2026-02-10 |
| SYS-004 | API Design Standards | `/specs/system/api-standards.md` | Active | 2026-01-20 |
| SYS-005 | Error Handling Strategy | `/specs/system/error-handling.md` | Active | 2026-01-25 |
| SYS-006 | Security Requirements | `/specs/system/security.md` | Active | 2026-02-05 |

## Module-Level Specs

### User Management Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| USR-001 | User Registration | `/specs/modules/user/registration.md` | Active | SYS-002, SYS-003 |
| USR-002 | User Profile | `/specs/modules/user/profile.md` | Active | SYS-002, SYS-003 |
| USR-003 | User Settings | `/specs/modules/user/settings.md` | Active | SYS-002 |
| USR-004 | Team Management | `/specs/modules/user/teams.md` | Draft | SYS-002, SYS-003 |

### Billing Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| BIL-001 | Subscription Plans | `/specs/modules/billing/plans.md` | Active | SYS-003, SYS-004 |
| BIL-002 | Payment Processing | `/specs/modules/billing/payments.md` | Active | SYS-003, SYS-006 |
| BIL-003 | Invoice Generation | `/specs/modules/billing/invoices.md` | Active | SYS-003, BIL-001 |
| BIL-004 | Usage Metering | `/specs/modules/billing/metering.md` | Draft | SYS-003, BIL-001 |

### Dashboard Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| DSH-001 | Dashboard Layout | `/specs/modules/dashboard/layout.md` | Active | SYS-001 |
| DSH-002 | Analytics Widgets | `/specs/modules/dashboard/widgets.md` | Active | SYS-001, SYS-004 |
| DSH-003 | Real-time Updates | `/specs/modules/dashboard/realtime.md` | Active | SYS-001, SYS-004 |
| DSH-004 | Export & Reporting | `/specs/modules/dashboard/export.md` | Draft | SYS-004 |

### Notification Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| NTF-001 | Email Notifications | `/specs/modules/notifications/email.md` | Active | SYS-002 |
| NTF-002 | In-App Notifications | `/specs/modules/notifications/in-app.md` | Active | SYS-002, SYS-004 |
| NTF-003 | Webhook Integrations | `/specs/modules/notifications/webhooks.md` | Draft | SYS-004, SYS-006 |

## Component-Level Specs

| ID | Name | Path | Status | Parent |
|----|------|------|--------|--------|
| CMP-001 | DataTable Component | `/specs/components/data-table.md` | Active | DSH-001 |
| CMP-002 | Modal System | `/specs/components/modal.md` | Active | SYS-001 |
| CMP-003 | Form Builder | `/specs/components/form-builder.md` | Active | SYS-001 |
| CMP-004 | Chart Components | `/specs/components/charts.md` | Active | DSH-002 |
| CMP-005 | File Upload | `/specs/components/file-upload.md` | Active | SYS-006 |

## Cross-Cutting Specs

| ID | Name | Path | Status | Affects |
|----|------|------|--------|---------|
| XCT-001 | Styling Standards (Tailwind) | `/specs/cross-cutting/styling.md` | Active | All modules |
| XCT-002 | State Management (Zustand) | `/specs/cross-cutting/state.md` | Active | All modules |
| XCT-003 | Testing Standards | `/specs/cross-cutting/testing.md` | Active | All modules |
| XCT-004 | Accessibility (WCAG 2.1 AA) | `/specs/cross-cutting/accessibility.md` | Active | All modules |
| XCT-005 | Internationalization | `/specs/cross-cutting/i18n.md` | Draft | All modules |
| XCT-006 | Performance Budgets | `/specs/cross-cutting/performance.md` | Active | All modules |

---

## Dependency Graph

```
SYS-001 (Architecture)
  ├── DSH-001 (Dashboard Layout)
  │   ├── CMP-001 (DataTable)
  │   └── DSH-002 (Analytics Widgets)
  │       └── CMP-004 (Charts)
  ├── CMP-002 (Modal System)
  └── CMP-003 (Form Builder)

SYS-002 (Auth)
  ├── USR-001 (Registration)
  ├── USR-002 (Profile)
  ├── USR-003 (Settings)
  ├── USR-004 (Teams)
  ├── NTF-001 (Email)
  └── NTF-002 (In-App)

SYS-003 (Database)
  ├── USR-001 (Registration)
  ├── USR-002 (Profile)
  ├── BIL-001 (Plans)
  ├── BIL-002 (Payments)
  └── BIL-003 (Invoices)
```
```

### Why the Registry Pattern Works

The registry solves three problems simultaneously:

1. **Discovery:** The AI (or a new developer) can quickly find which spec is relevant for any given task. Instead of searching through dozens of files, they read the registry.

2. **Scoping:** The registry includes dependency information, so the AI knows not just *which* spec to read but *what other specs are related*. When implementing USR-001 (User Registration), the AI can see that it also needs SYS-002 (Auth) and SYS-003 (Database).

3. **Minimal Context:** The registry itself is lightweight -- maybe 2-3K tokens. It provides a map of the territory without actually loading all the territory into context.

---

## 3.4 Hierarchical Specs: System, Module, Component

### The Three-Level Hierarchy

The most effective way to organize specs for context management is a three-level hierarchy:

```
Level 1: SYSTEM-LEVEL SPECS
  - Architecture decisions that affect everything
  - Technology choices, patterns, standards
  - Cross-cutting concerns (auth, logging, error handling)
  - ~5-10 specs total
  - Read when: starting a new module or making architectural decisions

Level 2: MODULE-LEVEL SPECS
  - Feature-area specifications
  - Business logic and domain rules
  - API contracts and data models
  - ~15-30 specs total
  - Read when: implementing a feature within a specific module

Level 3: COMPONENT-LEVEL SPECS
  - Individual UI component behavior
  - Specific function or service specifications
  - Detailed interaction patterns
  - ~30-100 specs total
  - Read when: implementing a specific component or function
```

### How the Hierarchy Reduces Context

Without hierarchy, implementing a single feature might require reading 20+ spec files. With hierarchy:

```
Task: "Implement the DataTable component for the dashboard"

Without hierarchy:
  Load: ALL specs (~80 files, ~200K tokens) -- context window full

With hierarchy:
  Load: SPEC_REGISTRY.md              (~2K tokens) -- find relevant specs
  Load: XCT-001 (Styling Standards)   (~3K tokens) -- cross-cutting
  Load: XCT-004 (Accessibility)       (~2K tokens) -- cross-cutting
  Load: DSH-001 (Dashboard Layout)    (~4K tokens) -- parent module
  Load: CMP-001 (DataTable Component) (~5K tokens) -- specific component
  Total: ~16K tokens                  -- plenty of room for reasoning
```

That's a 12x reduction in context usage while providing *all* the relevant information.

### Spec Summarization

Each spec should include a summary section at the top that captures the key constraints in 5-10 lines. This allows for a "two-pass" approach:

**Pass 1: Summaries only** -- Load the registry and summaries of potentially relevant specs (~500 tokens each). Determine which specs are truly needed.

**Pass 2: Full specs** -- Load the full text of only the specs identified in Pass 1.

```markdown
# Spec: CMP-001 -- DataTable Component

## Summary
<!-- 5-10 lines capturing the essential constraints -->
- Renders tabular data with sorting, filtering, and pagination
- Uses TanStack Table (v8) for table logic
- Server-side pagination with cursor-based API
- Columns are configurable via a schema object
- Supports row selection (single and multi-select)
- Must meet WCAG 2.1 AA accessibility requirements
- All styling via Tailwind utility classes
- Responsive: stacks to card layout below 768px

## Full Specification
[... detailed spec follows, ~200-300 lines ...]
```

---

## 3.5 How to Decide What Context the AI Needs

### The Context Decision Framework

When you're about to ask the AI to do something, run through this decision tree:

```
TASK: What am I asking the AI to do?
  |
  +-- Is it a NEW feature?
  |     |
  |     +-- Load: Registry, relevant module spec, relevant system specs
  |     +-- Load: Cross-cutting specs that apply (styling, testing, etc.)
  |     +-- Load: Similar existing implementations for pattern reference
  |
  +-- Is it a BUG FIX?
  |     |
  |     +-- Load: The specific spec section that defines correct behavior
  |     +-- Load: The existing implementation file
  |     +-- Load: The existing test file
  |     +-- DO NOT load: Unrelated specs or code
  |
  +-- Is it a REFACTORING?
  |     |
  |     +-- Load: Architecture spec (system-level)
  |     +-- Load: The files being refactored
  |     +-- Load: Tests for those files
  |     +-- DO NOT load: Business logic specs (behavior shouldn't change)
  |
  +-- Is it a TEST?
        |
        +-- Load: The relevant spec (this is what we're testing against)
        +-- Load: The implementation (this is what we're testing)
        +-- Load: Testing standards spec (cross-cutting)
        +-- DO NOT load: Other module specs
```

### The "Just Enough Context" Principle

This is the governing principle of context management in SDD:

> **Provide the AI with exactly the information it needs to complete the current task correctly -- no more, no less.**

"No more" because excess context degrades performance.
"No less" because missing context leads to incorrect output.

Here's a practical heuristic:

| Context Type | Include? | Size Budget |
|-------------|----------|-------------|
| System prompt / instructions | Always | 2-5K tokens |
| Project configuration (.cursorrules) | Always | 1-3K tokens |
| Relevant spec(s) | Always | 3-15K tokens |
| Existing related code | Usually | 5-20K tokens |
| Existing tests | For bug fixes & refactors | 3-10K tokens |
| Conversation history | Automatic | Varies |
| Unrelated specs | Never | 0 tokens |
| Entire codebase | Never | 0 tokens |

**Target: Keep total context under 50% of the window capacity.** This leaves room for the AI to reason and generate output.

> **Professor's Aside:** I've seen developers paste an entire 100-file codebase into Claude's 200K context window and then wonder why the output quality dropped. The model was spending all its "attention budget" on 100 files when it only needed 3. It's like asking someone to find a typo in a sentence while simultaneously reading them the entire encyclopedia. Give the AI focus. It will reward you with precision.

---

## 3.6 Techniques for Context Management

### Technique 1: Spec Summarization

Create condensed versions of your specs that capture the essential constraints in minimal tokens:

```typescript
// tools/spec-summarizer.ts

/**
 * Generates condensed summaries of spec files for context optimization.
 * The summary captures key constraints while reducing token count by ~80%.
 */

import { readFileSync, writeFileSync } from 'fs';

interface SpecSummary {
  id: string;
  title: string;
  path: string;
  constraints: string[];
  dependencies: string[];
  tokenEstimate: number;
}

function summarizeSpec(specPath: string): SpecSummary {
  const content = readFileSync(specPath, 'utf-8');
  const lines = content.split('\n');

  const summary: SpecSummary = {
    id: '',
    title: '',
    path: specPath,
    constraints: [],
    dependencies: [],
    tokenEstimate: 0,
  };

  for (const line of lines) {
    // Extract ID
    const idMatch = line.match(/^#\s+Spec:\s+(\S+)/);
    if (idMatch) summary.id = idMatch[1];

    // Extract title
    const titleMatch = line.match(/^#\s+(.+)/);
    if (titleMatch && !summary.title) summary.title = titleMatch[1];

    // Extract SHALL/MUST requirements (these are the constraints)
    if (/\b(SHALL|MUST)\b/i.test(line) && !line.startsWith('#')) {
      summary.constraints.push(line.trim().replace(/^\d+\.\s*/, ''));
    }

    // Extract dependencies
    const depMatch = line.match(/Dependencies?:\s*(.+)/i);
    if (depMatch) {
      summary.dependencies = depMatch[1].split(',').map(d => d.trim());
    }
  }

  // Estimate tokens (rough: 1 token per 4 characters)
  const summaryText = JSON.stringify(summary);
  summary.tokenEstimate = Math.ceil(summaryText.length / 4);

  return summary;
}

function generateContextBundle(
  specPaths: string[],
  maxTokens: number = 15000
): string {
  const summaries = specPaths.map(summarizeSpec);
  let bundle = '# Relevant Spec Summaries\n\n';
  let tokenCount = 0;

  for (const summary of summaries) {
    const section = [
      `## ${summary.id}: ${summary.title}`,
      `Path: ${summary.path}`,
      `Dependencies: ${summary.dependencies.join(', ') || 'None'}`,
      '',
      '### Key Constraints:',
      ...summary.constraints.map((c, i) => `${i + 1}. ${c}`),
      '',
      '---',
      '',
    ].join('\n');

    const sectionTokens = Math.ceil(section.length / 4);

    if (tokenCount + sectionTokens > maxTokens) {
      bundle += `\n> Note: ${summaries.length - summaries.indexOf(summary)} specs omitted due to context budget.\n`;
      bundle += `> Load full specs individually if needed.\n`;
      break;
    }

    bundle += section;
    tokenCount += sectionTokens;
  }

  bundle += `\n<!-- Total estimated tokens: ${tokenCount} -->\n`;
  return bundle;
}

export { summarizeSpec, generateContextBundle };
```

### Technique 2: Dependency Graphs for Spec Loading

```typescript
// tools/spec-dependency-graph.ts

/**
 * Builds a dependency graph from the spec registry.
 * Given a target spec, returns the minimal set of specs
 * needed for full context.
 */

interface SpecNode {
  id: string;
  path: string;
  dependencies: string[];
}

class SpecDependencyGraph {
  private nodes: Map<string, SpecNode> = new Map();

  addSpec(id: string, path: string, dependencies: string[]): void {
    this.nodes.set(id, { id, path, dependencies });
  }

  /**
   * Returns the minimal set of specs needed for a given task.
   * Walks the dependency graph from the target spec(s) upward.
   */
  resolve(targetIds: string[]): SpecNode[] {
    const resolved = new Map<string, SpecNode>();
    const queue = [...targetIds];

    while (queue.length > 0) {
      const currentId = queue.shift()!;

      if (resolved.has(currentId)) continue;

      const node = this.nodes.get(currentId);
      if (!node) {
        console.warn(`Warning: Spec ${currentId} not found in registry.`);
        continue;
      }

      resolved.set(currentId, node);

      // Add dependencies to the queue
      for (const depId of node.dependencies) {
        if (!resolved.has(depId)) {
          queue.push(depId);
        }
      }
    }

    // Return in dependency order (dependencies first)
    return this.topologicalSort(resolved);
  }

  /**
   * Returns a topologically sorted list of specs.
   * Dependencies come before their dependents.
   */
  private topologicalSort(nodes: Map<string, SpecNode>): SpecNode[] {
    const sorted: SpecNode[] = [];
    const visited = new Set<string>();
    const visiting = new Set<string>();

    const visit = (id: string) => {
      if (visited.has(id)) return;
      if (visiting.has(id)) {
        throw new Error(`Circular dependency detected involving: ${id}`);
      }

      visiting.add(id);

      const node = nodes.get(id);
      if (node) {
        for (const dep of node.dependencies) {
          if (nodes.has(dep)) {
            visit(dep);
          }
        }
        visited.add(id);
        visiting.delete(id);
        sorted.push(node);
      }
    };

    for (const id of nodes.keys()) {
      visit(id);
    }

    return sorted;
  }

  /**
   * Suggests which specs to load for a given file path.
   * Uses heuristics based on file location and naming.
   */
  suggestSpecs(filePath: string): string[] {
    const suggestions: string[] = [];

    // Always include cross-cutting specs
    for (const [id] of this.nodes) {
      if (id.startsWith('XCT-')) {
        suggestions.push(id);
      }
    }

    // Match based on file path patterns
    if (filePath.includes('/components/')) {
      // Component files need the component spec + parent module spec
      const componentName = filePath.split('/').pop()?.replace(/\.(tsx?|jsx?)$/, '');
      for (const [id, node] of this.nodes) {
        if (id.startsWith('CMP-') && node.path.includes(componentName?.toLowerCase() || '')) {
          suggestions.push(id);
        }
      }
    }

    if (filePath.includes('/services/') || filePath.includes('/api/')) {
      // Service/API files need the module spec + system specs
      for (const [id] of this.nodes) {
        if (id.startsWith('SYS-')) {
          suggestions.push(id);
        }
      }
    }

    return [...new Set(suggestions)];
  }
}

// Build the graph from registry
function buildGraphFromRegistry(registryPath: string): SpecDependencyGraph {
  const content = readFileSync(registryPath, 'utf-8');
  const graph = new SpecDependencyGraph();

  // Parse the registry markdown table
  const lines = content.split('\n');
  for (const line of lines) {
    // Match table rows: | ID | Name | Path | Status | Dependencies |
    const match = line.match(
      /\|\s*(\S+)\s*\|\s*([^|]+)\s*\|\s*`([^`]+)`\s*\|\s*(\w+)\s*\|\s*([^|]*)\s*\|/
    );
    if (match) {
      const [, id, , path, status, depsStr] = match;
      if (status === 'Active' || status === 'Draft') {
        const deps = depsStr.trim()
          ? depsStr.split(',').map(d => d.trim())
          : [];
        graph.addSpec(id, path, deps);
      }
    }
  }

  return graph;
}

import { readFileSync } from 'fs';

export { SpecDependencyGraph, buildGraphFromRegistry };

// Example usage:
// const graph = buildGraphFromRegistry('SPEC_REGISTRY.md');
// const needed = graph.resolve(['USR-001']); // Returns USR-001 + SYS-002 + SYS-003
// console.log(needed.map(n => `${n.id}: ${n.path}`));
```

### Technique 3: Incremental Context Loading

Instead of loading all context at once, load it incrementally as the AI needs it:

```markdown
# Pattern: Incremental Context Loading

## Phase 1: Registry Only
Load SPEC_REGISTRY.md. Ask the AI to identify which specs are relevant.

## Phase 2: Summaries
Load summaries of the identified specs. Ask the AI to confirm
which ones it needs in full.

## Phase 3: Full Specs
Load the full text of only the confirmed specs.

## Phase 4: Implementation Context
Load existing code files that are directly related to the task.
```

This pattern works particularly well with AI development tools that support tool-based file reading (like Claude Code), where the AI can request files as needed rather than having them pre-loaded.

### Technique 4: Context Budgeting

Assign a token budget to each category of context and enforce it:

```typescript
// tools/context-budget.ts

interface ContextBudget {
  systemPrompt: number;
  projectConfig: number;
  specs: number;
  existingCode: number;
  conversationHistory: number;
  reserve: number;  // Reserved for AI reasoning and response
}

function createBudget(totalTokens: number): ContextBudget {
  return {
    systemPrompt: Math.floor(totalTokens * 0.02),      // 2%
    projectConfig: Math.floor(totalTokens * 0.02),      // 2%
    specs: Math.floor(totalTokens * 0.15),              // 15%
    existingCode: Math.floor(totalTokens * 0.15),       // 15%
    conversationHistory: Math.floor(totalTokens * 0.16),// 16%
    reserve: Math.floor(totalTokens * 0.50),            // 50% reserved
  };
}

// Example: Claude's 200K token window
const claudeBudget = createBudget(200_000);
// {
//   systemPrompt: 4000,
//   projectConfig: 4000,
//   specs: 30000,
//   existingCode: 30000,
//   conversationHistory: 32000,
//   reserve: 100000
// }

// Example: GPT-4's 128K token window
const gptBudget = createBudget(128_000);
// {
//   systemPrompt: 2560,
//   projectConfig: 2560,
//   specs: 19200,
//   existingCode: 19200,
//   conversationHistory: 20480,
//   reserve: 64000
// }

function estimateTokens(text: string): number {
  // Rough estimate: 1 token per 4 characters
  return Math.ceil(text.length / 4);
}

function checkBudget(
  budget: ContextBudget,
  category: keyof ContextBudget,
  content: string
): { fits: boolean; tokens: number; remaining: number } {
  const tokens = estimateTokens(content);
  const remaining = budget[category] - tokens;

  return {
    fits: remaining >= 0,
    tokens,
    remaining: Math.max(0, remaining),
  };
}
```

---

## 3.7 How AI Development Tools Handle Large Codebases

### Anthropic's Claude Code

Claude Code takes a pragmatic approach to context management:

1. **CLAUDE.md as persistent context:** The `CLAUDE.md` file at the project root is loaded automatically into every interaction. This is your "always-on" context -- it should contain the most critical constraints, not everything.

2. **Tool-based file reading:** Claude Code uses tools to read files on demand. Rather than pre-loading the entire codebase, it reads specific files when it needs them. This is the "pull" model we discussed earlier.

3. **Hierarchical CLAUDE.md files:** You can place `CLAUDE.md` files in subdirectories. When working in a specific directory, Claude Code loads the root `CLAUDE.md` plus the local one. This naturally implements the hierarchical spec pattern.

```
project/
  CLAUDE.md              # System-level context (always loaded)
  src/
    modules/
      billing/
        CLAUDE.md        # Billing module context (loaded when working here)
      user/
        CLAUDE.md        # User module context (loaded when working here)
```

4. **Conversation management:** Claude Code encourages short, focused conversations. Each conversation has its own context, and the recommendation is to start new conversations for new tasks rather than continuing indefinitely.

### Google's Gemini Code Assist

Google's approach leverages Gemini's massive context window differently:

1. **Repository indexing:** Gemini Code Assist can index an entire repository and build a semantic understanding of the codebase. This index is separate from the context window -- it's a retrieval system that helps the AI find relevant code.

2. **Smart context selection:** Based on the current file and task, Gemini selects relevant code snippets from the repository to include in context. This is automated context management -- the tool decides what's relevant.

3. **Project-level configuration:** Similar to `.cursorrules`, Gemini Code Assist supports project-level instructions that provide persistent context.

### Cursor IDE

Cursor IDE has pioneered several context management patterns:

1. **`.cursorrules` file:** A project-level configuration that provides persistent spec constraints to the AI.

2. **`@` references:** Cursor allows you to explicitly reference files, folders, or documentation in your prompts using `@` syntax (e.g., `@file:src/components/DataTable.tsx`). This gives you precise control over what context the AI sees.

3. **Codebase indexing:** Cursor builds a semantic index of your codebase and uses it to automatically include relevant context when you ask questions or request code generation.

4. **Docs integration:** Cursor can index external documentation (framework docs, library docs) and include relevant sections in context automatically.

### Key Patterns Across Tools

Despite their different approaches, all these tools share common patterns:

| Pattern | Claude Code | Gemini Code Assist | Cursor |
|---------|-------------|-------------------|--------|
| Persistent config file | CLAUDE.md | Project config | .cursorrules |
| On-demand file reading | Yes (tools) | Yes (indexing) | Yes (@ refs) |
| Automatic context selection | Minimal | Aggressive | Moderate |
| Repository indexing | No | Yes | Yes |
| Hierarchical config | Yes | Partial | Partial |

---

## 3.8 The Spec-Aware Prompt: Putting It All Together

### A Complete Prompt Structure for Context-Managed SDD

When you ask an AI to implement a feature in a context-managed SDD workflow, your prompt (and context) should follow this structure:

```markdown
## Context

### Project Configuration
[Contents of .cursorrules or CLAUDE.md -- always included]

### Spec Registry (Summary)
[The relevant portion of SPEC_REGISTRY.md]

### Relevant Specs
[Full text of the 2-4 specs that apply to this task]

### Existing Code Context
[The existing files that the AI needs to reference -- files it will modify,
files it should follow the pattern of, test files to update]

## Task

Implement [specific feature] according to the spec [SPEC-ID].

### Constraints
- Follow all patterns defined in the project configuration
- Write tests first, then implementation (per Module 3, Chapter 1)
- Ensure all spec requirements have test coverage
- Do not introduce any dependencies not listed in the approved deps spec

### Deliverables
1. Test file: `src/[path]/[feature].test.ts`
2. Implementation file: `src/[path]/[feature].ts`
3. Spec coverage annotation: which spec lines are covered by which tests
```

### Example: Implementing a Feature with Context Management

Let's walk through a real example. The task: implement the `calculateInvoice` function for the billing module.

**Step 1: Consult the Registry**

```
AI reads SPEC_REGISTRY.md and identifies:
  - BIL-003 (Invoice Generation) -- the primary spec
  - BIL-001 (Subscription Plans) -- dependency (invoice needs plan info)
  - SYS-003 (Database Schema) -- dependency (data model)
  - SYS-004 (API Standards) -- dependency (response format)
  - XCT-001 (Styling Standards) -- cross-cutting (if there's a UI component)
  - XCT-003 (Testing Standards) -- cross-cutting (always relevant)
```

**Step 2: Load Relevant Specs (Summaries First)**

```
Load summaries of: BIL-003, BIL-001, SYS-003, SYS-004, XCT-003
Estimated tokens: 5 specs x ~500 tokens each = ~2,500 tokens
```

**Step 3: Identify Which Full Specs Are Needed**

```
After reading summaries, determine:
  - BIL-003: Need full spec (primary task)
  - BIL-001: Need full spec (invoice references plan data)
  - SYS-003: Only need the Invoice table schema (partial load)
  - SYS-004: Only need the response format section (partial load)
  - XCT-003: Already in .cursorrules, skip
```

**Step 4: Load Full Specs + Code Context**

```
Load:
  - BIL-003 full spec:            ~4K tokens
  - BIL-001 full spec:            ~3K tokens
  - SYS-003 (Invoice schema only): ~1K tokens
  - SYS-004 (response format only): ~1K tokens
  - Existing billing service:       ~3K tokens
  - Existing billing tests:         ~2K tokens
  - .cursorrules:                   ~2K tokens

Total context: ~16K tokens out of 200K available
```

That's 8% of Claude's context window. The remaining 92% is available for reasoning and response generation. This is how you get quality output from an AI -- by giving it focus.

---

## 3.9 Managing Spec Dependencies

### When Spec A Depends on Spec B

In any non-trivial project, specs will have dependencies on each other. Managing these dependencies is crucial for both human understanding and AI context loading.

### Types of Spec Dependencies

**1. Data Dependencies**

Spec A references data structures defined in Spec B.

```markdown
# BIL-003: Invoice Generation

## Data Model
The invoice SHALL contain the following fields:
- planId: Reference to a SubscriptionPlan (see BIL-001, Section 3.2)
- userId: Reference to a User (see USR-001, Section 2.1)
```

**2. Behavioral Dependencies**

Spec A's behavior depends on behavior defined in Spec B.

```markdown
# NTF-001: Email Notifications

## Trigger Conditions
An email notification SHALL be sent when:
- A user registers (USR-001, event: USER_REGISTERED)
- A payment fails (BIL-002, event: PAYMENT_FAILED)
- A subscription renews (BIL-001, event: SUBSCRIPTION_RENEWED)
```

**3. Constraint Dependencies**

Spec A inherits constraints from Spec B.

```markdown
# DSH-002: Analytics Widgets

## API Constraints
All widget data endpoints SHALL follow the API standards defined in SYS-004,
including:
- Pagination format (SYS-004, Section 4)
- Error response format (SYS-004, Section 6)
- Authentication requirements (SYS-002, Section 2)
```

### Tracking Dependencies in the Registry

The registry should maintain a dependency matrix:

```markdown
## Dependency Matrix

| Spec | Depends On | Depended On By |
|------|-----------|----------------|
| SYS-001 | (none) | DSH-001, CMP-002, CMP-003 |
| SYS-002 | (none) | USR-001, USR-002, USR-003, USR-004, NTF-001, NTF-002 |
| SYS-003 | (none) | USR-001, USR-002, BIL-001, BIL-002, BIL-003 |
| SYS-004 | (none) | BIL-001, BIL-002, BIL-003, DSH-002, DSH-003, DSH-004 |
| USR-001 | SYS-002, SYS-003 | NTF-001 |
| BIL-001 | SYS-003, SYS-004 | BIL-003, BIL-004 |
| BIL-003 | SYS-003, BIL-001 | (none) |
```

### Handling Circular Dependencies

Occasionally, specs may have circular dependencies. This is a design smell -- it usually means the specs need to be refactored.

```
BIL-001 (Plans) depends on USR-001 (Registration) -- plans need user data
USR-001 (Registration) depends on BIL-001 (Plans) -- registration needs plan data

SOLUTION: Extract the shared concern into a new spec
  - SYS-007 (User-Plan Binding) -- defines the relationship between users and plans
  - BIL-001 depends on SYS-007
  - USR-001 depends on SYS-007
  - No more circular dependency
```

### Automated Dependency Validation

```typescript
// tools/validate-spec-deps.ts

import { readFileSync, existsSync } from 'fs';

interface DepValidation {
  valid: boolean;
  errors: string[];
  warnings: string[];
  circularDeps: string[][];
}

function validateSpecDependencies(registryPath: string): DepValidation {
  const result: DepValidation = {
    valid: true,
    errors: [],
    warnings: [],
    circularDeps: [],
  };

  // Parse registry and build adjacency list
  const deps = new Map<string, string[]>();
  const paths = new Map<string, string>();

  const content = readFileSync(registryPath, 'utf-8');
  const lines = content.split('\n');

  for (const line of lines) {
    const match = line.match(
      /\|\s*(\S+)\s*\|[^|]+\|\s*`([^`]+)`\s*\|[^|]+\|\s*([^|]*)\s*\|/
    );
    if (match) {
      const [, id, path, depsStr] = match;
      const specDeps = depsStr.trim()
        ? depsStr.split(',').map(d => d.trim()).filter(Boolean)
        : [];
      deps.set(id, specDeps);
      paths.set(id, path);
    }
  }

  // Check 1: All dependencies reference existing specs
  for (const [id, specDeps] of deps) {
    for (const dep of specDeps) {
      if (!deps.has(dep)) {
        result.errors.push(
          `Spec ${id} depends on ${dep}, but ${dep} is not in the registry.`
        );
        result.valid = false;
      }
    }
  }

  // Check 2: All spec files exist on disk
  for (const [id, path] of paths) {
    if (!existsSync(path)) {
      result.warnings.push(
        `Spec ${id} references path ${path}, but file does not exist.`
      );
    }
  }

  // Check 3: Detect circular dependencies
  const visited = new Set<string>();
  const inStack = new Set<string>();

  function detectCycle(id: string, path: string[]): boolean {
    if (inStack.has(id)) {
      const cycleStart = path.indexOf(id);
      result.circularDeps.push(path.slice(cycleStart).concat(id));
      return true;
    }
    if (visited.has(id)) return false;

    visited.add(id);
    inStack.add(id);

    for (const dep of deps.get(id) || []) {
      if (detectCycle(dep, [...path, id])) {
        result.valid = false;
      }
    }

    inStack.delete(id);
    return false;
  }

  for (const id of deps.keys()) {
    if (!visited.has(id)) {
      detectCycle(id, []);
    }
  }

  if (result.circularDeps.length > 0) {
    for (const cycle of result.circularDeps) {
      result.errors.push(
        `Circular dependency detected: ${cycle.join(' -> ')}`
      );
    }
  }

  return result;
}

// Run validation
const result = validateSpecDependencies('SPEC_REGISTRY.md');

if (result.warnings.length > 0) {
  console.warn('\nWarnings:');
  for (const w of result.warnings) {
    console.warn(`  - ${w}`);
  }
}

if (result.errors.length > 0) {
  console.error('\nErrors:');
  for (const e of result.errors) {
    console.error(`  - ${e}`);
  }
  process.exit(1);
}

console.log('Spec dependency validation passed.');
```

---

## 3.10 Practical Exercise: Designing a Spec Registry

### The Task

Design a complete spec registry for a medium-sized SaaS application: a **Project Management Tool** (think a simplified version of Linear, Jira, or Asana).

### The Application Scope

The application has the following features:

- **Authentication:** Login, registration, SSO, password reset
- **Projects:** Create, archive, configure projects
- **Issues:** Create, assign, label, prioritize, close issues
- **Boards:** Kanban board view, list view, timeline view
- **Comments:** Comment on issues, mention users, attach files
- **Notifications:** Email, in-app, webhook notifications
- **Integrations:** GitHub, Slack, Discord integrations
- **Analytics:** Project velocity, burndown charts, team metrics
- **Admin:** User management, role-based access, billing

### Your Deliverable

Create a complete `SPEC_REGISTRY.md` file that includes:

1. **System-level specs** (5-8 specs covering architecture, auth, database, API standards, security, etc.)

2. **Module-level specs** (15-20 specs covering each feature area)

3. **Component-level specs** (10-15 specs for key UI components)

4. **Cross-cutting specs** (5-8 specs for styling, testing, accessibility, performance, etc.)

5. **A dependency graph** showing how specs relate to each other

6. **A dependency matrix** for quick reference

### Guidance

Here's a starting point to get you going:

```markdown
# SPEC_REGISTRY.md

## Project: ProjectFlow (Project Management Tool)

## System-Level Specs

| ID | Name | Path | Status | Last Updated |
|----|------|------|--------|--------------|
| SYS-001 | Architecture Overview | `/specs/system/architecture.md` | Active | 2026-02-01 |
| SYS-002 | Authentication & Authorization | `/specs/system/auth.md` | Active | 2026-02-01 |
| SYS-003 | Database Schema & Migrations | `/specs/system/database.md` | Active | 2026-02-01 |
| SYS-004 | REST API Design Standards | `/specs/system/api-standards.md` | Active | 2026-02-01 |
| ... | ... | ... | ... | ... |

## Module-Level Specs

### Project Management Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| PRJ-001 | Project CRUD | `/specs/modules/project/crud.md` | Active | SYS-002, SYS-003 |
| PRJ-002 | Project Configuration | `/specs/modules/project/config.md` | Active | SYS-002, PRJ-001 |
| ... | ... | ... | ... | ... |

### Issue Tracking Module
| ID | Name | Path | Status | Dependencies |
|----|------|------|--------|-------------|
| ISS-001 | Issue CRUD & Lifecycle | `/specs/modules/issues/lifecycle.md` | Active | SYS-002, SYS-003, PRJ-001 |
| ... | ... | ... | ... | ... |

[Continue for all modules...]
```

Complete the registry with all modules, ensuring:
- Every spec has a unique ID following the naming convention
- Dependencies are explicitly listed
- Status is marked (Active, Draft, Deprecated)
- The dependency graph is acyclic (no circular dependencies)

---

## 3.11 Advanced Topic: Context Window Strategies for Different Model Providers

### Adapting Your Strategy to the AI Model

Different models have different strengths and weaknesses when it comes to context utilization. Your context strategy should adapt accordingly.

### Strategy for Claude (200K context)

```
Claude's strengths:
  - Strong recall across the full 200K window
  - Good at following detailed instructions
  - Excellent at working with structured specs

Recommended strategy:
  - Budget: 30% for specs and code, 20% for conversation, 50% reserve
  - Load full specs (not just summaries) when within budget
  - Use CLAUDE.md for persistent constraints
  - Leverage tool-based file reading for on-demand context
  - Keep conversations focused on single tasks
```

### Strategy for Gemini (1M+ context)

```
Gemini's strengths:
  - Massive context window allows loading more material
  - Good at finding information across large contexts
  - Multimodal (can process images, diagrams alongside specs)

Recommended strategy:
  - Budget: 20% for specs and code, 10% for conversation, 70% reserve
  - Even with 1M tokens, don't load everything
  - Use the extra capacity for broader code context (more reference files)
  - Include diagrams and visual specs when available
  - Still prioritize relevant context over quantity
```

### Strategy for GPT-4o (128K context)

```
GPT-4o's strengths:
  - Well-tuned for code generation tasks
  - Good at following structured prompts
  - Strong reasoning with focused context

Recommended strategy:
  - Budget: 25% for specs and code, 15% for conversation, 60% reserve
  - Use spec summaries more aggressively (full specs may exceed budget)
  - Prioritize the most critical spec sections
  - Break large tasks into smaller, focused interactions
  - Use system prompt efficiently (keep it concise)
```

### Strategy for Open-Source Models (Llama, Mistral -- 128K)

```
Open-source model considerations:
  - Context recall may be weaker than frontier models
  - Performance degrades more with context length
  - Best results with highly focused context

Recommended strategy:
  - Budget: 20% for specs and code, 10% for conversation, 70% reserve
  - Always use summaries, load full specs only when absolutely needed
  - Keep context under 50K tokens if possible
  - Use very explicit, structured prompts
  - Break tasks into smallest possible units
```

---

## 3.12 The Spec Registry in Practice: A Worked Example

### Building a Context-Managed Workflow

Let's put everything together with a real workflow. You've been asked to add a "due date" feature to the issue tracking module.

**Step 1: Start with the Registry**

You open `SPEC_REGISTRY.md` and find:

```
ISS-001: Issue CRUD & Lifecycle -- Dependencies: SYS-002, SYS-003, PRJ-001
ISS-002: Issue Assignment -- Dependencies: SYS-002, USR-001
ISS-003: Issue Labels & Priorities -- Dependencies: ISS-001
```

The "due date" is part of the issue lifecycle, so the primary spec is ISS-001.

**Step 2: Resolve Dependencies**

Using the dependency graph:

```
ISS-001 depends on:
  - SYS-002 (Auth) -- need to know permission model
  - SYS-003 (Database) -- need the Issue table schema
  - PRJ-001 (Projects) -- issues belong to projects
```

**Step 3: Load Context**

```markdown
## For the AI:

I'm adding a "due date" feature to issues. Here's the context:

### From SYS-003 (Database Schema):
The Issue table currently has:
- id (UUID)
- title (string, max 200)
- description (text, nullable)
- status (enum: open, in_progress, done, closed)
- priority (enum: none, low, medium, high, urgent)
- projectId (FK to Project)
- assigneeId (FK to User, nullable)
- createdAt, updatedAt (timestamps)

### From ISS-001 (Issue Lifecycle), Section 4 - Due Dates:
[If this section exists, load it. If not, we need to write it.]

### From SYS-004 (API Standards):
- All date fields use ISO 8601 format (UTC)
- Nullable fields return null, not undefined or empty string
- Validation errors return 400 with field-specific messages

### Task:
1. Update the spec ISS-001 to include due date requirements
2. Write tests for the due date feature
3. Implement the database migration
4. Implement the API changes
5. Update the existing tests
```

**Step 4: Generate the Spec Addition**

```markdown
## ISS-001 Addendum: Due Dates (Section 4)

### Requirements
1. Each issue MAY have a dueDate field (nullable timestamp, UTC)
2. dueDate SHALL be settable at issue creation or via update
3. dueDate SHALL be in the future or null (past dates not allowed on creation)
4. dueDate SHALL be stored as ISO 8601 UTC timestamp
5. The API SHALL support filtering issues by dueDate range
6. The API SHALL support sorting issues by dueDate (nulls last)
7. Issues with dueDate in the past and status != done/closed SHALL be flagged as "overdue"

### Constraints
8. dueDate validation SHALL allow dates up to 5 years in the future
9. Updating dueDate to a past date IS allowed (for corrections)
10. Clearing dueDate (setting to null) IS always allowed
```

**Step 5: Write Tests from the Spec Addition**

The AI now has focused context: the specific spec addition, the existing data model, and the API standards. It can generate targeted tests without being overwhelmed by irrelevant information.

> **Professor's Aside:** Notice how we loaded exactly 4 focused pieces of context, not the entire spec library. The AI has everything it needs for this specific task and nothing it doesn't need. This is the "just enough context" principle in action.

---

## 3.13 Common Pitfalls and How to Avoid Them

### Pitfall 1: The "Kitchen Sink" Prompt

**What it looks like:** Pasting every spec, every code file, and a long conversation history into a single prompt.

**Why it fails:** The AI can't distinguish important information from noise. Critical constraints get lost in the volume.

**Fix:** Use the registry and dependency graph to select only relevant specs. Budget your context.

### Pitfall 2: The "Amnesia" Problem

**What it looks like:** Starting a new conversation for a related task and forgetting to include context that was established in the previous conversation.

**Why it fails:** The AI generates code that's inconsistent with decisions made in the previous conversation.

**Fix:** Document decisions in the spec files (not just conversations). Use persistent configuration files. Commit spec updates between conversations.

### Pitfall 3: The "Stale Context" Problem

**What it looks like:** Loading specs that haven't been updated to reflect recent changes.

**Why it fails:** The AI implements against an outdated spec, then fails tests that were written against the current spec.

**Fix:** Include "Last Updated" dates in the registry. Set up CI checks that flag specs not updated in 90+ days. Make spec updates part of the definition of "done" for every feature.

### Pitfall 4: The "Implicit Context" Assumption

**What it looks like:** Assuming the AI "knows" about your project because you've been working with it all day.

**Why it fails:** Every new conversation starts fresh. The AI doesn't remember previous sessions.

**Fix:** Never assume. Always provide explicit context. If a constraint matters, it should be in a file that gets loaded into context.

### Pitfall 5: The "Monolithic Spec" Problem

**What it looks like:** One giant spec file that covers everything.

**Why it fails:** You can't load part of it. It's all or nothing, and "all" is usually too much.

**Fix:** Break specs into the three-level hierarchy. Keep each spec file focused on a single concern. A spec file should ideally be 100-300 lines -- detailed enough to be useful, concise enough to fit in a context budget.

---

## 3.14 Exercises

### Exercise 1: Context Budget Calculation (Beginner)

Given the following project parameters, calculate the context budget and determine whether the requested context fits:

**Model:** Claude (200K token context window)
**Project configuration (.cursorrules):** 2,500 tokens
**Conversation history so far:** 15,000 tokens

**Task:** Implement the Payment Processing module

**Specs to load:**
- BIL-002 (Payment Processing): 4,200 tokens
- BIL-001 (Subscription Plans): 3,800 tokens
- SYS-003 (Database Schema): 6,500 tokens
- SYS-004 (API Standards): 3,200 tokens
- SYS-006 (Security Requirements): 5,100 tokens
- XCT-003 (Testing Standards): 2,800 tokens

**Code files to load:**
- Existing billing service: 4,500 tokens
- Existing billing tests: 3,200 tokens
- Payment gateway integration: 5,800 tokens

**Questions:**
1. What is the total context being loaded?
2. What percentage of the context window is consumed?
3. Is there enough reserve for reasoning? (Target: 50% reserve)
4. If not, which specs should you load as summaries instead?

### Exercise 2: Spec Decomposition (Intermediate)

You have a monolithic spec file for an e-commerce checkout system (estimated 12,000 tokens). Decompose it into a hierarchical set of specs:

**The monolithic spec covers:**
- Shopping cart operations
- Address validation
- Shipping method selection
- Tax calculation
- Payment processing
- Order confirmation
- Email receipts
- Inventory reservation

**Your deliverable:**
1. A list of 6-10 individual spec files with their estimated sizes
2. A dependency graph showing how they relate
3. A context loading plan for three different tasks:
   - Task A: Fix a bug in tax calculation
   - Task B: Add a new shipping method
   - Task C: Implement the complete checkout flow

### Exercise 3: Build a Context Loader (Advanced)

Implement a `context-loader.ts` tool that:

1. Reads the `SPEC_REGISTRY.md`
2. Accepts a list of target spec IDs
3. Resolves dependencies (using the dependency graph)
4. Estimates the total token count
5. If the total exceeds a budget, automatically switches to summaries for less-critical specs
6. Outputs a formatted context bundle ready to paste into an AI prompt

**Your deliverable:** A complete, working TypeScript file that:
- Parses the registry markdown
- Builds the dependency graph
- Resolves transitive dependencies
- Estimates token counts
- Generates a context bundle within a specified token budget
- Falls back to summaries when full specs exceed the budget

### Exercise 4: Registry Design for Your Project (Capstone)

Take a real or hypothetical project you're working on and:

1. **Identify all the specs** you would need (even if they don't exist yet)
2. **Organize them** into the three-level hierarchy (system, module, component)
3. **Map the dependencies** between specs
4. **Create a complete SPEC_REGISTRY.md** file
5. **Create a CLAUDE.md** (or .cursorrules) file that references the registry
6. **Document three example tasks** and the context loading plan for each

**Your deliverable:** A complete set of files that could be added to a real project to enable context-managed SDD.

---

## Summary

In this chapter, we've explored the engineering discipline of context management -- one of the most practical and impactful skills in Spec-Driven Development.

**Key takeaways:**

1. **Context windows are finite and precious.** Even with models that offer 200K or 1M tokens, filling the window degrades performance. Treat context like memory: scarce and valuable.

2. **More context is not better context.** The "needle in a haystack" problem means that relevant information surrounded by irrelevant information is harder for the AI to use. Focus beats volume.

3. **The Registry Pattern is essential.** A master index of all specs, with IDs, paths, statuses, and dependencies, allows both humans and AIs to quickly navigate to the right context.

4. **Hierarchical specs (system, module, component) enable granular loading.** Instead of loading everything, load only the level(s) relevant to the current task.

5. **The "just enough context" principle governs all decisions.** Provide exactly what the AI needs -- no more, no less. Budget your context, measure it, and reserve space for reasoning.

6. **Persistent configuration files reduce repetitive context loading.** .cursorrules, CLAUDE.md, and similar files inject critical constraints automatically, freeing context budget for task-specific information.

7. **Spec dependencies must be explicit and managed.** When Spec A depends on Spec B, the context loader needs to know. Track dependencies in the registry and validate them automatically.

8. **Different models need different strategies.** Adapt your context management approach to the model's strengths and weaknesses.

This concludes Module 3: Validation & The Feedback Loop. You now have the tools to:
- Map specs to tests (Chapter 1)
- Prevent intent drift through automated linting (Chapter 2)
- Manage context strategically for optimal AI performance (Chapter 3)

In Module 4, we'll take everything you've learned and apply it to a complete, real-world project from start to finish.

---

*End of Module 3*

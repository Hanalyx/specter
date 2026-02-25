# Chapter 1: The Multi-Agent Workflow

## MODULE 04 — Advanced Orchestration & Agents (Advanced Level)

---

### Lecture Preamble

*Welcome back. If you have made it this far into the course, you have internalized the fundamentals: what a specification is, why it matters, and how to write one that an AI coding agent can consume without hallucinating its way into a mess. You have learned to think in contracts, to separate intent from implementation, and to treat the spec as the single source of truth.*

*Now we enter the territory where things get genuinely interesting -- and, I will be honest, genuinely complex. Today we are going to talk about what happens when you stop thinking of "an AI agent" as a single entity and start thinking of it as a team. A team of specialized agents, each with a defined role, each operating under its own constraints, and all of them coordinated by the specification you write.*

*This is not science fiction. This is how the most sophisticated AI-assisted development workflows operate right now, in 2026. Anthropic's Claude Code spawns sub-agents with its Task tool. Microsoft's AutoGen orchestrates multi-agent conversations. Google's Gemini pipelines chain reasoning steps across specialized modules. OpenAI's agent frameworks compose tool-using agents into workflows. Meta's Llama-based systems are being deployed in open-source multi-agent configurations across thousands of organizations.*

*But here is the thing that most people miss: none of these multi-agent systems work reliably without a specification driving the coordination. The spec is not just an input to one agent -- it is the shared contract that binds all of them together. Today, we are going to learn exactly how that works.*

---

## 1.1 The Three-Agent Pattern: Architect, Builder, Critic

Let us start with the core pattern. In Spec-Driven Development, the most effective multi-agent workflow decomposes the development process into three distinct roles:

| Agent | Role | Analogy | Primary Input | Primary Output |
|-------|------|---------|---------------|----------------|
| **Architect Agent** | Writes and refines the specification | Product Manager / Tech Lead | Requirements, constraints, context | A complete, validated spec |
| **Builder Agent** | Executes the spec into working code | Software Developer | The spec | Code, tests, artifacts |
| **Critic Agent** | Validates code against the spec | QA Engineer / Code Reviewer | The spec + the code | Validation report, issues list |

This is not an arbitrary decomposition. It mirrors the fundamental tension in all software development: the people who decide *what* to build should not be the same people who decide *how* to build it, and neither group should be the ones who judge whether it was built correctly.

> **Professor's aside:** If you have ever worked at a company where the developer writes the code, writes the tests, and then approves their own pull request -- you know exactly why separation of concerns matters. The three-agent pattern enforces that separation structurally, not culturally.

Here is the pattern visualized as a flow:

```
                    Requirements / User Story
                            |
                            v
                  +-------------------+
                  |  ARCHITECT AGENT  |
                  |  (Spec Writer)    |
                  +-------------------+
                            |
                       [THE SPEC]
                            |
                            v
                  +-------------------+
                  |   BUILDER AGENT   |
                  |  (Code Generator) |
                  +-------------------+
                            |
                    [CODE + TESTS]
                            |
                            v
                  +-------------------+
                  |   CRITIC AGENT    |
                  |  (Validator)      |
                  +-------------------+
                            |
                     [VALIDATION REPORT]
                            |
                    +-------+-------+
                    |               |
                    v               v
                 PASS            FAIL
                  |               |
                  v               v
               DEPLOY    Back to ARCHITECT
                         (with issue details)
```

Notice something critical in that diagram: when the Critic finds issues, it sends them back to the **Architect**, not to the Builder. This is one of the most important design decisions in the entire pattern, and we will explore why later in this chapter.

---

## 1.2 The Architect Agent: Spec Writer and Refiner

The Architect Agent is responsible for taking ambiguous, incomplete, or high-level requirements and transforming them into a precise, unambiguous specification that the Builder Agent can execute.

### What the Architect Agent Does

1. **Interprets requirements** -- Takes natural language user stories, feature requests, or bug reports and extracts the core intent.
2. **Resolves ambiguity** -- Identifies places where the requirements are unclear and either asks clarifying questions or makes explicit decisions.
3. **Structures the spec** -- Produces a spec in a consistent, machine-readable format that follows the project's spec schema.
4. **Validates completeness** -- Ensures the spec covers all necessary aspects: inputs, outputs, constraints, edge cases, error handling.
5. **Refines iteratively** -- When the Critic reports issues, the Architect updates the spec to address them.

### Architect Agent Capabilities

The Architect Agent needs access to:

- **The project's existing codebase** (read-only) -- to understand current architecture and conventions
- **Existing specs** -- to ensure consistency and avoid contradictions
- **A spec schema/template** -- to produce specs in the expected format
- **Domain knowledge** -- context about the business domain, API contracts, etc.

### Architect Agent Constraints

Equally important are the things the Architect Agent must **not** do:

- **Must not write code** -- Its output is specifications, never implementation
- **Must not make implementation decisions** -- It specifies *what*, never *how*
- **Must not bypass the schema** -- Every spec must conform to the agreed format
- **Must not ignore the Critic** -- When issues come back, the spec must be updated

Here is a concrete example of an Architect Agent's system prompt:

```markdown
# Architect Agent System Prompt

You are the Architect Agent in a Spec-Driven Development pipeline.

## Your Role
You transform requirements into precise, actionable specifications.

## Your Constraints
- You MUST output specs in the project's YAML spec format
- You MUST NOT include implementation code in specs
- You MUST ensure every spec includes: purpose, inputs, outputs,
  constraints, error cases, and acceptance criteria
- You MUST reference existing specs when your new spec interacts
  with existing functionality
- You MUST flag any ambiguity you cannot resolve as an
  "OPEN_QUESTION" in the spec

## Your Inputs
- User requirements (natural language)
- Existing project specs (YAML files)
- Project architecture context (read-only codebase access)

## Your Output Format
A single YAML spec file following the project schema.

## When You Receive Critic Feedback
- Address every issue raised by the Critic
- Update the spec, do not argue with the Critic
- If a Critic issue reveals a fundamental requirements problem,
  flag it as REQUIRES_HUMAN_INPUT
```

### A Real Architect Agent Output

Given the requirement: *"Users should be able to export their dashboard data as a CSV file"*, the Architect Agent might produce:

```yaml
# spec/features/dashboard-csv-export.spec.yaml
spec_version: "1.0.0"
feature: "dashboard-csv-export"
status: "draft"
created: "2026-02-24"
author: "architect-agent"

purpose: |
  Allow authenticated users to export the data currently displayed
  on their dashboard as a downloadable CSV file. The export should
  reflect the current filter and date range selections.

inputs:
  - name: "user_session"
    type: "AuthenticatedSession"
    required: true
    description: "The user must be authenticated with a valid session"
  - name: "dashboard_filters"
    type: "DashboardFilterState"
    required: true
    description: "Current filter state including date range, category, and search terms"

outputs:
  - name: "csv_file"
    type: "File"
    format: "text/csv"
    encoding: "UTF-8"
    description: "A CSV file containing the filtered dashboard data"
    properties:
      filename_pattern: "dashboard-export-{date}-{timestamp}.csv"
      headers: "First row must contain column headers matching display names"
      max_rows: 50000
      delimiter: ","
      quoting: "Fields containing commas or newlines must be quoted"

constraints:
  - "Export must respect current RBAC permissions -- users only export data they can see"
  - "Maximum export size: 50,000 rows"
  - "Export must complete within 30 seconds or return a timeout error"
  - "CSV must be valid per RFC 4180"
  - "Date fields must be formatted as ISO 8601"
  - "Numeric fields must not include locale-specific formatting (no commas in numbers)"

error_cases:
  - condition: "User session expired during export"
    behavior: "Return 401 with message 'Session expired. Please log in again.'"
  - condition: "Export exceeds 50,000 rows"
    behavior: "Return 413 with message 'Export too large. Please narrow your filters.'"
  - condition: "Export times out (>30s)"
    behavior: "Return 504 with message 'Export timed out. Please try a smaller date range.'"
  - condition: "No data matches current filters"
    behavior: "Return CSV with headers only and 200 status"

acceptance_criteria:
  - "Given a user with 100 dashboard records and no filters, when they click Export, then a CSV with 101 rows (1 header + 100 data) downloads"
  - "Given a user with date filter set to 'Last 7 days', when they export, the CSV contains only records from the last 7 days"
  - "Given a user without admin role, when they export, the CSV does not contain admin-only columns"
  - "Given an export of 50,001 rows, the system returns a 413 error before generating the file"

dependencies:
  - spec: "auth/session-management.spec.yaml"
    reason: "Requires authenticated session validation"
  - spec: "dashboard/filter-state.spec.yaml"
    reason: "Consumes the current filter state"

open_questions: []
```

> **Professor's aside:** Look at how much detail is in that spec. The Architect Agent did not just say "export to CSV." It specified the encoding, the delimiter behavior, the maximum rows, the error messages, and the exact conditions under which each error fires. This is the level of precision that makes the Builder Agent's job straightforward rather than guesswork.

---

## 1.3 The Builder Agent: Faithful Executor of the Contract

The Builder Agent has one job: take the spec and produce code that satisfies it. This sounds simple. It is not.

### How the Builder Stays Faithful to the Contract

The Builder Agent's relationship with the spec is one of **strict compliance**, not creative interpretation. Here are the principles:

**1. The spec is the complete truth.**
The Builder does not add features the spec did not request. If the spec says "return a 413 error when rows exceed 50,000," the Builder does not also add a warning at 40,000 rows -- even if that seems like a good idea.

**2. Ambiguity is a blocker, not an invitation.**
When the spec is unclear, the Builder does not guess. It returns a structured error report to the Architect indicating what is ambiguous.

**3. Implementation decisions belong to the Builder.**
While the spec defines *what*, the Builder decides *how*. The spec says "export must complete within 30 seconds." The Builder decides whether to use streaming, pagination, a background job with polling, or some other approach.

**4. Test generation is part of building.**
The Builder generates tests that map directly to the spec's acceptance criteria. Each acceptance criterion becomes at least one test.

Here is a Builder Agent system prompt:

```markdown
# Builder Agent System Prompt

You are the Builder Agent in a Spec-Driven Development pipeline.

## Your Role
You transform specifications into working, tested code.

## Your Constraints
- You MUST implement every requirement in the spec
- You MUST NOT add functionality not specified in the spec
- You MUST generate tests for every acceptance criterion
- You MUST follow the project's coding conventions and patterns
- You MUST report any spec ambiguity rather than guessing

## Your Inputs
- A validated spec (YAML)
- Project codebase (read/write access)
- Project coding conventions document

## Your Outputs
- Implementation code
- Test code (unit + integration as appropriate)
- A build report listing:
  - Files created/modified
  - Spec requirements addressed (by ID)
  - Any spec ambiguities encountered

## When You Encounter Ambiguity
- DO NOT guess or make assumptions
- Output an AMBIGUITY_REPORT with:
  - The ambiguous spec section
  - What is unclear
  - Two or more possible interpretations
  - Your recommended interpretation (but do not act on it)
```

### Builder Agent Output: Implementation

Given the CSV export spec above, the Builder Agent might produce:

```typescript
// src/features/dashboard-export/csv-export.service.ts

import { Injectable, HttpException, HttpStatus } from '@nestjs/common';
import { DashboardDataService } from '../dashboard/dashboard-data.service';
import { AuthService } from '../auth/auth.service';
import { DashboardFilterState } from '../dashboard/types/filter-state';
import { stringify } from 'csv-stringify/sync';

/**
 * CSV Export Service
 *
 * Implements: spec/features/dashboard-csv-export.spec.yaml
 * Spec Version: 1.0.0
 */

const MAX_EXPORT_ROWS = 50_000;
const EXPORT_TIMEOUT_MS = 30_000;

interface CsvExportResult {
  filename: string;
  content: string;
  contentType: 'text/csv';
  encoding: 'UTF-8';
}

@Injectable()
export class CsvExportService {
  constructor(
    private readonly dashboardData: DashboardDataService,
    private readonly auth: AuthService,
  ) {}

  /**
   * Export dashboard data as CSV.
   *
   * Spec constraints:
   * - Respects RBAC permissions
   * - Max 50,000 rows
   * - 30-second timeout
   * - RFC 4180 compliant CSV
   * - ISO 8601 dates
   * - No locale-specific number formatting
   */
  async exportDashboardCsv(
    sessionId: string,
    filters: DashboardFilterState,
  ): Promise<CsvExportResult> {
    // Validate session (spec: error_cases[0])
    const session = await this.auth.validateSession(sessionId);
    if (!session.valid) {
      throw new HttpException(
        'Session expired. Please log in again.',
        HttpStatus.UNAUTHORIZED,
      );
    }

    // Fetch data with timeout (spec: constraints[2])
    const data = await this.fetchWithTimeout(
      () => this.dashboardData.getFilteredData(filters, session.user),
      EXPORT_TIMEOUT_MS,
    );

    // Check row limit (spec: error_cases[1])
    if (data.rows.length > MAX_EXPORT_ROWS) {
      throw new HttpException(
        'Export too large. Please narrow your filters.',
        HttpStatus.PAYLOAD_TOO_LARGE,
      );
    }

    // Get visible columns based on user role (spec: constraints[0])
    const visibleColumns = this.dashboardData.getColumnsForRole(
      session.user.role,
    );

    // Build CSV (spec: output properties)
    const headers = visibleColumns.map((col) => col.displayName);
    const rows = data.rows.map((row) =>
      visibleColumns.map((col) => this.formatCellValue(row[col.key], col.type)),
    );

    const csvContent = stringify([headers, ...rows], {
      delimiter: ',',        // spec: outputs[0].properties.delimiter
      quoted_string: true,   // spec: outputs[0].properties.quoting
    });

    const now = new Date();
    const dateStr = now.toISOString().split('T')[0];
    const timestampStr = now.getTime().toString();

    return {
      filename: `dashboard-export-${dateStr}-${timestampStr}.csv`,
      content: csvContent,
      contentType: 'text/csv',
      encoding: 'UTF-8',
    };
  }

  /**
   * Format cell value according to spec constraints:
   * - Dates: ISO 8601
   * - Numbers: no locale formatting
   */
  private formatCellValue(
    value: unknown,
    type: 'string' | 'number' | 'date' | 'boolean',
  ): string {
    if (value === null || value === undefined) return '';

    switch (type) {
      case 'date':
        // spec: constraints[4] - ISO 8601
        return value instanceof Date
          ? value.toISOString()
          : new Date(value as string).toISOString();
      case 'number':
        // spec: constraints[5] - no locale formatting
        return String(value);
      case 'boolean':
        return value ? 'true' : 'false';
      default:
        return String(value);
    }
  }

  private async fetchWithTimeout<T>(
    fn: () => Promise<T>,
    timeoutMs: number,
  ): Promise<T> {
    const timeoutPromise = new Promise<never>((_, reject) =>
      setTimeout(
        () =>
          reject(
            new HttpException(
              // spec: error_cases[2]
              'Export timed out. Please try a smaller date range.',
              HttpStatus.GATEWAY_TIMEOUT,
            ),
          ),
        timeoutMs,
      ),
    );

    return Promise.race([fn(), timeoutPromise]);
  }
}
```

And the corresponding tests:

```typescript
// src/features/dashboard-export/__tests__/csv-export.service.spec.ts

import { CsvExportService } from '../csv-export.service';
import { DashboardDataService } from '../../dashboard/dashboard-data.service';
import { AuthService } from '../../auth/auth.service';

/**
 * Tests for: spec/features/dashboard-csv-export.spec.yaml
 * Each test maps to a specific acceptance criterion.
 */
describe('CsvExportService', () => {
  let service: CsvExportService;
  let mockDashboardData: jest.Mocked<DashboardDataService>;
  let mockAuth: jest.Mocked<AuthService>;

  beforeEach(() => {
    mockAuth = {
      validateSession: jest.fn().mockResolvedValue({
        valid: true,
        user: { id: 'user-1', role: 'member' },
      }),
    } as any;

    mockDashboardData = {
      getFilteredData: jest.fn(),
      getColumnsForRole: jest.fn().mockReturnValue([
        { key: 'name', displayName: 'Name', type: 'string' },
        { key: 'value', displayName: 'Value', type: 'number' },
        { key: 'created', displayName: 'Created', type: 'date' },
      ]),
    } as any;

    service = new CsvExportService(mockDashboardData, mockAuth);
  });

  // Acceptance Criterion 1:
  // "Given a user with 100 dashboard records and no filters,
  //  when they click Export, then a CSV with 101 rows downloads"
  it('should export 101 rows for 100 records (header + data)', async () => {
    const rows = Array.from({ length: 100 }, (_, i) => ({
      name: `Item ${i}`,
      value: i * 10,
      created: new Date('2026-01-15'),
    }));

    mockDashboardData.getFilteredData.mockResolvedValue({ rows });

    const result = await service.exportDashboardCsv('session-1', {});
    const lines = result.content.trim().split('\n');

    expect(lines).toHaveLength(101); // 1 header + 100 data rows
    expect(lines[0]).toContain('Name');
    expect(result.contentType).toBe('text/csv');
    expect(result.encoding).toBe('UTF-8');
  });

  // Acceptance Criterion 2:
  // "Given a user with date filter set to 'Last 7 days',
  //  when they export, the CSV contains only records from the last 7 days"
  it('should pass date filters to data service', async () => {
    const filters = { dateRange: 'last_7_days' };
    mockDashboardData.getFilteredData.mockResolvedValue({ rows: [] });

    await service.exportDashboardCsv('session-1', filters);

    expect(mockDashboardData.getFilteredData).toHaveBeenCalledWith(
      filters,
      expect.objectContaining({ id: 'user-1' }),
    );
  });

  // Acceptance Criterion 3:
  // "Given a user without admin role, when they export,
  //  the CSV does not contain admin-only columns"
  it('should filter columns based on user role', async () => {
    mockDashboardData.getFilteredData.mockResolvedValue({
      rows: [{ name: 'Test', value: 42, created: new Date() }],
    });

    await service.exportDashboardCsv('session-1', {});

    expect(mockDashboardData.getColumnsForRole).toHaveBeenCalledWith('member');
  });

  // Acceptance Criterion 4:
  // "Given an export of 50,001 rows, the system returns a 413 error
  //  before generating the file"
  it('should throw 413 when rows exceed 50,000', async () => {
    const rows = Array.from({ length: 50_001 }, (_, i) => ({
      name: `Item ${i}`,
      value: i,
      created: new Date(),
    }));

    mockDashboardData.getFilteredData.mockResolvedValue({ rows });

    await expect(
      service.exportDashboardCsv('session-1', {}),
    ).rejects.toThrow('Export too large. Please narrow your filters.');
  });

  // Error case: expired session
  it('should throw 401 when session is expired', async () => {
    mockAuth.validateSession.mockResolvedValue({ valid: false });

    await expect(
      service.exportDashboardCsv('invalid-session', {}),
    ).rejects.toThrow('Session expired. Please log in again.');
  });

  // Edge case: no data matches filters
  it('should return CSV with headers only when no data matches', async () => {
    mockDashboardData.getFilteredData.mockResolvedValue({ rows: [] });

    const result = await service.exportDashboardCsv('session-1', {});
    const lines = result.content.trim().split('\n');

    expect(lines).toHaveLength(1); // Headers only
    expect(lines[0]).toContain('Name');
  });
});
```

> **Professor's aside:** Notice how every test has a comment referencing the specific acceptance criterion it validates. This traceability from spec to code to test is the backbone of SDD. When someone asks "does the system do X?", you can trace from the spec requirement to the implementation to the test that proves it.

---

## 1.4 The Critic Agent: Automated Reviewer

The Critic Agent is perhaps the most underappreciated role in the three-agent pattern. Its job is to take the spec and the code produced by the Builder, and systematically verify that the code satisfies every requirement in the spec.

### What the Critic Agent Checks

The Critic Agent performs several categories of validation:

**Completeness Check** -- Is every spec requirement addressed in the code?

```yaml
# Critic Agent completeness check output
completeness_report:
  spec: "dashboard-csv-export.spec.yaml"
  total_requirements: 14
  implemented: 13
  missing:
    - requirement: "constraints[3]: CSV must be valid per RFC 4180"
      status: "PARTIALLY_IMPLEMENTED"
      detail: |
        The code uses csv-stringify which handles most RFC 4180 cases,
        but does not explicitly handle the CRLF line ending requirement.
        RFC 4180 specifies CRLF, but csv-stringify defaults to LF on
        Linux systems.
      severity: "medium"
```

**Correctness Check** -- Does the code do what the spec says, not something subtly different?

```yaml
# Critic Agent correctness check output
correctness_report:
  issues:
    - location: "csv-export.service.ts:L85"
      spec_reference: "error_cases[1]"
      expected: "Return 413 BEFORE generating the file"
      actual: "Code fetches all data first, then checks count"
      severity: "high"
      detail: |
        The spec says to return the error before generating the file,
        implying a count check before full data retrieval. The current
        implementation loads all 50,001+ rows into memory before
        checking the count. This could cause OOM errors for very
        large datasets. Consider a COUNT query first.
```

**Consistency Check** -- Does the code match the project's patterns and conventions?

**Test Coverage Check** -- Does every acceptance criterion have a corresponding test?

### Critic Agent System Prompt

```markdown
# Critic Agent System Prompt

You are the Critic Agent in a Spec-Driven Development pipeline.

## Your Role
You validate that code produced by the Builder Agent faithfully
implements the specification produced by the Architect Agent.

## Your Constraints
- You MUST check every requirement in the spec against the code
- You MUST NOT suggest new features not in the spec
- You MUST NOT rewrite code -- you report issues only
- You MUST categorize issues by severity: critical, high, medium, low
- You MUST provide specific file and line references for every issue
- You MUST verify test coverage for every acceptance criterion

## Your Inputs
- The specification (YAML)
- The implementation code
- The test code
- Project coding conventions

## Your Output Format
A structured validation report in YAML containing:
- completeness_report: requirements coverage
- correctness_report: behavioral accuracy
- consistency_report: convention adherence
- test_coverage_report: acceptance criteria coverage
- verdict: PASS | FAIL | PASS_WITH_WARNINGS
- issues: detailed list of all problems found

## Severity Definitions
- critical: The code contradicts the spec or has security implications
- high: A spec requirement is missing or incorrectly implemented
- medium: Implementation works but deviates from spec intent
- low: Style or convention issues that don't affect behavior
```

---

## 1.5 Mapping to Real Software Teams

The three-agent pattern is not an invention of the AI era. It is a formalization of roles that have existed in software teams for decades:

| Agent Role | Traditional Role | Responsibility |
|-----------|-----------------|----------------|
| Architect Agent | Product Manager + Tech Lead | Defines what gets built, resolves ambiguity, maintains the contract |
| Builder Agent | Software Developer | Implements the solution, makes technical decisions, writes tests |
| Critic Agent | QA Engineer + Code Reviewer | Validates correctness, completeness, and quality |

The key insight is this: **when humans fill these roles, communication is lossy.** The product manager writes a Jira ticket. The developer interprets it. The QA engineer interprets both the ticket and the code. At every handoff, information is lost, assumptions are introduced, and drift accumulates.

In the SDD multi-agent pattern, the specification eliminates this drift because it is:
- **Machine-readable** -- No ambiguity in parsing
- **Versioned** -- Changes are tracked
- **Complete** -- It covers inputs, outputs, constraints, and error cases
- **Shared** -- All agents read the exact same document

> **Professor's aside:** I am not suggesting we replace human teams with AI agents. What I am suggesting is that the *coordination mechanism* -- the spec -- is the same whether your team is three humans, three AI agents, or a mix. The spec is the protocol, and agents (human or artificial) are the participants.

---

## 1.6 How Anthropic's Claude Code Uses Multi-Agent Patterns

Let us look at how this works in practice at the companies building these systems.

Anthropic's Claude Code (the CLI tool for AI-assisted development) implements a multi-agent pattern through its **Task tool**. When Claude Code encounters a complex problem, it can spawn sub-agents to handle specialized parts of the work.

### The Task Tool Pattern

Here is how it works conceptually:

```
Main Claude Code Session (Orchestrator)
    |
    |--- Task: "Analyze the existing codebase structure"
    |         (Sub-agent with read-only access)
    |         Returns: Architecture summary
    |
    |--- Task: "Generate implementation for the CSV export feature"
    |         (Sub-agent with write access, scoped to specific files)
    |         Returns: Code files
    |
    |--- Task: "Review the generated code against these requirements"
    |         (Sub-agent with read-only access to both spec and code)
    |         Returns: Review report
    |
    Main Session synthesizes results and presents to user
```

This is a direct implementation of the Architect-Builder-Critic pattern. The main session acts as the orchestrator, and each Task is a specialized agent with:

- **Scoped context** -- Each sub-agent sees only what it needs
- **Defined output** -- Each sub-agent has a clear deliverable
- **Limited authority** -- Sub-agents cannot exceed their mandate

### Team Spawning

Claude Code also supports what Anthropic calls "team spawning" -- the ability for agents to create other agents dynamically based on the needs of the task. For example:

```
Architect Agent assesses a complex feature request
    |
    |--- Determines it needs 3 Builder Agents:
    |    |--- Builder 1: Frontend component
    |    |--- Builder 2: Backend API
    |    |--- Builder 3: Database migration
    |
    |--- Spawns 3 parallel Builder tasks
    |--- Collects results
    |--- Spawns Critic Agent to review all three
```

The critical enabler here is the spec. Without a structured specification, the orchestrator would have no reliable way to divide the work, and the Critic would have no standard against which to judge the results.

> **Professor's aside:** When you use Claude Code's Task tool, you are implicitly using this pattern. The prompt you write for each Task is, in essence, a mini-spec. The better your prompt, the better your results. SDD just formalizes this into a rigorous practice.

---

## 1.7 How Google's Gemini and DeepMind's AlphaCode Approach Multi-Step Code Generation

Google takes a somewhat different architectural approach, but the underlying principle is the same.

### Gemini's Multi-Step Reasoning

Google's Gemini models, particularly when integrated into development workflows through tools like Project IDX and Gemini Code Assist, use a multi-step reasoning approach:

1. **Plan step** -- The model generates a plan for what code needs to be written (analogous to the Architect phase)
2. **Generate step** -- The model produces the code (Builder phase)
3. **Verify step** -- The model checks its own output against the plan (Critic phase)

What is interesting about Google's approach is that these "agents" are often the same model executing different prompts in sequence, rather than truly separate models. The specification (plan) still serves as the coordination mechanism between steps.

### DeepMind's AlphaCode and Its Descendants

DeepMind's AlphaCode research demonstrated a powerful variation: **generate many candidates, then select the best one**. The workflow looks like:

```
Problem Specification
        |
        v
+------------------+
| GENERATOR        |    Produces 100-1000 candidate solutions
| (Many Builders)  |
+------------------+
        |
        v
+------------------+
| FILTER/SELECTOR  |    Eliminates solutions that fail tests
| (Critic)         |    Clusters remaining by behavior
+------------------+    Selects representative from best cluster
        |
        v
   Best Solution
```

This is the Architect-Builder-Critic pattern at scale. The specification (problem statement + test cases) drives the entire pipeline. The "Architect" is implicit -- it is the problem specification itself. The "Builders" are massively parallel. The "Critic" is a filter that uses the spec's test cases to select the best output.

---

## 1.8 OpenAI's Agent Frameworks and SDD Orchestration

OpenAI has been investing heavily in agent frameworks, and their approach reveals important design patterns for SDD.

### The Assistants API and Function Calling

OpenAI's Assistants API allows developers to create agents with:
- **Instructions** (the spec for the agent's behavior)
- **Tools** (capabilities the agent can use)
- **Files** (context the agent can reference)

In an SDD pipeline, you might configure three Assistants:

```python
# Setting up a 3-agent SDD pipeline with OpenAI Assistants API

import openai

client = openai.OpenAI()

# The Architect Agent
architect = client.beta.assistants.create(
    name="Architect Agent",
    instructions="""
    You are a specification writer. Given requirements, produce
    a detailed YAML specification following the provided schema.
    Never write implementation code. Focus on what, not how.
    """,
    model="gpt-4o",
    tools=[{"type": "file_search"}],  # Can search existing specs
)

# The Builder Agent
builder = client.beta.assistants.create(
    name="Builder Agent",
    instructions="""
    You are a code implementer. Given a YAML specification, produce
    working code that satisfies every requirement. Generate tests
    for every acceptance criterion. Report any spec ambiguities.
    """,
    model="gpt-4o",
    tools=[
        {"type": "code_interpreter"},  # Can run and test code
        {"type": "file_search"},       # Can search codebase
    ],
)

# The Critic Agent
critic = client.beta.assistants.create(
    name="Critic Agent",
    instructions="""
    You are a code reviewer. Given a specification and implementation,
    verify that every spec requirement is correctly implemented.
    Output a structured validation report. Never write code yourself.
    """,
    model="gpt-4o",
    tools=[{"type": "file_search"}],
)
```

### OpenAI's Swarm Framework

OpenAI's experimental Swarm framework takes a more dynamic approach, where agents can hand off to each other based on the current state of the conversation. In an SDD context:

```python
from swarm import Swarm, Agent

client = Swarm()

def hand_off_to_builder(spec_yaml: str):
    """Architect hands off the completed spec to the Builder."""
    return builder_agent

def hand_off_to_critic(code: str, spec: str):
    """Builder hands off completed code to the Critic."""
    return critic_agent

def hand_off_to_architect(issues: str):
    """Critic sends issues back to the Architect for spec revision."""
    return architect_agent

architect_agent = Agent(
    name="Architect",
    instructions="Write specs from requirements. When done, hand off to Builder.",
    functions=[hand_off_to_builder],
)

builder_agent = Agent(
    name="Builder",
    instructions="Implement code from specs. When done, hand off to Critic.",
    functions=[hand_off_to_critic],
)

critic_agent = Agent(
    name="Critic",
    instructions="Validate code against spec. If issues, hand off to Architect.",
    functions=[hand_off_to_architect],
)
```

---

## 1.9 Meta's Approach with Llama-Based Coding Agents

Meta's contribution to the multi-agent landscape is significant because of Llama's open-source nature. This means anyone can build multi-agent SDD pipelines without depending on a commercial API.

### Llama-Based Agent Configurations

The open-source community has built several multi-agent frameworks on top of Llama models:

```python
# Simplified example of a Llama-based 3-agent pipeline
# using a local model server (e.g., vLLM, Ollama, or TGI)

import requests
from dataclasses import dataclass
from typing import Literal

LLAMA_API_BASE = "http://localhost:8000/v1"

@dataclass
class AgentConfig:
    name: str
    role: Literal["architect", "builder", "critic"]
    system_prompt: str
    temperature: float
    max_tokens: int

AGENTS = {
    "architect": AgentConfig(
        name="Architect",
        role="architect",
        system_prompt="You write detailed YAML specifications...",
        temperature=0.3,   # Lower temp for precise specs
        max_tokens=4096,
    ),
    "builder": AgentConfig(
        name="Builder",
        role="builder",
        system_prompt="You implement code from YAML specifications...",
        temperature=0.2,   # Even lower for deterministic code
        max_tokens=8192,
    ),
    "critic": AgentConfig(
        name="Critic",
        role="critic",
        system_prompt="You validate code against specifications...",
        temperature=0.1,   # Lowest for rigorous analysis
        max_tokens=4096,
    ),
}

def run_agent(agent_name: str, user_message: str) -> str:
    """Run a single agent turn using the local Llama model."""
    config = AGENTS[agent_name]
    response = requests.post(
        f"{LLAMA_API_BASE}/chat/completions",
        json={
            "model": "meta-llama/Llama-3.3-70B-Instruct",
            "messages": [
                {"role": "system", "content": config.system_prompt},
                {"role": "user", "content": user_message},
            ],
            "temperature": config.temperature,
            "max_tokens": config.max_tokens,
        },
    )
    return response.json()["choices"][0]["message"]["content"]

def run_sdd_pipeline(requirements: str, max_iterations: int = 3) -> dict:
    """
    Run the full Architect -> Builder -> Critic pipeline.
    Loops until Critic passes or max iterations reached.
    """
    spec = None
    code = None
    critic_feedback = None

    for iteration in range(max_iterations):
        # Phase 1: Architect writes/refines spec
        architect_input = f"Requirements:\n{requirements}"
        if critic_feedback:
            architect_input += f"\n\nCritic Feedback:\n{critic_feedback}"
        spec = run_agent("architect", architect_input)

        # Phase 2: Builder implements from spec
        builder_input = f"Specification:\n{spec}"
        code = run_agent("builder", builder_input)

        # Phase 3: Critic validates
        critic_input = f"Specification:\n{spec}\n\nCode:\n{code}"
        validation = run_agent("critic", critic_input)

        if "verdict: PASS" in validation:
            return {
                "status": "success",
                "iterations": iteration + 1,
                "spec": spec,
                "code": code,
                "validation": validation,
            }

        critic_feedback = validation

    return {
        "status": "max_iterations_reached",
        "iterations": max_iterations,
        "spec": spec,
        "code": code,
        "validation": validation,
    }
```

The advantage of Meta's open-source approach is flexibility: you can run specialized, fine-tuned models for each agent role. An Architect model fine-tuned on specification writing. A Builder model fine-tuned on code generation. A Critic model fine-tuned on code review. Each optimized for its specific task.

---

## 1.10 Microsoft's AutoGen and Multi-Agent Frameworks

Microsoft's AutoGen framework is one of the most mature multi-agent frameworks available, and it maps naturally to SDD workflows.

### AutoGen's Conversational Agent Pattern

AutoGen treats multi-agent workflows as conversations between agents. Here is how an SDD pipeline looks in AutoGen:

```python
# AutoGen-based SDD pipeline (simplified)
from autogen import AssistantAgent, UserProxyAgent, GroupChat, GroupChatManager

# Define agents with SDD-specific roles
architect = AssistantAgent(
    name="Architect",
    system_message="""You are the Architect Agent. Your job is to write
    detailed YAML specifications from requirements. You never write code.
    When you receive feedback from Critic, you refine the spec.
    Output only the YAML spec, wrapped in ```yaml``` code blocks.""",
    llm_config={"model": "gpt-4o", "temperature": 0.3},
)

builder = AssistantAgent(
    name="Builder",
    system_message="""You are the Builder Agent. Your job is to implement
    code from YAML specifications. You write code and tests. You never
    modify the spec. If the spec is ambiguous, you report the ambiguity
    instead of guessing.""",
    llm_config={"model": "gpt-4o", "temperature": 0.2},
)

critic = AssistantAgent(
    name="Critic",
    system_message="""You are the Critic Agent. You validate code against
    the spec. Check completeness, correctness, and test coverage.
    Output a structured report with verdict: PASS or FAIL.
    If FAIL, list specific issues for the Architect to address.""",
    llm_config={"model": "gpt-4o", "temperature": 0.1},
)

# The orchestrator manages the conversation flow
group_chat = GroupChat(
    agents=[architect, builder, critic],
    messages=[],
    max_round=12,
    speaker_selection_method="round_robin",  # Architect -> Builder -> Critic
)

manager = GroupChatManager(
    groupchat=group_chat,
    llm_config={"model": "gpt-4o"},
)

# Kick off the pipeline
user_proxy = UserProxyAgent(
    name="Human",
    human_input_mode="NEVER",
    code_execution_config=False,
)

user_proxy.initiate_chat(
    manager,
    message="""
    Requirement: Users should be able to export their dashboard
    data as a CSV file. The export should respect their current
    filters and role-based access permissions.
    """,
)
```

What makes AutoGen particularly well-suited for SDD is its conversation management: agents can reference previous messages, the conversation history serves as a log, and the framework handles the turn-taking automatically.

---

## 1.11 The Communication Protocol Between Agents

Now that we have seen the agents in isolation and in various frameworks, let us formalize the communication protocol. This is the most important engineering decision in a multi-agent SDD system.

### What Gets Passed Between Agents

```
ORCHESTRATOR -> ARCHITECT:
  {
    "type": "spec_request",
    "requirements": "Natural language requirements...",
    "existing_specs": ["path/to/related.spec.yaml"],
    "project_context": { ... },
    "previous_critic_feedback": null | CriticReport
  }

ARCHITECT -> BUILDER:
  {
    "type": "build_request",
    "spec": "The complete YAML spec",
    "project_conventions": "Coding standards doc",
    "target_files": ["suggested/file/paths"],
    "existing_code_context": { ... }
  }

BUILDER -> CRITIC:
  {
    "type": "review_request",
    "spec": "The same YAML spec the Builder received",
    "implementation": {
      "files_created": [...],
      "files_modified": [...],
      "code_content": { "path": "content" }
    },
    "tests": {
      "files_created": [...],
      "test_content": { "path": "content" }
    },
    "build_notes": "Any notes from the Builder about decisions made"
  }

CRITIC -> ORCHESTRATOR:
  {
    "type": "validation_report",
    "verdict": "PASS" | "FAIL" | "PASS_WITH_WARNINGS",
    "spec_reference": "dashboard-csv-export.spec.yaml",
    "completeness": { ... },
    "correctness": { ... },
    "test_coverage": { ... },
    "issues": [
      {
        "id": "ISSUE-001",
        "severity": "high",
        "category": "correctness",
        "spec_requirement": "error_cases[1]",
        "description": "...",
        "file": "csv-export.service.ts",
        "line": 45,
        "suggestion": "..."
      }
    ]
  }
```

### The Format Question: Structured Data vs. Natural Language

There is an active debate in the field about whether inter-agent communication should be structured data (JSON, YAML) or natural language. Here is the practical guidance:

| Approach | Pros | Cons | Best For |
|----------|------|------|----------|
| Structured (JSON/YAML) | Parseable, consistent, less ambiguous | Harder for models to produce perfectly, rigid | Agent-to-agent communication |
| Natural Language | Easy to produce, flexible, rich | Ambiguous, hard to parse reliably, verbose | Human-to-agent communication |
| Hybrid | Best of both worlds | More complex to implement | Production systems |

The recommended approach in 2026 is **hybrid**: use structured formats for the core data (specs, validation reports) and natural language for commentary and explanations.

```yaml
# Hybrid format example: Critic output
validation_report:
  verdict: "FAIL"
  issues:
    - id: "ISSUE-001"
      severity: "high"
      spec_requirement: "error_cases[1]"
      file: "csv-export.service.ts"
      line: 45
      # Structured fields above, natural language explanation below
      explanation: |
        The spec requires that the 413 error be returned *before*
        generating the file. The current implementation loads all
        rows into memory via getFilteredData() and then checks the
        count. For a dataset with 500,000 rows, this would consume
        significant memory before the check ever fires.

        Consider adding a COUNT query before the full data fetch,
        or using a streaming approach with an early-exit condition.
```

---

## 1.12 Practical Walkthrough: Setting Up a 3-Agent Pipeline for a Feature Build

Let us walk through a complete, practical example from start to finish. We will build a user notification preferences feature.

### Step 1: Define the Orchestrator

```python
# orchestrator.py -- The pipeline coordinator

import yaml
import json
from pathlib import Path
from typing import Optional
from agents import ArchitectAgent, BuilderAgent, CriticAgent

class SDDPipeline:
    """
    Orchestrates the Architect -> Builder -> Critic pipeline.
    """

    def __init__(
        self,
        project_root: Path,
        spec_dir: Path,
        max_iterations: int = 3,
    ):
        self.project_root = project_root
        self.spec_dir = spec_dir
        self.max_iterations = max_iterations

        self.architect = ArchitectAgent(
            spec_dir=spec_dir,
            schema_path=spec_dir / "schema.yaml",
        )
        self.builder = BuilderAgent(
            project_root=project_root,
            conventions_path=project_root / "CONVENTIONS.md",
        )
        self.critic = CriticAgent(
            spec_dir=spec_dir,
            project_root=project_root,
        )

    def run(self, requirements: str) -> dict:
        """Execute the full SDD pipeline."""
        history = []
        critic_feedback: Optional[dict] = None

        for iteration in range(self.max_iterations):
            print(f"\n{'='*60}")
            print(f"ITERATION {iteration + 1}")
            print(f"{'='*60}")

            # --- ARCHITECT PHASE ---
            print("\n[ARCHITECT] Writing specification...")
            spec = self.architect.generate_spec(
                requirements=requirements,
                critic_feedback=critic_feedback,
            )
            print(f"[ARCHITECT] Spec generated: {spec['feature']}")

            # Validate spec against schema
            schema_errors = self.architect.validate_schema(spec)
            if schema_errors:
                print(f"[ARCHITECT] Schema errors: {schema_errors}")
                # Architect self-corrects schema issues
                spec = self.architect.fix_schema_errors(spec, schema_errors)

            # Save spec to disk
            spec_path = self.spec_dir / f"{spec['feature']}.spec.yaml"
            with open(spec_path, 'w') as f:
                yaml.dump(spec, f, default_flow_style=False)

            # --- BUILDER PHASE ---
            print("\n[BUILDER] Implementing specification...")
            build_result = self.builder.implement(spec=spec)
            print(f"[BUILDER] Files created: {len(build_result['files_created'])}")
            print(f"[BUILDER] Tests created: {len(build_result['tests_created'])}")

            if build_result.get("ambiguities"):
                print(f"[BUILDER] Ambiguities found: {len(build_result['ambiguities'])}")
                # Feed ambiguities back as critic feedback
                critic_feedback = {
                    "verdict": "FAIL",
                    "issues": [
                        {
                            "severity": "high",
                            "category": "ambiguity",
                            "description": amb["description"],
                            "spec_section": amb["section"],
                        }
                        for amb in build_result["ambiguities"]
                    ],
                }
                continue

            # --- CRITIC PHASE ---
            print("\n[CRITIC] Validating implementation...")
            validation = self.critic.validate(
                spec=spec,
                implementation=build_result,
            )
            print(f"[CRITIC] Verdict: {validation['verdict']}")

            history.append({
                "iteration": iteration + 1,
                "spec_version": spec.get("spec_version"),
                "verdict": validation["verdict"],
                "issue_count": len(validation.get("issues", [])),
            })

            if validation["verdict"] in ("PASS", "PASS_WITH_WARNINGS"):
                return {
                    "status": "success",
                    "iterations": iteration + 1,
                    "spec_path": str(spec_path),
                    "files": build_result["files_created"],
                    "tests": build_result["tests_created"],
                    "validation": validation,
                    "history": history,
                }

            # Feed issues back to Architect
            critic_feedback = validation
            print(f"[CRITIC] Sending {len(validation['issues'])} issues to Architect")

        return {
            "status": "max_iterations_reached",
            "iterations": self.max_iterations,
            "history": history,
            "final_validation": validation,
        }
```

### Step 2: Run the Pipeline

```python
# run_pipeline.py

from pathlib import Path
from orchestrator import SDDPipeline

pipeline = SDDPipeline(
    project_root=Path("./my-project"),
    spec_dir=Path("./my-project/specs"),
    max_iterations=3,
)

result = pipeline.run(
    requirements="""
    Users need to manage their notification preferences.
    They should be able to:
    - Toggle email notifications on/off
    - Toggle push notifications on/off
    - Set quiet hours (time range when no notifications are sent)
    - Choose notification frequency: immediate, hourly digest, daily digest
    The preferences should persist across sessions and sync across devices.
    """
)

print(f"\nPipeline completed: {result['status']}")
print(f"Iterations needed: {result['iterations']}")
if result['status'] == 'success':
    print(f"Files created: {result['files']}")
```

### Step 3: Observe the Pipeline Execution

Here is what a typical run looks like:

```
============================================================
ITERATION 1
============================================================

[ARCHITECT] Writing specification...
[ARCHITECT] Spec generated: notification-preferences

[BUILDER] Implementing specification...
[BUILDER] Files created: 4
[BUILDER] Tests created: 2

[CRITIC] Validating implementation...
[CRITIC] Verdict: FAIL
[CRITIC] Sending 2 issues to Architect:
  - ISSUE-001 (high): Spec does not define behavior when quiet hours
    span midnight (e.g., 22:00-06:00). Builder assumed wrapping behavior
    but this should be explicit.
  - ISSUE-002 (medium): Spec says "sync across devices" but does not
    specify conflict resolution when two devices set different preferences
    simultaneously.

============================================================
ITERATION 2
============================================================

[ARCHITECT] Writing specification...
  -> Addressing ISSUE-001: Added explicit midnight-spanning behavior
  -> Addressing ISSUE-002: Added last-write-wins conflict resolution
[ARCHITECT] Spec generated: notification-preferences (v1.0.1)

[BUILDER] Implementing specification...
[BUILDER] Files created: 4 (updated)
[BUILDER] Tests created: 3 (added midnight-spanning test)

[CRITIC] Validating implementation...
[CRITIC] Verdict: PASS

Pipeline completed: success
Iterations needed: 2
```

> **Professor's aside:** Two iterations. That is typical. The first pass catches the big issues -- the things the Architect did not think of. The second pass is usually clean. If you are regularly going to three or more iterations, your Architect Agent needs better prompting or your spec schema is not comprehensive enough.

---

## 1.13 Error Handling Between Agents: When the Builder Cannot Fulfill the Spec

What happens when the Builder Agent encounters a spec it cannot implement? This is not a failure -- it is a feature of the system. Here are the categories of Builder failures and how to handle them:

### Category 1: Spec Ambiguity

The spec is unclear about what should happen.

```yaml
# Builder's ambiguity report
ambiguity_report:
  spec: "notification-preferences.spec.yaml"
  ambiguities:
    - section: "constraints[3]"
      spec_text: "Preferences should sync across devices"
      problem: "No conflict resolution strategy specified"
      interpretations:
        - "Last write wins (most recent timestamp)"
        - "Per-field merge (each field independently takes latest value)"
        - "Prompt user to resolve conflicts"
      recommendation: "Per-field merge -- most granular, least data loss"
```

**Resolution path:** Ambiguity report goes to the Architect. The Architect updates the spec. The Builder re-executes.

### Category 2: Technical Impossibility

The spec requests something that cannot be done within the stated constraints.

```yaml
# Builder's impossibility report
impossibility_report:
  spec: "notification-preferences.spec.yaml"
  issue:
    spec_requirement: "constraints[5]: All operations must complete within 50ms"
    problem: |
      The spec requires syncing preferences to three external services
      (email provider, push service, analytics) on every save.
      Network round trips alone exceed the 50ms constraint.
    possible_resolutions:
      - "Relax the latency constraint to 500ms"
      - "Make external syncs asynchronous (save locally in 50ms, sync in background)"
      - "Remove the external sync requirement from this spec"
```

**Resolution path:** Impossibility report goes to the Architect. The Architect either relaxes the constraint, changes the approach, or escalates to a human.

### Category 3: Missing Dependencies

The spec references something that does not exist yet.

```yaml
# Builder's dependency report
dependency_report:
  spec: "notification-preferences.spec.yaml"
  missing_dependencies:
    - dependency: "auth/session-management.spec.yaml"
      referenced_in: "inputs[0]"
      status: "spec exists but not implemented"
      blocks: "Cannot validate authenticated sessions"
    - dependency: "PushNotificationService"
      referenced_in: "constraints[2]"
      status: "no spec or implementation found"
      blocks: "Cannot send push notifications"
```

**Resolution path:** Dependency report goes to the Orchestrator, which may need to trigger separate pipelines for the dependencies.

---

## 1.14 Agent Memory and State: Specs as Shared Memory

One of the most elegant properties of the SDD multi-agent pattern is how it solves the memory problem.

AI agents have limited context windows. They forget. They lose track of decisions made earlier. In a multi-agent system, this problem compounds: Agent A makes a decision, Agent B needs to know about it, but Agent B's context does not include Agent A's reasoning.

**The spec is the solution.** It serves as **externalized shared memory** between agents.

```
Traditional Multi-Agent Memory Problem:
  Agent A decides X -> (context lost) -> Agent B does not know about X

SDD Multi-Agent Memory Solution:
  Agent A decides X -> writes X into spec -> Agent B reads spec -> knows X
```

### What the Spec Captures as Shared Memory

| Decision Type | Where in the Spec | Example |
|--------------|-------------------|---------|
| Functional requirements | `purpose`, `acceptance_criteria` | "Users can export CSV" |
| Data contracts | `inputs`, `outputs` | Field names, types, formats |
| Behavioral constraints | `constraints` | "Max 50,000 rows" |
| Error behavior | `error_cases` | "Return 413 for too many rows" |
| Cross-cutting concerns | `dependencies` | "Requires auth spec v2.1" |
| Open issues | `open_questions` | "TBD: conflict resolution strategy" |
| Change history | `changelog` | "v1.0.1: added midnight-span handling" |

This means you can pause the pipeline, swap out the Builder Agent for a different one, and the new Builder has everything it needs in the spec. You can run the Critic Agent days later, against a different version of the code, and it still knows what the spec demands.

> **Professor's aside:** This is why I keep hammering on spec completeness. An incomplete spec is not just a bad input to the Builder -- it is a memory gap that affects every agent downstream. If the Architect forgets to specify error handling, the Builder guesses, the Critic has nothing to validate against, and you end up with silent failures in production.

---

## 1.15 The Feedback Loop: Why the Critic Sends Issues to the Architect, Not the Builder

This is a design decision that trips people up, so let me explain it carefully.

When the Critic finds that the code does not match the spec, the natural instinct is: "Send the issue to the Builder. The Builder wrote the code, so the Builder should fix it."

**This is wrong.** Here is why.

### Scenario: Builder Receives Critic Feedback Directly

```
Critic says: "The code doesn't handle the case where quiet hours span midnight."

Builder thinks: "The spec didn't mention midnight-spanning. I'll add handling for it."

Builder adds code to handle midnight-spanning quiet hours.

BUT: The spec still doesn't mention midnight-spanning behavior.
```

**What went wrong?** The Builder just introduced behavior that is not in the spec. Now the spec and the code are out of sync. The spec -- which is supposed to be the single source of truth -- is incomplete. If a new Builder Agent takes over, it will not know about the midnight-spanning behavior because it is not in the spec.

### Scenario: Architect Receives Critic Feedback (Correct Flow)

```
Critic says: "The code doesn't handle the case where quiet hours span midnight."

Architect thinks: "The spec needs to define midnight-spanning behavior."

Architect updates the spec with explicit midnight-spanning rules.

Builder receives updated spec and implements the now-specified behavior.
```

**What went right?** The spec remains the single source of truth. The midnight-spanning behavior is now documented, specified, and testable. Any future Builder Agent will know about it.

### The Principle

**The spec must always be ahead of the code.** Every behavior in the code must trace back to a spec requirement. If the Critic finds a missing behavior, it means the **spec** is incomplete, not just the code.

```
                    +-------------------+
                    |   CRITIC AGENT    |
                    +-------------------+
                            |
                     FAIL: Issues found
                            |
                            v
                 +-----------------------+
                 |  Issues go to         |
                 |  ARCHITECT (not       |
                 |  Builder) because     |
                 |  the spec must be     |
                 |  updated first        |
                 +-----------------------+
                            |
                            v
                  +-------------------+
                  |  ARCHITECT AGENT  |
                  |  Updates the spec |
                  +-------------------+
                            |
                       Updated spec
                            |
                            v
                  +-------------------+
                  |   BUILDER AGENT   |
                  |  Re-implements    |
                  +-------------------+
```

---

## 1.16 Real Code Examples of Multi-Agent Orchestration

Let us put it all together with a production-grade orchestration example using TypeScript:

```typescript
// sdd-pipeline/src/orchestrator.ts

import { z } from 'zod';
import { EventEmitter } from 'events';

// ============================================================
// Type Definitions
// ============================================================

const SpecSchema = z.object({
  spec_version: z.string(),
  feature: z.string(),
  status: z.enum(['draft', 'approved', 'implemented', 'deprecated']),
  purpose: z.string(),
  inputs: z.array(z.object({
    name: z.string(),
    type: z.string(),
    required: z.boolean(),
    description: z.string(),
  })),
  outputs: z.array(z.object({
    name: z.string(),
    type: z.string(),
    description: z.string(),
  })),
  constraints: z.array(z.string()),
  error_cases: z.array(z.object({
    condition: z.string(),
    behavior: z.string(),
  })),
  acceptance_criteria: z.array(z.string()),
});

type Spec = z.infer<typeof SpecSchema>;

interface BuildResult {
  files: Record<string, string>;     // path -> content
  tests: Record<string, string>;     // path -> content
  ambiguities: AmbiguityReport[];
  buildNotes: string;
}

interface AmbiguityReport {
  section: string;
  description: string;
  interpretations: string[];
}

interface ValidationIssue {
  id: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  category: 'completeness' | 'correctness' | 'consistency' | 'coverage';
  specRequirement: string;
  description: string;
  file?: string;
  line?: number;
  suggestion?: string;
}

interface ValidationReport {
  verdict: 'PASS' | 'FAIL' | 'PASS_WITH_WARNINGS';
  issues: ValidationIssue[];
  completeness: { total: number; implemented: number };
  testCoverage: { criteria: number; tested: number };
}

// ============================================================
// Agent Interfaces
// ============================================================

interface AgentProvider {
  chat(systemPrompt: string, userMessage: string): Promise<string>;
}

// ============================================================
// The Orchestrator
// ============================================================

class SDDOrchestrator extends EventEmitter {
  private provider: AgentProvider;
  private maxIterations: number;

  constructor(provider: AgentProvider, maxIterations = 3) {
    super();
    this.provider = provider;
    this.maxIterations = maxIterations;
  }

  async execute(requirements: string): Promise<{
    status: 'success' | 'failed';
    iterations: number;
    spec: Spec | null;
    code: BuildResult | null;
    validation: ValidationReport | null;
  }> {
    let spec: Spec | null = null;
    let code: BuildResult | null = null;
    let validation: ValidationReport | null = null;
    let criticFeedback: ValidationReport | null = null;

    for (let i = 0; i < this.maxIterations; i++) {
      this.emit('iteration:start', { iteration: i + 1 });

      // ----- ARCHITECT PHASE -----
      this.emit('phase:start', { phase: 'architect', iteration: i + 1 });

      const architectPrompt = this.buildArchitectPrompt(
        requirements,
        criticFeedback,
      );
      const specYaml = await this.provider.chat(
        ARCHITECT_SYSTEM_PROMPT,
        architectPrompt,
      );

      try {
        spec = SpecSchema.parse(this.parseYaml(specYaml));
      } catch (parseError) {
        this.emit('error', {
          phase: 'architect',
          error: 'Spec did not conform to schema',
          details: parseError,
        });
        continue; // Retry iteration
      }

      this.emit('phase:complete', {
        phase: 'architect',
        result: { feature: spec.feature, version: spec.spec_version },
      });

      // ----- BUILDER PHASE -----
      this.emit('phase:start', { phase: 'builder', iteration: i + 1 });

      const builderPrompt = this.buildBuilderPrompt(spec);
      const buildOutput = await this.provider.chat(
        BUILDER_SYSTEM_PROMPT,
        builderPrompt,
      );
      code = this.parseBuildResult(buildOutput);

      if (code.ambiguities.length > 0) {
        this.emit('ambiguity', {
          count: code.ambiguities.length,
          details: code.ambiguities,
        });
        // Convert ambiguities to critic feedback format
        criticFeedback = {
          verdict: 'FAIL',
          issues: code.ambiguities.map((amb, idx) => ({
            id: `AMB-${String(idx + 1).padStart(3, '0')}`,
            severity: 'high' as const,
            category: 'completeness' as const,
            specRequirement: amb.section,
            description: amb.description,
          })),
          completeness: { total: 0, implemented: 0 },
          testCoverage: { criteria: 0, tested: 0 },
        };
        continue;
      }

      this.emit('phase:complete', {
        phase: 'builder',
        result: {
          filesCreated: Object.keys(code.files).length,
          testsCreated: Object.keys(code.tests).length,
        },
      });

      // ----- CRITIC PHASE -----
      this.emit('phase:start', { phase: 'critic', iteration: i + 1 });

      const criticPrompt = this.buildCriticPrompt(spec, code);
      const validationOutput = await this.provider.chat(
        CRITIC_SYSTEM_PROMPT,
        criticPrompt,
      );
      validation = this.parseValidationReport(validationOutput);

      this.emit('phase:complete', {
        phase: 'critic',
        result: {
          verdict: validation.verdict,
          issueCount: validation.issues.length,
        },
      });

      if (validation.verdict === 'PASS'
        || validation.verdict === 'PASS_WITH_WARNINGS') {
        return {
          status: 'success',
          iterations: i + 1,
          spec,
          code,
          validation,
        };
      }

      // Feed issues back to Architect
      criticFeedback = validation;
      this.emit('feedback', {
        fromPhase: 'critic',
        toPhase: 'architect',
        issueCount: validation.issues.length,
      });
    }

    return {
      status: 'failed',
      iterations: this.maxIterations,
      spec,
      code,
      validation,
    };
  }

  private buildArchitectPrompt(
    requirements: string,
    feedback: ValidationReport | null,
  ): string {
    let prompt = `## Requirements\n\n${requirements}`;
    if (feedback) {
      prompt += `\n\n## Issues from Previous Review\n\n`;
      prompt += `The Critic found ${feedback.issues.length} issues:\n\n`;
      for (const issue of feedback.issues) {
        prompt += `- [${issue.severity}] ${issue.description}\n`;
        prompt += `  Spec section: ${issue.specRequirement}\n`;
        if (issue.suggestion) {
          prompt += `  Suggestion: ${issue.suggestion}\n`;
        }
        prompt += '\n';
      }
      prompt += `Please update the spec to address ALL of these issues.`;
    }
    return prompt;
  }

  private buildBuilderPrompt(spec: Spec): string {
    return `## Specification\n\n\`\`\`yaml\n${JSON.stringify(spec, null, 2)}\n\`\`\`\n\nImplement this specification. Generate code and tests.`;
  }

  private buildCriticPrompt(spec: Spec, build: BuildResult): string {
    let prompt = `## Specification\n\n\`\`\`yaml\n${JSON.stringify(spec, null, 2)}\n\`\`\`\n\n`;
    prompt += `## Implementation\n\n`;
    for (const [path, content] of Object.entries(build.files)) {
      prompt += `### ${path}\n\`\`\`\n${content}\n\`\`\`\n\n`;
    }
    prompt += `## Tests\n\n`;
    for (const [path, content] of Object.entries(build.tests)) {
      prompt += `### ${path}\n\`\`\`\n${content}\n\`\`\`\n\n`;
    }
    prompt += `Validate the implementation against every spec requirement.`;
    return prompt;
  }

  // Parsing methods would handle extracting structured data
  // from the LLM's response -- omitted for brevity
  private parseYaml(raw: string): unknown { /* ... */ return {}; }
  private parseBuildResult(raw: string): BuildResult { /* ... */ return {} as any; }
  private parseValidationReport(raw: string): ValidationReport { /* ... */ return {} as any; }
}

// ============================================================
// System Prompts
// ============================================================

const ARCHITECT_SYSTEM_PROMPT = `You are the Architect Agent.
Write detailed YAML specifications from requirements.
Never write implementation code.
Output only valid YAML conforming to the spec schema.
Address all Critic feedback when provided.`;

const BUILDER_SYSTEM_PROMPT = `You are the Builder Agent.
Implement code from YAML specifications.
Generate tests for every acceptance criterion.
Report ambiguities rather than guessing.
Follow project coding conventions.`;

const CRITIC_SYSTEM_PROMPT = `You are the Critic Agent.
Validate code against the specification.
Check completeness, correctness, and test coverage.
Output a structured validation report.
Be thorough but fair -- only flag real issues.`;

export { SDDOrchestrator };
```

### Using the Orchestrator

```typescript
// run.ts

import { SDDOrchestrator } from './orchestrator';
import { AnthropicProvider } from './providers/anthropic';

const provider = new AnthropicProvider({
  apiKey: process.env.ANTHROPIC_API_KEY!,
  model: 'claude-sonnet-4-20250514',
});

const pipeline = new SDDOrchestrator(provider, 3);

// Listen to pipeline events
pipeline.on('iteration:start', ({ iteration }) => {
  console.log(`\n${'='.repeat(60)}`);
  console.log(`ITERATION ${iteration}`);
  console.log(`${'='.repeat(60)}`);
});

pipeline.on('phase:start', ({ phase, iteration }) => {
  console.log(`\n[${phase.toUpperCase()}] Starting...`);
});

pipeline.on('phase:complete', ({ phase, result }) => {
  console.log(`[${phase.toUpperCase()}] Complete:`, result);
});

pipeline.on('feedback', ({ fromPhase, toPhase, issueCount }) => {
  console.log(
    `[FEEDBACK] ${fromPhase} -> ${toPhase}: ${issueCount} issues`,
  );
});

// Execute
const result = await pipeline.execute(`
  Users need to manage their notification preferences.
  They should be able to:
  - Toggle email notifications on/off
  - Toggle push notifications on/off
  - Set quiet hours (time range when no notifications are sent)
  - Choose notification frequency: immediate, hourly, daily digest
  Preferences must persist and sync across devices.
`);

console.log(`\nResult: ${result.status}`);
console.log(`Iterations: ${result.iterations}`);
```

---

## 1.17 Putting It All Together: Key Takeaways

Let us summarize the core lessons from this chapter:

1. **The three-agent pattern (Architect, Builder, Critic) mirrors the natural division of labor in software teams.** It separates concern for *what* to build, *how* to build it, and *whether it was built correctly*.

2. **The spec is the coordination mechanism.** It is shared memory, communication protocol, and contract all in one. Without it, multi-agent workflows devolve into chaos.

3. **The Critic sends feedback to the Architect, not the Builder.** This ensures the spec remains the single source of truth and that every behavior is documented before it is implemented.

4. **Every major AI company is converging on this pattern.** Anthropic's Claude Code (Task tool), Google's Gemini (multi-step reasoning), OpenAI's agent frameworks (Assistants, Swarm), Meta's Llama (open-source agents), and Microsoft's AutoGen all implement variations of the Architect-Builder-Critic workflow.

5. **Error handling between agents is a first-class concern.** Ambiguity reports, impossibility reports, and dependency reports are not failures -- they are essential communication channels that make the system self-correcting.

6. **Hybrid communication (structured data + natural language) is the practical choice** for inter-agent communication in 2026.

> **Professor's aside:** If you take away one thing from this chapter, let it be this: the quality of your multi-agent system is bounded by the quality of your specifications. Invest in the spec, and the agents will take care of the rest. Skimp on the spec, and no amount of agent sophistication will save you.

---

### Exercises

**Exercise 1:** Design a three-agent pipeline for a different feature: user authentication with OAuth2. Write the Architect Agent's system prompt, the spec it would produce, and the validation criteria the Critic would check.

**Exercise 2:** Modify the orchestrator code to support parallel Builder Agents -- where a complex spec is decomposed into sub-specs that can be built concurrently. What coordination challenges arise?

**Exercise 3:** Implement the "impossible spec" scenario: write a spec with contradictory constraints and observe how each agent (Architect, Builder, Critic) should handle the contradiction. Which agent should catch it first?

---

*Next chapter: "Evolutionary Specs" -- how to manage specifications that change over time without breaking everything.*

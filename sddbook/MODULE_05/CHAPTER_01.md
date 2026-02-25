# Chapter 1: The Refactor Spec

## MODULE 05 — Maintenance & Scaling (Advanced Level)

---

### Lecture Preamble

> *Welcome back. If you have made it to Module 5, you have already proven something important about yourself: you think in systems, not in prompts. You have learned how to write specs from scratch, how to validate them, how to orchestrate multi-agent workflows around them, and how to version them as requirements evolve. But now we confront a reality that every working engineer faces eventually: the code already exists, it is messy, and nobody wrote a spec for it.*

> *This is the most common situation you will encounter in professional software development. Greenfield projects are rare. The vast majority of your career will be spent working with code that someone else wrote — or code that you wrote six months ago, which might as well have been written by a stranger. The question is not whether you will face legacy code. The question is whether you will approach it with a plan or with a prayer.*

> *Today we learn how to write the Refactor Spec — the most demanding and arguably the most valuable type of specification in the SDD toolkit. This is where craft meets archaeology, where patience meets precision, and where the true Product Architects separate themselves from the prompt engineers.*

---

## 1.1 Why Refactoring Needs a Spec

Let us start with an uncomfortable truth: most refactors fail. Not because the engineers lack skill, but because they lack clarity about what they are actually trying to accomplish. They dive into the code, start "cleaning things up," and three weeks later the application is in a worse state than when they started. Features are broken. Tests are red. Deadlines have slipped. And the team is demoralized.

This happens because refactoring without a spec is like renovating a house without blueprints. You might know you want to "make the kitchen bigger," but without a plan for load-bearing walls, plumbing, and electrical, you are going to end up with a pile of rubble.

The Refactor Spec solves this by forcing you to answer three questions before you touch a single line of code:

1. **What exists right now?** (The Archaeology Phase)
2. **What should exist when we are done?** (The Target Architecture)
3. **How do we get from here to there without breaking anything?** (The Migration Path)

> **Professor's Aside:** I cannot stress this enough — the number one cause of failed refactors is skipping step one. Engineers are optimists by nature. They want to build the new thing. But if you do not understand the old thing thoroughly, you will miss hidden dependencies, undocumented behaviors, and implicit contracts that the system relies on. Respect the archaeology.

---

## 1.2 The Archaeology Phase: Understanding What Exists

Before you can specify what should change, you must understand what exists. This is the archaeology phase, and it is the most intellectually demanding part of writing a refactor spec. You are not just reading code — you are reconstructing intent from artifacts.

### 1.2.1 The Four Layers of Code Archaeology

When you approach an existing codebase, you need to excavate four distinct layers:

**Layer 1: The Structural Layer**
What files exist? How are they organized? What are the dependency relationships?

```bash
# Generate a dependency graph of an Express.js project
# This is your first archaeological tool
npx madge --image dependency-graph.svg src/

# Count lines of code by file type
find src/ -name "*.ts" -o -name "*.js" | xargs wc -l | sort -n

# List all route definitions
grep -rn "router\.\(get\|post\|put\|delete\|patch\)" src/
```

**Layer 2: The Behavioral Layer**
What does the system actually do? Not what the comments say it does, not what the README claims — what does it actually do when requests come in?

```typescript
// Archaeological tool: request logging middleware
// Deploy this BEFORE you start specifying changes
import { Request, Response, NextFunction } from 'express';

interface RequestLog {
  timestamp: string;
  method: string;
  path: string;
  queryParams: Record<string, unknown>;
  bodyShape: Record<string, string>;  // key -> typeof value
  responseStatus: number;
  responseTime: number;
  headers: Record<string, string>;
}

const archaeologyMiddleware = (
  req: Request,
  res: Response,
  next: NextFunction
): void => {
  const start = Date.now();
  const originalSend = res.send;

  res.send = function (body: any) {
    const log: RequestLog = {
      timestamp: new Date().toISOString(),
      method: req.method,
      path: req.path,
      queryParams: req.query as Record<string, unknown>,
      bodyShape: Object.fromEntries(
        Object.entries(req.body || {}).map(([k, v]) => [k, typeof v])
      ),
      responseStatus: res.statusCode,
      responseTime: Date.now() - start,
      headers: req.headers as Record<string, string>,
    };

    // Write to archaeology log — NOT to production logs
    appendToArchaeologyLog(log);
    return originalSend.call(this, body);
  };

  next();
};
```

**Layer 3: The Contractual Layer**
What implicit and explicit contracts exist? Who calls this code? What do they expect?

```typescript
// Document discovered contracts in your archaeology notes
interface DiscoveredContract {
  endpoint: string;
  consumers: string[];          // Who calls this?
  expectedInputShape: object;   // What do they send?
  expectedOutputShape: object;  // What do they expect back?
  errorBehaviors: string[];     // What happens on failure?
  sideEffects: string[];        // Database writes, emails, etc.
  undocumentedBehaviors: string[]; // Surprises you found
}
```

**Layer 4: The Historical Layer**
Why does the code look this way? What decisions were made, and what constraints existed at the time?

```bash
# Git archaeology: who changed what and when
git log --oneline --all --graph -- src/api/users.ts

# Find the commit that introduced a suspicious pattern
git log -S "workaround" --oneline

# Show the full context of when a file was created
git log --diff-filter=A -- src/utils/legacyHelper.ts
```

### 1.2.2 The Archaeology Report

The output of your archaeology phase should be a structured document — the Archaeology Report. This becomes the foundation for your Refactor Spec.

```markdown
# Archaeology Report: User Service API

## Date: 2026-02-24
## Archaeologist: [Your Name]
## Scope: src/api/users/, src/services/userService.ts, src/models/user.ts

### System Overview
The User Service handles CRUD operations for user accounts.
It was originally written in 2023 using Express.js with raw SQL queries.
A partial migration to Prisma ORM was attempted in 2024 but abandoned.

### Structural Findings
- 14 route handlers across 3 files (should be 1)
- Mixed use of callbacks and async/await
- No consistent error handling pattern
- Two competing user models: `User` (Prisma) and `UserLegacy` (raw SQL)

### Behavioral Findings (from 7-day request log)
- 47 unique endpoints discovered (docs list only 31)
- 16 undocumented endpoints still receiving traffic
- Average response time: 340ms (target: <100ms)
- 3 endpoints return inconsistent shapes based on query params

### Contractual Findings
- Mobile app v2.1 depends on legacy field `user.fullName` (deprecated)
- Admin dashboard calls 4 undocumented endpoints
- Webhook system assumes synchronous user creation

### Historical Findings
- Original author left company in 2024
- Prisma migration abandoned due to deadline pressure (see commit a3f7b2c)
- "Temporary" workaround for auth added 2023-11-15, never removed

### Risk Assessment
- HIGH: 16 undocumented endpoints — consumers unknown
- MEDIUM: Mixed ORM state — partial Prisma, partial raw SQL
- LOW: Inconsistent error handling — no known production issues
```

> **Professor's Aside:** Notice that the archaeology report is not a spec yet. It is evidence. It is facts about the world as it exists. The spec comes next, and it will be informed by these facts. Too many engineers skip straight to "here is what I want it to look like" without first understanding "here is what it actually looks like." That is how you break things.

---

## 1.3 The Reverse Spec Technique

Once your archaeology is complete, you have a powerful technique available to you: the reverse spec. Instead of writing a spec for what you want to build and then building it, you write a spec that describes what already exists — as if you had written the spec before the code.

Why is this useful? Because it makes the gap between "what is" and "what should be" explicit and measurable.

```typescript
// REVERSE SPEC: What the User Service currently does
// (Not what it should do — what it actually does right now)

/**
 * @reverse-spec UserService
 * @status DISCOVERED (not designed)
 * @archaeology-date 2026-02-24
 */
interface UserServiceCurrentBehavior {
  // === DOCUMENTED ENDPOINTS ===

  /**
   * GET /api/users
   * Returns paginated user list
   * @note Pagination is broken — returns all users if page param missing
   * @note Response shape changes if `?legacy=true` is passed
   */
  listUsers(params: {
    page?: number;      // Optional — defaults to returning ALL (bug)
    limit?: number;     // Optional — defaults to 50
    legacy?: boolean;   // Undocumented flag that changes response shape
  }): Promise<UserListResponse | LegacyUserListResponse>;

  /**
   * POST /api/users
   * Creates a new user
   * @note Synchronous — blocks until DB write completes
   * @note Sends welcome email inline (not queued)
   * @note No input validation on email field
   */
  createUser(data: {
    email: string;      // Not validated — accepts malformed emails
    name: string;
    password: string;   // Stored as bcrypt hash (good)
    role?: string;      // Defaults to "user" — no enum validation
  }): Promise<User>;

  // === UNDOCUMENTED ENDPOINTS ===

  /**
   * GET /api/users/search
   * @undocumented — Not in API docs
   * @consumers Admin Dashboard (confirmed), possibly others
   * Full-text search across user records
   * @warning Uses raw SQL — potential injection vulnerability
   */
  searchUsers(query: string): Promise<User[]>;

  // ... (16 more undocumented endpoints)
}
```

Now compare this reverse spec with what the system *should* look like:

```typescript
// TARGET SPEC: What the User Service should do after refactor

/**
 * @spec UserService v2.0
 * @migration-from UserServiceCurrentBehavior
 * @target-date 2026-Q2
 */
interface UserServiceTargetBehavior {
  /**
   * GET /api/v2/users
   * Returns paginated user list
   * @requires Pagination is mandatory — no full-table scans
   * @validation page >= 1, limit between 1 and 100
   */
  listUsers(params: {
    page: number;       // Required — enforced
    limit: number;      // Required — enforced, max 100
  }): Promise<PaginatedResponse<UserDTO>>;

  /**
   * POST /api/v2/users
   * Creates a new user
   * @requires Async processing — returns 202 Accepted
   * @requires Email validation via regex + MX record check
   * @requires Welcome email queued via job system
   */
  createUser(data: CreateUserDTO): Promise<{
    id: string;
    status: 'pending';
  }>;

  /**
   * GET /api/v2/users/search
   * @requires Parameterized queries only — no raw SQL
   * @requires Rate limiting: 10 requests/minute per client
   */
  searchUsers(params: SearchParams): Promise<PaginatedResponse<UserDTO>>;
}
```

The gap between these two specs IS your refactor scope. Every difference is a work item. Every matching element is something you do not need to touch.

---

## 1.4 Scoping the Refactor: What to Touch and What to Leave Alone

One of the hardest skills in refactoring is knowing when to stop. The temptation to "fix everything while we are in here" is strong, and it is a trap. Every additional change increases risk, extends timelines, and complicates testing.

### The Scoping Matrix

Use this matrix to classify every potential change:

```
                    HIGH IMPACT
                        |
         QUADRANT 2     |     QUADRANT 1
         "Schedule It"  |     "Do It Now"
                        |
  LOW EFFORT -----------+------------ HIGH EFFORT
                        |
         QUADRANT 3     |     QUADRANT 4
         "Maybe Never"  |     "Think Twice"
                        |
                    LOW IMPACT
```

**Quadrant 1 (High Impact, High Effort):** These are your core refactor items. They justify the refactor's existence. Spec them thoroughly.

**Quadrant 2 (High Impact, Low Effort):** Quick wins. Include them in the refactor spec but keep them separate from core changes so they can be shipped independently.

**Quadrant 3 (Low Impact, Low Effort):** Tempting to include "while we are in here." Resist. Log them as future improvements. They dilute focus.

**Quadrant 4 (Low Impact, High Effort):** Absolutely not. These are the refactor-killers. Someone will argue "we should just do it now since we are already touching that code." The answer is no.

### Scoping in the Spec

Your refactor spec must have an explicit scope section:

```markdown
## Refactor Scope

### IN SCOPE (Will be changed in this refactor)
- [ ] Migrate all user endpoints from Express routes to NestJS controllers
- [ ] Replace raw SQL queries with Prisma ORM
- [ ] Implement proper input validation using class-validator
- [ ] Add consistent error handling with NestJS exception filters
- [ ] Migrate 16 undocumented endpoints to documented v2 API

### OUT OF SCOPE (Explicitly will NOT be changed)
- [ ] Authentication system (working correctly, separate refactor planned)
- [ ] Database schema (no schema changes in this refactor)
- [ ] Frontend API client code (will be updated in a follow-up)
- [ ] Email sending infrastructure (works, just needs to be async)

### BOUNDARY CONDITIONS
- Legacy v1 endpoints must remain functional during migration
- No downtime during migration — both v1 and v2 run simultaneously
- Performance must not degrade — target p99 latency <= 200ms
```

> **Professor's Aside:** The "Out of Scope" section is arguably more important than the "In Scope" section. It is your shield against scope creep. When someone says "while you are in there, could you also..." you point to the spec. It is not in scope. It was explicitly excluded. If they want it, they can write a separate spec for it.

---

## 1.5 The Strangler Fig Pattern Applied to Specs

The Strangler Fig pattern, originally described by Martin Fowler, is one of the most powerful techniques for large-scale refactoring. The idea is simple: instead of replacing a system all at once, you gradually grow a new system around the old one, routing more and more traffic to the new system until the old one can be removed entirely.

Applied to specs, this means you write a series of incremental specs, each one strangling a piece of the old system:

```markdown
# Strangler Fig Spec Series: User Service Migration

## Phase 1 Spec: The Facade (Week 1-2)
### Objective
Stand up a NestJS application that proxies ALL requests to the existing
Express.js service. Zero behavior change. Pure passthrough.

### Success Criteria
- All existing tests pass against the new facade
- Response times increase by no more than 10ms (proxy overhead)
- No client-visible changes

### What Changes
- New NestJS app deployed alongside Express app
- Load balancer routes to NestJS
- NestJS proxies all requests to Express

### What Does Not Change
- Express app continues running
- All business logic remains in Express
- No data model changes

---

## Phase 2 Spec: First Endpoint Migration (Week 3-4)
### Objective
Migrate GET /api/users (list users) to native NestJS implementation.
All other endpoints continue to proxy to Express.

### Success Criteria
- GET /api/users served natively by NestJS
- Response shape identical to Express version
- Response time improved (target: <100ms, currently 340ms)
- All other endpoints still proxied successfully

---

## Phase 3 Spec: CRUD Migration (Week 5-8)
### Objective
Migrate all documented CRUD endpoints to native NestJS.
Undocumented endpoints continue to proxy.

---

## Phase 4 Spec: Undocumented Endpoint Resolution (Week 9-10)
### Objective
For each of the 16 undocumented endpoints:
- Confirm active consumers
- Either migrate to v2 API or deprecate with notice

---

## Phase 5 Spec: Express Decommission (Week 11-12)
### Objective
Remove the Express application entirely.
All traffic served natively by NestJS.

### Prerequisites
- Zero requests proxied to Express for 7 consecutive days
- All consumer teams have confirmed migration to v2 endpoints
```

Each phase spec is a complete, standalone specification. It has its own success criteria, its own testing plan, and its own rollback strategy. If any phase fails, you can stop the migration at that point and the system is still functional.

### Why This Works With AI

The Strangler Fig approach is particularly well-suited to AI-assisted development because each phase spec is small enough to fit within a single context window. You are not asking the AI to understand the entire migration at once — you are asking it to execute one well-defined phase at a time.

```typescript
// Phase 1: The Facade — this is what you hand to the AI

// spec: nestjs-facade.spec.md
// context: existing Express routes (provided as reference)
// constraint: ZERO business logic in this phase
// constraint: EVERY request must be proxied to Express backend

import { Controller, All, Req, Res } from '@nestjs/common';
import { Request, Response } from 'express';
import axios from 'axios';

@Controller()
export class ProxyController {
  private readonly expressBackend = process.env.EXPRESS_BACKEND_URL;

  @All('*')
  async proxyRequest(
    @Req() req: Request,
    @Res() res: Response,
  ): Promise<void> {
    try {
      const response = await axios({
        method: req.method,
        url: `${this.expressBackend}${req.originalUrl}`,
        headers: {
          ...req.headers,
          host: new URL(this.expressBackend).host,
        },
        data: req.body,
        validateStatus: () => true, // Forward ALL status codes
      });

      res.status(response.status);
      Object.entries(response.headers).forEach(([key, value]) => {
        if (value) res.setHeader(key, value as string);
      });
      res.send(response.data);
    } catch (error) {
      // If Express is down, return 502 — do not fabricate responses
      res.status(502).json({
        error: 'Backend service unavailable',
        timestamp: new Date().toISOString(),
      });
    }
  }
}
```

---

## 1.6 Risk Assessment in Refactor Specs

Every refactor carries risk. The purpose of the risk assessment section in your spec is not to eliminate risk — that is impossible — but to enumerate it, quantify it, and define mitigation strategies for each identified risk.

### The Risk Registry

```markdown
## Risk Registry

### RISK-001: Undocumented Endpoint Consumers
- **Probability:** HIGH
- **Impact:** HIGH (broken functionality for unknown consumers)
- **Mitigation:** Deploy request logging 2 weeks before migration.
  Identify all consumers of undocumented endpoints.
  Contact consumer teams before deprecating any endpoint.
- **Fallback:** Maintain Express proxy for undocumented endpoints
  indefinitely until all consumers are identified.

### RISK-002: Data Shape Differences Between v1 and v2
- **Probability:** MEDIUM
- **Impact:** HIGH (client-side parsing failures)
- **Mitigation:** v2 responses include all v1 fields (deprecated).
  New fields added alongside, not replacing.
  Provide 90-day deprecation window for old fields.
- **Fallback:** Content negotiation header allows clients to
  request v1 response shape from v2 endpoints.

### RISK-003: Performance Regression During Proxy Phase
- **Probability:** LOW
- **Impact:** MEDIUM (increased latency during Phase 1-3)
- **Mitigation:** Proxy adds <10ms overhead (benchmarked).
  Circuit breaker prevents cascade failures.
  Health checks monitor both NestJS and Express.
- **Fallback:** Load balancer can route directly to Express
  within 30 seconds if NestJS proxy degrades.

### RISK-004: Prisma ORM Query Performance
- **Probability:** MEDIUM
- **Impact:** MEDIUM (slower queries than hand-optimized SQL)
- **Mitigation:** Benchmark all migrated queries against originals.
  Use Prisma raw queries for performance-critical paths.
  Query analysis in staging before production deploy.
- **Fallback:** Maintain raw SQL fallback for top 5 queries by volume.
```

> **Professor's Aside:** Notice how every risk has both a mitigation (prevent the problem) and a fallback (what to do if the problem happens anyway). This is not paranoia — this is engineering. Hope is not a strategy.

---

## 1.7 How Google Manages Large-Scale Refactors

To understand what refactoring at extreme scale looks like, let us examine how Google approaches it. Google's monorepo contains billions of lines of code. When they need to make a change that touches thousands of files — say, migrating from one API version to another — they cannot simply open a pull request.

### Rosie and Large-Scale Changes (LSCs)

Google developed an internal system called Rosie for managing Large-Scale Changes. The process works like this:

1. **The change author writes a spec** describing the transformation — what pattern to find, what to replace it with, and what the expected behavior change is (often "none" for pure refactors).

2. **Automated tooling generates the changes** across the entire monorepo. This can touch tens of thousands of files.

3. **The changes are automatically sharded** into reviewable chunks. Each shard is small enough for a human to review meaningfully — typically 100-500 files.

4. **Each shard is sent to the appropriate code owners** for review. The reviewer does not need to understand the entire LSC — they only need to verify that the transformation was applied correctly to their code.

5. **Approved shards are merged incrementally.** The system tracks which shards have been approved, which are pending, and which were rejected (requiring manual intervention).

The key insight is that Google does not treat large refactors as a single massive change. They treat them as thousands of small, spec-compliant changes that can be reviewed and merged independently.

### What We Can Learn From This

Even if you are not operating at Google's scale, the principles apply:

```markdown
## Large-Scale Change Principles (Adapted for SDD)

1. **Spec the transformation, not the result.**
   Do not describe what every file should look like after.
   Describe the RULE for transforming any file that matches the pattern.

2. **Automate the application.**
   If you can describe the transformation precisely enough to spec it,
   you can describe it precisely enough to automate it.

3. **Shard the review.**
   No human should have to review more than they can hold in their head.
   Break changes into logical, reviewable chunks.

4. **Track progress mechanically.**
   Do not rely on status meetings to know where you are.
   Build dashboards that show transformation progress in real time.

5. **Allow partial completion.**
   The system must be functional at every intermediate state.
   If you can only complete 60% of the migration this quarter,
   the 60% that is done must work alongside the 40% that is not.
```

### Applying Google-Scale Thinking to Your Refactor Spec

```typescript
// Define the transformation rule, not the individual changes
interface TransformationRule {
  name: string;
  description: string;

  // Pattern to match in existing code
  match: {
    fileGlob: string;
    codePattern: RegExp;
    astPattern?: string; // For AST-based matching
  };

  // Transformation to apply
  transform: {
    type: 'replace' | 'wrap' | 'extract' | 'inline';
    template: string;
    preserveComments: boolean;
    preserveFormatting: boolean;
  };

  // Validation after transformation
  validate: {
    mustCompile: boolean;
    mustPassTests: boolean;
    customChecks: string[];
  };
}

// Example: migrate Express route handlers to NestJS controllers
const expressToNestMigration: TransformationRule = {
  name: 'express-route-to-nest-controller',
  description: 'Convert Express router.get/post/etc to NestJS @Get/@Post decorators',

  match: {
    fileGlob: 'src/routes/**/*.ts',
    codePattern: /router\.(get|post|put|delete|patch)\(['"]([^'"]+)['"]/,
  },

  transform: {
    type: 'replace',
    template: `
      @{{METHOD}}('{{PATH}}')
      async {{HANDLER_NAME}}(
        @Req() req: Request,
        @Res() res: Response,
      ): Promise<void> {
        {{BODY}}
      }
    `,
    preserveComments: true,
    preserveFormatting: false, // NestJS has different conventions
  },

  validate: {
    mustCompile: true,
    mustPassTests: true,
    customChecks: [
      'no-express-imports-in-controllers',
      'all-routes-have-decorators',
      'response-types-are-explicit',
    ],
  },
};
```

---

## 1.8 How Anthropic Approaches Iterative Improvement

Anthropic's approach to iterating on Claude's codebase offers a different perspective on refactor discipline. While Google's approach emphasizes scale and automation, Anthropic's approach emphasizes safety and careful evaluation at every step.

When Anthropic refactors a component of Claude's system — whether it is the training pipeline, the evaluation framework, or the serving infrastructure — they follow a pattern that aligns closely with SDD principles:

1. **Establish a behavioral baseline.** Before changing anything, capture the current system's behavior exhaustively. For Claude, this means running comprehensive evaluation suites that measure not just accuracy but also safety, helpfulness, and honesty across thousands of test cases.

2. **Spec the change with explicit safety constraints.** Every change spec includes not just what should improve, but what must not regress. The spec defines acceptable tolerance for any metric movement.

3. **Implement behind feature flags.** Changes are deployed but not activated. Both old and new code paths exist simultaneously.

4. **Gradual rollout with continuous evaluation.** Traffic is shifted incrementally — 1%, 5%, 10%, 25%, 50%, 100% — with evaluation at each step. Any regression triggers automatic rollback.

5. **Post-migration evaluation.** After full rollout, the old code path remains available for 30 days before removal.

This pattern maps directly to our Strangler Fig approach, with the addition of rigorous evaluation gates at every stage.

### The Safety-Aware Refactor Spec

Inspired by Anthropic's approach, your refactor spec should include evaluation criteria at each phase:

```markdown
## Phase 2 Spec: User List Endpoint Migration

### Evaluation Gates

#### Gate 1: Unit Test Parity
- All existing unit tests pass against new implementation
- New implementation has >= 90% code coverage
- MUST PASS before proceeding to Gate 2

#### Gate 2: Integration Test Parity
- All existing integration tests pass
- New tests added for previously untested edge cases
- Response shape validation: v2 output matches v1 output exactly
- MUST PASS before proceeding to Gate 3

#### Gate 3: Performance Benchmarking
- p50 latency: must be <= v1 p50 (currently 120ms)
- p99 latency: must be <= v1 p99 (currently 890ms)
- Throughput: must handle >= v1 peak load (currently 450 req/s)
- MUST PASS before proceeding to Gate 4

#### Gate 4: Shadow Traffic Validation
- Run new implementation in shadow mode for 48 hours
- Compare responses with v1 for every request
- Discrepancy rate must be < 0.01%
- MUST PASS before proceeding to production cutover

#### Gate 5: Production Canary
- Route 5% of production traffic to v2
- Monitor error rates, latency, and business metrics for 24 hours
- No alerts triggered
- MUST PASS before full rollout
```

---

## 1.9 "Freeze and Replace" vs "Incremental Migration" Spec Patterns

There are two fundamentally different approaches to specifying a refactor, and choosing the wrong one can doom your project before it starts.

### Pattern 1: Freeze and Replace

In this pattern, you freeze the old system (no new features), build the new system in parallel, and switch over on a specific date.

```markdown
## Freeze and Replace Spec

### Phase 1: Feature Freeze (2 weeks)
- No new features added to v1
- Bug fixes only, with approval
- Complete archaeology and reverse spec

### Phase 2: Parallel Build (6 weeks)
- Build v2 from target spec
- v1 continues serving production traffic
- v2 tested against v1 behavioral baseline

### Phase 3: Cutover (1 week)
- Switch production traffic from v1 to v2
- v1 remains on standby for rollback
- 24/7 monitoring during transition

### Phase 4: Decommission (2 weeks)
- Remove v1 after 14 days with zero rollbacks
- Archive v1 code
- Update all documentation
```

**When to use Freeze and Replace:**
- The old system is so tangled that incremental changes are impossible
- You need a clean break from legacy technology
- The team has capacity for a dedicated migration sprint
- Downtime or feature freeze is acceptable

**When NOT to use Freeze and Replace:**
- The system must continue evolving during migration
- Stakeholders will not accept a feature freeze
- The old system is too large to replace in one go
- You cannot afford a "big bang" deployment risk

### Pattern 2: Incremental Migration

In this pattern, you migrate piece by piece, with the old and new systems coexisting throughout.

```markdown
## Incremental Migration Spec

### Principles
- Both v1 and v2 code coexist in the same deployment
- Each migration step is independently deployable and reversible
- New features are built against v2 specs
- v1 features are migrated on a prioritized schedule

### Migration Queue (Priority Order)
1. User CRUD endpoints (highest traffic, most bugs)
2. Search functionality (security concerns with raw SQL)
3. Admin endpoints (high business impact)
4. Reporting endpoints (low traffic, low risk)
5. Legacy webhook handlers (need consumer coordination)

### Per-Item Migration Process
For each item in the migration queue:
1. Write target spec for this component
2. Implement v2 version behind feature flag
3. Run shadow traffic comparison for 48 hours
4. Gradual rollout: 5% → 25% → 50% → 100%
5. Monitor for 7 days at 100%
6. Remove v1 code path
```

**When to use Incremental Migration:**
- The system must continue accepting new features
- Risk tolerance is low (financial systems, health care, etc.)
- The migration spans multiple quarters
- Multiple teams need to coordinate

> **Professor's Aside:** In my experience, incremental migration is the right choice about 80% of the time. Freeze and Replace feels cleaner, and engineers love it because they get to start fresh. But business stakeholders rarely accept a feature freeze, and the "big bang" cutover is where most Freeze and Replace projects fail. The Strangler Fig is your friend. Learn to love it.

---

## 1.10 Practical Walkthrough: Messy Express.js API to Clean NestJS Architecture

Let us walk through a complete refactor spec, from archaeology to execution plan. This is the kind of document you would produce in a professional setting.

### The Scenario

You have inherited a 3-year-old Express.js API for an e-commerce platform. It has 47 endpoints, no consistent error handling, a mix of callbacks and promises, and three different approaches to database access. Your mandate: migrate to NestJS with Prisma ORM, without breaking anything.

### Step 1: The Archaeology Report (Abbreviated)

```markdown
# Archaeology Report: E-Commerce API

## Structure
- 23 route files in src/routes/
- 8 "service" files (really just utility functions)
- 4 database access patterns:
  1. Raw pg queries (oldest endpoints)
  2. Knex query builder (mid-era endpoints)
  3. Sequelize ORM (newest endpoints, poorly configured)
  4. Direct SQL strings in route handlers (worst offenders)

## Traffic Analysis (14-day sample)
- Total endpoints: 47
- Active endpoints (>1 req/day): 31
- High-traffic endpoints (>1000 req/day): 8
- Zero-traffic endpoints: 9 (candidates for removal)

## Critical Findings
- No request validation on 38 of 47 endpoints
- SQL injection possible on 6 endpoints (raw SQL with string concat)
- No rate limiting anywhere
- Authentication middleware inconsistently applied
- 3 endpoints have hardcoded database credentials (!!!)
```

### Step 2: The Reverse Spec (Key Endpoints)

```typescript
// reverse-spec: what the product endpoint actually does today
interface ProductEndpointCurrentBehavior {
  /**
   * GET /products
   * @quirk Returns max 1000 products, no pagination support
   * @quirk Includes soft-deleted products if no filter specified
   * @quirk Price is returned as string, not number
   * @quirk Images array sometimes null, sometimes empty array
   */
  listProducts(): Promise<{
    products: Array<{
      id: number;          // Auto-increment integer
      name: string;
      price: string;       // "19.99" — string, not number
      category: string;    // Free text, no enum
      images: string[] | null;  // Inconsistent nullability
      deleted: boolean;    // Soft delete flag, exposed to clients
      created_at: string;  // ISO string
    }>;
  }>;
}
```

### Step 3: The Target Spec

```typescript
// target-spec: what the product endpoint should do after migration
interface ProductEndpointTargetBehavior {
  /**
   * GET /api/v2/products
   * @spec Paginated product listing with filtering
   * @validation All query params validated by class-validator
   * @performance Response time p99 < 150ms
   */
  listProducts(params: {
    page: number;           // Required, >= 1
    limit: number;          // Required, 1-100
    category?: ProductCategory;  // Enum, validated
    minPrice?: number;      // Decimal, >= 0
    maxPrice?: number;      // Decimal, >= minPrice
    sortBy?: 'price' | 'name' | 'created';
    sortOrder?: 'asc' | 'desc';
  }): Promise<{
    data: ProductDTO[];
    pagination: {
      page: number;
      limit: number;
      total: number;
      totalPages: number;
    };
  }>;
}

// Clean DTO — no internal fields exposed
interface ProductDTO {
  id: string;              // UUID, not auto-increment
  name: string;
  price: number;           // Number, not string
  category: ProductCategory;
  images: string[];        // Always array, never null
  createdAt: string;       // ISO 8601
  updatedAt: string;       // ISO 8601
}

enum ProductCategory {
  ELECTRONICS = 'electronics',
  CLOTHING = 'clothing',
  HOME = 'home',
  SPORTS = 'sports',
  BOOKS = 'books',
}
```

### Step 4: The Migration Spec

```markdown
# Refactor Spec: E-Commerce API Migration
# Express.js → NestJS + Prisma

## Metadata
- **Author:** [Your Name]
- **Status:** Draft
- **Created:** 2026-02-24
- **Target Completion:** 2026-05-15
- **Reviewers:** [Tech Lead], [Product Owner], [SRE Lead]

## Executive Summary
Migrate the e-commerce API from Express.js to NestJS with Prisma ORM.
The migration will be incremental, using the Strangler Fig pattern over
12 weeks. Zero downtime. Zero breaking changes for existing clients.

## Phases

### Phase 1: Foundation (Week 1-2)
**Deliverables:**
- NestJS application scaffold
- Prisma schema matching existing database
- Proxy controller forwarding all traffic to Express
- CI/CD pipeline for new NestJS app
- Health check and monitoring integration

**Spec for AI Agent:**
```text
CONTEXT: We have an existing Express.js e-commerce API. We are
beginning a migration to NestJS. In this phase, you will create
the NestJS scaffold ONLY.

OBJECTIVE: Create a NestJS application that:
1. Starts on port 3001 (Express runs on 3000)
2. Proxies ALL incoming requests to http://localhost:3000
3. Logs every proxied request (method, path, status, latency)
4. Has a /health endpoint that checks both NestJS and Express

CONSTRAINTS:
- Do NOT implement any business logic
- Do NOT create any database connections
- Do NOT modify the existing Express application
- Use axios for proxying
- Use @nestjs/config for configuration
- Use @nestjs/terminus for health checks

TEST REQUIREMENTS:
- Unit test the proxy controller
- Integration test: request to NestJS returns same response as
  direct request to Express (for 10 sample endpoints)
- Health check test: returns healthy when Express is up,
  unhealthy when Express is down
```

### Phase 2: Data Layer (Week 3-4)
**Deliverables:**
- Prisma schema with all models
- Prisma client configured in NestJS
- Database migration scripts (Prisma Migrate)
- Seed data for testing

**Spec for AI Agent:**
```text
CONTEXT: NestJS proxy is running. Express handles all business
logic. We need to set up the Prisma data layer.

OBJECTIVE: Create Prisma schema matching the existing PostgreSQL
database. Generate Prisma client. Create NestJS database module.

CONSTRAINTS:
- Schema must match existing tables EXACTLY
  (we are NOT changing the database schema yet)
- Use Prisma introspection: npx prisma db pull
- Clean up introspected schema (add proper types, relations)
- Do NOT run any Prisma migrations against production
- Configure separate database URLs for dev/staging/prod
- Create a DatabaseModule that provides PrismaService

REFERENCE: See archaeology report section "Database Schema"
for current table definitions and relationships.

TEST REQUIREMENTS:
- PrismaService connects successfully
- All models queryable
- Relation traversals work correctly
- Connection pooling configured (min: 5, max: 20)
```

### Phase 3: Product Endpoints (Week 5-6)
[Detailed spec for migrating the 8 product-related endpoints]

### Phase 4: User Endpoints (Week 7-8)
[Detailed spec for migrating the 12 user-related endpoints]

### Phase 5: Order Endpoints (Week 9-10)
[Detailed spec for migrating the 11 order-related endpoints]

### Phase 6: Cleanup and Decommission (Week 11-12)
[Spec for removing Express, cleaning up proxies, final testing]
```

### Step 5: The Test Migration Plan

```markdown
## Test Migration Plan

### Current Test State
- 127 unit tests (Jest) — 94% passing (8 known failures)
- 43 integration tests — 88% passing (5 flaky tests)
- 0 end-to-end tests
- No performance tests
- No contract tests

### Target Test State
- All existing passing tests migrated to NestJS testing framework
- 8 known failures fixed or documented as tech debt
- 5 flaky tests stabilized or replaced
- New E2E test suite using Supertest
- Performance benchmarks using k6
- Contract tests using Pact

### Migration Strategy
1. DO NOT delete any existing tests until v2 equivalents exist
2. Tests migrate WITH their corresponding endpoints
3. Each phase includes its own test deliverables
4. Shadow testing: run both v1 and v2 tests during migration
5. Test coverage must not decrease at any phase

### Per-Phase Test Requirements

#### Phase 1 Tests (Foundation)
- Proxy passthrough tests: verify request/response fidelity
- Health check tests: NestJS up/down, Express up/down scenarios
- Latency overhead tests: proxy adds < 10ms

#### Phase 2 Tests (Data Layer)
- Model relationship tests: all FK relations traversable
- Query performance tests: baseline measurements for comparison
- Connection pool tests: concurrent access patterns

#### Phase 3-5 Tests (Endpoint Migration)
For EACH migrated endpoint:
- [ ] Unit tests for controller
- [ ] Unit tests for service
- [ ] Unit tests for DTOs/validation
- [ ] Integration test: full request → response cycle
- [ ] Contract test: response matches API contract
- [ ] Performance test: meets latency SLA
- [ ] Shadow test: v2 response matches v1 response
```

---

## 1.11 Dependencies and Ripple Effects

When you refactor one part of a system, other parts feel the tremor. Your spec must map these dependencies explicitly.

### The Dependency Map

```typescript
// Formal dependency specification for the refactor
interface RefactorDependencyMap {
  // Components we are changing
  targets: {
    component: string;
    changeType: 'rewrite' | 'modify' | 'replace' | 'remove';
    affectedInterfaces: string[];
  }[];

  // Components that depend on what we are changing
  downstreamDependencies: {
    component: string;
    dependsOn: string;      // Which target
    interfaceUsed: string;  // Which interface
    impactAssessment: 'none' | 'minor' | 'major' | 'breaking';
    mitigationPlan: string;
  }[];

  // Components that our targets depend on
  upstreamDependencies: {
    component: string;
    usedBy: string;         // Which target
    changeRequired: boolean;
    changeDescription?: string;
  }[];
}

// Example for our e-commerce migration
const ecommerceDependencyMap: RefactorDependencyMap = {
  targets: [
    {
      component: 'ProductAPI',
      changeType: 'rewrite',
      affectedInterfaces: ['GET /products', 'GET /products/:id', 'POST /products'],
    },
  ],

  downstreamDependencies: [
    {
      component: 'Mobile App v2.1',
      dependsOn: 'ProductAPI',
      interfaceUsed: 'GET /products',
      impactAssessment: 'minor',
      mitigationPlan: 'v2 API returns superset of v1 fields. Mobile app ignores unknown fields.',
    },
    {
      component: 'Search Indexer',
      dependsOn: 'ProductAPI',
      interfaceUsed: 'GET /products (internal)',
      impactAssessment: 'major',
      mitigationPlan: 'Search indexer must be updated to use v2 pagination. Coordinate with Search team.',
    },
    {
      component: 'Analytics Pipeline',
      dependsOn: 'ProductAPI',
      interfaceUsed: 'Webhook: product.created',
      impactAssessment: 'none',
      mitigationPlan: 'Webhook payload unchanged in v2.',
    },
  ],

  upstreamDependencies: [
    {
      component: 'PostgreSQL Database',
      usedBy: 'ProductAPI',
      changeRequired: false,
      changeDescription: 'Schema unchanged. Only data access layer changes (raw SQL → Prisma).',
    },
    {
      component: 'Redis Cache',
      usedBy: 'ProductAPI',
      changeRequired: true,
      changeDescription: 'Cache key format changes from product:{id} to v2:product:{uuid}. Cache must be flushed during migration.',
    },
  ],
};
```

### Ripple Effect Analysis

```markdown
## Ripple Effect Analysis

### Direct Effects (Tier 1)
Components directly modified by this refactor:
- Product API: Rewritten in NestJS
- User API: Rewritten in NestJS
- Order API: Rewritten in NestJS

### Indirect Effects (Tier 2)
Components that must adapt to Tier 1 changes:
- Mobile App: Update API client for v2 endpoints
- Admin Dashboard: Update API calls (16 undocumented endpoints)
- Search Indexer: Update to use pagination
- Redis Cache: Flush and re-warm with new key format

### Tertiary Effects (Tier 3)
Components that might be affected by Tier 2 changes:
- CDN: Cache invalidation patterns may change
- Monitoring: Alert thresholds need recalibration for new latency baselines
- CI/CD: Pipeline needs NestJS build steps

### Coordination Requirements
| Team          | Coordination Need              | Timeline     |
|---------------|-------------------------------|--------------|
| Mobile        | v2 API client update          | Week 6-8     |
| Admin         | Dashboard API migration       | Week 8-10    |
| Search        | Indexer pagination support     | Week 5-6     |
| SRE           | Monitoring recalibration      | Week 1 + each phase |
| QA            | Regression testing per phase  | Continuous   |
```

---

## 1.12 The Refactor Approval Gate

A refactor spec is not just a technical document — it is a proposal. It requires approval from multiple stakeholders, each evaluating it from a different perspective.

### The Approval Matrix

```markdown
## Approval Requirements

### Technical Lead — Architectural Review
- [ ] Target architecture is sound
- [ ] Migration path is feasible
- [ ] Risk assessment is comprehensive
- [ ] Testing plan is adequate
- [ ] Performance targets are realistic

### Product Owner — Business Impact Review
- [ ] Feature freeze duration is acceptable
- [ ] No customer-facing changes without communication plan
- [ ] Business metrics monitoring included
- [ ] Rollback plan protects revenue-critical paths

### SRE/DevOps Lead — Operational Review
- [ ] Deployment plan is sound
- [ ] Monitoring and alerting included
- [ ] Rollback procedures are tested
- [ ] Resource requirements (compute, memory) identified
- [ ] On-call procedures updated for migration period

### Security Lead — Security Review
- [ ] SQL injection vulnerabilities addressed in migration
- [ ] Authentication/authorization not weakened
- [ ] No new attack surface introduced
- [ ] Credentials properly managed (no hardcoded secrets)

### QA Lead — Quality Review
- [ ] Test migration plan is complete
- [ ] No test coverage regression
- [ ] Performance benchmarking plan included
- [ ] Regression test suite updated

## Approval Status
| Reviewer        | Status    | Date       | Notes           |
|----------------|-----------|------------|-----------------|
| Tech Lead      | PENDING   |            |                 |
| Product Owner  | PENDING   |            |                 |
| SRE Lead       | PENDING   |            |                 |
| Security Lead  | PENDING   |            |                 |
| QA Lead        | PENDING   |            |                 |

## Approval Rule
All five approvals required before Phase 1 begins.
Phase-level approvals required from Tech Lead and QA Lead only.
```

> **Professor's Aside:** The approval gate is not bureaucracy. It is a forcing function that ensures your spec is complete. If the Security Lead asks "how are you handling the SQL injection vulnerabilities?" and your spec does not address that, the spec is incomplete. The approval process makes the spec better.

---

## 1.13 Exercise: Audit a Legacy Codebase and Produce a Prioritized Refactor Spec

### Your Assignment

You have been given access to a legacy Node.js application — a task management API built two years ago. The original developer has left the company. There are no specs, minimal tests, and the documentation is a single README that has not been updated in 18 months.

**Part 1: Archaeology (Estimated time: 2 hours)**

1. Clone the repository and run the application locally
2. Generate a dependency graph
3. Catalog all endpoints (documented and undocumented)
4. Run the test suite and document results
5. Identify all database access patterns
6. Review git history for significant changes
7. Produce an Archaeology Report following the template from Section 1.2.2

**Part 2: Reverse Spec (Estimated time: 2 hours)**

1. For each discovered endpoint, write a reverse spec describing current behavior
2. Document all quirks, bugs, and undocumented behaviors
3. Identify implicit contracts with downstream consumers
4. Map all side effects (emails, webhooks, cache writes, etc.)

**Part 3: Target Spec (Estimated time: 3 hours)**

1. Design the target architecture (your choice of framework)
2. Write target specs for all endpoints
3. Define the data model improvements
4. Specify the validation rules
5. Define performance targets

**Part 4: Migration Plan (Estimated time: 3 hours)**

1. Choose a migration strategy (Freeze-and-Replace or Incremental)
2. Define phases with clear deliverables
3. Write AI-ready specs for at least two phases
4. Create a test migration plan
5. Map dependencies and ripple effects
6. Define approval gates

**Part 5: Risk Assessment (Estimated time: 1 hour)**

1. Identify at least 5 risks
2. For each risk: probability, impact, mitigation, and fallback
3. Define monitoring and alerting for each risk

### Submission Criteria

Your refactor spec should be a single Markdown document that another engineer (or an AI agent) could pick up and execute without asking you any questions. Every decision should be justified. Every constraint should be explicit. Every risk should have a mitigation plan.

```markdown
# Evaluation Rubric

## Archaeology (20 points)
- Complete structural analysis ............ 5 pts
- Behavioral analysis with evidence ....... 5 pts
- Contract identification ................. 5 pts
- Historical context ...................... 5 pts

## Reverse Spec (20 points)
- All endpoints documented ................ 5 pts
- Quirks and bugs identified .............. 5 pts
- Implicit contracts mapped ............... 5 pts
- Side effects cataloged .................. 5 pts

## Target Spec (20 points)
- Clean architecture design ............... 5 pts
- Complete endpoint specifications ........ 5 pts
- Validation and security rules ........... 5 pts
- Performance targets defined ............. 5 pts

## Migration Plan (25 points)
- Strategy choice justified ............... 5 pts
- Phases well-defined .................... 5 pts
- AI-ready spec quality .................. 5 pts
- Test migration plan .................... 5 pts
- Dependency mapping ..................... 5 pts

## Risk Assessment (15 points)
- Risk identification .................... 5 pts
- Mitigation strategies .................. 5 pts
- Monitoring and alerting ................ 5 pts
```

---

## Chapter Summary

The Refactor Spec is perhaps the most challenging type of specification to write, because it demands that you understand two systems simultaneously: the one that exists and the one you want to create. The techniques we covered in this chapter — the archaeology phase, the reverse spec, the scoping matrix, the Strangler Fig pattern, and the risk registry — give you a systematic approach to what is otherwise an overwhelming task.

Remember the core principle: **understand before you specify, specify before you change, and change incrementally with validation at every step.**

In the next chapter, we will explore how specs serve double duty as documentation — eliminating the eternal problem of documentation that is always out of date. If your spec is the single source of truth, and your docs are generated from your spec, then your docs are always current. That is not just convenient — it is transformative.

> *"The best refactor is the one where nobody notices anything changed. The system just got faster, more reliable, and easier to work with — and the users never knew. That is the power of a well-executed Refactor Spec."*

---

**Next Chapter:** Documentation as Code — How specs eliminate the "docs are always out of date" problem

---

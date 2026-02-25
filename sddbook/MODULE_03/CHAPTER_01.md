# Chapter 1: Spec-to-Test Mapping (TDD for AI)

## MODULE 03 — Validation & The Feedback Loop (Intermediate Level)

---

## Lecture Preamble

*The lecture hall settles into a focused quiet. The professor pulls up a terminal on the projector -- two panes side by side. On the left, a specification document. On the right, an empty test file. The cursor blinks.*

Good morning, everyone. Today we begin Module 3, and I want to start with a confession.

Early in my career, I skipped writing tests. I told myself I'd "add them later." Later never came. The code worked -- until it didn't. And when it broke, I had no safety net, no contract to tell me what "correct" even meant. I was guessing.

Now multiply that problem by a factor of a thousand. That's what happens when you let an AI write code without tests. The AI doesn't just skip tests -- it doesn't even know what "correct" means unless you tell it. And the way you tell it is through a spec. And the way you *verify* it understood is through tests.

This chapter is about the most important pipeline in Spec-Driven Development:

**Spec --> Tests --> Implementation --> Validation**

Not spec to implementation. Not "write the code and hope for the best." Spec to *tests first*. Then implementation. Then validation against those tests.

This is TDD -- Test-Driven Development -- but adapted for an era where your co-developer is a large language model that can produce a thousand lines of plausible-looking code in seconds, and get the subtle details wrong in ways that won't surface until production.

Let's begin.

---

## 1.1 Why Test-First Is Even More Important with AI

### The Human Developer's Testing Problem

When a human developer writes code, they carry a mental model of the system in their head. They understand the "why" behind design decisions. They remember conversations with stakeholders. They have intuition about edge cases because they've been burned before.

Human developers still need tests, of course. But a senior developer can often spot a bug by *reading* their own code, because they understand the intent behind every line.

### The AI Developer's Testing Problem

An AI has none of these advantages. It has:

- **No persistent memory** of prior conversations (unless you engineer it)
- **No understanding of "why"** -- only pattern matching against training data
- **No intuition about your domain** -- it knows general patterns, not your business rules
- **A strong tendency to produce plausible-looking code** that passes a cursory glance but fails under scrutiny
- **Hallucination risk** -- it may invent APIs, misremember function signatures, or fabricate entire libraries

> **Professor's Aside:** I've seen Claude generate a beautiful implementation of a caching layer that used a library method that didn't exist. The code *looked* perfect. It had proper error handling, thoughtful comments, elegant structure. But `cache.invalidatePattern()` was a hallucinated method. Without a test, this would have shipped.

This is why test-first is not merely "a good practice" with AI -- it is a **critical safety mechanism**. The tests are your contract. They are the only thing standing between the AI's confident-sounding output and reality.

### The Fundamental Asymmetry

Here's the key insight that makes TDD essential for AI-assisted development:

| Aspect | Human Developer | AI Developer |
|--------|----------------|--------------|
| Can verify their own work | Yes (code review, reasoning) | No (cannot execute code) |
| Remembers requirements | Usually | Only if in context window |
| Detects logical errors | Through reasoning | Only through pattern matching |
| Produces compilable code | Usually | Usually (but not always) |
| Produces *correct* code | Sometimes | Plausible but unverified |
| Speed of output | Slow (minutes/hours) | Fast (seconds) |

The asymmetry is clear: AI is *fast* but *unverified*. Tests are the verification layer that bridges this gap.

### What the Industry Is Doing

In 2025-2026, every major AI company has recognized this problem:

- **Anthropic** builds Claude with the understanding that its outputs must be testable. Claude Code, their CLI-based development tool, is designed to work within existing testing frameworks. When Claude generates code, the recommended workflow is to generate tests from the spec *before* asking for the implementation.

- **Google DeepMind** uses formal verification techniques to validate AI behavior against specifications. Their AlphaCode and AlphaProof systems don't just generate code -- they generate code that must pass extensive test suites derived from problem specifications.

- **OpenAI** has published research on using tests as a signal for code correctness. Their Codex and GPT-based code generation tools improve significantly when given test cases as part of the prompt -- the tests constrain the solution space.

- **Meta** open-sourced their testing infrastructure for Llama-based code generation, emphasizing that "untested AI-generated code is technical debt, not a feature."

The consensus across the industry is clear: **if you're using AI to write code, you need tests more than ever, not less**.

---

## 1.2 The Spec-to-Test-to-Implementation Pipeline

### The Pipeline, Step by Step

```
+--------+      +---------+      +----------------+      +------------+
|  SPEC  | ---> |  TESTS  | ---> | IMPLEMENTATION | ---> | VALIDATION |
+--------+      +---------+      +----------------+      +------------+
     |               |                   |                      |
     |               |                   |                      |
  "What we      "How we know       "The code that         "Did it
   want"         it works"          does the work"         actually
                                                           work?"
```

Let's walk through each stage.

#### Stage 1: The Spec (What We Want)

You've already learned this in Modules 1 and 2. The spec is a precise, structured document that describes:

- **What** the system should do (functional requirements)
- **How** it should behave at boundaries (constraints)
- **What** it should *not* do (negative requirements)
- **How** success is measured (acceptance criteria)

#### Stage 2: The Tests (How We Know It Works)

This is where most developers -- and most AI workflows -- go wrong. They skip this step. They go straight from spec to implementation.

**Don't.**

The tests are derived *directly* from the spec. Every assertion in a test should trace back to a specific line or section of the spec. If you can't trace a test back to the spec, either the test is unnecessary or the spec is incomplete.

#### Stage 3: The Implementation (The Code)

Only *after* the tests exist do you (or the AI) write the implementation. The tests provide:

- A clear definition of "done"
- Immediate feedback on correctness
- Protection against hallucinated behavior
- A regression safety net for future changes

#### Stage 4: The Validation (Did It Work?)

Run the tests. If they pass, the implementation satisfies the spec (at least as far as the tests cover). If they fail, the implementation is wrong -- not the tests (assuming the tests correctly reflect the spec).

> **Professor's Aside:** I want to be crystal clear about this: when a test fails, your *first* assumption should be that the implementation is wrong, not the test. If you start changing tests to match incorrect implementations, you've defeated the entire purpose of this pipeline. Tests are the spec's proxy. Treat them with the same respect you'd give the spec itself.

### Why This Order Matters for AI

When you give an AI a spec and ask it to write code, the AI has a tendency to:

1. **Interpret ambiguity optimistically** -- assuming the simplest case when the spec is unclear
2. **Over-engineer solutions** -- adding features not in the spec
3. **Under-implement edge cases** -- handling the happy path but missing boundaries
4. **Drift from the spec** -- starting aligned but gradually introducing its own patterns

By generating tests *first*, you create a fence around the AI's implementation space. The tests say: "Here are the exact behaviors we expect. Nothing more, nothing less."

---

## 1.3 Mapping Spec Lines to Test Cases

### The One-to-One (Minimum) Principle

Every statement in a spec should map to *at least* one test case. Many statements will map to multiple test cases. Here's how to think about the mapping:

| Spec Statement Type | Minimum Test Cases |
|---------------------|-------------------|
| Functional requirement | 1 positive test |
| Constraint / boundary | 2 tests (at boundary, past boundary) |
| Negative requirement | 1 negative test |
| Performance requirement | 1 benchmark test |
| Error handling | 1 test per error condition |
| State transition | 1 test per transition |

### A Concrete Example

Let's take a micro-spec from the kind you'd write in Module 1:

```markdown
## Spec: User Registration Endpoint

### Functional Requirements
1. The endpoint SHALL accept a POST request to `/api/users/register`
2. The request body SHALL contain: email (string), password (string), name (string)
3. The endpoint SHALL return a 201 status code with a user object on success
4. The user object SHALL contain: id, email, name, createdAt
5. The password SHALL NOT be included in the response

### Constraints
6. Email SHALL be a valid email format (RFC 5322)
7. Password SHALL be at least 8 characters long
8. Password SHALL contain at least one uppercase letter, one lowercase letter, and one number
9. Name SHALL be between 1 and 100 characters
10. Email SHALL be unique across all users

### Error Handling
11. Invalid email format SHALL return 400 with error message "Invalid email format"
12. Weak password SHALL return 400 with error message "Password does not meet requirements"
13. Duplicate email SHALL return 409 with error message "Email already registered"
14. Missing required fields SHALL return 400 with specific field error messages
```

Now, let's map each line to test cases:

```typescript
// test/user-registration.spec.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createApp } from '../src/app';
import { resetDatabase } from './helpers/db';

describe('User Registration Endpoint', () => {
  let app: ReturnType<typeof createApp>;

  beforeEach(async () => {
    app = createApp();
    await resetDatabase();
  });

  afterEach(async () => {
    await app.close();
  });

  // ===========================================================
  // Spec Line 1: POST /api/users/register
  // ===========================================================
  describe('POST /api/users/register', () => {

    // Spec Line 1: SHALL accept a POST request
    it('should accept POST requests to /api/users/register', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      expect(response.statusCode).not.toBe(404);
      expect(response.statusCode).not.toBe(405);
    });

    // Spec Line 2: Request body contains email, password, name
    it('should require email, password, and name in request body', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {},  // empty body
      });

      expect(response.statusCode).toBe(400);
    });

    // Spec Line 3: SHALL return 201 with user object on success
    it('should return 201 status code on successful registration', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      expect(response.statusCode).toBe(201);
    });

    // Spec Line 4: User object contains id, email, name, createdAt
    it('should return user object with id, email, name, and createdAt', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      const body = JSON.parse(response.body);
      expect(body).toHaveProperty('id');
      expect(body).toHaveProperty('email', 'test@example.com');
      expect(body).toHaveProperty('name', 'Test User');
      expect(body).toHaveProperty('createdAt');
      expect(new Date(body.createdAt).getTime()).not.toBeNaN();
    });

    // Spec Line 5: Password SHALL NOT be in response
    it('should NOT include password in the response', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      const body = JSON.parse(response.body);
      expect(body).not.toHaveProperty('password');
      expect(body).not.toHaveProperty('passwordHash');
      expect(body).not.toHaveProperty('hashedPassword');
    });
  });

  // ===========================================================
  // Constraints (Spec Lines 6-10)
  // ===========================================================
  describe('Constraints', () => {

    // Spec Line 6: Valid email format
    describe('Email validation', () => {
      it('should accept a valid email address', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'valid@example.com',
            password: 'ValidPass1',
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(201);
      });

      it.each([
        'not-an-email',
        '@missing-local.com',
        'missing-domain@',
        'missing@.com',
        'spaces in@email.com',
        '',
      ])('should reject invalid email: "%s"', async (invalidEmail) => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: invalidEmail,
            password: 'ValidPass1',
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(400);
      });
    });

    // Spec Line 7: Password at least 8 characters
    describe('Password length', () => {
      it('should accept password with exactly 8 characters', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'Valid1Aa', // Exactly 7 -- wait, we need 8
            name: 'Test User',
          },
        });
        // This is 7 chars, should be rejected
        // Demonstrating: always check your test data!
      });

      it('should reject password with 7 characters', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'Short1A',  // 7 characters
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(400);
      });

      it('should accept password with exactly 8 characters', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'Valid1Aa!', // 8 characters with all requirements
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(201);
      });
    });

    // Spec Line 8: Password complexity
    describe('Password complexity', () => {
      it('should reject password without uppercase letter', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'nouppercase1',
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(400);
      });

      it('should reject password without lowercase letter', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'NOLOWERCASE1',
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(400);
      });

      it('should reject password without a number', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'NoNumberHere',
            name: 'Test User',
          },
        });

        expect(response.statusCode).toBe(400);
      });
    });

    // Spec Line 9: Name length 1-100
    describe('Name length', () => {
      it('should accept a name with 1 character', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'ValidPass1',
            name: 'A',
          },
        });

        expect(response.statusCode).toBe(201);
      });

      it('should accept a name with 100 characters', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'ValidPass1',
            name: 'A'.repeat(100),
          },
        });

        expect(response.statusCode).toBe(201);
      });

      it('should reject an empty name', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'ValidPass1',
            name: '',
          },
        });

        expect(response.statusCode).toBe(400);
      });

      it('should reject a name with 101 characters', async () => {
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'test@example.com',
            password: 'ValidPass1',
            name: 'A'.repeat(101),
          },
        });

        expect(response.statusCode).toBe(400);
      });
    });

    // Spec Line 10: Unique email
    describe('Email uniqueness', () => {
      it('should reject duplicate email registration', async () => {
        // First registration
        await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'duplicate@example.com',
            password: 'ValidPass1',
            name: 'First User',
          },
        });

        // Second registration with same email
        const response = await app.inject({
          method: 'POST',
          url: '/api/users/register',
          payload: {
            email: 'duplicate@example.com',
            password: 'ValidPass1',
            name: 'Second User',
          },
        });

        expect(response.statusCode).toBe(409);
      });
    });
  });

  // ===========================================================
  // Error Handling (Spec Lines 11-14)
  // ===========================================================
  describe('Error Handling', () => {

    // Spec Line 11: Invalid email error message
    it('should return specific error for invalid email format', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'not-valid',
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error).toBe('Invalid email format');
    });

    // Spec Line 12: Weak password error message
    it('should return specific error for weak password', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'weak',
          name: 'Test User',
        },
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error).toBe('Password does not meet requirements');
    });

    // Spec Line 13: Duplicate email error message
    it('should return specific error for duplicate email', async () => {
      await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'existing@example.com',
          password: 'ValidPass1',
          name: 'Existing User',
        },
      });

      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'existing@example.com',
          password: 'ValidPass1',
          name: 'New User',
        },
      });

      expect(response.statusCode).toBe(409);
      const body = JSON.parse(response.body);
      expect(body.error).toBe('Email already registered');
    });

    // Spec Line 14: Missing required fields
    it('should return field-specific error for missing email', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          password: 'ValidPass1',
          name: 'Test User',
        },
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error).toContain('email');
    });

    it('should return field-specific error for missing password', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          name: 'Test User',
        },
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error).toContain('password');
    });

    it('should return field-specific error for missing name', async () => {
      const response = await app.inject({
        method: 'POST',
        url: '/api/users/register',
        payload: {
          email: 'test@example.com',
          password: 'ValidPass1',
        },
      });

      expect(response.statusCode).toBe(400);
      const body = JSON.parse(response.body);
      expect(body.error).toContain('name');
    });
  });
});
```

Notice several things about this test file:

1. **Every test has a comment linking it to a spec line number.** This is the traceability that makes SDD work.
2. **Boundary conditions are tested on both sides.** Name length 1 (valid), 0 (invalid), 100 (valid), 101 (invalid).
3. **Error messages are tested exactly.** The spec says the error message SHALL be "Invalid email format" -- so we assert that exact string.
4. **Negative cases are explicit.** The spec says password SHALL NOT be in the response, so we explicitly check for its absence.

---

## 1.4 Boundary Conditions and Edge Cases: Extracting Them from Constraints

### The Boundary Extraction Method

Every constraint in a spec implies boundaries. Here's how to extract them systematically:

```
Constraint: "Password SHALL be at least 8 characters long"

Boundaries:
  - Length 7: INVALID (just below boundary)
  - Length 8: VALID   (at boundary)
  - Length 9: VALID   (just above boundary)
  - Length 0: INVALID (empty input)
  - Length 1000: ? (spec doesn't say -- this is a SPEC GAP)
```

> **Professor's Aside:** Notice that last entry? The spec doesn't mention a *maximum* password length. Is a 10,000-character password valid? What about 1 million characters? This is a spec gap -- and finding these gaps through test mapping is one of the most valuable outcomes of this process. Go back and update the spec. Add a maximum password length constraint. The test-mapping process just improved your spec.

### The Edge Case Taxonomy

When extracting edge cases from a spec, think about these categories:

**1. Null / Empty / Missing**
```typescript
// What happens when the input is null, undefined, empty string, empty array?
it('should handle null email gracefully', async () => {
  const response = await app.inject({
    method: 'POST',
    url: '/api/users/register',
    payload: {
      email: null,
      password: 'ValidPass1',
      name: 'Test User',
    },
  });
  expect(response.statusCode).toBe(400);
});
```

**2. Boundary Values (Min/Max)**
```typescript
// At the boundary, one below, one above
it('should accept name at maximum length', async () => {
  // ...name with exactly 100 characters
});

it('should reject name exceeding maximum length', async () => {
  // ...name with 101 characters
});
```

**3. Type Mismatches**
```typescript
// What if the input is the wrong type?
it('should handle numeric email gracefully', async () => {
  const response = await app.inject({
    method: 'POST',
    url: '/api/users/register',
    payload: {
      email: 12345,
      password: 'ValidPass1',
      name: 'Test User',
    },
  });
  expect(response.statusCode).toBe(400);
});
```

**4. Special Characters and Encoding**
```typescript
// Unicode, emoji, SQL injection, XSS
it('should handle unicode characters in name', async () => {
  const response = await app.inject({
    method: 'POST',
    url: '/api/users/register',
    payload: {
      email: 'test@example.com',
      password: 'ValidPass1',
      name: 'Jean-Pierre Lefevre',
    },
  });
  expect(response.statusCode).toBe(201);
});

it('should sanitize potentially dangerous input', async () => {
  const response = await app.inject({
    method: 'POST',
    url: '/api/users/register',
    payload: {
      email: 'test@example.com',
      password: 'ValidPass1',
      name: '<script>alert("xss")</script>',
    },
  });
  // Should either reject or sanitize, not store raw HTML
  if (response.statusCode === 201) {
    const body = JSON.parse(response.body);
    expect(body.name).not.toContain('<script>');
  }
});
```

**5. Concurrency and Race Conditions**
```typescript
// What if two users register with the same email simultaneously?
it('should handle concurrent duplicate registrations', async () => {
  const payload = {
    email: 'race@example.com',
    password: 'ValidPass1',
    name: 'Test User',
  };

  const [response1, response2] = await Promise.all([
    app.inject({ method: 'POST', url: '/api/users/register', payload }),
    app.inject({ method: 'POST', url: '/api/users/register', payload }),
  ]);

  const statuses = [response1.statusCode, response2.statusCode].sort();
  expect(statuses).toEqual([201, 409]);
});
```

### The Spec Gap Discovery Process

One of the most powerful side-effects of mapping specs to tests is discovering gaps in the spec. Here's a systematic process:

```
For each spec constraint:
  1. Identify the happy path -> write a positive test
  2. Identify the boundary -> write boundary tests
  3. Ask: "What does the spec NOT say?" -> these are spec gaps
  4. For each gap:
     a. Is this an oversight? -> Update the spec
     b. Is this intentionally unspecified? -> Document it as such
     c. Is this ambiguous? -> Clarify with stakeholders
  5. Write tests for the gaps you've identified
```

> **Professor's Aside:** This process is where junior developers become senior developers. It's also where AI-assisted development becomes *truly* spec-driven. The AI will happily implement whatever you tell it. The value you bring is asking: "What did we forget to tell it?"

---

## 1.5 Testing Frameworks and Their Role in SDD

### Choosing the Right Framework

The framework you choose matters less than *how* you use it. That said, here's the current landscape and how each fits into SDD:

#### JavaScript/TypeScript: Vitest and Jest

**Vitest** has become the dominant testing framework in the JavaScript ecosystem for new projects (2025-2026). It's fast, compatible with the Jest API, and integrates natively with Vite-based projects.

```typescript
// vitest.config.ts
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    coverage: {
      reporter: ['text', 'json', 'html'],
      // Spec coverage configuration
      thresholds: {
        branches: 90,
        functions: 90,
        lines: 90,
        statements: 90,
      },
    },
  },
});
```

**Jest** remains widely used, especially in React ecosystems and enterprise codebases. Its snapshot testing feature is particularly useful for UI specs:

```typescript
// Snapshot testing as spec validation
it('should render the user profile card according to spec', () => {
  const { container } = render(
    <UserProfileCard
      name="Jane Doe"
      email="jane@example.com"
      role="admin"
    />
  );

  // First run: creates the snapshot (the "spec" of the rendered output)
  // Subsequent runs: compares against the snapshot
  expect(container).toMatchSnapshot();
});
```

#### Python: Pytest

**Pytest** dominates the Python testing landscape and excels at SDD-style testing with its fixture system and parameterization:

```python
# test_user_registration.py
import pytest
from app import create_app
from database import reset_db


@pytest.fixture
def client():
    app = create_app(testing=True)
    with app.test_client() as client:
        yield client
    reset_db()


class TestUserRegistration:
    """
    Tests mapped to: specs/user-registration.md
    """

    # Spec Line 3: SHALL return 201 on success
    def test_successful_registration_returns_201(self, client):
        response = client.post('/api/users/register', json={
            'email': 'test@example.com',
            'password': 'ValidPass1',
            'name': 'Test User',
        })
        assert response.status_code == 201

    # Spec Line 7: Password at least 8 characters
    @pytest.mark.parametrize('password,expected_status', [
        ('Short1A', 400),       # 7 chars - too short
        ('Valid1Aa', 201),      # 8 chars - exactly at boundary
        ('LongValid1Aa', 201), # 12 chars - above boundary
    ])
    def test_password_length_boundaries(self, client, password, expected_status):
        response = client.post('/api/users/register', json={
            'email': 'test@example.com',
            'password': password,
            'name': 'Test User',
        })
        assert response.status_code == expected_status

    # Spec Line 8: Password complexity requirements
    @pytest.mark.parametrize('password,missing_requirement', [
        ('alllowercase1', 'uppercase letter'),
        ('ALLUPPERCASE1', 'lowercase letter'),
        ('NoNumbersHere', 'number'),
    ])
    def test_password_complexity_requirements(self, client, password, missing_requirement):
        response = client.post('/api/users/register', json={
            'email': 'test@example.com',
            'password': password,
            'name': 'Test User',
        })
        assert response.status_code == 400, (
            f"Password missing {missing_requirement} should be rejected"
        )
```

#### End-to-End: Playwright

**Playwright** handles full-stack spec validation -- testing not just the API but the entire user flow:

```typescript
// e2e/registration.spec.ts
import { test, expect } from '@playwright/test';

test.describe('User Registration Flow', () => {
  // Spec: "The registration form SHALL be accessible at /register"
  test('registration page is accessible', async ({ page }) => {
    await page.goto('/register');
    await expect(page).toHaveURL('/register');
    await expect(page.locator('h1')).toHaveText('Create Account');
  });

  // Spec: "The form SHALL display validation errors inline"
  test('displays inline validation errors for invalid email', async ({ page }) => {
    await page.goto('/register');
    await page.fill('[name="email"]', 'not-an-email');
    await page.fill('[name="password"]', 'ValidPass1');
    await page.fill('[name="name"]', 'Test User');
    await page.click('button[type="submit"]');

    await expect(page.locator('.error-email')).toBeVisible();
    await expect(page.locator('.error-email')).toHaveText('Invalid email format');
  });

  // Spec: "On successful registration, redirect to /dashboard"
  test('redirects to dashboard on successful registration', async ({ page }) => {
    await page.goto('/register');
    await page.fill('[name="email"]', 'newuser@example.com');
    await page.fill('[name="password"]', 'ValidPass1');
    await page.fill('[name="name"]', 'New User');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('.welcome-message')).toContainText('New User');
  });
});
```

---

## 1.6 How Anthropic Tests Claude's Outputs Against Behavioral Specs

### Constitutional AI as a Testing Framework

This is a fascinating real-world example of spec-driven validation at scale.

Anthropic's approach to aligning Claude is fundamentally a spec-driven process:

1. **The "Constitution"** is a spec -- a set of behavioral principles that define how Claude should respond
2. **Evaluations (evals)** are the tests -- automated checks that verify Claude's behavior matches the spec
3. **Training** is the implementation -- the model weights are adjusted until the tests pass

This mirrors our pipeline exactly:

```
Constitution (Spec) --> Evals (Tests) --> Training (Implementation) --> Evaluation (Validation)
```

The evals Anthropic runs include:

- **Behavioral tests**: "Given this prompt, does Claude refuse to help with harmful requests?"
- **Consistency tests**: "Does Claude give consistent answers to semantically identical questions?"
- **Boundary tests**: "Where is the line between a helpful response and a harmful one?"
- **Regression tests**: "Did the latest training run break any previously correct behaviors?"

> **Professor's Aside:** Notice the parallel to our software specs. Anthropic is essentially writing specs for *behavior* rather than *code*, but the testing methodology is identical. Every principle in the constitution maps to test cases. Every test case validates a specific behavior. Every training run is evaluated against the full test suite. This is SDD at the model level.

### What We Can Learn From Anthropic's Approach

The key takeaways for your own spec-to-test mapping:

1. **Specs can be hierarchical.** Anthropic's constitution has high-level principles ("Be helpful, harmless, and honest") that decompose into specific behavioral rules. Your specs should work the same way.

2. **Tests should be automated and continuous.** Anthropic doesn't manually check Claude's behavior -- they run thousands of automated evals. Your test suite should run on every commit.

3. **Edge cases are where alignment fails.** Just as Claude can behave unexpectedly at the boundaries of its training, your AI-generated code will have the most issues at boundary conditions.

4. **The spec evolves with the tests.** Anthropic continuously updates their constitution based on what the evals reveal. Your specs should evolve based on what your tests uncover.

---

## 1.7 How Google's DeepMind Validates AI Behavior Against Specifications

### Formal Verification Meets AI

Google DeepMind takes a more formal approach to spec validation. Their work on AlphaProof and related systems uses mathematical proofs to verify that AI-generated solutions are correct.

The process:

1. **Formal specification**: The problem is stated in a formal language (e.g., Lean, Isabelle)
2. **AI generation**: The AI proposes a solution
3. **Proof verification**: A theorem prover checks that the solution satisfies the specification
4. **Iteration**: If verification fails, the AI tries again with the failure as feedback

This is the most rigorous form of the spec-to-test pipeline:

```
Formal Spec --> Proof Obligation (Tests) --> AI Solution --> Proof Checker (Validation)
```

While formal verification is overkill for most application development, the *principle* is powerful: **the specification should be precise enough that verification can be automated.**

### Practical Lessons from DeepMind's Approach

For everyday SDD work, take these lessons from DeepMind:

1. **Make your specs as precise as possible.** Ambiguity in the spec means ambiguity in the tests, which means ambiguity in the implementation. The more precise your spec, the more useful your tests.

2. **Use the AI's failures as feedback.** When the AI generates code that fails tests, don't just re-prompt. Analyze *why* it failed. Was the spec ambiguous? Was the test wrong? Was the AI's approach fundamentally flawed?

3. **Consider invariants.** DeepMind's formal specs often include *invariants* -- properties that must always hold true. In your specs, these might be: "The total should always equal the sum of line items" or "A user's email should never change after registration."

---

## 1.8 Property-Based Testing as Spec Validation

### Beyond Example-Based Tests

Everything we've written so far has been *example-based* testing: "Given this specific input, expect this specific output." But specs often describe *properties* -- general rules that should hold for *any* valid input.

This is where **property-based testing** shines.

### fast-check (JavaScript/TypeScript)

```typescript
import { describe, it, expect } from 'vitest';
import * as fc from 'fast-check';
import { validateEmail, validatePassword, registerUser } from '../src/registration';

describe('Registration Properties', () => {

  // Property: Any valid email should be accepted
  it('should accept any well-formed email', () => {
    fc.assert(
      fc.property(
        fc.emailAddress(),
        (email) => {
          const result = validateEmail(email);
          expect(result.valid).toBe(true);
        }
      )
    );
  });

  // Property: Any password meeting complexity requirements should be accepted
  it('should accept any password meeting all requirements', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 8, maxLength: 128 }).filter(pw => {
          return /[A-Z]/.test(pw) && /[a-z]/.test(pw) && /[0-9]/.test(pw);
        }),
        (password) => {
          const result = validatePassword(password);
          expect(result.valid).toBe(true);
        }
      )
    );
  });

  // Property: Registration should never return the password
  it('should never include password in any successful response', () => {
    fc.assert(
      fc.property(
        fc.emailAddress(),
        fc.string({ minLength: 8, maxLength: 128 }).filter(pw => {
          return /[A-Z]/.test(pw) && /[a-z]/.test(pw) && /[0-9]/.test(pw);
        }),
        fc.string({ minLength: 1, maxLength: 100 }),
        async (email, password, name) => {
          const result = await registerUser({ email, password, name });
          if (result.success) {
            const responseStr = JSON.stringify(result.user);
            expect(responseStr).not.toContain(password);
          }
        }
      )
    );
  });

  // Property: Idempotency - registering the same email twice should always fail the second time
  it('should always reject duplicate emails regardless of other fields', () => {
    fc.assert(
      fc.property(
        fc.emailAddress(),
        fc.string({ minLength: 8, maxLength: 128 }).filter(pw => {
          return /[A-Z]/.test(pw) && /[a-z]/.test(pw) && /[0-9]/.test(pw);
        }),
        fc.string({ minLength: 1, maxLength: 100 }),
        fc.string({ minLength: 8, maxLength: 128 }).filter(pw => {
          return /[A-Z]/.test(pw) && /[a-z]/.test(pw) && /[0-9]/.test(pw);
        }),
        fc.string({ minLength: 1, maxLength: 100 }),
        async (email, pw1, name1, pw2, name2) => {
          await registerUser({ email, password: pw1, name: name1 });
          const result = await registerUser({ email, password: pw2, name: name2 });
          expect(result.success).toBe(false);
          expect(result.statusCode).toBe(409);
        }
      )
    );
  });
});
```

### Hypothesis (Python)

```python
# test_registration_properties.py
from hypothesis import given, settings, assume
from hypothesis.strategies import (
    emails, text, integers,
    from_regex, composite
)
import re
from app.registration import validate_email, validate_password, register_user


@composite
def valid_passwords(draw):
    """Generate passwords that meet all complexity requirements."""
    # Ensure at least one uppercase, one lowercase, one digit
    password = draw(text(min_size=8, max_size=128))
    assume(re.search(r'[A-Z]', password))
    assume(re.search(r'[a-z]', password))
    assume(re.search(r'[0-9]', password))
    return password


class TestRegistrationProperties:

    # Property: Spec Line 5 - password never in response
    @given(
        email=emails(),
        password=valid_passwords(),
        name=text(min_size=1, max_size=100)
    )
    @settings(max_examples=200)
    def test_password_never_in_response(self, email, password, name):
        result = register_user(email=email, password=password, name=name)
        if result['success']:
            response_str = str(result['user'])
            assert password not in response_str, (
                f"Password '{password}' found in response!"
            )

    # Property: Spec Line 6 - invalid emails always rejected
    @given(email=text(min_size=1, max_size=200).filter(
        lambda e: '@' not in e or '.' not in e.split('@')[-1]
    ))
    def test_invalid_emails_always_rejected(self, email):
        result = validate_email(email)
        assert not result['valid'], (
            f"Invalid email '{email}' was accepted"
        )

    # Property: Spec Line 7 - short passwords always rejected
    @given(password=text(min_size=0, max_size=7))
    def test_short_passwords_always_rejected(self, password):
        result = validate_password(password)
        assert not result['valid'], (
            f"Short password (len={len(password)}) was accepted"
        )
```

### Why Property-Based Testing Matters for SDD

Property-based testing is particularly powerful in SDD because:

1. **Specs describe properties, not examples.** When your spec says "passwords must be at least 8 characters," that's a *property* -- it should hold for *all* passwords, not just the three you happened to test.

2. **It catches edge cases you didn't think of.** Property-based testing frameworks generate hundreds or thousands of random inputs, often finding edge cases that neither you nor the AI anticipated.

3. **It validates the *intent* of the spec.** Example-based tests check specific cases. Property-based tests check the general rule -- which is closer to what the spec actually describes.

4. **It's excellent at finding AI hallucination bugs.** If the AI implemented a validation function that works for typical inputs but fails for unusual ones (emoji in passwords, very long strings, null bytes), property-based testing will find it.

---

## 1.9 The Concept of "Spec Coverage"

### Beyond Code Coverage

You're familiar with code coverage: what percentage of your code is executed by tests? Spec coverage asks a different question: **what percentage of your spec is validated by tests?**

Code coverage can be 100% and still miss the point if the code doesn't match the spec. Spec coverage ensures that every requirement in the spec has corresponding test validation.

### Measuring Spec Coverage

Here's a practical approach to measuring spec coverage:

```markdown
## Spec Coverage Report: User Registration

| Spec Line | Description                        | Test Coverage | Status    |
|-----------|------------------------------------|---------------|-----------|
| 1         | POST /api/users/register          | 1 test        | COVERED   |
| 2         | Request body: email, pw, name     | 3 tests       | COVERED   |
| 3         | 201 on success                    | 1 test        | COVERED   |
| 4         | Response: id, email, name, date   | 1 test        | COVERED   |
| 5         | No password in response           | 2 tests       | COVERED   |
| 6         | Valid email format                 | 6 tests       | COVERED   |
| 7         | Password >= 8 chars               | 3 tests       | COVERED   |
| 8         | Password complexity               | 3 tests       | COVERED   |
| 9         | Name 1-100 chars                  | 4 tests       | COVERED   |
| 10        | Unique email                      | 2 tests       | COVERED   |
| 11        | Error: invalid email              | 1 test        | COVERED   |
| 12        | Error: weak password              | 1 test        | COVERED   |
| 13        | Error: duplicate email            | 1 test        | COVERED   |
| 14        | Error: missing fields             | 3 tests       | COVERED   |
|           |                                    |               |           |
| TOTAL     | 14 spec lines                     | 32 tests      | 100%      |
```

### Automating Spec Coverage

You can automate spec coverage tracking by embedding spec references in your test files:

```typescript
/**
 * @spec user-registration
 * @specLine 5
 * @requirement Password SHALL NOT be included in the response
 */
it('should NOT include password in the response', async () => {
  // ...
});
```

Then build a simple tool that:

1. Parses your spec files to extract all requirement lines
2. Parses your test files to extract `@specLine` annotations
3. Generates a coverage report showing which spec lines have tests and which don't

```typescript
// tools/spec-coverage.ts
import { readFileSync, readdirSync } from 'fs';
import { join } from 'path';

interface SpecLine {
  lineNumber: number;
  text: string;
  testsCovering: string[];
}

interface CoverageReport {
  specFile: string;
  totalLines: number;
  coveredLines: number;
  coverage: number;
  lines: SpecLine[];
}

function extractSpecLines(specPath: string): SpecLine[] {
  const content = readFileSync(specPath, 'utf-8');
  const lines = content.split('\n');
  const specLines: SpecLine[] = [];
  let lineNumber = 0;

  for (const line of lines) {
    // Match numbered requirements (e.g., "1. The endpoint SHALL...")
    const match = line.match(/^\d+\.\s+.*(SHALL|MUST|SHOULD).*/i);
    if (match) {
      lineNumber++;
      specLines.push({
        lineNumber,
        text: line.trim(),
        testsCovering: [],
      });
    }
  }

  return specLines;
}

function extractTestAnnotations(testDir: string): Map<number, string[]> {
  const annotations = new Map<number, string[]>();
  const files = readdirSync(testDir, { recursive: true }) as string[];

  for (const file of files) {
    if (!file.endsWith('.spec.ts') && !file.endsWith('.test.ts')) continue;

    const content = readFileSync(join(testDir, file), 'utf-8');
    const regex = /@specLine\s+(\d+)/g;
    let match;

    while ((match = regex.exec(content)) !== null) {
      const lineNum = parseInt(match[1]);
      if (!annotations.has(lineNum)) {
        annotations.set(lineNum, []);
      }
      annotations.get(lineNum)!.push(file);
    }
  }

  return annotations;
}

function generateReport(specPath: string, testDir: string): CoverageReport {
  const specLines = extractSpecLines(specPath);
  const annotations = extractTestAnnotations(testDir);

  for (const line of specLines) {
    line.testsCovering = annotations.get(line.lineNumber) || [];
  }

  const coveredLines = specLines.filter(l => l.testsCovering.length > 0).length;

  return {
    specFile: specPath,
    totalLines: specLines.length,
    coveredLines,
    coverage: (coveredLines / specLines.length) * 100,
    lines: specLines,
  };
}

// Usage
const report = generateReport(
  'specs/user-registration.md',
  'test/'
);

console.log(`Spec Coverage: ${report.coverage.toFixed(1)}%`);
console.log(`${report.coveredLines}/${report.totalLines} spec lines covered\n`);

for (const line of report.lines) {
  const status = line.testsCovering.length > 0 ? 'COVERED' : 'MISSING';
  const icon = status === 'COVERED' ? '[+]' : '[-]';
  console.log(`  ${icon} Line ${line.lineNumber}: ${line.text}`);
  if (line.testsCovering.length > 0) {
    for (const test of line.testsCovering) {
      console.log(`      -> ${test}`);
    }
  }
}
```

> **Professor's Aside:** Spec coverage is one of those metrics that, once you start tracking it, you can never go back. You'll look at test suites with 95% code coverage and realize they only cover 40% of the spec. That's the gap where bugs live.

---

## 1.10 Practical Walkthrough: From Micro-Spec to Full Test Suite to Implementation

Let's do a complete walkthrough. We'll take a small spec, generate a full test suite, and then implement the code -- in that order.

### Step 1: The Micro-Spec

```markdown
## Spec: Price Calculator

### Purpose
Calculate the total price of items in a shopping cart, applying discounts and tax.

### Functional Requirements
1. The function SHALL accept an array of cart items
2. Each cart item SHALL have: name (string), price (number), quantity (number)
3. The function SHALL return an object with: subtotal, discount, tax, total
4. Subtotal SHALL be the sum of (price * quantity) for all items

### Discount Rules
5. If subtotal >= $100, apply a 10% discount
6. If subtotal >= $200, apply a 15% discount (overrides rule 5)
7. If subtotal >= $500, apply a 20% discount (overrides rule 6)
8. Discount SHALL be applied to the subtotal

### Tax Rules
9. Tax rate SHALL be 8.5%
10. Tax SHALL be calculated on the discounted subtotal (subtotal - discount)

### Constraints
11. Prices SHALL be non-negative numbers
12. Quantities SHALL be positive integers
13. All monetary values in the response SHALL be rounded to 2 decimal places
14. An empty cart SHALL return all zeros

### Error Handling
15. Invalid items (negative price, zero quantity, etc.) SHALL throw a ValidationError
```

### Step 2: The Test Suite (Written BEFORE Implementation)

```typescript
// test/price-calculator.spec.ts
import { describe, it, expect } from 'vitest';
import { calculatePrice, ValidationError } from '../src/price-calculator';

describe('Price Calculator', () => {

  // ===========================================
  // Functional Requirements (Spec Lines 1-4)
  // ===========================================

  /** @specLine 1 - Accepts array of cart items */
  it('should accept an array of cart items', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 10.00, quantity: 1 },
    ]);
    expect(result).toBeDefined();
  });

  /** @specLine 3 - Returns object with subtotal, discount, tax, total */
  it('should return object with subtotal, discount, tax, and total', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 10.00, quantity: 1 },
    ]);
    expect(result).toHaveProperty('subtotal');
    expect(result).toHaveProperty('discount');
    expect(result).toHaveProperty('tax');
    expect(result).toHaveProperty('total');
  });

  /** @specLine 4 - Subtotal is sum of price * quantity */
  it('should calculate subtotal as sum of price * quantity', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 10.00, quantity: 2 },
      { name: 'Gadget', price: 25.00, quantity: 3 },
    ]);
    // 10*2 + 25*3 = 20 + 75 = 95
    expect(result.subtotal).toBe(95.00);
  });

  // ===========================================
  // Discount Rules (Spec Lines 5-8)
  // ===========================================

  /** @specLine 5 - No discount below $100 */
  it('should apply no discount when subtotal < $100', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 99.99, quantity: 1 },
    ]);
    expect(result.discount).toBe(0);
  });

  /** @specLine 5 - 10% discount at exactly $100 */
  it('should apply 10% discount when subtotal is exactly $100', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 100.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(10.00); // 10% of 100
  });

  /** @specLine 5 - 10% discount between $100 and $199.99 */
  it('should apply 10% discount when subtotal is $150', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 150.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(15.00); // 10% of 150
  });

  /** @specLine 6 - 15% discount at exactly $200 */
  it('should apply 15% discount when subtotal is exactly $200', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 200.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(30.00); // 15% of 200
  });

  /** @specLine 6 - 15% discount between $200 and $499.99 */
  it('should apply 15% discount when subtotal is $350', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 350.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(52.50); // 15% of 350
  });

  /** @specLine 7 - 20% discount at exactly $500 */
  it('should apply 20% discount when subtotal is exactly $500', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 500.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(100.00); // 20% of 500
  });

  /** @specLine 7 - 20% discount above $500 */
  it('should apply 20% discount when subtotal is $1000', () => {
    const result = calculatePrice([
      { name: 'Widget', price: 1000.00, quantity: 1 },
    ]);
    expect(result.discount).toBe(200.00); // 20% of 1000
  });

  // Boundary test: $99.99 vs $100.00
  it('should not discount at $99.99 but should at $100.00', () => {
    const below = calculatePrice([
      { name: 'Widget', price: 99.99, quantity: 1 },
    ]);
    const atBoundary = calculatePrice([
      { name: 'Widget', price: 100.00, quantity: 1 },
    ]);
    expect(below.discount).toBe(0);
    expect(atBoundary.discount).toBe(10.00);
  });

  // Boundary test: $199.99 vs $200.00
  it('should apply 10% at $199.99 and 15% at $200.00', () => {
    const below = calculatePrice([
      { name: 'Widget', price: 199.99, quantity: 1 },
    ]);
    const atBoundary = calculatePrice([
      { name: 'Widget', price: 200.00, quantity: 1 },
    ]);
    expect(below.discount).toBeCloseTo(20.00, 2);  // 10% of 199.99
    expect(atBoundary.discount).toBe(30.00);        // 15% of 200
  });

  // ===========================================
  // Tax Rules (Spec Lines 9-10)
  // ===========================================

  /** @specLine 9-10 - Tax on discounted subtotal */
  it('should calculate tax at 8.5% on discounted subtotal', () => {
    // Subtotal: $100, Discount: 10% = $10, Taxable: $90
    // Tax: $90 * 0.085 = $7.65
    const result = calculatePrice([
      { name: 'Widget', price: 100.00, quantity: 1 },
    ]);
    expect(result.tax).toBe(7.65);
  });

  it('should calculate tax on full subtotal when no discount', () => {
    // Subtotal: $50, Discount: $0, Taxable: $50
    // Tax: $50 * 0.085 = $4.25
    const result = calculatePrice([
      { name: 'Widget', price: 50.00, quantity: 1 },
    ]);
    expect(result.tax).toBe(4.25);
  });

  // ===========================================
  // Total Calculation
  // ===========================================

  it('should calculate total as subtotal - discount + tax', () => {
    // Subtotal: $100, Discount: $10 (10%), Taxable: $90
    // Tax: $90 * 0.085 = $7.65
    // Total: $100 - $10 + $7.65 = $97.65
    const result = calculatePrice([
      { name: 'Widget', price: 100.00, quantity: 1 },
    ]);
    expect(result.total).toBe(97.65);
  });

  // ===========================================
  // Constraints (Spec Lines 11-14)
  // ===========================================

  /** @specLine 13 - Rounding to 2 decimal places */
  it('should round all values to 2 decimal places', () => {
    // Subtotal: $33.33 * 3 = $99.99
    // No discount (under $100)
    // Tax: $99.99 * 0.085 = $8.49915 -> $8.50
    const result = calculatePrice([
      { name: 'Widget', price: 33.33, quantity: 3 },
    ]);
    expect(result.subtotal).toBe(99.99);
    expect(result.tax).toBe(8.50);
    expect(result.total).toBe(108.49);
  });

  /** @specLine 14 - Empty cart returns all zeros */
  it('should return all zeros for empty cart', () => {
    const result = calculatePrice([]);
    expect(result.subtotal).toBe(0);
    expect(result.discount).toBe(0);
    expect(result.tax).toBe(0);
    expect(result.total).toBe(0);
  });

  // ===========================================
  // Error Handling (Spec Line 15)
  // ===========================================

  /** @specLine 15 - Negative price throws ValidationError */
  it('should throw ValidationError for negative price', () => {
    expect(() => calculatePrice([
      { name: 'Widget', price: -10.00, quantity: 1 },
    ])).toThrow(ValidationError);
  });

  /** @specLine 15 - Zero quantity throws ValidationError */
  it('should throw ValidationError for zero quantity', () => {
    expect(() => calculatePrice([
      { name: 'Widget', price: 10.00, quantity: 0 },
    ])).toThrow(ValidationError);
  });

  /** @specLine 15 - Negative quantity throws ValidationError */
  it('should throw ValidationError for negative quantity', () => {
    expect(() => calculatePrice([
      { name: 'Widget', price: 10.00, quantity: -1 },
    ])).toThrow(ValidationError);
  });

  /** @specLine 15 - Non-integer quantity throws ValidationError */
  it('should throw ValidationError for non-integer quantity', () => {
    expect(() => calculatePrice([
      { name: 'Widget', price: 10.00, quantity: 1.5 },
    ])).toThrow(ValidationError);
  });
});
```

### Step 3: The Implementation (Written AFTER Tests)

Now -- and only now -- do we write (or ask the AI to write) the implementation:

```typescript
// src/price-calculator.ts

export class ValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ValidationError';
  }
}

interface CartItem {
  name: string;
  price: number;
  quantity: number;
}

interface PriceResult {
  subtotal: number;
  discount: number;
  tax: number;
  total: number;
}

const TAX_RATE = 0.085;

const DISCOUNT_TIERS = [
  { threshold: 500, rate: 0.20 },
  { threshold: 200, rate: 0.15 },
  { threshold: 100, rate: 0.10 },
];

function round2(value: number): number {
  return Math.round(value * 100) / 100;
}

function validateItem(item: CartItem): void {
  if (item.price < 0) {
    throw new ValidationError(
      `Invalid price for "${item.name}": ${item.price}. Price must be non-negative.`
    );
  }
  if (item.quantity <= 0) {
    throw new ValidationError(
      `Invalid quantity for "${item.name}": ${item.quantity}. Quantity must be a positive integer.`
    );
  }
  if (!Number.isInteger(item.quantity)) {
    throw new ValidationError(
      `Invalid quantity for "${item.name}": ${item.quantity}. Quantity must be an integer.`
    );
  }
}

function getDiscountRate(subtotal: number): number {
  for (const tier of DISCOUNT_TIERS) {
    if (subtotal >= tier.threshold) {
      return tier.rate;
    }
  }
  return 0;
}

export function calculatePrice(items: CartItem[]): PriceResult {
  // Validate all items first
  for (const item of items) {
    validateItem(item);
  }

  // Calculate subtotal (Spec Line 4)
  const subtotal = round2(
    items.reduce((sum, item) => sum + item.price * item.quantity, 0)
  );

  // Calculate discount (Spec Lines 5-8)
  const discountRate = getDiscountRate(subtotal);
  const discount = round2(subtotal * discountRate);

  // Calculate tax on discounted amount (Spec Lines 9-10)
  const taxableAmount = subtotal - discount;
  const tax = round2(taxableAmount * TAX_RATE);

  // Calculate total
  const total = round2(subtotal - discount + tax);

  return { subtotal, discount, tax, total };
}
```

### Step 4: Run the Tests

```bash
$ npx vitest run test/price-calculator.spec.ts

 ✓ Price Calculator
   ✓ should accept an array of cart items
   ✓ should return object with subtotal, discount, tax, and total
   ✓ should calculate subtotal as sum of price * quantity
   ✓ should apply no discount when subtotal < $100
   ✓ should apply 10% discount when subtotal is exactly $100
   ✓ should apply 10% discount when subtotal is $150
   ✓ should apply 15% discount when subtotal is exactly $200
   ✓ should apply 15% discount when subtotal is $350
   ✓ should apply 20% discount when subtotal is exactly $500
   ✓ should apply 20% discount when subtotal is $1000
   ✓ should not discount at $99.99 but should at $100.00
   ✓ should apply 10% at $199.99 and 15% at $200.00
   ✓ should calculate tax at 8.5% on discounted subtotal
   ✓ should calculate tax on full subtotal when no discount
   ✓ should calculate total as subtotal - discount + tax
   ✓ should round all values to 2 decimal places
   ✓ should return all zeros for empty cart
   ✓ should throw ValidationError for negative price
   ✓ should throw ValidationError for zero quantity
   ✓ should throw ValidationError for negative quantity
   ✓ should throw ValidationError for non-integer quantity

 Test Files  1 passed (1)
      Tests  21 passed (21)
```

All 21 tests pass. The implementation satisfies the spec -- at least as far as these tests validate.

> **Professor's Aside:** Notice what happened here. We wrote the tests first. Every test maps to a specific spec line. The implementation was written *to satisfy the tests*, which themselves encode the spec. If the AI had hallucinated a different discount formula, the tests would have caught it immediately. This is the power of the pipeline.

---

## 1.11 Teaching the AI to Write Tests FROM the Spec

### The Prompt Engineering for Test Generation

When using an AI to generate tests from a spec, your prompt structure matters enormously. Here's a template that works well:

```markdown
## Task: Generate a test suite from the following specification.

### Rules:
1. Generate tests ONLY -- do not generate the implementation
2. Every spec requirement must have at least one test
3. Include boundary tests for all numeric constraints
4. Include negative tests for all error conditions
5. Annotate each test with the spec line number it validates
6. Use [framework: Vitest/Jest/Pytest] with [language: TypeScript/Python]

### Specification:
[paste your spec here]

### Output Format:
- Complete, runnable test file
- Tests should import from a module that does not yet exist
- Include setup/teardown if needed
- Group tests by spec section
```

### Common AI Test Generation Mistakes

Even with good prompts, AI models tend to make these mistakes when generating tests:

1. **Testing the implementation instead of the spec.** The AI might write tests that verify internal implementation details rather than external behavior. Catch this by asking: "Could a different implementation pass this test?"

2. **Missing boundary tests.** The AI will usually write the happy path and obvious error cases but skip the boundary tests (e.g., exactly at the threshold).

3. **Weak assertions.** The AI might write `expect(result).toBeDefined()` when it should write `expect(result.subtotal).toBe(95.00)`. Vague assertions are nearly useless.

4. **Not testing error messages.** If the spec defines specific error messages, the AI often tests only the status code and not the message content.

5. **Assuming implementation details.** The AI might write tests that assume a specific internal structure (e.g., testing that a specific private method was called) rather than testing the public interface.

### Validating AI-Generated Tests

Always review AI-generated tests against this checklist:

- [ ] Does every spec requirement have at least one test?
- [ ] Are boundary conditions tested on both sides?
- [ ] Are error messages validated exactly as specified?
- [ ] Are negative requirements tested (things that should NOT happen)?
- [ ] Are assertions specific (not just "is defined" or "is truthy")?
- [ ] Could a completely wrong implementation accidentally pass these tests?
- [ ] Are concurrency and race conditions considered?
- [ ] Is each test independent (no shared state that could cause flaky tests)?

---

## 1.12 Exercises

### Exercise 1: Spec-to-Test Mapping (Beginner)

Given the following micro-spec, write a complete test suite. Do NOT write the implementation.

```markdown
## Spec: Temperature Converter

### Requirements
1. The module SHALL export a function `convert(value, from, to)`
2. Supported units: "celsius", "fahrenheit", "kelvin"
3. Conversion SHALL be accurate to 2 decimal places
4. Converting a unit to itself SHALL return the same value
5. Kelvin values SHALL never be negative (absolute zero = 0K)
6. Invalid unit names SHALL throw an Error with message "Unknown unit: [unit]"

### Conversion Formulas
7. Celsius to Fahrenheit: F = C * 9/5 + 32
8. Fahrenheit to Celsius: C = (F - 32) * 5/9
9. Celsius to Kelvin: K = C + 273.15
10. Kelvin to Celsius: C = K - 273.15
```

**Your deliverable:** A complete test file with at least 15 test cases, each annotated with the spec line it validates. Include boundary tests for the Kelvin constraint.

### Exercise 2: Find the Spec Gaps (Intermediate)

The following test suite was generated by an AI. It has 100% code coverage but only ~60% spec coverage. Identify the gaps.

```typescript
describe('URL Shortener', () => {
  it('should shorten a URL', async () => {
    const result = await shorten('https://example.com/very/long/path');
    expect(result.shortUrl).toBeDefined();
    expect(result.shortUrl.length).toBeLessThan(30);
  });

  it('should redirect to original URL', async () => {
    const { shortUrl } = await shorten('https://example.com');
    const result = await resolve(shortUrl);
    expect(result).toBe('https://example.com');
  });

  it('should handle invalid URLs', async () => {
    await expect(shorten('not-a-url')).rejects.toThrow();
  });
});
```

**Your deliverable:** List at least 10 additional test cases that should exist, based on what a reasonable URL shortener spec would contain. For each, explain what spec requirement it would validate.

### Exercise 3: Property-Based Testing (Advanced)

Write property-based tests using fast-check (JavaScript) or Hypothesis (Python) for the following spec:

```markdown
## Spec: Sorting Function

### Requirements
1. The function SHALL sort an array of numbers in ascending order
2. The sorted array SHALL have the same length as the input
3. The sorted array SHALL contain exactly the same elements as the input
4. Each element SHALL be less than or equal to the next element
5. The function SHALL handle empty arrays (returning [])
6. The function SHALL handle arrays with one element
7. The function SHALL handle arrays with duplicate values
8. Sorting an already-sorted array SHALL return an identical array
```

**Your deliverable:** A property-based test file with at least 6 properties that validate requirements 1-8. Each property should generate random arrays and verify the invariants.

### Exercise 4: Full Pipeline (Advanced)

Take any micro-spec you wrote in Module 1. Apply the full pipeline:

1. Review the spec for completeness
2. Generate a test suite (manually or with AI assistance)
3. Run the tests (they should all fail -- there's no implementation yet)
4. Implement the code (manually or with AI assistance)
5. Run the tests (they should all pass)
6. Generate a spec coverage report
7. Identify any spec gaps found during the process
8. Update the spec to address the gaps
9. Write additional tests for the updated spec

**Your deliverable:** The complete spec, test suite, implementation, and spec coverage report. Include a reflection on what spec gaps you found and how you addressed them.

---

## Summary

In this chapter, we've covered the foundational discipline of Spec-Driven Development: mapping specs to tests before writing any implementation code.

**Key takeaways:**

1. **Test-first is essential with AI.** The AI doesn't know what "correct" means unless you define it through tests derived from the spec.

2. **Every spec line maps to at least one test.** This is the fundamental traceability that makes SDD work.

3. **Boundary conditions are where bugs hide.** Extract them systematically from every constraint in the spec.

4. **Property-based testing validates spec *intent*.** Example-based tests check specific cases; property-based tests check general rules.

5. **Spec coverage matters more than code coverage.** Code coverage tells you if the code runs; spec coverage tells you if the code is correct.

6. **The pipeline is non-negotiable: Spec --> Tests --> Implementation --> Validation.** Breaking this order defeats the purpose.

In the next chapter, we'll look at how to prevent the AI from *drifting* away from the spec during implementation -- a phenomenon we call "intent drift," and how to build automated linting tools to catch it.

---

*Next: Chapter 2 -- Automated Linting of Intent*

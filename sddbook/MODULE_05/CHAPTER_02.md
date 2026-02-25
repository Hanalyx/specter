# Chapter 2: Documentation as Code

## MODULE 05 — Maintenance & Scaling (Advanced Level)

---

### Lecture Preamble

> *Let me tell you a story that every one of you has experienced. You are onboarding onto a new project. Someone points you to the documentation wiki. You open it. The last edit was nine months ago. The API endpoints listed do not match the actual API. The architecture diagram shows a microservice that was decommissioned in January. The "Getting Started" guide references a CLI tool that was replaced by a different one. You close the wiki and go read the code instead.*

> *This is the universal documentation problem: docs are always out of date. And the reason is simple — documentation and code are separate artifacts maintained by separate processes. When the code changes, someone has to remember to update the docs. And nobody remembers. Nobody ever remembers.*

> *But what if the documentation and the specification were the same thing? What if updating the spec automatically updated the docs? What if the docs could never drift from the implementation because they were generated from the same source of truth that the implementation was generated from?*

> *That is what we are going to build today. Not documentation that describes code, but documentation that IS code. Documentation that is generated, validated, and deployed with the same rigor as your production software. This is Documentation as Code, and it is one of the most transformative ideas in the SDD toolkit.*

---

## 2.1 The DRY Principle Applied to Documentation

You know the DRY principle — Don't Repeat Yourself. Every piece of knowledge should have a single, authoritative representation in a system. We apply this principle to code religiously, extracting shared logic into functions, creating abstractions to eliminate duplication. Yet when it comes to documentation, we violate DRY constantly.

Consider a typical project. The same information about an API endpoint exists in:

1. The TypeScript interface definition
2. The OpenAPI/Swagger spec
3. The route handler implementation
4. The unit tests
5. The integration tests
6. The API documentation website
7. The internal wiki
8. The onboarding guide
9. The client SDK documentation
10. The changelog

That is ten places where the same truth is recorded. When the endpoint changes, how many of those ten places get updated? In my experience, one — the implementation. Maybe two if you are lucky and the developer also updates the tests. The other eight become lies.

### The SDD Solution: Specs ARE the Documentation

In a Spec-Driven Development workflow, the specification is the single source of truth. Everything else — the implementation, the tests, AND the documentation — is derived from the spec.

```
                    ┌─────────────┐
                    │    SPEC     │ ← Single Source of Truth
                    │  (source)   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │   CODE   │ │  TESTS   │ │   DOCS   │
        │(derived) │ │(derived) │ │(derived) │
        └──────────┘ └──────────┘ └──────────┘
```

When the spec changes, all three derived artifacts are regenerated. The docs can never be out of date because they are not maintained independently — they are produced from the same spec that produces the code.

```typescript
// The spec IS the documentation. Look at this interface:

/**
 * @title User Management API
 * @version 2.0.0
 * @description Manages user accounts, authentication, and profiles.
 *
 * @baseUrl https://api.example.com/v2
 * @auth Bearer token required for all endpoints except POST /users
 */

/**
 * Creates a new user account.
 *
 * @endpoint POST /users
 * @tag Users
 * @tag Onboarding
 * @since 2.0.0
 *
 * @param {CreateUserDTO} body - The user data
 * @returns {UserResponseDTO} 201 - The created user
 * @throws {ValidationError} 400 - Invalid input data
 * @throws {ConflictError} 409 - Email already registered
 *
 * @example
 * // Request
 * POST /users
 * Content-Type: application/json
 * {
 *   "email": "ada@example.com",
 *   "name": "Ada Lovelace",
 *   "password": "securePassword123!"
 * }
 *
 * // Response (201 Created)
 * {
 *   "id": "usr_abc123",
 *   "email": "ada@example.com",
 *   "name": "Ada Lovelace",
 *   "createdAt": "2026-02-24T10:30:00Z"
 * }
 *
 * @rateLimit 10 requests per minute per IP
 * @sideEffects Sends welcome email via email service
 */
interface CreateUser {
  body: CreateUserDTO;
  response: UserResponseDTO;
  errors: ValidationError | ConflictError;
}
```

That interface is simultaneously:
- A **spec** that tells the AI what to implement
- A **type definition** that the compiler enforces
- **API documentation** that can be rendered into a website
- A **test contract** that defines what to validate
- A **client SDK blueprint** that defines method signatures

One source. Five uses. Zero duplication.

---

## 2.2 How Specs Eliminate the "Docs Are Always Out of Date" Problem

The reason documentation falls out of date is that it exists in a separate maintenance cycle from the code. Updating docs is a manual, afterthought process that has no enforcement mechanism. Let us compare the traditional workflow with the SDD workflow:

### Traditional Workflow

```
1. Developer receives feature request
2. Developer writes code
3. Developer writes tests (sometimes)
4. Developer submits PR
5. PR is reviewed and merged
6. Developer remembers they should update docs (rare)
7. Developer opens wiki, makes edits (rarer)
8. Edits are reviewed and published (rarest)

Result: Docs are 3 steps removed from the code change,
with no enforcement at any step.
```

### SDD Workflow

```
1. Developer receives feature request
2. Developer updates the SPEC
3. Code is regenerated/updated from spec
4. Tests are regenerated/updated from spec
5. Docs are regenerated/updated from spec
6. CI validates that code, tests, AND docs are in sync with spec
7. PR includes spec change, code change, test change, AND doc change
8. Review covers all four together

Result: Docs are updated in the same commit as the code,
enforced by CI, reviewed in the same PR.
```

### The CI Enforcement Layer

Here is how you build the enforcement mechanism:

```yaml
# .github/workflows/doc-sync.yml
name: Documentation Sync Check

on:
  pull_request:
    branches: [main]

jobs:
  check-doc-sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install dependencies
        run: npm ci

      - name: Generate docs from specs
        run: npm run generate:docs

      - name: Check for uncommitted doc changes
        run: |
          # If generating docs from specs produces changes,
          # it means the PR has spec changes without doc updates
          if [ -n "$(git status --porcelain docs/)" ]; then
            echo "ERROR: Documentation is out of sync with specs."
            echo "The following doc files need to be regenerated:"
            git status --porcelain docs/
            echo ""
            echo "Run 'npm run generate:docs' and commit the changes."
            exit 1
          fi

      - name: Validate generated docs
        run: |
          # Check that all links are valid
          npm run docs:validate-links
          # Check that all code examples compile
          npm run docs:validate-examples
          # Check that all API examples match spec types
          npm run docs:validate-contracts
```

> **Professor's Aside:** This CI check is the key that makes the entire system work. It is not optional. Without enforcement, developers will inevitably forget to regenerate docs, and you are back to the same problem. The CI check makes it physically impossible to merge a PR that has spec changes without corresponding doc updates. That is the difference between a policy and a process.

---

## 2.3 Tools of the Trade

Let us survey the tools that make Documentation as Code practical in 2026.

### 2.3.1 TypeDoc — TypeScript API Documentation

TypeDoc generates documentation from TypeScript source files and JSDoc comments. In an SDD workflow, your spec interfaces are your TypeDoc source.

```typescript
// src/specs/user.spec.ts
// This file IS your spec AND your doc source

/**
 * Represents a user in the system.
 *
 * Users are created through the registration flow and can
 * have one of several roles that determine their permissions.
 *
 * @category Domain Models
 * @see {@link UserService} for operations on users
 * @see {@link AuthService} for authentication
 *
 * @example
 * ```typescript
 * const user: User = {
 *   id: 'usr_abc123',
 *   email: 'ada@example.com',
 *   name: 'Ada Lovelace',
 *   role: UserRole.ADMIN,
 *   createdAt: new Date('2026-02-24'),
 *   updatedAt: new Date('2026-02-24'),
 * };
 * ```
 */
export interface User {
  /** Unique identifier. Format: usr_{alphanumeric} */
  id: string;

  /** User's email address. Must be unique across all users. */
  email: string;

  /** User's display name. 1-100 characters. */
  name: string;

  /** User's role. Determines permissions. */
  role: UserRole;

  /** When the user account was created. ISO 8601. */
  createdAt: Date;

  /** When the user account was last modified. ISO 8601. */
  updatedAt: Date;
}

/**
 * Available user roles.
 *
 * Roles are hierarchical: ADMIN > MANAGER > USER > VIEWER
 *
 * @category Domain Models
 */
export enum UserRole {
  /** Full system access. Can manage all users and settings. */
  ADMIN = 'admin',

  /** Can manage team members and their resources. */
  MANAGER = 'manager',

  /** Standard user. Can manage their own resources. */
  USER = 'user',

  /** Read-only access. Cannot create or modify resources. */
  VIEWER = 'viewer',
}
```

```json
// typedoc.json — Configuration for doc generation
{
  "entryPoints": ["src/specs/**/*.spec.ts"],
  "out": "docs/api-reference",
  "theme": "default",
  "includeVersion": true,
  "categorizeByGroup": true,
  "categoryOrder": [
    "Domain Models",
    "API Endpoints",
    "Services",
    "Utilities"
  ],
  "plugin": [
    "typedoc-plugin-markdown",
    "typedoc-plugin-mermaid"
  ],
  "excludePrivate": true,
  "excludeInternal": true
}
```

### 2.3.2 Swagger/OpenAPI — API Documentation

The OpenAPI specification is the industry standard for describing REST APIs. In SDD, your API specs can generate both the implementation and the OpenAPI document.

```yaml
# Generated from specs — not written by hand
# openapi.generated.yaml

openapi: 3.1.0
info:
  title: E-Commerce API
  version: 2.0.0
  description: |
    The E-Commerce API provides access to products, users, and orders.
    This documentation is auto-generated from the API specification files
    and is guaranteed to match the current implementation.

    **Last generated:** 2026-02-24T10:30:00Z
    **Spec hash:** a3f7b2c (for cache-busting)

servers:
  - url: https://api.example.com/v2
    description: Production
  - url: https://staging-api.example.com/v2
    description: Staging

paths:
  /products:
    get:
      operationId: listProducts
      summary: List products with pagination and filtering
      tags:
        - Products
      parameters:
        - name: page
          in: query
          required: true
          schema:
            type: integer
            minimum: 1
          description: Page number (1-indexed)
        - name: limit
          in: query
          required: true
          schema:
            type: integer
            minimum: 1
            maximum: 100
          description: Number of items per page
        - name: category
          in: query
          required: false
          schema:
            $ref: '#/components/schemas/ProductCategory'
          description: Filter by product category
      responses:
        '200':
          description: Paginated list of products
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PaginatedProductResponse'
              example:
                data:
                  - id: "prod_abc123"
                    name: "Mechanical Keyboard"
                    price: 149.99
                    category: "electronics"
                pagination:
                  page: 1
                  limit: 20
                  total: 156
                  totalPages: 8
        '400':
          $ref: '#/components/responses/ValidationError'
        '401':
          $ref: '#/components/responses/UnauthorizedError'
```

### 2.3.3 Storybook — Component Documentation

For frontend components, Storybook generates interactive documentation from component contracts. In SDD, your component specs define both the implementation contract and the Storybook stories.

```typescript
// src/specs/components/Button.spec.ts
// This spec drives both implementation AND documentation

/**
 * @component Button
 * @category UI Components
 * @figma https://figma.com/file/abc123/design-system
 *
 * @description
 * Primary interactive element for user actions.
 * Supports multiple variants, sizes, and states.
 */
export interface ButtonSpec {
  props: {
    /** The button's visual variant */
    variant: 'primary' | 'secondary' | 'danger' | 'ghost';

    /** Size of the button */
    size: 'sm' | 'md' | 'lg';

    /** Button label text */
    label: string;

    /** Whether the button is disabled */
    disabled?: boolean;

    /** Whether to show a loading spinner */
    loading?: boolean;

    /** Click handler */
    onClick: () => void;

    /** Optional icon to show before the label */
    icon?: React.ReactNode;
  };

  /** Accessibility requirements */
  a11y: {
    role: 'button';
    ariaLabel: string;
    ariaDisabled: boolean;
    focusable: boolean;
    keyboardActivation: 'Enter' | 'Space';
  };

  /** Visual states */
  states: {
    default: { cursor: 'pointer' };
    hover: { opacity: 0.9 };
    active: { transform: 'scale(0.98)' };
    disabled: { cursor: 'not-allowed'; opacity: 0.5 };
    loading: { cursor: 'wait' };
    focused: { outline: '2px solid blue' };
  };
}
```

```typescript
// Generated Storybook stories from the spec
// src/stories/Button.stories.ts (auto-generated)

import type { Meta, StoryObj } from '@storybook/react';
import { Button } from '../components/Button';

// Auto-generated from ButtonSpec
const meta: Meta<typeof Button> = {
  title: 'UI Components/Button',
  component: Button,
  tags: ['autodocs'],
  argTypes: {
    variant: {
      control: 'select',
      options: ['primary', 'secondary', 'danger', 'ghost'],
      description: "The button's visual variant",
    },
    size: {
      control: 'select',
      options: ['sm', 'md', 'lg'],
      description: 'Size of the button',
    },
    label: {
      control: 'text',
      description: 'Button label text',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the button is disabled',
    },
    loading: {
      control: 'boolean',
      description: 'Whether to show a loading spinner',
    },
  },
};

export default meta;
type Story = StoryObj<typeof Button>;

// Auto-generated stories for each variant
export const Primary: Story = {
  args: { variant: 'primary', size: 'md', label: 'Click Me' },
};

export const Secondary: Story = {
  args: { variant: 'secondary', size: 'md', label: 'Click Me' },
};

export const Danger: Story = {
  args: { variant: 'danger', size: 'md', label: 'Delete' },
};

export const Ghost: Story = {
  args: { variant: 'ghost', size: 'md', label: 'Cancel' },
};

// Auto-generated stories for each state
export const Disabled: Story = {
  args: { variant: 'primary', size: 'md', label: 'Disabled', disabled: true },
};

export const Loading: Story = {
  args: { variant: 'primary', size: 'md', label: 'Loading', loading: true },
};

// Auto-generated stories for each size
export const Small: Story = {
  args: { variant: 'primary', size: 'sm', label: 'Small' },
};

export const Large: Story = {
  args: { variant: 'primary', size: 'lg', label: 'Large' },
};
```

### 2.3.4 Docusaurus — Full Documentation Sites

Docusaurus is Meta's open-source documentation framework. It turns Markdown (and MDX) files into beautiful documentation websites. In an SDD workflow, Docusaurus consumes the Markdown generated from your specs.

```javascript
// docusaurus.config.js
/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'E-Commerce API Documentation',
  tagline: 'Auto-generated from specifications',
  url: 'https://docs.example.com',
  baseUrl: '/',

  // Custom plugin to regenerate docs from specs before build
  plugins: [
    [
      './plugins/spec-to-docs',
      {
        specsDir: 'src/specs',
        outputDir: 'docs/api',
        templateDir: 'doc-templates',
      },
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.auto-generated.js',
          editUrl: 'https://github.com/org/repo/edit/main/specs/',
          // Note: edit URL points to SPECS, not docs
          // because specs are the source of truth
          showLastUpdateAuthor: true,
          showLastUpdateTime: true,
        },
      },
    ],
  ],
};

module.exports = config;
```

---

## 2.4 Generating API Documentation from OpenAPI Specs

Let us build a complete pipeline that takes a TypeScript spec and produces a full API documentation site.

### Step 1: Define the Spec

```typescript
// src/specs/api/products.spec.ts

import { z } from 'zod';

/**
 * @api Product Endpoints
 * @baseUrl /api/v2/products
 * @auth Bearer token required
 */

// --- Request/Response Schemas (using Zod for runtime validation) ---

export const ProductCategorySchema = z.enum([
  'electronics',
  'clothing',
  'home',
  'sports',
  'books',
]);

export const CreateProductSchema = z.object({
  name: z.string().min(1).max(200).describe('Product name'),
  price: z.number().positive().describe('Price in USD'),
  category: ProductCategorySchema.describe('Product category'),
  description: z.string().max(5000).optional().describe('Product description'),
  images: z.array(z.string().url()).max(10).describe('Product image URLs'),
  tags: z.array(z.string()).max(20).optional().describe('Searchable tags'),
});

export const ProductResponseSchema = z.object({
  id: z.string().describe('Unique product identifier (prod_*)'),
  name: z.string().describe('Product name'),
  price: z.number().describe('Price in USD'),
  category: ProductCategorySchema.describe('Product category'),
  description: z.string().nullable().describe('Product description'),
  images: z.array(z.string()).describe('Product image URLs'),
  tags: z.array(z.string()).describe('Searchable tags'),
  createdAt: z.string().datetime().describe('ISO 8601 creation timestamp'),
  updatedAt: z.string().datetime().describe('ISO 8601 last update timestamp'),
});

export const PaginatedProductsSchema = z.object({
  data: z.array(ProductResponseSchema),
  pagination: z.object({
    page: z.number().int().positive(),
    limit: z.number().int().positive(),
    total: z.number().int().nonnegative(),
    totalPages: z.number().int().nonnegative(),
  }),
});

// --- Type Exports (derived from schemas) ---

export type ProductCategory = z.infer<typeof ProductCategorySchema>;
export type CreateProduct = z.infer<typeof CreateProductSchema>;
export type ProductResponse = z.infer<typeof ProductResponseSchema>;
export type PaginatedProducts = z.infer<typeof PaginatedProductsSchema>;
```

### Step 2: Generate OpenAPI from Zod Schemas

```typescript
// scripts/generate-openapi.ts

import { z } from 'zod';
import { zodToJsonSchema } from 'zod-to-json-schema';
import {
  CreateProductSchema,
  ProductResponseSchema,
  PaginatedProductsSchema,
  ProductCategorySchema,
} from '../src/specs/api/products.spec';
import * as fs from 'fs';
import * as yaml from 'yaml';

interface OpenAPISpec {
  openapi: string;
  info: { title: string; version: string; description: string };
  paths: Record<string, unknown>;
  components: { schemas: Record<string, unknown> };
}

function generateOpenAPI(): OpenAPISpec {
  const spec: OpenAPISpec = {
    openapi: '3.1.0',
    info: {
      title: 'E-Commerce API',
      version: '2.0.0',
      description: 'Auto-generated from TypeScript specifications.',
    },
    paths: {
      '/api/v2/products': {
        get: {
          operationId: 'listProducts',
          summary: 'List products with pagination',
          tags: ['Products'],
          parameters: [
            {
              name: 'page',
              in: 'query',
              required: true,
              schema: { type: 'integer', minimum: 1 },
            },
            {
              name: 'limit',
              in: 'query',
              required: true,
              schema: { type: 'integer', minimum: 1, maximum: 100 },
            },
            {
              name: 'category',
              in: 'query',
              required: false,
              schema: zodToJsonSchema(ProductCategorySchema),
            },
          ],
          responses: {
            '200': {
              description: 'Paginated product list',
              content: {
                'application/json': {
                  schema: { $ref: '#/components/schemas/PaginatedProducts' },
                },
              },
            },
          },
        },
        post: {
          operationId: 'createProduct',
          summary: 'Create a new product',
          tags: ['Products'],
          requestBody: {
            required: true,
            content: {
              'application/json': {
                schema: { $ref: '#/components/schemas/CreateProduct' },
              },
            },
          },
          responses: {
            '201': {
              description: 'Product created successfully',
              content: {
                'application/json': {
                  schema: { $ref: '#/components/schemas/ProductResponse' },
                },
              },
            },
          },
        },
      },
    },
    components: {
      schemas: {
        CreateProduct: zodToJsonSchema(CreateProductSchema),
        ProductResponse: zodToJsonSchema(ProductResponseSchema),
        PaginatedProducts: zodToJsonSchema(PaginatedProductsSchema),
      },
    },
  };

  return spec;
}

// Generate and write the OpenAPI spec
const openApiSpec = generateOpenAPI();
fs.writeFileSync(
  'docs/openapi.yaml',
  yaml.stringify(openApiSpec, { indent: 2 })
);
console.log('OpenAPI spec generated at docs/openapi.yaml');
```

### Step 3: Generate Human-Readable Docs from OpenAPI

```typescript
// scripts/generate-markdown-docs.ts

import * as fs from 'fs';
import * as yaml from 'yaml';
import * as path from 'path';

interface Parameter {
  name: string;
  in: string;
  required: boolean;
  schema: Record<string, unknown>;
  description?: string;
}

interface PathOperation {
  operationId: string;
  summary: string;
  tags: string[];
  parameters?: Parameter[];
  requestBody?: {
    required: boolean;
    content: Record<string, { schema: Record<string, unknown> }>;
  };
  responses: Record<string, {
    description: string;
    content?: Record<string, { schema: Record<string, unknown> }>;
  }>;
}

function generateMarkdownFromOpenAPI(specPath: string, outputDir: string): void {
  const specContent = fs.readFileSync(specPath, 'utf-8');
  const spec = yaml.parse(specContent);

  for (const [pathUrl, methods] of Object.entries(spec.paths)) {
    for (const [method, operation] of Object.entries(
      methods as Record<string, PathOperation>
    )) {
      const op = operation as PathOperation;
      const filename = `${op.operationId}.md`;

      let markdown = `---
title: "${op.summary}"
sidebar_label: "${method.toUpperCase()} ${pathUrl}"
---

# ${op.summary}

\`\`\`
${method.toUpperCase()} ${pathUrl}
\`\`\`

`;

      // Parameters
      if (op.parameters && op.parameters.length > 0) {
        markdown += `## Parameters\n\n`;
        markdown += `| Name | Location | Required | Type | Description |\n`;
        markdown += `|------|----------|----------|------|-------------|\n`;
        for (const param of op.parameters) {
          markdown += `| \`${param.name}\` | ${param.in} | ${
            param.required ? 'Yes' : 'No'
          } | ${JSON.stringify(param.schema)} | ${param.description || ''} |\n`;
        }
        markdown += '\n';
      }

      // Request Body
      if (op.requestBody) {
        markdown += `## Request Body\n\n`;
        markdown += `Required: ${op.requestBody.required ? 'Yes' : 'No'}\n\n`;
        const jsonContent = op.requestBody.content['application/json'];
        if (jsonContent) {
          markdown += `\`\`\`json\n${JSON.stringify(
            resolveRef(spec, jsonContent.schema),
            null,
            2
          )}\n\`\`\`\n\n`;
        }
      }

      // Responses
      markdown += `## Responses\n\n`;
      for (const [statusCode, response] of Object.entries(op.responses)) {
        const resp = response as {
          description: string;
          content?: Record<string, { schema: Record<string, unknown> }>;
        };
        markdown += `### ${statusCode} — ${resp.description}\n\n`;
        if (resp.content && resp.content['application/json']) {
          markdown += `\`\`\`json\n${JSON.stringify(
            resolveRef(spec, resp.content['application/json'].schema),
            null,
            2
          )}\n\`\`\`\n\n`;
        }
      }

      const outputPath = path.join(outputDir, filename);
      fs.writeFileSync(outputPath, markdown);
      console.log(`Generated: ${outputPath}`);
    }
  }
}

function resolveRef(spec: any, schema: Record<string, unknown>): unknown {
  if ('$ref' in schema) {
    const refPath = (schema['$ref'] as string).replace('#/', '').split('/');
    let resolved: any = spec;
    for (const segment of refPath) {
      resolved = resolved[segment];
    }
    return resolved;
  }
  return schema;
}

// Run generation
generateMarkdownFromOpenAPI('docs/openapi.yaml', 'docs/api-reference/');
```

---

## 2.5 How API-First Companies Use Spec-Driven Documentation

The most respected API documentation in the industry comes from companies that treat specs as the source of truth for their docs. Let us examine what makes their approach work.

### Anthropic's API Documentation

Anthropic's documentation for the Claude API is widely regarded as excellent. There is a reason for that: it is spec-driven. The documentation is not written after the API is built — the documentation and the API are both derived from the same specification.

What makes Anthropic's approach effective:

1. **The spec defines the contract first.** Before a new API feature is implemented, the spec defines exactly what the endpoint accepts, what it returns, and what errors it produces.

2. **Code examples are tested.** The code examples in the documentation are not handwritten and forgotten — they are extracted from the spec's example section and validated against the actual API in CI.

3. **SDKs and docs share a source.** The Python SDK, the TypeScript SDK, and the API reference all derive from the same OpenAPI specification. When the spec changes, all three update.

4. **Versioning is explicit.** The documentation clearly indicates which API version each feature belongs to, because the spec encodes version information.

### Stripe's Documentation Model

Stripe is often cited as the gold standard for API documentation. Their approach aligns perfectly with SDD principles:

- **Every API endpoint has a machine-readable specification** that defines the request format, response format, and error cases.
- **Documentation is generated from these specifications**, not written independently.
- **Code examples in the docs are auto-generated** for multiple languages (Python, Ruby, Node.js, Go, Java, etc.) from the same spec.
- **The API changelog is derived from spec diffs.** When a spec changes, the changelog entry is generated automatically, describing what changed.

### Twilio's Documentation Architecture

Twilio takes spec-driven documentation further with what they call "code-first documentation":

- **Helper libraries are generated from OpenAPI specs.** The Twilio SDKs for every language are auto-generated.
- **Quickstart guides reference generated code.** The "getting started" tutorials use the auto-generated SDK methods, ensuring they stay current.
- **Interactive API explorer powered by specs.** Twilio's API explorer lets you make real API calls directly from the documentation, with the request/response format defined by the spec.

> **Professor's Aside:** Notice a pattern here? The companies with the best documentation are not the ones that hire the most technical writers. They are the ones that invest in infrastructure to generate documentation from specifications. The documentation quality is a side effect of specification quality. If you want better docs, write better specs.

---

## 2.6 The Documentation Pyramid

Not all documentation serves the same purpose. There is a hierarchy of documentation types, and in an SDD workflow, all of them can be derived from specs — but they serve different audiences and purposes.

```
                    ┌───────────────┐
                    │   TUTORIALS   │  ← "Learning-oriented"
                    │  (narrative)  │     For newcomers
                    ├───────────────┤
                    │    GUIDES     │  ← "Goal-oriented"
                    │  (task-based) │     For practitioners
                    ├───────────────┤
                    │  API REFERENCE│  ← "Information-oriented"
                    │  (exhaustive) │     For implementers
                    ├───────────────┤
                    │   CHANGELOG   │  ← "Change-oriented"
                    │ (what changed)│     For upgraders
                    └───────────────┘
```

### Generating Each Level from Specs

**Level 1: API Reference (fully automatable)**

The API reference is a direct, complete rendering of your specs. Every endpoint, every type, every error code, every example. This level is 100% automatable.

```typescript
// From this spec...
interface ListProductsSpec {
  method: 'GET';
  path: '/api/v2/products';
  query: {
    page: number;    // Required, >= 1
    limit: number;   // Required, 1-100
    category?: ProductCategory;
  };
  response: PaginatedResponse<ProductDTO>;
  errors: [ValidationError, UnauthorizedError];
}

// ...generate this reference doc automatically:
// ## GET /api/v2/products
// List products with pagination and filtering.
//
// ### Query Parameters
// | Parameter | Type | Required | Description |
// |-----------|------|----------|-------------|
// | page      | integer | Yes | Page number (min: 1) |
// | limit     | integer | Yes | Items per page (1-100) |
// | category  | string  | No  | Filter by category |
//
// ### Response (200 OK)
// ```json
// { "data": [...], "pagination": { ... } }
// ```
//
// ### Errors
// | Status | Code | Description |
// |--------|------|-------------|
// | 400    | VALIDATION_ERROR | Invalid query parameters |
// | 401    | UNAUTHORIZED | Missing or invalid token |
```

**Level 2: Changelog (fully automatable)**

The changelog is generated by diffing spec versions.

```typescript
// scripts/generate-changelog.ts

interface SpecDiff {
  added: string[];
  removed: string[];
  modified: {
    path: string;
    changes: string[];
  }[];
}

function generateChangelog(oldSpec: object, newSpec: object): string {
  const diff = computeSpecDiff(oldSpec, newSpec);
  let changelog = `# Changelog\n\n## v2.1.0 (2026-02-24)\n\n`;

  if (diff.added.length > 0) {
    changelog += `### Added\n`;
    for (const item of diff.added) {
      changelog += `- ${item}\n`;
    }
    changelog += '\n';
  }

  if (diff.modified.length > 0) {
    changelog += `### Changed\n`;
    for (const mod of diff.modified) {
      changelog += `- **${mod.path}**: ${mod.changes.join(', ')}\n`;
    }
    changelog += '\n';
  }

  if (diff.removed.length > 0) {
    changelog += `### Deprecated\n`;
    for (const item of diff.removed) {
      changelog += `- ${item} (removed in next major version)\n`;
    }
    changelog += '\n';
  }

  return changelog;
}

// Usage: diff between git tags
// git show v2.0.0:specs/api.yaml > /tmp/old-spec.yaml
// git show v2.1.0:specs/api.yaml > /tmp/new-spec.yaml
// npx ts-node scripts/generate-changelog.ts
```

**Level 3: How-To Guides (semi-automatable)**

Guides require more narrative than a reference, but the structure and examples can still be derived from specs. You write the narrative scaffolding once, and the technical details are filled in from specs.

```markdown
<!-- docs/guides/creating-products.mdx -->
<!-- The narrative is hand-written. The technical details are injected. -->

# Creating Products

This guide walks you through creating products in the E-Commerce API.

## Prerequisites

Before you begin, make sure you have:
- An API key with `products:write` scope
- The API client library installed

## Step 1: Prepare Your Product Data

<!-- AUTO-INJECTED from CreateProductSchema -->
<SchemaTable schema="CreateProduct" />
<!-- Renders the schema as a table with field names, types, and descriptions -->

## Step 2: Make the API Call

<!-- AUTO-INJECTED from spec examples, for each supported language -->
<CodeExamples operationId="createProduct" />
<!-- Renders code examples in TypeScript, Python, curl -->

## Step 3: Handle the Response

<!-- AUTO-INJECTED from ProductResponseSchema -->
<SchemaTable schema="ProductResponse" />

## Error Handling

<!-- AUTO-INJECTED from spec error definitions -->
<ErrorTable operationId="createProduct" />
```

**Level 4: Tutorials (mostly manual, but spec-informed)**

Tutorials are narrative-heavy and pedagogical. They cannot be fully automated. But they can reference spec-generated components to stay current.

```markdown
<!-- docs/tutorials/build-a-product-catalog.mdx -->

# Tutorial: Build a Product Catalog

In this tutorial, you will build a simple product catalog using the
E-Commerce API. By the end, you will have a working application that
lists, creates, and filters products.

## What You Will Build

A React application that:
1. Displays products in a grid
2. Supports pagination
3. Allows filtering by category
4. Lets admins create new products

## Setting Up

First, install the API client:

```bash
npm install @example/ecommerce-sdk
```

The SDK is auto-generated from our API specification, so the TypeScript
types will always match the current API.

<!-- AUTO-INJECTED: current SDK type definitions -->
<SDKTypes module="products" />

## Fetching Products

Here is how to list products using the SDK:

<!-- AUTO-INJECTED: working code example from spec -->
<TutorialStep operationId="listProducts">
```typescript
import { ECommerceClient } from '@example/ecommerce-sdk';

const client = new ECommerceClient({ apiKey: 'your-api-key' });

// Types are auto-generated from the API spec
const response = await client.products.list({
  page: 1,
  limit: 20,
  category: 'electronics',
});

console.log(`Found ${response.pagination.total} products`);
for (const product of response.data) {
  console.log(`${product.name}: $${product.price}`);
}
```
</TutorialStep>
```

---

## 2.7 Living Documentation: Docs That Update When Specs Update

The ultimate goal is documentation that is "alive" — it updates automatically whenever the underlying specification changes. Here is how to build this pipeline end to end.

### The Complete Pipeline

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│  Spec    │────▶│ Generate │────▶│  Build   │────▶│  Deploy  │
│  Change  │     │  Docs    │     │  Site    │     │  Docs    │
│  (PR)    │     │ (CI job) │     │(Docusaurus)│   │  (CDN)   │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
     │                                                   │
     │           ┌──────────┐                            │
     └──────────▶│ Validate │                            │
                 │  Sync    │◀───────────────────────────┘
                 └──────────┘
                 "Are docs in sync with specs?"
```

### Implementation

```yaml
# .github/workflows/living-docs.yml
name: Living Documentation Pipeline

on:
  push:
    branches: [main]
    paths:
      - 'src/specs/**'  # Only trigger when specs change
  pull_request:
    branches: [main]
    paths:
      - 'src/specs/**'

jobs:
  generate-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm ci

      # Step 1: Generate OpenAPI from TypeScript specs
      - name: Generate OpenAPI spec
        run: npx ts-node scripts/generate-openapi.ts

      # Step 2: Generate Markdown docs from OpenAPI
      - name: Generate API reference docs
        run: npx ts-node scripts/generate-markdown-docs.ts

      # Step 3: Generate TypeDoc from spec interfaces
      - name: Generate TypeDoc
        run: npx typedoc

      # Step 4: Generate changelog from spec diff
      - name: Generate changelog
        run: npx ts-node scripts/generate-changelog.ts

      # Step 5: Validate all generated docs
      - name: Validate documentation
        run: |
          npm run docs:validate-links
          npm run docs:validate-examples
          npm run docs:validate-schemas

      # Step 6: Build the documentation site
      - name: Build Docusaurus site
        run: cd docs-site && npm run build

      # Step 7: Deploy (only on main branch)
      - name: Deploy to CDN
        if: github.ref == 'refs/heads/main'
        run: |
          npm run docs:deploy
          echo "Documentation deployed at $(date)"

      # Step 8: Notify team of doc updates
      - name: Notify documentation changes
        if: github.ref == 'refs/heads/main'
        run: |
          npx ts-node scripts/notify-doc-changes.ts \
            --commit=${{ github.sha }} \
            --channel=docs-updates
```

### Validation: Ensuring Docs Match Specs

```typescript
// scripts/validate-doc-sync.ts
// Run in CI to ensure docs are never out of sync with specs

import * as fs from 'fs';
import * as crypto from 'crypto';

interface SyncManifest {
  specs: Record<string, string>;  // filename -> content hash
  docs: Record<string, string>;   // filename -> content hash
  generatedAt: string;
}

function hashFile(filePath: string): string {
  const content = fs.readFileSync(filePath, 'utf-8');
  return crypto.createHash('sha256').update(content).digest('hex').slice(0, 12);
}

function validateSync(): void {
  // Read the sync manifest (generated during doc build)
  const manifest: SyncManifest = JSON.parse(
    fs.readFileSync('.doc-sync-manifest.json', 'utf-8')
  );

  const errors: string[] = [];

  // Check each spec file
  for (const [specFile, expectedHash] of Object.entries(manifest.specs)) {
    const currentHash = hashFile(specFile);
    if (currentHash !== expectedHash) {
      errors.push(
        `SPEC CHANGED: ${specFile} has been modified since docs were generated. ` +
        `Expected hash: ${expectedHash}, Current hash: ${currentHash}. ` +
        `Run 'npm run generate:docs' to update documentation.`
      );
    }
  }

  // Check each generated doc file
  for (const [docFile, expectedHash] of Object.entries(manifest.docs)) {
    if (!fs.existsSync(docFile)) {
      errors.push(`DOC MISSING: ${docFile} should exist but was not found.`);
      continue;
    }
    const currentHash = hashFile(docFile);
    if (currentHash !== expectedHash) {
      errors.push(
        `DOC MODIFIED: ${docFile} has been manually edited. ` +
        `This file is auto-generated and should not be edited directly. ` +
        `Edit the source spec instead and run 'npm run generate:docs'.`
      );
    }
  }

  if (errors.length > 0) {
    console.error('Documentation sync validation FAILED:\n');
    errors.forEach(e => console.error(`  - ${e}\n`));
    process.exit(1);
  }

  console.log('Documentation sync validation PASSED.');
  console.log(`  ${Object.keys(manifest.specs).length} specs checked.`);
  console.log(`  ${Object.keys(manifest.docs).length} doc files verified.`);
}

validateSync();
```

---

## 2.8 MDX as the Bridge Between Specs and Readable Documentation

MDX — Markdown with JSX — is the perfect format for spec-driven documentation because it lets you embed dynamic, spec-derived components directly into narrative documentation.

### Why MDX Matters for SDD

Regular Markdown is static. Once you write it, it does not change. MDX lets you import live components that pull data from your specs at build time:

```mdx
{/* docs/api/products.mdx */}

import { SchemaTable } from '../components/SchemaTable';
import { CodeExample } from '../components/CodeExample';
import { Endpoint } from '../components/Endpoint';
import { ErrorTable } from '../components/ErrorTable';
import { VersionBadge } from '../components/VersionBadge';

# Products API

<VersionBadge version="2.0.0" since="2026-01-15" />

The Products API allows you to manage your product catalog.

## List Products

<Endpoint method="GET" path="/api/v2/products" auth="required" />

Retrieve a paginated list of products with optional filtering.

### Parameters

<SchemaTable
  spec="ListProductsQuery"
  columns={['name', 'type', 'required', 'description', 'example']}
/>

{/* This component reads the spec file and renders a table.
    If the spec changes, this table updates automatically
    on the next build. No manual editing needed. */}

### Response

<CodeExample
  operationId="listProducts"
  languages={['typescript', 'python', 'curl']}
/>

{/* This component generates working code examples from the spec.
    The examples are validated against the actual API in CI. */}

### Errors

<ErrorTable operationId="listProducts" />

{/* This component lists all possible errors from the spec,
    including status codes, error codes, and descriptions. */}
```

### Building the MDX Components

```typescript
// docs-site/src/components/SchemaTable.tsx
// Renders a spec schema as a documentation table

import React from 'react';
import specs from '../../generated/spec-data.json';

interface SchemaTableProps {
  spec: string;
  columns?: string[];
}

export function SchemaTable({
  spec,
  columns = ['name', 'type', 'required', 'description'],
}: SchemaTableProps): React.ReactElement {
  const schema = specs.schemas[spec];

  if (!schema) {
    return (
      <div className="admonition admonition-danger">
        Schema "{spec}" not found. Has the spec been updated?
      </div>
    );
  }

  return (
    <table>
      <thead>
        <tr>
          {columns.map(col => (
            <th key={col}>{col.charAt(0).toUpperCase() + col.slice(1)}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {Object.entries(schema.properties || {}).map(([name, prop]: [string, any]) => (
          <tr key={name}>
            {columns.includes('name') && <td><code>{name}</code></td>}
            {columns.includes('type') && <td><code>{prop.type}</code></td>}
            {columns.includes('required') && (
              <td>{schema.required?.includes(name) ? 'Yes' : 'No'}</td>
            )}
            {columns.includes('description') && <td>{prop.description}</td>}
            {columns.includes('example') && (
              <td><code>{JSON.stringify(prop.example)}</code></td>
            )}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
```

```typescript
// docs-site/src/components/CodeExample.tsx
// Generates code examples from spec definitions

import React from 'react';
import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
import CodeBlock from '@theme/CodeBlock';
import specs from '../../generated/spec-data.json';

interface CodeExampleProps {
  operationId: string;
  languages?: string[];
}

const generators: Record<string, (op: any) => string> = {
  typescript: (op) => `import { client } from './api-client';

const response = await client.${op.operationId}(${
    op.parameters
      ? JSON.stringify(op.exampleParams, null, 2)
      : ''
  });

console.log(response.data);`,

  python: (op) => `from ecommerce_sdk import Client

client = Client(api_key="your-api-key")

response = client.${toSnakeCase(op.operationId)}(${
    op.parameters
      ? Object.entries(op.exampleParams)
          .map(([k, v]) => `${toSnakeCase(k)}=${JSON.stringify(v)}`)
          .join(', ')
      : ''
  })

print(response.data)`,

  curl: (op) => {
    const params = op.exampleParams
      ? '?' + new URLSearchParams(op.exampleParams as any).toString()
      : '';
    return `curl -X ${op.method.toUpperCase()} \\
  "https://api.example.com${op.path}${params}" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json"`;
  },
};

function toSnakeCase(str: string): string {
  return str.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
}

export function CodeExample({
  operationId,
  languages = ['typescript', 'python', 'curl'],
}: CodeExampleProps): React.ReactElement {
  const operation = specs.operations[operationId];

  if (!operation) {
    return (
      <div className="admonition admonition-danger">
        Operation "{operationId}" not found in specs.
      </div>
    );
  }

  return (
    <Tabs>
      {languages.map(lang => (
        <TabItem key={lang} value={lang} label={lang.charAt(0).toUpperCase() + lang.slice(1)}>
          <CodeBlock language={lang === 'curl' ? 'bash' : lang}>
            {generators[lang](operation)}
          </CodeBlock>
        </TabItem>
      ))}
    </Tabs>
  );
}
```

---

## 2.9 Automating Doc Generation in CI/CD Pipelines

Let us put together a complete `package.json` scripts section that covers the entire documentation lifecycle:

```json
{
  "scripts": {
    "specs:validate": "ts-node scripts/validate-specs.ts",
    "specs:lint": "spectral lint src/specs/**/*.spec.ts",

    "docs:generate:openapi": "ts-node scripts/generate-openapi.ts",
    "docs:generate:markdown": "ts-node scripts/generate-markdown-docs.ts",
    "docs:generate:typedoc": "typedoc --options typedoc.json",
    "docs:generate:storybook": "storybook build",
    "docs:generate:changelog": "ts-node scripts/generate-changelog.ts",
    "docs:generate:all": "npm-run-all docs:generate:*",

    "docs:validate:links": "ts-node scripts/validate-links.ts",
    "docs:validate:examples": "ts-node scripts/validate-code-examples.ts",
    "docs:validate:sync": "ts-node scripts/validate-doc-sync.ts",
    "docs:validate:all": "npm-run-all docs:validate:*",

    "docs:build": "npm run docs:generate:all && cd docs-site && npm run build",
    "docs:serve": "cd docs-site && npm run start",
    "docs:deploy": "cd docs-site && npm run deploy",

    "docs:full-pipeline": "npm run specs:validate && npm run docs:generate:all && npm run docs:validate:all && npm run docs:build"
  }
}
```

### The Pre-Commit Hook

```bash
#!/bin/bash
# .husky/pre-commit
# Ensure docs are regenerated when specs change

SPEC_CHANGES=$(git diff --cached --name-only -- 'src/specs/')

if [ -n "$SPEC_CHANGES" ]; then
  echo "Spec changes detected. Regenerating documentation..."

  npm run docs:generate:all

  # Check if generated docs have changed
  DOC_CHANGES=$(git diff --name-only -- 'docs/')

  if [ -n "$DOC_CHANGES" ]; then
    echo ""
    echo "WARNING: Spec changes produced documentation updates."
    echo "The following doc files were regenerated:"
    echo "$DOC_CHANGES"
    echo ""
    echo "These changes have been staged automatically."
    git add docs/
  fi
fi
```

---

## 2.10 The Role of AI in Documentation

Here is where things get genuinely exciting. AI models like Claude and GPT are extraordinarily good at one specific task: translating technical specifications into human-friendly prose. This is the bridge between your machine-readable spec and documentation that humans actually enjoy reading.

### Using AI to Generate Docs from Specs

```typescript
// scripts/ai-doc-generator.ts
// Uses Claude to generate human-friendly documentation from specs

import Anthropic from '@anthropic-ai/sdk';
import * as fs from 'fs';

const anthropic = new Anthropic();

interface DocGenerationRequest {
  specContent: string;
  docType: 'api-reference' | 'guide' | 'tutorial';
  audience: 'beginner' | 'intermediate' | 'advanced';
  tone: 'formal' | 'conversational' | 'tutorial';
}

async function generateDocFromSpec(
  request: DocGenerationRequest
): Promise<string> {
  const systemPrompt = `You are a technical writer generating documentation
from API specifications. Your documentation should be:
- Accurate to the spec (never add information not in the spec)
- Clear and well-structured
- Appropriate for a ${request.audience} audience
- Written in a ${request.tone} tone
- Include practical examples derived from the spec
- Highlight common pitfalls and edge cases`;

  const userPrompt = `Generate a ${request.docType} document from this
TypeScript specification:

\`\`\`typescript
${request.specContent}
\`\`\`

Requirements:
- Cover every field and endpoint in the spec
- Include code examples in TypeScript and Python
- Document all error cases
- Add a "Common Mistakes" section
- Include a "Quick Start" section at the top`;

  const message = await anthropic.messages.create({
    model: 'claude-sonnet-4-20250514',
    max_tokens: 4096,
    messages: [
      { role: 'user', content: userPrompt },
    ],
    system: systemPrompt,
  });

  const textContent = message.content.find(block => block.type === 'text');
  return textContent ? textContent.text : '';
}

// Usage in the documentation pipeline
async function generateAllDocs(): Promise<void> {
  const specFiles = fs.readdirSync('src/specs/api/')
    .filter(f => f.endsWith('.spec.ts'));

  for (const specFile of specFiles) {
    const specContent = fs.readFileSync(
      `src/specs/api/${specFile}`,
      'utf-8'
    );

    // Generate API reference (formal, exhaustive)
    const reference = await generateDocFromSpec({
      specContent,
      docType: 'api-reference',
      audience: 'advanced',
      tone: 'formal',
    });

    // Generate guide (practical, goal-oriented)
    const guide = await generateDocFromSpec({
      specContent,
      docType: 'guide',
      audience: 'intermediate',
      tone: 'conversational',
    });

    const baseName = specFile.replace('.spec.ts', '');
    fs.writeFileSync(`docs/api-reference/${baseName}.md`, reference);
    fs.writeFileSync(`docs/guides/${baseName}.md`, guide);

    console.log(`Generated docs for ${baseName}`);
  }
}

generateAllDocs().catch(console.error);
```

### The Human Review Layer

AI-generated documentation should never be published without human review. The AI is excellent at structure, completeness, and clarity, but it can hallucinate details, miss subtle nuances, and sometimes produce content that is technically correct but pedagogically unhelpful.

```markdown
## AI Documentation Review Checklist

### Accuracy
- [ ] Every statement is supported by the spec
- [ ] No hallucinated features or behaviors
- [ ] Code examples compile and run correctly
- [ ] Error codes match the spec exactly

### Completeness
- [ ] All endpoints/types/errors covered
- [ ] Edge cases documented
- [ ] Authentication requirements stated
- [ ] Rate limits documented

### Clarity
- [ ] Jargon is explained on first use
- [ ] Examples are practical (not contrived)
- [ ] Complex concepts have analogies or diagrams
- [ ] Reading order makes sense for the target audience

### Tone
- [ ] Consistent with existing documentation
- [ ] Appropriate for the target audience
- [ ] No condescending language
- [ ] No unexplained assumptions
```

> **Professor's Aside:** The AI is your first-draft machine. It turns specs into prose faster than any human writer. But the human review is what turns a first draft into documentation you can be proud of. The AI handles the 80% that is mechanical — the structure, the completeness, the boilerplate. The human handles the 20% that requires judgment — the tone, the pedagogical flow, the "what will confuse a newcomer" instinct.

---

## 2.11 Internationalization of Docs from Specs

When your specs are the source of truth, internationalization (i18n) becomes dramatically simpler. Instead of translating entire documentation sites, you translate a structured spec and regenerate the docs in each language.

### The i18n Pipeline

```typescript
// scripts/i18n-doc-generator.ts

interface TranslationManifest {
  sourceLocale: string;
  targetLocales: string[];
  translationStrategy: 'ai-assisted' | 'professional' | 'community';
  specsToTranslate: string[];
}

const manifest: TranslationManifest = {
  sourceLocale: 'en',
  targetLocales: ['es', 'fr', 'de', 'ja', 'zh', 'ko', 'pt-br'],
  translationStrategy: 'ai-assisted',
  specsToTranslate: [
    'src/specs/api/products.spec.ts',
    'src/specs/api/users.spec.ts',
    'src/specs/api/orders.spec.ts',
  ],
};

interface TranslatableContent {
  // These get translated
  descriptions: Record<string, string>;
  examples: Record<string, string>;
  errorMessages: Record<string, string>;
  guideContent: string;

  // These stay in English (technical identifiers)
  fieldNames: string[];
  endpointPaths: string[];
  statusCodes: number[];
  typeDefinitions: string[];
}

function extractTranslatableContent(specPath: string): TranslatableContent {
  // Parse the spec and separate translatable strings
  // from technical identifiers
  // ...
  return {} as TranslatableContent;
}

async function translateWithAI(
  content: TranslatableContent,
  targetLocale: string
): Promise<TranslatableContent> {
  // Use Claude to translate descriptions and guide content
  // Keep code examples in English but translate comments
  // Preserve technical terms with glossary consistency
  // ...
  return {} as TranslatableContent;
}

function generateLocalizedDocs(
  specPath: string,
  translatedContent: TranslatableContent,
  locale: string
): void {
  // Generate the same doc structure but with translated content
  // Technical details (types, endpoints, code) stay in English
  // Descriptions, guides, error messages are localized
  // ...
}
```

### What Gets Translated and What Does Not

```markdown
## i18n Rules for Spec-Driven Docs

### TRANSLATE (locale-specific)
- Endpoint descriptions and summaries
- Field descriptions
- Error message descriptions
- Guide narrative text
- Tutorial content
- Code comments (within examples)
- UI labels referenced in docs

### DO NOT TRANSLATE (universal)
- Field names (e.g., `createdAt` stays `createdAt`)
- Endpoint paths (e.g., `/api/v2/products` stays the same)
- Type names (e.g., `ProductDTO` stays `ProductDTO`)
- Status codes (e.g., `200 OK`, `404 Not Found`)
- Code syntax (obviously)
- JSON keys
- Header names (e.g., `Authorization`, `Content-Type`)
```

---

## 2.12 Practical Walkthrough: From TypeScript Interface Spec to Auto-Generated API Docs to User-Facing Guide

Let us trace the complete path from a single TypeScript spec through to a published documentation page.

### The Spec

```typescript
// src/specs/api/orders.spec.ts

/**
 * @api Order Management
 * @version 2.0.0
 * @description Create, track, and manage customer orders.
 * @baseUrl /api/v2/orders
 */

import { z } from 'zod';

// --- Enums ---

export const OrderStatusSchema = z.enum([
  'pending',
  'confirmed',
  'processing',
  'shipped',
  'delivered',
  'cancelled',
  'refunded',
]);

// --- DTOs ---

export const OrderItemSchema = z.object({
  productId: z.string().describe('Product identifier'),
  quantity: z.number().int().positive().describe('Quantity ordered'),
  priceAtPurchase: z.number().positive().describe('Price at time of purchase (USD)'),
});

export const CreateOrderSchema = z.object({
  items: z.array(OrderItemSchema).min(1).max(50)
    .describe('Items to order (1-50 items per order)'),
  shippingAddressId: z.string()
    .describe('ID of saved shipping address'),
  paymentMethodId: z.string()
    .describe('ID of saved payment method'),
  notes: z.string().max(500).optional()
    .describe('Optional order notes'),
});

export const OrderResponseSchema = z.object({
  id: z.string().describe('Order identifier (ord_*)'),
  status: OrderStatusSchema.describe('Current order status'),
  items: z.array(OrderItemSchema).describe('Ordered items'),
  subtotal: z.number().describe('Sum of item prices'),
  tax: z.number().describe('Calculated tax amount'),
  shipping: z.number().describe('Shipping cost'),
  total: z.number().describe('Grand total (subtotal + tax + shipping)'),
  createdAt: z.string().datetime().describe('Order creation timestamp'),
  estimatedDelivery: z.string().datetime().nullable()
    .describe('Estimated delivery date (null if not shipped)'),
});

// --- Type Exports ---
export type OrderStatus = z.infer<typeof OrderStatusSchema>;
export type OrderItem = z.infer<typeof OrderItemSchema>;
export type CreateOrder = z.infer<typeof CreateOrderSchema>;
export type OrderResponse = z.infer<typeof OrderResponseSchema>;

// --- Endpoint Specs ---

/**
 * @endpoint POST /api/v2/orders
 * @auth Required (Bearer token)
 * @rateLimit 5 orders per minute per user
 *
 * @example
 * Request:
 * {
 *   "items": [
 *     { "productId": "prod_abc123", "quantity": 2, "priceAtPurchase": 29.99 }
 *   ],
 *   "shippingAddressId": "addr_xyz789",
 *   "paymentMethodId": "pm_def456"
 * }
 *
 * Response (201):
 * {
 *   "id": "ord_ghi012",
 *   "status": "pending",
 *   "items": [...],
 *   "subtotal": 59.98,
 *   "tax": 5.40,
 *   "shipping": 7.99,
 *   "total": 73.37,
 *   "createdAt": "2026-02-24T10:30:00Z",
 *   "estimatedDelivery": null
 * }
 *
 * @errors
 * 400 VALIDATION_ERROR — Invalid order data
 * 402 PAYMENT_REQUIRED — Payment method declined
 * 404 NOT_FOUND — Product or address not found
 * 409 CONFLICT — Insufficient inventory
 * 429 RATE_LIMITED — Too many orders
 */
export interface CreateOrderEndpoint {
  method: 'POST';
  path: '/api/v2/orders';
  body: CreateOrder;
  response: OrderResponse;
}
```

### The Generated OpenAPI Fragment

```yaml
# Auto-generated — do not edit manually
# Source: src/specs/api/orders.spec.ts
# Generated: 2026-02-24T10:30:00Z

paths:
  /api/v2/orders:
    post:
      operationId: createOrder
      summary: Create a new order
      description: Create, track, and manage customer orders.
      tags:
        - Orders
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateOrder'
            example:
              items:
                - productId: "prod_abc123"
                  quantity: 2
                  priceAtPurchase: 29.99
              shippingAddressId: "addr_xyz789"
              paymentMethodId: "pm_def456"
      responses:
        '201':
          description: Order created successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OrderResponse'
        '400':
          description: Invalid order data
        '402':
          description: Payment method declined
        '404':
          description: Product or address not found
        '409':
          description: Insufficient inventory
        '429':
          description: Too many orders (rate limit exceeded)
```

### The Generated API Reference Page

```markdown
<!-- Auto-generated from orders.spec.ts — do not edit -->
<!-- Regenerate with: npm run docs:generate:all -->

---
title: "Create Order"
sidebar_label: "POST /orders"
---

# Create Order

```
POST /api/v2/orders
```

Create a new order with one or more items.

**Authentication:** Required (Bearer token)
**Rate Limit:** 5 orders per minute per user

## Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `items` | OrderItem[] | Yes | Items to order (1-50 items) |
| `items[].productId` | string | Yes | Product identifier |
| `items[].quantity` | integer | Yes | Quantity (min: 1) |
| `items[].priceAtPurchase` | number | Yes | Price in USD |
| `shippingAddressId` | string | Yes | Saved shipping address ID |
| `paymentMethodId` | string | Yes | Saved payment method ID |
| `notes` | string | No | Order notes (max 500 chars) |

## Response (201 Created)

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Order identifier (ord_*) |
| `status` | OrderStatus | Current status |
| `items` | OrderItem[] | Ordered items |
| `subtotal` | number | Sum of items |
| `tax` | number | Tax amount |
| `shipping` | number | Shipping cost |
| `total` | number | Grand total |
| `createdAt` | datetime | Creation timestamp |
| `estimatedDelivery` | datetime | Delivery estimate (null if not shipped) |

## Errors

| Status | Code | Description |
|--------|------|-------------|
| 400 | VALIDATION_ERROR | Invalid order data |
| 402 | PAYMENT_REQUIRED | Payment declined |
| 404 | NOT_FOUND | Product or address not found |
| 409 | CONFLICT | Insufficient inventory |
| 429 | RATE_LIMITED | Too many orders |
```

### The Human-Friendly Guide (AI-Assisted)

```markdown
<!-- Generated by AI from orders.spec.ts, reviewed by technical writer -->

---
title: "Placing Orders"
sidebar_label: "Placing Orders"
---

# How to Place an Order

This guide walks you through creating an order using the E-Commerce API.

## Before You Start

You will need:
- An API key with `orders:write` permission
- At least one saved shipping address (see Addresses guide)
- At least one saved payment method (see Payments guide)
- Product IDs for the items you want to order

## Creating an Order

To create an order, send a POST request to `/api/v2/orders` with your
items, shipping address, and payment method.

### TypeScript

```typescript
const order = await client.orders.create({
  items: [
    {
      productId: 'prod_abc123',
      quantity: 2,
      priceAtPurchase: 29.99,
    },
  ],
  shippingAddressId: 'addr_xyz789',
  paymentMethodId: 'pm_def456',
  notes: 'Please gift wrap',
});

console.log(`Order ${order.id} created. Total: $${order.total}`);
```

### Python

```python
order = client.orders.create(
    items=[
        {
            "product_id": "prod_abc123",
            "quantity": 2,
            "price_at_purchase": 29.99,
        }
    ],
    shipping_address_id="addr_xyz789",
    payment_method_id="pm_def456",
    notes="Please gift wrap",
)

print(f"Order {order.id} created. Total: ${order.total}")
```

## Understanding Order Status

Orders move through these statuses:

```
pending → confirmed → processing → shipped → delivered
    ↓                      ↓
 cancelled              cancelled
                           ↓
                        refunded
```

## Common Mistakes

1. **Forgetting to include `priceAtPurchase`**: This field is required
   for audit purposes. Use the current product price from the catalog.

2. **Exceeding the item limit**: Orders are limited to 50 items.
   For larger orders, split them into multiple API calls.

3. **Rate limiting**: You can create at most 5 orders per minute.
   If you need bulk order creation, contact support for a higher limit.
```

---

## 2.13 Exercise: Take 3 Specs from Previous Modules and Generate Complete Documentation

### Your Assignment

Select three specifications you have written in previous modules (or use the examples provided throughout this course). For each spec, produce the complete documentation stack.

**Part 1: API Reference Generation**

For each spec:
1. Create the Zod schema (if not already in Zod)
2. Generate an OpenAPI fragment
3. Generate a Markdown API reference page
4. Validate that all fields are documented
5. Include at least two code examples (TypeScript + Python)

**Part 2: Guide Generation**

For each spec:
1. Write a task-oriented guide using the spec as source material
2. Use MDX components that reference the spec (no hardcoded values)
3. Include a "Common Mistakes" section
4. Include a "Quick Start" section

**Part 3: Pipeline Setup**

1. Create a `package.json` with all necessary generation scripts
2. Create a CI workflow that validates doc/spec sync
3. Create a pre-commit hook that regenerates docs when specs change
4. Set up a Docusaurus site that consumes the generated docs

**Part 4: AI-Assisted Documentation**

1. Use Claude or GPT to generate a first draft of a tutorial from one of your specs
2. Review the AI output for accuracy, completeness, and tone
3. Edit the AI output to production quality
4. Document what the AI got right and what needed human correction

### Evaluation Rubric

```markdown
## Evaluation Rubric

### API Reference (25 points)
- All fields documented ..................... 5 pts
- Types accurate ........................... 5 pts
- Examples compile and run ................. 5 pts
- Error cases documented ................... 5 pts
- Generated (not hand-written) ............. 5 pts

### Guides (25 points)
- Task-oriented structure .................. 5 pts
- Uses MDX spec components ................. 5 pts
- Common mistakes section .................. 5 pts
- Quick start works end-to-end ............. 5 pts
- Appropriate for target audience .......... 5 pts

### Pipeline (25 points)
- Generation scripts work .................. 5 pts
- CI validation catches drift .............. 5 pts
- Pre-commit hook works .................... 5 pts
- Docusaurus site builds ................... 5 pts
- Full pipeline runs end-to-end ............ 5 pts

### AI-Assisted Documentation (25 points)
- AI prompt is well-structured ............. 5 pts
- AI output reviewed thoroughly ............ 5 pts
- Human edits improve quality .............. 5 pts
- Reflection on AI strengths/weaknesses .... 5 pts
- Final doc is production quality .......... 5 pts
```

---

## Chapter Summary

Documentation as Code is not just a best practice — it is a paradigm shift. When your specs are your documentation source, the eternal problem of stale docs simply vanishes. The documentation cannot be out of date because it is generated from the same artifact that drives the implementation and the tests.

The key principles from this chapter:

1. **Specs ARE the documentation.** One source of truth, multiple derived outputs.
2. **Generation, not maintenance.** Docs are generated, not manually maintained.
3. **CI enforcement.** The pipeline physically prevents spec/doc drift.
4. **The documentation pyramid.** Different audiences need different doc types, all derived from the same specs.
5. **AI as a first-draft engine.** Let Claude or GPT generate prose from specs, then apply human editorial judgment.
6. **MDX as the bridge.** MDX components pull live data from specs into narrative documentation.

In the next and final chapter, we confront the most important topic in all of SDD: the human. Where does the human belong in a world where AI can generate code, tests, and documentation from specs? The answer, it turns out, is everywhere that matters.

> *"The best documentation is the documentation that writes itself — because it is derived from the same truth that the software is derived from."*

---

**Next Chapter:** The Human-in-the-Loop — Mastering the Approval Gate

---

# Chapter 3: Environment-Aware Specs

## MODULE 04 — Advanced Orchestration & Agents (Advanced Level)

---

### Lecture Preamble

*Here is a scenario I want you to think about. You write a perfect spec. The Architect Agent produces it flawlessly. The Builder Agent implements it exactly. The Critic Agent validates it and gives it a clean PASS. You run the tests. Green across the board. You push to production. And it breaks.*

*Why? Because the spec described **what** the software should do. It described **how** it should behave. But it never described **where** it would run. The development environment had 4GB of RAM; production has 16GB but also handles 10,000 concurrent users. The local database was SQLite; production is PostgreSQL. The tests used a mocked email service; production needs real SMTP credentials. The CI pipeline had 10-minute timeouts; the deployment pipeline had 5-minute timeouts.*

*Environment-aware specs solve this. They are specifications that do not just describe the software -- they describe the software **in its habitat**. They answer questions like: What does this system need from its environment to function correctly? What changes between environments? What stays the same? What gets tested where?*

*This is where Spec-Driven Development meets DevOps, meets Infrastructure as Code, meets the reality that software does not live in a vacuum. It lives on servers, in containers, behind load balancers, with environment variables, in CI/CD pipelines, across development and staging and production environments. And if your spec does not account for all of that, your spec is incomplete.*

*Google, Amazon, Microsoft, Anthropic -- every one of these companies has learned this lesson the hard way, and every one of them has invested massively in declarative, specification-driven infrastructure. Today, we learn from their experience.*

---

## 3.1 Why Deployment Context Matters in Specs

Let us start with the fundamental question: why should a specification care about deployment?

The traditional answer from software architecture is: "It should not. Specs describe behavior, not deployment." And in a perfect world, that is true. But we do not live in a perfect world. We live in a world where:

### The Same Code Behaves Differently Across Environments

```
Environment    Database       Cache     Email Provider    Max Memory    Log Level
─────────────────────────────────────────────────────────────────────────────────
Local Dev      SQLite         None      Console logger    512MB         debug
CI/Test        PostgreSQL     Redis     Mock SMTP         2GB           info
Staging        PostgreSQL     Redis     SendGrid (test)   4GB           info
Production     PostgreSQL     Redis     SendGrid (live)   16GB          warn
```

A spec that says "send a confirmation email" means different things in each of these environments:

- **Local dev:** Print the email content to the console
- **CI/Test:** Send to a mock SMTP server and verify delivery
- **Staging:** Send to SendGrid's test mode (no actual delivery)
- **Production:** Send for real, to real users, with real consequences

If your spec does not acknowledge this, your Builder Agent produces code that works in one environment and fails in another.

### Constraints Are Environment-Specific

Consider this constraint from a spec: "The export must complete within 30 seconds."

In local development, 30 seconds might be trivial -- small dataset, local database, no network latency. In production, with 50,000 rows, a remote database, and competing workload, 30 seconds might be ambitious.

An environment-aware spec makes this explicit:

```yaml
constraints:
  - name: "export_timeout"
    description: "Maximum time for export to complete"
    values:
      local: "60s"       # Relaxed for development
      ci: "30s"          # Matches production target
      staging: "30s"     # Same as production
      production: "30s"  # The actual requirement
```

> **Professor's aside:** I have seen teams waste weeks debugging "flaky tests" that were not flaky at all. They were environment-dependent. The test passed on the developer's laptop (fast SSD, lots of RAM) and failed in CI (shared runners, slow I/O). An environment-aware spec would have caught this at specification time, not debugging time.

---

## 3.2 Environment Variables as Spec Inputs

Environment variables are the most common mechanism for configuring software across environments. In SDD, we treat them as **spec inputs** -- explicit declarations of what the system needs from its environment.

### The Problem with Undocumented Environment Variables

Most projects have environment variables scattered across:
- README files (if you are lucky)
- `.env.example` files (often outdated)
- Docker Compose files
- CI configuration
- Deployment scripts
- Tribal knowledge ("oh, you need to set THAT_VARIABLE too")

The SDD approach: **every environment variable is declared in the spec.**

### Environment Variables Spec

```yaml
# spec/environment/environment-variables.spec.yaml

spec_version: "1.0.0"
feature: "environment-configuration"
type: "environment"

purpose: |
  Declares all environment variables required by the application,
  their types, validation rules, defaults, and which environments
  require them.

variables:
  # ──────────────────────────────────────────────────────
  # Database Configuration
  # ──────────────────────────────────────────────────────
  - name: "DATABASE_URL"
    type: "string"
    format: "connection-string"
    required:
      local: true
      ci: true
      staging: true
      production: true
    sensitive: true
    description: "PostgreSQL connection string"
    examples:
      local: "postgresql://localhost:5432/myapp_dev"
      ci: "postgresql://postgres:postgres@localhost:5432/myapp_test"
      staging: "postgresql://user:pass@staging-db.internal:5432/myapp_staging"
      production: "postgresql://user:pass@prod-db.internal:5432/myapp_prod"
    validation:
      pattern: "^postgresql://.*"
      must_contain: ["postgresql://"]

  - name: "DATABASE_POOL_SIZE"
    type: "integer"
    required:
      local: false
      ci: false
      staging: true
      production: true
    sensitive: false
    description: "Maximum number of database connections in the pool"
    defaults:
      local: 5
      ci: 5
      staging: 10
      production: 50
    validation:
      min: 1
      max: 200

  - name: "DATABASE_SSL_MODE"
    type: "enum"
    values: ["disable", "require", "verify-ca", "verify-full"]
    required:
      local: false
      ci: false
      staging: true
      production: true
    defaults:
      local: "disable"
      ci: "disable"
      staging: "require"
      production: "verify-full"

  # ──────────────────────────────────────────────────────
  # Authentication
  # ──────────────────────────────────────────────────────
  - name: "JWT_SECRET"
    type: "string"
    required:
      local: true
      ci: true
      staging: true
      production: true
    sensitive: true
    description: "Secret key for signing JWT tokens"
    validation:
      min_length: 32
      must_not_be: ["secret", "password", "changeme", "jwt_secret"]
    generation:
      method: "random"
      length: 64
      charset: "alphanumeric+special"

  - name: "OAUTH_CLIENT_ID"
    type: "string"
    required:
      local: false       # Can use mock auth locally
      ci: false           # Tests use mock auth
      staging: true
      production: true
    sensitive: false
    description: "OAuth provider client ID"

  - name: "OAUTH_CLIENT_SECRET"
    type: "string"
    required:
      local: false
      ci: false
      staging: true
      production: true
    sensitive: true
    description: "OAuth provider client secret"

  # ──────────────────────────────────────────────────────
  # Email Service
  # ──────────────────────────────────────────────────────
  - name: "EMAIL_PROVIDER"
    type: "enum"
    values: ["console", "smtp", "sendgrid", "ses"]
    required:
      local: true
      ci: true
      staging: true
      production: true
    defaults:
      local: "console"      # Print to console in dev
      ci: "smtp"             # Use mock SMTP in tests
      staging: "sendgrid"    # Use SendGrid test mode
      production: "sendgrid" # Use SendGrid live mode

  - name: "SENDGRID_API_KEY"
    type: "string"
    required:
      local: false                    # Not needed with console provider
      ci: false                       # Not needed with mock SMTP
      staging: true                   # Required for SendGrid
      production: true
    sensitive: true
    depends_on:
      variable: "EMAIL_PROVIDER"
      condition: "value in ['sendgrid']"

  # ──────────────────────────────────────────────────────
  # Application Settings
  # ──────────────────────────────────────────────────────
  - name: "NODE_ENV"
    type: "enum"
    values: ["development", "test", "staging", "production"]
    required:
      local: true
      ci: true
      staging: true
      production: true
    defaults:
      local: "development"
      ci: "test"
      staging: "staging"
      production: "production"

  - name: "LOG_LEVEL"
    type: "enum"
    values: ["debug", "info", "warn", "error"]
    required:
      local: false
      ci: false
      staging: true
      production: true
    defaults:
      local: "debug"
      ci: "info"
      staging: "info"
      production: "warn"

  - name: "CORS_ALLOWED_ORIGINS"
    type: "string[]"
    separator: ","
    required:
      local: false
      ci: false
      staging: true
      production: true
    defaults:
      local: ["http://localhost:3000"]
      ci: ["http://localhost:3000"]
      staging: ["https://staging.myapp.com"]
      production: ["https://myapp.com", "https://www.myapp.com"]

  # ──────────────────────────────────────────────────────
  # Feature Flags
  # ──────────────────────────────────────────────────────
  - name: "FEATURE_FLAG_PROVIDER"
    type: "enum"
    values: ["local", "launchdarkly", "unleash"]
    required:
      local: false
      ci: false
      staging: true
      production: true
    defaults:
      local: "local"
      ci: "local"
      staging: "launchdarkly"
      production: "launchdarkly"

validation_rules:
  - rule: "All sensitive variables must not appear in logs or error messages"
  - rule: "All required variables must be present at application startup"
  - rule: "Missing optional variables must use their documented defaults"
  - rule: "Invalid variable values must cause startup failure with clear error messages"
  - rule: "Variables with depends_on conditions are only required when the condition is met"
```

### Environment Variable Validation at Startup

Given the spec above, the Builder Agent produces a validation module:

```typescript
// src/config/environment.ts

import { z } from 'zod';

/**
 * Environment variable validation.
 * Implements: spec/environment/environment-variables.spec.yaml v1.0.0
 *
 * This module validates all environment variables at application startup.
 * If any required variable is missing or invalid, the application fails
 * fast with a clear error message.
 */

const environmentSchema = z.object({
  // Database
  DATABASE_URL: z
    .string()
    .startsWith('postgresql://', 'DATABASE_URL must be a PostgreSQL connection string'),
  DATABASE_POOL_SIZE: z
    .string()
    .transform(Number)
    .pipe(z.number().int().min(1).max(200))
    .default('5'),
  DATABASE_SSL_MODE: z
    .enum(['disable', 'require', 'verify-ca', 'verify-full'])
    .default('disable'),

  // Authentication
  JWT_SECRET: z
    .string()
    .min(32, 'JWT_SECRET must be at least 32 characters')
    .refine(
      (val) => !['secret', 'password', 'changeme', 'jwt_secret'].includes(val),
      'JWT_SECRET must not be a common/default value',
    ),
  OAUTH_CLIENT_ID: z.string().optional(),
  OAUTH_CLIENT_SECRET: z.string().optional(),

  // Email
  EMAIL_PROVIDER: z
    .enum(['console', 'smtp', 'sendgrid', 'ses'])
    .default('console'),
  SENDGRID_API_KEY: z.string().optional(),

  // Application
  NODE_ENV: z
    .enum(['development', 'test', 'staging', 'production'])
    .default('development'),
  LOG_LEVEL: z
    .enum(['debug', 'info', 'warn', 'error'])
    .default('debug'),
  CORS_ALLOWED_ORIGINS: z
    .string()
    .transform((val) => val.split(',').map((s) => s.trim()))
    .default('http://localhost:3000'),

  // Feature Flags
  FEATURE_FLAG_PROVIDER: z
    .enum(['local', 'launchdarkly', 'unleash'])
    .default('local'),
});

type Environment = z.infer<typeof environmentSchema>;

/**
 * Cross-field validation rules from the spec.
 */
function validateCrossFieldDependencies(env: Environment): string[] {
  const errors: string[] = [];

  // SENDGRID_API_KEY depends on EMAIL_PROVIDER being 'sendgrid'
  if (env.EMAIL_PROVIDER === 'sendgrid' && !env.SENDGRID_API_KEY) {
    errors.push(
      'SENDGRID_API_KEY is required when EMAIL_PROVIDER is "sendgrid"',
    );
  }

  // OAUTH credentials required in staging and production
  if (['staging', 'production'].includes(env.NODE_ENV)) {
    if (!env.OAUTH_CLIENT_ID) {
      errors.push(
        `OAUTH_CLIENT_ID is required in ${env.NODE_ENV} environment`,
      );
    }
    if (!env.OAUTH_CLIENT_SECRET) {
      errors.push(
        `OAUTH_CLIENT_SECRET is required in ${env.NODE_ENV} environment`,
      );
    }
  }

  // DATABASE_SSL_MODE should be verify-full in production
  if (
    env.NODE_ENV === 'production' &&
    env.DATABASE_SSL_MODE !== 'verify-full'
  ) {
    errors.push(
      'DATABASE_SSL_MODE must be "verify-full" in production '
      + `(currently "${env.DATABASE_SSL_MODE}")`,
    );
  }

  return errors;
}

/**
 * Validate and load environment configuration.
 * Fails fast with clear error messages if validation fails.
 */
export function loadEnvironment(): Environment {
  console.log('Validating environment configuration...');

  // Phase 1: Schema validation
  const result = environmentSchema.safeParse(process.env);
  if (!result.success) {
    const errors = result.error.issues.map(
      (issue) => `  ${issue.path.join('.')}: ${issue.message}`,
    );
    console.error('Environment validation failed:');
    console.error(errors.join('\n'));
    process.exit(1);
  }

  // Phase 2: Cross-field validation
  const crossFieldErrors = validateCrossFieldDependencies(result.data);
  if (crossFieldErrors.length > 0) {
    console.error('Environment cross-field validation failed:');
    console.error(crossFieldErrors.map((e) => `  ${e}`).join('\n'));
    process.exit(1);
  }

  // Phase 3: Sensitive variable protection
  const sensitiveVars = [
    'DATABASE_URL', 'JWT_SECRET', 'OAUTH_CLIENT_SECRET', 'SENDGRID_API_KEY',
  ];
  for (const varName of sensitiveVars) {
    if (process.env[varName]) {
      // Mask sensitive variables in any future logging
      Object.defineProperty(result.data, `${varName}_MASKED`, {
        get: () => `${varName}=***`,
        enumerable: false,
      });
    }
  }

  console.log(
    `Environment validated successfully (${result.data.NODE_ENV} mode)`,
  );
  return result.data;
}

// Export the validated config as a singleton
export const env = loadEnvironment();
```

> **Professor's aside:** Notice the "fail fast" pattern. The application does not start if the environment is invalid. This is vastly better than discovering at 3 AM that the SENDGRID_API_KEY was missing and your password reset emails have been silently failing for six hours. Validate early, validate loudly, and validate everything.

---

## 3.3 CI/CD Pipelines as Specs

Here is a perspective shift that is central to this chapter: **a CI/CD pipeline is a specification**. It specifies:

- What steps must be performed to validate code
- In what order those steps must run
- What constitutes success or failure
- What happens when a step fails
- What artifacts are produced

When you write a GitHub Actions workflow or a GitLab CI pipeline, you are writing a declarative specification of your development process. SDD recognizes this and brings the same rigor to pipeline specs that it brings to feature specs.

### GitHub Actions as a Spec

```yaml
# .github/workflows/sdd-pipeline.yml
#
# This workflow IS a spec. It specifies the complete validation
# pipeline for our SDD project.
#
# Spec dependencies:
#   - spec/environment/environment-variables.spec.yaml
#   - spec/schemas/spec-schema.yaml
#   - spec/constraints/backward-compatibility.constraint.yaml

name: SDD Pipeline

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

# ──────────────────────────────────────────────────────
# ENVIRONMENT SPEC: CI-specific configuration
# Maps to: spec/environment/environment-variables.spec.yaml
# ──────────────────────────────────────────────────────
env:
  NODE_ENV: "test"
  DATABASE_URL: "postgresql://postgres:postgres@localhost:5432/myapp_test"
  DATABASE_POOL_SIZE: "5"
  DATABASE_SSL_MODE: "disable"
  JWT_SECRET: "ci-test-secret-that-is-at-least-32-characters-long"
  EMAIL_PROVIDER: "smtp"
  LOG_LEVEL: "info"
  CORS_ALLOWED_ORIGINS: "http://localhost:3000"
  FEATURE_FLAG_PROVIDER: "local"

jobs:
  # ──────────────────────────────────────────────────────
  # PHASE 1: Spec Validation
  # Validates specs before any code runs
  # ──────────────────────────────────────────────────────
  spec-validation:
    name: "Validate Specifications"
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'

      - run: npm ci

      - name: "Schema conformance check"
        run: npx ts-node scripts/validate-spec-schema.ts

      - name: "Backward compatibility check"
        run: npx ts-node scripts/check-backward-compat.ts
        if: github.event_name == 'pull_request'

      - name: "Migration spec completeness check"
        run: npx ts-node scripts/check-migrations.ts
        if: github.event_name == 'pull_request'

      - name: "Changelog entry check"
        run: npx ts-node scripts/check-changelog.ts
        if: github.event_name == 'pull_request'

  # ──────────────────────────────────────────────────────
  # PHASE 2: Code Quality
  # Linting, type checking, formatting
  # ──────────────────────────────────────────────────────
  code-quality:
    name: "Code Quality"
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
      - run: npm ci
      - run: npm run lint
      - run: npm run typecheck
      - run: npm run format:check

  # ──────────────────────────────────────────────────────
  # PHASE 3: Unit Tests
  # Fast, isolated tests -- no external dependencies
  # ──────────────────────────────────────────────────────
  unit-tests:
    name: "Unit Tests"
    needs: [spec-validation, code-quality]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
      - run: npm ci
      - run: npm run test:unit -- --coverage
      - uses: actions/upload-artifact@v4
        with:
          name: coverage-unit
          path: coverage/

  # ──────────────────────────────────────────────────────
  # PHASE 4: Integration Tests
  # Tests with real database and services
  # ──────────────────────────────────────────────────────
  integration-tests:
    name: "Integration Tests"
    needs: [spec-validation, code-quality]
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: myapp_test
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:7
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
      - run: npm ci
      - run: npm run db:migrate
      - run: npm run test:integration

  # ──────────────────────────────────────────────────────
  # PHASE 5: Spec Compliance Tests
  # Validates code against spec acceptance criteria
  # ──────────────────────────────────────────────────────
  spec-compliance:
    name: "Spec Compliance Tests"
    needs: [unit-tests, integration-tests]
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: myapp_test
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
      - run: npm ci
      - run: npm run db:migrate
      - name: "Run spec acceptance criteria tests"
        run: npm run test:spec-compliance
      - name: "Generate spec coverage report"
        run: npx ts-node scripts/spec-coverage-report.ts
      - uses: actions/upload-artifact@v4
        with:
          name: spec-coverage
          path: reports/spec-coverage.json

  # ──────────────────────────────────────────────────────
  # PHASE 6: Build & Deploy (main branch only)
  # ──────────────────────────────────────────────────────
  deploy-staging:
    name: "Deploy to Staging"
    needs: [spec-compliance]
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4
      - run: npm ci
      - run: npm run build
      - name: "Deploy to staging"
        run: npx ts-node scripts/deploy.ts --environment staging
        env:
          DEPLOY_TOKEN: ${{ secrets.STAGING_DEPLOY_TOKEN }}
```

### The Pipeline Spec as a Contract

Notice how the CI/CD pipeline embodies several spec-driven principles:

1. **Phase ordering** -- Spec validation runs before code quality, which runs before tests. You do not waste CI minutes testing code against an invalid spec.

2. **Environment configuration** -- The CI environment variables are explicitly declared and match the environment spec. No implicit configuration.

3. **Service dependencies** -- PostgreSQL and Redis are declared as services, matching the application's infrastructure requirements.

4. **Gated progression** -- Each phase depends on the previous phase passing. Spec compliance tests do not run until unit and integration tests pass.

5. **Artifacts** -- Coverage reports and spec compliance reports are uploaded as artifacts, providing traceability.

---

## 3.4 Infrastructure as Code: The Ultimate Environment Spec

Infrastructure as Code (IaC) tools like Terraform, Pulumi, and AWS CDK are, in essence, specification engines for infrastructure. They declare what infrastructure should exist, and the tool makes reality match the declaration.

From an SDD perspective, your IaC code is the environment spec for your production infrastructure.

### Terraform as Environment Spec

```hcl
# infrastructure/terraform/main.tf
#
# This Terraform configuration IS the environment spec for production.
# It declares every infrastructure resource the application needs.
#
# Related spec: spec/environment/environment-variables.spec.yaml
# Related spec: spec/environment/infrastructure.spec.yaml

terraform {
  required_version = ">= 1.7"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    bucket = "myapp-terraform-state"
    key    = "production/terraform.tfstate"
    region = "us-east-1"
  }
}

# ──────────────────────────────────────────────────────
# Variables (inputs to the environment spec)
# ──────────────────────────────────────────────────────
variable "environment" {
  type        = string
  description = "Deployment environment (staging or production)"
  validation {
    condition     = contains(["staging", "production"], var.environment)
    error_message = "Environment must be 'staging' or 'production'."
  }
}

variable "db_instance_class" {
  type = string
  default = "db.t3.medium"
  description = "RDS instance class"
}

# Environment-specific configuration
# Maps to: spec/environment/environment-variables.spec.yaml
locals {
  env_config = {
    staging = {
      db_instance_class    = "db.t3.medium"
      db_pool_size         = 10
      db_ssl_mode          = "require"
      app_instance_count   = 2
      app_instance_type    = "t3.medium"
      log_level            = "info"
      feature_flag_provider = "launchdarkly"
    }
    production = {
      db_instance_class    = "db.r6g.large"
      db_pool_size         = 50
      db_ssl_mode          = "verify-full"
      app_instance_count   = 4
      app_instance_type    = "t3.large"
      log_level            = "warn"
      feature_flag_provider = "launchdarkly"
    }
  }

  config = local.env_config[var.environment]
}

# ──────────────────────────────────────────────────────
# Database (spec: DATABASE_URL, DATABASE_POOL_SIZE, DATABASE_SSL_MODE)
# ──────────────────────────────────────────────────────
resource "aws_db_instance" "main" {
  identifier     = "myapp-${var.environment}"
  engine         = "postgres"
  engine_version = "16.2"
  instance_class = local.config.db_instance_class

  db_name  = "myapp_${var.environment}"
  username = "myapp_admin"
  password = aws_secretsmanager_secret_version.db_password.secret_string

  # From spec: DATABASE_SSL_MODE
  parameter_group_name = aws_db_parameter_group.main.name

  # Production-grade settings
  multi_az                = var.environment == "production"
  backup_retention_period = var.environment == "production" ? 30 : 7
  deletion_protection     = var.environment == "production"

  storage_encrypted = true
  storage_type      = "gp3"
  allocated_storage = var.environment == "production" ? 100 : 20

  vpc_security_group_ids = [aws_security_group.db.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name

  tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
    Spec        = "spec/environment/infrastructure.spec.yaml"
  }
}

# ──────────────────────────────────────────────────────
# Application (ECS Fargate)
# ──────────────────────────────────────────────────────
resource "aws_ecs_task_definition" "app" {
  family                   = "myapp-${var.environment}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.environment == "production" ? 1024 : 512
  memory                   = var.environment == "production" ? 2048 : 1024

  container_definitions = jsonencode([
    {
      name  = "myapp"
      image = "myapp:${var.app_version}"

      # Environment variables from spec
      environment = [
        { name = "NODE_ENV", value = var.environment },
        { name = "DATABASE_POOL_SIZE", value = tostring(local.config.db_pool_size) },
        { name = "DATABASE_SSL_MODE", value = local.config.db_ssl_mode },
        { name = "LOG_LEVEL", value = local.config.log_level },
        { name = "EMAIL_PROVIDER", value = "sendgrid" },
        { name = "FEATURE_FLAG_PROVIDER", value = local.config.feature_flag_provider },
        {
          name  = "CORS_ALLOWED_ORIGINS",
          value = var.environment == "production"
            ? "https://myapp.com,https://www.myapp.com"
            : "https://staging.myapp.com"
        },
      ]

      # Sensitive variables from AWS Secrets Manager
      secrets = [
        {
          name      = "DATABASE_URL"
          valueFrom = aws_secretsmanager_secret.db_url.arn
        },
        {
          name      = "JWT_SECRET"
          valueFrom = aws_secretsmanager_secret.jwt_secret.arn
        },
        {
          name      = "SENDGRID_API_KEY"
          valueFrom = aws_secretsmanager_secret.sendgrid_key.arn
        },
        {
          name      = "OAUTH_CLIENT_SECRET"
          valueFrom = aws_secretsmanager_secret.oauth_secret.arn
        },
      ]

      portMappings = [
        { containerPort = 3000, protocol = "tcp" }
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = "/ecs/myapp-${var.environment}"
          "awslogs-region"        = "us-east-1"
          "awslogs-stream-prefix" = "app"
        }
      }

      healthCheck = {
        command     = ["CMD-SHELL", "curl -f http://localhost:3000/health || exit 1"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
    }
  ])
}
```

> **Professor's aside:** Look at how the Terraform code references the spec. The environment variables are not arbitrary -- they are explicitly tied back to the environment spec. The database configuration matches the spec's constraints. The health check interval, timeout, and retries are not magic numbers -- they come from operational requirements that should be in a spec. Infrastructure as Code is not just automation; it is specification-driven infrastructure.

---

## 3.5 Docker/Container Specs: Dockerfiles as Deployment Specifications

A Dockerfile is a specification for a runtime environment. In SDD, we treat it as part of the environment spec and ensure it aligns with the application's requirements.

### Spec-Driven Dockerfile

```dockerfile
# Dockerfile
#
# Environment spec: spec/environment/runtime.spec.yaml
# This Dockerfile specifies the application's runtime environment.
#
# Requirements from spec:
# - Node.js 22 LTS
# - Non-root user for security
# - Health check endpoint
# - Minimal attack surface (no dev dependencies in production)
# - Reproducible builds (pinned base image)

# ── Build Stage ──────────────────────────────────────
FROM node:22.14-bookworm-slim AS builder

# Spec: build environment requires these system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    python3 \
    make \
    g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Spec: dependency installation (deterministic via lockfile)
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts && \
    npm run postinstall

# Spec: build step
COPY . .
RUN npm run build

# Spec: prune dev dependencies for production image
RUN npm prune --production

# ── Runtime Stage ────────────────────────────────────
FROM node:22.14-bookworm-slim AS runtime

# Spec: security -- run as non-root user
RUN groupadd --gid 1001 appgroup && \
    useradd --uid 1001 --gid appgroup --shell /bin/false appuser

WORKDIR /app

# Spec: minimal runtime -- only production artifacts
COPY --from=builder --chown=appuser:appgroup /app/dist ./dist
COPY --from=builder --chown=appuser:appgroup /app/node_modules ./node_modules
COPY --from=builder --chown=appuser:appgroup /app/package.json ./

# Spec: environment configuration
# These are defaults; actual values come from orchestrator (ECS, K8s, etc.)
ENV NODE_ENV=production
ENV PORT=3000

# Spec: security -- drop to non-root
USER appuser

# Spec: expose the application port
EXPOSE 3000

# Spec: health check (must match spec/environment/health-check.spec.yaml)
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD node -e "fetch('http://localhost:3000/health').then(r => { if (!r.ok) process.exit(1) })"

# Spec: application entry point
CMD ["node", "dist/main.js"]
```

### The Runtime Environment Spec

The Dockerfile above implements this spec:

```yaml
# spec/environment/runtime.spec.yaml

spec_version: "1.0.0"
feature: "runtime-environment"
type: "environment"

purpose: |
  Specifies the runtime environment requirements for the application.
  This spec is implemented by the Dockerfile and validated by CI.

runtime:
  language: "node"
  version: "22.14"
  base_image: "node:22.14-bookworm-slim"
  package_manager: "npm"
  lockfile: "package-lock.json"

security:
  run_as_root: false
  user: "appuser"
  uid: 1001
  gid: 1001
  read_only_filesystem: false  # Would require volume mounts for temp files
  no_new_privileges: true
  capabilities_drop: ["ALL"]

health_check:
  endpoint: "/health"
  port: 3000
  interval: "30s"
  timeout: "5s"
  start_period: "60s"
  retries: 3
  expected_response:
    status: 200
    body_contains: '{"status":"healthy"}'

build:
  strategy: "multi-stage"
  stages:
    - name: "builder"
      purpose: "Install dependencies and compile TypeScript"
      includes: ["source code", "dev dependencies", "build tools"]
    - name: "runtime"
      purpose: "Minimal production image"
      includes: ["compiled code", "production dependencies"]
      excludes: ["source code", "dev dependencies", "build tools", "tests"]

  constraints:
    - "Build must be deterministic (npm ci, not npm install)"
    - "Dev dependencies must not be in the runtime image"
    - "Source code must not be in the runtime image"
    - "Build must complete in under 5 minutes on CI hardware"

ports:
  - number: 3000
    protocol: "tcp"
    purpose: "HTTP application server"

resource_limits:
  cpu:
    request: "250m"
    limit: "1000m"
  memory:
    request: "256Mi"
    limit: "512Mi"
  description: |
    These are Kubernetes resource requests/limits.
    The application should function within these bounds under normal load.
    Exceeding the memory limit will result in OOM kill.
```

---

## 3.6 How Cloud Providers Use Declarative Specs for Infrastructure

The major cloud providers have each developed their own declarative specification formats for infrastructure. Understanding these helps you write better environment-aware specs.

### Google Cloud: Declarative Resource Management

Google Cloud's approach centers on declarative configurations:

- **Google Kubernetes Engine (GKE)** uses YAML manifests to specify desired state
- **Cloud Run** uses service YAML specs for serverless deployments
- **Google Cloud Deploy** uses pipeline configurations for delivery

```yaml
# Google Cloud Run service specification
# This is an environment-aware deployment spec
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: myapp
  annotations:
    run.googleapis.com/description: |
      Implements: spec/environment/infrastructure.spec.yaml
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: '2'
        autoscaling.knative.dev/maxScale: '10'
        run.googleapis.com/cpu-throttling: 'false'
    spec:
      containerConcurrency: 80
      timeoutSeconds: 300
      containers:
        - image: gcr.io/myproject/myapp:latest
          ports:
            - containerPort: 3000
          resources:
            limits:
              cpu: '1'
              memory: 512Mi
          env:
            - name: NODE_ENV
              value: production
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: connection-string
          startupProbe:
            httpGet:
              path: /health
              port: 3000
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /health
              port: 3000
            periodSeconds: 30
```

### AWS: CloudFormation and CDK

AWS uses CloudFormation templates (YAML/JSON) and the Cloud Development Kit (CDK) for declarative infrastructure:

```typescript
// AWS CDK -- Infrastructure as spec (TypeScript)
// Implements: spec/environment/infrastructure.spec.yaml

import * as cdk from 'aws-cdk-lib';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as rds from 'aws-cdk-lib/aws-rds';

/**
 * Application Infrastructure Stack
 *
 * This CDK stack is the executable form of the infrastructure spec.
 * Every resource maps to a requirement in the environment spec.
 */
export class AppStack extends cdk.Stack {
  constructor(scope: cdk.App, id: string, props: {
    environment: 'staging' | 'production';
  }) {
    super(scope, id);

    const isProduction = props.environment === 'production';

    // Database (spec: DATABASE_URL, DATABASE_POOL_SIZE)
    const database = new rds.DatabaseInstance(this, 'Database', {
      engine: rds.DatabaseInstanceEngine.postgres({
        version: rds.PostgresEngineVersion.VER_16_2,
      }),
      instanceType: isProduction
        ? ec2.InstanceType.of(ec2.InstanceClass.R6G, ec2.InstanceSize.LARGE)
        : ec2.InstanceType.of(ec2.InstanceClass.T3, ec2.InstanceSize.MEDIUM),
      multiAz: isProduction,
      deletionProtection: isProduction,
      backupRetention: cdk.Duration.days(isProduction ? 30 : 7),
      storageEncrypted: true,
    });

    // Application (spec: runtime, health_check, resource_limits)
    const taskDefinition = new ecs.FargateTaskDefinition(this, 'TaskDef', {
      cpu: isProduction ? 1024 : 512,
      memoryLimitMiB: isProduction ? 2048 : 1024,
    });

    const container = taskDefinition.addContainer('App', {
      image: ecs.ContainerImage.fromRegistry('myapp:latest'),
      logging: ecs.LogDrivers.awsLogs({ streamPrefix: 'app' }),
      healthCheck: {
        command: ['CMD-SHELL', 'curl -f http://localhost:3000/health || exit 1'],
        interval: cdk.Duration.seconds(30),   // From spec
        timeout: cdk.Duration.seconds(5),     // From spec
        retries: 3,                           // From spec
        startPeriod: cdk.Duration.seconds(60), // From spec
      },
      environment: {
        NODE_ENV: props.environment,
        DATABASE_POOL_SIZE: String(isProduction ? 50 : 10),
        DATABASE_SSL_MODE: isProduction ? 'verify-full' : 'require',
        LOG_LEVEL: isProduction ? 'warn' : 'info',
        EMAIL_PROVIDER: 'sendgrid',
      },
    });

    container.addPortMappings({ containerPort: 3000 });
  }
}
```

### Azure: ARM Templates and Bicep

Microsoft Azure uses ARM (Azure Resource Manager) templates and the Bicep language for declarative infrastructure. The principle is the same: declare the desired state, and the platform makes it happen.

All three cloud providers have converged on the same fundamental idea: **infrastructure should be specified declaratively, versioned in source control, and applied automatically.** This is exactly what SDD does for application code, and environment-aware specs bridge the two.

---

## 3.7 Feature Flags as Environment-Aware Spec Toggles

Feature flags are one of the most powerful tools for environment-aware development. They allow you to deploy the same code to every environment while controlling which features are active.

In SDD, feature flags are spec elements:

```yaml
# spec/features/notification-preferences.spec.yaml (excerpt)

feature_flags:
  - name: "ENABLE_SLACK_NOTIFICATIONS"
    description: "Controls availability of Slack as a notification channel"
    type: "boolean"
    default: false
    environments:
      local: true       # Enabled for development
      ci: true           # Enabled for testing
      staging: true      # Enabled for QA
      production: false  # Disabled until launch date
    rollout:
      strategy: "percentage"
      production_rollout_plan:
        - date: "2026-04-01"
          percentage: 5
          criteria: "Internal users only"
        - date: "2026-04-08"
          percentage: 25
          criteria: "Users who opted into beta"
        - date: "2026-04-15"
          percentage: 100
          criteria: "All users"

  - name: "ENABLE_NOTIFICATION_DIGEST"
    description: "Controls availability of digest notification mode"
    type: "boolean"
    default: false
    environments:
      local: true
      ci: true
      staging: true
      production: true   # Already launched
    launched_in: "1.1.0"

  - name: "MAX_SOCIAL_LINKS"
    description: "Maximum number of social links per user profile"
    type: "integer"
    default: 10
    environments:
      local: 10
      ci: 10
      staging: 10
      production: 10
    notes: "Can be adjusted without code deployment"
```

### Feature Flag Implementation Tied to Spec

```typescript
// src/config/feature-flags.ts

/**
 * Feature flag configuration.
 * Implements: feature_flags sections across all feature specs.
 *
 * In production, these are managed by LaunchDarkly.
 * In development/CI, these use local defaults from the spec.
 */

import { env } from './environment';

interface FeatureFlag {
  name: string;
  enabled: boolean;
  value?: unknown;
}

interface FeatureFlagProvider {
  isEnabled(flagName: string, defaultValue: boolean): boolean;
  getValue<T>(flagName: string, defaultValue: T): T;
}

class LocalFeatureFlagProvider implements FeatureFlagProvider {
  private flags: Record<string, unknown>;

  constructor(environment: string) {
    // Defaults from spec, per environment
    this.flags = LOCAL_FLAG_DEFAULTS[environment] ?? {};
  }

  isEnabled(flagName: string, defaultValue: boolean): boolean {
    const value = this.flags[flagName];
    return typeof value === 'boolean' ? value : defaultValue;
  }

  getValue<T>(flagName: string, defaultValue: T): T {
    const value = this.flags[flagName];
    return value !== undefined ? (value as T) : defaultValue;
  }
}

// From spec: feature flag defaults per environment
const LOCAL_FLAG_DEFAULTS: Record<string, Record<string, unknown>> = {
  development: {
    ENABLE_SLACK_NOTIFICATIONS: true,
    ENABLE_NOTIFICATION_DIGEST: true,
    MAX_SOCIAL_LINKS: 10,
  },
  test: {
    ENABLE_SLACK_NOTIFICATIONS: true,
    ENABLE_NOTIFICATION_DIGEST: true,
    MAX_SOCIAL_LINKS: 10,
  },
  staging: {
    ENABLE_SLACK_NOTIFICATIONS: true,
    ENABLE_NOTIFICATION_DIGEST: true,
    MAX_SOCIAL_LINKS: 10,
  },
  production: {
    ENABLE_SLACK_NOTIFICATIONS: false,  // Controlled by LaunchDarkly
    ENABLE_NOTIFICATION_DIGEST: true,
    MAX_SOCIAL_LINKS: 10,
  },
};

/**
 * Create the appropriate feature flag provider based on environment.
 */
export function createFeatureFlagProvider(): FeatureFlagProvider {
  if (env.FEATURE_FLAG_PROVIDER === 'local') {
    return new LocalFeatureFlagProvider(env.NODE_ENV);
  }

  // In production/staging, use LaunchDarkly
  // (implementation would import LaunchDarkly SDK)
  return new LocalFeatureFlagProvider(env.NODE_ENV); // Simplified
}

export const featureFlags = createFeatureFlagProvider();
```

### Using Feature Flags in Spec-Driven Code

```typescript
// src/features/notifications/notification-channels.ts

import { featureFlags } from '../../config/feature-flags';

/**
 * Get available notification channels.
 *
 * The available channels depend on feature flags, which in turn
 * depend on the environment (spec: feature_flags section).
 */
export function getAvailableChannels(): string[] {
  const channels = ['email', 'push'];  // Always available (spec v1.0.0)

  // spec: feature_flags.ENABLE_SLACK_NOTIFICATIONS
  if (featureFlags.isEnabled('ENABLE_SLACK_NOTIFICATIONS', false)) {
    channels.push('slack');
  }

  return channels;
}
```

> **Professor's aside:** Feature flags are the bridge between "the spec says this feature exists" and "the deployment says this feature is active." A feature can be fully implemented and pass all tests, but remain invisible to users in production until the flag is flipped. This is why your spec must declare feature flags -- they are part of the feature's contract with the environment.

---

## 3.8 Multi-Environment Testing Specs

Different environments need different testing strategies. Your spec should define what gets tested where.

```yaml
# spec/testing/testing-strategy.spec.yaml

spec_version: "1.0.0"
feature: "testing-strategy"
type: "testing"

purpose: |
  Defines what tests run in which environments, with what data,
  and under what conditions.

environments:
  local:
    test_types: ["unit", "integration"]
    database: "sqlite"          # Fast, no setup required
    external_services: "mocked"
    test_data: "fixtures"
    coverage_requirement: "none" # Development speed over coverage
    timeout: "60s per test"

  ci:
    test_types: ["unit", "integration", "spec-compliance"]
    database: "postgresql"       # Match production
    external_services: "mocked"  # No real API calls in CI
    test_data: "fixtures + generated"
    coverage_requirement:
      line: 80
      branch: 70
      function: 85
    timeout: "30s per test"
    parallelism: 4               # Run test suites in parallel

  staging:
    test_types: ["smoke", "e2e", "performance"]
    database: "postgresql"       # Real database with test data
    external_services: "real (test mode)"  # Real APIs in test mode
    test_data: "synthetic production-like"
    coverage_requirement: "none" # Coverage checked in CI
    timeout: "120s per test"

  production:
    test_types: ["smoke", "synthetic-monitoring"]
    database: "production"       # Real production database
    external_services: "real"    # Real everything
    test_data: "none (read-only tests only)"
    coverage_requirement: "none"
    timeout: "10s per test"
    constraints:
      - "Tests MUST be read-only -- no writes to production data"
      - "Tests MUST NOT affect real users"
      - "Tests MUST complete within 10 seconds"
      - "Tests run on a 5-minute interval"

test_types:
  unit:
    description: "Isolated tests with mocked dependencies"
    runs_in: ["local", "ci"]
    characteristics:
      - "No external dependencies"
      - "No database access"
      - "No network calls"
      - "Fast: <100ms per test"
    naming_convention: "*.spec.ts or *.test.ts"

  integration:
    description: "Tests with real database and internal services"
    runs_in: ["local", "ci"]
    characteristics:
      - "Uses real database (test instance)"
      - "Uses real internal services"
      - "External APIs are mocked"
      - "Medium speed: <5s per test"
    naming_convention: "*.integration.test.ts"

  spec-compliance:
    description: "Tests that validate acceptance criteria from specs"
    runs_in: ["ci"]
    characteristics:
      - "One test per acceptance criterion"
      - "Tests are generated from spec YAML"
      - "Failure means spec violation"
    naming_convention: "*.compliance.test.ts"

  smoke:
    description: "Quick health checks after deployment"
    runs_in: ["staging", "production"]
    characteristics:
      - "Tests critical user paths only"
      - "Fast: <30s total"
      - "Read-only in production"
    tests:
      - "Health endpoint returns 200"
      - "Authentication flow works"
      - "Main page loads"
      - "API returns valid responses"

  e2e:
    description: "Full end-to-end tests with browser automation"
    runs_in: ["staging"]
    characteristics:
      - "Uses real browser (Playwright)"
      - "Tests complete user flows"
      - "Uses test accounts"
      - "Slow: may take minutes per test"

  performance:
    description: "Load and latency tests"
    runs_in: ["staging"]
    characteristics:
      - "Tests response times under load"
      - "Tests concurrent user capacity"
      - "Validates spec latency constraints"
    thresholds:
      p50_latency: "100ms"
      p95_latency: "500ms"
      p99_latency: "2000ms"
      max_error_rate: "0.1%"
      concurrent_users: 1000

  synthetic-monitoring:
    description: "Continuous production health monitoring"
    runs_in: ["production"]
    characteristics:
      - "Runs every 5 minutes"
      - "Alerts on failure"
      - "Read-only operations only"
      - "Uses synthetic test accounts"
    alert_on_failure:
      channel: "pagerduty"
      severity: "high"
```

### Spec-Compliance Test Generation

One of the most powerful SDD patterns is generating tests directly from spec acceptance criteria:

```typescript
// scripts/generate-spec-compliance-tests.ts

/**
 * Reads spec YAML files and generates test stubs for each
 * acceptance criterion. These tests validate that the code
 * faithfully implements the spec.
 */

import * as yaml from 'js-yaml';
import * as fs from 'fs';
import * as path from 'path';
import * as glob from 'glob';

interface Spec {
  feature: string;
  spec_version: string;
  acceptance_criteria: string[];
}

function generateTestFile(spec: Spec): string {
  const testCases = spec.acceptance_criteria
    .map((criterion, index) => {
      // Parse "Given X, when Y, then Z" format
      const parts = parseCriterion(criterion);

      return `
  // Acceptance Criterion ${index + 1}
  // ${criterion}
  it('${escapeTestName(criterion)}', async () => {
    // GIVEN: ${parts.given}
    // TODO: Set up test preconditions

    // WHEN: ${parts.when}
    // TODO: Execute the action

    // THEN: ${parts.then}
    // TODO: Assert the expected outcome

    throw new Error('Test not implemented -- implement this acceptance criterion');
  });`;
    })
    .join('\n');

  return `/**
 * Spec Compliance Tests
 * Feature: ${spec.feature}
 * Spec Version: ${spec.spec_version}
 *
 * AUTO-GENERATED from spec acceptance criteria.
 * Implement each test to validate spec compliance.
 */

describe('Spec Compliance: ${spec.feature} v${spec.spec_version}', () => {
${testCases}
});
`;
}

function parseCriterion(criterion: string): {
  given: string;
  when: string;
  then: string;
} {
  const givenMatch = criterion.match(/[Gg]iven\s+(.+?)(?:,\s*[Ww]hen)/);
  const whenMatch = criterion.match(/[Ww]hen\s+(.+?)(?:,\s*[Tt]hen)/);
  const thenMatch = criterion.match(/[Tt]hen\s+(.+)/);

  return {
    given: givenMatch?.[1] ?? 'precondition not parsed',
    when: whenMatch?.[1] ?? 'action not parsed',
    then: thenMatch?.[1] ?? 'expectation not parsed',
  };
}

function escapeTestName(name: string): string {
  return name.replace(/'/g, "\\'").replace(/"/g, '\\"');
}

// Main execution
const specFiles = glob.sync('specs/features/**/*.spec.yaml');

for (const specFile of specFiles) {
  const content = fs.readFileSync(specFile, 'utf-8');
  const spec = yaml.load(content) as Spec;

  if (!spec.acceptance_criteria?.length) continue;

  const testPath = specFile
    .replace('specs/features/', 'tests/spec-compliance/')
    .replace('.spec.yaml', '.compliance.test.ts');

  const testDir = path.dirname(testPath);
  fs.mkdirSync(testDir, { recursive: true });

  const testContent = generateTestFile(spec);
  fs.writeFileSync(testPath, testContent);

  console.log(`Generated: ${testPath} (${spec.acceptance_criteria.length} tests)`);
}
```

---

## 3.9 Security Specs: Environment-Specific Security Requirements

Security requirements often vary by environment. Development might allow self-signed certificates; production requires verified TLS. Staging might allow broad CORS origins; production must be restrictive.

```yaml
# spec/environment/security.spec.yaml

spec_version: "1.0.0"
feature: "security-configuration"
type: "environment"

purpose: |
  Specifies security requirements for each deployment environment.
  These constraints are enforced at deployment time and validated
  by the Critic Agent.

tls:
  local:
    required: false
    certificate: "self-signed (optional)"
    min_version: "none"
  ci:
    required: false
    certificate: "none"
    min_version: "none"
  staging:
    required: true
    certificate: "Let's Encrypt"
    min_version: "TLS 1.2"
  production:
    required: true
    certificate: "Organization-validated (OV) or Extended Validation (EV)"
    min_version: "TLS 1.2"
    preferred_version: "TLS 1.3"
    hsts:
      enabled: true
      max_age: 31536000       # 1 year
      include_subdomains: true
      preload: true

cors:
  local:
    allowed_origins: ["*"]                           # Permissive for dev
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    credentials: true
  ci:
    allowed_origins: ["http://localhost:3000"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    credentials: true
  staging:
    allowed_origins: ["https://staging.myapp.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Content-Type", "Authorization", "X-Request-ID"]
    credentials: true
    max_age: 86400
  production:
    allowed_origins: ["https://myapp.com", "https://www.myapp.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Content-Type", "Authorization", "X-Request-ID"]
    credentials: true
    max_age: 86400

headers:
  production:
    "Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'"
    "X-Content-Type-Options": "nosniff"
    "X-Frame-Options": "DENY"
    "X-XSS-Protection": "0"    # Disabled in favor of CSP
    "Referrer-Policy": "strict-origin-when-cross-origin"
    "Permissions-Policy": "camera=(), microphone=(), geolocation=()"

authentication:
  session_timeout:
    local: "24h"       # Long sessions for dev convenience
    ci: "1h"           # Short for tests
    staging: "8h"
    production: "8h"

  rate_limiting:
    local:
      enabled: false
    ci:
      enabled: false
    staging:
      enabled: true
      login_attempts: 10       # More lenient for testing
      window: "15m"
    production:
      enabled: true
      login_attempts: 5
      window: "15m"
      lockout_duration: "30m"

secrets_management:
  local:
    provider: ".env file"
    rotation: "none"
  ci:
    provider: "GitHub Secrets"
    rotation: "none"
  staging:
    provider: "AWS Secrets Manager"
    rotation: "90 days"
  production:
    provider: "AWS Secrets Manager"
    rotation: "30 days"
    encryption: "AES-256"
    access_logging: true
    constraints:
      - "Secrets must never appear in logs"
      - "Secrets must never be committed to source control"
      - "Secret access must be audited"
      - "Secret rotation must not cause downtime"

data_protection:
  pii_handling:
    local:
      encryption_at_rest: false
      encryption_in_transit: false
      data_masking: false
    staging:
      encryption_at_rest: true
      encryption_in_transit: true
      data_masking: true          # Mask PII in logs
    production:
      encryption_at_rest: true
      encryption_in_transit: true
      data_masking: true
      data_retention: "7 years"   # Regulatory requirement
      right_to_erasure: true      # GDPR compliance
```

### Security Spec Enforcement

```typescript
// src/middleware/security-headers.middleware.ts

/**
 * Security headers middleware.
 * Implements: spec/environment/security.spec.yaml -- headers section.
 *
 * Headers are environment-aware: production gets strict CSP,
 * development gets none.
 */

import { Request, Response, NextFunction } from 'express';
import { env } from '../config/environment';

interface SecurityHeaders {
  [key: string]: string;
}

// From spec: environment-specific security headers
const SECURITY_HEADERS: Record<string, SecurityHeaders> = {
  production: {
    'Content-Security-Policy':
      "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'",
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'X-XSS-Protection': '0',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=()',
    'Strict-Transport-Security':
      'max-age=31536000; includeSubDomains; preload',
  },
  staging: {
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
    'X-XSS-Protection': '0',
    'Referrer-Policy': 'strict-origin-when-cross-origin',
  },
  // Development and test: minimal headers
  development: {},
  test: {},
};

export function securityHeadersMiddleware(
  req: Request,
  res: Response,
  next: NextFunction,
): void {
  const headers = SECURITY_HEADERS[env.NODE_ENV] ?? {};

  for (const [key, value] of Object.entries(headers)) {
    res.setHeader(key, value);
  }

  next();
}
```

---

## 3.10 How Anthropic Deploys Claude Across Environments with Spec-Driven Configuration

Anthropic's deployment of Claude provides a useful illustration of environment-aware specification in practice. While the internal details are proprietary, the general patterns are visible in their public documentation and API design.

### Model Deployment Environments

Anthropic operates Claude across multiple environments, each with different characteristics:

```yaml
# Conceptual representation of Anthropic's deployment environments

environments:
  research:
    purpose: "Model training and evaluation"
    characteristics:
      - "Full model capabilities enabled"
      - "Extensive logging for analysis"
      - "Performance metrics collection"
      - "Safety evaluation suites"
    access: "Internal researchers only"

  internal_testing:
    purpose: "Pre-release testing and red-teaming"
    characteristics:
      - "Safety filters in test mode"
      - "Behavior logging for review"
      - "A/B testing of model versions"
    access: "Internal teams and trusted testers"

  api_production:
    purpose: "Public API (api.anthropic.com)"
    characteristics:
      - "Full safety filters active"
      - "Rate limiting enforced"
      - "Usage tracking and billing"
      - "Model version pinning available"
      - "SLA guarantees"
    access: "API customers with keys"

  consumer_production:
    purpose: "claude.ai consumer product"
    characteristics:
      - "Additional content filtering"
      - "User-facing safety features"
      - "Usage quotas per subscription tier"
      - "Different model selection logic"
    access: "Consumer users"
```

### What SDD Practitioners Can Learn

The key lesson is that the same model (Claude) behaves differently across environments because of **environment-specific configuration**, not code changes. Safety filters, rate limits, logging levels, feature availability -- all of these are environment-aware settings.

Your SDD pipeline should follow the same pattern: the same code, configured differently per environment through your environment spec.

---

## 3.11 Practical Walkthrough: Writing a Complete Environment-Aware Spec

Let us build a complete, production-ready environment-aware spec for a feature from scratch. We will spec a "webhook delivery system" that sends HTTP webhooks to external URLs when events occur.

This is a great example because webhooks are inherently environment-sensitive: you cannot send real webhooks in CI, you need different retry policies in staging vs. production, and you need different logging levels for debugging vs. operating.

```yaml
# spec/features/webhook-delivery.spec.yaml

spec_version: "1.0.0"
feature: "webhook-delivery"
status: "ga"
created: "2026-02-24"

purpose: |
  Deliver HTTP webhook notifications to external URLs when
  application events occur. Supports retry logic, signature
  verification, and delivery tracking.

# ──────────────────────────────────────────────────────
# Feature Specification (environment-independent)
# ──────────────────────────────────────────────────────
inputs:
  - name: "event"
    type: "WebhookEvent"
    required: true
    properties:
      type:
        type: "string"
        description: "Event type (e.g., 'user.created', 'order.completed')"
      payload:
        type: "object"
        description: "Event data"
      timestamp:
        type: "ISO8601DateTime"
      idempotency_key:
        type: "string"
        description: "Unique key to prevent duplicate deliveries"

  - name: "subscription"
    type: "WebhookSubscription"
    required: true
    properties:
      url:
        type: "string"
        format: "url"
        description: "The endpoint to deliver the webhook to"
      secret:
        type: "string"
        description: "Shared secret for HMAC signature"
      events:
        type: "string[]"
        description: "Event types this subscription listens for"
      active:
        type: "boolean"

outputs:
  - name: "delivery_result"
    type: "DeliveryResult"
    properties:
      delivery_id:
        type: "string"
      status:
        type: "enum"
        values: ["delivered", "failed", "retrying"]
      http_status:
        type: "integer"
        nullable: true
      attempts:
        type: "integer"
      next_retry_at:
        type: "ISO8601DateTime"
        nullable: true

constraints:
  - "Webhook payloads must be signed using HMAC-SHA256"
  - "Signature must be included in the X-Webhook-Signature header"
  - "Deliveries must include an X-Webhook-ID header with the delivery_id"
  - "Deliveries must include an X-Webhook-Timestamp header"
  - "Successful delivery: HTTP 2xx response within timeout"
  - "Webhook payload must be JSON with Content-Type: application/json"

error_cases:
  - condition: "Target URL returns non-2xx status"
    behavior: "Mark as failed, schedule retry according to retry policy"
  - condition: "Target URL is unreachable (DNS failure, connection refused)"
    behavior: "Mark as failed, schedule retry"
  - condition: "Request times out"
    behavior: "Mark as failed, schedule retry"
  - condition: "Maximum retries exhausted"
    behavior: "Mark as permanently failed, send alert"
  - condition: "Duplicate idempotency_key"
    behavior: "Return existing delivery result, do not re-deliver"

acceptance_criteria:
  - "Given a valid event and active subscription, the webhook is delivered to the subscription URL"
  - "Given a successful delivery (2xx response), the delivery status is 'delivered'"
  - "Given a failed delivery, the system retries according to the retry policy"
  - "Given max retries exhausted, the delivery status is 'failed' and an alert is sent"
  - "Given a duplicate idempotency_key, the webhook is not re-delivered"
  - "Given the webhook payload, the X-Webhook-Signature header contains a valid HMAC-SHA256"

# ──────────────────────────────────────────────────────
# Environment-Aware Configuration
# ──────────────────────────────────────────────────────
environment_config:
  delivery:
    timeout:
      local: "10s"
      ci: "5s"
      staging: "15s"
      production: "30s"

    retry_policy:
      local:
        max_retries: 1
        backoff: "fixed"
        interval: "5s"
        description: "Fast retries for development feedback"
      ci:
        max_retries: 2
        backoff: "fixed"
        interval: "1s"
        description: "Quick retries for test speed"
      staging:
        max_retries: 3
        backoff: "exponential"
        initial_interval: "30s"
        max_interval: "5m"
        multiplier: 2
        description: "Production-like but faster for testing"
      production:
        max_retries: 5
        backoff: "exponential"
        initial_interval: "1m"
        max_interval: "1h"
        multiplier: 2
        jitter: true
        description: "Full production retry with jitter to avoid thundering herd"

    concurrency:
      local: 2
      ci: 4
      staging: 10
      production: 50
      description: "Maximum concurrent webhook deliveries"

  target_url_restrictions:
    local:
      allowed_schemes: ["http", "https"]        # Allow HTTP for local testing
      allow_localhost: true                      # Can target localhost
      allow_private_ip: true                     # Can target private IPs
    ci:
      allowed_schemes: ["http", "https"]
      allow_localhost: true                      # Mock webhook receivers
      allow_private_ip: true
    staging:
      allowed_schemes: ["https"]                 # HTTPS only
      allow_localhost: false
      allow_private_ip: false
      blocklist: ["*.internal.company.com"]      # No internal URLs
    production:
      allowed_schemes: ["https"]                 # HTTPS only
      allow_localhost: false
      allow_private_ip: false
      blocklist: ["*.internal.company.com"]
      require_ssl_verification: true

  logging:
    local:
      log_request_body: true                     # Full payload logging for debugging
      log_response_body: true
      log_headers: true
      log_level: "debug"
    ci:
      log_request_body: true
      log_response_body: true
      log_headers: true
      log_level: "info"
    staging:
      log_request_body: true                     # For debugging integration issues
      log_response_body: true
      log_headers: false                         # Headers may contain secrets
      log_level: "info"
    production:
      log_request_body: false                    # Payloads may contain PII
      log_response_body: false                   # Response may contain PII
      log_headers: false
      log_level: "warn"
      log_delivery_status: true                  # Always log success/failure
      log_delivery_latency: true                 # Always log timing

  alerting:
    local:
      enabled: false
    ci:
      enabled: false
    staging:
      enabled: true
      channel: "slack"
      alert_on: ["permanent_failure"]
      throttle: "1 per minute"
    production:
      enabled: true
      channel: "pagerduty"
      alert_on: ["permanent_failure", "high_failure_rate"]
      high_failure_rate_threshold: "5% over 15 minutes"
      throttle: "1 per 5 minutes"

  mock_mode:
    description: |
      In mock mode, webhooks are not actually delivered. Instead,
      they are recorded in a mock delivery log for testing.
    local:
      enabled: false                             # Real delivery for local testing
      mock_receiver_port: 9999                   # Local mock server available
    ci:
      enabled: true                              # NEVER make real HTTP calls in CI
      mock_response_status: 200
      mock_response_delay: "50ms"
    staging:
      enabled: false                             # Real delivery to test endpoints
    production:
      enabled: false                             # Real delivery always

# ──────────────────────────────────────────────────────
# Feature Flags
# ──────────────────────────────────────────────────────
feature_flags:
  - name: "WEBHOOK_DELIVERY_ENABLED"
    type: "boolean"
    environments:
      local: true
      ci: true
      staging: true
      production: true

  - name: "WEBHOOK_BATCH_DELIVERY"
    type: "boolean"
    description: "Enable batching multiple events into a single delivery"
    environments:
      local: true
      ci: true
      staging: true
      production: false              # Not yet launched

# ──────────────────────────────────────────────────────
# Dependencies
# ──────────────────────────────────────────────────────
dependencies:
  - spec: "environment/environment-variables.spec.yaml"
    variables_used:
      - "WEBHOOK_SIGNING_SECRET"
      - "WEBHOOK_MAX_CONCURRENT"
  - spec: "environment/security.spec.yaml"
    sections_used:
      - "tls (for outbound HTTPS)"
```

### Implementation of Environment-Aware Webhook Delivery

```typescript
// src/features/webhooks/webhook-delivery.service.ts

import { env } from '../../config/environment';

/**
 * Webhook Delivery Service
 * Implements: spec/features/webhook-delivery.spec.yaml v1.0.0
 *
 * Environment-aware: behavior changes based on NODE_ENV.
 */

interface RetryPolicy {
  maxRetries: number;
  backoff: 'fixed' | 'exponential';
  initialIntervalMs: number;
  maxIntervalMs: number;
  multiplier: number;
  jitter: boolean;
}

interface DeliveryConfig {
  timeoutMs: number;
  retryPolicy: RetryPolicy;
  maxConcurrent: number;
  mockMode: boolean;
  logRequestBody: boolean;
  logResponseBody: boolean;
  allowHttp: boolean;
  allowLocalhost: boolean;
  allowPrivateIp: boolean;
}

/**
 * Load delivery configuration based on current environment.
 * All values come from the spec's environment_config section.
 */
function loadDeliveryConfig(): DeliveryConfig {
  const configs: Record<string, DeliveryConfig> = {
    development: {
      timeoutMs: 10_000,
      retryPolicy: {
        maxRetries: 1,
        backoff: 'fixed',
        initialIntervalMs: 5_000,
        maxIntervalMs: 5_000,
        multiplier: 1,
        jitter: false,
      },
      maxConcurrent: 2,
      mockMode: false,
      logRequestBody: true,
      logResponseBody: true,
      allowHttp: true,
      allowLocalhost: true,
      allowPrivateIp: true,
    },
    test: {
      timeoutMs: 5_000,
      retryPolicy: {
        maxRetries: 2,
        backoff: 'fixed',
        initialIntervalMs: 1_000,
        maxIntervalMs: 1_000,
        multiplier: 1,
        jitter: false,
      },
      maxConcurrent: 4,
      mockMode: true,                   // NEVER make real calls in CI
      logRequestBody: true,
      logResponseBody: true,
      allowHttp: true,
      allowLocalhost: true,
      allowPrivateIp: true,
    },
    staging: {
      timeoutMs: 15_000,
      retryPolicy: {
        maxRetries: 3,
        backoff: 'exponential',
        initialIntervalMs: 30_000,
        maxIntervalMs: 300_000,
        multiplier: 2,
        jitter: false,
      },
      maxConcurrent: 10,
      mockMode: false,
      logRequestBody: true,
      logResponseBody: true,
      allowHttp: false,                  // HTTPS only
      allowLocalhost: false,
      allowPrivateIp: false,
    },
    production: {
      timeoutMs: 30_000,
      retryPolicy: {
        maxRetries: 5,
        backoff: 'exponential',
        initialIntervalMs: 60_000,
        maxIntervalMs: 3_600_000,
        multiplier: 2,
        jitter: true,                    // Prevent thundering herd
      },
      maxConcurrent: 50,
      mockMode: false,
      logRequestBody: false,             // PII protection
      logResponseBody: false,
      allowHttp: false,
      allowLocalhost: false,
      allowPrivateIp: false,
    },
  };

  const config = configs[env.NODE_ENV];
  if (!config) {
    throw new Error(`No delivery config for environment: ${env.NODE_ENV}`);
  }

  return config;
}

export const deliveryConfig = loadDeliveryConfig();

/**
 * Calculate next retry delay using the environment's retry policy.
 */
export function calculateRetryDelay(
  attemptNumber: number,
  policy: RetryPolicy,
): number {
  let delay: number;

  if (policy.backoff === 'fixed') {
    delay = policy.initialIntervalMs;
  } else {
    // Exponential backoff
    delay = Math.min(
      policy.initialIntervalMs * Math.pow(policy.multiplier, attemptNumber - 1),
      policy.maxIntervalMs,
    );
  }

  if (policy.jitter) {
    // Add random jitter (0-50% of delay)
    delay = delay + Math.random() * delay * 0.5;
  }

  return Math.round(delay);
}

/**
 * Validate that a target URL is allowed in the current environment.
 * From spec: environment_config.target_url_restrictions
 */
export function validateTargetUrl(url: string): {
  valid: boolean;
  reason?: string;
} {
  const parsed = new URL(url);

  // Check scheme
  if (!deliveryConfig.allowHttp && parsed.protocol === 'http:') {
    return {
      valid: false,
      reason: `HTTP is not allowed in ${env.NODE_ENV}. Use HTTPS.`,
    };
  }

  // Check localhost
  if (
    !deliveryConfig.allowLocalhost &&
    (parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1')
  ) {
    return {
      valid: false,
      reason: `Localhost URLs are not allowed in ${env.NODE_ENV}.`,
    };
  }

  // Check private IP ranges
  if (!deliveryConfig.allowPrivateIp && isPrivateIp(parsed.hostname)) {
    return {
      valid: false,
      reason: `Private IP addresses are not allowed in ${env.NODE_ENV}.`,
    };
  }

  return { valid: true };
}

function isPrivateIp(hostname: string): boolean {
  const privateRanges = [
    /^10\./,
    /^172\.(1[6-9]|2[0-9]|3[0-1])\./,
    /^192\.168\./,
    /^fc00:/,
    /^fd/,
  ];
  return privateRanges.some((range) => range.test(hostname));
}
```

---

## 3.12 The Relationship Between Environment Specs and Application Specs

Let us close with the big picture: how environment specs and application specs relate to each other.

```
                    ┌──────────────────────────┐
                    │    Application Specs      │
                    │                            │
                    │  What the system does.     │
                    │  Features, behaviors,      │
                    │  inputs, outputs,          │
                    │  acceptance criteria.      │
                    │                            │
                    │  Example:                  │
                    │  "Export dashboard to CSV"  │
                    └─────────┬────────────────┘
                              │
                              │ DEPENDS ON
                              │
                    ┌─────────▼────────────────┐
                    │    Environment Specs       │
                    │                            │
                    │  Where the system runs.    │
                    │  Infrastructure, config,   │
                    │  variables, security,      │
                    │  deployment targets.       │
                    │                            │
                    │  Example:                  │
                    │  "PostgreSQL, 30s timeout, │
                    │   HTTPS only in prod"      │
                    └─────────┬────────────────┘
                              │
                              │ IMPLEMENTED BY
                              │
                    ┌─────────▼────────────────┐
                    │  Infrastructure as Code    │
                    │                            │
                    │  How the system is built.  │
                    │  Terraform, Docker, K8s,   │
                    │  CI/CD pipelines.          │
                    │                            │
                    │  Example:                  │
                    │  "RDS instance, ECS task,  │
                    │   GitHub Actions pipeline" │
                    └──────────────────────────┘
```

### The Three Layers of Specification

**Layer 1: Application Specs** answer "What does the software do?"
- Feature specs, API specs, behavior specs
- Environment-independent (ideally)
- Read by Builder Agents to produce application code

**Layer 2: Environment Specs** answer "Where does the software run?"
- Infrastructure requirements, configuration, security
- Environment-specific (by definition)
- Read by DevOps agents to configure deployments

**Layer 3: Infrastructure as Code** answer "How is the environment provisioned?"
- Terraform, CloudFormation, Kubernetes manifests
- The executable form of environment specs
- Applied by deployment pipelines

### When These Layers Interact

The layers interact at specific points:

1. **Application spec references environment config:**
   ```yaml
   # Application spec
   constraints:
     - "Export must complete within ${ENV.EXPORT_TIMEOUT}"
   ```

2. **Environment spec provides the values:**
   ```yaml
   # Environment spec
   EXPORT_TIMEOUT:
     local: "60s"
     production: "30s"
   ```

3. **IaC implements the environment:**
   ```hcl
   # Terraform
   environment = [
     { name = "EXPORT_TIMEOUT", value = "30s" }
   ]
   ```

The Builder Agent reads the application spec and writes code. The code reads the environment variable at runtime. The IaC sets the environment variable at deployment time. The environment spec ensures all three are consistent.

> **Professor's aside:** This three-layer model is how the best engineering organizations operate in 2026. The application developers write feature specs and application code. The platform team writes environment specs and IaC. The CI/CD pipeline validates that everything is consistent. And the specs are the contracts that keep all three layers aligned. If any layer drifts from the spec, the pipeline catches it. That is Spec-Driven Development at its most powerful.

---

## 3.13 Key Takeaways

1. **Environment context is a spec concern.** Specs that ignore deployment context are incomplete. The same code behaves differently in dev, CI, staging, and production, and your spec should acknowledge and control those differences.

2. **Environment variables are spec inputs.** Declare them formally: name, type, which environments require them, defaults, validation rules, and sensitivity classification.

3. **CI/CD pipelines are specs.** Your GitHub Actions workflow is a declarative specification of your development process. Treat it with the same rigor as your feature specs.

4. **Infrastructure as Code is the ultimate environment spec.** Terraform, CloudFormation, CDK, and Kubernetes manifests are executable specifications of your infrastructure.

5. **Feature flags bridge specs and environments.** They allow the same code to exhibit different behavior across environments, controlled by spec-declared toggles.

6. **Security requirements are environment-specific.** What is acceptable in development (HTTP, broad CORS, self-signed certs) is unacceptable in production (HTTPS only, strict CORS, verified certificates).

7. **Testing strategies vary by environment.** Unit tests in CI, smoke tests in production, performance tests in staging. Your spec should declare what gets tested where.

8. **The three layers (Application Specs, Environment Specs, IaC) must stay aligned.** The spec is the contract that ensures consistency across layers.

---

### Exercises

**Exercise 1:** Write a complete environment-aware spec for a file upload feature. The spec should handle: local development (filesystem storage), CI (mock storage), staging (S3 test bucket), and production (S3 production bucket with encryption). Include environment variables, security constraints, and testing strategy.

**Exercise 2:** Create a CI/CD pipeline (GitHub Actions) that validates the webhook delivery spec from section 3.11. The pipeline should: validate the spec schema, run mock-mode tests in CI, run integration tests against a real webhook receiver in staging, and deploy to production with a smoke test.

**Exercise 3:** Write a security spec for an application that handles healthcare data (HIPAA compliance). How do the security constraints differ between environments? What additional constraints does HIPAA impose on the production environment that do not apply to development?

---

*This concludes Module 04: Advanced Orchestration & Agents. You now have the tools to build multi-agent SDD pipelines, manage spec evolution over time, and write specifications that account for the full deployment lifecycle. In Module 05, we will explore the frontier: how SDD integrates with emerging AI capabilities and where the field is heading.*

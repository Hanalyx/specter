# Mastering Spec-Driven Development (SDD)

## Complete Course Index

> *"If the AI fails to build it correctly, the fault lies in the Spec, not the Code."*

**17 Chapters | 5 Modules | Beginner to Advanced**

---

## Course Overview

| Module | Level | Chapters | Focus |
|--------|-------|----------|-------|
| [Module 01](#module-01--foundations-the-contract-mindset) | Beginner | 4 | The "Contract" Mindset |
| [Module 02](#module-02--defining-the-architecture-the-how) | Intermediate | 4 | Schema-First Architecture |
| [Module 03](#module-03--validation--the-feedback-loop) | Intermediate | 3 | Testing, Linting, Context |
| [Module 04](#module-04--advanced-orchestration--agents) | Advanced | 3 | Multi-Agent Workflows |
| [Module 05](#module-05--maintenance--scaling) | Advanced | 3 | Refactoring, Docs, Human-in-the-Loop |

---

## Module 01 — Foundations: The "Contract" Mindset

*Level: Beginner*

### [Chapter 1: From Prose to Protocol](MODULE_01/CHAPTER_01.md)

- 1.1 The Seductive Lie of Natural Language
- 1.2 The Three Eras of AI-Assisted Development
  - Era 1: Vibe Coding (2022-2024)
  - Era 2: Structured Prompting (2024-2025)
  - Era 3: Spec-Driven Development (2025-Present)
- 1.3 Real-World Failures: The Museum of Ambiguity
  - Failure 1: The Accidental Data Deletion
  - Failure 2: The Authentication Hallucination
  - Failure 3: The Scope Creep Generator
  - Failure 4: The Inconsistency Cascade
- 1.4 How the Industry Got Here
  - Anthropic's Path: Constitutional AI and System Prompts
  - Google's Path: Design Docs and Structured Engineering
  - OpenAI's Path: Function Calling and Structured Outputs
  - Meta's Path: Open Models and Community Patterns
- 1.5 A Taxonomy of Ambiguity
  - Type 1: Lexical Ambiguity
  - Type 2: Referential Ambiguity
  - Type 3: Scope Ambiguity
  - Type 4: Temporal Ambiguity
  - Type 5: Priority Ambiguity
  - Type 6: Implicit Requirement Ambiguity
- 1.5.1 A Side-by-Side: Vibe Output vs. Spec Output
- 1.6 The Cost of Ambiguity (Rework, Hallucination, Drift)
- 1.7 The Paradox of Capability
- 1.8 The Spec as a Communication Protocol
- 1.9 What Changes When You Adopt SDD
- 1.10 The Economics of SDD
- 1.11 SDD and the Future of Development
- 1.12 The SDD Manifesto

### [Chapter 2: The Single Source of Truth (SSOT)](MODULE_01/CHAPTER_02.md)

- 2.1 What "Source of Truth" Actually Means
- 2.2 The Spec-Code Relationship
  - The Spec is the "What and Why"
  - The Code is the "How"
  - A Concrete Example
- 2.3 Why Not Just Use Code as the Source of Truth?
  - Reason 1: Code Captures "How", Not "Why"
  - Reason 2: Code Cannot Express Intent for Things That Are NOT There
  - Reason 3: Code Is Too Detailed to Review for Intent
  - Reason 4: Code Is Model-Specific; Specs Are Model-Agnostic
  - Reason 5: Code Cannot Drive Validation
- 2.4 The SSOT Principle in Industry
  - Google's Design Docs
  - Anthropic's Constitutional AI
  - OpenAI's Schema-Driven APIs
- 2.5 The Spec File and the Code File: A Practical Architecture
- 2.6 Version Control for Specs vs. Version Control for Code
- 2.7 What Happens When Spec and Code Diverge
  - Cause 1: Someone Edits the Code Without Updating the Spec
  - Cause 2: The Spec Is Updated but the Code Is Not Regenerated
  - Cause 3: The AI Generates Code That Does Not Match the Spec
- 2.8 The Spec as a Communication Layer
- 2.9 The Living Spec vs. The Dead Document
- 2.10 The SSOT Contract: Rules of Engagement (7 Rules)
- 2.11 Practical Workflow: A Day in the Life
- 2.12 Common Anti-Patterns
  - The Retroactive Spec
  - The Orphan Spec
  - The Kitchen Sink Spec
  - The Immutable Spec
  - The Ignored Spec

### [Chapter 3: The Anatomy of a Micro-Spec](MODULE_01/CHAPTER_03.md)

- 3.1 The Three Pillars (Context, Objective, Constraints)
- 3.2 Pillar 1: Context
  - System Context
  - Feature Context
  - The Context Completeness Test
- 3.3 Pillar 2: Objective
  - The Summary
  - Acceptance Criteria
  - Scope Definition
  - The Delta Principle
- 3.4 Pillar 3: Constraints
  - Types of Constraints (Technical, Security, Performance, Accessibility, Business)
  - The MUST / MUST NOT Convention (RFC 2119)
  - The Constraint Completeness Test
- 3.5 The Complete Micro-Spec: An Annotated Walkthrough
- 3.6 How Each Section Maps to AI Behavior
  - Context → Reduces Hallucination
  - Objective → Defines Completeness
  - Constraints → Prevents Bad Decisions
- 3.7 Common Mistakes: Over-Specifying vs. Under-Specifying
- 3.8 The Micro-Spec and Industry Parallels
  - OpenAI's Function Calling Schema
  - Anthropic's Tool Use Definitions
  - Google's Vertex AI Function Declarations
- 3.9 Building a Micro-Spec: Step by Step
- 3.10 Spec Quality Evaluation (Completeness, Clarity, Appropriateness)
- 3.11 Templates and Starting Points

### [Chapter 4: Practice — From Vibe to Spec](MODULE_01/CHAPTER_04.md)

- 4.1 The Anatomy of a Vibe Prompt
- 4.2 Exercise 1: A Simple CRUD Feature
- 4.3 Exercise 2: A UI Component Request (Date Picker)
- 4.4 Exercise 3: An API Endpoint Request (File Upload)
- 4.5 Common Anti-Patterns in Prompting
  - The Magic Word
  - The Implied Stack
  - The Absent Boundary
  - The Missing Negative
  - The Context Vacuum
  - The Testless Request
  - The Conversational Debug Loop
- 4.6 The Spec Quality Checklist (30+ items)
- 4.7 The Spec Writing Process (Gather → Draft → Review → Generate)
- 4.8 From Spec Back to Prompt: The Delivery Format
- 4.9 Homework: Convert Your Own Prompt

---

## Module 02 — Defining the Architecture (The "How")

*Level: Intermediate*

### [Chapter 1: Schema-First Design](MODULE_02/CHAPTER_01.md)

- 1.1 What Is Schema-First Design?
- 1.2 Why "Shape Before Logic"?
  - AI Models Are Exceptionally Good at Conforming to Schemas
  - Schemas Are Language-Agnostic Specs
  - Schemas Enable Parallel Development
  - Schemas Are Testable
  - Schemas Prevent the "Drift Problem"
- 1.3 How AI Companies Enforce Schema-First at the API Level
  - OpenAI's Structured Outputs
  - Anthropic's Tool Use Schemas
  - Google's Gemini Function Calling
- 1.4 JSON Schema: The Universal Language
- 1.5 Practical Walkthrough: Designing a Complete Data Model
- 1.6 Protocol Buffers and gRPC: Google's Schema-First Philosophy
- 1.7 The Validator Ecosystem: Pydantic and Zod
- 1.8 Schema-First in Practice: The Development Workflow
- 1.9 Common Schema-First Patterns
  - The Envelope Pattern
  - The Create/Update Split
  - The Discriminated Union
  - The Versioned Schema
- 1.10 Schema-First Anti-Patterns
- 1.11 Exercises (User Auth, E-Commerce Cart, Blog System)

### [Chapter 2: The Component Contract](MODULE_02/CHAPTER_02.md)

- 2.1 What Is a Component Contract?
- 2.2 Separation of Concerns: Behavior Spec vs. Visual Spec
- 2.3 The Props Interface as a Contract
  - Designing Props as Contracts
  - The "Controlled vs. Uncontrolled" Decision
- 2.4 State Machines as Component Specs (XState-style)
  - File Upload Component Example
  - State Machine Visualization
- 2.5 Side Effects: What External Things Does This Component Touch?
- 2.6 How Design Systems Use Component Contracts
  - Google's Material Design
  - Meta's React Ecosystem
  - The Headless UI Pattern
- 2.7 Real Example: Specifying a SearchBar Component (Full Walkthrough)
- 2.8 Anti-Patterns in Component Specification
- 2.9 From Contract to Implementation: The Handoff
- 2.10 Exercises (DataTable, NotificationCenter, Spec-to-Implementation Challenge)

### [Chapter 3: API Blueprinting](MODULE_02/CHAPTER_03.md)

- 3.1 What Is an API Blueprint?
- 3.2 OpenAPI/Swagger: The Industry Standard
- 3.3 How to Write an API Spec That an AI Can Implement Faithfully
  - Principle 1: Be Explicit About Every Decision
  - Principle 2: Define Error Responses as Thoroughly as Success Responses
  - Principle 3: Include Examples for Every Endpoint
  - Principle 4: Specify Behavior, Not Just Shape
  - Principle 5: Define What Does NOT Happen
- 3.4 Authentication and Authorization in API Specs
- 3.5 Error Handling as a First-Class Spec Concern
  - The Error Taxonomy
  - Mapping Errors to HTTP Status Codes
  - Per-Endpoint Error Specification
- 3.6 How AI API Providers Practice Spec-Driven Design (Anthropic, OpenAI)
- 3.7 Rate Limiting, Pagination, and Edge Cases
- 3.8 Practical Walkthrough: Full REST API Spec for a Resource
- 3.9 From API Spec to Implementation
- 3.10 Exercises (User Management API, Error Handling, Pagination, Full API Review)

### [Chapter 4: State Management Specs](MODULE_02/CHAPTER_04.md)

- 4.1 Why State Management Needs a Spec
  - The Redux Insight: Actions Are Contracts
- 4.2 Defining Store Shapes (Normalization)
- 4.3 Defining Actions (Specification Format, State Change Specs)
- 4.4 Defining Selectors (Memoization Strategies)
- 4.5 Server State vs. Client State
  - TanStack Query / React Query Patterns
- 4.6 How to Spec Real-Time Data Flows
  - WebSocket Events as Contracts
  - Conflict Resolution Spec
- 4.7 The Relationship Between API Specs and State Management Specs
- 4.8 Zustand Store Specification: A Complete Example
- 4.9 Exercise: Complete State Management Spec for a Chat Application

---

## Module 03 — Validation & The Feedback Loop

*Level: Intermediate*

### [Chapter 1: Spec-to-Test Mapping (TDD for AI)](MODULE_03/CHAPTER_01.md)

- 1.1 Why Test-First Is Even More Important with AI
  - The Human Developer's Testing Problem
  - The AI Developer's Testing Problem
  - The Fundamental Asymmetry
  - What the Industry Is Doing
- 1.2 The Spec-to-Test-to-Implementation Pipeline
- 1.3 Mapping Spec Lines to Test Cases (One-to-One Minimum Principle)
- 1.4 Boundary Conditions and Edge Cases
  - The Boundary Extraction Method
  - The Edge Case Taxonomy
  - The Spec Gap Discovery Process
- 1.5 Testing Frameworks and Their Role in SDD (Jest, Vitest, Pytest, Playwright)
- 1.6 How Anthropic Tests Claude Against Behavioral Specs (Constitutional AI as Testing)
- 1.7 How Google's DeepMind Validates AI Behavior Against Specifications
- 1.8 Property-Based Testing as Spec Validation
  - fast-check (TypeScript)
  - Hypothesis (Python)
- 1.9 The Concept of "Spec Coverage"
  - Measuring and Automating Spec Coverage
- 1.10 Practical Walkthrough: Micro-Spec → Test Suite → Implementation
- 1.11 Teaching the AI to Write Tests FROM the Spec
- 1.12 Exercises (Beginner through Advanced)

### [Chapter 2: Automated Linting of Intent](MODULE_03/CHAPTER_02.md)

- 2.1 Understanding Intent Drift (Three Types)
- 2.2 How AI Companies Think About Intent Drift
  - OpenAI's RLHF: Preventing Drift at the Model Level
  - Anthropic's RLAIF and Constitutional AI: Self-Linting
  - Google's Gemini: Structured Output Validation
- 2.3 Custom ESLint Rules as Spec Enforcers
  - Enforcing Tailwind-Only Styling
  - Enforcing State Management Patterns
  - Enforcing Error Handling Patterns
- 2.4 Architecture Decision Records (ADRs) as Formalized Constraints
- 2.5 Persistent Spec Constraints
  - .cursorrules (Cursor IDE)
  - CLAUDE.md (Anthropic's Claude Code)
  - .github/copilot-instructions.md (GitHub Copilot)
- 2.6 Techniques to Detect Drift
  - Diff Analysis Against Spec Constraints
  - Pattern Matching for Approved Dependencies
  - AST-Based Import Analysis
- 2.7 Building Your Own "Spec Linter" (Complete Framework)
- 2.8 CI/CD Integration: Automated Spec Compliance Checks
- 2.9 Real-World Case Studies of Drift
- 2.10 The Feedback Loop: From Detection to Prevention
- 2.11 Exercises (ESLint Rule, Spec Linter Config, PR Drift Detection, Full System Design)

### [Chapter 3: The Context Window Strategy](MODULE_03/CHAPTER_03.md)

- 3.1 Understanding Context Windows (Claude 200K, Gemini 1M+, GPT, Llama)
- 3.2 Why More Context Isn't Always Better
  - The "Needle in a Haystack" Problem
  - The Google Gemini Paradox
  - Anthropic's Approach: Right-Sized Context
- 3.3 The Registry Pattern: A Master Index of Specs
- 3.4 Hierarchical Specs: System, Module, Component
- 3.5 How to Decide What Context the AI Needs
  - The Context Decision Framework
  - The "Just Enough Context" Principle
- 3.6 Techniques for Context Management
  - Spec Summarization
  - Dependency Graphs for Spec Loading
  - Incremental Context Loading
  - Context Budgeting
- 3.7 How AI Development Tools Handle Large Codebases
  - Anthropic's Claude Code
  - Google's Gemini Code Assist
  - Cursor IDE
- 3.8 The Spec-Aware Prompt: Putting It All Together
- 3.9 Managing Spec Dependencies
  - Types of Spec Dependencies
  - Handling Circular Dependencies
  - Automated Dependency Validation
- 3.10 Practical Exercise: Designing a Spec Registry
- 3.11 Context Window Strategies for Different Model Providers
- 3.12 The Spec Registry in Practice: A Worked Example
- 3.13 Common Pitfalls and How to Avoid Them
- 3.14 Exercises (Context Budget, Spec Decomposition, Context Loader, Registry Design)

---

## Module 04 — Advanced Orchestration & Agents

*Level: Advanced*

### [Chapter 1: The Multi-Agent Workflow](MODULE_04/CHAPTER_01.md)

- 1.1 The Three-Agent Pattern: Architect, Builder, Critic
- 1.2 The Architect Agent: Spec Writer and Refiner
- 1.3 The Builder Agent: Faithful Executor of the Contract
- 1.4 The Critic Agent: Automated Reviewer
- 1.5 Mapping to Real Software Teams (PM → Developer → QA)
- 1.6 How Anthropic's Claude Code Uses Multi-Agent Patterns
  - The Task Tool Pattern
  - Team Spawning
- 1.7 Google's Gemini and DeepMind's AlphaCode
- 1.8 OpenAI's Agent Frameworks (Assistants API, Swarm)
- 1.9 Meta's Approach with Llama-Based Coding Agents
- 1.10 Microsoft's AutoGen and Multi-Agent Frameworks
- 1.11 The Communication Protocol Between Agents
- 1.12 Practical Walkthrough: 3-Agent Pipeline for a Feature Build
- 1.13 Error Handling Between Agents
  - Spec Ambiguity
  - Technical Impossibility
  - Missing Dependencies
- 1.14 Agent Memory and State: Specs as Shared Memory
- 1.15 The Feedback Loop: Critic → Architect (Not Builder)
- 1.16 Real Code Examples: Production-Grade Orchestrator
- 1.17 Key Takeaways and Exercises

### [Chapter 2: Evolutionary Specs](MODULE_04/CHAPTER_02.md)

- 2.1 The Fundamental Problem: Change Is Inevitable
- 2.2 Version Control for Specs: Semantic Versioning
- 2.3 Additive Changes (Safe) vs. Breaking Changes (Dangerous)
- 2.4 Migration Specs: Specifying the Transition from v1 to v2
- 2.5 Deprecation Patterns (ACTIVE → DEPRECATED → REMOVED)
- 2.6 How This Mirrors API Versioning
  - Google's API Versioning Strategy
  - Stripe's API Evolution
- 2.7 Changelog-Driven Development
- 2.8 The "Diff Spec": Specifying Only What Changed
- 2.9 Backward Compatibility as a Spec Constraint
- 2.10 How Anthropic Versions Claude Across Model Releases
- 2.11 How OpenAI Manages Breaking Changes Across GPT Versions
- 2.12 Practical Exercise: Evolving a Spec Through 3 Versions (v1.0, v1.1, v2.0)
- 2.13 Git Strategies for Spec Management
  - Directory Structure
  - Branching Strategy
  - CI Checks
  - Handling Merge Conflicts
- 2.14 Key Takeaways and Exercises

### [Chapter 3: Environment-Aware Specs](MODULE_04/CHAPTER_03.md)

- 3.1 Why Deployment Context Matters in Specs
- 3.2 Environment Variables as Spec Inputs
  - Environment Variable Validation at Startup
- 3.3 CI/CD Pipelines as Specs (GitHub Actions)
- 3.4 Infrastructure as Code: The Ultimate Environment Spec (Terraform)
- 3.5 Docker/Container Specs: Dockerfiles as Deployment Specifications
- 3.6 How Cloud Providers Use Declarative Specs
  - Google Cloud
  - AWS (CloudFormation, CDK)
  - Azure (ARM, Bicep)
- 3.7 Feature Flags as Environment-Aware Spec Toggles
- 3.8 Multi-Environment Testing Specs
- 3.9 Security Specs: Environment-Specific Security Requirements
- 3.10 How Anthropic Deploys Claude Across Environments
- 3.11 Practical Walkthrough: Complete Environment-Aware Spec (Webhook Delivery System)
- 3.12 The Three Layers: Application Specs, Environment Specs, Infrastructure as Code
- 3.13 Key Takeaways and Exercises

---

## Module 05 — Maintenance & Scaling

*Level: Advanced*

### [Chapter 1: The Refactor Spec](MODULE_05/CHAPTER_01.md)

- 1.1 Why Refactoring Needs a Spec
- 1.2 The Archaeology Phase: Understanding What Exists
  - The Four Layers of Code Archaeology
  - The Archaeology Report
- 1.3 The Reverse Spec Technique
- 1.4 Scoping the Refactor: What to Touch and What to Leave Alone
  - The Scoping Matrix
- 1.5 The Strangler Fig Pattern Applied to Specs
- 1.6 Risk Assessment in Refactor Specs (Risk Registry)
- 1.7 How Google Manages Large-Scale Refactors (Rosie, LSCs)
- 1.8 How Anthropic Approaches Iterative Improvement
- 1.9 "Freeze and Replace" vs "Incremental Migration"
- 1.10 Practical Walkthrough: Express.js → NestJS Migration
  - Step 1: The Archaeology Report
  - Step 2: The Reverse Spec
  - Step 3: The Target Spec
  - Step 4: The Migration Spec
  - Step 5: The Test Migration Plan
- 1.11 Dependencies and Ripple Effects
- 1.12 The Refactor Approval Gate
- 1.13 Exercise: Audit and Produce a Prioritized Refactor Spec

### [Chapter 2: Documentation as Code](MODULE_05/CHAPTER_02.md)

- 2.1 The DRY Principle Applied to Documentation
- 2.2 How Specs Eliminate the "Docs Are Always Out of Date" Problem
- 2.3 Tools of the Trade
  - TypeDoc — TypeScript API Documentation
  - Swagger/OpenAPI — API Documentation
  - Storybook — Component Documentation
  - Docusaurus — Full Documentation Sites
- 2.4 Generating API Documentation from OpenAPI Specs
- 2.5 How API-First Companies Use Spec-Driven Documentation
  - Anthropic's API Documentation
  - Stripe's Documentation Model
  - Twilio's Documentation Architecture
- 2.6 The Documentation Pyramid
- 2.7 Living Documentation: Docs That Update When Specs Update
- 2.8 MDX as the Bridge Between Specs and Readable Documentation
- 2.9 Automating Doc Generation in CI/CD Pipelines
- 2.10 The Role of AI in Documentation
- 2.11 Internationalization of Docs from Specs
- 2.12 Practical Walkthrough: TypeScript Spec → OpenAPI → API Reference → User Guide
- 2.13 Exercise: Generate Complete Documentation from 3 Prior Specs

### [Chapter 3: The Human-in-the-Loop (Course Capstone)](MODULE_05/CHAPTER_03.md)

- 3.1 Why Full Automation Is a Myth (And Why That Is Actually Good)
- 3.2 The Trust Spectrum
- 3.3 The Approval Gate Pattern
- 3.4 How Anthropic Implements Human Oversight (RLHF, Red-Teaming, Constitutional AI)
- 3.5 How OpenAI Uses Human Feedback Loops
- 3.6 Google's Responsible AI Practices and Human Oversight
- 3.7 The Confidence Score Concept
- 3.8 Common Deviation Patterns: Where AI Goes Off-Spec
  - The Helpful Addition
  - The Premature Optimization
  - The Silent Error Swallow
  - The Library Substitution
  - The Scope Creep
- 3.9 Designing Review Workflows: PR Reviews as Approval Gates
- 3.10 The Escalation Pattern: When Should the AI Stop and Ask?
- 3.11 Explicit "Check With Human" Markers in Specs
- 3.12 The Balance Between Speed and Safety
- 3.13 How to Train Yourself to Be a Better Spec Reviewer
- 3.14 The Future of Human-AI Collaboration: From Reviewer to Architect
- 3.15 **Final Course Synthesis: Bringing All 5 Modules Together**
- 3.16 **The SDD Maturity Model**
  - Level 1: Spec-Aware
  - Level 2: Spec-Driven
  - Level 3: Spec-Optimized
  - Level 4: Spec-Native
- 3.17 **What Is Next: The Trajectory of AI Development and SDD**
  - Near-Term (2026-2027): AI as Reliable Executor
  - Mid-Term (2027-2029): AI as Collaborative Architect
  - Long-Term (2029+): AI as System Designer
- 3.18 Closing Thoughts and The SDD Manifesto

---

## Quick Reference: Key Concepts by Module

| Concept | First Introduced | Deepened In |
|---------|-----------------|-------------|
| The Micro-Spec (Context, Objective, Constraints) | Module 01, Ch. 3 | Module 02, all chapters |
| Single Source of Truth (SSOT) | Module 01, Ch. 2 | Module 04, Ch. 2 |
| Schema-First Design | Module 02, Ch. 1 | Module 03, Ch. 1 |
| Component Contracts | Module 02, Ch. 2 | Module 05, Ch. 2 |
| API Blueprinting | Module 02, Ch. 3 | Module 04, Ch. 3 |
| Spec-to-Test Mapping | Module 03, Ch. 1 | Module 05, Ch. 1 |
| Intent Drift | Module 03, Ch. 2 | Module 05, Ch. 3 |
| Context Window Strategy | Module 03, Ch. 3 | Module 04, Ch. 1 |
| Multi-Agent Workflow | Module 04, Ch. 1 | Module 05, Ch. 3 |
| Evolutionary Specs | Module 04, Ch. 2 | Module 05, Ch. 1 |
| Approval Gates | Module 05, Ch. 3 | — (Capstone) |

## Suggested Reading Order

**For complete beginners:** Follow modules sequentially (01 → 02 → 03 → 04 → 05).

**For experienced developers new to AI:** Start with Module 01 Ch. 1-2 for philosophy, then jump to Module 02 for practical architecture patterns.

**For those already using AI coding tools:** Skim Module 01, focus on Module 03 (validation) and Module 04 (orchestration).

**For team leads and architects:** Module 01 Ch. 2 (SSOT), Module 04 Ch. 1 (Multi-Agent), Module 05 Ch. 3 (Human-in-the-Loop).

---

*Course materials grounded in the 2026 AI development landscape, referencing practices from Anthropic (Claude), Google (Gemini/DeepMind), OpenAI (GPT), Meta (Llama), Microsoft (AutoGen), Stripe, Twilio, and other industry leaders.*

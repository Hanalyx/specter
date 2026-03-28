# Mastering Spec-Driven Development (SDD)

> *"If the AI fails to build it correctly, the fault lies in the Spec, not the Code."*

A comprehensive course that teaches developers how to stop writing code via loose prompts and start **architecting systems via structured specifications**. By the end, you'll think like a Product Architect -- writing specs that any AI model can execute faithfully.

## Why This Course Exists

The AI coding landscape has evolved through three distinct eras:

1. **Vibe Coding (2022-2024)** -- "Build me a login page" and hoping for the best
2. **Structured Prompting (2024-2025)** -- Better prompts, but still too ambiguous for complex systems
3. **Spec-Driven Development (2025-Present)** -- Formal specifications as contracts between human intent and AI execution

Most developers are still stuck in eras 1 or 2. This course takes you to era 3.

## What You'll Learn

- Write **micro-specs** with Context, Objective, and Constraints that eliminate ambiguity
- Design **schemas, component contracts, and API blueprints** before any code is written
- Build **validation pipelines** where tests are generated from specs before implementation
- Orchestrate **multi-agent workflows** (Architect, Builder, Critic) for complex feature builds
- Manage **spec evolution** as requirements change without breaking existing systems
- Master the **human-in-the-loop** -- knowing exactly where AI deviates and how to prevent it

## Course Structure

| Module | Level | Chapters | What You'll Master |
|--------|-------|:--------:|---------------------|
| [**01 -- Foundations**](MODULE_01/) | Beginner | 4 | The contract mindset, SSOT, micro-spec anatomy |
| [**02 -- Architecture**](MODULE_02/) | Intermediate | 4 | Schema-first design, component contracts, API blueprints, state specs |
| [**03 -- Validation**](MODULE_03/) | Intermediate | 3 | TDD for AI, intent drift linting, context window strategy |
| [**04 -- Orchestration**](MODULE_04/) | Advanced | 3 | Multi-agent workflows, evolutionary specs, environment-aware specs |
| [**05 -- Maintenance**](MODULE_05/) | Advanced | 3 | Refactor specs, documentation as code, human-in-the-loop |

**17 chapters | 28,000+ lines of lecture content | Beginner to Advanced**

See the full [Table of Contents](INDEX.md) for a detailed breakdown of every section.

## Where to Start

**Complete beginner?** Start at [Module 01, Chapter 1: From Prose to Protocol](MODULE_01/CHAPTER_01.md) and work sequentially.

**Experienced developer new to AI-assisted coding?** Read Module 01 Chapters 1-2 for the philosophy, then jump to [Module 02](MODULE_02/) for hands-on architecture patterns.

**Already using AI coding tools (Cursor, Claude Code, Copilot)?** Skim Module 01, then focus on [Module 03 (Validation)](MODULE_03/) and [Module 04 (Orchestration)](MODULE_04/).

**Team lead or architect?** Go straight to [SSOT](MODULE_01/CHAPTER_02.md), [Multi-Agent Workflows](MODULE_04/CHAPTER_01.md), and [Human-in-the-Loop](MODULE_05/CHAPTER_03.md).

## What's Inside Each Chapter

Every chapter is written as a **full lecture** from a patient, knowledgeable professor and includes:

- Real-world examples grounded in how **Anthropic, Google, OpenAI, Meta, Microsoft, Stripe**, and other industry leaders approach these problems
- Production-grade **TypeScript, Python, YAML, and JSON** code examples (not pseudocode)
- **Professor's Aside** callouts with hard-won wisdom and industry context
- **Practical exercises** with evaluation rubrics
- **Anti-patterns** -- what NOT to do, and why

## Key Concepts at a Glance

| Concept | What It Means |
|---------|---------------|
| **Micro-Spec** | A structured specification with three pillars: Context, Objective, Constraints |
| **SSOT** | Single Source of Truth -- the spec is authoritative; code is derived from it |
| **Intent Drift** | When AI output gradually deviates from the original specification |
| **Approval Gate** | A human checkpoint where AI work is validated before proceeding |
| **Spec Coverage** | How much of a specification has corresponding test cases |
| **Architect/Builder/Critic** | Three-agent pattern for AI-assisted development at scale |

## Tooling

The methodology taught in this course is implemented by [**Specter**](../specter/) -- a spec compiler toolchain that validates, links, and type-checks `.spec.yaml` files. See the [Specter docs](../specter/docs/GETTING_STARTED.md) to start using the tooling.

## License

All course materials are the intellectual property of the author. All rights reserved.

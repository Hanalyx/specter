# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is **not a software project** — it is a course/textbook repository. The repo contains the full lecture materials for "Mastering Spec-Driven Development (SDD)", a 5-module course teaching developers to transition from vibe-coding to architecting systems via structured specifications.

The core philosophy: **"If the AI fails to build it correctly, the fault lies in the Spec, not the Code."**

## Repository Structure

- `spec-dd_learning.md` — Original course outline/syllabus (the source spec for the course itself)
- `sddbook/INDEX.md` — Complete table of contents with all section headings and navigation links
- `sddbook/MODULE_01/` through `sddbook/MODULE_05/` — Chapter files (`CHAPTER_XX.md`)

## Content Architecture

| Module | Level | Focus |
|--------|-------|-------|
| MODULE_01 (4 chapters) | Beginner | Foundations — Contract mindset, SSOT, micro-spec anatomy |
| MODULE_02 (4 chapters) | Intermediate | Architecture — Schema-first, component contracts, API blueprints, state management |
| MODULE_03 (3 chapters) | Intermediate | Validation — TDD for AI, intent drift linting, context window strategy |
| MODULE_04 (3 chapters) | Advanced | Orchestration — Multi-agent workflows, evolutionary specs, environment-aware specs |
| MODULE_05 (3 chapters) | Advanced | Maintenance — Refactor specs, docs-as-code, human-in-the-loop (capstone) |

## Content Conventions

- All content is Markdown (`.md`) files — no build system, no dependencies
- Chapters are written in **professorial lecture tone** with "Professor's Aside" blockquotes
- Code examples use TypeScript, Python, YAML, and JSON throughout
- Industry references focus on Anthropic (Claude), Google (Gemini/DeepMind), OpenAI (GPT), Meta (Llama), Microsoft (AutoGen), Stripe, and Twilio
- Each chapter follows the structure: Lecture Preamble → Numbered Sections → Exercises → Summary/Discussion Questions
- Chapter numbering is scoped to the module (each module's chapters start at 1.x)

## Key SDD Terminology

These terms are used consistently throughout and should not be renamed or redefined:

- **Micro-Spec**: A structured specification with three pillars — Context, Objective, Constraints
- **SSOT**: Single Source of Truth — the `.spec` file is authoritative over `.code`
- **Intent Drift**: When AI output gradually deviates from the original spec
- **Approval Gate**: A checkpoint where humans validate AI work before it proceeds
- **Spec Coverage**: Analogous to code coverage — measures how much of a spec has corresponding tests
- **The Three Eras**: Vibe Coding (2022-2024) → Structured Prompting (2024-2025) → SDD (2025-Present)

## When Editing Content

- Maintain the professorial tone — patient, knowledgeable, conversational
- Preserve cross-references between modules (concepts introduced in earlier modules are deepened in later ones — see the Quick Reference table in INDEX.md)
- Keep code examples production-grade and runnable, not pseudocode
- `spec-dd_learning.md` is the **source syllabus** — if adding new topics, check alignment with it first
- INDEX.md must stay in sync with actual chapter content — update it when chapters change

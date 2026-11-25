# TODO.AI.md

## Critical (P0) — Security and data integrity

- [ ] Map and resolve any security, privacy, authentication, data-integrity, or destructive-operation gaps against AI.md PARTS 0–32 and the final checklist.

## High (P1) — Required implementation

- [ ] Compare all required AI.md PARTS 0–32 against the current code, tests, server/API routes, client, and runtime behavior; implement every missing or incorrect requirement.
- [ ] Reconcile project structure and configuration with AI.md, including removing tracked temporary tooling that is prohibited in the project tree.

## Medium (P2) — User-facing surfaces and documentation

- [ ] Bring every registered server page, paired API surface, template, accessibility behavior, and translation into compliance with AI.md; add or correct coverage for each.
- [ ] Reconcile IDEA.md's Business Logic with shipped behavior and AI.md without changing Project variables; update README.md and all docs/ pages to describe the actual product and interfaces.
- [ ] Regenerate all 13 `.claude/rules/*.md` cheatsheets from their assigned AI.md PARTs and keep project memory accurate without duplicating generic rules.

## Low (P3) — Build, release, and final verification

- [ ] Align Makefile, Docker, CI/CD providers, service packaging, and tests with their AI.md PARTs; repair all validation failures.
- [ ] Complete the AI.md final checklist, run required build/test/lint/documentation/workflow checks, and verify the resulting behavior and repository diff.

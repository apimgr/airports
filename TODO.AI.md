# TODO.AI.md

## Completed in this alignment pass

- [x] Confirmed the active branch is `main`; preserved the existing worktree edits.
- [x] Applied the `src/paths` package layout and removed tracked temporary Go tooling.
- [x] Aligned the PWA manifest route and build-stamped cache headers, with route and conditional-request coverage.
- [x] Consolidated project-owned browser JavaScript into `static/js/app.js`; documentation vendor bundles remain separate.
- [x] Refreshed all 13 `.claude/rules/*.md` summaries from their AI.md PART assignments.
- [x] Updated the README Official Site section from the existing `site.txt` value and aligned the installation examples with the production bridge bind and optional overlay daemons.
- [x] Verified `make test` (62.0% coverage), Go lint (0 issues), the 8-platform Makefile build, strict MkDocs, GitHub workflow parsing, and the Docker Phase 2 smoke suite. The smoke checks passed; the generic gate wrapper reported that this manual script did not emit a Phase 1 gate marker. I2P was skipped because it was unavailable.

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

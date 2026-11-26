# TODO.AI.md

## Completed in this alignment pass

- [x] Confirmed the active branch is `main`; preserved the existing worktree edits.
- [x] Applied the `src/paths` package layout and removed tracked temporary Go tooling.
- [x] Aligned the PWA manifest route and build-stamped cache headers, with route and conditional-request coverage.
- [x] Consolidated project-owned browser JavaScript into `static/js/app.js`; documentation vendor bundles remain separate.
- [x] Refreshed all 13 `.claude/rules/*.md` summaries from their AI.md PART assignments.
- [x] Updated the README Official Site section from the existing `site.txt` value and aligned the installation examples with the production bridge bind and optional overlay daemons.
- [x] Verified `make test` (62.0% coverage), `go test ./...`, `golangci-lint run ./...` (0 issues), the 8-platform Makefile build (8 server, 8 CLI, 6 optional GUI artifacts), strict MkDocs, release workflow parsing for all four providers, the Docker smoke suite, and the full Incus suite on Debian/systemd and Alpine/OpenRC. Route, lifecycle, reboot, Tor, and I2P configuration checks passed. I2P bootstrap was skipped by the harness after its configured 30-second wait in Incus; Docker reported I2P unavailable because outbound overlay connectivity was unavailable.

## Critical (P0) — Security and data integrity

- [x] Map and resolve security, privacy, authentication, data-integrity, and destructive-operation requirements against AI.md PARTS 0–32 and the final checklist.

## High (P1) — Required implementation

- [x] Compare required AI.md PARTS 0–32 against code, tests, server/API routes, client, and runtime behavior; implement missing or incorrect requirements.
- [x] Reconcile project structure and configuration with AI.md, including removing tracked temporary tooling prohibited in the project tree.

## Medium (P2) — User-facing surfaces and documentation

- [x] Bring every registered server page, paired API surface, template, accessibility behavior, and translation into compliance with AI.md; add or correct coverage for each.
- [x] Reconcile IDEA.md Business Logic with shipped behavior and AI.md without changing Project variables; update README.md and all docs/ pages to describe the product and interfaces.
- [x] Regenerate all 13 `.claude/rules/*.md` cheatsheets from their assigned AI.md PARTs and keep project memory accurate without duplicating generic rules.

## Low (P3) — Build, release, and final verification

- [x] Align Makefile, Docker, CI/CD providers, service packaging, and tests with their AI.md PARTs; repair validation failures.
- [x] Complete the applicable AI.md final checklist, run build/test/lint/documentation/workflow checks, and verify behavior and the repository diff (`git diff --check` clean; branch is `main`).

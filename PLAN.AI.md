# Project Alignment Plan

## Dependency order

1. Inventory all applicable AI.md PARTs and final-checklist requirements against the repository.
2. Fix security, data-integrity, code, configuration, and structure gaps from the inventory.
3. Align each server page and paired API route; then synchronize translations, tests, IDEA.md, README.md, and docs/.
4. Regenerate the 13 rule files and align Makefile, Docker, service, tests, and all CI providers.
5. Run required validation, inspect generated and tracked artifacts, and complete the final checklist.

Steps 2–4 depend on step 1. Step 3 depends on step 2 where routes or behavior change. Step 5 depends on all implementation and documentation work.

## Completion criteria

- Every applicable mandatory AI.md requirement is implemented; optional behavior is enabled only when the spec and IDEA.md say it applies.
- All server pages and user-facing APIs agree with the route/page registry and are covered for their applicable HTML, JSON, and text responses.
- IDEA.md contains product requirements only and accurately describes the shipped product; Project variables remain unchanged.
- README.md, docs/, Swagger/OpenAPI, GraphQL, translations, CI/CD, container files, and rule cheatsheets match the implementation and AI.md.
- Required tests, builds, linters, documentation checks, and workflow validations pass; final repository state is reviewed on `main`.

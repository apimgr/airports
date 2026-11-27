# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

> Quick reference only: read the assigned AI.md PARTs completely before acting. AI.md is authoritative; do not create a SPEC.md exception without the user's explicit approval.

## CRITICAL - NEVER DO

- Guess or assume a requirement — stop and ask
- Pre-load the entire AI.md speculatively — use its PART index, read every applicable PART section by section, and follow cross-references before returning to the original PART
- Run `go` commands on the host — all builds/tests are Docker (`casjaysdev/go:latest`) or Incus
- Modify AI.md — it is read-only source of truth; project changes go in IDEA.md
- Add or amend a SPEC.md exception on your own — explain the conflict and request the user's explicit approval first
- Create report files (AUDIT.md, COMPLIANCE.md, SUMMARY.md) — fix issues directly instead
  (temporary `AUDIT.AI.md` is allowed only for an explicit audit with >5 issues, deleted when resolved)
- Reference the assistant as an author anywhere — code, comments, commits, PRs, docs must read as
  written entirely by a human senior developer, with no tool-authorship trailers or mentions
- Weaken auth/TLS/CSRF/CSP/CORS/rate-limiting/input-validation to make something "easier"
- Use `SELECT *` in application code
- As a subagent: write `.git/COMMIT_MESS` or call `gitcommit` — report back to the parent instead

## CRITICAL - ALWAYS DO

- Read the relevant AI.md PART(s) before implementing; re-verify every 3-5 changes (drift check)
- Treat server apps as internet-facing and hostile-traffic-exposed by default
- When a spec requirement appears to break product functionality, stop and propose the exception; only record it in SPEC.md after explicit user approval, and keep the product reason in IDEA.md
- Validate/sanitize all input; parameterized queries only; name columns explicitly (never `SELECT *`)
- Pair each user-facing API route with an HTML page that presents the same data, and each page with its API route; system-only and explicitly machine-only routes may be API-only per AI.md PART 1 and PART 14
- Verify with real tools before claiming "done" (`make test`, curl, build, etc.) — never "looks right"
- Keep IDEA.md in sync with actual features; keep README/docs/Swagger/GraphQL in sync with code
- Use intent-revealing names — no bare `Mode`, `Type`, `Status`, `Config`, `Get()`, `New()`, etc.

- During an explicit audit, walk every AI.md PART line by line before forming compliance findings; fix verified gaps directly and use `AUDIT.AI.md` only when more than five issues were found.
- Keep all 13 `.claude/rules/*.md` files current whenever AI.md is newer, following PART 0's file-to-PART mapping and required section order.
- Store assistant-wide COMMIT/NEVER/MUST memory in the assistant's session memory only; do not duplicate it in project `.claude/memory/`, which is reserved for project-specific discoveries.
- Record only verified defects in AI.md itself in the parent template inbox; project implementation tasks belong in `TODO.AI.md`.

## Key Rules Summary

| Area | Rule |
|------|------|
| Spec hierarchy | A user-approved SPEC.md exception may override AI.md; otherwise AI.md is source of truth and wins over IDEA.md |
| Container-only dev | `make dev` / `make local` / `make build` / `make test` — never bare `go build` |
| Security posture | Fail secure, least privilege, defense in depth, secure by default |
| Client requirement | Every project ships the CLI client; PART 32 has no skip case |
| Error messages | User: minimal. Operator/log: actionable. Never leak stack traces, paths, internals |
| Naming | files `lowercase_snake.go`; public `PascalCase`; private `camelCase`; booleans `isX/hasX/canX` |
| Commit workflow | `gitcommit --dir {dir} all` only; never plain `git commit`/`git push` |
| TODO.AI.md | AI-owned, remove items only once resolved+committed; completion ritual on full clear |
| Assistant memory | Global COMMIT/NEVER/MUST rules belong in assistant session memory; `.claude/memory/` holds project-specific discoveries only |

For complete details, see AI.md PART 0, PART 1.

# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Guess or assume a requirement — stop and ask
- Pre-load the entire AI.md speculatively — read only the PART(s) the current task needs
- Run `go` commands on the host — all builds/tests are Docker (`casjaysdev/go:latest`) or Incus
- Modify AI.md — it is read-only source of truth; project changes go in IDEA.md
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
- Validate/sanitize all input; parameterized queries only; name columns explicitly (never `SELECT *`)
- Give every feature a web route (HTML) AND a matching `/api/{api_version}/...` route (JSON)
- Verify with real tools before claiming "done" (`make test`, curl, build, etc.) — never "looks right"
- Keep IDEA.md in sync with actual features; keep README/docs/Swagger/GraphQL in sync with code
- Use intent-revealing names — no bare `Mode`, `Type`, `Status`, `Config`, `Get()`, `New()`, etc.

## Key Rules Summary

| Area | Rule |
|------|------|
| Spec hierarchy | SPEC.md > AI.md > global CLAUDE.md; AI.md wins over IDEA.md if they conflict |
| Container-only dev | `make dev` / `make local` / `make build` / `make test` — never bare `go build` |
| Security posture | Fail secure, least privilege, defense in depth, secure by default |
| Error messages | User: minimal. Operator/log: actionable. Never leak stack traces, paths, internals |
| Naming | files `lowercase_snake.go`; public `PascalCase`; private `camelCase`; booleans `isX/hasX/canX` |
| Commit workflow | `gitcommit --dir {dir} all` only; never plain `git commit`/`git push` |
| TODO.AI.md | AI-owned, remove items only once resolved+committed; completion ritual on full clear |

For complete details, see AI.md PART 0, PART 1.

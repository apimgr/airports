# AI Rules (PART 0, 1)

Cheatsheet — see AI.md PART 0 (line 2331) and PART 1 (line 4526) for full text.

## AI.md Hierarchy

- `AI.md` (PARTS 0-33) — HOW to implement. **NEVER modify.**
- `IDEA.md` — WHAT the project does. Update as features change.
- `CLAUDE.md` — short loader, ≤20 lines.
- `.claude/rules/*.md` — auto-loaded cheatsheets (this directory).

## Never Guess Rules

| Situation | Action |
|-----------|--------|
| Unsure about requirement | STOP and ASK |
| Can't find file/function | Search first, ask if not found |
| Multiple valid approaches | List options, ask user |
| Spec seems incomplete | Ask for clarification |
| Don't know the answer | Say "I don't know" and research |

## Mandatory Workflow

1. Read PART 0 + PART 1 at session start.
2. Before each task: identify relevant PART(s), read them completely.
3. Implement exactly as specified.
4. Every 3-5 changes: re-verify against spec.
5. When you see "See PART X": jump, read, return.

## Red Flags — STOP IMMEDIATELY

"This is probably what they meant…" · "I'll just assume…" · "This should work…" · "Close enough…" · "I think I remember…" · "Let me quickly…" · "I don't need to check…"

## Verification Before "Done"

- [ ] Read relevant files (not skimmed)
- [ ] Searched for existing patterns
- [ ] Tested changes (or explained why couldn't)
- [ ] Output matches expectations
- [ ] Did NOT guess or assume

## Full Web Application Architecture

Every feature MUST work via Browser (HTML), PWA, API clients (JSON), and (if PART 32) CLI tool.

| Web Route (HTML) | API Route (JSON) |
|------------------|------------------|
| `/` | `/api/{api_version}/` |
| `/server/healthz` | `/api/{api_version}/server/healthz` |
| `/server/docs/swagger` | `/api/{api_version}/server/swagger` (alias `/api/swagger`) |
| `/server/docs/graphql` | `/api/{api_version}/server/graphql` (alias `/api/graphql`) |

Rule: every web page has a corresponding API endpoint; every API endpoint can be displayed in a web page.

## Container-Only Development

Local machine does NOT have Go installed. NEVER run `go` commands directly.

| Task | Tool |
|------|------|
| Build | `make dev` / `make local` / `make build` (Docker `golang:alpine` internally) |
| Unit tests | `make test` |
| Integration tests | `./tests/run_tests.sh` (auto-detect incus/docker) |
| Full OS test | `./tests/incus.sh` (preferred — systemd) |

## Security-First Design

| Principle | Rule |
|-----------|------|
| Never trust input | Validate type/length/format/range before use |
| Defense in depth | Multiple layers, not single points |
| Fail secure | On error, deny |
| Secure by default | Safe defaults; users opt-in to less secure |
| Internet-facing baseline | Assume hostile public network unless explicitly private |
| Suggest, don't block | Recommend MFA; never force |

## Attack Prevention

- SQL Injection: parameterized queries ONLY
- XSS: HTML-escape user content + CSP headers
- CSRF: tokens on all state-changing forms
- Command injection: never shell with user input
- Path traversal: `filepath.Clean()`, reject `..`
- DDoS: rate limiting + size limits + timeouts

## Rate Limiting Defaults

| Endpoint | Limit | Window |
|----------|-------|--------|
| Login | 5 | 15 min |
| Password reset | 3 | 1 hr |
| API auth/unauth | Configurable | 1 min |
| Registration | 5 | 1 hr |

## No Report Files

Fix issues directly. No `AUDIT.md`, `COMPLIANCE.md`, `SUMMARY.md`. Temporary `AUDIT.AI.md` allowed only for explicit audits; delete when resolved.

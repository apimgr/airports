# Airports SPEC

Project: airports
Role: Efficient loader for AI.md

THIS FILE IS AUTO-LOADED EVERY CONVERSATION. FOLLOW IT EXACTLY.

Purpose:
- This file is a short loader for the most important rules
- `AI.md` is the full source of truth
- For complete details, read the referenced PARTs in `AI.md`

## FIRST TURN - MANDATORY

On EVERY new conversation or after "context compacted":
1. READ `AI.md` PART 0 and PART 1 before doing ANYTHING
2. READ `IDEA.md` for project description, variables, and business logic
3. READ the relevant `.claude/rules/*.md` for the current task
4. NEVER assume or guess - verify against AI.md before implementing

## Project Variables (from IDEA.md)

| Variable | Value |
|----------|-------|
| project_name | airports |
| project_org | apimgr |
| internal_name | airports |
| internal_org | apimgr |
| app_name | Airports API |
| binary | airports |
| client_binary | airports-cli |
| license | MIT |
| repo | https://github.com/apimgr/airports |

## CRITICAL - NEVER DO

- Modify `AI.md` (read-only spec)
- Use bcrypt for passwords (use Argon2id)
- Put Dockerfile or docker-compose.yml in project root (must be in `docker/`)
- Use `.env` files (hardcode sane defaults in docker-compose)
- Enable CGO (`CGO_ENABLED=0` always)
- Run `go build`/`go test`/`go run` directly on host (use `make dev`/`make local`/`make build`/`make test` — Docker internally)
- Use `strconv.ParseBool()` (use the project `config.ParseBool()`)
- Add user accounts, login, or admin web panel (this project is public read-only)
- Hardcode dev machine values (hostname, IP, cores) — detect at runtime
- Use plural directory names in `src/` (use `handler/`, `model/`, not `handlers/`)
- Leave `TODO`/`FIXME`/`HACK` in committed code
- Add AI attribution to commits (`Co-Authored-By:`, "Generated with X")

## CRITICAL - ALWAYS DO

- Read the relevant `AI.md` PART before implementing any feature
- Build via Makefile targets only (Docker internally — `golang:alpine`)
- Embed assets with Go `embed` (single static binary)
- Detect hostname / IP / CPU / memory at runtime
- Use server-side Go templates (no client-side rendering)
- Hash tokens with SHA-256 before storing; never log raw credentials
- Pin third-party GitHub Actions to a full commit SHA
- Keep `release.txt` as the sole version source of truth

## KEY DECISIONS (pre-answered)

| Question | Answer | Reference |
|----------|--------|-----------|
| Password hash? | Argon2id | AI.md PART 11 |
| Where is Dockerfile? | `docker/Dockerfile` | AI.md PART 26 |
| CGO enabled? | NEVER | AI.md PART 7 |
| Premium / paid tiers? | NEVER (all features free) | AI.md PART 1 |
| External cron? | NEVER (built-in scheduler) | AI.md PART 18 |
| Client-side rendering? | NEVER (server-side Go templates) | AI.md PART 16 |
| User accounts? | NEVER (this project is public read-only) | IDEA.md |

## Where to Find Details

| Topic | AI.md Section |
|-------|---------------|
| Directory structure | "Directory Structure" |
| File naming conventions | "File & Directory Naming Conventions" |
| Build & binary rules | "Build & Binary Rules" |
| Docker | "Docker Rules" + PART 26 |
| CI/CD | "CI/CD Rules" + PART 27 |
| Makefile | PART 25 |
| API routes | "Route Naming Convention", "Route Compliance" |
| Security | "Security-First Design", PART 11 |

## COMPLIANCE CHECK

Before completing ANY task:
- [ ] Read relevant PART(s) in AI.md
- [ ] Implementation matches spec EXACTLY
- [ ] No guessing — all decisions from spec
- [ ] Docs updated if code changed

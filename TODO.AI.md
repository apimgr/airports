# TODO.AI.md

Outstanding items required by AI.md spec that are not yet complete.
Items are ordered by dependency — prerequisites before dependents.

---

## 1. Remove admin panel code (PART 1 / IDEA.md non-goal)

`src/admin/` contains admin authentication and handlers. The spec (IDEA.md) explicitly states "No admin web panel." The admin code must be removed and any server routes that wire in admin handlers must be updated.

Files: `src/admin/handlers.go`, `src/admin/auth.go`, `src/server/server.go` (admin route registration).

## 2. Clean up config struct of admin fields (PART 5 / PART 12)

`src/config/config.go` has `AdminConfig` struct with `Username`, `Password`, `APIToken` fields. These are not needed since there is no admin panel. Remove from the config struct and all references.

Files: `src/config/config.go`, any code reading `config.Server.Admin.*`.

## 3. Fix macOS paths in src/paths/paths.go (PART 4)

macOS user paths currently use `~/.config` and `~/.local/share`. The spec requires:
- Config: `~/Library/Application Support/{project_org}/{internal_name}/`
- Data: `~/Library/Application Support/{project_org}/{internal_name}/`
- Logs: `~/Library/Logs/{project_org}/{internal_name}/`
- Cache: `~/Library/Caches/{project_org}/{internal_name}/`

Linux user log path also uses `.local/share` but spec says `.local/log`.

File: `src/paths/paths.go`

## 4. Add unit tests for src/config/bool.go (PART 28)

`config.ParseBool()` was added during bootstrap but has no test file. Add `src/config/bool_test.go` covering true/false variants, unknown values, and whitespace trimming.

## 5. Fix YAML inline comments in config structs (PART 5)

Several YAML struct tags in `src/config/config.go` have inline Go comments following the struct fields (e.g. `yaml:"port" // Single port...`). While these are Go comments not YAML inline comments, review that any generated or documented YAML examples follow the "comments above, not inline" rule.

## 6. Verify server route table matches spec (PART 14)

The API route table in `src/server/server.go` should be verified against the PART 14 route spec:
- `/api/v1/airports/{ident}` (not `/api/v1/airport/{ident}`)
- `/api/v1/airports/search`
- `/api/v1/airports/nearby`
- `/api/v1/airports/within`
- `/server/healthz`, `/api/v1/server/healthz`
- `/server/about`, `/api/v1/server/about`
- `/server/docs/swagger`, `/api/v1/server/swagger`
- `/server/docs/graphql`, `/api/v1/server/graphql`
- `/server/privacy`, `/server/terms`, `/server/help`
- `/metrics`

Current integration test hits `/api/v1/airport/KJFK` (singular) — verify spec route is `/api/v1/airports/{ident}` (plural) and update.

## 7. Add config.ParseBool() usage where strconv.ParseBool is used (PART 5)

Search codebase for any remaining `strconv.ParseBool` calls and replace with `config.ParseBool`.

## 8. Verify --service subcommands are implemented (PART 23)

`src/service/service.go` provides the `--service` integration. Verify all required subcommands are present:
`install`, `uninstall`, `start`, `stop`, `restart`, `status`, `enable`, `disable`, `logs`.

## 9. Add rate limiting middleware (PART 9 / PART 20)

Rate limiting must be applied to all API endpoints. Verify `src/server/server.go` wires a rate-limit middleware (check `src/server/` for `RateLimitMiddleware`). If missing, implement and wire it.

## 10. Verify response envelope format (PART 14)

All API responses must use the standard envelope:
`{"data": ..., "meta": {"count": N, "page": ...}, "errors": [...]}`.
The integration test uses `{"ok": true, "data": ...}` but health returns `{"status": "healthy"}`.
Ensure all airport data endpoints use the standard envelope and health/about use their own format per spec.

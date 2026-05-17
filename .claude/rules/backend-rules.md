# Backend Rules (PART 9, 10, 11, 31)

Cheatsheet — see AI.md PART 9 (line 13664), PART 10 (line 14052), PART 11 (line 14652), PART 31 (line 43925).

## Error Handling

- Never silently swallow errors (`_ = err`, bare `except:`, `.unwrap()` in production).
- User-facing errors: vague, no stack traces, no internal paths.
- Logged errors: full detail, include request ID, never include raw credentials.
- HTTP errors: standard status codes only — never invent.
- Auth errors: identical messaging + timing for "wrong password" vs "no such user" (enumeration mitigation).

## Caching

- HTTP responses set `Cache-Control` based on resource type.
- Static assets: long max-age + immutable + content-hash filenames.
- API responses: `private, max-age=0` unless explicitly cacheable.
- ETag for large resources.
- Use stale-while-revalidate when appropriate.
- In-process cache: bounded (LRU with size limit) — never unbounded map.

## Database

- SQLite default; PostgreSQL optional via env config.
- Database names are GLOBALLY consistent: `server.db`, `users.db` (this project: only `server.db`).
- All SQLite files in `{data_dir}/db/sqlite/`.
- Connection pool: bounded `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`.
- All queries parameterized — never string concatenation.
- All write paths inside transactions where appropriate.
- Migrations run automatically on startup; idempotent; version tracked in `schema_migrations` table.
- Every query has a context with deadline.

## Cluster / Multi-Instance

- Stateless app instances. Shared state (sessions, cache) goes to external store (DB or Redis).
- Background jobs: leader election via DB advisory lock or external coordinator.
- Health endpoints `/healthz` and `/api/{api_version}/server/healthz` must reflect dependency health.

## Security & Logging (PART 11)

- Password hashing: **Argon2id only**. Never bcrypt, scrypt, MD5, SHA-*.
- Token storage: hash with SHA-256 before storing. Never log raw tokens.
- Constant-time comparison for tokens, password hashes, HMACs.
- All credential display masked: key name kept, value replaced with `xxxxx`.
- CSRF tokens on all state-changing forms.
- CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy headers on all HTML responses.
- Strict-Transport-Security on HTTPS.
- Audit log security-relevant events (auth success/fail, perm changes, admin actions, data exports). Append-only. Never log raw credentials.

## Logging

- Structured logs (JSON in production, text in dev).
- Include: timestamp (ISO 8601 UTC), level, request ID, source location, message, key fields.
- Never log: passwords, tokens, secrets, full request bodies that may contain PII.
- Log files: `{log_dir}/access.log`, `error.log`, `tor.log`.
- Rotate via external rotator (logrotate) or built-in rotation.

## Rate Limiting

| Endpoint class | Default |
|----------------|---------|
| Login | 5 / 15 min + lockout |
| Password reset | 3 / 1 hr (silent) |
| API auth/unauth | Configurable / 1 min + Retry-After |
| Registration | 5 / 1 hr |

All limits configurable via API and config file.

## Input Validation

- Type, length, format, range — all before processing.
- Allowlist permitted values; reject everything else.
- `strings.TrimSpace()` on all text inputs before processing.
- Size-cap untrusted input — no `ReadAll`/`io.Copy` without `LimitedReader`.

## Memory & Resource Safety

- Every network call, DB query, lock acquisition has a timeout/deadline.
- Every opened file/socket has `defer Close()`.
- Goroutines bounded via worker pool, semaphore, or context cancellation. Never unbounded.

## Tor Hidden Service (PART 31)

- Binary owns Tor completely — installs config, starts/stops, manages keys.
- Tor binary present in image; Go binary controls it (never the Dockerfile).
- Config: `{config_dir}/tor/torrc`. Hostname: `{data_dir}/tor/hostname`.
- Hidden service key files (`hs_ed25519_*`) permissions: 0600.
- Default enabled in container (`ENABLE_TOR=true`).
- If Tor fails to start: log error, continue without Tor (graceful degradation).
- .onion address surfaced in banner, web UI, and `/server/about`.

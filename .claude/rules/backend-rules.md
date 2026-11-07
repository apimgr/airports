# Error Handling, Caching, Database, Security & Logging, Tor & I2P Rules (PART 9, 10, 11, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Return stack traces, internal error chains, or filesystem paths in any production HTTP response
- Retry non-retryable errors (4xx) — only retry transient/5xx/timeout/network errors with backoff
- Skip DB query timeouts — every query/transaction MUST run under `context.WithTimeout`
- Use `DROP COLUMN`/`DROP TABLE`/`DELETE`/rename columns in schema updates — additive only, never destructive
- Expose sequential integer PKs in URLs/JSON — external IDs MUST be opaque (UUID v4/v7)
- Compare tokens/passwords/HMACs/signatures with `==`, `bytes.Equal`, or `strings.EqualFold` — always `subtle.ConstantTimeCompare`
- Return different error messages/timing for "wrong password" vs "no such user" vs "token expired" — identical message, identical status, padded to a timing floor (≥100ms)
- Log secrets, tokens, passwords, private keys, credit card numbers, or SMTP credentials — even hashed values of the raw secret
- Emit ANSI color, emojis, or control chars into any log FILE (console/stdout is fine)
- Cast user-controlled content to `template.HTML`, or inline untrusted SVG/XML/HTML in templates
- Use system Tor/i2pd or hardcode Tor ports (9050/9051) — the server owns a dedicated Tor process; an explicitly enabled I2P service owns a dedicated i2pd process or SAM session
- Fail server startup because no Tor binary is available or Tor fails to bootstrap — the required Tor integration is auto-detected and best-effort at runtime
- Create I2P state, ports, config, or SAM sessions while I2P is disabled — I2P is opt-in and defaults off
- Forward Tor hidden-service traffic to the clearnet listener — Tor requires a separate loopback PROXY-protocol backend; I2P uses a separate plain loopback backend

## CRITICAL - ALWAYS DO

- Use the canonical `{ok, data}` / `{ok, error, message}` JSON envelope for every API response (PART 14 is authoritative)
- Log every error with request_id, error_code, http_status, and internal detail (internal detail never in the response)
- Use `CREATE TABLE IF NOT EXISTS` + idempotent `ALTER TABLE ADD COLUMN` for all schema — no migration files, no version table
- Pool every DB connection; size pool per deployment tier; verify with `PingContext` on startup
- Wrap every query/transaction in a context timeout matched to its type (5s simple SELECT, 15s JOIN, 10s write, 60s bulk, 5m migration)
- Apply defense-in-depth for every threat class (SQLi, XSS, enumeration, timing, CSRF) across all 4 layers (input, data access, output, transport) — never assume one layer covers it
- Run every public response through the Output Sanitization Pipeline (allow-list fields, redact sensitive query params, strip internal IPs/paths, truncate, strip dev-only fields, constant-time finalize)
- Classify every publicly-exposed field into Tier 1 (never, even debug), Tier 2 (always public/operational), or Tier 3 (debug-only) — default to Tier 3 when unsure vs Tier 2, default to Tier 1 when unsure vs Tier 3
- Send all mandatory security headers (CSP, HSTS when SSL on, X-Content-Type-Options, Permissions-Policy, etc.) on every response
- Start a dedicated Tor child process when a Tor binary is found (inherits the server's dropped-privilege user), store its data under `{data_dir}/tor/`, and log absence/failure at INFO/WARN — never ERROR/fatal
- Send Tor hidden-service traffic through its dedicated PROXY-protocol loopback listener; use the exported circuit ID as `tor:{circuit_id}` and never expose loopback as the visitor IP
- Keep I2P disabled by default; allocate no port or state unless explicitly enabled, prefer managed i2pd over an external SAM bridge, and log unavailable providers as a warning while continuing startup
- For I2P, persist the destination under `{data_dir}/i2p/site/`, regenerate managed i2pd `tunnels.conf` at startup, enforce 0700 directories/0600 keys, and use a separate plain loopback backend with no PROXY header
- Never route a destination supplied by an inbound Tor visitor through the app's outbound Tor client

## Key Rules Summary

| Area | Rule |
|------|------|
| Error response shape | `{"ok":true,"data":{}}` success; `{"ok":false,"error":"CODE","message":"..."}` failure; optional `details` |
| Error codes | Standard set: `BAD_REQUEST`(400), `VALIDATION_FAILED`(400), `UNAUTHORIZED`(401), `FORBIDDEN`(403), `NOT_FOUND`(404), `CONFLICT`(409), `RATE_LIMITED`(429), `SERVER_ERROR`(500), `MAINTENANCE`(503) |
| Retry/backoff | Exponential 0/1/2/4/8s, capped at 30s max; only for retryable (transient/5xx/timeout) errors |
| Cache drivers | `memory` (dev default) / `valkey` (preferred prod) / `redis` |
| Cache keys | `{type}:{id}`, lowercase, colon-separated, version-prefixed for busting (`v1:user:123`) |
| Cache TTLs | tokens: none; rate counters: 1m; user profile: 5m; config: 1m; static hash: 24h; GeoIP: 7d; page cache: 5m |
| HTTP cache headers | static: `public,max-age=31536000,immutable`; HTML/authenticated: `no-store`; public API: `public,max-age=60` |
| DB engine | SQLite (default) or libsql/Turso remote; `CREATE TABLE IF NOT EXISTS` only, no migrations table |
| Pool sizing | dev 5/2, small 25/5, medium 50/10, large 100/20 (max_open/max_idle) |
| Transactions | Optimistic locking via `version` column; `sql.LevelSerializable` + retry-on-`SQLITE_BUSY` for high-contention paths |
| Auth identity rules | constant-time compares; identical auth error msg+timing; opaque external IDs; never echo credentials |
| Security headers | `X-Content-Type-Options`, `X-Frame-Options`, `CSP`, `Permissions-Policy`, `Referrer-Policy`, HSTS (SSL only), `Reporting-Endpoints`/`Report-To`/`NEL` |
| CSP default | `default-src 'self'`, `script-src 'self'` (no inline), `object-src 'none'`, report to `/api/{api_version}/server/reports/csp` |
| Secrets | `installation_secret`, `cookie_signing_key`, `csrf_token_secret` in `server.db`; `server.security.encryption_key` (AES-256-GCM) in `server.yml`; never logged/returned via API |
| Log files | `access.log`(apache), `server.log`(text), `error.log`(text), `audit.log`(json only, tamper-evident, append-only), `security.log`(fail2ban), `auth.log`(syslog) |
| Log levels | `info` normal ops, `warn` failed/recoverable, `error` needs attention, `critical` security incidents/server failure |
| Log rotation defaults | access.log monthly; audit.log daily; all others weekly,50MB; `keep: none` by default |
| Compliance standards | gdpr/ccpa/hipaa/soc2/pci_dss/iso27001/fedramp/lgpd/pipeda/appi/pdpa — all disabled by default, enable individually |
| Tor hidden service | Always enabled if `tor` binary found; v3 onion; `.onion:{virtual_port}` → dedicated loopback PROXY listener, never the clearnet port |
| Tor process | Dedicated, isolated from system Tor; control on `127.0.0.1:auto`; runs as server's (dropped-privilege) user; dies with server |
| Tor library | `github.com/cretz/bine` (pure Go, CGO_ENABLED=0 compatible) |
| Tor directories | config: `{config_dir}/tor/torrc` (0600); data: `{data_dir}/tor/` (0700); keys: `{data_dir}/tor/site/` (0700) |
| Tor outbound | `server.tor.use_network` (default false); routes via Tor SOCKS5 dialer when enabled, per-user override allowed |
| Tor client identity | `HiddenServiceExportCircuitID haproxy`; parse PROXY v1 metadata, label `tor:{circuit_id}`, never log/display `127.0.0.1` for Tor requests |
| I2P enablement | `features.i2p.enabled` / `I2P_ENABLED=true` / `--i2p`; default false, disabled means no port/config/SAM session |
| I2P providers | Prefer dedicated managed `i2pd`; fall back to reachable SAMv3 bridge (default `127.0.0.1:7656`); unavailable providers warn and do not fail startup |
| I2P backend/state | Separate plain loopback backend (no PROXY header); config `{config_dir}/i2p/`, data/key `{data_dir}/i2p/site/`; directories 0700, keys 0600 |

For complete details, see AI.md PART 9, PART 10, PART 11, PART 31.

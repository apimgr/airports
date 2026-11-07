# Configuration, Application Modes, Server Configuration Rules (PART 5, 6, 12)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Put inline YAML comments — always on the line ABOVE the setting (SHA-pin annotations are the only exception)
- Use `strconv.ParseBool()` directly — always `config.ParseBool()` / `config.IsTruthy()` for every boolean source (env, config, CLI, API, form, query string)
- Fail startup on an invalid config value — warn and replace with the default instead
- Let `--debug`/`DEBUG=true` bypass auth, TLS, CSRF, or any security check — debug affects verbosity/diagnostics ONLY, in every mode including production
- Expose `/debug/*`, pprof, or expvar endpoints unless `--debug`/`DEBUG=true` is explicitly set — otherwise 404
- Trust `X-Forwarded-*` headers from a peer not in `trusted_proxies` (private ranges + `additional` allow-list) — drop them and fall back to `r.Host`/`r.RemoteAddr`
- Rewrite `r.RemoteAddr` before the trust check runs — preserve the original TCP peer in context first; trust gates must evaluate it, not the rewritten client IP
- Re-select a random port after first run — once chosen (random or configured) it is persisted in `server.yml` and reused
- Bind privileged ports (<1024) without escalation, or run permanently as root without a documented IDEA.md exception
- Leak clearnet FQDN, clearnet email, or `Preferred-Languages` into any response served over a detected Tor (`.onion`) request

## CRITICAL - ALWAYS DO

- Validate every config value on load; on invalid input, log a warning and substitute the default — server must still start
- Accept the full truthy/falsy word set (yes/no, on/off, enable/disable, y/n, t/f, etc.), case-insensitive, via `config.ParseBool`/`config.IsTruthy`
- Resolve mode as `--mode` CLI > `MODE` env > default `production`; resolve debug as `--debug` CLI > `DEBUG` env (truthy) > `--mode debug`/`MODE=debug` alias > default `false`
- Treat `MODE=debug`/`--mode debug` as `development` + debug on, but let an explicitly set `DEBUG` env or `--debug` flag override the alias
- Keep debug endpoints (`/debug/pprof/*`, `/debug/vars`, `/debug/config`, `/debug/routes`, `/debug/cache`, `/debug/db`, `/debug/scheduler`) gated strictly behind the debug flag, never behind `development` mode alone
- Sanitize any config dumped via `/debug/config` (redact secrets)
- Resolve FQDN/proto/port/baseurl in reverse-proxy-preferred order, honoring proxy headers only from trusted peers; Tor requests are priority-0 and bypass the trust gate entirely
- Store `server.yml` as the sole source of truth for configuration; database holds resource state/tokens/audit log only, never operator config or user accounts
- Enter maintenance mode (read-only, 503 writes) only for the two truly critical errors (DB connection failure, cannot write files); self-heal continuously and auto-recover
- Resolve `server.contact.*` role emails/webhooks through their documented fallback chains (security→admin, abuse→general→admin, general→admin)
- Sign every outbound webhook POST with `X-Webhook-Signature`/`X-Webhook-Timestamp`/`X-Webhook-ID`/`X-Webhook-Event`

## Key Rules Summary

| Area | Rule |
|------|------|
| Config file name/location | `server.yml` only (never `.yaml`); root `/etc/{internal_org}/{internal_name}/server.yml`, user `~/.config/{internal_org}/{internal_name}/server.yml`; auto-migrate `server.yaml` → `server.yml` on startup |
| Env var classes | Runtime vars (`NO_COLOR`, `TERM`, `DOMAIN`, `MODE`, `DATABASE_*`, `SMTP_*`) always re-checked; init-only vars (`CONFIG_DIR`, `DATA_DIR`, `LOG_DIR`, `PORT`, `LISTEN`, `APPLICATION_*`) apply once on first run only |
| Port default | Random unused port 64000-64999 on first run, then persisted; port 80/443 auto-enable Let's Encrypt/SSL; port 0 = OS-assigned |
| Four operational states | Production (default), Production+Debug, Development, Development+Debug — see behavior tables for logging/caching/error verbosity per state |
| Config precedence | CLI flag > env var > config file > built-in default (mode/debug); `--mode`/`--debug` always win |
| Config validation | Invalid value → log warning, apply default, continue startup — never crash |
| Base URL | `baseurl` resolved: `X-Forwarded-Prefix` > `X-Forwarded-Path` > `X-Script-Name` > config/CLI `server.baseurl`/`--baseurl` > default `/` |
| Trusted proxies | Private ranges + loopback + link-local always trusted; `server.trusted_proxies.additional` adds public IP/CIDR/DNS; ungated direct deployments should leave `additional: []` |
| Tor handling | `tor.onion_address` match = priority-0 Tor request; proto forced `http://`, port stripped, no clearnet leakage, no `Preferred-Languages` |
| Rate limiting | Per-IP sliding window in `server.db` (or external cache if configured); defaults: read 120/min, write 10/min, health 120/min, global burst 240/min; `429` + `Retry-After` header |
| Contact roles | `server.contact.{admin,security,abuse,general}.email` + `webhooks{telegram,discord,slack,generic,...}`; canonical keys only, no aliases |
| Cache config | `server.cache.type`: none/memory (default)/valkey/redis; `url` takes precedence over host/port/password fields |
| Privacy/consent | GDPR/CCPA cookie banner config under `server.privacy.*`; opt-out default (`consent.default_enabled: true`); essential cookies always on |
| Request limits | `server.limits.{max_body_size,read_timeout,write_timeout,idle_timeout}`; defaults 10MB/30s/30s/120s |

For complete details, see AI.md PART 5, PART 6, PART 12.

# Health, Versioning, API Structure, TLS/Let's Encrypt Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Add sub-routes under `/server/healthz` (no `/server/healthz/db`, etc.)
- Leak sensitive data in health responses (credentials, DB conn strings, internal
  IPs, file paths, env vars, config contents, stack traces, `/metrics` internals)
- Use `0.x.x` for the first stable release, a `v` prefix in the version string,
  or leading zeros (`v1.0.0`, `1.0.01` are both wrong)
- Hardcode `v1` in code — always use `APIBasePath()` / `{api_version}`
- Keep legacy/removed endpoints "for compatibility" — delete them, no shims,
  no deprecation periods, no redirects
- Use singular resource names, mixed case, underscores, or verbs in API routes
  (`/item`, `/Items`, `/api_keys`, `/getItems` are all wrong)
- End a route with a trailing slash, or duplicate a path param as a query param
- Emit a bare top-level JSON array, or invent a one-off error/success shape
  outside the canonical `{ok, data}` / `{ok, error, message, details}` envelopes
- Manually hand-edit `openapi.json` or a GraphQL schema file — both are
  generated from code at build time and must live under `src/swagger/` /
  `src/graphql/` only (never project root)
- Implement RFC-defined protocols (DNS, SMTP, DHCP, HTTP, FTP, NTP, WebDAV)
  partially — full RFC compliance is mandatory once you're building that protocol
- Replicate an external service's entire API when the user only asked for
  "compatibility" — default is feature compatibility, not route cloning
- Let the app auto-renew certs outside `{config_dir}/ssl/letsencrypt/{fqdn}/`,
  or touch `/etc/letsencrypt/**` renewal (system/certbot owns that path)
- Put icons/emojis/ASCII art in log output — logs are always raw plain text

## CRITICAL - ALWAYS DO

- Serve `/server/healthz` (HTML/text/JSON via content negotiation) and
  `/api/{api_version}/server/healthz` (JSON default); mount `/healthz` as the
  exact same handler only when `server.healthz.root.enabled: true`
- Keep the health response field order fixed: project → status → version/build
  → runtime → features → checks → stats; extend via IDEA.md, never reinvent
- Follow SemVer strictly for stable releases (MAJOR.breaking, MINOR.feature,
  PATCH.fix); read version from `release.txt` → git tag → `dev` fallback
- Version every API route as `/api/{api_version}/...`, plural nouns, lowercase,
  hyphens for multi-word, path params for identity, query params for modifiers
- Give every project-specific API endpoint a matching frontend route; make
  CRUD fully usable without JavaScript (progressive enhancement only)
- Do server-side processing (validation, transforms, business logic,
  formatting, pagination, rendering) — JS is enhancement only, never required
- Ship all 3 API surfaces (REST, Swagger, GraphQL), auto-generated and kept in
  sync with the code and each other, themed to match the site (dark default)
- Use the canonical response envelopes: single item bare, action response
  `{ok:true, data:{...}}`, error `{ok:false, error, message, details}`,
  paginated `{data:[], pagination:{page, limit, total, pages}}` (default limit 250)
- 2-space indent for JSON/HTML/YAML/CSS/JS; tabs for Go/Makefiles; every
  response and file ends with exactly one trailing newline
- Support all 3 Let's Encrypt challenge types (HTTP-01, TLS-ALPN-01, DNS-01)
  and every lego DNS-01 provider via `server.tls.dns_provider` + encrypted creds
- Resolve `{fqdn}` in priority order: reverse-proxy headers → `DOMAIN` env →
  `os.Hostname()` → `$HOSTNAME` → public IPv6 → public IPv4 → `localhost`
- Check for existing certs in order: `/etc/letsencrypt/live/domain/` →
  `/etc/letsencrypt/live/{fqdn}/` → `{config_dir}/ssl/letsencrypt/{fqdn}/` →
  `{config_dir}/ssl/local/{fqdn}/`; only request a new LE cert if none found
- Auto-renew app-managed LE certs 7 days before expiry (daily check, 03:00);
  never touch system (`/etc/letsencrypt`) or user-managed (`ssl/local`) certs
- Always strip `:80`/`:443` from displayed URLs; adapt the startup banner to
  terminal width (full/compact/minimal/micro) and respect `NO_COLOR`/`--color`

## Key Rules Summary

| Area | Rule |
|------|------|
| Health endpoints | `/server/healthz` (HTML/text/JSON), `/api/{api_version}/server/healthz` (JSON), `/api/healthz` (unversioned alias), optional `/healthz` root alias — same handler, no redirects |
| Health field order | project, status, version/build, uptime/mode/timestamp, features, checks, stats — app-specific fields extend, never replace |
| Health security | Only `ok`/`error`/aggregate counts/public version info; never credentials, DB strings, paths, internal IPs |
| Versioning scheme | Stable: `MAJOR.MINOR.PATCH` (start at `1.0.0`); Beta: `YYYYMMDDHHMMSS-beta`; Daily: `YYYYMMDDHHMMSS` |
| Version source order | `release.txt` → git tag → `dev` |
| API route rules | Versioned, plural, lowercase, hyphenated, no trailing slash, no verbs — nouns + HTTP methods only |
| Legacy vs compat | Legacy (your own old routes) = delete always; Compatibility (external service) = feature parity by default, routes only if explicitly requested |
| Unversioned aliases | Only for stable/tooling endpoints (`/api/swagger`, `/api/graphql`, `/api/healthz`) — served directly, never a redirect, same handler as versioned route |
| Content negotiation (API) | Default JSON; `.txt` ext / `Accept: text/plain` / non-interactive client → text |
| Content negotiation (frontend) | Browser → HTML+JS; our CLI → JSON; text browsers → no-JS HTML; HTTP tools (curl/wget) → HTML2Text |
| Response envelopes | Item: bare object. Action: `{ok, data}`. Error: `{ok:false, error, message, details}`. List: `{data, pagination}` |
| Swagger/GraphQL location | `src/swagger/*.go`, `src/graphql/*.go` only; JSON-only OpenAPI (no YAML); build-time generated, never hand-edited |
| RFC protocols | DNS/SMTP/DHCP/HTTP/FTP/NTP/WebDAV servers require FULL RFC compliance, not partial |
| TLS challenge types | HTTP-01 (port 80), TLS-ALPN-01 (port 443), DNS-01 (wildcard, any DNS provider via lego) |
| FQDN resolution order | Reverse-proxy headers → `DOMAIN` env → `os.Hostname()` → `$HOSTNAME` → public IPv6 → public IPv4 → `localhost` |
| Cert lookup order | `/etc/letsencrypt/live/domain/` → `/etc/letsencrypt/live/{fqdn}/` → `{config_dir}/ssl/letsencrypt/{fqdn}/` → `{config_dir}/ssl/local/{fqdn}/` |
| Cert ownership | `/etc/letsencrypt/**` = system/certbot (app never renews); `ssl/letsencrypt/{fqdn}/` = app auto-renews (7 days before expiry); `ssl/local/{fqdn}/` = user, manual only |
| Port/proto rules | Single port ≠443 → HTTP; single port 443 → HTTPS-only; dual port → first=HTTP, second=HTTPS; always strip `:80`/`:443` from URLs |
| Logs vs console | Startup banner/console: icons, colors, ASCII art (responsive to width); log files/stdout logs: always raw plain text, never icons |

For complete details, see AI.md PART 13, PART 14, PART 15.

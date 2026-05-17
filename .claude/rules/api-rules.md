# API Rules (PART 13, 14, 15)

Cheatsheet — see AI.md PART 13 (line 18678), PART 14 (line 19438), PART 15 (line 21154).

## Health & Versioning

- Health endpoints exist independently:
  - `/server/healthz` (web, returns HTML status page)
  - `/api/{api_version}/server/healthz` (JSON: status, version, uptime, deps)
- `/server/about` and `/api/{api_version}/server/about` — version, build, commit, official site.
- `release.txt` is the SOLE version source of truth.
- Version exposed via `--version`, `/server/about`, `/api/.../about`, banner, response header `X-App-Version`.

## API Versioning

- URL-prefix versioning: `/api/v1/...`, `/api/v2/...`.
- Current version: `v1`.
- Unversioned aliases for stable endpoints: `/api/swagger`, `/api/graphql`.
- Breaking changes → new major version, old version retained for one full release cycle.
- Version response header: `X-API-Version: v1`.

## API Structure (PART 14)

### Required server endpoints (under `/api/{api_version}/server/`)

| Endpoint | Purpose |
|----------|---------|
| `/server/healthz` | Health JSON |
| `/server/about` | Build metadata |
| `/server/swagger` | OpenAPI spec |
| `/server/graphql` | GraphiQL UI + POST endpoint |
| `/server/help` | Help text |
| `/server/privacy` | Privacy policy |
| `/server/terms` | Terms |

Aliases at root: `/api/swagger`, `/api/graphql`.

### Project-specific endpoints (this project)

Read AI.md / IDEA.md for full list. Pattern:

| Web | API |
|-----|-----|
| `/airports` | `/api/v1/airports` |
| `/airports/{ident}` | `/api/v1/airports/{ident}` |
| `/airports/search` | `/api/v1/airports/search` |
| `/airports/nearby` | `/api/v1/airports/nearby` |
| `/airports/within` | `/api/v1/airports/within` |

## Response Format

- Content negotiation via `Accept` header AND `?format=` query param (`json`, `csv`, `geojson`).
- Default JSON.
- Standard envelope:
  ```json
  { "data": ..., "meta": {"count": N, "page": ...}, "errors": [...] }
  ```
- Errors: standard HTTP status + JSON body with `error.code`, `error.message`, `error.request_id`.
- All responses include `X-Request-ID` header.

## CORS

This project: `Access-Control-Allow-Origin: *` on all API responses (public read-only data — intentional, documented in IDEA.md).

## Rate Limiting

- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` on 429.

## SSL/TLS & Let's Encrypt (PART 15)

- TLS 1.2 minimum; TLS 1.3 preferred.
- Strong cipher suites only (no RC4, no 3DES, no CBC modes with TLS < 1.2).
- HSTS header on HTTPS: `max-age=31536000; includeSubDomains; preload`.
- Certificate sources (auto-detected by binary):
  1. ACME / Let's Encrypt (HTTP-01 or DNS-01 challenge)
  2. Operator-provided cert + key (paths in `server.yml`)
  3. Self-signed (development only)
- Cert + key location: `{config_dir}/ssl/`.
- ACME state: `{config_dir}/ssl/acme/`.
- Auto-renew via built-in scheduler (every 12 hr check, renew when < 30 days).
- ACME HTTP-01 challenge served on port 80; redirects all other traffic to HTTPS.

## OpenAPI / Swagger

- Spec served at `/api/{api_version}/server/swagger.json` (also `/api/swagger.json`).
- Interactive UI at `/server/docs/swagger`.
- Spec MUST stay in sync with code — generated or hand-written but verified in CI.

## GraphQL

- Endpoint: POST `/api/{api_version}/server/graphql` (also `/api/graphql`).
- GraphiQL UI at `/server/docs/graphql` (development only; disabled in production unless `--debug`).
- Query depth limit + complexity limit enforced.

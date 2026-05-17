# Security Model

## Posture in One Sentence

Airports is a public, read-only reference API. There are **no user accounts**, **no admin panel**, **no API keys**, and **no privileged endpoints** — abuse prevention rests entirely on rate limiting, request-size limits, strict input validation, and transport hardening.

## What Exists

| Surface | Auth | Notes |
|---------|------|-------|
| All HTML pages | None | Public read-only |
| All REST endpoints (`/api/v1/...`) | None | Public read-only |
| GraphQL (`/graphql`) | None | Public read-only; depth + complexity limited |
| `/metrics` | Network ACL | Loopback or private network only |
| `/server/healthz` | None | Public |
| Service CLI (`airports --service ...`) | OS user privilege | Requires sudo where needed |

## Trust Boundaries

| Boundary | Trust | Notes |
|----------|-------|-------|
| Embedded airport dataset | Fully trusted | Static, compiled into the binary |
| MaxMind / ip-location-db downloads | Trusted via HTTPS + checksum | Used only for caller GeoIP |
| Incoming HTTP requests | **Untrusted** | All input validated; GeoIP lookup is best-effort |
| CI/CD build pipeline | Trusted | Produces signed binaries |

## Input Validation

- Every query parameter is type-checked and bounds-checked before use.
- Coordinates must be in valid lat/lon ranges (lat -90..90, lon -180..180).
- Search strings are size-capped (≤ 256 chars) and stripped of control characters.
- ICAO/IATA codes are validated against `[A-Z0-9]{3,4}` before any lookup.
- All inputs trimmed of whitespace before processing.

## Rate Limiting

| Endpoint class | Default | Window | Configurable |
|----------------|---------|--------|--------------|
| All endpoints | 100 req | 1 min per IP | Yes — `server.yml` `rate_limit:` block |

Response headers on every API response:

```
X-RateLimit-Limit:     100
X-RateLimit-Remaining: 95
X-RateLimit-Reset:     2026-05-16T12:15:00Z
Retry-After:           60         # on 429 only
```

## Transport Security

- TLS 1.2 minimum; TLS 1.3 preferred.
- Strong cipher suites only (no RC4, no 3DES, no plain CBC < TLS 1.2).
- HSTS on HTTPS: `max-age=31536000; includeSubDomains; preload`.
- TLS certificates may come from ACME (Let's Encrypt), operator-provided files, or self-signed (development only).
- All ACME challenges served on port 80 with redirect to HTTPS for everything else.

## Tor Hidden Service

The binary owns Tor completely — it installs `torrc`, manages keys (permissions 0600), and starts/stops the Tor process. The `.onion` address is surfaced in the startup banner and `/server/about`.

If Tor fails to start, the server logs a warning and continues without Tor (graceful degradation).

## Response Headers

Every HTML response:

```
Content-Security-Policy:  default-src 'self'; ...
X-Content-Type-Options:   nosniff
X-Frame-Options:          DENY
Referrer-Policy:          no-referrer
Strict-Transport-Security: max-age=31536000; includeSubDomains   (HTTPS only)
```

Every API response:

```
Access-Control-Allow-Origin: *
X-Request-ID:                <uuid>
X-API-Version:               v1
```

`Access-Control-Allow-Origin: *` is intentional — this is a public-data API designed for cross-origin browser use. Documented in `IDEA.md`.

## Logging & Privacy

- No PII is stored.
- GeoIP lookups for callers are used only to surface nearby airports on the home page — results are **never logged or stored**.
- The access log records: timestamp, method, path, status, duration, request ID, hashed IP. Raw IP and User-Agent are not logged.
- The error log never contains user-supplied content beyond the request ID.

## Threats In Scope

- **DoS / DDoS** — mitigated by rate limiting, request-size limits, and timeouts.
- **Bulk scraping** — mitigated by rate limiting; dataset is public and re-distributable so we do not technically prevent bulk download.
- **Injection** — mitigated by strict validation; in-memory dataset eliminates SQL injection by construction.

## Threats Explicitly Out of Scope

- Admin panel compromise — no admin panel exists.
- Account enumeration / credential stuffing — no accounts exist.
- IDOR — no user-scoped resources exist.
- Live ATIS / METAR / NOTAM tampering — not provided.

## Reporting a Security Issue

If you believe you have found a security vulnerability:

1. **Do not** open a public GitHub issue.
2. Email <security@apimgr.us> with reproduction steps and impact.
3. Include the commit/version you tested against.

We aim to acknowledge within 72 hours and ship a fix within 14 days for verified critical/high issues.

## CVE Pre-flight

Dependencies are scanned with `govulncheck` on every CI build. Critical/high vulnerabilities in direct dependencies block release.

## Public Endpoints Reference

For a full list of public endpoints with response shapes, see [api.md](./api.md). For external services the binary contacts, see [integrations.md](./integrations.md).

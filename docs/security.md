# Security Model

## Posture in One Sentence

Airports is a public, read-only reference API. There are **no user accounts** and **no admin panel**. Public HTML and data APIs are unauthenticated; a small set of Tor process-control routes is available only to loopback callers for CLI-to-server communication. Abuse prevention rests on rate limiting, request-size limits, strict input validation, and transport hardening.

## What Exists

| Surface | Auth | Notes |
|---------|------|-------|
| All HTML pages | None | Public read-only |
| Public REST endpoints (`/api/{api_version}/...`) | None | Public read-only data and server information |
| GraphQL (`/api/{api_version}/server/graphql`, alias `/api/graphql`) | None | Public read-only; depth + complexity limited |
| Metrics (`/server/metrics/{service}`, `/api/metrics/{service}`, `/api/{api_version}/server/metrics/{service}`, `/metrics/{service}`) | Per-service bearer token | Network ACL is additional defense in depth; unauthenticated access is not allowed |
| `/server/healthz` | None | Public |
| Tor control (`/server/tor/*`) | Loopback peer | Internal CLI-to-server channel; remote peers receive 404 |
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

| Endpoint class | Default | Window | Identity |
|----------------|---------|--------|----------|
| Read (GET, HEAD) | 120 requests | 60 seconds | Per IP, or per valid bearer token |
| Write (POST, PUT, PATCH, DELETE) | 10 requests | 60 seconds | Per IP, or per valid bearer token |
| Health and status | 120 requests | 60 seconds | Per IP, or per valid bearer token |
| Global ceiling | 240 requests | 60 seconds | Per IP, or per valid bearer token |

Rate-limit values are configured in `server.yml` under `server.rate_limit`. The
limiter is always enabled. Failed authentication attempts remain counted by IP.
Rate-limit response headers report the applicable bucket; `Retry-After` is sent
when a request is rejected.

Metrics use separate bearer tokens for the `prometheus`, `grafana`, and `loki`
services. Firewalling metrics endpoints is additional defense in depth.

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

- Security report details, including researcher contact information, are stored only as AES-256-GCM ciphertext in the database. Contact-form messages are delivered through configured email and are not retained as plaintext records by the application.
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

## `.well-known` and Discovery Files

The server serves these discovery paths:

| Path | Notes |
|------|-------|
| `/robots.txt` | Crawler policy |
| `/sitemap.xml` | Public frontend pages allowed by `robots.txt` |
| `/feed.xml` | Atom 1.0 feed; valid with zero entries for this read-only service |
| `/rss.xml` | RSS 2.0 feed; valid with zero entries for this read-only service |
| `/.well-known/security.txt` | Canonical RFC 9116 security disclosure file |
| `/.well-known/llms.txt` | Generated API discovery document |
| `/llms.txt` | Alias for the generated API discovery document |
| `/.well-known/pgp-key.asc` | Public key when a configured key is available |

Other unsupported `/.well-known/*` paths return `404`. `/security.txt` is not
an alias; use the canonical `/.well-known/security.txt` path.

## Reporting a Security Issue

For a vulnerability in this repository, use GitHub's private vulnerability
reporting flow. For a vulnerability in a running instance, use its
`/server/security` page and submit the security report form linked from
`/.well-known/security.txt`.

Security reports are encrypted at rest using the configured
`server.security.encryption_key`. When SMTP is configured, the operator
notification contains the encrypted report payload and the researcher receives
a private status link with a token that permits one status-page use per day.
The status page exposes the triage state, submission date, maintainer comment,
and expected disclosure date, never report content. One-shot status tokens are
removed from access and application log URLs.
Invalid or expired security form IDs are logged by hash prefix only.

We aim to acknowledge within 72 hours and ship a fix within 14 days for verified critical/high issues.

## CVE Pre-flight

Dependencies are scanned with `govulncheck` on every CI build. Critical/high vulnerabilities in direct dependencies block release.

## Public Endpoints Reference

For a full list of public endpoints with response shapes, see [api.md](./api.md). For external services the binary contacts, see [integrations.md](./integrations.md).

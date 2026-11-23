# Security Model

## Posture in One Sentence

Airports is a public, read-only reference API. There are **no user accounts** and **no admin panel**. Public informational HTML pages and data APIs are unauthenticated; owner-token management is scoped to the owner cookie, and a small set of Tor process-control routes is available only to loopback callers for CLI-to-server communication. Abuse prevention rests on rate limiting, request-size limits, strict input validation, and transport hardening.

## What Exists

| Surface | Auth | Notes |
|---------|------|-------|
| Public HTML pages | None | Public read-only |
| `/server/preferences/tokens` | Owner token cookie | Lists and manages only the token owner's credentials and linked tokens; without a valid owner cookie, only token issue/restore forms appear |
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

MCP at `/api/{api_version}/server/mcp` and its aliases reuses API Bearer
authentication and authorization. Every request needs a linked owner token
created for that client. Never use the primary owner token or the operator
token (`server.token`). API cookies are ignored, and there is no MCP-specific
credential or scope.

## Owner Token Handling

The owner-token page masks credentials by default. A browser create, reveal, or
rotate action uses POST/303 and a one-use `airports_token_view` cookie whose
value identifies only the owner card. The following GET decrypts and displays
only that owner's selected token, sends `Cache-Control: no-store`, and clears
the cookie. Reloads show the masked value again. Raw tokens never appear in a
URL. The confirmation message is rendered by the server for no-JavaScript
users and enhanced to a toast when JavaScript is available.

The API equivalents do not redirect: create, reveal, and rotate return the raw
token in the JSON response body. Store that response securely. Token lists and
ordinary page loads never decrypt token values.

## Transport Security

- TLS 1.2 minimum; TLS 1.3 preferred.
- Strong cipher suites only (no RC4, no 3DES, no plain CBC < TLS 1.2).
- HSTS on HTTPS: `max-age=63072000; includeSubDomains; preload`.
- Preference-token forget sends `Clear-Site-Data: "storage"` to remove the
  browser's convenience copy while keeping the credential valid. A build
  version mismatch sends `Clear-Site-Data: "cache", "storage"` to recover
  stale assets and service workers; it never clears cookies.
- Consent withdrawal sends `Clear-Site-Data: "cache", "storage"` and never
  clears cookies; the server expires cookies for withdrawn categories while
  preserving the updated consent choice and owner token.
- TLS certificates may come from ACME (Let's Encrypt), operator-provided files, or self-signed (development only).
- All ACME challenges served on port 80 with redirect to HTTPS for everything else.

## Tor Hidden Service

The binary owns Tor completely — it installs `torrc`, manages keys (permissions 0600), and starts/stops the Tor process. The `.onion` address is surfaced in the startup banner and `/server/about`.

If Tor fails to start, the server logs a warning and continues without Tor (graceful degradation).

## Response Headers

Every response includes the following baseline security headers (some values are configured from the request and project feature declarations):

```
X-Content-Type-Options: nosniff
X-Frame-Options: SAMEORIGIN
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
X-Permitted-Cross-Domain-Policies: none
Origin-Agent-Cluster: ?1
Cross-Origin-Opener-Policy: unsafe-none (default)
Cross-Origin-Embedder-Policy: unsafe-none (default)
Cross-Origin-Resource-Policy: cross-origin (default)
Content-Security-Policy: generated from the project's policy
Permissions-Policy: generated from declared browser features
Reporting-Endpoints: default="https://{fqdn}/api/{api_version}/server/reports/default"
Report-To: generated reporting endpoint configuration
NEL: generated network error logging configuration
X-Request-ID: per-request UUID
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload (HTTPS only)
```

Public API responses additionally include:

```
Access-Control-Allow-Origin: <resolved configured origin>
X-Request-ID:                <uuid>
X-API-Version:               v1
```

The CORS origin follows the resolved allow-list. A wildcard does not authorize
credentialed requests; browser clients that send credentials need an explicitly
allowed origin.

## Logging & Privacy

- Security report bodies are stored as PGP ciphertext when a project key is available, with AES-256-GCM as the configured-key fallback. Researcher contact details are always stored as AES-256-GCM ciphertext. Operator notifications attach only the encrypted report; without a project PGP key, the message warns that the attachment uses AES-256-GCM. Researcher acknowledgments contain only the tracking ID and one-shot status link.
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
| `/server/security/report/{tracking_id}` | Researcher status page; the emailed one-shot token is required and no API mirror exists |

Other unsupported `/.well-known/*` paths return `404`. `/security.txt` is not
an alias; use the canonical `/.well-known/security.txt` path.

## Reporting a Security Issue

For a vulnerability in this repository, use GitHub's private vulnerability
reporting flow. For a vulnerability in a running instance, use its
`/server/security` page and submit the security report form linked from
`/.well-known/security.txt`.

The disclosure policy at `/server/security/policy` defaults to explicit scope,
out-of-scope activities, a 90-day coordinated-disclosure window, and safe-harbor
terms for good-faith research that follows the policy. Operators can replace
the default Markdown through `web.security.policy`.

Security report bodies use the project PGP key when available and otherwise
the configured `server.security.encryption_key`; contact details always use
AES-256-GCM with that encryption key. When SMTP is configured, the operator
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

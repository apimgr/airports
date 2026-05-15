## Project description

Airports is a full-stack Go web application providing comprehensive global airport information. It serves data on 35,000+ airports worldwide — ICAO/IATA codes, names, city, country, coordinates, elevation, and type — through a versioned REST API, GraphQL endpoint, and a server-side rendered web UI. GeoIP integration detects the caller's location to surface nearby airports on the home page. All airport data is embedded in the binary at build time; GeoIP databases are downloaded on first run and auto-updated weekly. Deployed as a single self-contained static binary.

## Project variables

project_name: airports
project_org: apimgr
internal_name: airports
internal_org: apimgr
app_name: Airports API
repo: https://github.com/apimgr/airports
license: MIT
binary: airports
client_binary: airports-cli

## Business logic

### Product scope & non-goals

**In scope:**
- Serving 35,000+ airport records via REST API and web UI
- GeoIP-based caller location detection to surface nearby airports
- Search by ICAO/IATA code, name, city, country, or coordinates
- Geographic queries: airports within radius, bounding box, nearest N airports
- Multiple export formats: JSON, CSV, GeoJSON
- Full web frontend (server-side Go templates, dark/light/auto theme, PWA, mobile-first)
- Server pages: `/server/about`, `/server/help`, `/server/healthz`, `/server/privacy`, `/server/terms`
- CLI client (`airports-cli`) for querying from the terminal
- OpenAPI/Swagger docs at `/api/{api_version}/server/swagger`
- GraphQL at `/graphql`

**Non-goals:**
- No user accounts, registration, or login of any kind
- No admin web panel (server configured via `server.yml` only)
- No write/mutation API (airport data is read-only, embedded at build time)
- No live flight tracking or real-time data
- No aviation ATIS, METAR, NOTAMs, or operational data
- No paid tiers, no API keys, no rate-limited access tiers

### Roles & permissions

There are no user roles. All endpoints are public and require no authentication.

| Actor | Access |
|-------|--------|
| **Anonymous visitor (browser)** | Full read access to all web pages and API endpoints |
| **Anonymous API client (curl/CLI)** | Full read access to all API endpoints |
| **Server operator** | Configures server via `server.yml` only; no web management interface |

The server operator uses the CLI (`airports --service`, `airports --maintenance`, `airports --config`) to manage the process. No web-based administration exists.

### Data model & sensitivity

**Airport record** (embedded at build time, no PII):

| Field | Type | Sensitivity |
|-------|------|-------------|
| `id` | string | Public |
| `ident` | string — ICAO code | Public |
| `iata_code` | string | Public |
| `name` | string | Public |
| `type` | string — airport type | Public |
| `latitude_deg` | float | Public |
| `longitude_deg` | float | Public |
| `elevation_ft` | int | Public |
| `continent` | string | Public |
| `iso_country` | string — ISO 3166-1 alpha-2 | Public |
| `iso_region` | string | Public |
| `municipality` | string | Public |
| `scheduled_service` | bool | Public |
| `gps_code` | string | Public |
| `local_code` | string | Public |
| `home_link` | string — URL | Public |
| `wikipedia_link` | string — URL | Public |

No PII is stored or served. GeoIP lookup results for the caller are used only to determine nearby airports and are never logged or stored.

### Trust boundaries & external services

| Boundary | Trust level | Notes |
|----------|-------------|-------|
| Airport dataset (embedded at build) | Fully trusted | Static, embedded at compile time |
| MaxMind GeoLite2 (downloaded at first run) | Trusted — HTTPS + checksum verified | Used for caller GeoIP lookup only |
| Incoming HTTP requests | **Untrusted** | All input validated; GeoIP lookup is best-effort, not security-critical |
| CI/CD build pipeline | Trusted | Produces signed binaries |

Failure mode for GeoIP: if databases are unavailable or lookup fails, the home page shows a generic view without nearby airports. All other functionality is unaffected.

### Threat model & abuse cases

**Primary assets:** the service itself (availability), the embedded airport dataset (integrity).

**Attacker/abuser goals:**
- DoS via high-rate or expensive geographic queries
- Scraping the full dataset in bulk
- Search query injection (path traversal, SQL injection — not applicable since data is in-memory)

**Defenses:**
- Rate limiting on all endpoints (DDoS / abuse protection)
- Request size limits on all inputs
- All user-supplied query parameters are type-validated and bounds-checked before use
- GeoIP lookup result is read-only; attacker cannot influence stored state
- No user accounts eliminates credential stuffing, account takeover, and privilege escalation entirely

**Non-threats (explicitly out of scope):**
- Admin panel compromise — no admin panel exists
- Account enumeration — no accounts exist
- IDOR — no user-scoped resources exist

### Security decisions & exceptions

- **No authentication on any endpoint**: intentional. This is a public read-only reference API. Rate limiting is the sole abuse-prevention mechanism.
- **GeoIP databases downloaded at runtime**: intentional for size reasons (GeoIP databases are ~80 MB and updated weekly; embedding them would double binary size and require weekly releases). Integrity checked via HTTPS.
- **All responses include `Access-Control-Allow-Origin: *`**: intentional. Public data API designed for cross-origin browser use.

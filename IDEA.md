## Project description

Airports is a full-stack Go web application providing comprehensive global airport information. It serves data on 35,000+ airports worldwide — ICAO/IATA codes, names, city, country, coordinates, elevation, and type — through a versioned REST API, GraphQL endpoint, and a server-side rendered web UI. GeoIP integration detects the caller's location to surface nearby airports on the home page. The web UI renders airport locations on OpenStreetMap tiles (no API key required). All airport data is embedded in the binary at build time; GeoIP databases are downloaded on first run and auto-updated weekly. Deployed as a single self-contained static binary.

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
- Map view of airport locations on OpenStreetMap tiles (no API key) for single airport, search results, and radius/bounding-box query results — server-rendered no-JS baseline (coordinates table + static OSM link), progressively enhanced to an interactive pan/zoom map when JavaScript is available

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
| sapics/ip-location-db (MMDB via jsdelivr CDN, downloaded at first run) | Trusted — HTTPS-verified | Used for caller GeoIP lookup only |
| OpenStreetMap tile server (fetched client-side, browser to tile server) | Trusted — public tile CDN, no API key, no server-side proxy | Used only to render map view; server never calls out to it |
| Incoming HTTP requests | **Untrusted** | All input validated; GeoIP lookup is best-effort, not security-critical |
| CI/CD build pipeline | Trusted | Produces signed binaries |

Failure mode for GeoIP: if databases are unavailable or lookup fails, the home page shows a generic view without nearby airports. All other functionality is unaffected.

Map rendering follows progressive enhancement, per the no-JS-required rule: the server always renders a no-JS baseline — a coordinates table plus a static link out to `openstreetmap.org/?mlat=..&mlon=..` — directly in the page HTML. When JavaScript is available, it upgrades that baseline in place to an interactive pan/zoom map (Leaflet/MapLibre GL) using OSM tiles. The interactive map is the only piece that requires JS; it is strictly additive and never gates access to airport data. Failure mode: if the tile server is unreachable, the JS layer falls back to the same no-JS baseline (table + static link) already present in the DOM — no page reload or feature loss. The server itself never calls out to the tile server; tiles load directly in the visitor's browser.

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
- **Map tiles loaded client-side from OpenStreetMap, no API key**: intentional. Keeps zero-config/no-paywall posture; the server never proxies or caches tile requests, so tile-server load and ToS compliance rest on standard OSM attribution/usage-policy adherence in the frontend, not on server-side rate limiting.
- **Container binds privileged port 80 as a non-root user**: intentional. The runtime image runs as the non-root `airports` user (UID/GID 1000). Because the app listens on internal port 80 (<1024), the Dockerfile grants `cap_net_bind_service` on the binary via `setcap` rather than running as root or doing a root-then-drop dance. This is the documented exception to the non-root/no-privileged-port rule.

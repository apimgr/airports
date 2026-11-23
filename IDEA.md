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
owner_token: airports_owner_token_6kP9qV2x
resource_label: airport
resource_label_plural: airports

## Business logic

### Product scope & non-goals

**In scope:**
- Serving 35,000+ airport records via REST API and web UI
- GeoIP-based caller location detection to surface nearby airports
- Search by ICAO/IATA code, name, city, country, or coordinates
- Geographic queries: airports within radius, bounding box, nearest N airports
- Multiple export formats: JSON, CSV, GeoJSON
- Full web frontend (server-side Go templates, dark/light/auto theme, PWA, mobile-first)
- Public server pages for About, Help, Health, Privacy, Preferences, Terms, Contact, DPO, Security, security reporting, and MCP setup; machine-readable mirrors follow AI.md
- Public sitemap and Atom/RSS discovery feeds (empty when the read-only product has no public items)
- CLI client (`airports-cli`) for querying from the terminal
- OpenAPI/Swagger docs at `/api/{api_version}/server/swagger`
- GraphQL POST at `/api/{api_version}/server/graphql`, with the current-version alias `/api/graphql`
- Map view of airport locations on OpenStreetMap tiles (no API key) for single airport, search results, and radius/bounding-box query results — server-rendered no-JS baseline (coordinates table + static OSM link), progressively enhanced to an interactive pan/zoom map when JavaScript is available

**Non-goals:**
- No user accounts, registration, or login of any kind
- No admin web panel (server configured via `server.yml` only)
- No write/mutation API (airport data is read-only, embedded at build time)
- No live flight tracking or real-time data
- No aviation ATIS, METAR, NOTAMs, or operational data
- No paid tiers or user-facing API-key product; the operator, owner, linked-owner, and per-service metrics tokens required by AI.md are platform authentication controls, not access tiers

### Roles & permissions

There are no login accounts or user roles. Public page reads and airport-data reads are anonymous. Anonymous owners use bearer tokens to associate access with their own resources; the owner-token page uses the primary owner token for management actions. MCP requires a linked owner token, metrics require their per-service bearer tokens, and local Tor control endpoints accept loopback callers only. Public preference, contact, consent, and reporting forms accept only their documented submissions.

| Actor | Access |
|-------|--------|
| **Anonymous visitor (browser)** | Read access to public web pages and airport-data endpoints; token management remains credential-gated |
| **Anonymous API client (curl/CLI)** | Read access to public airport data and server information; can submit documented public forms; MCP and metrics require their respective bearer tokens |
| **Owner-token holder** | Manages the anonymous owner's token set and uses linked tokens for MCP; this is a credential-based owner identity, not a login account |
| **Server operator** | Configures server via `server.yml` only; no web management interface |

The server operator uses local configuration files and the CLI (`airports --service`, `airports --maintenance`, `airports --config`) to manage the process. No web-based administration exists.

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

Airport records contain no PII. A caller's IP-derived GeoIP result is sensitive location data: it is returned only to that caller to find nearby airports and is never logged or stored.

### Trust boundaries & external services

| Boundary | Trust level | Notes |
|----------|-------------|-------|
| Airport dataset (embedded at build) | Fully trusted | Static, embedded at compile time |
| sapics/ip-location-db (MMDB GitHub Release assets with published SHA-256 checksums, downloaded at first run) | Trusted — checksum-verified | Used for caller GeoIP lookup only |
| OpenStreetMap tile server (fetched client-side, browser to tile server) | External public service | Used only to render map view; the browser sends its IP and viewed map area directly to the provider; server never calls out to it |
| Incoming HTTP requests | **Untrusted** | All input validated; GeoIP lookup is best-effort, not security-critical |
| CI/CD build pipeline | Trusted | Produces signed binaries |

Failure mode for GeoIP: if databases are unavailable or lookup fails, the home page shows a generic view without nearby airports. All other functionality is unaffected.

Map rendering follows progressive enhancement, per the no-JS-required rule: the server always renders a no-JS baseline — a coordinates table plus a static link out to `openstreetmap.org/?mlat=..&mlon=..` — directly in the page HTML. When JavaScript is available, it upgrades that baseline in place to an interactive pan/zoom map (Leaflet/MapLibre GL) using OSM tiles. The interactive map is the only piece that requires JS; it is strictly additive and never gates access to airport data. Failure mode: if the tile server is unreachable, the JS layer falls back to the same no-JS baseline (table + static link) already present in the DOM — no page reload or feature loss. The server itself never calls out to the tile server; tiles load directly in the visitor's browser.

### Threat model & abuse cases

**Primary assets:** service availability; airport dataset integrity; caller location privacy; owner and operator credentials; security-report confidentiality.

**Attacker/abuser goals:**
- DoS via high-rate or expensive geographic queries
- Scraping the full dataset in bulk
- Search query injection (path traversal, SQL injection — not applicable since data is in-memory)
- Theft or misuse of owner, linked-owner, metrics, or operator tokens
- Exposure of caller location data or confidential security reports

**Defenses:**
- Rate limiting on all endpoints (DDoS / abuse protection)
- Request size limits on all inputs
- All user-supplied query parameters are type-validated and bounds-checked before use
- GeoIP lookup result is read-only; attacker cannot influence stored state
- No login accounts means credential stuffing and account takeover are not applicable; stolen or misused owner tokens and operator credentials remain in scope and follow the authentication controls in AI.md PART 11
- Owner-token and operator access follows AI.md PART 11; MCP requires linked owner tokens, metrics use separate service tokens, and token-management requests are owner-scoped
- Caller GeoIP results are not logged or stored; security-report status links do not expose report bodies

**Non-threats (explicitly out of scope):**
- Admin panel compromise — no admin panel exists
- Account enumeration — no accounts exist
- Airport records are public and have no user-scoped ownership; owner-token management and private security reports remain protected resources

### Security decisions & exceptions

- **No login accounts**: intentional. Public reads remain anonymous; the owner-token system supplies anonymous owner identities, while operator, linked-owner, and per-service metrics credentials follow AI.md.
- **GeoIP databases downloaded at runtime**: intentional for size reasons (GeoIP databases are ~80 MB and updated weekly; embedding them would double binary size and require weekly releases). Downloads are verified against published SHA-256 checksums.
- **Public data API supports cross-origin browser use**: intentional. CORS follows the configured public-origin policy.
- **Map tiles loaded client-side from OpenStreetMap, no API key**: intentional. Keeps zero-config/no-paywall posture; the server never proxies or caches tile requests, so tile-server load and ToS compliance rest on standard OSM attribution/usage-policy adherence in the frontend, not on server-side rate limiting.
- **Container binds privileged port 80 as a non-root user**: intentional. The runtime image runs as the non-root `airports` user (UID/GID 1000). Because the app listens on internal port 80 (<1024), the Dockerfile grants `cap_net_bind_service` on the binary via `setcap` rather than running as root or doing a root-then-drop dance. This is the documented exception to the non-root/no-privileged-port rule.

### API routes

All routes below are public read APIs unless marked otherwise. `{api_version}` is the configured version (default `v1`). API responses use the standard success/error envelope; `.txt` variants and content negotiation provide plain-text output where listed.

- `GET /api/{api_version}/airports`: list airports with pagination and optional result limits; returns airport records and pagination metadata.
- `GET /api/{api_version}/airports/{ident}` and `GET /api/{api_version}/airports/{ident}.txt`: retrieve one airport by ICAO/IATA identifier; returns the matching record or not-found error.
- `GET /api/{api_version}/airports/search[.txt]`: search airports by supported text fields; returns matching records and pagination metadata.
- `GET /api/{api_version}/airports/nearby[.txt]`: find airports near supplied coordinates; returns records with distances and search parameters.
- `GET /api/{api_version}/airports/within`: find airports inside supplied geographic bounds; returns matching records and a count.
- `GET /api/{api_version}/airports/autocomplete`: return airport suggestions for a query.
- `GET /api/{api_version}/airports.json`, `GET /api/{api_version}/airports.csv`, and `GET /api/{api_version}/airports.geojson`: export the airport dataset in the named format.
- `GET /api/{api_version}/countries[.txt]` and `GET /api/{api_version}/states/{country}`: return country airport counts and state/region counts for a country.
- `GET /api/{api_version}/stats[.txt]`: return aggregate airport dataset statistics.
- `GET /api/{api_version}/geoip[.txt]` and `GET /api/{api_version}/geoip/{ip}[.txt]`: look up the caller's or requested IP's approximate location; returns available GeoIP fields.
- `GET /api/{api_version}/geoip/airports/nearby`: return airports near the caller's GeoIP location.
- `GET /api/{api_version}/settings`: return the documented safe public settings subset; credentials and internal topology are excluded.
- `POST /api/{api_version}/server/graphql` and `POST /api/graphql`: query the read-only airport dataset through GraphQL; returns query results or GraphQL errors. The unversioned path aliases the current version.

Airport data and GraphQL queries do not require authentication. The platform's owner-token, metrics-token, MCP, server-information, and other baseline routes follow the route and authentication contracts in the applicable AI.md PARTs.

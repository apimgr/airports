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
owner_token: airports_owner_token_qwhzkfta
owner_token_legacy: airports_owner_token_6kP9qV2x
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
- GraphQL POST and read-only GET at `/api/{api_version}/server/graphql`, with current-version POST and GET aliases at `/api/graphql`
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

### Security and privacy requirements

- There are no login accounts. Public airport data remains anonymously accessible; platform operator, owner, linked-owner, and per-service metrics credentials are security controls, not product tiers.
- Caller GeoIP results are used only to find nearby airports, are returned only to that caller, and are never logged or stored. GeoIP failure leaves the rest of the product available and shows a generic location view.
- Public airport data is available to browser clients on other origins under the configured public-origin policy.
- Map tiles come from OpenStreetMap without an API key. The browser contacts the tile provider directly, and the product identifies the provider and its privacy implications to visitors.

### Data access capabilities

Airport data and GraphQL queries do not require authentication. The product provides:

- Browsing and pagination across airport records, plus lookup by ICAO/IATA identifier.
- Search and autocomplete across supported airport fields.
- Geographic queries for nearby airports, radius and bounding-box results, and caller-location-based nearby results when GeoIP is available.
- JSON, CSV, and GeoJSON exports, plus country/region counts and aggregate dataset statistics.
- A read-only GraphQL interface and OpenAPI documentation.
- A safe public settings view that excludes credentials and internal topology.

The platform's owner-token, metrics-token, MCP, server-information, and other baseline interfaces follow the contracts in the applicable AI.md PARTs. Their paths and transport details are defined by AI.md, not duplicated here.

### API routes

All airport-data routes below are public and read-only. The server's baseline routes are defined by AI.md PARTS 13–14 and are not repeated here.

- `GET /api/{api_version}/airports`: list airports with pagination and optional result limits; returns airport records and pagination metadata.
- `GET /api/{api_version}/airports/{ident}` and `GET /api/{api_version}/airports/{ident}.txt`: retrieve one airport by ICAO/IATA identifier.
- `GET /api/{api_version}/airports/search[.txt]`: search airports by supported text fields with pagination.
- `GET /api/{api_version}/airports/nearby[.txt]`: find airports near supplied coordinates, including distances and search parameters.
- `GET /api/{api_version}/airports/within`: find airports inside supplied geographic bounds.
- `GET /api/{api_version}/airports/autocomplete`: return airport suggestions for a query.
- `GET /api/{api_version}/airports.json`, `GET /api/{api_version}/airports.csv`, and `GET /api/{api_version}/airports.geojson`: export the airport dataset in the named format.
- `GET /api/{api_version}/countries[.txt]` and `GET /api/{api_version}/states/{country}`: return country airport counts and state/region counts for a country.
- `GET /api/{api_version}/stats[.txt]`: return aggregate airport dataset statistics.
- `GET /api/{api_version}/geoip[.txt]` and `GET /api/{api_version}/geoip/{ip}[.txt]`: look up the caller's or requested IP's approximate location and return the available GeoIP fields.
- `GET /api/{api_version}/geoip/airports/nearby`: return airports near the caller's GeoIP location when available.
- `GET /api/{api_version}/settings`: return the documented safe public settings subset; credentials and internal topology are excluded.

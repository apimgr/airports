# ✈️ Airports API

Global airport information API — 48,069 small, medium, and large airports worldwide, single static Go binary with embedded data.

[![CI](https://github.com/apimgr/airports/actions/workflows/ci.yml/badge.svg)](https://github.com/apimgr/airports/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/apimgr/airports)](https://github.com/apimgr/airports/releases)
[![License](https://img.shields.io/github/license/apimgr/airports)](LICENSE.md)
[![Docs](https://readthedocs.org/projects/apimgr-airports/badge/?version=latest)](https://apimgr-airports.readthedocs.io)
[![Docker](https://img.shields.io/badge/Docker-ghcr.io-2496ED?logo=docker)](https://github.com/apimgr/airports/pkgs/container/airports)

## About

Airports is a full-stack Go web application providing comprehensive global airport information. It serves data on 48,069 small, medium, and large airports — ICAO/IATA codes, names, city, country, coordinates, elevation, and type — through a versioned REST API, GraphQL endpoint, and a server-side rendered web UI. GeoIP integration detects the caller's location to surface nearby airports on the home page. All airport data is embedded in the binary at build time; GeoIP databases are downloaded on first run and auto-updated weekly. The application ships as a self-contained static binary; Tor and I2P integrations use their respective local daemons when available.

**No login accounts or admin panel. Public airport-data and server-information reads are unauthenticated. Owner-token management uses the primary owner token; MCP uses linked owner tokens; metrics use per-service bearer tokens. The airport dataset remains read-only.**

## Features

- **48,069 airports** with ICAO/IATA codes, names, city, country, coordinates, elevation, and type
- **GeoIP integration** — nearby airports surfaced on the home page based on caller location
- **Fast search** — by ICAO/IATA code, name, city, country, or coordinates
- **Geographic queries** — airports within radius, bounding box, or nearest N
- **Multiple formats** — JSON, CSV, GeoJSON via `Accept` header or `?format=` parameter
- **REST API** with OpenAPI/Swagger documentation
- **GraphQL** endpoint for flexible queries
- **Web UI** — dark/light/auto theme, mobile-first, PWA-capable
- **Localized UI and CLI** — eight languages; see the [translation guide](docs/translating.md)
- **CLI client** — `airports-cli` for terminal queries
- **Self-contained** — single static binary with embedded assets and dataset; Tor and I2P integrations use their local daemons when available
- **Auto-updates** — weekly GeoIP database refresh via built-in scheduler
- **Multi-platform** — Linux, macOS, Windows, FreeBSD (amd64 & arm64)
- **Tor hidden service** — auto-detected .onion service when Tor is available
- **I2P eepsite** — optional overlay service with container-aware defaults
- **MCP server** — remote Streamable HTTP tools for AI clients using linked tokens
- **OpenStreetMap maps** — interactive airport, search, and nearby maps with a no-JavaScript coordinate/link fallback

---

Map tile behavior, attribution, and privacy details are documented in [Airport maps](docs/maps.md).

## Production Deployment

### Docker Compose (Recommended)

```bash
# Download production compose file
curl -q -LSsf -O https://raw.githubusercontent.com/apimgr/airports/main/docker/docker-compose.yml

# Start
docker compose up -d

# View logs
docker compose logs -f

# Access API
curl -q -LSsf http://172.17.0.1:64580/api/{api_version}/airports/KJFK
```

The server listens on `172.17.0.1:64580` (Docker bridge only). Place a reverse proxy (nginx, Caddy, Traefik) in front for public access.

**On first start**, the server:
1. Creates config and data directories inside the mounted volumes
2. Downloads GeoIP databases (~80 MB)
3. Starts serving on port 80 internally

### Binary (Systemd)

```bash
# Download for Linux amd64
curl -q -LSsf -o airports https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
chmod +x airports
sudo mv airports /usr/local/bin/airports

# Install and start systemd service
sudo airports --service --install
sudo airports --service start

# Check status
sudo airports --status
```

**Root run** uses `/etc/apimgr/airports`, `/var/lib/apimgr/airports`, `/var/log/apimgr/airports`.
**User run** uses `~/.config/apimgr/airports`, `~/.local/share/apimgr/airports`, and `~/.local/state/apimgr/airports/log`.
If system-service escalation is declined or unavailable, Linux systemd and macOS install a per-user service, and Windows creates a Task Scheduler logon task. Linux with another init system and BSD print user-mode instructions. See [installation](docs/installation.md) for details.

---

## Client (`airports-cli`)

```bash
# Download for Linux amd64
curl -q -LSsf -o airports-cli https://github.com/apimgr/airports/releases/latest/download/airports-cli-linux-amd64
chmod +x airports-cli
sudo mv airports-cli /usr/local/bin/airports-cli

# First run in a terminal: configure the server URL and open the interactive client
airports-cli

# Examples
airports-cli get KJFK
airports-cli search "New York"
airports-cli JFK
airports-cli --limit 10 "New York"
airports-cli nearby 40.63 -73.77 10
airports-cli --server https://YOUR_SERVER health
```

With no command, the client opens a TUI in a terminal or a native GUI in a supported local desktop session when built with `-tags gui`. The TUI includes a general API explorer. Command output defaults to JSON; use `--format text` for plain text. Color output respects `NO_COLOR`.

The native GUI labels its controls and supports keyboard focus, modal error dialogs, and Escape dismissal. The current `gogpu/ui` version does not provide platform screen-reader adapters (UIA, AT-SPI2, or NSAccessibility); use the TUI with `AIRPORTS_SCREEN_READER=1` or the CLI for screen-reader access.

---

## Configuration

Settings use CLI flags and supported environment overrides where documented;
otherwise they come from `server.yml` and the relevant `yaml.d/` drop-ins.
Secrets and generated tokens are stored separately in `secrets.yml`.

| Setting | Env Var | Default | Description |
|---------|---------|---------|-------------|
| `--port` | `PORT` | `80` (container) / auto (non-container) | Listen port |
| `--address` | `LISTEN` | `0.0.0.0` | Bind address |
| `--mode` | `MODE` | `production` | `development` or `production` |
| `--debug` | `DEBUG` | `false` | Enable operator-token-protected diagnostics under `/debug/*` |
| `--config` | — | auto-detected | Config directory containing `server.yml` |

Config file is auto-created at first run with sane defaults. No manual setup required.

Terminal color priority is `--color=yes|no`, then the applicable config setting, then nonempty
`NO_COLOR`, then TTY/`TERM` detection. Server `server.yml` accepts optional boolean `output.color`
and `output.emoji`; `output.emoji: true` keeps emojis enabled when `NO_COLOR` is set. Client `cli.yml`
uses `output.color: auto|yes|no` (boolean values are also accepted as `yes`/`no`). A legacy root-level
client `color` value is read as a fallback and written under `output.color` the next time the client
saves its config.

---

## API

Base URL: `http://YOUR_SERVER/api/{api_version}` (`{api_version}` defaults to `v1` and follows `server.api_version` in configuration).

### Airport endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/{api_version}/airports` | List airports (paginated) |
| `GET /api/{api_version}/airports/{ident}` | Airport by ICAO/IATA code |
| `GET /api/{api_version}/airports/search?q=...` | Search by name, city, country |
| `GET /api/{api_version}/airports/nearby?lat=...&lon=...&radius=...` | Airports within radius (miles by default; pass `units=metric` for kilometers) |
| `GET /api/{api_version}/airports/within?min_lat=...&max_lat=...&min_lon=...&max_lon=...` | Airports in bounding box |
| `GET /api/{api_version}/airports/autocomplete?q=...` | Airport suggestions for a partial query |
| `GET /api/{api_version}/airports.json`, `/airports.csv`, `/airports.geojson` | Full dataset exports |
| `GET /api/{api_version}/countries`, `/states/{country}` | Airport counts by country and region |
| `GET /api/{api_version}/stats` | Dataset statistics |
| `GET /api/{api_version}/geoip`, `/geoip/{ip}` | Approximate location for the caller or a supplied IP |

### Public web pages

| Page | Description |
|------|-------------|
| `GET /airports` | Server-rendered, paginated airport directory |
| `GET /airports/{ident}` | Airport detail page |
| `GET /airports/search?q=...` | Airport search page |
| `GET /airports/nearby` | Nearby and bounding-box results |
| `GET /stats` | Airport dataset statistics |
| `GET /geoip` | Caller location and nearby airports when available |

### Server endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /server/healthz` | Health status (HTML for browsers, human-readable text for CLI clients) |
| `GET /api/{api_version}/server/healthz` | Health status (JSON) |
| `GET /api/{api_version}/server/auth/check` | Validate an operator or owner bearer token without returning it |
| `GET /server/about` | Project description and version info (HTML) |
| `GET /api/{api_version}/server/about` | Project description (JSON) |
| `GET /server/help` | Usage and API documentation links (HTML) |
| `GET /api/{api_version}/server/help` | Help content and registered help pages (JSON) |

API health `.txt` routes and `Accept: text/plain` responses use machine-readable dot notation with compact values. Frontend health text is human-readable.
| `GET /server/help/mcp` | MCP setup instructions and live tools (HTML) |
| `GET /server/privacy` | Privacy policy (HTML) |
| `GET /api/{api_version}/server/privacy` | Privacy policy data (JSON) |
| `GET /server/terms` | Terms of service (HTML) |
| `GET /api/{api_version}/server/terms` | Terms of service (JSON) |
| `GET /server/dpo` | Data Protection Officer contact (HTML) |
| `GET /api/{api_version}/server/dpo` | DPO contact information (JSON) |
| `GET /server/security` | Security reporting information (HTML) |
| `GET /api/{api_version}/server/security` | Security disclosure channels and key status (JSON) |
| `GET /server/security/policy` | Coordinated disclosure policy (HTML) |
| `GET /api/{api_version}/server/security/policy` | Coordinated disclosure policy (JSON) |
| `GET /server/security/thanks` | Security acknowledgments (HTML) |
| `GET /api/{api_version}/server/security/thanks` | Security acknowledgments (JSON) |
| `GET /server/preferences` | Language and theme preferences |
| `GET /server/preferences/sync` | Import or export shareable language and theme preferences |
| `GET /server/preferences/tokens` | View and manage owner and linked tokens with the owner cookie; without it, only token issue and restore forms appear |
| `GET /server/contact` | Public abuse, DMCA, privacy, or security report form |
| `GET /server/security/report/{tracking_id}` | One-time emailed security report status; report contents are not shown |
| `GET /api/{api_version}/server/contact` | Contact form descriptors and challenge (JSON) |
| `POST /api/{api_version}/server/contact` | Submit a validated public report |
| `POST /server/consent` | Save cookie consent choices |
| `POST /server/ccpa` | Save or clear the CCPA opt-out choice |
| `GET /sitemap.xml` | Search-engine sitemap for allowed public pages |
| `GET /feed.xml`, `/rss.xml` | Atom and RSS feeds (currently empty; no user-created content) |
| `GET /server/docs/swagger` | Swagger UI |
| `GET /api/{api_version}/server/swagger` | OpenAPI spec |
| `POST /api/{api_version}/server/mcp` | Bearer-authenticated airport tools over Model Context Protocol Streamable HTTP |
| `/api/{api_version}/server/preferences/tokens` | Primary and linked owner-token management (primary Bearer required except issuance) |
| `GET /server/help/mcp` | MCP setup and live tool reference |
| `GET /api/{api_version}/server/help/mcp` | MCP setup metadata and live tools (JSON) |
| `GET /debug/*` | Operator-token-protected diagnostics when debug mode is enabled; internal diagnostics outside the API surface |
| `GET /server/docs/graphql` | GraphiQL UI |
| `POST /api/{api_version}/server/graphql` | GraphQL endpoint |
| `GET /api/{api_version}/server/graphql` | Read-only GraphQL query using URL parameters |
| `POST /api/graphql` | Direct alias for the current-version GraphQL endpoint |
| `GET /api/graphql` | Read-only GraphQL query alias |
| `GET /server/metrics/{service}` | Prometheus, Grafana, or Loki metrics (per-service bearer token required) |
| `GET /api/metrics/{service}` | Metrics alias (per-service bearer token required) |
| `GET /api/{api_version}/server/metrics/{service}` | Versioned metrics endpoint (per-service bearer token required) |

The health JSON keeps I2P's normalized `status` separate from its provider
`state` (`disabled`, `no_provider`, `starting`, `building`, `running`, or
`error`). With no provider, `status` is `disabled` and `state` is
`no_provider`; the I2P check is omitted when the network is inactive.

**Response format:** JSON by default. Use `Accept: text/csv`, `Accept: application/geo+json`, or `?format=csv|geojson` for alternate formats.

**CORS:** API origins follow the server's resolved CORS allow-list. Credentialed
browser requests require an explicitly allowed origin.

MCP clients can use the canonical `/api/{api_version}/server/mcp` endpoint or
the direct `/api/mcp` and `/mcp` aliases. Every MCP request requires an
`Authorization: Bearer` linked owner token created for that client. Never use
the primary owner token or operator token (`server.token`). See
`/server/help/mcp` for client configuration, protocol requirements, and
available tools.

Owner token management is available at `/server/preferences/tokens`; its
versioned API mirror supports primary and linked token management. Token
values are returned only by issuance, reveal, and rotation actions. Browser
reveals display one token for one page load; reloading masks it again. The API
mirror returns issued values in its JSON response.

### Quick examples

```bash
# Health check
curl -q -LSsf http://YOUR_SERVER/api/{api_version}/server/healthz

# Airport by ICAO code
curl -q -LSsf http://YOUR_SERVER/api/{api_version}/airports/KJFK

# Search
curl -q -LSsf "http://YOUR_SERVER/api/{api_version}/airports/search?q=Heathrow"

# Nearby airports (50 km radius)
curl -q -LSsf "http://YOUR_SERVER/api/{api_version}/airports/nearby?lat=40.63&lon=-73.77&radius=50&units=metric"

# Export as GeoJSON
curl -q -LSsf -H "Accept: application/geo+json" http://YOUR_SERVER/api/{api_version}/airports
```

---

## Data Sources

- **Airport data:** [OurAirports](https://ourairports.com/) — public domain. Snapshot commit `a9426f7711df70678dfb745ceaa3d8891bf98328` of [ourairports-data](https://github.com/davidmegginson/ourairports-data), retrieved 2026-11-27; only small, medium, and large airports are included.
- **Timezone mapping:** [mwgg/Airports](https://github.com/mwgg/Airports) — MIT, commit `a89e4e0dda9d9878337e2da190ebe1ef1d6af705`, retrieved 2026-11-27. The optional `tz` field is populated for 24,581 records and omitted when no ident match exists. Its README credits TimeZoneDB (CC BY 3.0) and TimeAPI; see [LICENSE.md](LICENSE.md).
- The exact source hashes and transformation details are recorded in [`src/data/airports.provenance.json`](src/data/airports.provenance.json).
- **GeoIP databases:** [sapics/ip-location-db](https://github.com/sapics/ip-location-db) — ASN/country data under PDDL and DB-IP city data under CC BY 4.0; downloaded on first run and updated weekly

---

## Development

**Requirements:** Docker and Make. No Go toolchain is needed on the local machine; all builds run in Docker.

```bash
git clone https://github.com/apimgr/airports.git
cd airports

make help      # List the supported targets
make build     # Build debug binaries for all supported platforms
make test      # Run quality gates, unit tests, and the Docker basic tier
make release   # Build production binaries, source archive, and checksums locally
make docker    # Build and check the host-platform image locally; never push
make clean     # Remove build artifacts
```

Run integration tests (requires Docker):

```bash
./tests/run_tests.sh
```

---

## Disclaimer

Airport data is provided for informational purposes only. Do not use for navigation, flight planning, or any safety-critical application. Data may be incomplete or out of date. Always consult official aviation sources (FAA, ICAO, EASA) for operational use.

---

## License

MIT — see [LICENSE.md](LICENSE.md)

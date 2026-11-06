# ✈️ Airports API

Global airport information API — 35,000+ airports worldwide, single static Go binary with embedded data.

[![CI](https://github.com/apimgr/airports/actions/workflows/ci.yml/badge.svg)](https://github.com/apimgr/airports/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/apimgr/airports)](https://github.com/apimgr/airports/releases)
[![License](https://img.shields.io/github/license/apimgr/airports)](LICENSE.md)
[![Docker](https://img.shields.io/badge/Docker-ghcr.io-2496ED?logo=docker)](https://github.com/apimgr/airports/pkgs/container/airports)

## About

Airports is a full-stack Go web application providing comprehensive global airport information. It serves data on 35,000+ airports — ICAO/IATA codes, names, city, country, coordinates, elevation, and type — through a versioned REST API, GraphQL endpoint, and a server-side rendered web UI. GeoIP integration detects the caller's location to surface nearby airports on the home page. All airport data is embedded in the binary at build time; GeoIP databases are downloaded on first run and auto-updated weekly. Deployed as a single self-contained static binary with zero runtime dependencies.

**No user accounts. No admin panel. No authentication. All endpoints are public and read-only.**

## Official Site

<https://github.com/apimgr/airports>

## Features

- **35,000+ airports** with ICAO/IATA codes, names, city, country, coordinates, elevation, and type
- **GeoIP integration** — nearby airports surfaced on the home page based on caller location
- **Fast search** — by ICAO/IATA code, name, city, country, or coordinates
- **Geographic queries** — airports within radius, bounding box, or nearest N
- **Multiple formats** — JSON, CSV, GeoJSON via `Accept` header or `?format=` parameter
- **REST API** with OpenAPI/Swagger documentation
- **GraphQL** endpoint for flexible queries
- **Web UI** — dark/light/auto theme, mobile-first, PWA-capable
- **CLI client** — `airports-cli` for terminal queries
- **Self-contained** — single static binary, all assets embedded, no runtime dependencies
- **Auto-updates** — weekly GeoIP database refresh via built-in scheduler
- **Multi-platform** — Linux, macOS, Windows, FreeBSD (amd64 & arm64)
- **Tor hidden service** — optional .onion address
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
airports-cli nearby 40.63 -73.77 10
airports-cli --server https://YOUR_SERVER health
```

With no command, the client opens a TUI in a terminal or a native GUI in a supported local desktop session when built with `-tags gui`. The TUI includes a general API explorer. Command output defaults to JSON; use `--format text` for plain text. Color output respects `NO_COLOR`.

---

## Configuration

All settings can be expressed as CLI flags, environment variables, or `server.yml` entries. CLI flags take highest precedence.

| Setting | Env Var | Default | Description |
|---------|---------|---------|-------------|
| `--port` | `PORT` | `80` (container) / auto (host) | Listen port |
| `--address` | `LISTEN` | `0.0.0.0` | Bind address |
| `--mode` | `MODE` | `production` | `development` or `production` |
| `--debug` | `DEBUG` | `false` | Enable pprof/expvar endpoints |
| `--config` | — | auto-detected | Path to `server.yml` |

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
| `GET /api/{api_version}/airports/nearby?lat=...&lon=...&radius=...` | Airports within radius (km) |
| `GET /api/{api_version}/airports/within?min_lat=...&max_lat=...&min_lon=...&max_lon=...` | Airports in bounding box |

### Server endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /server/healthz` | Health status (HTML) |
| `GET /api/{api_version}/server/healthz` | Health status (JSON) |
| `GET /server/about` | Version and build info (HTML) |
| `GET /api/{api_version}/server/about` | Version and build info (JSON) |
| `GET /server/docs/swagger` | Swagger UI |
| `GET /api/{api_version}/server/swagger` | OpenAPI spec |
| `GET /server/docs/graphql` | GraphiQL UI |
| `POST /api/{api_version}/server/graphql` | GraphQL endpoint |
| `GET /metrics` | Prometheus metrics |

**Response format:** JSON by default. Use `Accept: text/csv`, `Accept: application/geo+json`, or `?format=csv|geojson` for alternate formats.

**CORS:** `Access-Control-Allow-Origin: *` on all API responses (public read-only data).

### Quick examples

```bash
# Health check
curl -q -LSsf http://YOUR_SERVER/api/{api_version}/server/healthz

# Airport by ICAO code
curl -q -LSsf http://YOUR_SERVER/api/{api_version}/airports/KJFK

# Search
curl -q -LSsf "http://YOUR_SERVER/api/{api_version}/airports/search?q=Heathrow"

# Nearby airports (50 km radius)
curl -q -LSsf "http://YOUR_SERVER/api/{api_version}/airports/nearby?lat=40.63&lon=-73.77&radius=50"

# Export as GeoJSON
curl -q -LSsf -H "Accept: application/geo+json" http://YOUR_SERVER/api/{api_version}/airports
```

---

## Data Sources

- **Airport data:** [OurAirports](https://ourairports.com/) — public domain, embedded at build time
- **GeoIP databases:** [sapics/ip-location-db](https://github.com/sapics/ip-location-db) — CC0/PDDL, downloaded on first run, updated weekly

---

## Development

**Requirements:** Docker and Make. No Go toolchain needed on the host — all builds run in Docker.

```bash
git clone https://github.com/apimgr/airports.git
cd airports

make dev       # Quick build to temp dir
make test      # Run unit tests in Docker
make local     # Build for host platform -> binaries/
make build     # Cross-compile all platforms -> binaries/
make docker    # Build and push multi-arch Docker image
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

# Airports API Server - Project Specification

**Project**: airports
**Module**: github.com/apimgr/airports
**Language**: Go 1.24+
**Purpose**: Global airport location API with GeoIP integration
**Data**: 35,000+ airports worldwide (embedded)
**Organization**: apimgr
**Registry**: ghcr.io/apimgr/airports

---

## Compliance

This project follows **BASE.md** specification from the apimgr organization.

### Key Compliance Points

| Requirement | Status | Implementation |
|-------------|--------|----------------|
| Go-based single binary | ✅ | CGO_ENABLED=0, embedded assets |
| File-based YAML config | ✅ | /etc/apimgr/airports/server.yaml |
| No authentication | ✅ | All endpoints public |
| No database | ✅ | File-based config only |
| Tini init in Docker | ✅ | /sbin/tini as entrypoint |
| CLI commands | ✅ | --service, --maintenance |
| REST API + Swagger + GraphQL | ✅ | /api/v1/, /openapi, /graphql |
| .txt extension support | ✅ | /api/v1/stats.txt, etc. |
| PWA support | ✅ | manifest.json, service worker |
| robots.txt/security.txt | ✅ | Config-based generation |
| Multi-platform | ✅ | Linux, macOS, Windows, BSD (amd64/arm64) |

---

## Directory Structure

```
airports/
├── SPEC.md                    # This file
├── README.md                  # User documentation
├── Dockerfile                 # Alpine + tini
├── docker-compose.yml         # Production deployment
├── Makefile                   # Build system
├── go.mod / go.sum            # Go modules
├── release.txt                # Version tracking
├── scripts/                   # Installation scripts
│   ├── linux.sh
│   ├── macos.sh
│   └── windows.ps1
├── docs/                      # MkDocs documentation
└── src/                       # Source code
    ├── main.go                # Entry point
    ├── data/
    │   └── airports.json      # Embedded airport data (8.7MB)
    ├── airports/              # Airport service
    │   └── data.go
    ├── config/                # YAML configuration
    │   └── config.go
    ├── geoip/                 # GeoIP service
    │   └── service.go
    ├── paths/                 # OS-specific paths
    │   └── paths.go
    ├── scheduler/             # Task scheduler
    │   └── scheduler.go
    └── server/                # HTTP server
        ├── server.go
        ├── handlers.go
        ├── web_handlers.go
        ├── docs_handlers.go
        ├── templates/         # Embedded HTML
        └── static/            # Embedded CSS/JS
```

---

## Configuration

**Location**: `/etc/apimgr/airports/server.yaml` (root) or `~/.config/apimgr/airports/server.yaml` (user)

```yaml
server:
  port: "64555"
  fqdn: ""
  address: "0.0.0.0"
  schedule:
    enabled: true
    geoip_update: "weekly"
  ssl:
    enabled: false
  geoip:
    enabled: true
  metrics:
    enabled: false
    endpoint: "/metrics"

web-ui:
  theme: "dark"
  notifications:
    enabled: true

web-robots:
  allow: ["/", "/api"]
  deny: ["/debug"]

web-security:
  admin: "security@example.com"
  cors: "*"
```

---

## CLI Commands

```bash
# Basic
airports --help                  # Show help
airports --version               # Show version
airports --status                # Check if running

# Server
airports                         # Start with defaults
airports --port 8080             # Start on specific port
airports --config /path/to/dir   # Custom config directory

# Service management
airports --service start         # Start service
airports --service stop          # Stop service
airports --service restart       # Restart service
airports --service reload        # Reload configuration
airports --service status        # Service status
airports --service --install     # Install as service
airports --service --uninstall   # Remove service

# Maintenance
airports --maintenance backup [file]    # Backup config/data
airports --maintenance restore [file]   # Restore from backup
airports --maintenance update           # Check and install updates
```

---

## API Endpoints

### Public Endpoints (All routes - NO AUTH)

```
# Web Pages
GET  /                           Home page
GET  /search                     Search page
GET  /nearby                     Nearby search page
GET  /airport/{code}             Airport detail page
GET  /stats                      Statistics page
GET  /geoip                      GeoIP lookup page
GET  /healthz                    Health check

# Documentation
GET  /openapi                    Swagger UI
GET  /graphql                    GraphQL Playground

# Special Files
GET  /robots.txt                 Robots file (from config)
GET  /security.txt               Security contact
GET  /manifest.json              PWA manifest
GET  /sw.js                      Service worker

# API v1 - JSON responses
GET  /api/v1/                    API information
GET  /api/v1/airports            List airports (paginated)
GET  /api/v1/airport/{code}      Get airport by ICAO/IATA
GET  /api/v1/search?q=query      Search airports
GET  /api/v1/nearby?lat=&lon=    Find nearby airports
GET  /api/v1/countries           List countries
GET  /api/v1/stats               Database statistics
GET  /api/v1/geoip               Lookup request IP
GET  /api/v1/geoip/{ip}          Lookup specific IP
GET  /api/v1/health              Health check

# API v1 - Text responses (.txt extension)
GET  /api/v1/airport/{code}.txt  Airport as text
GET  /api/v1/search.txt          Search results as text
GET  /api/v1/nearby.txt          Nearby airports as text
GET  /api/v1/countries.txt       Countries as text
GET  /api/v1/stats.txt           Statistics as text
GET  /api/v1/geoip.txt           GeoIP as text
GET  /api/v1/health.txt          Health as text

# Export formats
GET  /api/v1/airports.json       Full database JSON
GET  /api/v1/airports.csv        Full database CSV
GET  /api/v1/airports.geojson    Full database GeoJSON

# Documentation
GET  /api/v1/openapi             Swagger UI
GET  /api/v1/openapi.json        OpenAPI spec
GET  /api/v1/graphql             GraphQL (GET=playground, POST=query)
```

---

## Data Sources

### airports.json (Embedded)
- **Location**: src/data/airports.json
- **Size**: ~8.7MB
- **Records**: 35,000+ airports
- **Source**: OurAirports (Public Domain)

### GeoIP Databases (External)
- **Source**: sapics/ip-location-db (via jsdelivr CDN)
- **Location**: {CONFIG_DIR}/geoip/
- **Update**: Weekly (configurable)
- **Databases**:
  - geolite2-city-ipv4.mmdb
  - geolite2-city-ipv6.mmdb
  - geo-whois-asn-country.mmdb
  - asn.mmdb

---

## Docker

```bash
# Production
docker run -d \
  --name airports \
  -p 8080:80 \
  -v ./config:/config \
  -v ./data:/data \
  ghcr.io/apimgr/airports:latest

# Docker Compose
docker-compose up -d
```

**Image Details**:
- Base: Alpine + tini
- Size: ~25MB
- Port: 80 (internal)
- User: nobody (65534)
- Entrypoint: /sbin/tini --
- Healthcheck: airports --status

---

## Build

```bash
# All platforms
make build

# Test
make test

# Docker image
make docker-dev

# Release
make release
```

**Output Binaries**:
- airports-linux-amd64
- airports-linux-arm64
- airports-macos-amd64
- airports-macos-arm64
- airports-windows-amd64.exe
- airports-windows-arm64.exe
- airports-bsd-amd64
- airports-bsd-arm64

---

## License

MIT License

**Data Licenses**:
- Airport data: Public Domain (OurAirports)
- GeoIP: CC BY-SA 4.0 (MaxMind GeoLite2)

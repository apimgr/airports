# ✈️ Airports API Server - Project Specification

**Project**: airports
**Module**: github.com/apimgr/airports
**Language**: Go (latest via golang:alpine)
**Purpose**: Global airport location API with GeoIP integration
**Data**: 35,000+ airports worldwide (embedded), GeoIP databases (sapics/ip-location-db)
**Organization**: apimgr
**Registry**: ghcr.io/apimgr/airports

---

## ⚠️ CRITICAL: Follow the SPEC

**This project follows**: `/root/Projects/github/apimgr/SPEC.md`

**Before making ANY changes:**
1. ✅ READ `/root/Projects/github/apimgr/SPEC.md` completely
2. ✅ FOLLOW all standards defined in SPEC.md
3. ✅ Use Docker for builds (`CGO_ENABLED=0`)
4. ✅ Use Incus/Docker for testing (NEVER host)
5. ✅ Reference SPEC sections when implementing features

**Key Standards:**
- Static binary: `CGO_ENABLED=0` (ALWAYS)
- Build with: Docker (`make build`)
- Test with: Incus → Docker (NEVER host)
- Database: `/data/db/airports.db`
- Logs: Apache format (access.log), JSON (others)
- Admin config: WebUI only (NOT CLI flags)
- CORS default: `*` (configurable via admin)
- Ports: Random 64000-64999 for testing

---

## 🎯 Project Overview

A **production-ready airport location API** with GeoIP integration, built as a single self-contained Go binary.

### Key Features

- **35,000+ Airports**: Complete global airport database with ICAO/IATA codes
- **GeoIP Integration**: sapics/ip-location-db (4 databases, daily updates)
- **Fast Search**: In-memory indexes for instant lookups
- **Geographic Queries**: Nearby search, bounding box, distance calculations
- **Multiple Formats**: JSON, CSV, GeoJSON exports
- **RESTful API**: Clean endpoints with Swagger/OpenAPI
- **GraphQL**: Interactive query interface
- **Web Interface**: Dark theme, responsive design
- **Admin Dashboard**: Live configuration reload
- **Single Binary**: ~28MB with all assets embedded
- **Multi-Platform**: Linux, macOS, Windows, BSD (amd64 + arm64)

### Technology Stack

- **Language**: Go (latest)
- **HTTP Router**: Chi v5
- **Database**: SQLite (modernc.org/sqlite - pure Go, no CGO)
- **Templates**: Go html/template
- **GeoIP**: oschwald/geoip2-golang + sapics databases
- **Embedding**: Go embed.FS
- **Authentication**: SHA-256 hashing, Bearer tokens, Basic Auth
- **Rate Limiting**: github.com/go-chi/httprate
- **Scheduler**: Built-in (no external cron)

---

## 📁 Directory Structure

**Follows SPEC Section 11 (Complete Project Layout)**

```
airports/
├── .github/workflows/
│   ├── release.yml              # Binary releases
│   └── docker.yml               # Docker images
├── .claude/settings.local.json
├── .readthedocs.yml
├── CLAUDE.md                    # This file
├── Dockerfile                   # golang:alpine + alpine:latest
├── docker-compose.yml           # Production (172.17.0.1:64180:80)
├── docker-compose.test.yml      # Development (64181:80)
├── Jenkinsfile                  # jenkins.casjay.cc
├── LICENSE.md                   # MIT
├── Makefile                     # build, test, release, docker
├── README.md                    # User documentation
├── release.txt                  # Version (0.0.1)
├── binaries/                    # Build output (gitignored)
├── releases/                    # GitHub releases (gitignored)
├── rootfs/                      # Docker volumes (gitignored)
├── docs/                        # ReadTheDocs (MkDocs + Dracula)
├── scripts/                     # Install scripts (all platforms)
└── src/                         # Source code
    ├── main.go                  # Entry point, embeds data/airports.json
    ├── data/
    │   └── airports.json        # 8.7MB, JSON ONLY (no .go files)
    ├── airports/
    │   ├── data.go              # NewService(jsonData []byte)
    │   └── data_test.go
    ├── geoip/
    │   ├── service.go           # sapics/ip-location-db integration
    │   └── service_test.go
    ├── database/
    │   ├── database.go
    │   ├── auth.go
    │   ├── credentials.go
    │   └── settings.go
    ├── paths/
    │   └── paths.go             # OS-specific directory detection
    ├── scheduler/
    │   └── scheduler.go         # Built-in task scheduler
    └── server/
        ├── server.go
        ├── handlers.go
        ├── docs_handlers.go     # Swagger/GraphQL
        ├── admin_handlers.go
        ├── auth_middleware.go
        ├── templates.go
        ├── static/              # Embedded (CSS, JS, images)
        └── templates/           # Embedded (HTML)
```

---

## 💾 Data Sources

### airports.json

**Location**: `src/data/airports.json` (JSON ONLY directory)
**Size**: 8.7MB
**Records**: 35,000+ worldwide airports
**Embedded**: Yes (via `//go:embed data/airports.json` in main.go)
**Format**: JSON object with ICAO keys

**Structure**:
```json
{
  "KJFK": {
    "icao": "KJFK",
    "iata": "JFK",
    "name": "John F Kennedy International Airport",
    "city": "New York",
    "state": "New York",
    "country": "US",
    "elevation": 13,
    "lat": 40.63980103,
    "lon": -73.77890015,
    "tz": "America/New_York"
  }
}
```

### GeoIP Databases (sapics/ip-location-db)

**Source**: https://github.com/sapics/ip-location-db
**CDN**: jsdelivr
**Location**: `{CONFIG_DIR}/geoip/*.mmdb`
**Downloaded**: On first run
**Total Size**: ~103MB (4 databases)

**Databases**:
1. `geolite2-city-ipv4.mmdb` (~50MB) - City data for IPv4
2. `geolite2-city-ipv6.mmdb` (~40MB) - City data for IPv6
3. `geo-whois-asn-country.mmdb` (~8MB) - Country data (public domain)
4. `asn.mmdb` (~5MB) - ASN/ISP information

**Update Schedule**: Weekly (Sunday 3:00 AM via built-in scheduler)

---

## 🗺️ Routes & Endpoints

**Route Matching Philosophy**: Frontend routes mirror API routes
- `/openapi` ↔ `/api/v1/openapi`
- `/graphql` ↔ `/api/v1/graphql`
- `/search` ↔ `/api/v1/search`

### Public Routes (No Authentication)

```yaml
Homepage:
  GET  /                           → Home page
  GET  /api/v1                     → API information

Search:
  GET  /search                     → Search page
  GET  /api/v1/search              → Search airports
    ?q=query, ?city=name, ?country=code, ?limit=50

Nearby:
  GET  /nearby                     → Nearby search page
  GET  /api/v1/nearby              → Find nearby airports
    ?lat=40.64, ?lon=-73.78, ?radius=50

Airport Details:
  GET  /airport/:code              → Airport detail page
  GET  /api/v1/airport/:code       → Airport data (ICAO or IATA code)

Statistics:
  GET  /stats                      → Stats page
  GET  /api/v1/stats               → Database statistics

GeoIP:
  GET  /geoip                      → GeoIP lookup page
  GET  /api/v1/geoip               → Lookup request IP
  GET  /api/v1/geoip/:ip           → Lookup specific IP (IPv4 or IPv6)
  GET  /api/v1/geoip/airports/nearby → Airports near IP location

Export:
  GET  /api/v1/airports.json       → Full database JSON
  GET  /api/v1/airports.csv        → Full database CSV
  GET  /api/v1/airports.geojson    → Full database GeoJSON

API Documentation:
  GET  /openapi                    → Swagger UI (dark theme, site nav)
  GET  /api/v1/openapi             → Swagger UI (API version)
  GET  /api/v1/openapi.json        → OpenAPI 3.0 specification
  GET  /graphql                    → GraphQL Playground
  GET  /api/v1/graphql             → GraphQL endpoint (GET=playground, POST=query)

Health:
  GET  /healthz                    → Health check
  GET  /api/v1/health              → API health check
```

### Admin Routes (Authentication Required)

```yaml
Dashboard:
  GET  /admin                      → Admin dashboard (Basic Auth)
  GET  /api/v1/admin               → Admin info (Bearer Token)

Settings:
  GET  /admin/settings             → Settings page (WebUI)
  POST /admin/settings             → Update settings
  GET  /api/v1/admin/settings      → Get settings (JSON)
  PUT  /api/v1/admin/settings      → Update settings (JSON)

Database:
  GET  /admin/database             → Database management page
  POST /admin/database/test        → Test connection

Logs:
  GET  /admin/logs                 → Logs viewer
  GET  /admin/logs/:type           → View specific log

Health:
  GET  /admin/health               → Server health page
```

---

## 🔨 Building & Deployment

**Follow SPEC Section 6 (Makefile)**

### Build Commands

```bash
# Build all platforms (8 binaries + host)
make build

# Run tests (in Docker)
make test

# Create GitHub release (after successful build)
make release

# Build and push Docker images
make docker

# Build dev image (local only)
make docker-dev
```

### Output

**./binaries/** (9 files):
- `airports-linux-amd64`
- `airports-linux-arm64`
- `airports-macos-amd64`
- `airports-macos-arm64`
- `airports-windows-amd64.exe`
- `airports-windows-arm64.exe`
- `airports-bsd-amd64`
- `airports-bsd-arm64`
- `airports` (host platform)

**./releases/** (10 files):
- 8 platform binaries
- `airports-{VERSION}-src.tar.gz`
- `airports-{VERSION}-src.zip`

---

## 🔐 Authentication

**Admin-only authentication** - All airport data is public

**Methods**:
1. Bearer Token (API): `Authorization: Bearer <token>`
2. Basic Auth (WebUI): Browser prompt

**First Run**:
- Credentials auto-generated
- Saved to `{CONFIG_DIR}/admin_credentials`
- Displayed in console (save securely!)

---

## 🚀 Running

**Production** (systemd):
```bash
# Install
curl -fsSL https://raw.githubusercontent.com/apimgr/airports/main/scripts/install-linux.sh | sudo bash

# Manage
systemctl status airports
journalctl -u airports -f
```

**Docker**:
```bash
docker-compose up -d
```

**Development** (container testing):
```bash
# Build dev image
make docker-dev

# Run test environment
docker-compose -f docker-compose.test.yml up -d

# Or Incus
incus launch images:alpine/3.19 test-airports
incus file push ./binaries/airports test-airports/usr/local/bin/
incus exec test-airports -- /usr/local/bin/airports --port 64555
```

---

## 📊 Configuration

**ALL configuration via Admin WebUI** at `/admin/settings`

**CLI Flags (Startup ONLY)**:
- `--port` - Server port
- `--address` - Listen address (default: `::` dual-stack)
- `--config`, `--data`, `--logs` - Directories

**Info Flags**:
- `--version` - Show version number
- `--help` - Show help
- `--status` - Health check

**Service Commands**:
- `service start|stop|restart|reload|status`

**Environment Variables**:
- `DEBUG=1` - Enable debug mode
- `PORT`, `ADDRESS` - Server config
- `CONFIG_DIR`, `DATA_DIR`, `LOGS_DIR` - Directories

---

## 🧪 Testing

**Follow SPEC Section 14 (Testing Environment)**

**Priority**:
1. Incus (preferred)
2. Docker (acceptable)
3. Host (FORBIDDEN without explicit approval)

**Multi-distro testing required**:
- Alpine (musl libc)
- Ubuntu 24.04 (glibc + systemd)

---

## 📝 License

MIT License - See LICENSE.md

**Data Licenses**:
- airports.json: Public Domain (OurAirports)
- GeoIP: CC BY-SA 4.0 (MaxMind GeoLite2), CC0/PDDL (geo-whois)

---

**For complete specifications, see `/root/Projects/github/apimgr/SPEC.md`**

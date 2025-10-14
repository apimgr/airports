# ✈️ Airports API Server - Project Specification

**Project**: airports
**Module**: github.com/apimgr/airports
**Language**: Go 1.21+
**Purpose**: Public airport location API with admin-protected server configuration
**Data**: 35,000+ airports worldwide (embedded), GeoIP databases (P3TERX)

---

## 📖 Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Directory Layout](#directory-layout)
4. [Data Sources](#data-sources)
5. [Authentication](#authentication)
6. [Routes & Endpoints](#routes--endpoints)
7. [Configuration](#configuration)
8. [Build & Deployment](#build--deployment)
9. [Development](#development)
10. [Testing](#testing)

---

## 🎯 Project Overview

### What This Is

A **public airport information API** with a web frontend, built as a single self-contained Go binary.

- **Public API**: All airport data is freely accessible (no authentication)
- **Admin Interface**: Server configuration protected by token/password authentication
- **Embedded Data**: airports.json (8.7MB) + GeoIP databases (84MB) built into binary
- **Fast Search**: In-memory indexes for instant lookups
- **Geographic Queries**: Nearby search, bounding box, distance calculations
- **Web Frontend**: Go html/template based UI with dark theme
- **Export Formats**: JSON, CSV, GeoJSON

### Key Features

- Search by ICAO/IATA code, city, country, state
- Find airports near coordinates (haversine distance)
- GeoIP lookup (find airports near IP address)
- Imperial/metric unit support
- RESTful API with matching web/API routes
- Admin dashboard for server configuration
- Single binary deployment (~100MB with embedded data)

---

## 🏗️ Architecture

### System Design

```
┌─────────────────────────────────────────┐
│         Single Go Binary                │
│  ┌─────────────────────────────────┐   │
│  │  Embedded Assets (go:embed)     │   │
│  │  • airports.json (8.7MB)        │   │
│  │  • GeoIP databases (84MB)       │   │
│  │  • HTML templates               │   │
│  │  • CSS/JS/Images                │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │  In-Memory Data Structures      │   │
│  │  • Airport maps/indexes         │   │
│  │  • GeoIP readers                │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │  HTTP Server (Chi Router)       │   │
│  │  • Public routes (no auth)      │   │
│  │  • Admin routes (auth required) │   │
│  │  • API v1 endpoints             │   │
│  └─────────────────────────────────┘   │
│  ┌─────────────────────────────────┐   │
│  │  SQLite Database                │   │
│  │  • Admin credentials (hashed)   │   │
│  │  • Server settings              │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### Technology Stack

- **Language**: Go 1.21+
- **HTTP Router**: Chi v5
- **Database**: SQLite (modernc.org/sqlite - pure Go, no CGO)
- **Templates**: Go html/template
- **GeoIP**: oschwald/geoip2-golang
- **Embedding**: Go embed.FS
- **Authentication**: SHA-256 hashing, Bearer tokens, Basic Auth

---

## 📁 Directory Layout

### OS-Specific Paths

```yaml
Linux/BSD (with root privileges):
  Config:  /etc/airports/
  Data:    /var/lib/airports/
  Logs:    /var/log/airports/
  Runtime: /run/airports/

Linux/BSD (without root):
  Config:  ~/.config/airports/
  Data:    ~/.local/share/airports/
  Logs:    ~/.local/state/airports/
  Runtime: ~/.local/run/airports/

macOS (with privileges):
  Config:  /Library/Application Support/Airports/
  Data:    /Library/Application Support/Airports/data/
  Logs:    /Library/Logs/Airports/
  Runtime: /var/run/airports/

macOS (without privileges):
  Config:  ~/Library/Application Support/Airports/
  Data:    ~/Library/Application Support/Airports/data/
  Logs:    ~/Library/Logs/Airports/
  Runtime: ~/Library/Application Support/Airports/run/

Windows:
  Config:  C:\ProgramData\Airports\config\
  Data:    C:\ProgramData\Airports\data\
  Logs:    C:\ProgramData\Airports\logs\
  Runtime: C:\ProgramData\Airports\run\

Windows (user):
  Config:  %APPDATA%\Airports\config\
  Data:    %APPDATA%\Airports\data\
  Logs:    %APPDATA%\Airports\logs\
  Runtime: %APPDATA%\Airports\run\

Docker:
  Config:  /config
  Data:    /data
  Logs:    /data/logs
  Runtime: /tmp
```

### Directory Contents

```yaml
Config Directory:
  - admin_credentials     # Generated on first run (0600 permissions)
  - settings.db          # SQLite database (if separate from data)

Data Directory:
  - airports.db          # SQLite database (default location)
  - backups/             # Automatic backups
  - cache/               # Optional cache files

Logs Directory:
  - access.log           # HTTP access logs
  - error.log            # Application errors
  - audit.log            # Admin actions

Runtime Directory:
  - airports.pid         # Process ID file
  - airports.sock        # Unix socket (optional)
```

### Environment Variables & Flags

```yaml
Directory Overrides (in priority order):
  1. Command-line flags
  2. Environment variables
  3. OS defaults

Flags:
  --config DIR        # Configuration directory
  --data DIR          # Data directory
  --logs DIR          # Logs directory

  --port PORT         # HTTP port (default: random 64000-64999)
  --address ADDR      # Listen address (default: 0.0.0.0)

  --db-type TYPE      # Database type: sqlite, mysql, postgres
  --db-path PATH      # SQLite database path
  --db-url URL        # Database connection string

Environment Variables:
  CONFIG_DIR          # Configuration directory
  DATA_DIR            # Data directory
  LOGS_DIR            # Logs directory

  PORT                # Server port
  ADDRESS             # Listen address

  DATABASE_URL        # Full connection string
  DB_TYPE             # Database type
  DB_PATH             # SQLite path

  ADMIN_USER          # Admin username (first run only)
  ADMIN_PASSWORD      # Admin password (first run only)
  ADMIN_TOKEN         # Admin API token (first run only)

Docker Environment:
  Mounted volumes:
    -v ./config:/config
    -v ./data:/data

  Environment:
    -e CONFIG_DIR=/config
    -e DATA_DIR=/data
    -e PORT=8080
    -e ADMIN_PASSWORD=changeme
```

### Project Source Layout

```
./
├── airports/              # Airport service package
│   ├── data/
│   │   └── airports.json  # Embedded airport data (8.7MB)
│   ├── data.go            # Data loading & indexing
│   └── handlers.go        # HTTP handlers
├── geoip/                 # GeoIP service package
│   ├── data/
│   │   ├── GeoLite2-City.mmdb     # ~70MB
│   │   ├── GeoLite2-Country.mmdb  # ~6MB
│   │   └── GeoLite2-ASN.mmdb      # ~8MB
│   ├── service.go         # GeoIP lookups
│   └── handlers.go        # HTTP handlers
├── database/              # Database package
│   ├── database.go        # Connection & schema
│   ├── settings.go        # Settings CRUD
│   └── auth.go            # Admin auth
├── paths/                 # OS path detection
│   └── paths.go           # OS-specific directory resolution
├── server/                # HTTP server package
│   ├── server.go          # Server setup & routing
│   ├── auth_middleware.go # Auth middleware
│   ├── admin_handlers.go  # Admin route handlers
│   ├── handlers.go        # Public handlers
│   ├── web_handlers.go    # Web page handlers
│   ├── static/            # Embedded static files
│   │   ├── css/
│   │   ├── js/
│   │   └── images/
│   └── templates/         # Embedded HTML templates
│       ├── base.html
│       ├── home.html
│       ├── search.html
│       └── admin/
│           ├── dashboard.html
│           └── settings.html
├── scripts/               # Production scripts
│   ├── install.sh         # Installation script
│   └── backup.sh          # Backup script
├── tests/                 # Test & debug scripts
│   ├── test-docker.sh     # Docker testing script
│   ├── unit/              # Unit tests
│   ├── integration/       # Integration tests
│   └── e2e/               # End-to-end tests
├── rootfs/                # Docker persistent volumes (gitignored)
│   ├── config/            # → /config (in container)
│   ├── data/              # → /data (in container)
│   ├── logs/              # → /logs (in container)
│   └── db/                # External databases
│       ├── postgres/
│       └── mysql/
├── main.go                # Entry point
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── docker-compose.yml      # Production compose
├── docker-compose.test.yml # Testing compose (/tmp volumes)
├── CLAUDE.md              # This file (specification)
├── docs/                  # Documentation
│   ├── README.md          # Documentation index
│   ├── SERVER.md          # Server administration guide
│   └── API.md             # Complete API documentation
└── README.md              # User documentation
```

---

## 💾 Data Sources

### airports.json

```yaml
Location: airports/data/airports.json
Size: 8.7MB
Records: 35,000+ worldwide airports
Embedded: Yes (go:embed)
Format: JSON object with ICAO keys

Structure:
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

Indexes Built on Startup:
  - By ICAO (primary)
  - By IATA (can have duplicates/empty)
  - By City (lowercase, multiple per city)
  - By Country (ISO codes)
  - By State (where applicable)
```

### GeoIP Databases

```yaml
Source: P3TERX/GeoLite.mmdb
Repository: https://github.com/P3TERX/GeoLite.mmdb
Location: geoip/data/*.mmdb
Embedded: Yes (go:embed)
Total Size: ~84MB

Databases:
  1. GeoLite2-City.mmdb (~70MB)
     - City-level geolocation
     - Coordinates, timezone

  2. GeoLite2-Country.mmdb (~6MB)
     - Country-level fallback

  3. GeoLite2-ASN.mmdb (~8MB)
     - ISP information

Download:
  Manual: make download-geoip
  Auto: Part of build process
  URL: https://github.com/P3TERX/GeoLite.mmdb/releases/latest
```

---

## 🔐 Authentication

### Overview

This project uses **admin-only authentication** - all airport data is public, only server configuration requires authentication.

**Complete guide**: [SERVER.md](./docs/SERVER.md)

### Authentication Methods

```yaml
1. API Token (Bearer):
   Header: Authorization: Bearer <token>
   Use: Programmatic access to admin API
   Format: 64-character hex string
   Routes: /api/v1/admin/*

2. Basic Auth:
   Header: Authorization: Basic <base64(user:pass)>
   Use: Web UI access
   Browser: Prompts automatically
   Routes: /admin/*
```

### First Run Setup

```yaml
On first startup:
  1. Check if admin credentials exist in database

  2. If not, generate:
     - Username: $ADMIN_USER or "administrator"
     - Password: $ADMIN_PASSWORD or random 16-char
     - Token: $ADMIN_TOKEN or random 64-char hex

  3. Save to database (SHA-256 hashed)

  4. Write to {CONFIG_DIR}/admin_credentials (0600)
     Example: /etc/airports/admin_credentials

  5. Display credentials in console output
     ⚠️  Shown once - save securely!

Credential File Format:
  AIRPORTS API - ADMIN CREDENTIALS
  ========================================
  WEB UI LOGIN:
    URL:      http://server:port/admin
    Username: administrator
    Password: <password>

  API TOKEN:
    Header:   Authorization: Bearer <token>
    Token:    <64-char-hex>

  Created: 2024-01-01 12:00:00
  ========================================
```

### Environment Variables

```yaml
First Run Only (ignored after setup):
  ADMIN_USER=admin            # Default: administrator
  ADMIN_PASSWORD=secure123    # Default: random 16-char
  ADMIN_TOKEN=abc123...       # Default: random 64-char hex

After first run:
  Credentials stored in database
  Environment variables ignored
  To reset: delete database
```

---

## 🗺️ Routes & Endpoints

### Route Matching Philosophy

**Routes must mirror between web and API:**
- `/` ↔ `/api/v1`
- `/search` ↔ `/api/v1/search`
- `/docs` ↔ `/api/v1/docs`
- `/admin` ↔ `/api/v1/admin`

### Public Routes (No Authentication)

```yaml
Homepage:
  GET  /                      → Home page with search interface
  GET  /api/v1                → API information JSON

Search:
  GET  /search                → Search page
  GET  /api/v1/search         → Search airports (JSON)
    Query params:
      ?q=query               - Search term
      ?city=name            - Filter by city
      ?country=code         - Filter by country
      ?state=name           - Filter by state
      ?limit=50             - Results limit
      ?offset=0             - Pagination
      ?units=imperial       - imperial or metric

Nearby Search:
  GET  /nearby                → Nearby search page
  GET  /api/v1/nearby         → Find nearby airports (JSON)
    Query params:
      ?lat=40.64            - Latitude (required)
      ?lon=-73.78           - Longitude (required)
      ?radius=50            - Radius in km (default: 50)
      ?limit=20             - Max results
      ?units=imperial       - Distance units

Airport Details:
  GET  /airport/:code         → Airport detail page
  GET  /api/v1/airport/:code  → Airport data (JSON)
    :code = ICAO or IATA

Statistics:
  GET  /stats                 → Stats page
  GET  /api/v1/stats          → Database statistics (JSON)

GeoIP:
  GET  /geoip                 → GeoIP lookup page
  GET  /api/v1/geoip          → Lookup request IP (JSON)
  GET  /api/v1/geoip/:ip      → Lookup specific IP (JSON)
  GET  /api/v1/geoip/airports/nearby → Airports near IP location
    Query params:
      ?ip=1.2.3.4           - IP to geolocate
      ?radius=100           - Search radius km
      ?limit=10             - Max results
      ?units=imperial       - Distance units

Export:
  GET  /api/v1/airports.json     → Full database JSON
  GET  /api/v1/airports.csv      → Full database CSV
  GET  /api/v1/airports.geojson  → Full database GeoJSON
  GET  /api/v1/search.csv        → Search results as CSV
  GET  /api/v1/search.geojson    → Search results as GeoJSON

Documentation:
  GET  /docs                  → API documentation page
  GET  /api/v1/docs           → API docs JSON (OpenAPI/Swagger)

Health:
  GET  /healthz               → Health check (JSON)
  GET  /api/v1/health         → Health check (JSON)
  GET  /api/v1/health.txt     → Health check (plain text)

Static Assets:
  GET  /static/*              → CSS, JS, images (embedded)
  GET  /favicon.ico           → Favicon
  GET  /robots.txt            → Robots file
```

### Admin Routes (Authentication Required)

```yaml
Dashboard:
  GET  /admin                 → Admin dashboard (Basic Auth)
  GET  /api/v1/admin          → Admin info (Bearer Token)

Settings:
  GET  /admin/settings        → Settings page
  POST /admin/settings        → Update settings
  GET  /api/v1/admin/settings → Get all settings (JSON)
  PUT  /api/v1/admin/settings → Update settings (JSON)

Database:
  GET  /admin/database        → Database management page
  POST /admin/database/test   → Test connection
  GET  /api/v1/admin/database → Database status (JSON)
  POST /api/v1/admin/database/test → Test connection (JSON)

Logs:
  GET  /admin/logs            → Logs viewer page
  GET  /admin/logs/:type      → View specific log
  GET  /api/v1/admin/logs     → List available logs (JSON)
  GET  /api/v1/admin/logs/:type → Get log content (JSON)

Backup:
  GET  /admin/backup          → Backup management page
  POST /admin/backup/create   → Create backup
  POST /admin/backup/restore  → Restore backup
  GET  /api/v1/admin/backup   → List backups (JSON)
  POST /api/v1/admin/backup   → Create backup (JSON)
  DELETE /api/v1/admin/backup/:id → Delete backup

Health:
  GET  /admin/health          → Server health page
  GET  /api/v1/admin/health   → Detailed health (JSON)
```

### Response Format

```yaml
JSON Success:
  {
    "success": true,
    "data": { ... },
    "timestamp": "2024-01-01T12:00:00Z"
  }

JSON Error:
  {
    "success": false,
    "error": {
      "code": "INVALID_INPUT",
      "message": "Invalid coordinates",
      "field": "lat"
    },
    "timestamp": "2024-01-01T12:00:00Z"
  }

Text Format (.txt endpoints):
  Plain text, human-readable
  No JSON structure
```

---

## ⚙️ Configuration

### Database Schema

```sql
-- Settings table
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('string', 'number', 'boolean', 'json')),
  category TEXT NOT NULL,
  description TEXT,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Admin credentials table
CREATE TABLE IF NOT EXISTS admin_credentials (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Default Settings

```yaml
Server:
  server.title: "Airports API"
  server.address: "0.0.0.0"
  server.http_port: 64000-64999 (random)
  server.https_enabled: false

Database:
  db.type: "sqlite"
  db.path: "{DATA_DIR}/airports.db"

Logging:
  log.level: "info"
  log.format: "json"
  log.access: true

Units:
  units.default: "imperial"
  units.distance: "miles"
  units.elevation: "feet"
```

### Modifying Settings

```yaml
Web UI:
  1. Navigate to /admin/settings
  2. Change values in form
  3. Click Save (applies immediately)

API:
  PUT /api/v1/admin/settings
  {
    "settings": {
      "server.title": "My Airport API",
      "units.default": "metric"
    }
  }

Environment (first run only):
  DATABASE_URL=sqlite:/data/airports.db
  PORT=8080
```

---

## 🔨 Build & Deployment

### Makefile Targets

```makefile
Targets:
  make deps              # Download Go dependencies
  make download-geoip    # Download latest GeoIP databases
  make build             # Build all platforms
  make test              # Run tests
  make run               # Build and run (current platform)
  make docker            # Build Docker image
  make release           # Create GitHub release
  make clean             # Remove build artifacts

Build Process:
  1. Download GeoIP databases (if missing)
  2. go mod download
  3. go build for all platforms:
     - Linux: amd64, arm64
     - Windows: amd64, arm64
     - macOS: amd64, arm64 (Apple Silicon)
     - FreeBSD: amd64
  4. Create binaries/ directory with outputs
  5. Auto-increment version in release.txt

Platforms:
  binaries/airports-linux-amd64
  binaries/airports-linux-arm64
  binaries/airports-windows-amd64.exe
  binaries/airports-windows-arm64.exe
  binaries/airports-macos-amd64
  binaries/airports-macos-arm64
  binaries/airports-bsd-amd64
  binaries/airports              # Current platform
```

### Docker

```yaml
Dockerfile:
  Multi-stage build (Go builder → scratch runtime)
  CGO_ENABLED=0 for static binary
  Size: ~100MB (with embedded data)
  Health check: /healthz endpoint via --status flag
  Volumes: /config, /data, /logs
  User: 65534:65534 (nobody)
  Exposed port: 8080

Building:
  docker build -t airports:latest .

  With version:
    docker build \
      --build-arg VERSION=1.0.0 \
      --build-arg COMMIT=$(git rev-parse --short HEAD) \
      --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
      -t airports:1.0.0 .

Production Deployment:
  Uses docker-compose.yml with ./rootfs for persistent storage

  Start:
    docker-compose up -d

  Volumes mounted to ./rootfs:
    - ./rootfs/config → /config (in container)
    - ./rootfs/data → /data (in container)
    - ./rootfs/logs → /logs (in container)

  Default configuration:
    - Internal port: 80 (Docker)
    - External port: 64080 (65xxx range)
    - Localhost only: 127.0.0.1:64080:80

  For public access:
    Change port mapping in docker-compose.yml:
      - "64080:80"      # Public HTTP

  Access:
    http://localhost:64080            # Homepage
    http://localhost:64080/admin      # Admin UI (Basic Auth)
    http://localhost:64080/api/v1     # API endpoints

  Check credentials:
    cat ./rootfs/config/admin_credentials

  View logs:
    docker-compose logs -f airports
    cat ./rootfs/logs/access.log

  Set admin credentials (first run):
    Edit docker-compose.yml environment:
      - ADMIN_USER=administrator
      - ADMIN_PASSWORD=strong-password

  Stop:
    docker-compose down

Testing/Debugging:
  Uses docker-compose.test.yml with /tmp for ephemeral storage

  Test:
    cd tests
    ./test-docker.sh

  Or manually:
    docker-compose -f docker-compose.test.yml up -d

  Volumes in /tmp/airports/rootfs (automatically cleaned):
    - /tmp/airports/rootfs/config → /config
    - /tmp/airports/rootfs/data → /data
    - /tmp/airports/rootfs/logs → /logs

  Access:
    http://localhost:8080             # Test server

  Cleanup:
    docker-compose -f docker-compose.test.yml down
    sudo rm -rf /tmp/airports/rootfs

Docker Run (Manual):
  # Production (with ./rootfs)
  docker run -d \
    --name airports \
    -p 127.0.0.1:64080:80 \
    -v $(pwd)/rootfs/config:/config \
    -v $(pwd)/rootfs/data:/data \
    -v $(pwd)/rootfs/logs:/logs \
    -e PORT=80 \
    -e ADMIN_PASSWORD=changeme \
    --restart unless-stopped \
    airports:latest

  # Testing (with /tmp)
  docker run -d \
    --name airports-test \
    -p 127.0.0.1:8080:80 \
    -v /tmp/airports/rootfs/config:/config \
    -v /tmp/airports/rootfs/data:/data \
    -v /tmp/airports/rootfs/logs:/logs \
    -e PORT=80 \
    -e ADMIN_PASSWORD=testpass \
    airports:latest

External Database (PostgreSQL):
  docker-compose.yml includes optional postgres service

  1. Uncomment postgres section
  2. Set environment on airports service:
     - DATABASE_URL=postgres://airports:changeme@postgres:5432/airports

  3. Start both:
     docker-compose up -d

  Database is mounted to /tmp/airports/rootfs/db/postgres for testing
```

### Manual Installation

```bash
# Download binary
wget https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
chmod +x airports-linux-amd64
sudo mv airports-linux-amd64 /usr/local/bin/airports

# Create directories (as root)
sudo mkdir -p /etc/airports /var/lib/airports /var/log/airports

# Run
sudo airports --port 80

# Or run as user (random port)
airports
# Check output for port and credentials
```

---

## 🛠️ Development

### Development Mode

```yaml
Enable:
  --dev flag
  OR DEV=true environment variable
  OR binary named airports-dev

Features:
  - Hot reload templates (no restart)
  - Detailed logging (SQL queries, stack traces)
  - Debug endpoints enabled
  - CORS allow all origins
  - Fast session timeout (5 min)

Debug Endpoints:
  GET /debug/routes          - List all routes
  GET /debug/config          - Show configuration
  GET /debug/db              - Database stats
  GET /debug/airports        - Airport index stats
  POST /debug/reset          - Reset to fresh state
```

### Local Development

```bash
# Install dependencies
make deps
make download-geoip

# Run with hot reload
make run-dev

# Or manually
go run . --dev --port 8080

# Server starts on http://localhost:8080
# Admin credentials displayed in console
```

---

## ✅ Testing

### Test Structure

```
tests/
├── unit/
│   ├── airports_test.go      # Airport service tests
│   ├── geoip_test.go          # GeoIP service tests
│   └── database_test.go       # Database tests
├── integration/
│   ├── api_test.go            # API endpoint tests
│   └── admin_test.go          # Admin auth tests
└── e2e/
    └── scenarios_test.go      # End-to-end tests
```

### Running Tests

```bash
# All tests
make test

# Or manually
go test -v -race ./...

# With coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Benchmarks
go test -v -bench=. -benchmem ./...
```

### Test Coverage Requirements

```yaml
Minimum Coverage: 80%

Critical Paths (100% coverage):
  - Admin authentication
  - Database initialization
  - Settings CRUD
  - Airport search/indexing
  - GeoIP lookups
  - Distance calculations
```

---

## 📊 Unit System

### Supported Units

```yaml
Imperial (Default):
  Distance: miles (mi)
  Elevation: feet (ft)
  Speed: knots (kts)

Metric:
  Distance: kilometers (km)
  Elevation: meters (m)
  Speed: kilometers/hour (km/h)

Conversion:
  1 km = 0.621371 miles
  1 meter = 3.28084 feet
```

### Using Units

```yaml
API Query Parameter:
  ?units=imperial  (default)
  ?units=metric

Response:
  {
    "distance": 31.07,
    "distance_unit": "mi",
    "elevation": 13,
    "elevation_unit": "ft"
  }

Settings:
  units.default: "imperial" or "metric"
  Changes apply to all future requests
  Does not affect stored data (always metric internally)
```

---

## 🔒 Security

### Best Practices

```yaml
Credentials:
  - Change default admin password immediately
  - Rotate API tokens periodically
  - Use HTTPS in production
  - Restrict admin routes to internal network

File Permissions:
  admin_credentials: 0600 (owner read/write only)
  Database: 0644 (owner write, all read)
  Logs: 0644

Network:
  - Bind to 0.0.0.0 only if needed
  - Use 127.0.0.1 for local-only access
  - Configure firewall rules
  - Use reverse proxy (nginx/Caddy) for HTTPS

Database:
  - Passwords hashed with SHA-256
  - Tokens hashed with SHA-256
  - SQL injection protection (prepared statements)
  - Input validation on all endpoints
```

---

## 📝 License

MIT License - See LICENSE file

### Embedded Data Licenses

```yaml
airports.json:
  Source: OurAirports.com
  License: Public Domain

GeoIP Databases (P3TERX):
  Source: https://github.com/P3TERX/GeoLite.mmdb
  License: CC BY-SA 4.0 (database), MIT (code)
  Attribution: MaxMind GeoLite2
```

---

**Airports API Server v1.0** - A focused, production-ready airport information API with admin-only authentication. Built for simplicity, performance, and ease of deployment.

# ✈️ Airports API Server

Global airport location information API with GeoIP integration - A single static binary with embedded data.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev)
[![Docker](https://img.shields.io/badge/Docker-Available-2496ED?logo=docker)](https://github.com/apimgr/airports/pkgs/container/airports)

## 🎯 About

A production-ready RESTful API server providing comprehensive airport information with integrated GeoIP location services. Built as a single self-contained binary with all assets and data embedded for simple deployment.

### Features

- **35,000+ Airports** - Complete global airport database with ICAO/IATA codes
- **GeoIP Integration** - Automatic location detection using MaxMind GeoLite2 databases
- **Fast Search** - Search by code, name, city, country, or coordinates
- **Geographic Queries** - Find airports nearby, within radius, or bounding box
- **Multiple Formats** - Export as JSON, CSV, or GeoJSON
- **RESTful API** - Clean, intuitive endpoints with OpenAPI documentation
- **Web Interface** - Simple web UI with API documentation
- **Admin Dashboard** - Protected configuration interface
- **Single Binary** - No dependencies, ~21MB static binary
- **Auto-Updates** - Weekly GeoIP database updates via built-in scheduler
- **Multi-Platform** - Linux, macOS, Windows, FreeBSD (amd64 & arm64)

---

## 📦 Production Installation

### Binary Installation

Download and run the pre-built binary for your platform:

```bash
# Linux AMD64
curl -L -o airports https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
chmod +x airports
sudo mv airports /usr/local/bin/

# Create directories (will use /etc, /var/lib, /var/log if run as root)
sudo airports

# Or run as user (uses ~/.config, ~/.local/share, ~/.local/state)
./airports
```

The server will:
1. Detect OS-specific directories automatically
2. Download GeoIP databases on first run (~84MB)
3. Generate admin credentials and save to `{CONFIG_DIR}/admin_credentials`
4. Select a random available port (64000-64999) or use `PORT` environment variable
5. Display the accessible URL (hostname or IP, never localhost/127.0.0.1)

**First Run Output:**
```
Config directory: /etc/airports
Data directory: /var/lib/airports
Logs directory: /var/log/airports
Database initialized successfully
Admin authentication initialized
Selected random available port: 64555
⚠️  ADMIN CREDENTIALS SAVED TO: /etc/airports/admin_credentials
⚠️  Username: administrator
⚠️  API Token: abc123...
⚠️  Access URL: http://your-server.com:64555
⚠️  Save these credentials securely! They will not be shown again.
Server listening on http://your-server.com:64555
```

### Environment Variables

**First run only** (stored in database after initial setup):

```bash
# Server Configuration
export PORT=8080                    # HTTP port (default: random 64000-64999)
export ADDRESS=0.0.0.0              # Listen address

# Directory Overrides (optional)
export CONFIG_DIR=/etc/airports
export DATA_DIR=/var/lib/airports
export LOGS_DIR=/var/log/airports

# Database Configuration
export DB_PATH=/var/lib/airports/airports.db    # SQLite path
export DATABASE_URL=sqlite:/data/airports.db    # Or full connection string

# Admin Credentials (first run only)
export ADMIN_USER=administrator
export ADMIN_PASSWORD=your-secure-password
export ADMIN_TOKEN=your-api-token
```

### Systemd Service

Create `/etc/systemd/system/airports.service`:

```ini
[Unit]
Description=Airports API Server
After=network.target

[Service]
Type=simple
User=airports
Group=airports
ExecStart=/usr/local/bin/airports
Restart=always
RestartSec=5
Environment="PORT=8080"

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable airports
sudo systemctl start airports
sudo systemctl status airports
```

---

## 🐳 Docker Deployment

### Docker Compose (Recommended)

**Production:**

```bash
# Download docker-compose.yml
curl -O https://raw.githubusercontent.com/apimgr/airports/main/docker-compose.yml

# Start service
docker-compose up -d

# Check credentials
cat ./rootfs/config/airports/admin_credentials

# View logs
docker-compose logs -f

# Access API
curl http://172.17.0.1:64180/api/v1
```

**Development:**

```bash
# Use test compose file (ephemeral storage in /tmp)
docker-compose -f docker-compose.test.yml up -d

# Access
curl http://localhost:64181/api/v1

# Cleanup
docker-compose -f docker-compose.test.yml down
sudo rm -rf /tmp/airports/rootfs
```

### Docker Run

```bash
# Production
docker run -d \
  --name airports \
  -p 172.17.0.1:64180:80 \
  -v ./config:/config \
  -v ./data:/data \
  -v ./logs:/logs \
  -e ADMIN_PASSWORD=changeme \
  --restart unless-stopped \
  ghcr.io/casapps/airports:latest

# Development
docker run -d \
  --name airports-dev \
  -p 64181:80 \
  -v /tmp/airports/config:/config \
  -v /tmp/airports/data:/data \
  -e ADMIN_PASSWORD=testpass \
  airports:dev
```

### Docker Configuration

**Image Details:**
- Base: Alpine 3.19
- Size: ~110MB (with curl, bash, and all dependencies)
- Port: 80 (internal)
- User: nobody (65534:65534)
- Health Check: `--status` flag

**Volumes:**
- `/config` - Configuration and credentials
- `/data` - SQLite database and GeoIP databases
- `/logs` - Application logs

**Environment Variables:**
```bash
PORT=80                           # Internal port
CONFIG_DIR=/config
DATA_DIR=/data
LOGS_DIR=/logs
DB_PATH=/data/db/airports.db      # SQLite database location
ADMIN_USER=administrator          # First run only
ADMIN_PASSWORD=changeme           # First run only
```

---

## 🔌 API Usage

### Quick Examples

```bash
# Health check
curl http://your-server:port/healthz

# Get airport by code
curl http://your-server:port/api/v1/airport/JFK

# Search airports
curl "http://your-server:port/api/v1/search?q=New+York&limit=10"

# Find nearby airports (50km radius)
curl "http://your-server:port/api/v1/nearby?lat=40.6398&lon=-73.7789&radius=50"

# GeoIP lookup (your IP)
curl http://your-server:port/api/v1/geoip

# GeoIP lookup (specific IP)
curl http://your-server:port/api/v1/geoip/8.8.8.8

# Find airports near IP location
curl "http://your-server:port/api/v1/geoip/airports/nearby?radius=100"

# Export full database
curl -o airports.json http://your-server:port/api/v1/airports.json
curl -o airports.csv http://your-server:port/api/v1/airports.csv
curl -o airports.geojson http://your-server:port/api/v1/airports.geojson

# Statistics
curl http://your-server:port/api/v1/stats
```

### Admin Panel

Access the admin dashboard to manage server configuration:

```bash
# Web UI (Basic Auth)
http://your-server:port/admin

# API (Bearer Token)
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://your-server:port/api/v1/admin/settings
```

Credentials are saved in `{CONFIG_DIR}/admin_credentials` on first run.

### API Documentation

Interactive API documentation with Swagger UI and GraphQL Playground:

**OpenAPI/Swagger:**
- **Web UI**: `http://your-server:port/openapi`
- **API UI**: `http://your-server:port/api/v1/openapi`
- **OpenAPI Spec**: `http://your-server:port/api/v1/openapi.json`

**GraphQL:**
- **Playground**: `http://your-server:port/graphql`
- **API Playground**: `http://your-server:port/api/v1/graphql`
- **Query Endpoint**: `POST http://your-server:port/api/v1/graphql`

Both interfaces match the site theme with dark mode and full navigation.

---

## 🛠️ Development

### Requirements

- Go 1.23+
- Docker (for builds and testing)
- Make
- git

### Quick Start

```bash
# Clone repository
git clone https://github.com/apimgr/airports.git
cd airports

# Build all platforms (uses Docker Alpine builder)
make build

# Run tests
make test

# Build development Docker image
make docker-dev

# Test with Docker Compose
docker-compose -f docker-compose.test.yml up -d

# View build information
./binaries/airports --version
```

### Build System & Testing

**Makefile Targets:**

```bash
make build      # Build binaries for all platforms (Linux, macOS, Windows, BSD)
make test       # Run all tests
make release    # Create GitHub release
make docker     # Build and push multi-arch Docker images
make clean      # Clean build artifacts
```

**Testing:**

```bash
# All tests (runs in Docker)
make test

# With coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Benchmarks
go test -v -bench=. -benchmem ./...
```

**Platforms:**
- Linux: amd64, arm64
- macOS: amd64, arm64 (Apple Silicon)
- Windows: amd64, arm64
- FreeBSD: amd64, arm64

**Versioning:**
- Semantic versioning (MAJOR.MINOR.PATCH)
- Stored in `release.txt`
- Override: `VERSION=1.2.3 make build`

**Output:**
- `./binaries/` - All platform binaries + host binary
- `./releases/` - Release artifacts for GitHub

### Project Structure

```
airports/
├── src/                        # Source code
│   ├── main.go                 # Entry point
│   ├── airports/               # Airport service
│   │   ├── data/
│   │   │   └── airports.json   # Embedded airport data (8.7MB)
│   │   ├── service.go
│   │   └── handlers.go
│   ├── geoip/                  # GeoIP service
│   │   ├── service.go          # Downloads databases on first run
│   │   └── handlers.go
│   ├── database/               # Database layer
│   │   ├── database.go
│   │   ├── settings.go
│   │   └── auth.go
│   ├── paths/                  # OS-specific path detection
│   │   └── paths.go
│   ├── scheduler/              # Task scheduler
│   │   └── scheduler.go
│   └── server/                 # HTTP server
│       ├── server.go
│       ├── handlers.go
│       ├── static/             # Embedded assets
│       └── templates/          # Embedded templates
├── binaries/                   # Built binaries (gitignored)
├── release/                    # Release artifacts (gitignored)
├── rootfs/                     # Docker volumes (gitignored)
│   ├── config/airports/
│   ├── data/airports/
│   └── logs/airports/
├── go.mod
├── go.sum
├── release.txt                 # Version tracking
├── Makefile                    # Build system
├── Dockerfile                  # Alpine-based multi-stage build
├── docker-compose.yml          # Production
├── docker-compose.test.yml     # Development/testing
├── Jenkinsfile                 # CI/CD pipeline
├── CLAUDE.md                   # Project specification
└── README.md                   # This file
```

### Development Mode

Run with debug features enabled:

```bash
# Using binary
./binaries/airports --dev --port 8080

# Using Docker
docker-compose -f docker-compose.test.yml up -d
```

**Development Features:**
- Hot reload templates
- Detailed logging (SQL queries, stack traces)
- Debug endpoints enabled (`/debug/*`)
- CORS allow all origins

### CI/CD

Automated builds, testing, and deployment via multiple pipelines.

#### GitHub Actions

Two separate workflows for binary and Docker releases:

**Workflows:**
- `.github/workflows/release.yml` - Binary builds and GitHub releases
- `.github/workflows/docker.yml` - Multi-arch Docker images

**Triggers:**
- Push to `main` branch
- Monthly schedule (1st at 3:00 AM UTC)
- Manual workflow dispatch

**Artifacts:**
- All 8 platform binaries → GitHub Releases
- Docker images → `ghcr.io/apimgr/airports:latest`
- Version from `release.txt` (never modified by Actions)

**Setup:**
Enable in repository settings:
```
Settings → Actions → General
- Workflow permissions: "Read and write permissions"
```

#### Jenkins Pipeline

Multi-architecture builds on self-hosted infrastructure:

- **Server**: jenkins.casjay.cc
- **Agents**: amd64, arm64 (parallel builds)
- **Stages**:
  1. Checkout
  2. Test (parallel on both architectures)
  3. Build Binaries (parallel)
  4. Build Docker Images (multi-arch)
  5. Push to ghcr.io
  6. GitHub Release

**Required Credentials:**
- `github-registry` - GitHub Container Registry
- `github-token` - GitHub API token

#### ReadTheDocs

Automatic documentation deployment:

- **URL**: https://airports.readthedocs.io
- **Theme**: MkDocs Material with Dracula color scheme
- **Formats**: HTML, PDF, EPUB
- **Trigger**: Every push to main

**Local preview:**
```bash
cd docs
pip install -r requirements.txt
mkdocs serve
```

#### Monthly Automated Builds

Both GitHub Actions and Jenkins rebuild monthly to:
- Keep dependencies current
- Refresh Docker base images
- Ensure reproducible builds

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Build: `make build`
6. Submit a pull request

---

## 📊 Data Sources

- **Airports**: [OurAirports](https://ourairports.com/) - Public domain airport database
- **GeoIP**: [sapics/ip-location-db](https://github.com/sapics/ip-location-db) - MaxMind GeoLite2 + aggregated sources (updated daily via jsdelivr CDN)

### GeoIP Updates

GeoIP databases are automatically downloaded on first run and can be manually updated anytime. Daily updates are available via jsdelivr CDN from sapics/ip-location-db which aggregates MaxMind GeoLite2 and WHOIS data.

**Benefits of sapics/ip-location-db:**
- Daily updates (vs weekly from other sources)
- Multiple data sources aggregated for higher accuracy
- Public domain country data (geo-whois-asn-country)
- Faster, more reliable via jsdelivr CDN
- Separate IPv4/IPv6 databases for better performance

**Manual Update:**
Databases are automatically downloaded on first run. To manually update, delete the existing files and restart:
```bash
# Remove old databases
sudo rm -rf /etc/airports/geoip/*.mmdb

# Restart server (will re-download automatically)
sudo systemctl restart airports
```

---

## 🔒 Security

### Best Practices

- Change default admin password immediately after first run
- Use HTTPS in production (reverse proxy: nginx, Caddy, Traefik)
- Restrict admin routes to internal network
- Rotate API tokens periodically
- Review file permissions:
  - `admin_credentials`: 0600 (owner read/write only)
  - Database: 0644
  - Logs: 0644

### Authentication

**Admin authentication is required for:**
- `/admin/*` - Web UI (Basic Auth)
- `/api/v1/admin/*` - API (Bearer Token)

**Public routes (no auth):**
- All airport data endpoints
- GeoIP lookups
- Export endpoints
- Documentation

---

## 📝 License

MIT License - See [LICENSE](LICENSE) file for details

### Third-Party Data Licenses

- **Airport Data**: Public Domain ([OurAirports](https://ourairports.com/))
- **GeoIP Databases**: CC BY-SA 4.0 ([MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data))

---

## 🙏 Credits

- Airport data provided by [OurAirports](https://ourairports.com/)
- GeoIP data from [MaxMind GeoLite2](https://www.maxmind.com/) and aggregated sources (distributed via [sapics/ip-location-db](https://github.com/sapics/ip-location-db))
- Built following production-grade Go API best practices

---

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/apimgr/airports/issues)
- **Documentation**: See [docs/](docs/) directory
  - [API.md](docs/API.md) - Complete API reference
  - [SERVER.md](docs/SERVER.md) - Server administration guide

---

**Airports API Server** - Production-ready global airport information API with GeoIP integration 🚀

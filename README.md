# ✈️ Airports API Server

Global airport location information API with GeoIP integration.

## Features

- **35,000+ Airports**: Complete global airport database
- **Fast Search**: Search by ICAO/IATA code, city, country, coordinates
- **GeoIP Integration**: Automatic location detection using P3TERX GeoIP databases
- **Geographic Queries**: Find airports nearby, within radius, or bounding box
- **Export Formats**: JSON, CSV, GeoJSON
- **Single Binary**: All data embedded (~95MB)
- **RESTful API**: Clean, intuitive endpoints
- **Web Interface**: Simple homepage with endpoint documentation

## Quick Start

### Download Binary

```bash
# Download latest release
curl -L -o airports https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
chmod +x airports

# Run
./airports --port 8080
```

### Build from Source

```bash
# Clone repository
git clone https://github.com/apimgr/airports.git
cd airports

# Build (downloads GeoIP databases automatically)
make build

# Run
./binaries/airports --port 8080
```

### Docker

```bash
# Run with Docker Compose
docker-compose up -d

# Or with Docker directly
docker run -p 8080:80 ghcr.io/apimgr/airports:latest
```

## API Endpoints

### Airport Search

```bash
# Get airport by code
curl http://localhost:8080/api/v1/airports/JFK

# Search airports
curl "http://localhost:8080/api/v1/airports/search?q=New+York"

# Find nearby airports
curl "http://localhost:8080/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50"

# Autocomplete
curl "http://localhost:8080/api/v1/airports/autocomplete?q=JFK"

# List all countries
curl http://localhost:8080/api/v1/airports/countries

# Get statistics
curl http://localhost:8080/api/v1/airports/stats

# Download full database
curl -o airports.json http://localhost:8080/api/v1/airports.json
```

### GeoIP

```bash
# Lookup your IP
curl http://localhost:8080/api/v1/geoip

# Lookup specific IP
curl http://localhost:8080/api/v1/geoip/8.8.8.8

# Find airports near IP location
curl http://localhost:8080/api/v1/geoip/airports/nearby
```

### Health Check

```bash
curl http://localhost:8080/healthz
```

## Command Line Options

```
airports [OPTIONS]

Options:
  --help            Show help message
  --version         Show version information
  --status          Show server status
  --port PORT       Set port (default: random 64000-64999)
  --dev             Run in development mode

Examples:
  airports                    # Start on random port
  airports --port 8080        # Start on port 8080
  airports --port "80,443"    # HTTP on 80, HTTPS on 443
```

## Configuration

The server uses a database for all configuration (no config files). On first run, environment variables are checked once:

- `DB_TYPE`: sqlite|mysql|postgres|mssql (default: sqlite)
- `PORT`: Server port (default: random 64000-64999)

After first run, all settings stored in database and managed via admin UI.

## Development

### Requirements

- Go 1.21+
- Make
- curl (for downloading GeoIP databases)

### Build

```bash
make build          # Build all platforms
make dev            # Build and run in development mode
make test           # Run tests
make docker         # Build Docker image
make clean          # Clean build artifacts
```

### Project Structure

```
.
├── airports/           # Airport data service
├── geoip/             # GeoIP lookup service
├── server/            # HTTP server and handlers
├── main.go            # Entry point
├── src/data/          # Embedded data files
│   ├── airports.json  # Airport database
│   └── geoip/         # GeoIP databases
├── Makefile           # Build automation
├── Dockerfile         # Container definition
└── docker-compose.yml # Compose configuration
```

## Data Sources

- **Airports**: [OurAirports](https://ourairports.com/) dataset
- **GeoIP**: [P3TERX/GeoLite.mmdb](https://github.com/P3TERX/GeoLite.mmdb) - Weekly updated GeoLite2 databases

## License

MIT License - See LICENSE file for details

## Credits

- Airport data from OurAirports
- GeoIP data from MaxMind GeoLite2 (distributed via P3TERX)
- Built following the Universal Server Template Specification v1.0

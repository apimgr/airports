# Airports API Documentation

Welcome to the Airports API Server documentation.

## Quick Links

- [API Reference](API.md) - Complete API endpoint documentation
- [Server Administration](SERVER.md) - Server setup and configuration guide
- [GitHub Repository](https://github.com/apimgr/airports)

## Overview

Global airport location information API with GeoIP integration - A single static binary with embedded data.

### Features

- **35,000+ Airports** - Complete global airport database
- **GeoIP Integration** - Automatic location detection
- **Fast Search** - In-memory indexes for instant lookups
- **RESTful API** - Clean, intuitive endpoints
- **Single Binary** - ~21MB static binary, no dependencies

## Getting Started

### Quick Start

```bash
# Download and run
curl -L -o airports https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
chmod +x airports
./airports
```

### Docker

```bash
docker-compose up -d
```

See the [Server Administration Guide](SERVER.md) for detailed installation instructions.

## Documentation

- [API Reference](API.md) - All endpoints, parameters, and examples
- [Server Guide](SERVER.md) - Installation, configuration, and administration
- [README](../README.md) - Project overview and quick start

## Support

- **Issues**: [GitHub Issues](https://github.com/apimgr/airports/issues)
- **Source**: [GitHub Repository](https://github.com/apimgr/airports)

# Airports API

Welcome to the documentation for the **Airports API** — a single static Go binary serving 35,000+ airports via REST, GraphQL, and a server-side rendered web UI.

## Quick links

- [Installation](installation.md) — Docker, binary, systemd
- [Configuration](configuration.md) — every setting
- [API reference](api.md) — REST + GraphQL endpoints
- [CLI reference](cli.md) — `airports` and `airports-cli`
- [Admin & operations](admin.md) — service lifecycle, backups, monitoring
- [Security model](security.md) — what's authenticated, what isn't, and why
- [External integrations](integrations.md) — GeoIP, Tor, ACME, SMTP
- [Development](development.md) — contributing, build, test

## What you get

- 35,000+ airport records embedded in the binary at build time
- ICAO/IATA lookup, free-text search, nearby-by-radius, bounding-box, nearest-N
- GeoIP-based caller-location detection on the home page
- JSON / CSV / GeoJSON output
- REST under `/api/v1/...`, GraphQL at `/graphql`, OpenAPI at `/api/v1/server/swagger`
- Server-side rendered web UI with dark / light / auto theme
- Tor hidden-service support (binary owns Tor entirely)
- Single static binary, no runtime dependencies

## Quick start

```bash
docker run --rm -d \
  --name airports \
  -p 8080:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/airports:latest

curl http://localhost:8080/api/v1/airports/KJFK
```

See [installation.md](installation.md) for binary installs and systemd integration.

## Repository

- Source: <https://github.com/apimgr/airports>
- Issues: <https://github.com/apimgr/airports/issues>
- License: MIT

# Installation

The Airports API ships as a single self-contained static binary with embedded airport dataset (35,000+ records). No runtime dependencies. No database server required.

## Docker (recommended)

```bash
docker run --rm -d \
  --name airports \
  -p 8080:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/airports:latest
```

Then open <http://localhost:8080>.

### Docker Compose

A production compose file ships in the repository at `docker/docker-compose.yml`. Copy it to a working directory and run:

```bash
mkdir -p volumes/config volumes/data
docker compose -f docker/docker-compose.yml up -d
```

The compose file ships with sane hardcoded defaults — **no `.env` file is required or supported.** Edit the compose file directly to override.

## Binary Install (Linux / macOS / FreeBSD)

Download the appropriate archive from the [latest release](https://github.com/apimgr/airports/releases) page:

| Platform | Asset |
|----------|-------|
| Linux x86_64 | `airports-linux-amd64.tar.gz` |
| Linux ARM64 | `airports-linux-arm64.tar.gz` |
| macOS Intel | `airports-darwin-amd64.tar.gz` |
| macOS Apple Silicon | `airports-darwin-arm64.tar.gz` |
| Windows x86_64 | `airports-windows-amd64.zip` |
| FreeBSD x86_64 | `airports-freebsd-amd64.tar.gz` |

```bash
curl -q -LSsf -o airports.tar.gz \
  https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64.tar.gz
sha256sum -c SHA256SUMS
tar xzf airports.tar.gz
sudo install -m 0755 airports /usr/local/bin/airports
```

Verify with `airports --version`.

## First Run

Run the binary with no arguments:

```bash
airports
```

On first run the binary will:

1. Detect your OS and resolve config/data/log directories.
2. Auto-create `server.yml` with sane defaults at `~/.config/airports/server.yml` (or OS equivalent).
3. Create required directories.
4. Download GeoIP databases (~80 MB) to `~/.local/share/airports/security/geoip/`.
5. Print a startup banner with listen URLs, version, commit, and build date.
6. Begin serving on port 8080 (or 80 if run as root with `CAP_NET_BIND_SERVICE`).

## systemd Service (Linux)

The binary can install and manage its own systemd unit:

```bash
sudo airports --service install
sudo airports --service start
sudo airports --service status
```

Logs are written to journald and to `/var/log/airports/`.

## launchd Service (macOS)

```bash
sudo airports --service install
sudo airports --service start
```

The launchd plist is installed at `/Library/LaunchDaemons/io.github.apimgr.airports.plist`.

## Windows Service

```powershell
airports --service install
airports --service start
```

## Build from Source

The build runs entirely inside Docker — you do not need Go installed on the host.

```bash
git clone https://github.com/apimgr/airports.git
cd airports
make local        # build to binaries/airports
make build        # cross-compile all platforms
```

## Upgrade

```bash
# Docker
docker pull ghcr.io/apimgr/airports:latest
docker compose -f docker/docker-compose.yml up -d

# Binary self-update (downloads, verifies, replaces in place)
sudo airports --update
```

The self-update path refuses to run inside Docker — pull the new image instead.

## Uninstall

```bash
sudo airports --service uninstall      # remove service unit
sudo rm /usr/local/bin/airports         # remove binary
rm -rf ~/.config/airports ~/.local/share/airports ~/.cache/airports
```

Uninstall **never** removes config or data — they must be deleted manually as shown above.

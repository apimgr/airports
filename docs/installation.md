# Installation

The Airports API ships as a single self-contained static binary with the airport dataset (48,069 records) embedded. It does not require a database server. Tor and I2P integrations use their local daemons when available.

## Docker (recommended)

```bash
mkdir -p volumes/config volumes/data
docker run --rm -d \
  --name airports \
  -p 172.17.0.1:64580:80 \
  -v ./volumes/config:/config:z \
  -v ./volumes/data:/data:z \
  ghcr.io/apimgr/airports:latest
```

The container listens on the Docker bridge address at <http://172.17.0.1:64580>. Configure a reverse proxy to reach that address and port for public access.

### Docker Compose

A production compose file ships in the repository at `docker/docker-compose.yml`. Copy it to a working directory and run:

```bash
mkdir -p volumes/config volumes/data
docker compose -f docker/docker-compose.yml up -d
```

The compose file ships with sane hardcoded defaults — **no `.env` file is required or supported.** Edit the compose file directly to override.
The production compose file binds port `64580` to the Docker bridge address `172.17.0.1`; configure the reverse proxy to reach that address and port.

## Binary Install (Linux / macOS / FreeBSD)

Download the platform binary from the [latest release](https://github.com/apimgr/airports/releases) page:

| Platform | Asset |
|----------|-------|
| Linux x86_64 | `airports-linux-amd64` |
| Linux ARM64 | `airports-linux-arm64` |
| macOS Intel | `airports-darwin-amd64` |
| macOS Apple Silicon | `airports-darwin-arm64` |
| Windows x86_64 | `airports-windows-amd64.exe` |
| Windows ARM64 | `airports-windows-arm64.exe` |
| FreeBSD x86_64 | `airports-freebsd-amd64` |
| FreeBSD ARM64 | `airports-freebsd-arm64` |

```bash
curl -q -LSsf -o airports-linux-amd64 \
  https://github.com/apimgr/airports/releases/latest/download/airports-linux-amd64
curl -q -LSsf -o sha256.txt \
  https://github.com/apimgr/airports/releases/latest/download/sha256.txt
grep '  airports-linux-amd64$' sha256.txt | sha256sum -c -
sudo install -m 0755 airports-linux-amd64 /usr/local/bin/airports
```

Verify with `airports --version`.

## First Run

Run the binary with no arguments:

```bash
airports
```

On first run the binary will:

1. Detect your OS and resolve config/data/log directories.
2. Auto-create `server.yml` with sane defaults at `~/.config/apimgr/airports/server.yml` (or OS equivalent).
3. Create required directories.
4. Download GeoIP databases (~80 MB) to `~/.local/share/apimgr/airports/security/geoip/`.
5. Print a startup banner with listen URLs, version, commit, and build date.
6. Begin serving on a randomly selected `64xxx` port by default. Set `PORT` or `server.port` to choose a fixed port.

## systemd Service (Linux)

The binary can install and manage its own systemd unit:

```bash
sudo airports --service --install
sudo airports --service start
sudo airports --status
```

Logs are written to journald and to `/var/log/airports/`.

If you decline the privilege prompt or cannot escalate, Linux systemd installs a user unit instead:

```bash
airports --service --install
airports --status
```

The user unit is stored in `~/.config/systemd/user/airports.service`, runs as your account, and uses your user config/data paths. It can bind only ports `1024` and higher. It starts at login; enable systemd lingering separately if it must survive logout.

On Linux with OpenRC, runit, s6, or SysVinit, no user service is installed. The command prints how to start the server with `airports --daemon` or ask an administrator to install the system service.

## launchd Service (macOS)

```bash
sudo airports --service --install
sudo airports --service start
```

The launchd plist is installed at `/Library/LaunchDaemons/io.github.apimgr.airports.plist`.

Without admin access, the same install command creates a LaunchAgent in `~/Library/LaunchAgents/`, runs as your account, and writes logs under `~/Library/Logs/apimgr/airports/`.

## Windows Service

```powershell
airports --service --install
airports --service start
```

Without administrator access, install creates and starts a Task Scheduler task named `airports` at the current user's logon.

## Build from Source

The build runs entirely inside Docker — you do not need Go installed on the local machine.

```bash
git clone https://github.com/apimgr/airports.git
cd airports
make build        # build debug binaries for all supported platforms
make release      # build production binaries and checksums locally
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
sudo airports --service --uninstall   # remove service and all server data (confirms first)
sudo rm /usr/local/bin/airports         # remove binary
rm -rf ~/.config/apimgr/airports ~/.local/share/apimgr/airports ~/.cache/apimgr/airports
```

Uninstall removes config, data, cache, logs, backups, the PID file, and the service user after confirmation. The binary remains and must be removed manually.

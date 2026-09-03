# CLI Reference

The Airports project ships two binaries:

- `airports` — the HTTP server (also includes operational subcommands).
- `airports-cli` — a terminal client that queries any running Airports server.

## `airports` — Server Binary

### Synopsis

```
airports [flags]
airports --service <subcommand>
airports --update check|yes|branch <name>
airports --version
```

Running `airports` with no flags starts the server in the foreground, creating defaults on first run.

### Flags

| Flag | Env | Description |
|------|-----|-------------|
| `--config DIR` | `CONFIG_DIR` | Configuration directory (default: OS-standard config dir) |
| `--data DIR` | `DATA_DIR` | Data directory (default: OS-standard data dir) |
| `--cache DIR` | — | Cache directory (default: OS-standard cache dir) |
| `--log DIR` | `LOG_DIR` | Log directory (default: OS-standard log dir) |
| `--backup DIR` | — | Backup directory (default: OS-standard backup dir) |
| `--pid FILE` | `PID_FILE` | PID file path (default: OS-standard PID path) |
| `--port PORT` | `PORT` | Override listen port (default: random 64000-64999, persisted after first run) |
| `--address ADDR` | `LISTEN` | Listen address (default: `0.0.0.0`) |
| `--baseurl PATH` | — | URL path prefix the router mounts under (default: `/`) |
| `--mode MODE` | `MODE` | `development` or `production` |
| `--daemon` | — | Detach from the terminal and run as a background daemon |
| `--debug` | `DEBUG` | Enable pprof, expvar, verbose request logs |
| `--color {auto\|yes\|no}` | — | Color output (default: `auto`) |
| `--lang CODE` | `LANG`/`LC_ALL` | Language code (e.g. en, es, zh, fr, ar, de, ja) |
| `--status` | — | Health check; exits 0 if healthy (used by Docker HEALTHCHECK) |
| `--version`, `-v` | — | Print version, commit, build date; exit |
| `--help`, `-h` | — | Show help; exit |
| `--maintenance` | — | Start in maintenance mode (read-only, serves maintenance page) |
| `--maintenance backup [path]` | — | Run a backup now |
| `--restore <path>` | — | Restore from a backup archive |

### Shell Integration

```
airports --shell completions [SHELL]  # print shell completions
airports --shell init [SHELL]         # print shell init command
airports --shell help                 # show shell help
```

`SHELL` is auto-detected from `$SHELL` when omitted.

### Update Commands

```
airports --update check        # check for updates
airports --update yes          # install available updates
airports --update branch NAME  # set update branch (stable, beta, daily)
```

### Service Subcommands

```
airports --service install     # install system service (systemd/launchd/SCM)
airports --service uninstall   # remove service unit + delete all data (confirms first)
airports --service start
airports --service stop
airports --service restart
airports --service reload      # reload configuration
airports --service status
airports --service enable      # enable on boot
airports --service disable
airports --service logs        # tail recent service logs
```

The binary auto-detects the host service manager: systemd, OpenRC, SysVinit, runit, launchd, FreeBSD rc, or Windows Service.

### Scheduler Subcommands

```
airports scheduler list                # list all scheduled tasks
airports scheduler show <id>           # show a task's detailed status
airports scheduler run <id>            # run a task immediately
airports scheduler enable <id>         # enable a task
airports scheduler disable <id>        # disable a task
airports scheduler history <id>        # show a task's execution history
```

### Examples

```bash
airports                                # start with defaults
airports --port 9000 --debug            # dev run on alt port with debug
airports --config /etc/apimgr/airports  # use alternate config directory
sudo airports --service install
sudo airports --service start
airports --backup /var/backups/airports/backup.tar.gz
airports --update check
```

## `airports-cli` — Client Binary

A standalone terminal client. It does not bundle any airport data of its own — the server is the source of truth.

### Synopsis

```
airports-cli [flags] <command> [args...]
```

### Commands

| Command | Description |
|---------|-------------|
| `search <query>` | Free-text search by name, city, or code |
| `get <code>` | Fetch a single airport by ICAO or IATA code |
| `nearby <lat> <lon> [n]` | N closest airports to a coordinate (optional result count) |
| `health` | Show server health status |
| `version` | Print client version, then the remote server's version/build info |

### Flags

| Flag | Env | Description |
|------|-----|-------------|
| `--server URL` | `AIRPORTS_SERVER_PRIMARY` | Server base URL (default: `http://localhost:80`) |
| `--token TOKEN` | `AIRPORTS_TOKEN` | API token for authenticated operations |
| `--config NAME` | — | Config profile name (default: `cli.yml`) |
| `--api-version VERSION` | — | API version path segment (default: `v1`) |
| `--format FORMAT` | — | Output format: `json`\|`yaml`\|`text` |
| `--json` | — | Output as JSON (default) |
| `--yaml` | — | Output as YAML |
| `--color {auto\|yes\|no}` | — | Color output (default: `auto`, respects `NO_COLOR`) |
| `--lang CODE` | `LANG`/`LC_ALL` | Language for output (default: config > `LANG`/`LC_ALL` env) |
| `--debug` | — | Enable debug output |
| `--help`, `-h` | — | Show this help |
| `--version`, `-v` | — | Show client version |

### Shell Integration

```
airports-cli --shell completions [SHELL]  # print shell completions
airports-cli --shell init [SHELL]         # print shell init command
airports-cli --shell help                 # show shell help
```

### Configuration

Server URL, token, color, and language are saved to `~/.config/apimgr/airports/cli.yml` (or the profile named by `--config`) on first run, with the server URL stored under the nested `server.primary` key. A saved value is never overwritten by a flag unless it was previously empty — delete the file or set `AIRPORTS_SERVER_PRIMARY`/`AIRPORTS_TOKEN` to override.

### Examples

```bash
airports-cli search "kennedy"
airports-cli get KJFK
airports-cli nearby 40.7128 -74.0060 5
airports-cli --server https://airports.example.com health
airports-cli --format yaml get JFK
airports-cli --shell completions bash > /etc/bash_completion.d/airports-cli
```

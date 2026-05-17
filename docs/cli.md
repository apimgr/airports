# CLI Reference

The Airports project ships two binaries:

- `airports` — the HTTP server (also includes operational subcommands).
- `airports-cli` — a terminal client that queries any running Airports server.

## `airports` — Server Binary

### Synopsis

```
airports [flags]
airports --service <subcommand>
airports --config <path>
airports --version
```

Running `airports` with no flags starts the server in the foreground, creating defaults on first run.

### Flags

| Flag | Env | Description |
|------|-----|-------------|
| `--config <path>` | `CONFIG` | Use alternate `server.yml` |
| `--port <n>` | `PORT` | Override listen port |
| `--address <ip>` | `ADDRESS` | Override bind address |
| `--mode <m>` | `MODE` | `development` or `production` |
| `--debug` | `DEBUG` | Enable pprof, expvar, verbose request logs |
| `--status` | — | Health check; exits 0 if healthy (used by Docker HEALTHCHECK) |
| `--version` | — | Print version, commit, build date; exit |
| `--maintenance` | — | Start in maintenance mode (read-only, serves maintenance page) |
| `--update` | — | Self-update from GitHub releases |
| `--backup [path]` | — | Run a backup now; default destination is `{data_dir}/backups/` |
| `--restore <path>` | — | Restore from a backup archive |
| `--config show` | — | Print the resolved configuration with secrets masked |
| `--config validate` | — | Lint `server.yml` without starting the server |

### Service Subcommands

```
airports --service install     # install system service (systemd/launchd/SCM)
airports --service uninstall   # remove service unit (preserves config/data)
airports --service start
airports --service stop
airports --service restart
airports --service status
airports --service enable      # enable on boot
airports --service disable
airports --service logs        # tail recent service logs
```

The binary auto-detects the host service manager: systemd, launchd, Windows SCM, OpenRC, or FreeBSD rc.

### Examples

```bash
airports                                # start with defaults
airports --port 9000 --debug            # dev run on alt port with debug
airports --config /etc/airports/server.yml
sudo airports --service install
sudo airports --service start
airports --backup /var/backups/airports/
airports --update
```

## `airports-cli` — Client Binary

A standalone terminal client. First run prompts for the default server URL and saves it to `~/.config/airports/cli.yml`.

### Synopsis

```
airports-cli <command> [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `airport <icao\|iata>` | Fetch a single airport |
| `search <query>` | Free-text search by name, city, country |
| `nearby --lat <f> --lon <f> [--radius <km>]` | Airports within a radius |
| `within --bbox <minLat,minLon,maxLat,maxLon>` | Airports inside a bounding box |
| `nearest --lat <f> --lon <f> [--n <count>]` | N closest airports |
| `country <iso2>` | List airports by ISO 3166-1 alpha-2 country code |
| `about` | Show remote server version, build, and dataset info |
| `config show` | Print resolved client config |
| `config set server.url <url>` | Update the default server URL |
| `completion <shell>` | Print shell completion script (`bash`, `zsh`, `fish`) |
| `--version` | Print client version |

### Output Formats

All commands accept:

- `--json` — JSON output
- `--yaml` — YAML output
- (default) — human-friendly table when stdout is a TTY, JSON otherwise

The client respects `NO_COLOR` and `FORCE_COLOR` environment variables.

### Examples

```bash
airports-cli airport KJFK
airports-cli search "Tokyo"
airports-cli nearby --lat 40.6398 --lon -73.7789 --radius 50
airports-cli country US --json | jq '.[] | .iata_code'
airports-cli completion bash > /etc/bash_completion.d/airports-cli
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (bad flags) |
| 3 | Server unreachable |
| 4 | Not found |
| 5 | Rate-limited |

# Binary Rules (PART 7, 8, 32)

Cheatsheet — see AI.md PART 7 (line 9839), PART 8 (line 10485), PART 32 (line 45677).

## Single Static Binary

| Requirement | Value |
|-------------|-------|
| Type | Single static binary |
| Assets | Embedded via Go `embed` package |
| Runtime deps | None |
| Build | `CGO_ENABLED=0` |
| Libraries | Pure Go only — no CGO |

## Default Behavior (No Args)

1. Detect OS, resolve dirs.
2. Auto-create `server.yml` with defaults if absent.
3. Auto-create required directories.
4. Show banner: name, version, commit, build date, listen URLs.
5. Start server in foreground.
6. PID file enabled by default.

## Signals

- SIGTERM, SIGINT → graceful shutdown (drain connections, close DB, write PID-removal).
- SIGHUP → reload config + reopen log files.
- Container STOPSIGNAL: `SIGRTMIN+3` (s6/tini convention).

## Embedded Assets

| Asset | Source |
|-------|--------|
| Templates | `src/server/template/` |
| Static files | `src/server/static/` |
| App data | `src/data/` (JSON, CSV) |

## External Data (NEVER Embedded)

GeoIP, blocklists, CVE feeds, Trivy DB — downloaded on first run, kept current by built-in scheduler. Stored in `{data_dir}/security/`.

If download fails: log warning, continue (graceful degradation).

## Server CLI Flags (PART 8)

| Flag | Purpose |
|------|---------|
| `--config <path>` | Use alternate `server.yml` |
| `--port <n>` | Override listen port |
| `--address <ip>` | Override bind address |
| `--mode <m>` | development \| production |
| `--debug` | Enable debug endpoints |
| `--status` | Health check (used by HEALTHCHECK) — exit 0 if healthy |
| `--version` | Print version + commit + build date, exit |
| `--maintenance` | Start in maintenance mode (read-only) |
| `--service install/uninstall/start/stop/restart/status` | systemd/launchd integration |
| `--update` | Self-update from GitHub releases |
| `--backup <path>` | Run a backup now |
| `--restore <path>` | Restore from archive |

All flags also exposed as env vars (uppercase, underscore): `--port` ↔ `PORT`.

## Client Binary (PART 32) — `airports-cli`

- Separate binary, same repo (`./src/client`).
- Subcommand pattern: `airports-cli <command> [flags]`.
- First-run wizard: prompts for default server URL, saves to `~/.config/airports/cli.yml`.
- Output modes: text (TTY), JSON (`--json`), YAML (`--yaml`).
- Respects `NO_COLOR` env var.
- All API operations from the server are reachable via subcommands.

## Build Commands

```
make dev     # Quick build to $TEMP_DIR
make local   # Build with version info to binaries/
make build   # Full cross-platform build
make test    # Unit tests
```

All Make targets use Docker `golang:alpine` internally. Never run `go` on host.

## Cross-Compilation Targets

`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `freebsd/amd64`.

## Display Environment Detection

All binaries detect: TTY vs pipe, terminal width, color support (`NO_COLOR`, `FORCE_COLOR`), headless. Adapt banner, prompts, output formatting accordingly.

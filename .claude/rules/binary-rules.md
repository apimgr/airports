# Binary Requirements, Server CLI, Client Rules (PART 7, 8, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Ship anything but a single static binary — `CGO_ENABLED=0`, pure Go, no runtime deps
- Embed security databases (GeoIP/blocklists/CVE/Trivy) in the binary — always downloaded to `{data_dir}/security/`
- Hand-roll flag parsing for the server binary — stdlib `flag` only (single-command, no subcommands)
- Implement UI mode flags (`--tui`, `--cli`, `--gui`, `--mode tui/cli/gui`) — display mode is ALWAYS auto-detected; `--mode` is production/development only
- Add short flags beyond `-h`/`-v` — everything else is long-form only
- Use Electron/web-based GUI for the CLI — native toolkit only (GTK4/Qt6, Cocoa, Win32)
- Put CLI config/data/cache/logs in system directories — CLI runtime state is ALWAYS user-scope, even when invoked by root
- Resolve `~`/`$HOME` after the server's privilege drop — service account HOME points at `{data_dir}`
- Create a PID file inside a container — container runtimes supervise the process; PIDs are namespace-local
- Attempt GUI mode over SSH/mosh — remote sessions always use TUI, even with X11 forwarding available
- Hardcode `localhost`/`127.0.0.1`/`0.0.0.0` in displayed URLs — always detect `{proto}`/`{fqdn}`/`{port}`
- Skip SHA-256 verification on CLI/server self-update binary downloads

## CRITICAL - ALWAYS DO

- Detect display environment (GUI/TUI/CLI/Headless) via `display.DetectDisplayEnv()`; honor `TERM=dumb` (no ANSI, no emoji, no TUI, ASCII fallback) and `NO_COLOR` (disables colors + emojis, not bold/unicode)
- Show the ACTUAL (possibly renamed) binary name in `--help`/`--version`/error messages; hardcode `{project_name}` only for internal identifiers (User-Agent, default paths, config keys)
- Follow the exact server startup sequence: parse flags → immediate-exit flags → `--service`/`--maintenance`/`--update` subcommands → resolve+cache all paths once (mode locked from EUID at start) → root setup (user, dirs, chown, bind privileged ports) → drop privileges → user-mode dirs → logging → PID file → config → DB → scheduler → Tor → HTTP listeners → signal handlers → serve
- Detect stale PID files every startup (dead process or PID reuse → remove and continue); skip PID file entirely in containers
- Handle SIGTERM/SIGINT/SIGQUIT/SIGRTMIN+3 as graceful shutdown, SIGHUP as ignored (auto config reload), SIGUSR1 reopen logs, SIGUSR2 status dump (Unix); Windows: `os.Interrupt` only
- Give ALL binaries built-in `--shell completions [SHELL]` / `--shell init [SHELL]` (auto-detect `$SHELL` if omitted) — no separate completion files
- Give the CLI a full TUI (bubbletea, default when interactive+no command) AND native GUI AND scriptable CLI mode, each with 100% server feature coverage; CLI is the only binary with a setup wizard
- Keep `cli.yml` at `0600`/user-only; save `--server`/`--token` flag values to config ONLY when current value is empty or invalid, never overwrite a valid one
- Resolve URLs via `{proto}`/`{fqdn}`/`{port}` per-request (reverse-proxy headers > DOMAIN env > hostname > public IP > localhost); strip `:80`/`:443`

## Key Rules Summary

| Area | Rule |
|------|------|
| Binary naming | Server `{project_name}`, Client `{project_name}-cli`; UA hardcoded `{project_name}[-cli]/{version}` |
| Shared flags (all binaries) | `--help`, `--version`, `--shell`, `--debug`, `--color {auto\|yes\|no}`, `--lang` |
| Server flags | `--mode`, `--config`, `--data`, `--cache`, `--log`, `--backup`, `--pid`, `--address`, `--port`, `--baseurl`, `--status`, `--service`, `--daemon`, `--maintenance`, `--update` |
| Server has no subcommands | Uses stdlib `flag` package, not cobra/viper |
| Client uses cobra/viper | Multi-command CLI; TUI via bubbletea, GUI via native toolkit |
| Env var fallbacks | `--config`→`CONFIG_DIR`, `--data`→`DATA_DIR`, `--log`→`LOG_DIR`, `--pid`→`PID_FILE`, `--port`→`PORT`, `--address`→`LISTEN`, `--mode`→`MODE`, plus `DATABASE_DIR`, `BACKUP_DIR` |
| Config priority | CLI flag > env var > config file > default |
| `--status` exit codes | 0 healthy, 1 unhealthy (used by Docker healthcheck) |
| Commands anyone can run | `--help`, `--version`, `--status`, `--update check` |
| Daemonize | Default foreground; `--service start` auto-detects service manager and ignores `--daemon`/config; `--daemon` only for manual/cron start (Unix only, Windows ignores it) |
| CLI display mode | GUI (local display) > TUI (interactive terminal, SSH/mosh forced here) > CLI (args/piped) > Headless (error) |
| CLI first run | Setup wizard (server URL, test connection, save config) — no WebUI fallback |
| CLI config paths | Unix `~/.config/{internal_org}/{internal_name}/cli.yml`; Windows `%APPDATA%\...\cli.yml`; data/cache/log under XDG/AppData equivalents |
| `--config` flag resolution | bare name → `{config_dir}/name.yml`; absolute/`~` path used as-is; `.yml`/`.yaml` auto-detected |
| Boolean parsing | `config.ParseBool()`/`IsTruthy()` only, never `strconv.ParseBool()` |
| Flag syntax | Both `--flag=value` and `--flag value` accepted everywhere |
| Server-side CLI update | `/api/autodiscover` → `cli_versions`/`cli_min_version`; download `{base}/cli/binaries/{name}-{os}-{arch}`; verify SHA-256; atomic swap; re-exec |
| Official site source order | `site.txt` > `IDEA.md official_site` > README fallback > ask user |

For complete details, see AI.md PART 7, PART 8, PART 32.

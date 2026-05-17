# Service Rules (PART 23, 24)

Cheatsheet — see AI.md PART 23 (line 32844), PART 24 (line 33758).

## Privilege Escalation

- Binary runs UNPRIVILEGED by default.
- Privilege required only for:
  - Binding ports < 1024 (or on Linux: needs `CAP_NET_BIND_SERVICE`).
  - Installing system service (systemd unit, launchd plist, Windows service).
  - Writing system-wide config (`/etc/{internal_name}`).
- When privilege required: binary detects + re-execs via `sudo` / `pkexec` / Windows UAC. NEVER hardcodes `sudo` path — uses `which sudo`.
- On Linux: prefer `setcap cap_net_bind_service=+ep` over running as root for port 80.
- After bind, immediately drop to unprivileged user (configured: `user:` / `group:` in `server.yml`).

## Service Subcommand (`airports --service ...`)

| Subcmd | Effect |
|--------|--------|
| `install` | Install unit/plist/service; do NOT start |
| `uninstall` | Remove unit/plist/service; preserve config and data |
| `start` | Start the service |
| `stop` | Stop the service |
| `restart` | Restart |
| `status` | Print service state (running, enabled, PID, uptime) |
| `enable` | Mark for boot-time start |
| `disable` | Remove boot-time start |
| `logs` | Tail recent logs from journal/Console.app/Event Viewer |

All subcommands self-detect platform (systemd, launchd, Windows SCM, OpenRC, FreeBSD rc).

## systemd Unit (Linux)

- Path: `/etc/systemd/system/{internal_name}.service`.
- `Type=notify` if binary supports sd_notify; else `Type=simple`.
- `Restart=on-failure`, `RestartSec=5s`.
- Hardening: `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true`, `ReadWritePaths={data_dir} {log_dir}`.
- `User=` and `Group=` set to configured unprivileged user.
- `StateDirectory=`, `LogsDirectory=`, `CacheDirectory=` set.

## launchd Plist (macOS)

- Path: `/Library/LaunchDaemons/{plist_name}.plist` (system) or `~/Library/LaunchAgents/{plist_name}.plist` (user).
- Bundle ID: `io.github.{project_org}.{internal_name}` → `io.github.apimgr.airports`.
- `RunAtLoad=true`, `KeepAlive=true`.
- Stdout/stderr → `{log_dir}/stdout.log`, `{log_dir}/stderr.log`.

## Windows Service

- Service name: `{internal_name}`.
- Display name: `{app_name}` from IDEA.md.
- Description from README first paragraph.
- Auto-start; restart on failure (3 retries, then alert).
- Uses `golang.org/x/sys/windows/svc`.

## OpenRC / FreeBSD rc

- OpenRC script in `/etc/init.d/{internal_name}` (created by `--service install` when OpenRC detected).
- FreeBSD rc script in `/usr/local/etc/rc.d/{internal_name}`.

## Service Lifecycle

- Installation NEVER auto-starts. User must run `--service start` (clear and predictable).
- Uninstallation NEVER deletes config or data. User must remove `{config_dir}`/`{data_dir}` manually.
- All operations idempotent — re-running `install` updates the unit file rather than failing.

## Logs

- Service stdout/stderr → journal (Linux), Console.app (macOS), Event Viewer (Windows).
- Application logs always go to `{log_dir}/*.log` regardless of platform.

## Health

- `--status` exit codes used by service managers:
  - 0 = healthy
  - non-zero = unhealthy (used by Docker HEALTHCHECK and systemd watchdog)

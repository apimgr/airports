# Configuration

The Airports server reads configuration from four sources, in order of precedence:

1. **Command-line flags** (`--port 9000`)
2. **Environment variables** (`PORT=9000`)
3. **Config file** (`server.yml`)
4. **Built-in defaults**

Every setting is expressible at all four layers.

## Config File Location

| Platform | Default path |
|----------|--------------|
| Linux (system) | `/etc/airports/server.yml` |
| Linux (user) | `~/.config/airports/server.yml` |
| macOS | `~/Library/Application Support/airports/server.yml` |
| Windows | `%APPDATA%\airports\server.yml` |
| Container | `/config/airports/server.yml` |

Override the config directory with `--config /path/to/dir` or `CONFIG_DIR=/path/to/dir`
(the server always reads `server.yml` from within that directory).

The file is auto-generated with sane defaults on first run.

## Modes

| Mode | When | Effect |
|------|------|--------|
| `development` | `MODE=development` or `--mode development` | Allows `localhost`, `.local`, `.test` hosts; verbose logs; relaxed CORS; pprof available with `--debug` |
| `production` | `MODE=production` or `--mode production` | Strict CORS; minimal logs; no debug endpoints |

`DEBUG=true` is independent of `MODE` and enables pprof/expvar/detailed-request-logging. Never default-on in production.

## Core Settings

```yaml
server:
  mode: production          # development | production
  address: 0.0.0.0          # bind address
  port: 8080                # listen port (containers always 80 internally)
  domain: ""                # comma-separated allowed hosts (auto-detect behind reverse proxy)
  timezone: America/New_York

paths:
  config_dir: ~/.config/airports
  data_dir:   ~/.local/share/airports
  log_dir:    ~/.local/state/airports/log
  cache_dir:  ~/.cache/airports
  database_dir: ~/.local/share/airports/db/sqlite

logging:
  format: json              # json | text
  level: info               # debug | info | warn | error
  files:
    access: access.log
    error:  error.log

rate_limit:
  enabled: true
  default_per_minute: 100
  burst: 20
  endpoint_overrides:
    /api/v1/airports/search: 200
    /api/v1/airports/nearby: 100

geoip:
  enabled: true
  provider: ip-location-db
  refresh_cron: "0 4 * * *"        # daily 04:00
  databases:
    asn:     { enabled: true }
    country: { enabled: true }
    city:    { enabled: true }
    whois:   { enabled: true }

tls:
  enabled: false
  cert_file: ""              # operator-provided cert
  key_file:  ""              # operator-provided key
  acme:
    enabled: false           # Let's Encrypt
    email: ""
    domains: []
    challenge: http-01       # http-01 | dns-01

tor:
  enabled: true              # binary owns Tor entirely
  publish_onion: true

scheduler:
  enabled: true
  jobs:
    geoip_refresh:    { enabled: true, cron: "0 4 * * *" }
    blocklist_refresh:{ enabled: true, cron: "30 4 * * *" }
    cache_cleanup:    { enabled: true, cron: "0 * * * *" }
    tls_renewal:      { enabled: true, cron: "0 */12 * * *" }
    backup:           { enabled: false, cron: "0 3 * * *" }

backup:
  enabled: false
  directory: ~/.local/share/airports/backups
  retain: 7
```

## Environment Variables

The variables below are the ones the server actually reads (this list is
not a generic "any config key maps to an env var" mapping).

| Env Var | Maps To |
|---------|---------|
| `MODE` | `server.mode` |
| `PORT` | `server.port` |
| `LISTEN` | `server.address` |
| `CONFIG_DIR` | `paths.config_dir` |
| `DATA_DIR` | `paths.data_dir` |
| `LOG_DIR` | `paths.log_dir` |
| `CACHE_DIR` | `paths.cache_dir` |
| `BACKUP_DIR` | `paths.backup_dir` |
| `DATABASE_DIR` | SQLite database directory (overrides the OS-default `paths.db_dir`) |
| `PID_FILE` | `paths.pid_file` |
| `DEBUG` | (independent toggle for pprof/verbose logs) |
| `NO_COLOR` | (disables ANSI color in CLI/log output) |
| `DOMAIN` | `server.fqdn` (comma-separated list, first entry wins; highest-priority override) |
| `TZ` | Process/server timezone (IANA name, e.g. `America/New_York`); also the fallback when `server.scheduler.timezone` is unset |
| `SMTP_HOST` | `server.notifications.email.smtp.host` |
| `SMTP_PORT` | `server.notifications.email.smtp.port` |
| `SMTP_USERNAME` | `server.notifications.email.smtp.username` |
| `SMTP_PASSWORD` | `server.notifications.email.smtp.password` |
| `SMTP_TLS` | `server.notifications.email.smtp.tls` |
| `SMTP_FROM_NAME` | `server.notifications.email.from.name` |
| `SMTP_FROM_EMAIL` | `server.notifications.email.from.email` |

`LOG_LEVEL` and `ENABLE_TOR` are intentionally **not** implemented: AI.md's
canonical env var table (PART 8 "Environment Variables") does not list
`LOG_LEVEL` at all (log level comes from `server.logging.level` only), and
PART 31 states explicitly "No `ENABLE_TOR` flag needed" — Tor auto-enables
whenever the `tor` binary is present.

## No `.env` Files

Configuration is `server.yml` + env vars + flags. `.env` files are **never** read, never created, and never required by any container or systemd unit. Docker Compose ships with hardcoded sane defaults — edit the compose file directly to change them.

## Boolean Parsing

All boolean values use a tolerant parser that accepts: `true`, `false`, `yes`, `no`, `on`, `off`, `1`, `0`, `enabled`, `disabled`, and their case variants.

## Validation

The server validates `server.yml` strictly on startup:

- Unknown keys are rejected.
- Out-of-range values (e.g., port > 65535) refuse to start.
- Missing files referenced by paths cause an error with a clear message.

Run `airports --config validate` to lint without starting the server.

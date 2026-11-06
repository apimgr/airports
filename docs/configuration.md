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
| Linux (system) | `/etc/apimgr/airports/server.yml` |
| Linux (user) | `~/.config/apimgr/airports/server.yml` |
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

The optional `server.baseurl` setting serves every route below a path prefix.
Use `/` for the site root or a normalized path such as `/myproject`; trusted
reverse proxy prefix headers take precedence when the request comes from a
configured trusted proxy. The `--baseurl` flag overrides the config value.

```yaml
server:
  mode: production          # development | production
  lang: auto                # output language: auto, or a supported locale code
  baseurl: /                # URL path prefix; defaults to /
  i18n:
    enabled: true
    default_language: en
    available_languages: [en, es, zh, fr, ar, de, ja]
    fallback_language: en
    cookie_name: lang
    cookie_max_age: 365d
  session:
    cookie_name: session
    max_age: 0s               # browser-session lifetime
    secure: auto              # trusted request scheme; always secure for Tor
    http_only: true
    same_site: lax
  address: 0.0.0.0          # bind address
  port: 8080                # listen port (containers always 80 internally)
  domain: ""                # comma-separated allowed hosts (auto-detect behind reverse proxy)
  timezone: America/New_York

`server.url_detection` controls request-aware URL resolution and trusted proxy
domain learning. Forwarded host and protocol values are considered only when
the request peer is a configured trusted proxy; invalid hosts and untrusted
forwarded headers are ignored. Tor requests detected through the dedicated
loopback listener's validated PROXY-protocol circuit metadata keep their HTTP
URL; a client-controlled `.onion` Host header does not establish Tor identity.
Learning is bounded by `sample_window` and
`min_samples`; `live_reload` controls whether an observed trusted host is used
immediately, and `log_changes` controls domain/protocol change logs.

`server.session` sets attributes for cookies issued by browser-session
features. The default `max_age: 0s` creates a browser-session cookie. The
`secure: auto` setting follows HTTPS as resolved from TLS or trusted proxy
headers and remains secure for trusted Tor requests. These settings do not change the separate
CSRF, consent, or JavaScript-readable preference-cookie policies.

```yaml
server:
  url_detection:
    learning: true
    min_samples: 3
    sample_window: 5m
    log_changes: true
    live_reload: true
```

paths:
  config_dir: ~/.config/apimgr/airports
  data_dir:   ~/.local/share/apimgr/airports
  log_dir:    ~/.local/state/airports/log
  cache_dir:  ~/.cache/apimgr/airports
  database_dir: ~/.local/share/apimgr/airports/db/sqlite

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
    /api/{api_version}/airports/search: 200
    /api/{api_version}/airports/nearby: 100

`server.contact` is the canonical recipient tree for server notifications,
security reports, abuse reports, and contact form submissions. The admin
address and webhook URLs are private; the security and abuse addresses may be
shown on public security/contact pages according to their fallback rules.

```yaml
server:
  contact:
    admin:
      email: "admin@{fqdn}"
      webhooks:
        telegram: ""
        discord: ""
        slack: ""
        mattermost: ""
        pushover: ""
        gotify: ""
        generic: ""
    security:
      email: "security@{fqdn}"
      webhooks: {}
    abuse:
      email: ""
      webhooks: {}
    general:
      email: ""
      webhooks: {}
```

An empty role email falls back to the admin email, except public abuse contact
display, which falls back to the general email and never exposes admin. Webhook
destinations use the same per-role fallback order. Keep webhook URLs private;
debug config output redacts them.

The contact page uses a self-hosted arithmetic challenge when
`server.pages.contact.captcha` is `simple`. Submissions are sent to the
effective general email address; SMTP must be enabled and configured.

The server's CLI binary endpoint reads matching versioned assets and
`sha256.txt` from the `apimgr/airports` GitHub Release. Downloads are public by
default; require the operator token with:

```yaml
cli:
  binary_download:
    require_auth: true
```

Analytics is disabled by default. Configure one provider under
`server.tracking`; the server validates its ID and URL and disables tracking
with a warning if the configuration is invalid. Analytics loads only when the
effective consent state allows it, and is always disabled for Tor requests.

```yaml
server:
  tracking:
    type: ""                 # google, matomo, piwik, owa, fathom, plausible, umami, simple, cloudflare
    id: ""
    url: ""
  privacy:
    consent:
      show_until_acknowledged: true
      default_enabled: true
      show_preferences: true
      preferences_text: "Manage Preferences"
```

`url` is required for Matomo, Piwik, Open Web Analytics, and Umami; it is
optional for Fathom and Plausible. Google Analytics, Simple Analytics, and
Cloudflare Web Analytics use their hosted endpoints. The consent banner stores
the visitor's analytics choice in the `cookie_consent` cookie. Tor requests do
not load third-party analytics scripts.

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
  binary: ""                 # auto-detected; Tor starts whenever the binary is found
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
  directory: ~/.local/share/apimgr/airports/backups
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

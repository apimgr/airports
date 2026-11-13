# Configuration

The Airports server reads configuration from four sources, in order of precedence:

1. **Command-line flags** (`--port 9000`)
2. **Environment variables** (`PORT=9000`)
3. **Config file** (`server.yml`)
4. **Built-in defaults**

Only settings with a documented command-line flag or environment-variable mapping can be set at those layers; all other settings are read from `server.yml` or use their built-in defaults.

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
  # Application mode: development or production.
  mode: production
  # Output language: auto or a supported locale code.
  lang: auto
  # URL path prefix; defaults to /.
  baseurl: /
  i18n:
    enabled: true
    default_language: en
    available_languages: [en, es, zh, fr, ar, de, ja]
    fallback_language: en
    cookie_name: lang
    cookie_max_age: 365d
  session:
    cookie_name: session
    # Browser-session lifetime.
    max_age: 0s
    # Follows the trusted request scheme and remains secure for Tor.
    secure: auto
    http_only: true
    same_site: lax
  # Bind address.
  address: 0.0.0.0
  # Listen port; containers use internal port 80.
  port: 8080
  # Comma-separated allowed hosts; auto-detected behind a reverse proxy.
  domain: ""
  timezone: America/New_York
```

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

```yaml
server:
  logs:
    level: info
    access:
      filename: access.log
      format: apache
    error:
      filename: error.log
      format: iso8601
  rate_limit:
    enabled: true
    read:
      requests: 120
      window: 60
    write:
      requests: 10
      window: 60
    health:
      requests: 120
      window: 60
    global_burst: 240
```

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
    dpo:
      name: ""
      email: ""
    general:
      email: ""
      webhooks: {}
```

An empty role email falls back to the admin email, except public abuse contact
display, which falls back to the general email and never exposes admin. Webhook
destinations use the same per-role fallback order. Keep webhook URLs private;
debug config output redacts them.

The public `/server/dpo` page and `/api/{api_version}/server/dpo` response use
the configured DPO name and email. If the DPO email is empty, they fall back to
the general contact email; they never expose the admin email. The HTML page
always links to the privacy policy and contact form.

The contact page requires a CAPTCHA on every submission. The default
`simple` provider uses a self-hosted arithmetic challenge and also applies to
Tor and I2P requests. Clearnet requests may use reCAPTCHA v2 or hCaptcha;
those providers require both a public site key and a private secret key.
Provider verification has a five-second timeout. The secret key is redacted
from debug configuration output.

The AI discovery document is generated at `/.well-known/llms.txt` and `/llms.txt`
by default. Endpoint paths are collected from the registered API routes, and
the current request host is used when building absolute URLs.

```yaml
web:
  llms:
    enabled: true
    include_endpoints: true
    include_schemas: false
    custom_sections: []
  api_contact: ""
```

`api_contact` is shown in the discovery document and falls back to the general
contact email when empty. Custom sections are appended as Markdown.

```yaml
server:
  pages:
    contact:
      enabled: true
      captcha: simple             # simple | recaptcha | hcaptcha
      captcha_site_key: ""        # required for recaptcha and hcaptcha
      captcha_secret_key: ""      # required for recaptcha and hcaptcha
      success_message: "Thank you for your message. We'll respond soon."
```

Submissions are sent to the effective general email address; SMTP must be
enabled and configured.

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
    # Provider: google, matomo, piwik, owa, fathom, plausible, umami, simple, or cloudflare.
    type: ""
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

```yaml
server:
  geoip:
    enabled: true
  ssl:
    enabled: false
    cert: ""
    key: ""
    letsencrypt:
      enabled: false
      email: ""
      challenge: http-01
  scheduler:
    enabled: true
    timezone: America/New_York
    tasks:
      geoip_update:
        enabled: true
        schedule: "0 3 * * 0"
      backup:
        enabled: false
        schedule: "0 2 * * *"
  tor:
    binary: ""
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

# Configuration

The Airports server merges configuration in this order, from lowest to highest precedence:

1. **Built-in defaults**
2. **`server.yml`** for node-specific settings
3. **Area drop-ins** in `yaml.d/`, sorted by filename
4. **`secrets.yml`** for credentials and generated tokens
5. **Environment variables and command-line flags**, where the setting has a documented override

Only settings with a documented command-line flag or environment-variable mapping can be set at those layers; other settings are read from the matching config file or use their built-in defaults. Credentials and generated tokens live in `secrets.yml`.

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

The main file is auto-generated with node-specific settings on first run. Other
settings belong in named files under `{config_dir}/yaml.d/` (for example,
`branding.yml`, `web.yml`, `security.yml`, and `metrics.yml`). The operator
token is generated with the `adm_` prefix and stored separately in
`secrets.yml`. The config directory and its private files use owner-only
permissions. Public branding and operator-managed page content belong in
`{config_dir}/yaml.d/branding.yml`.

An older `server.yml` containing area settings is accepted and split into the
matching area files at startup. Invalid individual values, explicit `null`
values, and duplicate keys fall back to defaults with a warning. A YAML file
that cannot be parsed is ignored; the last successfully loaded configuration
stays active during reload, or built-in defaults are used on first startup.

## Modes

| Mode | When | Effect |
|------|------|--------|
| `development` | `MODE=development` or `--mode development` | Allows development hosts; verbose logs; relaxed CORS; debug endpoints still require `--debug` or `DEBUG=true` |
| `production` | `MODE=production` or `--mode production` | Strict CORS; minimal logs; debug endpoints remain off unless explicitly enabled |

`DEBUG=true` is independent of `MODE` and enables diagnostics and detailed request logging. Debug routes exist only under `/debug/*`, require the operator Bearer token, and pprof/expvar require their respective settings. Never enable them by default in production.

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
    default_language: en
    available_languages: [en, es, fr, ar, de, it, ja, zh]
  session:
    cookie_name: session
    # Browser-session lifetime.
    max_age: 0s
    # Follows the trusted request scheme and remains secure for Tor.
    secure: auto
    http_only: true
    same_site: lax
  # Bind address.
  address: "[::]"
  # With no port configured, the server selects and persists a 64xxx port.
  # Comma-separated allowed hosts; auto-detected behind a reverse proxy.
  domain: ""
  timezone: America/New_York
```

Only `default_language` and `available_languages` are configurable. `default_language` selects the language for operator email; web requests fall back to English. Internationalization is always enabled, and the language preference cookie is fixed as `lang` with a one-year lifetime.

`server.url_detection` controls request-aware URL resolution and trusted proxy
domain learning. Forwarded host and protocol values are considered only when
the request peer is a configured trusted proxy; invalid hosts and untrusted
forwarded headers are ignored. Tor requests detected through the dedicated
loopback listener's validated PROXY-protocol circuit metadata keep their HTTP
URL; a client-controlled `.onion` Host header does not establish Tor identity.
Learning is bounded by `sample_window` and
`min_samples`; `live_reload` controls whether an observed trusted host is used
immediately, and `log_changes` controls domain/protocol change logs.

Owner-token policy is stored in `yaml.d/security.yml`. Primary and linked
tokens are encrypted at rest with `server.security.encryption_key`; the
owner-token cookie name is fixed per project. The default cookie lifetime is
one year and slides forward when a valid primary token is used.

Closed security reports are removed after `server.security.reports.retention_days`
(default `365`). Open reports are retained. This value controls the daily
`security_cleanup` task.

```yaml
server:
  security:
    reports:
      retention_days: 365
```

```yaml
server:
  tokens:
    max_linked: 25
    cookie_max_age: 1y
    idle_purge_days: 0
```

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
    read:
      requests: 120
      window: 60
    write:
      requests: 10
      window: 60
    health:
      requests: 120
      window: 60
    strict:
      requests: 10
      window: 900
    auth_failures:
      requests: 5
      window: 900
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
links to the privacy policy and `/server/security`; it never links directly to
the report form.

The contact page requires a CAPTCHA on every submission. The default
`simple` provider uses a self-hosted arithmetic challenge and also applies to
Tor and I2P requests. Clearnet requests may use reCAPTCHA v2 or hCaptcha;
those providers require both a public site key and a private secret key.
Simple challenges are stored as expiring hashes in the application database
and are single-use across server instances. Provider verification has a
five-second timeout. Contact submission uses a review step before any email is
sent. The secret key is redacted from debug configuration output and stored in
`secrets.yml`.

The AI discovery document is generated at `/.well-known/llms.txt` and `/llms.txt`
by default. Endpoint paths are collected from the registered API routes, and
the current request host is used when building absolute URLs.

Brand title, tagline, description, SEO tags, and the default theme live in
`yaml.d/branding.yml`. The optional `web.ui.accent` value accepts `#RRGGBB`;
when valid, the server generates contrast-adjusted colors in
`/static/css/accent.css`. Logo, favicon, and Open Graph images can be HTTPS
URLs, validated image data URIs, or file names under `assets/`. Local image
files require `assets/` mode `0700` and file mode `0600`; remote images are
fetched over HTTPS with public-address, redirect, MIME, and size checks and
cached under the data directory.

Configured images are served through opaque same-origin URLs. Raster variants
are generated on demand and cached in the private branding cache. Logo variants
at 50px and 200px widths preserve the source aspect ratio. Favicon variants at
16, 32, 48, 180, 192, and 512px are square and fit the source on a transparent
background without stretching or cropping. The Open Graph variant is exactly
1200×630 pixels, scaled to cover and center-cropped. SVG and ICO files remain
in their original format.

The built-in `/server/terms` page includes acceptance, acceptable use, liability,
changes, and governing law. Its default governing-law text names the United
States without regard to conflict-of-law rules. Operators can replace
the page through `server.branding.content.terms`.

## Public help, contact, and MCP pages

```yaml
server:
  pages:
    help:
      # Project-specific Markdown appended to the built-in help page.
      content: ""
      # Project-specific Markdown appended to the base MCP help page.
      mcp: ""
    contact:
      enabled: true
      captcha: simple
      captcha_site_key: ""
      # Kept in secrets.yml; never returned by the config API.
      captcha_secret_key: ""
      success_message: "Thank you for your message. We'll respond soon."
web:
  llms:
    enabled: true
    include_endpoints: true
    include_schemas: false
    custom_sections: []
```

The help page and its JSON mirror use `server.pages.help.content`. The MCP
setup page and its JSON mirror use `server.pages.help.mcp`; the built-in
sections, endpoint URL, aliases, and tool list remain generated from the
running server. See [`api.md`](./api.md#model-context-protocol) for client
connection details.

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
      antispam:
        honeypot: true
        min_fill_seconds: 3
        max_age_seconds: 43200
        per_ip: 5
        per_email: 3
        global: 60
        window: 1h
        max_links: 10
        reject_markup: true
        blocked_words: []
        blocked_email_domains: []
        strikes: 3
        ban_hours: 24
        review_min_seconds: 2
        review_max_seconds: 900
        duplicate_window_hours: 24
      success_message: "Thank you for your message. We'll respond soon."
```

The report form offers abuse, DMCA, and privacy requests; security reports use
the separate security-report flow. Each submission passes a server-rendered
review and confirmation step before delivery. Abuse and DMCA reports resolve
to the abuse recipient, privacy requests to the DPO recipient, and configured
webhooks follow the same per-role fallback order. SMTP must be enabled for
email delivery. CAPTCHA verification fails closed; Tor and I2P always use the
built-in simple challenge, and clearnet reCAPTCHA/hCaptcha forms keep the
simple challenge as a no-JavaScript fallback.

The antispam values control minimum form-fill time, token lifetime, accepted
submission limits, allowed same-site links, markup rejection, blocked words
and email domains, strike bans, review timing, and duplicate suppression.
Setting an individual numeric limit to `0` disables that rule. IP and strike
limits are skipped for Tor and I2P requests; per-email and global limits still
apply. Rejections are logged as `contact.spam_blocked` with only a reason and
client IP, never submitted content.

The CLI binary endpoint serves only locally provisioned release files from
`{data_dir}/cli/`. Configure the directory and optional authentication in
`yaml.d/maintenance.yml`; the server validates each binary against
`sha256.txt` and publishes the version from `version.txt`:

```yaml
cli:
  binary_download:
    dir: "{data_dir}/cli"
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
      policy:
        version: "1"
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

The built-in `disk_check` task runs every five minutes by default and checks
the filesystems containing the data, log, and backup directories. Low-disk
notifications are deduplicated per volume, repeat at most every 24 hours while
usage remains high, and reset after usage falls five percentage points below
the configured threshold.

```yaml
server:
  notifications:
    disk_low:
      enabled: true
      threshold_percent: 85
      check_interval: 5m
      repeat: 24h
  scheduler:
    tasks:
      disk_check:
        enabled: true
        schedule: "*/5 * * * *"
```

## Database and Cache

The database stores runtime state such as owners, tokens, scheduler history,
and audit events. Operator configuration stays in `server.yml`, `yaml.d/`,
and `secrets.yml`; it is never stored in the database. SQLite is the default
and writes `{data_dir}/db/server.db`. Remote libSQL/Turso is selected with
`driver: libsql`; credentials belong in `secrets.yml`.

```yaml
server:
  database:
    driver: sqlite                 # sqlite | libsql
    url: ""                        # local path or remote libsql URL
    pool:
      max_open: 5
      max_idle: 2
      max_lifetime: 5m
      max_idle_time: 1m
  cache:
    type: memory                   # none | memory | valkey | redis
    url: ""                        # use URL or host/port credentials
    host: localhost
    port: 6379
    username: ""
    password: ""                   # secret value belongs in secrets.yml
    db: 0
    tls: false
    tls_skip_verify: true
    tls_ca: ""
    tls_cert: ""
    tls_key: ""
    pool_size: 10
    min_idle: 2
    timeout: 5s
    prefix: "airports:"
    ttl: 1h
```

The default cache is in-process memory and is cleared when the process stops.
Use Valkey or Redis when rate-limit and cache state needs to be shared across
instances. For TLS, configure the CA and client certificate/key file names
under `{config_dir}/ssl/` as required.

## Backup and Restore

Backups are written under `{backup_dir}`. Retention settings live under
`server.backup.retention`; zero weekly/monthly/yearly values disable those
periodic copies. `max_total_size` accepts a percentage or an absolute size.
The backup password is a secret and should be stored in `secrets.yml`.
Compliance mode is active when any standard is enabled, and backups then
require encryption.

```yaml
server:
  backup:
    retention:
      max_backups: 1
      keep_weekly: 0
      keep_monthly: 0
      keep_yearly: 0
      max_total_size: "10%"
  restore:
    max_archive_size: 1G
    max_expanded_size: 4G
    max_files: 10000
    max_file_size: 512M
    max_ratio: 100
```

Old backups are removed only after the new backup passes its verification
checks. Restore limits bound compressed size, expanded size, file count,
individual file size, and expansion ratio. See [admin.md](./admin.md) for
backup, restore, retention, and recovery operations.

## Metrics

Metrics are internal operational endpoints. Each service has an independent
bearer token, generated into `secrets.yml` when missing. Keep authentication
enabled on public networks; `allow_unauthenticated` is intended only behind a
firewall.

```yaml
server:
  metrics:
    enabled: true
    root:
      enabled: true
    auth:
      allow_unauthenticated: false
      tokens:
        prometheus: ""              # generated secret
        grafana: ""                 # generated secret
        loki: ""                    # generated secret
    include_system: true
    include_runtime: true
    loki:
      max_entries: 1000
      max_age: 1h
```

The service-specific endpoints are `/server/metrics/prometheus`,
`/server/metrics/grafana`, and `/server/metrics/loki`; root aliases such as
`/metrics` are controlled by `server.metrics.root.enabled`. See
[admin.md](./admin.md) for scraper setup and dashboard operations.

## Compliance Settings

All standards are disabled by default and are configured independently in
`server.compliance`. Enabling any standard activates encrypted-backup
enforcement and the applicable configured retention policy. The compliance
report summarizes settings and controls; it is not a certification or legal
determination.

```yaml
server:
  compliance:
    gdpr: false
    ccpa: false
    hipaa: false
    soc2: false
    pci_dss: false
    iso27001: false
    fedramp: false
    lgpd: false
    pipeda: false
    appi: false
    pdpa: false
```

The standards have different requirements and should only be enabled when the
operator has configured the corresponding controls. See [security.md](./security.md)
for the security model and `airports --maintenance compliance report` for the
effective configured requirements.

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

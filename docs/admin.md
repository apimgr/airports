# Admin & Operations Guide

## No Web Admin Panel

**Airports has no admin web panel, user accounts, or login flow.** Public airport data is read-only. Metrics endpoints use service-specific bearer tokens, and the Tor process-control routes accept loopback callers only.

All operator tasks happen out-of-band via:

- The `airports` CLI (service install, backup, restore, update, security report maintenance, config validation)
- `server.yml`, area drop-ins in `yaml.d/`, and the separate `secrets.yml` file
- The host's service manager (`systemctl`, `launchctl`, etc.)

If you are looking for "how do I log into the admin panel?", there is no answer — the panel does not exist by design.

## Operator Surface

| Task | Tool |
|------|------|
| Start / stop / restart | `airports --service {start,stop,restart}` |
| View live status | `airports --status` |
| View health | `curl -q -LSsf http://YOUR_SERVER/server/healthz` |
| Tail logs | `journalctl -u airports` |
| Change config | Edit `server.yml` or a relevant file in `yaml.d/`; the server watches for changes |
| Run backup | `airports --backup` |
| Restore | `airports --maintenance restore <archive.tar.gz>` |
| Upgrade | `airports --update` or `docker pull` |
| Validate config | `airports --config validate` |
| Show config | `airports --config show` (secrets masked) |

## Where Things Live

| Path | Contents |
|------|----------|
| `{config_dir}/server.yml` | Main config |
| `{config_dir}/yaml.d/branding.yml` | Public branding, SEO, theme, and operator page content |
| `{config_dir}/secrets.yml` | Operator and service tokens, encryption key, SMTP/CAPTCHA secrets |
| `{config_dir}/ssl/` | TLS certs and keys |
| `{config_dir}/tor/` | Tor `torrc` (binary owns Tor entirely) |
| `{data_dir}/security/geoip/` | Downloaded MMDB files |
| `{data_dir}/tor/` | Tor hostname and hidden-service keys |
| `{data_dir}/db/sqlite/server.db` | App SQLite database (operational state only — airport data is embedded) |
| `{log_dir}/access.log` | Request access log |
| `{log_dir}/error.log` | Error log |
| `{data_dir}/backups/` | Backup archives |

See [installation.md](./installation.md) for OS-specific default paths.

## Backups

Backups include:

- `server.yml` and `secrets.yml`
- Configuration area drop-ins under `yaml.d/` (including `branding.yml`)
- Branding images in `assets/` and disclosure keys under `security/` when present
- The SQLite operational DB (`server.db`)
- TLS certificates and keys
- Tor hidden-service keys

Backups **do not** include GeoIP, blocklist, or CVE data — these are re-downloadable.
The fetched branding-image cache is also excluded because it is derived from
the configured image URLs.

```bash
airports --backup                                    # writes to {data_dir}/backups/
airports --backup /mnt/external/backups/             # custom destination
airports --maintenance restore /mnt/external/backups/airports-2026-05-15T03-00-00Z.tar.gz
```

Automated backups can be enabled via the scheduler block in `server.yml`.

## Security Reports

Coordinated disclosure reports are stored in the application database with the
report body and researcher contact details encrypted separately. The operator
can list reports, inspect non-report metadata, and advance a report using the
maintenance CLI:

```bash
airports --maintenance security list
airports --maintenance security list --status triaged
airports --maintenance security show sec_0123456789abcdef
airports --maintenance security update sec_0123456789abcdef --status confirmed --comment "Fix is in progress"
```

These commands require root or the configured `server.token`. `show` never
prints the vulnerability report body. Moving a report to `disclosed` adds an
eligible researcher credit to the public hall of fame.

## Sensitive maintenance

The maintenance commands below require root or the configured `server.token`.
Secret rotation and PGP private-key export/import/deletion also require typed
confirmation. The operator token is re-prompted for sensitive operations.

```bash
airports --maintenance pgp generate
airports --maintenance pgp rotate
airports --maintenance pgp publish
airports --maintenance pgp export public
airports --maintenance pgp export private /secure/path/pgp-private.asc
airports --maintenance pgp import /secure/path/pgp-private.asc
airports --maintenance pgp delete

airports --maintenance secret rotate installation_secret
airports --maintenance secret rotate encryption_key
airports --maintenance token list
airports --maintenance token revoke PREFIX
airports --maintenance data export --owner PREFIX --output airports-export.json
airports --maintenance data delete --owner PREFIX
airports --maintenance compliance report
```

Data exports contain owner and token metadata plus owned resources, never token
values or hashes. Airports currently has no owner-owned resource records.
Deletion removes the owner's tokens and record transactionally and retains only
an anonymized compliance event. PGP rotation re-encrypts retained report bodies;
deleting the PGP key makes PGP-encrypted reports unreadable, so the command
requires explicit confirmation.

## Monitoring

- **Prometheus metrics**: `GET /metrics` (Prometheus text exposition format).
- **Health endpoint**: `GET /server/healthz` (HTML) or `GET /api/{api_version}/server/healthz` (JSON).
- **About endpoint**: `GET /server/about` shows the project description, features, links, and version.
- **Docker healthcheck**: built in (`airports --status`).

## Scheduler

The binary includes its own scheduler — **never depend on host cron or systemd timers**. Default jobs:

| Job | Default cron | Purpose |
|-----|--------------|---------|
| TLS renewal | `0 3 * * *` | Renew certificates within the configured window |
| GeoIP refresh | `0 3 * * 0` | Weekly ip-location-db update |
| Blocklist refresh | `0 4 * * *` | Fetch configured blocklists |
| CVE refresh | `0 5 * * *` | Update vulnerability data |
| Update check | `0 6 * * *` | Check release metadata |
| Token cleanup | `*/15 * * * *` | Remove expired/revoked token state |
| Log rotation | `0 0 * * *` | Rotate application logs |
| Backup | `0 2 * * *` | Create a scheduled backup (five retained by default) |
| Health check | `*/5 * * * *` | Refresh server health state |
| Tor health | `*/10 * * * *` | Check the configured Tor service |

Schedules and enablement are configurable under `server.scheduler.tasks`.

## Reverse Proxy

The recommended deployment is behind a reverse proxy (nginx, Caddy, Traefik) terminating TLS. The container binds `172.17.0.1:64580:80` by default so only the proxy can reach it.

The binary auto-detects when it is behind a reverse proxy via `X-Forwarded-*` headers and adapts `DOMAIN`, scheme, and remote IP accordingly.

## Telemetry

There is no mandatory telemetry or phone-home. Optional analytics can be configured and is loaded in visitors' browsers only when consent permits it. The server binary makes outbound HTTPS calls only for:

- GeoIP database and checksum updates (`github.com` release assets)
- ACME certificate issuance (`acme-v02.api.letsencrypt.org`) when TLS-ACME is enabled
- Update checks (`api.github.com`) when `airports --update` is invoked

All outbound endpoints are documented in [integrations.md](./integrations.md).

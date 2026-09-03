# Admin & Operations Guide

## No Web Admin Panel

**Airports has no admin web panel, no user accounts, no login flow, and no privileged API.** This is intentional — the service is a public, read-only reference API for airport data.

All operator tasks happen out-of-band via:

- The `airports` CLI (service install, backup, restore, update, config validation)
- The `server.yml` config file
- The host's service manager (`systemctl`, `launchctl`, etc.)

If you are looking for "how do I log into the admin panel?", there is no answer — the panel does not exist by design.

## Operator Surface

| Task | Tool |
|------|------|
| Start / stop / restart | `airports --service {start,stop,restart}` |
| View live status | `airports --service status` |
| View health | `curl http://localhost:8080/server/healthz` |
| Tail logs | `airports --service logs` or `journalctl -u airports` |
| Reload config | Send `SIGHUP` to the process (`systemctl reload airports`) |
| Change config | Edit `server.yml`, then reload |
| Run backup | `airports --backup` |
| Restore | `airports --restore <archive.tar.gz>` |
| Upgrade | `airports --update` or `docker pull` |
| Validate config | `airports --config validate` |
| Show config | `airports --config show` (secrets masked) |

## Where Things Live

| Path | Contents |
|------|----------|
| `{config_dir}/server.yml` | Main config |
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

- `server.yml`
- The SQLite operational DB (`server.db`)
- TLS certificates and keys
- Tor hidden-service keys

Backups **do not** include GeoIP, blocklist, or CVE data — these are re-downloadable.

```bash
airports --backup                                    # writes to {data_dir}/backups/
airports --backup /mnt/external/backups/             # custom destination
airports --restore /mnt/external/backups/airports-2026-05-15T03-00-00Z.tar.gz
```

Automated backups can be enabled via the scheduler block in `server.yml`.

## Monitoring

- **Prometheus metrics**: `GET /metrics` (Prometheus text exposition format).
- **Health endpoint**: `GET /server/healthz` (HTML) or `GET /api/v1/server/healthz` (JSON).
- **About endpoint**: `GET /server/about` shows version, build date, commit, GeoIP DB freshness, Tor onion address (if enabled), and uptime.
- **Docker healthcheck**: built in (`airports --status`).

## Scheduler

The binary includes its own scheduler — **never depend on host cron or systemd timers**. Default jobs:

| Job | Default cron | Purpose |
|-----|--------------|---------|
| GeoIP refresh | `0 4 * * *` | Daily download of ip-location-db MMDB files |
| Blocklist refresh | `30 4 * * *` | Daily fetch of configured blocklists |
| Cache cleanup | `0 * * * *` | Hourly LRU sweep |
| TLS renewal check | `0 */12 * * *` | ACME renewal if cert age > 60 days |
| Backup | disabled by default | Set `scheduler.jobs.backup.enabled: true` in `server.yml` |

All jobs are leader-only in multi-instance deployments via a DB advisory lock.

## Reverse Proxy

The recommended deployment is behind a reverse proxy (nginx, Caddy, Traefik) terminating TLS. The container binds `172.17.0.1:64580:80` by default so only the proxy can reach it.

The binary auto-detects when it is behind a reverse proxy via `X-Forwarded-*` headers and adapts `DOMAIN`, scheme, and remote IP accordingly.

## Telemetry

There is **no telemetry**, no analytics, no phone-home. The binary makes outbound HTTPS calls only for:

- GeoIP database updates (`cdn.jsdelivr.net`)
- ACME certificate issuance (`acme-v02.api.letsencrypt.org`) when TLS-ACME is enabled
- Update checks (`api.github.com`) when `airports --update` is invoked

All outbound endpoints are documented in [integrations.md](./integrations.md).

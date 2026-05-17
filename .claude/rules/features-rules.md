# Feature Rules (PART 17-22)

Cheatsheet — see AI.md PART 17 (28324), 18 (29636), 19 (30114), 20 (30211), 21 (31655), 22 (32365).

## Email & Notifications (PART 17)

- SMTP optional; auto-detected when env vars or `server.yml` SMTP block present.
- Required SMTP keys: `host`, `port`, `username`, `password`, `from`.
- TLS preferred (STARTTLS on 587, implicit TLS on 465).
- All emails sent via background queue with retries (3 attempts, exponential backoff).
- Templates in `src/server/template/email/` — text + HTML variants.
- Never log raw SMTP credentials. Mask in `--config show` output.
- This project has no user accounts → email surface is admin notifications only (if any).

## Scheduler (PART 18)

- **Built-in scheduler** using `robfig/cron` or `go-co-op/gocron`. NEVER depend on host cron / systemd timers.
- Default jobs:
  - Daily: GeoIP DB refresh, blocklist refresh, CVE feed refresh.
  - Hourly: cache cleanup.
  - Every 12 hr: TLS cert renewal check.
- Job execution: leader-only in multi-instance via DB advisory lock.
- All jobs have timeout + recover middleware (panic → log → continue).
- Jobs configurable via `server.yml` `scheduler:` block (enable, cron expression).

## GeoIP (PART 19)

- Provider: **ip-location-db** (free, daily updates via jsDelivr CDN, CC0/PDDL).
- Databases: `asn.mmdb`, `country.mmdb`, `city.mmdb`, `whois.mmdb` in `{data_dir}/security/geoip/`.
- Downloaded on first run; updated daily by scheduler.
- All downloads use HTTPS; verify content-length.
- Used for: caller location detection on home page (nearby airports), abuse attribution.
- GeoIP lookup result is read-only — never stored, never logged.
- Failure mode: graceful — home page shows generic view; all other functionality unaffected.

## Metrics (PART 20)

- Prometheus-style metrics at `/metrics` (text exposition format).
- Default metrics: `http_requests_total`, `http_request_duration_seconds` (histogram), `process_*`, `go_*`.
- Custom metrics: per-feature counters (e.g., `airports_search_total{format="json"}`).
- Metrics endpoint may require an allowlist (loopback / private network) — never authenticated tokens for scraping.
- Cardinality budget: label values bounded; never user-supplied free-form strings as labels.

## Backup & Restore (PART 21)

- `airports --backup [path]` → produces `{name}-{ISO timestamp}.tar.gz` in `{data_dir}/backups/` or supplied path.
- Backup contents: `server.yml`, SQLite DB(s), TLS certs/keys, Tor keys. **Never** GeoIP / blocklist / CVE data (re-downloadable).
- Backup is atomic — write to `.tmp` then rename.
- `airports --restore <archive>` → halts server (if running), validates archive checksum, restores files, restarts.
- Built-in scheduler may run periodic backups (configurable in `server.yml`).
- Backups retain last N (configurable, default 7) and prune older.

## Update Command (PART 22)

- `airports --update` → checks GitHub releases for newer version.
- Compares against `release.txt` baked into binary at build time.
- Downloads matching `{os}/{arch}` tarball, verifies checksum (SHA-256 from release assets), replaces binary in-place (atomic rename), restarts via service manager.
- Refuses to self-update when:
  - Running as a managed service without `--service` integration enabled.
  - Currently in a Docker container (instruct user to `docker pull` instead).
- Prints clear "no update available" when on latest.

## General Feature Rules

- Every feature MUST work via Browser, PWA, API, CLI.
- Every feature has corresponding docs under `docs/`.
- Every feature emits at least one metric.
- Every feature has at least one test.

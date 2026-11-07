# Email & Notifications, Scheduler, GeoIP, Metrics, Backup/Restore, Update Rules (PART 17-22)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Send/queue/attempt email without a valid, tested SMTP connection — no "would have sent" logs
- Create account-related emails (password reset, verification) — this app has no user accounts
- Use an external scheduler (cron, systemd timers, Task Scheduler, launchd, K8s CronJob, cloud
  schedulers) for ANY scheduled task — the built-in scheduler is the only mechanism, no exceptions
- Treat GeoIP/country data as an access-control gate by itself — it is a risk signal only; never
  the sole thing standing between a request and a resource; fail-open on GeoIP lookup errors
- Use `geoip2-golang` for ip-location-db MMDB files — use `github.com/oschwald/maxminddb-golang`
- Expose `/metrics` publicly — internal only, firewall/proxy-block it; never inject update/version
  status into any public API response, header, or endpoint
- Use raw client IP (or any high-cardinality value) as a Prometheus label — unbounded cardinality
- Delete existing backups before a new backup passes ALL verification checks
- Store a backup encryption password anywhere, or accept it via CLI flag (shell history/process
  list leak) — always prompt interactively
- Run a scheduled backup when compliance mode is enabled but no encryption password is set
- Skip SHA256 checksum verification (against the release's `checksums.txt`) before installing
  a self-update, or auto-install an update without `update.auto_install: true` explicitly set
- Surface "update available" anywhere but operator channels (log/email/CLI) — never in the
  public WebUI or API

## CRITICAL - ALWAYS DO

- Fall back to the embedded default email template when no custom override file exists in
  `{config_dir}/template/email/`; validate templates (unknown variables, empty subject/body)
  before saving
- Suppress `scheduler_error` when a more specific failure event (`backup_failed`,
  `ssl_renewal_failed`) already fired for the same execution — one notification, not two
- Persist scheduler task state (`last_run`, `next_run`, `run_count`, `fail_count`) in `server.db`;
  run missed tasks on startup if within `catch_up_window`
- Download GeoIP MMDB databases on first run / via scheduler — never embed them in the binary
- Verify every backup immediately after creation (file exists, size>0, checksum, decrypt test,
  manifest parse, content extraction, DB integrity) — all checks must pass or the backup is
  deleted and old backups are kept
- Require operator authorization for restore (root confirmation or `server.token`) except when
  the database is empty (first-run)
- Gate the `update_check` task by `defer_days`, but let manual `--update check`/`--update yes`
  always see and install the true latest — an explicit operator action overrides the defer window
- Use platform-specific binary replacement for self-update (atomic rename on Unix, rename-to-.old
  + `MOVEFILE_DELAY_UNTIL_REBOOT` on Windows) and restart via the platform's service manager

## Key Rules Summary

| Area | Rule |
|------|------|
| Notification channels | Public WebUI (toast/banner, visitors only) · Logs (always) · Email (only if SMTP configured+working) — operator events never render in WebUI |
| SMTP | Auto-detect on first run (loopback → docker gateway → gateway IP → FQDN → global IPv4 → mail./smtp. subdomains); `SMTP_*` env vars override config; test connection every startup |
| Email templates | `{variable}` syntax, `Subject: ...` then `---` then body; embedded defaults in `src/server/template/email/`, overrides in `{config_dir}/template/email/` |
| Scheduler mechanism | Built-in only, Go `time.Ticker`, cron/`@every`/`@hourly` etc. syntax, state in `server.db`, catch-up window on restart, graceful shutdown (30s) |
| Built-in tasks | `ssl_renewal`(daily 03:00) · `geoip_update`(weekly Sun 03:00) · `blocklist_update`(daily 04:00) · `cve_update`(daily 05:00) · `update_check`(daily 06:00) · `token_cleanup`(15m) · `log_rotation`(daily 00:00) · `backup_daily`(02:00) · `backup_hourly`(off) · `healthcheck_self`(5m) · `tor_health`(10m) |
| GeoIP source | `sapics/ip-location-db` MMDB files via CDN (jsdelivr), no API key; downloaded to `{data_dir}/security/geoip`, updated weekly by scheduler; never a sole access gate |
| GeoIP databases | ASN, Country, City (IPv4+IPv6), WHOIS (derived join of ASN+Country, no separate file) |
| Metrics format/endpoint | Prometheus text exposition via `github.com/prometheus/client_golang`, path `/metrics` (configurable), metrics prefixed `{project_name}_`, optional Bearer token auth |
| Metrics naming | snake_case, unit suffix (`_seconds`/`_bytes`), counters end `_total`; low-cardinality labels only (never user_id/request_id/IP) |
| Backup format | `.tar.gz` or `.tar.gz.enc` (AES-256-GCM, Argon2id key derivation); `{project_name}_backup_YYYY-MM-DD[_HHMMSS].tar.gz[.enc]` full + `{project_name}-daily.tar.gz[.enc]` incremental + optional `-hourly` |
| Backup schedule | `backup_daily` at 02:00 (full+daily incremental, default 2 files kept), `backup_hourly` disabled by default; retention by `max_backups`/`keep_weekly`/`keep_monthly`/`keep_yearly`/`max_total_size` |
| Compliance mode | If `server.compliance.enabled`, backups are BLOCKED until an encryption password is set |
| Restore | `{project_name} --maintenance restore <file>`; verifies checksum/manifest/decrypt before proceeding; requires `server.token` unless DB is empty |
| Self-update command | `--update [check\|yes\|branch {stable\|beta\|daily}]` (alias `--maintenance update`); default `yes`; channels are cumulative (beta ⊇ stable, daily ⊇ beta ⊇ stable); checksum-verified download, platform-specific binary swap + service restart |
| Update config | `server.update.branch`, `server.update.auto_install` (default false — notify only), `server.update.defer_days` (0-365, gates scheduled task only) |

For complete details, see AI.md PART 17, PART 18, PART 19, PART 20, PART 21, PART 22.

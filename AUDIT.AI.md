# Project Audit

Started: 2026-09-03

Full line-by-line AI.md PART 0-32 + FINAL CHECKPOINT compliance sweep.
AI.md is authoritative over IDEA.md, over `.claude/rules/*.md` cheatsheets, and
over existing code. Items too large to safely complete in this pass are moved to
TODO.AI.md with a precise entry and deleted from here.

## Pass 5: Spec Compliance — PART 0-4

- [ ] PART 0 / `.claude/memory/`: absent; AI.md 2608 requires the dir plus an empty `MEMORY.md` index (gitignored, per PART 3 6203)
- [ ] PART 0 / `.claude/rules/*.md`: 12 of 13 headers missing the literal `Rules` token required by AI.md 2636 (`# {Topic} Rules (PART X, Y, Z)`)
- [ ] PART 0 / `.claude/rules/backend-rules.md`: PART 31 renamed "Overlay Networks (Tor & I2P)" at AI.md 3993; file covers Tor only, zero I2P content
- [ ] PART 1 / `src/mode/mode.go:15`: `type Mode string`; AI.md 4575 bans bare `Mode`, requires `AppMode`
- [ ] PART 1 / `src/mode/mode.go:99,123,128,283`: `IsDebug()`/`IsDevelopment()`/`IsProduction()`/`Validate()`; AI.md 4551-4553 require `IsDebugEnabled()`/`IsAppModeDev()`/`IsAppModeProd()`, 4540 bans bare `Validate()`
- [ ] PART 1 / `src/geoip/service.go:82`: bare `Available()` banned by AI.md 4557
- [ ] PART 1 / bare `type Config struct` in `src/config/config.go:21`, `src/tor/config.go:8`, `src/logging/config.go:66`, `src/ssl/ssl.go:19`; bare `type Status struct` in `src/tor/admin.go:30` — AI.md 4577-4578
- [ ] PART 1 / ~20 bare generic exported funcs (`New`/`Get`/`Load`/`Save`/`Update`/`Start`/`Stop`/`Create`/`Handle`/`Validate`) across cache, scheduler, server, config, i18n, service, backup, shellcompl, cve, tor — AI.md 4515-4545
- [ ] PART 1 / `src/common/display/`: absent. AI.md 4620-4627 requires `DetectDisplayEnv()` + `IsAutoDetectDisplayModeGUI/TUI/CLI/Headless()` — LARGE, likely TODO.AI.md
- [ ] PART 1 / `src/config/`: missing `defaults.go`, `validate.go` (AI.md 4695)
- [ ] PART 1 / `src/mode/`: missing `production.go`, `development.go` (AI.md 4697)
- [ ] PART 1 / `src/server/`: missing `routes.go`, `middleware.go` (AI.md 4693)
- [ ] PART 1 / `src/signals_unix.go`,`src/signals_windows.go`: AI.md 4699-4701 requires `src/signal/{signal.go,signal_unix.go,signal_windows.go}`
- [ ] PART 1 / `README.md:5`: Build badge points at `release.yml`; AI.md 4762-4773 requires `ci.yml` badge
- [ ] PART 1 / `README.md`: no Docs badge although RTD is configured (AI.md 4791, 4838-4849, 5117)
- [ ] PART 1 / curl format: AI.md 5293 mandates `curl -q -LSsf {url}` everywhere in docs/examples/tests/scripts. Violations in `README.md:44,53,67,88,158,161,164,167,170`, `docs/api.md:577-586`, `docs/index.md:37`, `docs/admin.md:21`, `scripts/linux.sh:70`, `scripts/macos.sh:47`
- [ ] PART 1 / curl in tests: `tests/docker.sh:70,77,86,102,104`, `tests/incus.sh:110,126,128` missing `-L`/`-S` (AI.md 5293 + 5363 status-capture exception allows dropping only `-f`)
- [ ] PART 1 / doc curl hosts: AI.md 4977 requires `{official_site}` not `localhost`/placeholder hosts — `README.md`, `docs/api.md`, `docs/index.md`, `docs/admin.md`
- [ ] PART 2 / `LICENSE.md:66,69`: stale versions `golang.org/x/crypto v0.54.0`, `golang.org/x/text v0.40.0`; `go.mod` has v0.55.0 / v0.41.0 (AI.md 5662)
- [ ] PART 2 / `LICENSE.md:57-72`: only 14 direct deps attributed; ~220 indirect modules compiled into the binary are unattributed (AI.md 5528)
- [ ] PART 2 / `scripts/verify-licenses.sh`: absent (AI.md 5699-5701)
- [ ] PART 2 / `.github/workflows/licenses.yml`: absent (AI.md 5667-5697)
- [ ] PART 3 / `tests/e2e.sh`: absent; AI.md 6033 marks it REQUIRED (headless Chromium browser E2E)
- [ ] PART 3 / `go.mod:11`: uses `github.com/robfig/cron/v3`; AI.md 6453 specifies `github.com/go-co-op/gocron/v2`

## Pass 5: Spec Compliance — PART 13-14 + API checklist

- [ ] P0 PART 14 / `src/common/httputil/` absent. AI.md 18693-18985 + checklist 47884-47919 mandate `detect.go` (`isOurCliClient`, `isTextBrowser`, `isHttpTool`, `isNonInteractiveClient`) + `html2text.go` (`renderNoJSHTML`, `HTML2TextConverter`). Zero non-test hits for all six — LARGE
- [ ] P0 PART 14 / `src/server/web_handlers.go`,`server_pages.go`: frontend routes do no content negotiation at all (no `Accept`/`User-Agent` read); AI.md 18449-18626 — LARGE
- [ ] P1 PART 13 / `src/server/healthz.go` `FeaturesInfo`: no `features.i2p` (AI.md 17416-17417, 17438-17450)
- [ ] P1 PART 13 / `src/server/healthz.go` `ChecksInfo`: no `checks.i2p` omitempty field (AI.md 17464-17465)
- [ ] P1 PART 13 / `src/server/healthz.go` `healthResponseText`: no `features.i2p.*` lines (AI.md 17879-17883)
- [ ] P1 PART 13 / `src/server/healthz.go:428-435`: both branches hardcode HTTP 200; AI.md 17938-17949 require 503 for `unhealthy`/`maintenance`/`shutting_down`. Breaks `--status`, Docker HEALTHCHECK, k8s probes
- [ ] P2 PART 13 / `overallStatus` never returns `restart_required`/`maintenance`/`shutting_down` (AI.md 17938-17949)
- [ ] P2 PART 13 / `src/server/healthz.go:416-435`: API healthz paths serve HTML when `Accept: text/html`; AI.md 18464-18467 require JSON default on API routes
- [ ] P2 PART 13 / `wantsHealthzText` (`healthz.go:391-405`) matches only curl/wget/httpie; AI.md 18741-18757 `isHttpTool()` also covers libcurl, python-requests, go-http-client, axios, node-fetch, and empty UA
- [ ] P2 PART 13 / `/api/{api_version}/server/healthz.txt` not registered (AI.md 18531)
- [ ] P1 PART 14 / 94 non-test literal `/api/v1` occurrences (`page/index.tmpl:45,50`, `server-help.tmpl`, `static/js/app.js:277,299,429,451`, …); AI.md 18196 forbids hardcoding `v1` — `templates.go:125` already injects `APIBase`
- [ ] P1 PART 14 / `/api/swagger.json` and `/api/{api_version}/server/swagger.json` registered; AI.md 19590 says no `.json` suffix on the path
- [ ] P1 PART 14 / parallel legacy route tree `/search`, `/search.txt`, `/nearby`, `/nearby.txt`, `/bbox`, `/autocomplete` duplicating canonical `/airports/*` (AI.md 18024-18037)
- [ ] P1 PART 14 / `/api/{v}/health`, `/health.txt` + `handlers.go:~20 handleHealth`: fifth non-canonical health route, wraps in `{ok,data}` violating bare-object rule (AI.md 17296-17304, 18016, 17762)
- [ ] P2 PART 14 / metrics route set incomplete: missing `/server/metrics/{service}`, `/metrics/{service}`, `/api/metrics[/{service}]`, `/api/{api_version}/server/metrics[/{service}]` (AI.md 19577-19587)
- [ ] P2 PART 14 / `/announcements/dismiss` uses a verb (AI.md 18056)
- [ ] P2 PART 14 / orphan routes: API-only `/api/{v}/{countries,states/{country},autocomplete,settings,airports/within}`; frontend-only POST `/theme`, `/server/consent`, `/announcements/dismiss` (checklist 47823-47919)
- [ ] P2 PART 14 / `respondText` (`server.go:629-633`) does not guarantee one trailing `\n`; `handlers.go:170,466,497` omit it (AI.md 18287-18298)
- [ ] P2 PART 14 / `handlers.go:170,466,497` emit plain-text errors without the `ERROR: CODE: message` shape (AI.md 19641-19734)
- [ ] P3 PART 14 / JSON responses emitted compact; AI.md 18287-18298 require 2-space indent (only `healthz.go:433` complies)
- [ ] P3 PART 14 / Swagger UI / GraphiQL point at `apiBase+"/server/swagger"`/`"/server/graphql"`; AI.md 19575-19576 use the unversioned `/api/swagger` and `/api/graphql`
- [ ] P3 PART 14 / `/api/{v}/airports.json`,`.csv`,`.geojson`: extensions beyond `.txt` (AI.md 18469, 18529-18533) — verify against IDEA.md before removing

## Pass 5: Spec Compliance — PART 17-22

- [ ] P0 PART 21 / `src/main.go:2286-2340 restoreBackup`: `backup.AuthorizeRestore` (`src/backup/restore.go:43`) has ZERO non-test callers — restore proceeds unauthorized from any account. AI.md 30678-30687 — SECURITY
- [ ] P0 PART 20 / `src/server/metrics.go:266-269`: `metricsAuthMiddleware` returns `next` unauthenticated when token empty; AI.md 28796-28808 make per-service bearer auth mandatory ("no unauthenticated default"; empty token → 403 empty body). Doc comment states the opposite of spec — SECURITY
- [ ] P0 PART 20 / `src/server/server.go:297,422-433`: only `/server/metrics` + one configurable root path. AI.md 28760-28776 require four route families incl. `/prometheus`,`/grafana`,`/loki` sub-routes and both `/api/...` families; reference mount at 29939-30000 — LARGE
- [ ] P1 PART 20 / no `/server/metrics/grafana` handler; dashboard JSON specified verbatim at AI.md 30152-30228
- [ ] P1 PART 20 / no `/server/metrics/loki` handler; `MetricsConfig` has no `Loki` struct (AI.md 28769, 28783, 28857-28860)
- [ ] P1 PART 20 / `src/config/config.go:413-430`: `MetricsConfig` missing `root.enabled`, `auth.allow_unauthenticated`, per-service `auth.tokens.*`, `include_runtime`, `loki.*` (AI.md 28830-28867)
- [ ] P1 PART 20 / `src/server/metrics.go:20-44,181-194`: required metric families absent — db(5), cache(5), scheduler(4), ratelimit(2), auth(2), system(7), tor(4) (AI.md 28965-29107) — LARGE
- [ ] P1 PART 22 / `src/main.go:2596-2723 checkAndUpdate`: unconditional `os.Rename` with no `GOOS=windows` branch; AI.md 30944-30973 require rename-to-`.old` + `MOVEFILE_DELAY_UNTIL_REBOOT`. Self-update broken on both windows release targets
- [ ] P2 PART 20 / `src/config/config.go:740`: `Metrics.Enabled` defaults false; AI.md 28833 defaults true
- [ ] P2 PART 20 / `src/server/metrics.go:103-110,246`: `airports_http_request_duration_seconds` carries a third `status_code` label; AI.md 28929 defines `method, path` only (cardinality, 29075)
- [ ] P2 PART 17 / `src/notify/render.go`: no template validation (unknown `{variable}`, empty subject/body) before accepting an override (AI.md 27969-27984)
- [ ] P3 PART 18 / `i2p_health` task never registered (AI.md 28255) — blocked on I2P
- [ ] P3 PART 18 / `src/main.go:1817`: `token_cleanup` registered as a declared permanent no-op (AI.md 28242-28255)
- [ ] P3 PART 17 / `src/notify/templates.go:15`: embeds from `src/notify/templates/`; AI.md 27614 specifies `src/server/template/email/`. Override path and all 10 templates are otherwise correct

## Pass 5: Spec Compliance — PART 7-8

- [ ] P1 PART 7 / `src/common/{display,terminal,banner,version}/` all absent (AI.md 9440-9565, 9641-9748, 9765-9793, 9830-9901) — LARGE
- [ ] P1 PART 7 / `src/main.go:1288-1292`: two bare `fmt.Printf` lines instead of `PrintServerStartupBanner` with width tiers ≥80/60-79/40-59/<40 (AI.md 9925-9959, 10696-10714, 48127-48213) — LARGE
- [ ] P1 PART 8 / `src/main.go:107,239-244,2940-2975`: `--mode` implemented as persist-and-exit maintenance action; AI.md 10162-10209, 12176 define it as the runtime mode for the current run
- [ ] P1 PART 8 / `src/main.go` `--maintenance`: only `backup`/`restore` handled; AI.md 10204 requires `update, mode, setup, pgp, secret, token, data, compliance, --help` too, and `--maintenance` doubles as an unspecified boolean — LARGE
- [ ] P1 PART 8 / `src/main.go:1559-1592 checkStatus`: queries `/healthz`, which is an opt-in root alias; `--status` and the Docker HEALTHCHECK fail whenever `server.healthz.root.enabled` is false. Must use `/server/healthz`
- [ ] P1 PART 8 / `src/config/config.go`: no `server.token` operator token (AI.md 11810-11820) — this is why `backup.AuthorizeRestore` has no caller
- [ ] P1 PART 8 / proxy headers: only `X-Forwarded-Proto`/`X-Forwarded-For` supported; AI.md 12474, 12539-12554, 12845-12852 require `X-Forwarded-Host`,`X-Real-Host`,`X-Original-Host`,`X-Forwarded-Port`,`X-Real-Port`,`X-Forwarded-Ssl`,`X-Url-Scheme`,`Front-End-Https`
- [ ] P2 PART 8 / `src/main.go:1061`: config-file mode passed as `cliMode`, so `server.yml` outranks `MODE` env (AI.md 11926-11933)
- [ ] P2 PART 8 / `src/main.go:1081,1098`: `firstNonEmpty(flag, config, env)` puts config ahead of env for `--port`/`--address` (AI.md 12162-12178)
- [ ] P2 PART 8 / `src/main.go:1321-1334`: SIGHUP is handled+reloads; AI.md 11127, 11204 require `signal.Ignore(syscall.SIGHUP)`
- [ ] P2 PART 8 / SIGUSR1 handler logs but never reopens log file handles (AI.md 11116-11326)
- [ ] P2 PART 8 / `src/main.go:2827-2853 resolveFQDN`: inserts the config value at priority 2 and has no IPv6-before-IPv4 split (AI.md 12469-12481)
- [ ] P2 PART 8 / `src/main.go:2775-2792 getAccessibleURL`: hardcodes `http://` and always appends the port; AI.md 12426, 12460-12463 require proto detection and stripping `:80`/`:443`
- [ ] P2 PART 8 / no shared `GetURLVars`/`BuildURL`/`GetBaseDomain`/`GetWildcardDomain`; five sites re-implement scheme detection (AI.md 12629-12647)
- [ ] P2 PART 8 / `src/main.go:1002`: `--lang` parsed then discarded (`_ = lang`) (AI.md 9975)
- [ ] P2 PART 8 / `src/main.go:108-140`: extra top-level `--restore` flag; spec has only `--maintenance restore <file>` (AI.md 10204)
- [ ] P2 PART 8 / `src/main.go:925-1285 run()`: 4 deviations from the 22-step startup order (config before privilege drop; PID after DB/scheduler/Tor; Tor after drop; immediate-exit flags dispatched late) (AI.md 10496-10726)
- [ ] P2 PART 8 / `src/server/handlers.go:565-585 getClientIP`: only `X-Forwarded-For`; AI.md 12856-12865 require `CF-Connecting-IP`→`True-Client-IP`→`X-Real-IP`→`X-Forwarded-For`→`X-Client-IP`→`RemoteAddr`
- [ ] P2 PART 8 / `src/path/paths.go:332-380 GetBackupDir`: re-evaluates euid live (post-drop takes the unprivileged branch) and falls back to `$HOME`-derived paths; AI.md 12224-12253 require `startedElevated` captured pre-drop, a writability probe, and a `{data_dir}/backup` fallback
- [ ] P2 PART 8 / no `publicsuffix`-based `IsValidHost`/`IsValidSSLHost`; `DOMAIN` accepted verbatim (AI.md 12662-12808)
- [ ] P2 PART 7 / `ColorEnabled`/`EmojiEnabled` not shared+exported and missing the config-file tier; server and client each roll their own (AI.md 10046-10158)
- [ ] P3 PART 8 / `--service` accepts `install|…|logs` not the spec's `--install`/`--uninstall`/`--disable` forms (AI.md 10195)
- [ ] P3 PART 8 / request ID: no `X-Correlation-ID`/`X-Trace-ID` fallback, no UUID v4 validation (AI.md 12872-12917)
- [ ] P3 PART 8 / auth-token header chain: only `Authorization` read (AI.md 12920-12951)
- [ ] P3 PART 8 / `--port` rejects the dual-port form `80,443` (AI.md 12414-12463)
- [ ] P3 PART 8 / `printHelp` advertises `--restore`, omits `--maintenance` subcommands, drops `{internal_org}` from the config path (AI.md 10211-10252)
- [ ] P3 PART 8 / no `server.url_detection.*` / domain learning (AI.md 12556-12621)
- [ ] P3 PART 7 / no `CanUseANSI(env)`; `TERM=dumb` only suppresses color/emoji, doesn't gate spinners/progress/tables (AI.md 9567-9639)

## Pass 5: Spec Compliance — PART 5, 6, 12

- [ ] P0 PART 5 / path-security module absent: no `normalizePath`/`validatePathSegment`/`validatePath`/`SafePath`, no `ErrPathTraversal`/`ErrInvalidPath`/`ErrPathTooLong` (AI.md 6909-7008); no `SafeFilePath(baseDir,userPath)` (7126-7153) — MEDIUM
- [ ] P0 PART 5 / `src/server/server.go:169-216`: no `PathSecurityMiddleware` at position #3 rejecting `..`/`RawPath`/`%2e` with 400 (AI.md 7064, 7081-7083) — MEDIUM
- [ ] P0 PART 5 / maintenance mode is dead config: `MaintenanceConfig`/`SelfHealingConfig` referenced only in `src/config/config.go`; no 503-on-write, no `X-Maintenance-*` headers, no `status:"maintenance"` healthz branch, no self-healing (AI.md 7222-7496) — LARGE
- [ ] P0 PART 12 / `src/server/ratelimit.go:54`,`server.go:112`: hardcoded in-memory `NewRateLimiter(60,120,…)`; `cfg.Server.RateLimit.*` never read at enforcement time; no per-IP sliding window in `server.db`, no read/write/health buckets, no `global_burst` (AI.md 16160-16187) — MEDIUM
- [ ] P0 PART 12 / no Tor request detection at all (`isTorRequest` absent): onion requests get clearnet HSTS, clearnet absolute URLs, `127.0.0.1` in logs (AI.md 16059-16134) — MEDIUM/LARGE
- [ ] P1 PART 5 / `src/server/server.go:173`: middleware order wrong — chain starts at RequestID (no URLNormalize anywhere), Logging at position 3 instead of last (AI.md 7159-7183) — MEDIUM
- [ ] P1 PART 5 / `src/config/config.go:352`: no `applyDatabaseEnvOverrides` for runtime `DATABASE_DRIVER`/`DATABASE_URL` (AI.md 7719-7721)
- [ ] P1 PART 5 / no `rate_limits` table; DB tables list at AI.md 7523-7535 incomplete — MEDIUM
- [ ] P1 PART 5 / `Maintenance.Cleanup.*` has no consumer (AI.md 7486-7487) — MEDIUM
- [ ] P1 PART 12 / no `server.baseurl` yaml key; `X-Forwarded-Prefix`/`X-Forwarded-Path`/`X-Script-Name` never read (AI.md 15895-15957) — MEDIUM
- [ ] P1 PART 12 / no `server.limits.{max_body_size,read_timeout,write_timeout,idle_timeout}` section (AI.md 15961-15975) — MEDIUM
- [ ] P1 PART 12 / contact schema non-canonical: has `server.privacy.contact.{general,abuse}` + `web.security.admin` instead of `server.contact.{admin,security,abuse,general}.email` + per-role webhooks (AI.md 16209-16281, 16358-16365) — MEDIUM
- [ ] P1 PART 12 / no webhook transport at all; no `X-Webhook-Signature`/`-Timestamp`/`-ID`/`-Event` (AI.md 16298-16329) — LARGE
- [ ] P1 PART 12 / `src/server/ratelimit.go:187`: `server.rate_limit.enabled` never checked — SMALL
- [ ] P1 PART 12 / `src/server/server.go:737`: `publicBaseURL` honors only `X-Forwarded-Proto`; FQDN/port/base-path header categories unread (AI.md 16003-16013) — MEDIUM
- [ ] P1 PART 12 / `src/tor/config.go`: no `tor.onion_address`/`tor.contact_email` keys (AI.md 16048-16055) — SMALL
- [ ] P1 PART 12 / `handleSecurityTxt` (`server.go:827`) has no Tor variant (AI.md 16136-16152) — SMALL
- [ ] P1 PART 12 / no `server.tracking.{type,id,url}` + 9-platform matrix (AI.md 16375-16385, 16650-16734) — LARGE
- [ ] P1 PART 12 / `PrivacyConfig` missing most of AI.md 16743-16912 (`data.sharing[]`, `retention.*`, `consent.show_until_acknowledged/default_enabled/show_preferences/preferences_text`, `cookies.*`, `third_party.services`, `content.*`) — LARGE
- [ ] P1 PART 12 / no `ConsentMiddleware`/`CanSetPreferenceCookie`/`CanLoadAnalytics` — nothing gates tracking on consent (AI.md 17163-17184) — MEDIUM
- [ ] P2 PART 5 / `src/config/bool.go:10-22`: truthy/falsy word sets deviate from AI.md 7583-7620 (13 truthy + 15 falsy words missing; non-spec `active`/`positive`/`inactive` added) — SMALL
- [ ] P2 PART 5 / `src/server/consent.go:97`: form booleans parsed with `=="on"||=="true"` instead of `config.IsTruthy` (AI.md 7539, 7711) — SMALL
- [ ] P2 PART 5 / `DATABASE_DIR`, `APPLICATION_NAME`, `APPLICATION_TAGLINE` env vars never read (AI.md 7743-7755) — SMALL
- [ ] P2 PART 6 / `src/server/debug.go:42`: `/debug/db` and `/debug/cache` omitted though `src/db` and `src/cache` both exist (AI.md 8813-8820) — SMALL
- [ ] P2 PART 6 / no expvar counters `requests_total`/`requests_duration_seconds`/`errors_total`, no published `uptime_seconds`/`goroutines`/`memory` (AI.md 9097-9127) — SMALL
- [ ] P2 PART 6 / no `server.debug.*` config section (pprof, log_queries, log_cache, log_bodies, max_body_log_size, block_profile_rate, mutex_profile_fraction, runtime_endpoints) (AI.md 9172-9200) — MEDIUM
- [ ] P2 PART 6 / no `debugLog`/`debugLogDB`/`debugLogCache`/`debugMiddleware` (AI.md 8969-9044) — MEDIUM
- [ ] P2 PART 6 / `src/mode/mode.go:244 getLogLevel` unexported with zero call sites; level fixed at `warn`, matching neither mode's spec'd level (AI.md 8714, 8730) — MEDIUM
- [ ] P2 PART 12 / no `server.compression.*` (AI.md 15979-15992) — MEDIUM
- [ ] P2 PART 12 / no `server.i18n.{default_language,supported}` (AI.md 16195-16200) — SMALL
- [ ] P2 PART 12 / `ratelimit.go:158 abuseNotifyRecipient` uses a non-spec fallback chain (AI.md 16285-16292) — SMALL
- [ ] P2 PART 12 / `ratelimit.go:200`: `Retry-After` and `X-RateLimit-Limit` hardcoded to `60` (AI.md 16189) — SMALL
- [ ] P2 PART 12 / `trustedproxy.go:14`: listen-address `/24` missing from always-trusted set (AI.md 16021-16027) — SMALL
- [ ] P2 PART 12 / `trustedproxy.go:57`: DNS names in `additional` resolved once at startup, not every 5 min (AI.md 16029) — MEDIUM
- [ ] P2 PART 12 / no `Onion-Location` header (AI.md 16118-16121) — SMALL
- [ ] P2 PART 12 / no `GetAnalyticsDescription`/`GetDataUsageContent`/`IsCCPAApplicable`/`ccpa_opt_out` cookie (AI.md 16930-17033) — MEDIUM
- [ ] P2 PART 12 / `validateConfig` skips `server.port`, `mode`, `address`, `api_version`, `ssl.min_version`, `web.ui.theme`, cache `type`, and every cron string (AI.md 15871-15885) — MEDIUM
- [ ] P3 PART 5 / no `MustParseBool` (AI.md 7658) — SMALL
- [ ] P3 PART 6 / no third `AppMode` Debug value / no `GetAppModeString()` `" [debugging]"` suffix (AI.md 9207-9215, 8776) — SMALL
- [ ] P3 PART 6 / `ParseMode` errors on unrecognized `server.mode` instead of warn+default (AI.md 15865-15891) — SMALL
- [ ] P3 PART 12 / original-TCP-peer context key absent; invariant holds only by construction (AI.md 16015) — SMALL
- [ ] P3 PART 12 / `config.go:826`: `privacy.data.stored_on_server` defaults false, spec says true (AI.md 16745) — SMALL
- [ ] P3 PART 12 / no session settings section (AI.md 17278-17286) — SMALL

### NEEDS USER DECISION
- PART 13 / `release.txt` contains `0.0.1`; AI.md 17969 requires the first stable release to start at `1.0.0` and forbids `0.x.x`. Bumping the version is externally visible (release tags, Docker tags, update channel) — not changed in this pass.

### Adjudicated, NOT findings
- `.claude/` gitignored is CORRECT per AI.md 6203-6208 + 6252-6257 + 6272. The FINAL CHECKPOINT line implying a committed `.claude/memory/` contradicts PART 3; PART 3 wins (more specific, normative tree).
- `docker/rootfs/` committed is CORRECT per PART 26; AI.md 6068 listing it as gitignored is a spec self-contradiction.
- PART 4 path table: 64/64 rows verified against `src/path/paths.go` — fully compliant.
- Root `coverage.out` / `.test-out.log`: untracked and gitignored; stale local artifacts, removed as cleanup, not spec violations.

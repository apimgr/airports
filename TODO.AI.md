# TODO.AI.md

## AI tool rule cheatsheets

- [ ] Compare all 13 `.claude/rules/*.md` summaries against their current AI.md PARTs; their
      headers are corrected, but their content needs a fresh spec comparison.

## AI.md compliance status

- [ ] Full compliance sweep against AI.md PART 0-33 and FINAL CHECKPOINT. The earlier completed
      status was inaccurate: `AUDIT.AI.md` still contains unresolved findings, and this project is
      not yet fully aligned. Resolve or verify every finding against AI.md before marking complete.

- [ ] PART 1 API naming: replace bare exported `Config`/`Status` types and generic exported
      functions with spec-compliant names and comments; keep behavior and call sites consistent.

- [ ] PART 6 diagnostics: SQL and cache operations are debug-gated across database, scheduler,
      backup, maintenance recovery, cache backends, and rate limiting. Query argument values are
      omitted, cache keys are hashed, and enabled request/response body logs are bounded and
      sanitized. Still wire `slog` diagnostics to configured log files and apply the configured
      global/mode log-level filtering; run the project lint gate when available.

- [ ] PART 7 display behavior: `TERM=dumb` now forces CLI mode even when TUI/GUI is configured,
      and disables color/emoji overrides across the CLI and server banner. Focused display, client,
      and banner checks pass; recheck the full binary matrix before resolving.

- [ ] PART 8 maintenance commands: implement remaining supported `mode`, `pgp`, `secret`, `token`,
      and `data` operations with the specified authorization and audit behavior. `--maintenance mode`
      persists `server.mode` through the protected operator command; per-run `--mode` remains
      independent. The persisted setting applies on the next normal startup, per user direction.

- [ ] PART 11 compliance configuration: all eleven standards now have independent `server.yml`
      booleans, any enabled standard activates encrypted-backup enforcement, and the maintenance
      report lists enabled standards. Standard-specific behaviors and privacy/data-rights workflows
      remain incomplete; do not represent the standards as fully enforced until those are resolved.

- [ ] PART 12 public contact and hooks: the contact page now issues single-use arithmetic
      challenges and the POST handler validates them, checks bounded fields, and submits through
      configured SMTP. Remaining: verify full delivery, support configured external CAPTCHA
      providers, provision webhook signing secrets, dispatch contact events, and verify tracking,
      consent, and all Tor privacy behavior.

- [ ] PART 12 session settings: global config, validation, generated YAML, documentation, and a
      cookie builder now use the user-selected defaults (`session`, browser-session lifetime,
      Secure auto, HttpOnly, SameSite=Lax). No session issuance path exists because product rules
      say the app has no sessions; do not add a stateful session model or change CSRF, consent, or
      preference-cookie behavior without a defined issuance purpose (AI.md 17427, 26379, 26724-27059).

- [ ] PART 32: the CLI, automatic TUI, setup/connect flow, GUI build-tag path, and generic API
      explorer are implemented. Remaining client gaps include full parity across
      every specified output, localization, accessibility, and lifecycle interaction. The server
      now discovers matching CLI assets/checksums from the project GitHub Release and streams the
      platform asset through `/cli/binaries`; `cli.binary_download.require_auth` is configurable.

- [ ] PART 17 template tools: validate unknown template variables before save and implement the
      specified editor/autocomplete/preview behavior once its API-only editing surface is clear.
- [ ] PART 20: per-service routes and token separation are wired, and the Grafana dashboard has
      panels for every required category. All application-owned metric families, including the
      five Go runtime metrics, use the required `airports_` prefix; the generic process collector
      is omitted because it exports unprefixed `process_*` metrics. System CPU, memory, disk, Go runtime, and database
      connection metrics are exported. Cache hit/miss metrics are tracked for memory and Redis;
      memory also reports expiration evictions and live logical size/bytes. Redis size/bytes/eviction
      metrics remain unavailable: go-redis pool stats describe connection reuse, while Redis
      `INFO evicted_keys` and `DBSIZE` include unrelated clients; prefix scans scale with database
      size and may include other instances, while keyspace notifications require server-wide
      config and can lose events. Loki returns bounded,
      sanitized recent logs. Database query/duration/error, scheduler, rate-limit, and auth metrics
      are implemented; auth counts the protected metrics service-token gate and reports zero active
      user sessions because the product has no login/session model. Tor enabled/running/circuit
      gauges and `tor_requests_total` use validated Tor circuit metadata from a dedicated
      PROXY-protocol listener. Grafana import validation, metrics YAML coverage, Redis metrics, and
      once-at-startup logging for
      disabled services also remain.

- [ ] PART 20 remaining metrics: accurately account for Redis size/eviction metrics without
      cross-client attribution, validate the Grafana dashboard in Grafana, and complete required
      metric documentation/configuration coverage.
- [ ] PART 13: `/server/healthz` now maps unhealthy/maintenance/shutting_down to HTTP 503, tracks
      restart/maintenance/shutdown state, and manual/automatic maintenance blocks writes. Runtime
      recovery probes DB/filesystem health, runs cleanup/retries, and sends configured notifications.
      Startup DB-open/schema failures still occur before the recovery monitor is available; I2P
      provider runtime verification and the complete recovery test pass remain open.
- [ ] PART 31.2: I2P opt-in lifecycle and scheduler health task are implemented. Primary test/lint
      verification and a real i2pd/SAM provider run remain before this work can be resolved and
      removed from TODO.AI.md.
- [ ] PART 28: the Docker-wrapped SSR/no-JS/full-JS browser harness now exists and covers major
      Airports pages, maps, theme, language, static assets, responsive layout, and browser errors.
      SSR API checks now cover JSON/CSV/GeoJSON exports and GraphQL airport lookup. Remaining
      scenarios include complete per-feature Tier 1/2/3 workflows, including all applicable
      read/error paths. Deterministic positive GeoIP and controlled HTTP 500 browser flows now
      pass. `./tests/e2e.sh` passed on 2026-10-26.

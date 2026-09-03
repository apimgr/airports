# TODO.AI.md

## AI tool rule cheatsheets — DONE

All 13 files generated in `.claude/rules/` per AI.md PART 0 → "Rule Files to Create/Update":
`ai-rules.md`, `project-rules.md`, `config-rules.md`, `binary-rules.md`, `backend-rules.md`,
`api-rules.md`, `frontend-rules.md`, `features-rules.md`, `service-rules.md`, `makefile-rules.md`,
`docker-rules.md`, `cicd-rules.md`, `testing-rules.md`.

Note: `.claude/` is gitignored per PART 3 — these files are local-only and never committed;
regenerate on any fresh clone or whenever AI.md is updated.

## Deferred verification (outside PART 0-6 scope)

- [x] Full compliance sweep against PART 7-33 — DONE (audit run 2026-07-31). Findings recorded
      in `AUDIT.AI.md` and fixed in the 2026-07-31 fix-all pass (see commit history); `AUDIT.AI.md`
      has been deleted now that every item is resolved.

## Env vars — resolved (2026-08-05)

Re-verified against AI.md PART 8's canonical env var table before implementing:
- [x] `DOMAIN` — implemented (`resolveFQDN` in src/main.go), overrides `cfg.Server.FQDN`
      before any other resolution source, per PART 8/13.
- [x] `TZ` — implemented (`applyTZEnv` in src/main.go, sets `time.Local`); `time/tzdata` is
      blank-imported so it works in the zoneinfo-less Alpine runtime image. Also fixed a
      related pre-existing bug found while wiring this up: `server.scheduler.timezone` was
      loaded into config but never passed to the cron scheduler — added
      `scheduler.NewWithLocation` and wired both `main.go` call sites to use it (falls back
      to `time.Local`, which TZ already sets, when unset/invalid).
- [x] `DATABASE_DIR` — implemented (`src/db/db.go` `Open`), overrides
      `paths.GetSQLiteDBPath()`.
- [x] `LOG_LEVEL` — confirmed NOT part of AI.md's spec (PART 8's canonical env var table has
      no `LOG_LEVEL` row; log level comes from `server.logging.level` config only). The
      original TODO entry was inaccurate. Not implemented; `docs/configuration.md` documents
      why.
- [x] `ENABLE_TOR` — confirmed NOT wanted: AI.md PART 31 states explicitly "No `ENABLE_TOR`
      flag needed" — Tor auto-enables whenever the `tor` binary is present. The line 7618
      `os.Getenv("ENABLE_TOR")` reference in AI.md is a generic `ParseBool()` usage example,
      not a requirement for this project. Not implemented; `docs/configuration.md` documents
      why.

## go-lint findings — resolved as false positives (2026-08-05 review)
- [x] src/scheduler/scheduler.go:88 — `cron.New()` (robfig/cron/v3) confirmed NOT an
      external-scheduler violation: the package is used only for in-process schedule
      parsing/ticking inside the built-in scheduler goroutine; no cron/systemd timer/Task
      Scheduler/launchd/K8s CronJob is invoked. Complies with AI.md PART 18. No code change
      needed.
- [x] src/graphql/theme.go — bundled React/GraphiQL playground confirmed NOT a violation of
      the server-side-Go-templates rule (AI.md PART 16 applies to the app's own UI under
      `static/js/app.js`, not vendored third-party dev tooling). GraphiQL is served as an
      isolated interactive query tool at `assetsPrefix`, matching PART 14's "Swagger/GraphQL
      location" allowance. No code change needed.

## go-lint findings — fixed (2026-08-05 re-run)
- [x] `src/paths` directory renamed to `src/path` (AI.md PART 3: Go package directories are
      singular). Import paths updated in all 6 consumers (`src/main.go`, `src/main_test.go`,
      `src/db/db.go`, `src/server/blocklist.go`, `src/service/service.go`,
      `src/tor/dirs.go`); the `package paths` clause inside the directory was left unchanged
      (Go doesn't require package identifier to match directory name, and renaming the
      identifier to `path` would shadow the stdlib `path` package for any future importer).
- [x] `docker/Dockerfile` — added `GOFLAGS=-buildvcs=false` and `-trimpath` to both `go build`
      invocations (server + CLI), matching the Makefile's `build`/`release`/`docker` targets
      per AI.md PART 25.
- [x] src/scheduler/scheduler.go `cron.New()` — reflagged by the same lint pass; already
      resolved as a false positive above, no further action.

## Deferred from 2026-08-05 full compliance sweep (AUDIT.AI.md)

All `[x]` findings from that sweep are fixed and committed. The following are unimplemented
features / larger restructuring the audit deferred — too large/risky for an uncommitted audit
fix. Tracked here per "no issue left only in conversation."

- [ ] PART 32: client uses stdlib `flag` not cobra/viper; no bubbletea TUI; no native GUI; no
      `--update` self-update with SHA-256; setup wizard doesn't test connection before saving.
      MAJOR feature work — the client is essentially a thin CLI vs the full spec.
- [ ] PART 7: `src/common/display` / `display.DetectDisplayEnv()` package does not exist; neither
      binary auto-detects GUI/TUI/CLI/Headless. Large, cross-binary.
- [x] PART 16: restructured `src/server/templates/*.html` into `src/server/template/{layout,partial,page,component}/*.tmpl`
      per AI.md's authoritative structure; added `src/common/theme/colors.go` shared palette;
      split `main.css` into `common.css`/`components.css`/`public.css` (load order common -> components -> public).
- [ ] Minor compose deviations (healthcheck intervals, dev DEBUG/container_name, prod MODE/LISTEN),
      email-template embed path, resolveFQDN order, version v-prefix logic — re-derive specifics
      from AI.md on pickup (not re-detailed here to avoid drift from a stale paraphrase).

## Discovered during 2026-08-14 Site Banner implementation — resolved (2026-08-30)

- [x] `public.tmpl`'s skip-link used key prefix `common.` (`{{t "common.skip_to_content"}}`) for a
      string actually stored under `nav.skip_to_content` in every locale file. Fixed to
      `{{t "nav.skip_to_content"}}` in `src/server/template/layout/public.tmpl` line 6. Verified all
      7 locale files (`en/es/zh/fr/ar/de/ja.json`) carry `nav.skip_to_content` and no remaining
      `common.skip_to_content` references exist anywhere in `src/`.

## Discovered during 2026-08-29 CI/CD audit (AI.md PART 27 vs source code) — resolved (2026-08-30)

- [x] Migrated the full build-info pipeline to AI.md PART 25/27's `BUILD_EPOCH` convention:
      - `Makefile`: captures `BUILD_EPOCH` (Unix seconds), derives `BUILD_DATE` from it for Docker
        OCI labels only, embeds `-X 'main.BuildEpoch=...'` (not `BuildDate`) in `LDFLAGS`, and passes
        `--build-arg BUILD_EPOCH=$(BUILD_EPOCH)` to `make docker`.
      - `docker/Dockerfile`: builder stage now takes `ARG BUILD_EPOCH` and embeds
        `-X 'main.BuildEpoch=${BUILD_EPOCH}'` in both `go build` invocations; runtime stage keeps
        `ARG BUILD_DATE` (consumed by CI's `org.opencontainers.image.created` annotation) and adds
        `ARG BUILD_EPOCH` alongside it, matching AI.md's canonical runtime-stage ARG set.
      - `src/main.go`: added `BuildEpoch = "0"` var, `buildEpoch() int64` helper (parses the ldflag,
        used for update-channel freshness comparisons), and an `init()` that derives `BuildDate`
        (RFC 3339 UTC) from `BuildEpoch` when set — all existing `BuildDate` call sites
        (`--version`, `server.BuildDate`, startup log/banner) are unaffected.
      - `src/client/main.go`: same `BuildEpoch`/`buildEpoch()`/`init()` pattern added — a go-lint
        pass caught that `release.yml` already passes `-X 'main.BuildEpoch=...'` to the CLI build
        too, but the client had no matching var, so the ldflag silently no-op'd and the client's
        `BuildDate` would have stayed `"unknown"` forever. Fixed to match the server.
      - `.github/workflows/release.yml`: `Set build info` step now captures `BUILD_EPOCH` and
        derives `BUILD_DATE` from it; both `go build -ldflags` strings switched from
        `main.BuildDate` to `main.BuildEpoch`. `.github/workflows/docker.yml` was already compliant
        (only uses `BUILD_DATE`/`BUILD_EPOCH` for OCI labels/annotations, never a Go ldflag) —
        verified, left unchanged.
      - Verified: `make test` passes (64.2% coverage, all packages green) after every change.
      - Note: the project's daily-update-channel detection (`matchesBranch`/`fetchReleaseForBranch`
        in `src/main.go`) uses a per-build unique 14-digit timestamp tag compared by string
        inequality against `Version`, not AI.md's generic single-rolling-`"daily"`-tag example that
        compares `PublishedAt.Unix()` against `buildEpoch()`. This project's scheme has no
        same-tag-freshness failure mode (each daily release gets a distinct tag), so `buildEpoch()`
        was added for embedding/exposure parity with AI.md but intentionally not wired into
        `matchesBranch`/`fetchReleaseForBranch` — wiring it in would not fix anything and would
        deviate from this project's already-correct, deliberate daily-tag design.
      - Note: `go-lint` flagged `BuildDate` not being directly ldflag-set as a rule violation — this
        is a false positive against the generic checker; AI.md PART 25/27 explicitly mandates this
        exact BuildEpoch-embedded/BuildDate-derived-at-runtime pattern, and AI.md wins over generic
        tooling conventions per the spec hierarchy.

## Discovered while implementing the 2026-08-29 AI.md robots/SEO update — deferred (not yet logged)

Found incidentally while implementing the robots.txt/geoip-attribution/CSS pass (commit `a198cff8`);
not part of that change's scope, too large to fix inline. Confirmed unbuilt by direct grep, not
assumption.

- [ ] PART 8: no startup console banner exists at all (`grep -rn "func.*[Bb]anner\|printBanner\|startupBanner"`
      across `src/main.go`/`src/server/*.go` returns nothing) — no FQDN/proto/port resolution display,
      no responsive full/compact/minimal/micro width adaptation, no `NO_COLOR`/`--color` handling for
      it. `src/server/*banner*` files that exist are the unrelated in-app site-announcement-banner
      feature, not this console startup banner.
- [ ] PART 31: I2P support is entirely unbuilt — the only repo-wide match for "i2p" is a stray
      reference in `src/notify/globalvars.go` (a config var name), not an actual I2P
      listener/hidden-service implementation. Tor (PART 31's other half) is fully implemented;
      I2P is not.
- [ ] PART 7: still unbuilt as of this note (re-confirmed) — no `display.DetectDisplayEnv()` package,
      matches the existing 2026-08-05 entry above; not re-detailed twice, cross-referencing only.

# Testing & Development, ReadTheDocs, I18N & A11Y Rules (PART 28, 29, 30)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

> Quick reference only: read the assigned AI.md PARTs completely before acting. AI.md is authoritative when this summary is incomplete or differs.

## CRITICAL - NEVER DO

- Run `go build`/`go test`/binaries directly on the local machine — Go is not installed there; always Docker (`casjaysdev/go:latest`) or Incus
- Skip `make test` before committing or the applicable manual binary checks before release; E2E is on demand
- Commit when `make test` fails or coverage is below 60%
- Use `docker-compose.yml` / `docker-compose.dev.yml` as AI — human-only; AI uses `docker-compose.test.yml` via `tests/` scripts
- Put runtime/test data under the project directory or bare `/tmp` — always `/tmp/{project_org}/{internal_name}-XXXXXX/`
- Use `pkill -f`/`killall`/`kill -9` first, or any `docker *prune`/`rm -rf` broad sweep — project-scoped exact PID/name only
- Put non-ReadTheDocs files in `docs/` — it is ONLY MkDocs/RTD content
- Use a config filename other than `mkdocs.yml`/`.readthedocs.yaml` for docs config
- Hardcode any user-facing string — every string (web, API, Swagger, GraphQL, email, CLI, agent, health page, cookie consent, legal pages) MUST use a translation key
- Ship a language file with missing/extra keys vs `en.json`, or error/crash on an unsupported `--lang`/`Accept-Language` — always silently fall back to `en`
- Convey information by color alone, or ship interactive UI without keyboard access, focus indicators, or ARIA landmarks

## CRITICAL - ALWAYS DO

- Create/update the matching `*_test.go` in the same work pass whenever package logic is added or changed
- Run `make test` (Phase 1, toolchain gate) before every commit; run binary validation scripts in `./tests/` manually before release. E2E is on demand and is not a commit gate
- Keep the tiers distinct: `tests/run_tests.sh` invokes `tests/docker.sh` for the binary-only basic Alpine tier; `tests/incus.sh` is the explicit full-app/init-system check; shipped-image verification is a separate check; E2E remains separate
- Test every route with all applicable Accept headers (`text/html`+`text/plain` for frontend; `application/json`+`text/plain` for API) and every `.txt` endpoint
- Keep `tests/run_tests.sh` Docker-only; it runs `tests/docker.sh` and exits `69` when Docker is missing. `tests/incus.sh` is a separate, developer-invoked full-app check and must not be used as a fallback.
- Keep `docs/` covering the full shipped surface (browser, CLI, API, config, well-known/discovery) with the 8 required pages (index, installation, configuration, api, cli, security, integrations, development)
- Maintain WCAG AA contrast (4.5:1) and both dark/light themes if customizing MkDocs Material colors
- Route every human-readable string through `t()`/`{{t .Lang key}}`; keep all locale files' keys identical to `en.json` (validated by the i18n tests in `make test`)
- Support the fallback chain `?lang= → cookie → Accept-Language → en` (web) and `--lang → config → LANG/LC_ALL → en` (CLI/server/agent)
- Set `dir="rtl"` for Arabic and use CSS logical properties; use CLDR plural categories per language
- Provide skip links, ARIA live regions for dynamic content, proper focus management, and 44x44px touch targets on every page

- Curl-based autodiscovery tests request `Accept: application/json`; helpers parse `data.api.current_version` with JSON tooling, never `sed`.

## Key Rules Summary

| Area | Rule |
|------|------|
| Test phases | Phase 1 `*_test.go` via `make test` (≥60% coverage, pre-commit gate); Phase 2 `tests/run_tests.sh` invokes `tests/docker.sh` for binary-only basic Alpine coverage; `tests/incus.sh` runs full-app/init-system checks; shipped-image verification and `./tests/e2e.sh` are separate/on-demand, never commit gates |
| Test file naming | `config_test.go`, `server_test.go`, `*_handler_test.go`, `mode_test.go`, `ssl_test.go`, `paths_test.go` — colocated with source |
| Coverage tooling | `go test -cover -coverprofile=coverage.out ./...` + `go tool cover -func=coverage.out`; CI fails build if `< 60` |
| Unit-test container | Docker toolchain image `casjaysdev/go:latest` via `make test`; never `alpine:latest` |
| Basic binary container | `tests/docker.sh` runs the compiled binary in `alpine:latest`; init-system and service lifecycle coverage belongs to `tests/incus.sh` |
| Integration containers | Incus `debian/trixie` (systemd) and `alpine/{latest_stable}` (OpenRC); shipped-image verification is separate from binary-only smoke coverage |
| Temp dir pattern | `/tmp/{project_org}/{internal_name}-XXXXXX/` — never bare `/tmp`, never in project dir |
| docs/ required files | `index.md`, `installation.md`, `configuration.md`, `api.md`, `cli.md` (if applicable), `security.md`, `integrations.md`, `development.md`, `requirements.txt` |
| RTD config | `mkdocs.yml` (Material theme, dark/light/auto) + `.readthedocs.yaml` (`os: ubuntu-24.04`, latest stable Python) |
| RTD URL | `https://{project_org}-{project_name}.readthedocs.io`; change only to the URL shown in the ReadTheDocs dashboard and update badges/links together |
| i18n locale files | `src/common/i18n/locales/{lang}.json`, embedded via `go:embed`, shared by ALL binaries |
| Supported languages | Minimum: en, es, fr, ar (rtl), de, it; this project also supports zh and ja. Every registered language must have complete locale-key parity |
| i18n fallback | Missing key → `en` value; unsupported lang → `en`; never error |
| a11y standard | WCAG 2.1 AA — 4.5:1 text contrast, 3:1 UI/focus, full keyboard nav, 44x44px touch targets |
| a11y testing tools | axe DevTools, WAVE, Lighthouse, NVDA/VoiceOver, manual keyboard-only pass |

For complete details, see AI.md PART 28, PART 29, PART 30.

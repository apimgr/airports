# Testing & Docs Rules (PART 28, 29, 30)

Cheatsheet — see AI.md PART 28 (line 39349), PART 29 (line 41141), PART 30 (line 41964).

## Testing Tiers

| Tier | Location | Tool | Purpose |
|------|----------|------|---------|
| Unit | `*_test.go` next to source | `go test` | Logic tested in isolation |
| Integration | `test/integration/` | `go test -tags integration` | End-to-end Go tests |
| E2E | `test/e2e/` | Shell + curl | HTTP-level smoke checks |
| Repo-root scripts | `tests/` | Bash | Container/VM beta-testing |

## Required Shell Scripts in `tests/`

| Script | Purpose |
|--------|---------|
| `tests/run_tests.sh` | Auto-detect Incus or Docker, run the suite |
| `tests/docker.sh` | Beta test inside a Docker container |
| `tests/incus.sh` | Beta test inside an Incus VM (preferred for systemd) |

All scripts: `#!/usr/bin/env bash`, `set -euo pipefail`, org-prefixed temp dirs (`mktemp -d "${TMPDIR:-/tmp}/apimgr/airports-XXXXXX"`), trap cleanup on exit.

## Test Compose Workflow (REQUIRED)

Always copy `docker/docker-compose.test.yml` to a temp dir before `docker compose up`. Never run from project dir (volume paths would resolve to repo).

## Test Network

- Always a NAMED bridge network — never default bridge, never `--network host`.
- Containers run with `--rm`.

## ReadTheDocs (PART 29)

- Engine: MkDocs Material.
- Theme files: `docs/stylesheets/dark.css`, `docs/stylesheets/light.css`.
- `docs/` is **EXCLUSIVELY** for MkDocs — no other files.
- `mkdocs.yml` at **REPO ROOT ONLY** (never in `docs/`).

### Required `docs/` files

| File | Purpose |
|------|---------|
| `index.md` | Documentation homepage |
| `installation.md` | Install guide (Docker, binary, systemd) |
| `configuration.md` | All settings |
| `api.md` | API endpoints, formats (lowercase filename) |
| `cli.md` | CLI reference (this project has `airports-cli` → required) |
| `admin.md` | Admin/operator guide (this project: no admin panel — state it) |
| `security.md` | Security model, public endpoints, security reporting |
| `integrations.md` | External integrations (GeoIP, ip-location-db, etc.) |
| `development.md` | Contributing/development guide |
| `requirements.txt` | Python deps for MkDocs |
| `stylesheets/dark.css` | Optional dark theme |
| `stylesheets/light.css` | Optional light theme |

### `mkdocs.yml` (root) requirements

- `theme: name: material` with dark/light/auto palette toggles.
- `extra_css:` references `stylesheets/dark.css` and `stylesheets/light.css`.
- `repo_name`, `repo_url`, `edit_uri`.
- `nav:` lists every required doc page.

### `.readthedocs.yaml`

- `version: 2`, `build.os: ubuntu-24.04`, `build.tools.python: "3.12"`.
- `mkdocs.configuration: mkdocs.yml`.
- `python.install.requirements: docs/requirements.txt`.

## I18N & A11Y (PART 30)

- All user-facing strings extractable for translation; `lang="en"` on `<html>`.
- Date/time formatted via locale-aware helpers.
- Numbers and units localized.
- WCAG 2.1 AA minimum.
- Keyboard reachable; visible focus ring.
- Color contrast ≥ 4.5:1 body, ≥ 3:1 large text.
- `prefers-reduced-motion` respected.
- All form fields `<label>`-associated.
- Semantic HTML; ARIA only when needed.

## Test Quality Gates

- `go test -race -timeout 5m ./...` must pass.
- `go vet ./...` must pass.
- `staticcheck ./...` must pass (if installed).
- `CGO_ENABLED=0` for all builds.
- No flaky tests — investigate root cause, don't retry-loop.

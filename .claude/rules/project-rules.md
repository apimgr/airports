# Project Rules (PART 2, 3, 4)

Cheatsheet — see AI.md PART 2 (line 5814), PART 3 (line 6146), PART 4 (line 7121).

## License & Attribution

- License: **MIT** in `LICENSE.md` (markdown filename; never `LICENSE`).
- Third-party / embedded license attributions appended to bottom of `LICENSE.md`.
- README first line should reference the official site / repo.
- NO AI attribution anywhere: no `Co-Authored-By:`, no "Generated with X" footers.

## Project Variables (from IDEA.md)

| Var | Value |
|-----|-------|
| project_name | airports |
| project_org | apimgr |
| internal_name | airports (frozen) |
| binary | airports |
| client_binary | airports-cli |
| repo | https://github.com/apimgr/airports |

`{internal_name}` is **frozen** — used in all on-disk identifiers (config_dir, data_dir, systemd unit, plist). `{project_name}` may change on rename; `{internal_name}` may NOT.

## Required Directory Structure

```
./
├── .github/workflows/       # release.yml, beta.yml, daily.yml, docker.yml
├── .claude/                 # settings.json, rules/, agents/
│   └── rules/              # 13 cheatsheet files
├── docs/                    # ReadTheDocs MkDocs source (ONLY RTD files)
├── src/                     # All Go source
├── scripts/                 # Production/install scripts
├── tests/                   # Repo-root integration test scripts
│   ├── run_tests.sh
│   ├── docker.sh
│   └── incus.sh
├── docker/                  # Dockerfile, docker-compose.yml, rootfs/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yml
│   ├── docker-compose.test.yml
│   └── rootfs/usr/local/bin/entrypoint.sh
├── binaries/                # Build output (gitignored)
├── releases/                # Release output (gitignored)
├── volumes/                 # Runtime data (gitignored)
├── README.md
├── LICENSE.md
├── AI.md
├── IDEA.md
├── CLAUDE.md
├── Makefile
├── mkdocs.yml               # ROOT only — never in docs/
├── .readthedocs.yaml
└── release.txt              # Sole version source of truth
```

## File Naming Rules

- `README.md` (NEVER `readme.md`, `Readme.md`)
- `LICENSE.md` (NEVER `LICENSE`, `license.md`)
- No `CHANGELOG.md`, `AUDIT.md`, `SUMMARY.md`, `NOTES.md`, `REPORT.md`, `ANALYSIS.md`.
- Source directories are **singular**: `handler/`, `model/`, `middleware/` — not plural.
- Exception (tooling convention): `scripts/`, `tests/`, `completions/`, `binaries/`.

## Forbidden at Repo Root

- No `Dockerfile` (must be `docker/Dockerfile`)
- No `docker-compose.yml` (must be `docker/docker-compose.yml`)
- No `.env` / `.env.example` files anywhere
- No `config/`, `data/`, `logs/`, `tmp/`, `temp/`, `build/`, `dist/`, `out/`, `vendor/`, `node_modules/`

## OS-Specific Paths (PART 4)

| OS | Config | Data | Log | Cache |
|----|--------|------|-----|-------|
| Linux (root) | `/etc/{internal_name}` | `/var/lib/{internal_name}` | `/var/log/{internal_name}` | `/var/cache/{internal_name}` |
| Linux (user) | `~/.config/{internal_name}` | `~/.local/share/{internal_name}` | `~/.local/state/{internal_name}/log` | `~/.cache/{internal_name}` |
| macOS | `~/Library/Application Support/{internal_name}` | same | `~/Library/Logs/{internal_name}` | `~/Library/Caches/{internal_name}` |
| Windows | `%APPDATA%\{internal_name}` | `%LOCALAPPDATA%\{internal_name}` | `%LOCALAPPDATA%\{internal_name}\logs` | `%LOCALAPPDATA%\{internal_name}\cache` |
| Container | `/config/{internal_name}` | `/data/{internal_name}` | `/data/log/{internal_name}` | `/data/{internal_name}/cache` |

`{plist_name}` (macOS): `io.github.{project_org}.{internal_name}` → `io.github.apimgr.airports`.

## Always Detect at Runtime

- Hostname, IP, CPU count, memory size — NEVER hardcode dev machine values.
- Display environment (TTY, terminal, headless) for output formatting.

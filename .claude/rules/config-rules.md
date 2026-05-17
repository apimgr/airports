# Configuration Rules (PART 5, 6, 12)

Cheatsheet — see AI.md PART 5 (line 7315), PART 6 (line 9231), PART 12 (line 17404).

## Configuration Hierarchy (highest to lowest precedence)

1. CLI flags
2. Environment variables
3. Config file (`server.yml`)
4. Built-in defaults

Every setting MUST be expressible at all four layers.

## Config File

- Sole file: `{config_dir}/server.yml` (YAML).
- Auto-generated on first run with sane defaults.
- No `.env` files — ever. Sane defaults are hardcoded in compose and the binary.
- Boolean parsing: use project `config.ParseBool()` (handles 40+ variants), NEVER `strconv.ParseBool()`.

## Application Modes

| Mode | Env | Purpose | Behavior |
|------|-----|---------|----------|
| `development` | `MODE=development` | Local dev | Allows `localhost`, `.local`, `.test`; relaxed CORS; verbose logs |
| `production` | `MODE=production` | Live deployment | Strict CORS; minimal logs; cache enabled |
| `maintenance` | `--maintenance` flag | Admin tasks | Read-only; serves maintenance page |

`DEBUG=true` is an independent flag — enables pprof/expvar/detailed request logs. Never default-on in production.

## Server Configuration Surface

- `MODE` — development | production
- `PORT` — internal port (containers: always 80; host detect)
- `ADDRESS` — bind address (default `0.0.0.0`)
- `DOMAIN` — comma-separated allowed hosts (optional; auto-detect behind reverse proxy)
- `TZ` — timezone (default `America/New_York`)
- `CONFIG_DIR`, `DATA_DIR`, `LOG_DIR`, `CACHE_DIR` — override OS defaults
- `DATABASE_DIR` — SQLite location (container default `/data/db/sqlite`)
- SMTP block (optional; auto-detect): `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`
- TLS block: cert/key paths, ACME settings
- Rate-limit block (per endpoint class)

## First-Run Behavior

1. Detect OS and resolve `{config_dir}`, `{data_dir}`, `{log_dir}`.
2. Create directories if missing.
3. If `server.yml` missing: write defaults.
4. Show banner with URLs, version, commit ID, build date.
5. Start server.

## Hardcoding Bans

- NEVER hardcode hostname, IP, CPU count, memory size, port that differs by host.
- NEVER hardcode `/tmp` — use `$TMPDIR` / `os.TempDir()`.
- Temp dirs must be org-prefixed: `mktemp -d "${TMPDIR:-/tmp}/apimgr/airports-XXXXXX"`.

## Container Path Defaults

| Container Path | Purpose |
|----------------|---------|
| `/config/airports/` | App config, ssl/, tor/ |
| `/data/airports/` | App data, uploads, cache, tor/ |
| `/data/db/sqlite/` | SQLite databases (`server.db`, `users.db`) |
| `/data/log/airports/` | Logs |
| `/data/backups/airports/` | Backup archives |

Compose mounts two volumes only: `./volumes/config:/config:z` and `./volumes/data:/data:z`.

## Validation Rules

- Reject unknown config keys (strict YAML decode).
- Type-validate all values; bounds-check ranges (ports 1-65535, timeouts > 0, etc.).
- Trim whitespace from string inputs.
- Passwords must NOT start/end with whitespace — reject, don't trim.

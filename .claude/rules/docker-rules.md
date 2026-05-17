# Docker Rules (PART 26)

Cheatsheet — see AI.md PART 26 (line 34850).

## Layout

```
docker/
├── Dockerfile                  # Production
├── Dockerfile.dev              # Optional
├── docker-compose.yml          # Production - HUMAN USE ONLY
├── docker-compose.dev.yml      # Dev - HUMAN USE ONLY
├── docker-compose.test.yml     # Test - AI/AUTOMATED ONLY
└── rootfs/
    └── usr/local/bin/entrypoint.sh
```

- NEVER place `Dockerfile` or any `docker-compose.yml` at repo root.
- `docker/rootfs/` mirrors container FS at build time.
- Build context: project root `.`; Dockerfile referenced as `-f docker/Dockerfile`.

## Dockerfile Requirements

| Item | Value |
|------|-------|
| Build type | Multi-stage (builder + runtime) |
| Builder | `golang:alpine` |
| Runtime | `alpine:latest` |
| Required packages | git, curl, bash, tini, tor |
| Init | `tini` — `ENTRYPOINT ["tini","-p","SIGTERM","--","/usr/local/bin/entrypoint.sh"]` |
| STOPSIGNAL | `SIGRTMIN+3` |
| Internal port | 80 (always) |
| HEALTHCHECK | `--start-period=10m --interval=5m --timeout=15s --retries=3` running `airports --status` |
| Binary location | `/usr/local/bin/airports` |
| ENV MODE | `development` (image default) |

## Container Paths

| Path | Purpose |
|------|---------|
| `/config/airports/` | App config, `ssl/`, `tor/` |
| `/data/airports/` | App data, `uploads/`, `cache/`, `tor/` |
| `/data/db/sqlite/` | SQLite DBs (`server.db`) |
| `/data/log/airports/` | App logs |
| `/data/backups/airports/` | Backup archives |

## Volume Mounts (compose files)

Two volumes only:

```yaml
volumes:
  - ./volumes/config:/config:z
  - ./volumes/data:/data:z
```

Binary creates subdirectories on first run.

## Required OCI Labels

`maintainer`, `org.opencontainers.image.vendor`, `.authors`, `.title`, `.base.name`, `.description`, `.licenses`, `.created`, `.version`, `.schema-version`, `.revision`, `.url`, `.source`, `.documentation`, `.vcs-type`, `com.github.containers.toolbox`.

For multi-arch images: same labels must ALSO be set as manifest annotations (`manifest:org.opencontainers.image.*=...`) in the build workflow — registries read manifest annotations, not per-arch image configs.

## Entrypoint

- All container startup customization goes in `docker/rootfs/usr/local/bin/entrypoint.sh`.
- NEVER override ENTRYPOINT or CMD in compose files.
- Pass commands to entrypoint, not to binary directly.
- Chain: `tini → entrypoint.sh → airports`.

## docker-compose.yml (Production)

- `name: airports`, `container_name: airports-app`.
- `restart: always`.
- Port mapping: `172.17.0.1:64580:80` (bound to Docker bridge — reverse proxy handles external).
- `MODE=production`, no `DEBUG`.
- Hardcoded sane defaults — works with zero `.env` file.

## docker-compose.dev.yml (Development)

- `name: airports-dev`, `container_name: airports-dev`.
- `MODE=development`.
- Port `64580:80` (all interfaces).
- `DEBUG=true` available (commented).
- HUMAN USE ONLY — AI must not run this.

## docker-compose.test.yml (Test)

- `name: airports-test`, `container_name: airports-test`.
- `restart: "no"`.
- `MODE=development`, `DEBUG=true`.
- Port `64581:80`.
- AI/AUTOMATED ONLY. **MUST be copied to temp dir before `docker compose up`** — NEVER run from project dir.

### Test Workflow (REQUIRED)

```bash
mkdir -p "${TMPDIR:-/tmp}/apimgr"
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/apimgr/airports-XXXXXX")
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp docker/docker-compose.test.yml "$TEMP_DIR/docker-compose.yml"
cd "$TEMP_DIR" && docker compose up --abort-on-container-exit
rm -rf "$TEMP_DIR"
```

## Image Tagging

- `:latest` — most recent stable.
- `:{semver}` — pinned version.
- `:beta` — pre-release.
- `:daily-{YYYYMMDD}` — daily build.

## Run Conventions

- Every `docker run` uses `--rm` — no orphaned containers.
- Test containers use a named bridge network — never default bridge or `--network host`.
- Cleanup is part of the task that started the container.
- Never `docker system prune` or any broad sweep — only project-scoped removal.

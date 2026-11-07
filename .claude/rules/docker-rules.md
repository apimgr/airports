# Docker Rules (PART 26)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Place `Dockerfile` or `docker-compose.yml` in the project root — always under `docker/`
- Modify `ENTRYPOINT` or `CMD` — all customization goes in `entrypoint.sh`
- Put runtime setup (dirs, permissions, user/group, Tor) in `entrypoint.sh` — the binary owns all of that
- Run docker compose from the project directory, or mount volumes to `{project_root}/volumes/`
- Commit runtime `./volumes/` contents from local runs
- Use `${VAR}`/`${VAR:-default}` syntax or `.env`/`.env.example` files in compose — hardcode sane defaults
- Include `build:` or `version:` keys in any compose file
- Add `LABEL` blocks to the Dockerfile — all OCI metadata is applied by CI via `--label`/`--annotation`
- Run as root in the runtime stage unless the app must bind a privileged port or manage system services (document exception in IDEA.md)
- Use docker-compose.dev.yml or docker-compose.yml as the AI testing interface — those are HUMAN USE ONLY; AI uses `tests/` scripts or docker-compose.test.yml as fallback
- Push `:dev` or `:test` tags to the production registry

## CRITICAL - ALWAYS DO

- Use multi-stage builds: `casjaysdev/go:latest` builder → `alpine:latest` runtime
- Use `tini` as init (`ENTRYPOINT ["tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh"]`)
- Set `STOPSIGNAL SIGRTMIN+3`; end `entrypoint.sh` with `exec "$@"` (or `exec <binary> ... "$@"`)
- Expose internal port **80**; `HEALTHCHECK` via `{binary} --status` (start 10m, interval 5m, timeout 15s, retries 3 in Dockerfile; compose uses interval 10s/timeout 5s/retries 3/start_period 90s)
- Mount exactly two volumes: `./volumes/config:/config:z` and `./volumes/data:/data:z` (`:z` in production, omit in dev temp dirs)
- Run docker compose from a temp dir (`${TMPDIR:-/tmp}/{project_org}/{internal_name}-XXXXXX/`), never the repo
- Include the `x-logging` anchor (`max-size: '5m'`, `max-file: '1'`, `driver: json-file`) and apply it to every service
- Build for both `linux/amd64` and `linux/arm64` on release
- Keep `docker/rootfs/` as the only build-time overlay (committed); `./volumes/` is runtime-only (never committed)

## Key Rules Summary

| Area | Rule |
|------|------|
| Files | `docker/Dockerfile`, `docker/Dockerfile.dev`, `docker/docker-compose.yml`, `docker/docker-compose.dev.yml`, `docker/docker-compose.test.yml`, `docker/rootfs/usr/local/bin/entrypoint.sh` |
| Build context | Project root (`.`); `-f docker/Dockerfile` |
| Base images | Builder `casjaysdev/go:latest`; runtime `alpine:latest` |
| Binary path | `/usr/local/bin/{project_name}` |
| Container paths | `/config/{project_name}/` (config, ssl/, tor/); `/data/{project_name}/` (uploads, cache, tor/); `/data/db/sqlite/server.db`; `/data/log/{project_name}/`; `/data/backups/{project_name}/` |
| Non-root user | Alpine: `addgroup -S app && adduser -S -G app app` then `USER app` |
| Healthcheck | `CMD /usr/local/bin/{project_name} --status \|\| exit 1` |
| Ports | Internal always `80`; prod `172.17.0.1:64xxx:80`; dev `64xxx:80` |
| Compose env | Hardcoded YAML map style; prod sets no `DEBUG`/`MODE`; dev/test set `DEBUG: 1`, `MODE: dev` |
| Service naming | App `{project_name}`/`{project_name}-app`; cache `{project_name}-cache`; db `{project_name}-db`; proxy `{project_name}-proxy` |
| Compose variants | `.yml` → `:latest` + persistent Valkey; `.dev.yml` → `:devel`, no cache; `.test.yml` → `:latest` + ephemeral tmpfs Valkey |
| Image tags | `:latest`, `:{version}`, `:{YYMM}`, `:{commit}` (release, pushed); `:dev`/`:test` (local only) |
| Tor | Installed always, auto-enabled if `tor` binary present; binary owns config/data under `{project_name}/tor/` |

For complete details, see AI.md PART 26.

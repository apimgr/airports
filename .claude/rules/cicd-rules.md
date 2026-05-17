# CI/CD Rules (PART 27)

Cheatsheet — see AI.md PART 27 (line 36357).

## Workflow Files

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `.github/workflows/release.yml` | Tag `v*` push | Stable release — builds + uploads binaries + creates GH release |
| `.github/workflows/beta.yml` | Tag `v*-beta*` or push to `beta` branch | Beta release |
| `.github/workflows/daily.yml` | Schedule (cron) | Daily build artifact |
| `.github/workflows/docker.yml` | Push to `main`, tags | Build + push multi-arch Docker image |

## Strict Rules

- **NO `make` in CI.** Use explicit `docker buildx` / `docker compose` / direct commands with env vars inlined.
- **All third-party Actions pinned to full commit SHA** — never a tag.
  - Wrong: `uses: actions/checkout@v4`
  - Right: `uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11  # v4.1.1`
- **Least-privilege `permissions:`** declared on each job.
- **No secrets exposed to forked PR workflows.** Never use `pull_request_target` with untrusted code execution.
- **Reproducible builds** — same inputs → same binary.

## Required Build Args

Every Docker build passes:
- `--build-arg VERSION=$(cat release.txt)`
- `--build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)`
- `--build-arg COMMIT_ID=$(git rev-parse --short HEAD)`
- `--build-arg LICENSE=MIT`

## Multi-Arch Docker

Platforms: `linux/amd64,linux/arm64`.

Use `docker/build-push-action` (pinned to SHA) with:
- `platforms: linux/amd64,linux/arm64`
- `provenance: true`
- `sbom: true`
- `labels:` + `annotations:` — annotations apply to manifest index (registries read those for description).

## Registry

- Primary: `ghcr.io/apimgr/airports`.
- Tags: `latest`, `:{semver}`, `:beta`, `:daily-YYYYMMDD`.
- Authentication: `GITHUB_TOKEN` (built-in), permissions `packages: write`.

## Cross-Platform Binary Build

`release.yml` produces binaries for: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `freebsd/amd64`.

Each binary uploaded as GH release asset along with `SHA256SUMS` file (signed if signing configured).

## Caching

- Go module cache: `~/go/pkg/mod`.
- Build cache: `~/.cache/go-build`.
- Use `actions/cache@<sha>` keyed on `go.sum`.
- Docker layer cache: `type=gha`.

## Test Step

CI runs `go test -race -timeout 5m ./...` inside the same builder image used for production build.

## Triggers for Docker Workflow

- Push to `main` → tag `:latest`.
- Push of `v*` tag → tag `:{semver}` + `:latest`.
- Push of `v*-beta*` → tag `:beta` + `:{semver-beta}`.
- Manual `workflow_dispatch` allowed.

## Concurrency

- `concurrency:` group per workflow + ref — newer pushes cancel in-flight builds.

## Notifications

- Failed `release.yml` → GitHub issue auto-opened with logs link.
- No email or chat notifications unless explicitly configured.

## Forbidden in CI

- Running `go` directly on the runner host without Docker.
- Hardcoded paths (use `${{ github.workspace }}`).
- Inline secrets (always from `${{ secrets.* }}`).
- Skipping tests with `if: failure()` workarounds.

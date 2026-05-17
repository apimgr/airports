# Makefile Rules (PART 25)

Cheatsheet — see AI.md PART 25 (line 34070).

## Purpose

- Makefile is for **local development only**. Never used in CI/CD.
- All targets internally use Docker `golang:alpine` — host need not have Go installed.
- CI workflows call `docker build` and explicit commands directly, NOT `make`.

## Required Targets

| Target | Purpose |
|--------|---------|
| `make dev` | Quick build to `$TMPDIR/apimgr/airports-XXXXXX/` |
| `make local` | Build with version info to `binaries/` |
| `make build` | Full cross-platform build (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, freebsd/amd64) |
| `make test` | Unit tests inside Docker |
| `make lint` | Run `go vet`, `staticcheck` |
| `make clean` | Remove `binaries/`, `releases/`, build cache |
| `make docker` | Build Docker image |
| `make docker-test` | Run `docker/docker-compose.test.yml` workflow (copies to temp dir first) |
| `make help` | Print target list with descriptions |

## Build Variables (injected via `-ldflags`)

- `main.Version` — from `release.txt`
- `main.CommitID` — from `git rev-parse --short HEAD`
- `main.BuildDate` — `date -u +%Y-%m-%dT%H:%M:%SZ`
- `main.OfficialSite` — from `IDEA.md` repo URL

## Docker Invocation Pattern

```makefile
DOCKER_RUN := docker run --rm \
    -v $$(pwd):/build \
    -v $$HOME/.cache/go-build:/root/.cache/go-build \
    -v $$HOME/go/pkg/mod:/go/pkg/mod \
    -w /build \
    -e CGO_ENABLED=0 \
    golang:alpine
```

## Rules

- Every target must work from a clean checkout (no host Go toolchain).
- `make build` produces binaries named `{binary}-{os}-{arch}[.exe]` in `binaries/`.
- `make test` mounts read-only source where possible.
- Never `chmod`/`chown` outside `binaries/`, `releases/`, `$TMPDIR`.
- No target may push to a registry — that is the CI/CD's job (PART 27).
- `make clean` is non-destructive outside the project root (never `rm -rf` an unbound variable).
- All output goes through one logging style for consistency.

## Version Source of Truth

`release.txt` at repo root. Plain text. Single line semantic version (e.g., `1.2.3`). Everything else derives from it.

## What NOT to Do

- No installing Go on the host.
- No `make` in CI (use explicit `docker buildx`/`docker compose` commands with inline env vars).
- No interactive prompts inside Make targets.
- No silent failures — every target sets `-e` shell flag.

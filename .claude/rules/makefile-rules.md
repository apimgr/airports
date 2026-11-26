# Makefile & Build/Release System Rules (PART 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

> Quick reference only: read AI.md PART 25 completely before acting. AI.md is authoritative when this summary is incomplete or differs.

## CRITICAL - NEVER DO

- Add Makefile targets beyond the seven required core targets and two allowed helpers
- Run `go build`/`go test` on the host — every target shells into `casjaysdev/go:latest` via Docker
- Hardcode `PROJECT_NAME`/`PROJECT_ORG` — always derive from `git remote get-url origin` or directory path
- Add a `v` prefix to a non-numeric version tag (`dev`, `beta`, timestamps) — only `X.Y.Z` semver gets `v`
- Double an existing `v` prefix (`vv1.2.3`)
- Copy or symlink binaries anywhere (`/usr/local/bin`, PATH, between `binaries/`↔`releases/`) — run in place
- Skip or weaken the coverage gate (`-short`, `-count=0`, ignore directives) — 60% minimum always enforced
- Let an explicitly set user `VERSION` environment variable override a non-empty `release.txt`

## CRITICAL - ALWAYS DO

- Keep the seven core targets: `dev`, `local`, `build`, `test`, `release`, `beta`, `docker`; allow only `clean` and internal `release-check` helpers
- Build every platform via Docker with `CGO_ENABLED=0` and `GOFLAGS=-buildvcs=false`
- Embed `Version`, `CommitID`, `BuildEpoch`, and `OfficialSite` via `-ldflags`; derive `BuildDate` at runtime
- Use persistent Go caches: `GO_CACHE` (`~/go/pkg/mod`) and `GO_BUILD` (`~/.cache/go-build/{project_name}`)
- Run `make dev` → `make test` → `./tests/run_tests.sh` → `make local` → `./tests/incus.sh` → `make build` in that order before a release
- Strip binaries before packaging into `releases/` (`make release`)
- Derive local version from explicit `VERSION` env var > non-empty `release.txt` > short commit ID (`devel` outside Git)

## Key Rules Summary

| Area | Rule |
|------|------|
| Target: `dev` | Fast local-arch build, no ldflags, to `${TMPDIR}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX/` |
| Target: `local` | `clean` + local-arch build with full ldflags → `binaries/{project_name}[-cli]` |
| Target: `build` | `clean` + all 8 platforms (linux/darwin/windows/freebsd × amd64/arm64) → `binaries/` |
| Target: `test` | Regenerates embedded OpenAPI, runs `go test -cover ./...` in Docker; fails if coverage < 60% |
| Target: `release` | Runs `release-check` then `build`; strips binaries, tars source, writes SHA-256/SHA-512 sums, replaces the stable tag/release, and publishes with `gh` (manual/local only) |
| Target: `beta` | Tags the current `main` commit `YYYYMMDDHHMMSS-beta` and pushes the tag for CI publishing |
| Helper: `release-check` | First `release` prerequisite; rejects non-stable SemVer without building |
| Target: `docker` | `docker buildx build --push` multi-arch (`linux/amd64,linux/arm64`) via `docker/Dockerfile` |
| Toolchain image | `casjaysdev/go:latest` for every build/test target |
| Required env flags | `CGO_ENABLED=0`, `GOFLAGS=-buildvcs=false` on all Docker Go invocations |
| Binary naming | Server: `{project_name}[-os-arch[.exe]]`; CLI: `{project_name}-cli[-os-arch[.exe]]` |
| Output dirs | `binaries/` (build artifacts, gitignored), `releases/` (packaged, with `version.txt`) |
| Local version source | Explicit `VERSION` env > non-empty `release.txt` > short commit ID (`devel` outside Git); CI uses tag/devel/beta rules |
| Version tag `v` | Add only to semantic versions, never to text/timestamp (`devel`, `beta`, `20251218...`) |
| Release types | Stable (semver tag, unlimited), Beta (`{ts}-beta`, unlimited), Daily (`{ts}`, max 1 rolling) |
| Coverage gate | 60% minimum via `go test -cover`; CI + local `make test`; never skip |
| Binary handling | Never symlink/copy to PATH or system dirs; run directly from `binaries/` |

For complete details, see AI.md PART 25.

# Makefile & Build/Release System Rules (PART 25)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Add Makefile targets beyond the six core ones (`dev`, `local`, `build`, `test`, `release`, `docker`)
- Run `go build`/`go test` on the host — every target shells into `casjaysdev/go:latest` via Docker
- Hardcode `PROJECT_NAME`/`PROJECT_ORG` — always derive from `git remote get-url origin` or directory path
- Add a `v` prefix to a non-numeric version tag (`dev`, `beta`, timestamps) — only `X.Y.Z` semver gets `v`
- Double an existing `v` prefix (`vv1.2.3`)
- Copy or symlink binaries anywhere (`/usr/local/bin`, PATH, between `binaries/`↔`releases/`) — run in place
- Skip or weaken the coverage gate (`-short`, `-count=0`, ignore directives) — 60% minimum always enforced
- Let `release.txt` be overridden by env var or derived value when the file exists — file always wins

## CRITICAL - ALWAYS DO

- Keep exactly six targets: `dev`, `local`, `build`, `test`, `release`, `docker` (plus `clean` helper)
- Build every platform via Docker with `CGO_ENABLED=0` and `GOFLAGS=-buildvcs=false`
- Embed `Version`, `CommitID`, `BuildDate`, `OfficialSite` via `-ldflags` on every real build (not `dev`)
- Use persistent Go caches: `GO_CACHE` (`~/go/pkg/mod`) and `GO_BUILD` (`~/.cache/go-build/{project_name}`)
- Run `make dev` → `make test` → `./tests/run_tests.sh` → `make local` → `./tests/incus.sh` → `make build` in that order before a release
- Strip binaries before packaging into `releases/` (`make release`)
- Delete/replace the existing `daily` release each build — only one daily release may ever exist
- Derive version from `release.txt` (wins) > `VERSION` env var > `devel` fallback

## Key Rules Summary

| Area | Rule |
|------|------|
| Target: `dev` | Fast local-arch build, no ldflags, to `${TMPDIR}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX/` |
| Target: `local` | `clean` + local-arch build with full ldflags → `binaries/{project_name}[-cli]` |
| Target: `build` | `clean` + all 8 platforms (linux/darwin/windows/freebsd × amd64/arm64) → `binaries/` |
| Target: `test` | `go test -cover ./...` in Docker; fails if coverage < 60% (override upward only, in IDEA.md) |
| Target: `release` | Runs `build`; strips binaries, tars source, `gh release create --latest` (manual/local only) |
| Target: `docker` | `docker buildx build --push` multi-arch (`linux/amd64,linux/arm64`) via `docker/Dockerfile` |
| Toolchain image | `casjaysdev/go:latest` for every build/test target |
| Required env flags | `CGO_ENABLED=0`, `GOFLAGS=-buildvcs=false` on all Docker Go invocations |
| Binary naming | Server: `{project_name}[-os-arch[.exe]]`; CLI: `{project_name}-cli[-os-arch[.exe]]` |
| Output dirs | `binaries/` (build artifacts, gitignored), `releases/` (packaged, with `version.txt`) |
| Version source | `release.txt` (wins) > `VERSION` env > `devel` fallback |
| Version tag `v` | Add only to numeric semver (`1.2.3`→`v1.2.3`); never to text/timestamp (`dev`, `beta`, `20251218...`) |
| Release types | Stable (semver tag, unlimited), Beta (`{ts}-beta`, unlimited), Daily (`{ts}`, max 1 rolling) |
| Coverage gate | 60% minimum via `go test -cover`; CI + local `make test`; never skip |
| Binary handling | Never symlink/copy to PATH or system dirs; run directly from `binaries/` |

For complete details, see AI.md PART 25.

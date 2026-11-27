# Makefile & Build/Release System Rules (AI.md PART 25)

AI.md is authoritative. Read PART 25 before changing the Makefile.

## Required Targets

The Makefile has exactly six targets: `build`, `test`, `release`, `docker`, `clean`, and `help`. `help` is the default. Do not add targets or tag/publish helpers.

- `build` removes old artifacts and builds debug binaries for the local platform and all eight supported platforms in one `casjaysdev/go:latest` container. Keep symbols and DWARF, disable optimization/inlining with `-gcflags "all=-N -l"`, and do not use `-trimpath` or `-s -w`.
- `test` runs Docker quality gates, vulnerability scanning, tests with the coverage floor, then `tests/run_tests.sh`.
- `release` builds stripped production binaries into `releases/`, writes `version.txt`, creates the source archive and checksums, and never tags, pushes, or publishes.
- `docker` builds and loads the host-platform image locally, then checks it with `tests/image.sh`; it never pushes.
- `clean` removes only `binaries/` and `releases/`.
- `help` lists these targets without building anything.

## Build Rules

- Run every Go command in `casjaysdev/go:latest` with `CGO_ENABLED=0` and `GOFLAGS=-buildvcs=false`.
- Embed `Version`, `CommitID`, `BuildEpoch`, and optional `OfficialSite`; derive `BuildDate` at runtime.
- Use `GO_CACHE=~/go/pkg/mod` and `GO_BUILD=~/.cache/go-build/{project_name}` for persistent caches.
- Resolve version from explicit `VERSION`, non-empty `release.txt`, then the short Git commit ID; use `devel` outside Git.
- Read `OFFICIAL_SITE` from `site.txt`, then the environment, otherwise leave it empty. Never infer a hosted site.
- Never copy or symlink build artifacts into system paths. `make release` builds production artifacts fresh instead of copying from `binaries/`.
- CI spells out build and test commands; it never invokes Makefile targets.

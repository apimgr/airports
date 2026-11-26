# Development & Contributing

## Local Toolchain

The build runs entirely inside Docker. You **do not** need Go installed on the local machine. The only local requirements are:

- Docker (Engine 20.10+ or Docker Desktop)
- `git`
- `make` (any GNU make 3.81+)

Optional:

- [Incus](https://linuxcontainers.org/incus/) for full-OS integration tests on systemd and OpenRC.

## Clone and Build

```bash
git clone https://github.com/apimgr/airports.git
cd airports

make dev          # quick build to $TMPDIR/apimgr/airports-XXXXXX/
make local        # build with version info to binaries/airports
make build        # cross-compile all platforms to binaries/
make test         # run unit tests
make docker       # build the Docker image
```

`make release` publishes a stable SemVer release. `make beta` tags the clean
`main` commit with a UTC timestamp and pushes the tag for CI to publish; use
these maintainer-only targets only when publishing is intended.

The Makefile's Go targets use `casjaysdev/go:latest`; no local Go toolchain is
needed. CI runs `go vet ./...` and `staticcheck ./...` in the same project
toolchain image. Documentation can be built with `mkdocs build` after installing
`docs/requirements.txt`.

## Repository Layout

```
.
├── src/                  # Go source
│   ├── main.go           # Thin process entry point
│   ├── app/              # Server startup, CLI dispatch and app tests
│   ├── server/           # HTTP server, routes, handlers, middleware and templates
│   ├── client/           # airports-cli source
│   ├── config/           # Configuration loading and validation
│   ├── airports/         # Airport dataset and query behavior
│   ├── data/             # Embedded airport JSON
│   └── ...
├── docker/               # Dockerfile, compose files, rootfs overlay
├── docs/                 # ReadTheDocs MkDocs source (only RTD files)
├── tests/                # Integration and end-to-end checks
│   ├── run_tests.sh      # Docker-only basic test runner
│   ├── docker.sh         # Docker-based basic test suite
│   ├── incus.sh          # Explicit full-OS integration suite
│   ├── integration/      # Go HTTP integration tests
│   ├── e2e/              # Go end-to-end tests
│   └── e2e.sh            # Browser/server E2E runner
├── scripts/              # Production / install scripts
├── .github/workflows/    # GitHub Actions CI/CD
├── .claude/rules/        # Assistant rule cheatsheets
├── binaries/             # Build output (gitignored)
├── volumes/              # Runtime data (gitignored)
├── AI.md                 # Full project specification (READ-ONLY)
├── IDEA.md               # Project description and business logic
├── CLAUDE.md             # Short loader for assistants
├── Makefile
├── mkdocs.yml
├── .readthedocs.yaml
└── release.txt           # Sole version source of truth
```

Source packages are grouped by domain under `src/`; HTTP routing and handlers
live in `src/server/`.

## Testing

```bash
make test                  # Go unit tests inside Docker
./tests/run_tests.sh       # Docker-only basic test runner
./tests/docker.sh          # Build the current tree and test it in Docker
./tests/incus.sh [systemd|openrc|all] # Explicit Debian/systemd and Alpine/OpenRC suite
./tests/e2e.sh             # On-demand SSR, no-JS, and full-browser E2E suite in Docker
```

The Go integration tests live under `tests/integration/` and end-to-end tests
under `tests/e2e/`. The browser E2E suite is run separately on demand; it is
not part of `make test` or `tests/run_tests.sh`.

The process entry point in `src/main.go` delegates to the testable application
package under `src/app/`. Domain packages and app-level tests live below `src/`.

The Docker integration and E2E scripts copy
`docker/docker-compose.test.yml` into a project-scoped OS temp directory. Each
run uses a unique Compose project and a test port in the 64000–64999 range,
then removes its own containers, network, volumes, and temporary files after a
successful run. The E2E script retains its temp directory and browser artifacts
after a failure for diagnosis. Do not run the test compose file from the project
directory or start it manually; use the scripts above so test resources remain
isolated.

The Incus suite defaults to `all` and runs Debian/systemd and numbered
Alpine/OpenRC cases sequentially. The suite creates uniquely named instances
on a dedicated temporary Incus network, installs the service dependencies
(Tor and I2P included), then verifies the native service file, enable/active state,
restart PID, stop/start behavior, API responses, content negotiation, and
binary renaming. It checks Tor with and without its provider binary, I2P's
system-container default-off behavior, explicit enablement, opt-out, and
missing-provider behavior. Finally it restarts the system container and checks
service persistence, disable/reinstall/uninstall, and application data removal.
`TOR_WAIT` and `I2P_WAIT` set the bootstrap wait periods (300 and 600 seconds by
default); `REQUIRE_OVERLAY=1` makes unavailable overlay bootstrapping fail.
`INCUS_IMAGE_SYSTEMD` and `INCUS_IMAGE_OPENRC` can override the default images.

Commit changes through the repository's `gitcommit` workflow and follow the
current project and global commit instructions.

## Coding Conventions

- `CGO_ENABLED=0` everywhere — pure Go, no C, no exceptions.
- Use the project `config.ParseBool()` — never `strconv.ParseBool()`.
- Server-side Go templates only — no client-side framework.
- No `TODO`, `FIXME`, `HACK` in committed code — resolve before commit or open a tracked issue.
- No commented-out code — delete it; git history is the undo mechanism.
- Hash all tokens with SHA-256 before storing; never log raw credentials.
- Detect hostname, IP, CPU count at runtime — never hardcode dev values.
- Singular source directory names.

## Adding a New Feature

1. Update `IDEA.md` "Business logic" section with the new capability.
2. Add routes in `src/server/routes.go` and follow the web/API contract in `AI.md`.
3. Implement the handler and related types in the appropriate existing `src/` packages.
4. Add a unit test next to the source (`*_test.go`).
5. Add an integration test under `tests/integration/` if HTTP behavior is non-trivial.
6. Document the endpoint in `docs/api.md` and the CLI subcommand in `docs/cli.md`.
7. Add or update an entry in `docs/configuration.md` if a new setting is involved.

## CI / Release

CI workflows live in `.github/workflows/`. All third-party Actions are pinned to a full commit SHA. CI never invokes `make` — it calls `docker buildx` and explicit commands directly.

Releases are tagged `v{semver}` and cut by pushing the tag — `release.yml` then builds binaries for all platforms and creates the GitHub release.

For local Makefile builds, `VERSION` overrides a non-empty `release.txt`; when neither is set, the build uses the short Git commit ID and falls back to `devel` outside a Git checkout. Stable manual releases require a SemVer version in `release.txt`; CI release tags supply their own release version.

## Reporting Issues

- Bugs: https://github.com/apimgr/airports/issues
- Vulnerabilities: https://github.com/apimgr/airports/security/advisories/new
- Security email: <security@apimgr.us> (secondary contact only; see [security.md](./security.md))

## License

MIT — see [LICENSE.md](https://github.com/apimgr/airports/blob/main/LICENSE.md).

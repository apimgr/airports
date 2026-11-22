# Development & Contributing

## Local Toolchain

The build runs entirely inside Docker. You **do not** need Go installed on the host. The only host requirements are:

- Docker (Engine 20.10+ or Docker Desktop)
- `git`
- `make` (any GNU make 3.81+)

Optional:

- [Incus](https://linuxcontainers.org/incus/) for full-OS integration tests on systemd.

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

The Makefile's Go targets use `casjaysdev/go:latest`; no host Go toolchain is
needed. CI runs `go vet ./...` and `staticcheck ./...` in the same project
toolchain image. Documentation can be built with `mkdocs build` after installing
`docs/requirements.txt`.

## Repository Layout

```
.
├── src/                  # Go source
│   ├── server/           # HTTP server, routes, handlers, middleware and templates
│   ├── client/           # airports-cli source
│   ├── config/           # Configuration loading and validation
│   ├── airports/         # Airport dataset and query behavior
│   ├── data/             # Embedded airport JSON
│   └── ...
├── docker/               # Dockerfile, compose files, rootfs overlay
├── docs/                 # ReadTheDocs MkDocs source (only RTD files)
├── tests/                # Integration and end-to-end checks
│   ├── run_tests.sh      # Auto-detect Docker or Incus for integration checks
│   ├── docker.sh         # Force Docker-based integration
│   ├── incus.sh          # Force Incus-based integration
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
./tests/run_tests.sh       # Auto-detect Incus or Docker, run integration suite
./tests/docker.sh          # Force Docker-based integration
./tests/incus.sh           # Force Incus VM (preferred — systemd available)
```

The Go integration tests live under `tests/integration/` and end-to-end tests
under `tests/e2e/`.

The Docker test compose file (`docker/docker-compose.test.yml`) **must** be copied to a temp dir before running — never run from the project directory:

```bash
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/apimgr/airports-XXXXXX")
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
chown -R 1000:1000 "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp docker/docker-compose.test.yml "$TEMP_DIR/docker-compose.yml"
cd "$TEMP_DIR" && docker compose up --abort-on-container-exit
rm -rf "$TEMP_DIR"
```

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

`release.txt` is the **sole** source of truth for the version. Bump it in the same commit as the user-facing change.

## Reporting Issues

- Bugs: https://github.com/apimgr/airports/issues
- Vulnerabilities: https://github.com/apimgr/airports/security/advisories/new
- Security email: <security@apimgr.us> (secondary contact only; see [security.md](./security.md))

## License

MIT — see [LICENSE.md](https://github.com/apimgr/airports/blob/main/LICENSE.md).

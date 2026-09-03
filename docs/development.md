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
make lint         # run go vet, staticcheck
make docker       # build the Docker image
```

All targets internally invoke `docker run --rm -v $(pwd):/build -w /build golang:alpine ...`. No host Go toolchain is touched.

## Repository Layout

```
.
├── src/                  # Go source
│   ├── server/           # HTTP server
│   ├── client/           # airports-cli source
│   ├── handler/          # HTTP handlers (singular dir name)
│   ├── model/            # Domain types
│   ├── data/             # Embedded airport JSON
│   └── ...
├── docker/               # Dockerfile, compose files, rootfs overlay
├── docs/                 # ReadTheDocs MkDocs source (only RTD files)
├── tests/                # Repo-root integration test scripts (bash)
│   ├── run_tests.sh
│   ├── docker.sh
│   └── incus.sh
├── test/                 # Go integration/E2E packages
│   ├── integration/
│   └── e2e/
├── scripts/              # Production / install scripts
├── .github/workflows/    # CI/CD (release, beta, daily, docker)
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

Source directories use **singular** names (`handler/`, `model/`, `middleware/`) — never plural.

## Testing

```bash
make test                  # Go unit tests inside Docker
./tests/run_tests.sh       # Auto-detect Incus or Docker, run integration suite
./tests/docker.sh          # Force Docker-based integration
./tests/incus.sh           # Force Incus VM (preferred — systemd available)
```

The Go integration tests live under `test/integration/` and are referenced from `go.mod` as `github.com/apimgr/airports/test/integration`.

The Docker test compose file (`docker/docker-compose.test.yml`) **must** be copied to a temp dir before running — never run from the project directory:

```bash
TEMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/apimgr/airports-XXXXXX")
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
chown -R 1000:1000 "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp docker/docker-compose.test.yml "$TEMP_DIR/docker-compose.yml"
cd "$TEMP_DIR" && docker compose up --abort-on-container-exit
rm -rf "$TEMP_DIR"
```

## Commit Messages

Use the existing commit style — emoji header, 64-char title, blank line, body with bulleted per-file change list.

```
Add bounding-box query endpoint

Adds a new /api/v1/airports/within endpoint that accepts a bbox
parameter and returns all airports inside the rectangle.

- src/handler/airports.go: add WithinBBox handler
- src/server/router.go: register /airports/within route
- docs/api.md: document the new endpoint
```

Do not add any tool-attribution footer to commit messages.

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
2. Add the route in `src/server/router.go` following the web + API pattern (every web page has a corresponding `/api/v1/...` endpoint).
3. Implement the handler under `src/handler/` and the model under `src/model/`.
4. Add a unit test next to the source (`*_test.go`).
5. Add an integration test under `test/integration/` if HTTP behaviour is non-trivial.
6. Document the endpoint in `docs/api.md` and the CLI subcommand in `docs/cli.md`.
7. Add or update an entry in `docs/configuration.md` if a new setting is involved.

## CI / Release

CI workflows live in `.github/workflows/`. All third-party Actions are pinned to a full commit SHA. CI never invokes `make` — it calls `docker buildx` and explicit commands directly.

Releases are tagged `v{semver}` and cut by pushing the tag — `release.yml` then builds binaries for all platforms and creates the GitHub release.

`release.txt` is the **sole** source of truth for the version. Bump it in the same commit as the user-facing change.

## Reporting Issues

- Bugs: https://github.com/apimgr/airports/issues
- Security: <security@apimgr.us> (see [security.md](./security.md) for the policy)

## License

MIT — see [LICENSE.md](https://github.com/apimgr/airports/blob/main/LICENSE.md).

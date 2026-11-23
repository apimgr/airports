#!/usr/bin/env bash
# On-demand, Docker-only browser E2E suite. This is intentionally outside make test.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ORG="apimgr"
INTERNAL_NAME="airports"
TEMP_DIR=""
RUNNER_IMAGE=""
CLEANED_UP=0
KEEP_TEMP=0

random_suffix() {
    printf '%08x' "$(( (RANDOM << 15) | RANDOM ))"
}

__cleanup() {
    [[ $CLEANED_UP -eq 1 ]] && return
    CLEANED_UP=1
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        (cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml down --rmi local --volumes --remove-orphans) >/dev/null 2>&1 || true
        if [[ $KEEP_TEMP -eq 0 ]]; then
            rm -rf -- "$TEMP_DIR"
        fi
    fi
    if [[ -n "$RUNNER_IMAGE" ]]; then
        docker image rm "$RUNNER_IMAGE" >/dev/null 2>&1 || true
    fi
}
trap __cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is required" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "ERROR: Docker daemon is not reachable" >&2; exit 1; }

COMPOSE_SRC="$REPO_ROOT/docker/docker-compose.test.yml"
[[ -f "$COMPOSE_SRC" ]] || { echo "ERROR: $COMPOSE_SRC not found" >&2; exit 1; }
COMPOSE_PROJECT_NAME="airports-e2e-$(random_suffix)"
TEST_PORT="$((64000 + RANDOM % 1000))"
HOST_BIND_ADDRESS="127.0.0.1"
export COMPOSE_PROJECT_NAME HOST_BIND_ADDRESS TEST_PORT
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"
mkdir -p "$TEMP_DIR/volumes/config/airports" "$TEMP_DIR/volumes/data"
chown -R 1000:1000 "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp "$COMPOSE_SRC" "$TEMP_DIR/docker-compose.yml"
cat >"$TEMP_DIR/volumes/config/airports/server.yml" <<'EOF'
server:
  mode: development
EOF
chmod 600 "$TEMP_DIR/volumes/config/airports/server.yml"
chown 1000:1000 "$TEMP_DIR/volumes/config/airports/server.yml"
mkdir -p "$TEMP_DIR/volumes/config/airports/yaml.d"
chmod 700 "$TEMP_DIR/volumes/config/airports/yaml.d"
chown 1000:1000 "$TEMP_DIR/volumes/config/airports/yaml.d"
cat >"$TEMP_DIR/volumes/config/airports/yaml.d/security.yml" <<'EOF'
server:
  rate_limit:
    read:
      requests: 100000
      window: 60
    write:
      requests: 100000
      window: 60
    health:
      requests: 100000
      window: 60
    strict:
      requests: 100000
      window: 60
    auth_failures:
      requests: 100000
      window: 60
    global_burst: 100000
  security:
    abuse_detection:
      request_flood:
        multiplier: 100000
      auto_block_ip: false
      auto_alert: false
EOF
chmod 600 "$TEMP_DIR/volumes/config/airports/yaml.d/security.yml"
chown 1000:1000 "$TEMP_DIR/volumes/config/airports/yaml.d/security.yml"
cat >"$TEMP_DIR/docker-compose.local.yml" <<EOF
services:
  airports:
    image: airports-e2e-local
    pull_policy: build
    build:
      context: "$REPO_ROOT"
      dockerfile: docker/Dockerfile
networks:
  airports-test:
    internal: true
EOF
cat >"$TEMP_DIR/Dockerfile.e2e" <<'EOF'
FROM casjaysdev/go:latest
RUN apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont
WORKDIR /workspace
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/usr/local/share/go/pkg/mod \
    --mount=type=cache,target=/usr/local/share/go/cache \
    go mod download
COPY tests/e2e ./tests/e2e
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV E2E_BASE_URL=http://airports-test
RUN --mount=type=cache,target=/usr/local/share/go/pkg/mod \
    --mount=type=cache,target=/usr/local/share/go/cache \
    go test -c -mod=mod -tags=e2e -o /usr/local/bin/airports-e2e.test ./tests/e2e/...
CMD ["/usr/local/bin/airports-e2e.test", "-test.count=1", "-test.timeout=8m"]
EOF
mkdir -p "$TEMP_DIR/runner/tests/e2e"
cp "$REPO_ROOT/go.mod" "$REPO_ROOT/go.sum" "$TEMP_DIR/runner/"
cp "$REPO_ROOT/tests/e2e/"*.go "$TEMP_DIR/runner/tests/e2e/"

echo ">> Starting isolated Airports test server"
(cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build)
echo ">> Waiting for server readiness"
deadline=$(( $(date +%s) + 120 ))
until [[ $(date +%s) -ge $deadline ]]; do
    AIRPORTS_CONTAINER="$(cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml ps -q airports)"
    if [[ -n "$AIRPORTS_CONTAINER" && "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$AIRPORTS_CONTAINER" 2>/dev/null || true)" == healthy ]]; then
        break
    fi
    sleep 2
done
if [[ -z "$AIRPORTS_CONTAINER" || "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$AIRPORTS_CONTAINER" 2>/dev/null || true)" != healthy ]]; then
    echo "ERROR: Airports test server did not become healthy" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi
E2E_SERVER_IP="$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$AIRPORTS_CONTAINER" 2>/dev/null || true)"
E2E_NETWORK="$(docker inspect --format '{{range $name, $config := .NetworkSettings.Networks}}{{$name}}{{end}}' "$AIRPORTS_CONTAINER" 2>/dev/null || true)"
if [[ -z "$E2E_SERVER_IP" || -z "$E2E_NETWORK" ]]; then
    echo "ERROR: could not determine the Airports test server network address" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

echo ">> Running SSR, no-JS browser, and full-JS browser scenarios in Docker"
mkdir -p "$TEMP_DIR/artifacts"
chmod 700 "$TEMP_DIR/artifacts"
RUNNER_IMAGE="airports-e2e-$(random_suffix)"
docker build --progress=plain --tag "$RUNNER_IMAGE" --file "$TEMP_DIR/Dockerfile.e2e" "$TEMP_DIR/runner"
if docker run --rm --name "airports-$(random_suffix)" --network "$E2E_NETWORK" \
    -e "E2E_BASE_URL=http://${E2E_SERVER_IP}" \
    -e E2E_ARTIFACT_DIR=/artifacts -v "$TEMP_DIR/artifacts:/artifacts" "$RUNNER_IMAGE" >"$TEMP_DIR/e2e.log" 2>&1; then
    cat "$TEMP_DIR/e2e.log"
else
    run_status=$?
    cat "$TEMP_DIR/e2e.log"
    (cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml logs --tail=200 >"$TEMP_DIR/server.log") || true
    KEEP_TEMP=1
    echo "ERROR: E2E failed; logs and browser artifacts retained in $TEMP_DIR" >&2
    exit "$run_status"
fi

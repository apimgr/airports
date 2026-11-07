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
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"
mkdir -p "$TEMP_DIR/volumes/config/airports" "$TEMP_DIR/volumes/data"
chown -R 1000:1000 "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp "$COMPOSE_SRC" "$TEMP_DIR/docker-compose.yml"
cat >"$TEMP_DIR/volumes/config/airports/server.yml" <<'EOF'
server:
  rate_limit:
    enabled: false
EOF
chmod 600 "$TEMP_DIR/volumes/config/airports/server.yml"
chown 1000:1000 "$TEMP_DIR/volumes/config/airports/server.yml"
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
FROM golang:1.26-alpine
RUN apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY tests/e2e ./tests/e2e
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV E2E_BASE_URL=http://airports-test
CMD ["go", "test", "-mod=mod", "-tags=e2e", "-count=1", "-timeout=8m", "./tests/e2e/..."]
EOF
mkdir -p "$TEMP_DIR/runner/tests/e2e"
cp "$REPO_ROOT/go.mod" "$REPO_ROOT/go.sum" "$TEMP_DIR/runner/"
cp "$REPO_ROOT/tests/e2e/"*.go "$TEMP_DIR/runner/tests/e2e/"

echo ">> Starting isolated Airports test server"
(cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build)
echo ">> Waiting for server readiness"
deadline=$(( $(date +%s) + 120 ))
until [[ $(date +%s) -ge $deadline ]]; do
    if [[ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' airports-test 2>/dev/null || true)" == healthy ]]; then
        break
    fi
    sleep 2
done
if [[ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' airports-test 2>/dev/null || true)" != healthy ]]; then
    echo "ERROR: Airports test server did not become healthy" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

echo ">> Running SSR, no-JS browser, and full-JS browser scenarios in Docker"
mkdir -p "$TEMP_DIR/artifacts"
chmod 700 "$TEMP_DIR/artifacts"
RUNNER_IMAGE="airports-e2e-$(random_suffix)"
docker build --tag "$RUNNER_IMAGE" --file "$TEMP_DIR/Dockerfile.e2e" "$TEMP_DIR/runner"
if docker run --rm --name "airports-$(random_suffix)" --network airports-test \
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

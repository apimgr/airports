#!/usr/bin/env bash
#
# tests/docker.sh
# Beta-testing harness for the Airports server inside a throw-away Docker
# container. Copies docker/docker-compose.test.yml to an org-prefixed temp
# directory, brings the stack up, runs smoke tests, and cleans up unconditionally.
#
# Hardcoded sane defaults — no .env file required.
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PROJECT_ORG="apimgr"
PROJECT_NAME="airports"
TEST_PORT="${TEST_PORT:-64581}"
BASE_URL="http://127.0.0.1:${TEST_PORT}"

TEMP_DIR=""
CLEANED_UP=0

__cleanup() {
    [[ $CLEANED_UP -eq 1 ]] && return
    CLEANED_UP=1
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        echo ">> Cleaning up Docker test environment in $TEMP_DIR"
        (cd "$TEMP_DIR" && docker compose down --volumes --remove-orphans) >/dev/null 2>&1 || true
        rm -rf -- "$TEMP_DIR"
    fi
    docker network rm "${PROJECT_NAME}-test" >/dev/null 2>&1 || true
}
trap __cleanup EXIT INT TERM

__require() {
    command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found in PATH" >&2; exit 1; }
}

__require docker
__require curl

if ! docker info >/dev/null 2>&1; then
    echo "ERROR: docker daemon is not reachable" >&2
    exit 1
fi

COMPOSE_SRC="$REPO_ROOT/docker/docker-compose.test.yml"
if [[ ! -f "$COMPOSE_SRC" ]]; then
    echo "ERROR: $COMPOSE_SRC not found" >&2
    exit 1
fi

mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")"
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
cp "$COMPOSE_SRC" "$TEMP_DIR/docker-compose.yml"

echo ">> Bringing up Airports test stack in $TEMP_DIR"
(cd "$TEMP_DIR" && docker compose up -d)

echo ">> Waiting for /server/healthz to become ready..."
__deadline=$(( $(date +%s) + 90 ))
while [[ $(date +%s) -lt $__deadline ]]; do
    if curl -q -fsS "${BASE_URL}/server/healthz" >/dev/null 2>&1; then
        echo ">> Server is up."
        break
    fi
    sleep 2
done

if ! curl -q -fsS "${BASE_URL}/server/healthz" >/dev/null 2>&1; then
    echo "ERROR: server did not become healthy in time" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

__expect_2xx() {
    local path="$1"
    local code
    code=$(curl -q -o /dev/null -s -w "%{http_code}" "${BASE_URL}${path}" || true)
    if [[ "$code" != 2* ]]; then
        echo "FAIL  $path  HTTP $code"
        return 1
    fi
    echo "OK    $path  HTTP $code"
}

echo ">> Smoke tests"
__failed=0
for path in \
    "/server/healthz" \
    "/server/about" \
    "/api/v1/server/healthz" \
    "/api/v1/server/about" \
    "/api/v1/airports/search?q=Tokyo&limit=1"
do
    __expect_2xx "$path" || __failed=1
done

if [[ $__failed -ne 0 ]]; then
    echo ">> One or more smoke tests failed; dumping recent logs"
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

echo ">> All smoke tests passed."

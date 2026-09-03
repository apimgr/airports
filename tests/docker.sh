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
# Frozen forever per AI.md PART 2/3 - same as PROJECT_NAME here since no rename has occurred
INTERNAL_NAME="airports"
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
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"
mkdir -p "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
# Container runs as a fixed non-root UID/GID (1000:1000, see docker/Dockerfile) -
# pre-chown the bind-mounted volumes so the app can write config/data on start.
chown -R 1000:1000 "$TEMP_DIR/volumes/config" "$TEMP_DIR/volumes/data"
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

# __expect_content_type requests path with the given Accept header (empty =
# curl default) and checks the response Content-Type contains want_substr,
# per AI.md PART 28 "Test every route with all applicable Accept headers"
# and PART 14 content negotiation rules.
__expect_content_type() {
    local path="$1" accept="$2" want_substr="$3"
    local ct
    if [[ -n "$accept" ]]; then
        ct=$(curl -q -s -o /dev/null -D - -H "Accept: ${accept}" "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
    else
        ct=$(curl -q -s -o /dev/null -D - "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
    fi
    if [[ "$ct" != *"$want_substr"* ]]; then
        echo "FAIL  $path  Accept:${accept:-<none>}  Content-Type '$ct' does not contain '$want_substr'"
        return 1
    fi
    echo "OK    $path  Accept:${accept:-<none>}  Content-Type: $ct"
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

echo ">> Content negotiation matrix"
while IFS='|' read -r path accept want; do
    [[ -z "$path" ]] && continue
    __expect_content_type "$path" "$accept" "$want" || __failed=1
done <<'MATRIX'
/server/about|text/html|text/html
/server/about|text/plain|text/plain
/api/v1/server/healthz|application/json|application/json
/api/v1/server/healthz|text/plain|text/plain
MATRIX

echo ">> .txt endpoint coverage"
for path in \
    "/robots.txt" \
    "/health.txt" \
    "/api/v1/airports/search.txt?q=Tokyo&limit=1"
do
    __expect_2xx "$path" || __failed=1
    __expect_content_type "$path" "" "text/plain" || __failed=1
done

if [[ $__failed -ne 0 ]]; then
    echo ">> One or more smoke tests failed; dumping recent logs"
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

echo ">> All smoke tests passed."

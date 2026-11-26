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
# Frozen forever per AI.md PART 2/3 - no project rename has occurred
INTERNAL_NAME="airports"
TEST_PORT="${TEST_PORT:-$((64000 + RANDOM % 1000))}"
BASE_URL="http://127.0.0.1:${TEST_PORT}"
HOST_BIND_ADDRESS="127.0.0.1"
COMPOSE_PROJECT_NAME="airports-test-$(printf '%08x' "$(( (RANDOM << 15) | RANDOM ))")"
TEST_IMAGE="${DOCKER_TEST_IMAGE:-airports-docker-test-local}"
DOCKER_MEM="${DOCKER_MEM:-4g}"
DOCKER_CPUS="${DOCKER_CPUS:-2}"
DOCKER_CPU_SHARES="${DOCKER_CPU_SHARES:-256}"
DOCKER_TIMEOUT="${DOCKER_TIMEOUT:-3600}"
export COMPOSE_PROJECT_NAME HOST_BIND_ADDRESS TEST_PORT

TEMP_DIR=""
CLEANED_UP=0

__cleanup() {
    [[ $CLEANED_UP -eq 1 ]] && return
    CLEANED_UP=1
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        echo ">> Cleaning up Docker test environment in $TEMP_DIR"
        (cd "$TEMP_DIR" && docker compose down --volumes --remove-orphans) >/dev/null 2>&1 || true
        if docker image inspect "$TEST_IMAGE" >/dev/null 2>&1; then
            local cleanup_uid cleanup_gid
            cleanup_uid="$(id -u)"
            cleanup_gid="$(id -g)"
            # The container shell expands the cleanup IDs from its environment.
            # shellcheck disable=SC2016
            local -a cleanup_command=(-c 'chown -R "$CLEANUP_UID:$CLEANUP_GID" /cleanup')
            timeout -k 30 "$DOCKER_TIMEOUT" docker run --rm --no-healthcheck \
                --name "${COMPOSE_PROJECT_NAME}-cleanup" \
                --memory="$DOCKER_MEM" --cpus="$DOCKER_CPUS" --cpu-shares="$DOCKER_CPU_SHARES" \
                --user 0 --network none --volume "$TEMP_DIR/volumes:/cleanup" \
                --env "CLEANUP_UID=$cleanup_uid" --env "CLEANUP_GID=$cleanup_gid" \
                --entrypoint sh "$TEST_IMAGE" "${cleanup_command[@]}" >/dev/null 2>&1 || true
        fi
        rm -rf -- "$TEMP_DIR"
    fi
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
mkdir -p "$TEMP_DIR/volumes/config/yaml.d" "$TEMP_DIR/volumes/data"
cat >"$TEMP_DIR/volumes/config/yaml.d/50-overlay-test.yml" <<'EOF'
server:
  tor:
    expose: true
  i2p:
    expose: true
EOF
cp "$COMPOSE_SRC" "$TEMP_DIR/docker-compose.yml"
if [[ -n "${DOCKER_TEST_IMAGE:-}" ]]; then
    cat >"$TEMP_DIR/docker-compose.local.yml" <<EOF
services:
  airports:
    image: "$TEST_IMAGE"
    pull_policy: never
EOF
else
    cat >"$TEMP_DIR/docker-compose.local.yml" <<EOF
services:
  airports:
    image: "$TEST_IMAGE"
    pull_policy: build
    build:
      context: "$REPO_ROOT"
      dockerfile: docker/Dockerfile
EOF
fi

echo ">> Bringing up Airports test stack in $TEMP_DIR"
(cd "$TEMP_DIR" && docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build)

# The command is passed verbatim to the test image's shell.
# shellcheck disable=SC2016
timeout -k 30 "$DOCKER_TIMEOUT" docker run --rm --no-healthcheck \
    --name "airports-$(printf '%08x' "$(( (RANDOM << 15) | RANDOM ))")" \
    --memory="$DOCKER_MEM" --cpus="$DOCKER_CPUS" --cpu-shares="$DOCKER_CPU_SHARES" \
    --network none --entrypoint sh "$TEST_IMAGE" \
    -c 'for c in bash curl tor i2pd tini groupadd useradd; do command -v "$c" || exit 1; done; test -f /etc/ssl/cert.pem; test -d /usr/share/zoneinfo'

echo ">> Waiting for /server/healthz to become ready..."
__deadline=$(( $(date +%s) + 90 ))
while [[ $(date +%s) -lt $__deadline ]]; do
    if curl -q -LSsf "${BASE_URL}/server/healthz" >/dev/null 2>&1; then
        echo ">> Server is up."
        break
    fi
    sleep 2
done

if ! curl -q -LSsf "${BASE_URL}/server/healthz" >/dev/null 2>&1; then
    echo "ERROR: server did not become healthy in time" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

API_VERSION="$(curl -q -LSsf "${BASE_URL}/api/autodiscover" | sed -n 's/.*"current_version":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
if [[ -z "$API_VERSION" ]]; then
    echo "ERROR: could not discover the current API version" >&2
    exit 1
fi
API_BASE="/api/${API_VERSION}"

TOR_WAIT="${TOR_WAIT:-180}"
__tor_status=""
__get_tor_status() {
    curl -q -LSsf -H 'Accept: text/plain' "${BASE_URL}${API_BASE}/server/healthz" 2>/dev/null \
        | sed -n 's/^features\.tor: \([^ ]*\).*/\1/p' | head -n1
}

echo ">> Waiting for Tor hidden service health (up to ${TOR_WAIT} seconds)..."
__tor_deadline=$(( $(date +%s) + TOR_WAIT ))
while [[ $(date +%s) -lt $__tor_deadline ]]; do
    __tor_status="$(__get_tor_status)"
    if [[ "$__tor_status" == healthy ]]; then
        echo ">> Tor hidden service is healthy."
        break
    fi
    sleep 2
done
if [[ "$__tor_status" != healthy ]]; then
    if [[ "${REQUIRE_OVERLAY:-0}" == 1 ]]; then
        echo "ERROR: Tor hidden service did not become healthy within ${TOR_WAIT} seconds" >&2
        (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
        exit 1
    fi
    case "$__tor_status" in
        starting|unavailable)
            echo "SKIP: no outbound network; Tor status is $__tor_status" >&2
            ;;
        *)
            echo "ERROR: unexpected Tor status '$__tor_status' after ${TOR_WAIT} seconds" >&2
            (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
            exit 1
            ;;
    esac
fi

I2P_WAIT="${I2P_WAIT:-600}"
__i2p_line=""
__get_i2p_line() {
    curl -q -LSsf -H 'Accept: text/plain' "${BASE_URL}${API_BASE}/server/healthz" 2>/dev/null \
        | sed -n 's/^features\.i2p: //p' | head -n1
}

echo ">> Waiting for I2P eepsite health (up to ${I2P_WAIT} seconds)..."
__i2p_deadline=$(( $(date +%s) + I2P_WAIT ))
while [[ $(date +%s) -lt $__i2p_deadline ]]; do
    __i2p_line="$(__get_i2p_line)"
    if [[ "${__i2p_line%% *}" == healthy ]]; then
        break
    fi
    sleep 2
done
if [[ "${__i2p_line%% *}" == healthy ]]; then
    if [[ ! "$__i2p_line" =~ ^healthy[[:space:]]+[a-z2-7]+\.b32\.i2p[[:space:]]+provider=i2pd[[:space:]]+state=running$ ]]; then
        echo "ERROR: unexpected healthy I2P feature line: $__i2p_line" >&2
        exit 1
    fi
    if ! curl -q -LSsf -H 'Accept: text/plain' "${BASE_URL}${API_BASE}/server/healthz" 2>/dev/null | grep -Fxq 'checks.i2p: ok'; then
        echo "ERROR: healthy I2P is missing checks.i2p: ok" >&2
        exit 1
    fi
    if ! (cd "$TEMP_DIR" && docker compose exec -T --user 0 airports sh -c 'test -s /data/airports/i2p/site/site-keys.dat && test "$(stat -c %a /data/airports/i2p/site/site-keys.dat)" = 600'); then
        echo "ERROR: I2P destination key was not persisted with mode 0600" >&2
        exit 1
    fi
    echo ">> I2P hidden service is healthy and its destination key is persisted."
else
    case "${__i2p_line%% *}" in
        starting|unavailable)
            if [[ "${REQUIRE_OVERLAY:-0}" == 1 ]]; then
                echo "ERROR: I2P did not become healthy within ${I2P_WAIT} seconds" >&2
                (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
                exit 1
            fi
            echo "SKIP: no outbound network; I2P status is ${__i2p_line%% *}" >&2
            ;;
        *)
            echo "ERROR: unexpected I2P status '${__i2p_line%% *}' after ${I2P_WAIT} seconds" >&2
            (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
            exit 1
            ;;
    esac
fi

__expect_2xx() {
    local path="$1"
    local code
    code=$(curl -q -LSs -o /dev/null -w "%{http_code}" "${BASE_URL}${path}" || true)
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
        ct=$(curl -q -LSs -o /dev/null -D - -H "Accept: ${accept}" "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
    else
        ct=$(curl -q -LSs -o /dev/null -D - "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
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
    "${API_BASE}/server/healthz" \
    "${API_BASE}/server/about" \
    "${API_BASE}/airports/search?q=Tokyo&limit=1"
do
    __expect_2xx "$path" || __failed=1
done

echo ">> Content negotiation matrix"
while IFS='|' read -r path accept want; do
    [[ -z "$path" ]] && continue
    __expect_content_type "$path" "$accept" "$want" || __failed=1
done <<MATRIX
/server/about|text/html|text/html
/server/about|text/plain|text/plain
${API_BASE}/server/healthz|application/json|application/json
${API_BASE}/server/healthz|text/plain|text/plain
MATRIX

echo ">> .txt endpoint coverage"
for path in \
    "/robots.txt" \
    "${API_BASE}/airports/search.txt?q=Tokyo&limit=1"
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

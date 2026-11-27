#!/usr/bin/env bash
#
# tests/docker.sh
# @@License : WTFPL
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
random_suffix() {
    head -c6 /dev/urandom | od -An -tx1 | tr -d ' \n' | cut -c1-8
}
port_seed="$(random_suffix)"
TEST_PORT="${TEST_PORT:-$((64000 + 16#${port_seed:0:3} % 1000))}"
BASE_URL="http://127.0.0.1:${TEST_PORT}"
HOST_BIND_ADDRESS="127.0.0.1"
COMPOSE_PROJECT_NAME="airports-test-$(random_suffix)"
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
    command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found in PATH" >&2; exit 69; }
}

__require docker
__require curl
__require jq

if ! docker info >/dev/null 2>&1; then
    echo "ERROR: docker daemon is not reachable" >&2
    exit 69
fi
if ! docker compose version >/dev/null 2>&1; then
    echo "ERROR: Docker Compose plugin is not available" >&2
    exit 69
fi

COMPOSE_SRC="$REPO_ROOT/docker/docker-compose.test.yml"
if [[ ! -f "$COMPOSE_SRC" ]]; then
    echo "ERROR: $COMPOSE_SRC not found" >&2
    exit 1
fi

mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"
mkdir -p "$TEMP_DIR/volumes/config/airports/yaml.d" "$TEMP_DIR/volumes/data"
cat >"$TEMP_DIR/volumes/config/yaml.d/40-endpoint-coverage.yml" <<'EOF'
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
EOF
chmod 0600 "$TEMP_DIR/volumes/config/airports/yaml.d/40-endpoint-coverage.yml"
cat >"$TEMP_DIR/volumes/config/airports/yaml.d/50-overlay-test.yml" <<'EOF'
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
    --name "airports-$(random_suffix)" \
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

rate_limit="$(curl -q -LSs --max-time 20 -o /dev/null -D - -H 'Accept: text/plain' "${BASE_URL}/server/healthz" | tr -d '\r' | awk -F': ' 'tolower($1)=="x-ratelimit-limit"{value=$2} END{print value}')"
if [[ "$rate_limit" != 100000 ]]; then
    echo "ERROR: endpoint coverage rate limit = ${rate_limit:-missing}, want 100000; test configuration was not loaded" >&2
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

API_VERSION="$(curl -q -LSsf -H 'Accept: application/json' "${BASE_URL}/api/autodiscover" | jq -er '.data.api.current_version | strings | select(length > 0)')"
if [[ -z "$API_VERSION" ]]; then
    echo "ERROR: could not discover the current API version" >&2
    exit 1
fi
API_BASE="/api/${API_VERSION}"

TOR_WAIT="${TOR_WAIT:-180}"
__tor_status=""
__get_tor_status() {
    curl -q -LSsf -H 'Accept: text/plain' "${BASE_URL}${API_BASE}/server/healthz" 2>/dev/null \
        | sed -n 's/^features\.tor: \([^ ]*\).*/\1/p' | awk 'NF {value=$0} END{print value}'
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
        | sed -n 's/^features\.i2p: //p' | awk 'NF {value=$0} END{print value}'
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
        echo "FAIL  $path  HTTP $code" >&2
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
        ct=$(curl -q -LSs -o /dev/null -D - -H "Accept: ${accept}" "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{value=$2} END{print value}')
    else
        ct=$(curl -q -LSs -o /dev/null -D - "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{value=$2} END{print value}')
    fi
    if [[ "$ct" != *"$want_substr"* ]]; then
        echo "FAIL  $path  Accept:${accept:-<none>}  Content-Type '$ct' does not contain '$want_substr'" >&2
        return 1
    fi
    echo "OK    $path  Accept:${accept:-<none>}  Content-Type: $ct"
}

# Exercise every live, documented API operation instead of relying on a
# hand-picked smoke list. OpenAPI is generated from the mounted route registry,
# so disabled optional routes do not create false failures.
__expect_api_contract() {
    local method="$1" path="$2" auth_required="$3" request_path status accept
    local param value owner_token linked_token response
    while [[ "$path" =~ \{([^}]+)\} ]]; do
        param="${BASH_REMATCH[1]}"
        case "$param" in
            api_version) value="$API_VERSION" ;;
            ident) value=KJFK ;;
            country) value=US ;;
            ip) value=8.8.8.8 ;;
            id) value=00000000-0000-7000-8000-000000000000 ;;
            tracking_id) value=00000000-0000-7000-8000-000000000000 ;;
            service) value=prometheus ;;
            admin_path) value=status ;;
            *) value='test' ;;
        esac
        path="${path/\{$param\}/$value}"
    done
    if [[ "$path" == /api/* || "$path" == /mcp || "$path" == /.well-known/* || "$path" == /robots.txt || "$path" == /llms.txt ]]; then
        request_path="$path"
    else
        request_path="${API_BASE}${path}"
    fi
    for accept in application/json text/plain; do
        local -a request=(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' -X "$method" -H "Accept: $accept")
        status="$("${request[@]}" "${BASE_URL}${request_path}" || true)"
        if [[ "$auth_required" == true ]]; then
            if [[ "$status" != 401 && "$status" != 403 ]]; then
                echo "FAIL  $method $request_path  missing bearer token returned HTTP $status, want 401/403" >&2
                return 1
            fi
            response="$(curl -q -LSs --max-time 20 -H 'Accept: application/json' -H 'Content-Type: application/json' --data '{}' "${BASE_URL}${API_BASE}/server/preferences/tokens/primary")" || return 1
            owner_token="$(jq -er '.data.owner_token | strings | select(length > 0)' <<<"$response")" || return 1
            if [[ "$path" == */server/mcp || "$path" == /mcp || "$path" == /api/mcp ]]; then
                response="$(curl -q -LSs --max-time 20 -H 'Accept: application/json' -H 'Content-Type: application/json' -H "Authorization: Bearer $owner_token" --data '{"name":"route-coverage"}' "${BASE_URL}${API_BASE}/server/preferences/tokens")" || return 1
                linked_token="$(jq -er '.data.owner_token | strings | select(length > 0)' <<<"$response")" || return 1
                request+=(-H "Authorization: Bearer $linked_token" -H 'Content-Type: application/json' --data '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"getAirport","arguments":{"ident":"KJFK"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}' -H 'MCP-Protocol-Version: 2026-07-28' -H 'Mcp-Method: tools/call' -H 'Mcp-Name: getAirport')
            else
                request+=(-H "Authorization: Bearer $owner_token")
                if [[ "$method" != GET && "$method" != HEAD ]]; then request+=(-H 'Content-Type: application/json' --data '{}'); fi
            fi
            status="$("${request[@]}" "${BASE_URL}${request_path}" || true)"
            if [[ ! "$status" =~ ^(2|3)[0-9][0-9]$ && ! "$status" =~ ^(400|403|404|409|422)$ ]]; then
                echo "FAIL  $method $request_path  valid bearer token returned HTTP $status" >&2
                return 1
            fi
            if [[ "$path" == */server/mcp || "$path" == /mcp || "$path" == /api/mcp ]] && [[ ! "$status" =~ ^2[0-9][0-9]$ ]]; then
                echo "FAIL  $method $request_path  valid linked token did not succeed (HTTP $status)" >&2
                return 1
            fi
        elif [[ ! "$status" =~ ^(2|3)[0-9][0-9]$ && "$status" != 400 && "$status" != 422 ]]; then
            echo "FAIL  $method $request_path  Accept:$accept returned HTTP $status (route/middleware failure)" >&2
            return 1
        fi
    done
    if [[ "$method" == GET && "$request_path" == /.well-known/* ]]; then
        status="$(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' -I "${BASE_URL}${request_path}" || true)"
        [[ "$status" == 200 ]] || { echo "FAIL  HEAD $request_path returned HTTP $status" >&2; return 1; }
    fi
    echo "OK    $method $request_path  JSON/text request contract"
}

__test_live_api_contract() {
    local method path auth_required
    local contract="${TEMP_DIR}/openapi.json"
    if ! curl -q -LSsf --max-time 20 -H 'Accept: application/json' "${BASE_URL}${API_BASE}/server/swagger" -o "$contract"; then
        echo "FAIL  unable to retrieve live OpenAPI route contract" >&2
        return 1
    fi
    if ! jq -e '.paths | type == "object" and length > 0' "$contract" >/dev/null; then
        echo "FAIL  live OpenAPI route contract has no paths" >&2
        return 1
    fi
    while IFS=$'\t' read -r method path auth_required; do
        [[ -z "$method" || -z "$path" ]] && continue
        __expect_api_contract "$method" "$path" "$auth_required" || return 1
    done < <(jq -r '
        . as $doc | .paths | to_entries[] as $path
        | $path.value | to_entries[]
        | select(.key == "get" or .key == "post" or .key == "put" or .key == "patch" or .key == "delete")
        | [(.key | ascii_upcase), $path.key, ((.value.security // $doc.security // []) | length > 0)] | @tsv
    ' "$contract")
}

__test_frontend_routes() {
    local path expected_status accept want status
    while IFS='|' read -r path expected_status; do
        [[ -z "$path" ]] && continue
        for accept in text/html text/plain; do
            if [[ "$accept" == text/html ]]; then want=text/html; else want=text/plain; fi
            status="$(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' -H "Accept: $accept" "${BASE_URL}${path}" || true)"
            if [[ "$status" != "${expected_status:-200}" ]]; then
                echo "FAIL  GET $path Accept:$accept returned HTTP $status" >&2
                return 1
            fi
            __expect_content_type "$path" "$accept" "$want" || return 1
        done
        echo "OK    GET $path frontend HTML/text negotiation"
    done <<'ROUTES'
/
/airports
/airports/search?q=JFK&limit=1
/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1
/airports/nearby?minLat=40&maxLat=41&minLon=-74&maxLon=-73
/airports/KJFK
/stats
/geoip?ip=8.8.8.8
/server/about
/server/help
/server/help/mcp
/server/healthz
/server/privacy
/server/preferences
/server/preferences/sync
/server/preferences/sync/import
/server/preferences/tokens
/server/dpo
/server/terms
/server/contact
/server/security
/server/security/policy
/server/security/thanks
/server/security/report/00000000-0000-7000-8000-000000000000|404
/server/docs/swagger
/server/docs/graphql
ROUTES
}

__test_fixed_resources() {
    local path want optional status content_type head_status
    while IFS='|' read -r path want optional; do
        [[ -z "$path" ]] && continue
        status="$(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' "${BASE_URL}${path}" || true)"
        if [[ "$optional" == true && "$status" == 404 ]]; then
            head_status="$(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' -I "${BASE_URL}${path}" || true)"
            [[ "$head_status" == 404 ]] || { echo "FAIL  HEAD $path returned HTTP $head_status, want 404" >&2; return 1; }
            echo "OK    GET $path is absent while its optional feature is disabled"
            continue
        fi
        if [[ "$status" != 200 ]]; then
            echo "FAIL  GET $path returned HTTP $status, want 200" >&2
            return 1
        fi
        if [[ "$want" == text/plain ]]; then
            content_type="$(curl -q -LSs --max-time 20 -o /dev/null -D - -H 'Accept: text/plain' "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{value=$2} END{print value}')"
        else
            content_type="$(curl -q -LSs --max-time 20 -o /dev/null -D - "${BASE_URL}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{value=$2} END{print value}')"
        fi
        if [[ "$content_type" != *"$want"* ]]; then
            echo "FAIL  GET $path returned Content-Type '$content_type', want '$want'" >&2
            return 1
        fi
        if [[ "$path" == /.well-known/* || "$path" == /robots.txt ]]; then
            head_status="$(curl -q -LSs --max-time 20 -o /dev/null -w '%{http_code}' -I "${BASE_URL}${path}" || true)"
            [[ "$head_status" == 200 ]] || { echo "FAIL  HEAD $path returned HTTP $head_status" >&2; return 1; }
        fi
        echo "OK    GET/HEAD $path content type $want"
    done <<'RESOURCES'
/robots.txt|text/plain|false
/.well-known/security.txt|text/plain|false
/.well-known/llms.txt|text/plain|false
/.well-known/pgp-key.asc|text/plain|true
/llms.txt|text/plain|false
/manifest.json|json|false
/sw.js|javascript|false
/sitemap.xml|xml|false
/feed.xml|atom+xml|false
/rss.xml|rss+xml|false
/locales/en.json|json|false
/static/css/accent.css|text/css|false
/server/docs/swagger/theme.css|text/css|false
/server/docs/graphql/theme.css|text/css|false
RESOURCES
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

echo ">> Live OpenAPI endpoint and method coverage"
__test_live_api_contract || __failed=1

echo ">> Frontend route and content-negotiation coverage"
__test_frontend_routes || __failed=1

echo ">> Fixed resource, discovery and .txt route coverage"
__test_fixed_resources || __failed=1

if [[ $__failed -ne 0 ]]; then
    echo ">> One or more smoke tests failed; dumping recent logs"
    (cd "$TEMP_DIR" && docker compose logs --tail=200) || true
    exit 1
fi

echo ">> All smoke tests passed."

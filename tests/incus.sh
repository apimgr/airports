#!/usr/bin/env bash
#
# tests/incus.sh
# Beta-testing harness for the Airports server inside a throw-away Incus VM
# running Debian. Preferred over docker.sh when a full systemd environment
# is required (service install/uninstall, journald, etc.).
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PROJECT_ORG="apimgr"
PROJECT_NAME="airports"
# Frozen forever per AI.md PART 2/3 - same as PROJECT_NAME here since no rename has occurred
INTERNAL_NAME="airports"
INCUS_IMAGE="${INCUS_IMAGE:-images:debian/12/cloud}"

INSTANCE=""
TEMP_DIR=""
CLEANED_UP=0

__cleanup() {
    [[ $CLEANED_UP -eq 1 ]] && return
    CLEANED_UP=1
    if [[ -n "$INSTANCE" ]]; then
        echo ">> Removing Incus instance $INSTANCE"
        incus delete --force "$INSTANCE" >/dev/null 2>&1 || true
    fi
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf -- "$TEMP_DIR"
    fi
}
trap __cleanup EXIT INT TERM

__require() {
    command -v "$1" >/dev/null 2>&1 || { echo "ERROR: $1 not found in PATH" >&2; exit 1; }
}

__require incus
__require curl

if ! incus info >/dev/null 2>&1; then
    echo "ERROR: incus daemon is not reachable" >&2
    exit 1
fi

# Build a Linux amd64 binary inside Docker (no host Go required).
__require docker
mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"

echo ">> Building airports binary (linux/amd64) in golang:alpine"
docker run --rm \
    -v "$REPO_ROOT:/build:ro" \
    -v "$TEMP_DIR:/out" \
    -w /build \
    -e CGO_ENABLED=0 \
    -e GOOS=linux \
    -e GOARCH=amd64 \
    golang:alpine \
    sh -c "go build -ldflags '-s -w' -o /out/airports ./src"

INSTANCE="${PROJECT_NAME}-test-$$"
echo ">> Launching Incus VM $INSTANCE from $INCUS_IMAGE"
incus launch "$INCUS_IMAGE" "$INSTANCE" >/dev/null

echo ">> Waiting for systemd to be ready"
__deadline=$(( $(date +%s) + 120 ))
until incus exec "$INSTANCE" -- systemctl is-system-running --wait >/dev/null 2>&1; do
    [[ $(date +%s) -gt $__deadline ]] && { echo "ERROR: systemd never came up" >&2; exit 1; }
    sleep 2
done

echo ">> Pushing binary into VM"
incus file push "$TEMP_DIR/airports" "$INSTANCE/usr/local/bin/airports" --mode=0755

echo ">> Installing service"
incus exec "$INSTANCE" -- /usr/local/bin/airports --service install
incus exec "$INSTANCE" -- /usr/local/bin/airports --service start

echo ">> Waiting for service to become healthy"
__deadline=$(( $(date +%s) + 90 ))
until incus exec "$INSTANCE" -- /usr/local/bin/airports --status >/dev/null 2>&1; do
    [[ $(date +%s) -gt $__deadline ]] && {
        echo "ERROR: service did not become healthy" >&2
        incus exec "$INSTANCE" -- journalctl -u airports --no-pager -n 200 || true
        exit 1
    }
    sleep 2
done

echo ">> Discovering listen port from server.yml"
# The server binds a random port in the 64000-64999 range on first run and
# persists it to server.yml (PART 5) - never assume a fixed port such as 8080.
LIVE_PORT="$(incus exec "$INSTANCE" -- sh -c \
    "grep -m1 -- '^  port:' /etc/apimgr/airports/server.yml | sed 's/.*\"\\(.*\\)\".*/\\1/'")"
if [[ -z "$LIVE_PORT" ]]; then
    echo "ERROR: could not determine listen port from server.yml" >&2
    incus exec "$INSTANCE" -- journalctl -u airports --no-pager -n 200 || true
    exit 1
fi
echo ">> Server listening on port $LIVE_PORT"

echo ">> Probing endpoints from inside the VM"
__expect_2xx() {
    local path="$1"
    local code
    code=$(incus exec "$INSTANCE" -- curl -q -o /dev/null -s -w "%{http_code}" "http://127.0.0.1:${LIVE_PORT}${path}" || true)
    if [[ "$code" != 2* ]]; then
        echo "FAIL  $path  HTTP $code"
        return 1
    fi
    echo "OK    $path  HTTP $code"
}

# __expect_content_type requests path from inside the VM with the given
# Accept header (empty = curl default) and checks the response Content-Type
# contains want_substr, per AI.md PART 28 "Test every route with all
# applicable Accept headers" and PART 14 content negotiation rules.
__expect_content_type() {
    local path="$1" accept="$2" want_substr="$3"
    local ct
    if [[ -n "$accept" ]]; then
        ct=$(incus exec "$INSTANCE" -- curl -q -s -o /dev/null -D - -H "Accept: ${accept}" "http://127.0.0.1:${LIVE_PORT}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
    else
        ct=$(incus exec "$INSTANCE" -- curl -q -s -o /dev/null -D - "http://127.0.0.1:${LIVE_PORT}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')
    fi
    if [[ "$ct" != *"$want_substr"* ]]; then
        echo "FAIL  $path  Accept:${accept:-<none>}  Content-Type '$ct' does not contain '$want_substr'"
        return 1
    fi
    echo "OK    $path  Accept:${accept:-<none>}  Content-Type: $ct"
}

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
    echo ">> One or more smoke tests failed; dumping journal"
    incus exec "$INSTANCE" -- journalctl -u airports --no-pager -n 200 || true
    exit 1
fi

echo ">> All smoke tests passed."

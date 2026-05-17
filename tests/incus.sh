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
INCUS_IMAGE="${INCUS_IMAGE:-images:debian/12/cloud}"
TEST_PORT="${TEST_PORT:-64581}"

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
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${PROJECT_NAME}-XXXXXX")"

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

echo ">> Probing endpoints from inside the VM"
__expect_2xx() {
    local path="$1"
    local code
    code=$(incus exec "$INSTANCE" -- curl -q -o /dev/null -s -w "%{http_code}" "http://127.0.0.1:8080${path}" || true)
    if [[ "$code" != 2* ]]; then
        echo "FAIL  $path  HTTP $code"
        return 1
    fi
    echo "OK    $path  HTTP $code"
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

if [[ $__failed -ne 0 ]]; then
    echo ">> One or more smoke tests failed; dumping journal"
    incus exec "$INSTANCE" -- journalctl -u airports --no-pager -n 200 || true
    exit 1
fi

echo ">> All smoke tests passed."

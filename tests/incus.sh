#!/usr/bin/env bash
# shellcheck shell=bash
# - - - - - - - - - - - - - - - - - - - - - - - - -
##@Version           :  202611222254-git
# @@Author           :  Jason Hempstead
# @@Contact          :  git-admin@casjaysdev.pro
# @@License          :  WTFPL
# @@ReadME           :  tests/incus.sh
# @@Copyright        :  Copyright: (c) 2026 Jason Hempstead, Casjays Developments
# @@Created          :  Sunday, November 22, 2026 22:54 EST
# @@File             :  tests/incus.sh
# @@Description      :  Exercise the complete service lifecycle in Debian/systemd and Alpine/OpenRC Incus containers
# @@Changelog         :  Add the AI.md systemd and OpenRC lifecycle and overlay matrix
# @@TODO              :  None
# @@Other             :  Uses Incus system containers and project-scoped temporary build output
# @@Resource          :  None
# @@Terminal App      :  no
# @@sudo/root         :  no
# @@Template          :  shell/bash
# - - - - - - - - - - - - - - - - - - - - - - - - -
# shellcheck disable=SC1001,SC1003,SC2001,SC2003,SC2016,SC2031,SC2090,SC2115,SC2120,SC2155,SC2199,SC2229,SC2317,SC2329
# - - - - - - - - - - - - - - - - - - - - - - - - -
set -eo pipefail

APPNAME="${0##*/}"
SCRIPT_SRC_DIR="$(cd "${BASH_SOURCE%/*}" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_SRC_DIR/.." && pwd)"
PROJECT_NAME="$(basename "$REPO_ROOT")"
PROJECT_ORG="$(basename "$(dirname "$REPO_ROOT")")"
INTERNAL_NAME="${INTERNAL_NAME:-$PROJECT_NAME}"
INTERNAL_ORG="${INTERNAL_ORG:-$PROJECT_ORG}"
MODE="${1:-all}"
INCUS_IMAGE_SYSTEMD="${INCUS_IMAGE_SYSTEMD:-images:debian/trixie}"
INCUS_IMAGE_OPENRC="${INCUS_IMAGE_OPENRC:-images:alpine/3.24}"
GO_CACHE="${GO_CACHE:-$HOME/go/pkg/mod}"
GO_BUILD="${GO_BUILD:-$HOME/.cache/go-build/$PROJECT_NAME}"
DOCKER_MEM="${DOCKER_MEM:-4g}"
DOCKER_CPUS="${DOCKER_CPUS:-2}"
DOCKER_CPU_SHARES="${DOCKER_CPU_SHARES:-256}"
DOCKER_TIMEOUT="${DOCKER_TIMEOUT:-3600}"
INCUS_CPUS="${INCUS_CPUS:-2}"
INCUS_MEM="${INCUS_MEM:-4GiB}"
INCUS_CPU_PRIORITY="${INCUS_CPU_PRIORITY:-2}"
TEMP_DIR=""
TEST_NETWORK=""
INSTANCES=()

random_suffix() {
  printf '%08x' "$(( (RANDOM << 15) | RANDOM ))"
}

case "$MODE" in
  systemd|openrc) MODES=("$MODE") ;;
  all) MODES=(systemd openrc) ;;
  *) printf 'Usage: %s [systemd|openrc|all]\n' "$APPNAME" >&2; exit 2 ;;
esac

__cleanup() {
  local instance
  for instance in "${INSTANCES[@]}"; do
    incus delete --force "$instance" >/dev/null 2>&1 || true
  done
  if [[ -n "$TEST_NETWORK" ]]; then
    incus network delete "$TEST_NETWORK" >/dev/null 2>&1 || true
  fi
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf -- "$TEMP_DIR"
  fi
}
trap __cleanup EXIT

__require() {
  command -v "$1" >/dev/null 2>&1 || { printf 'ERROR: %s not found in PATH\n' "$1" >&2; exit 1; }
}

__wait_boot() {
  local instance="$1" init="$2" attempt state
  for ((attempt = 0; attempt < 60; attempt++)); do
    incus exec "$instance" -- true >/dev/null 2>&1 && break
    sleep 1
  done
  if [[ "$init" == systemd ]]; then
    for ((attempt = 0; attempt < 90; attempt++)); do
      state="$(incus exec "$instance" -- systemctl is-system-running 2>/dev/null || true)"
      [[ -n "$state" && "$state" != starting ]] && break
      sleep 1
    done
  else
    for ((attempt = 0; attempt < 60; attempt++)); do
      incus exec "$instance" -- test -d /run/openrc >/dev/null 2>&1 && break
      sleep 1
    done
  fi
  for ((attempt = 0; attempt < 60; attempt++)); do
    incus exec "$instance" -- sh -c 'ip route | grep -q "^default"' >/dev/null 2>&1 && break
    sleep 1
  done
}

__remote() {
  local instance="$1" init="$2" phase="$3"
  incus exec "$instance" -- env \
    INIT_MODE="$init" PHASE="$phase" PROJECT_NAME="$PROJECT_NAME" \
    INTERNAL_NAME="$INTERNAL_NAME" INTERNAL_ORG="$INTERNAL_ORG" \
    REQUIRE_OVERLAY="${REQUIRE_OVERLAY:-0}" TOR_WAIT="${TOR_WAIT:-300}" \
    I2P_WAIT="${I2P_WAIT:-600}" bash -s <<'REMOTE_TESTS'
set -eo pipefail
FAILURES=0
mkdir -p "${TMPDIR:-/tmp}/${INTERNAL_ORG}"
REMOTE_TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${INTERNAL_ORG}/${INTERNAL_NAME}-XXXXXX")"
trap 'rm -rf -- "$REMOTE_TEMP_DIR"' EXIT
fail() { printf 'FAIL: %s\n' "$*"; FAILURES=$((FAILURES + 1)); }
pass() { printf 'OK: %s\n' "$*"; }
check() { local description="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$description"; else fail "$description"; fi; }
check_not() { local description="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$description"; else pass "$description"; fi; }
wait_for() { local count="$1" i; shift; for i in $(seq 1 "$count"); do "$@" >/dev/null 2>&1 && return 0; sleep 1; done; return 1; }
CONFIG_FILE="/etc/${INTERNAL_ORG}/${INTERNAL_NAME}/server.yml"
CONF_ROOT="/etc/${INTERNAL_ORG}/${INTERNAL_NAME}"
DATA_ROOT="/var/lib/${INTERNAL_ORG}/${INTERNAL_NAME}"
port() {
  awk '
    /^server:[[:space:]]*$/ { in_server = 1; next }
    in_server && /^[^[:space:]#]/ { in_server = 0 }
    in_server && /^(  |    )port:[[:space:]]*/ {
      sub(/^[^:]*:[[:space:]]*/, "")
      gsub(/"/, "")
      split($0, ports, ",")
      if (ports[1] ~ /^[0-9]+$/) print ports[1]
      exit
    }
  ' "$CONFIG_FILE" 2>/dev/null
}
api_version() { curl -q -LSsf --max-time 10 "http://127.0.0.1:$(port)/api/autodiscover" | jq -r '.data.api.current_version'; }
api() { printf 'http://127.0.0.1:%s/api/%s/server/healthz' "$(port)" "$(api_version)"; }
healthz() { curl -q -LSsf --max-time 10 -H 'Accept: application/json' "$(api)"; }
healthz_up() { healthz | jq -e '.status' >/dev/null; }
overlay() { healthz | jq -r ".features.$1.$2"; }
if [[ "$INIT_MODE" == systemd ]]; then
  UNIT_FILE="/etc/systemd/system/${INTERNAL_NAME}.service"
  svc_active() { systemctl is-active --quiet "$INTERNAL_NAME"; }
  svc_enabled() { systemctl is-enabled --quiet "$INTERNAL_NAME"; }
  svc_pid() { systemctl show -p MainPID --value "$INTERNAL_NAME"; }
else
  UNIT_FILE="/etc/init.d/${INTERNAL_NAME}"
  svc_active() { rc-service "$INTERNAL_NAME" status 2>&1 | grep started >/dev/null; }
  svc_enabled() { rc-update show default | grep -w "$INTERNAL_NAME" >/dev/null; }
  svc_pid() { cat "/var/run/${INTERNAL_ORG}/${INTERNAL_NAME}/${INTERNAL_NAME}.pid"; }
fi
svc_stopped() { ! svc_active; }

if [[ "$PHASE" == reboot ]]; then
  echo "=== $INIT_MODE reboot persistence ==="
  check "service remains enabled after reboot" svc_enabled
  check "service returns after reboot" wait_for 120 svc_active
  check "health responds after reboot" wait_for 120 healthz_up
  "${PROJECT_NAME}" --service --disable
  check_not "disabled service is not enabled" svc_enabled
  check "disabled service is stopped" wait_for 30 svc_stopped
  check "disable keeps its service file" test -f "$UNIT_FILE"
  "${PROJECT_NAME}" --service --install
  check "reinstalled service is enabled" svc_enabled
  check "reinstalled service is active" wait_for 30 svc_active
  check "reinstalled service answers health" wait_for 60 healthz_up
  printf 'y\n' | "${PROJECT_NAME}" --service --uninstall
  check_not "uninstall removes service file" test -e "$UNIT_FILE"
  check_not "uninstalled service is disabled" svc_enabled
  check "uninstalled service is stopped" wait_for 30 svc_stopped
  check_not "uninstall removes application data" test -d "$DATA_ROOT"
  [[ "$FAILURES" -eq 0 ]] || exit 1
  exit 0
fi

echo "=== $INIT_MODE init identity ==="
if [[ "$INIT_MODE" == systemd ]]; then
  check "PID 1 is systemd" test "$(cat /proc/1/comm)" = systemd
else
  check "OpenRC is initialized" test -d /run/openrc
  check "systemd is absent" test ! -d /run/systemd/system
fi
"${PROJECT_NAME}" --version
"${PROJECT_NAME}" --help >/dev/null
check "server binary is installed" test -x "/usr/local/bin/${PROJECT_NAME}"
file "/usr/local/bin/${PROJECT_NAME}"

echo "=== $INIT_MODE native service install and lifecycle ==="
"${PROJECT_NAME}" --service --install
check "service file exists" test -f "$UNIT_FILE"
if [[ "$INIT_MODE" == openrc ]]; then
  check "OpenRC script uses openrc-run" sh -c "head -1 '$UNIT_FILE' | grep -q '^#!/sbin/openrc-run'"
  check "OpenRC script is executable" test -x "$UNIT_FILE"
  check_not "OpenRC script is not SysV init" grep -q 'BEGIN INIT INFO' "$UNIT_FILE"
fi
check "service is enabled" svc_enabled
check "service becomes active" wait_for 30 svc_active
check "health responds after install" wait_for 60 healthz_up
check "service account exists" getent passwd "$INTERNAL_NAME"
check "server runs as service account" test "$(stat -c %U "/proc/$(svc_pid)")" = "$INTERNAL_NAME"
old_pid="$(svc_pid)"
"${PROJECT_NAME}" --service restart
check "service is active after restart" wait_for 30 svc_active
check "health responds after restart" wait_for 60 healthz_up
check "restart replaced process" test "$(svc_pid)" != "$old_pid"
"${PROJECT_NAME}" --service stop
check "service stops" wait_for 30 svc_stopped
check_not "health port closes while stopped" curl -q -LSsf --max-time 3 "$(api)"
"${PROJECT_NAME}" --service start
check "service starts" wait_for 30 svc_active
check "health responds after start" wait_for 60 healthz_up

echo "=== API smoke and project endpoint checks ==="
PORT="$(port)"
API_VERSION="$(api_version)"
for path in "/api/${API_VERSION}/server/healthz" "/api/${API_VERSION}/server/healthz.txt" "/server/healthz" "/server/about" "/api/${API_VERSION}/server/about" "/api/${API_VERSION}/airports/search?q=Tokyo&limit=1"; do
  code="$(curl -q -LSs -o /dev/null -w '%{http_code}' "http://127.0.0.1:${PORT}${path}" || true)"
  [[ "$code" == 2* ]] && pass "$path returns HTTP $code" || fail "$path returned HTTP $code"
done
for spec in "/server/about|text/html|text/html" "/server/about|text/plain|text/plain" "/api/${API_VERSION}/server/healthz|application/json|application/json" "/api/${API_VERSION}/server/healthz|text/plain|text/plain"; do
  IFS='|' read -r path accept want <<<"$spec"
  content_type="$(curl -q -LSs -o /dev/null -D - -H "Accept: $accept" "http://127.0.0.1:${PORT}${path}" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type"{print $2; exit}')"
  [[ "$content_type" == *"$want"* ]] && pass "$path negotiates $want" || fail "$path returned content type $content_type"
done
for path in /robots.txt /server/healthz.txt "/api/${API_VERSION}/airports/search.txt?q=Tokyo&limit=1"; do
  code="$(curl -q -LSs -o /dev/null -w '%{http_code}' "http://127.0.0.1:${PORT}${path}" || true)"
  [[ "$code" == 2* ]] && pass "$path returns HTTP $code" || fail "$path returned HTTP $code"
done
cp "/usr/local/bin/${PROJECT_NAME}" "$REMOTE_TEMP_DIR/renamed-server"
chmod +x "$REMOTE_TEMP_DIR/renamed-server"
renamed_server_help="$("$REMOTE_TEMP_DIR/renamed-server" --help 2>&1)"
if [[ "$renamed_server_help" == *renamed-server* ]]; then
  pass "renamed binary reports its name"
else
  printf '%s\n' "$renamed_server_help" >&2
  fail "renamed binary name is missing"
fi
if [[ -x "/usr/local/bin/${PROJECT_NAME}-cli" ]]; then
  "${PROJECT_NAME}-cli" --version
  "${PROJECT_NAME}-cli" --help >/dev/null
  cp "/usr/local/bin/${PROJECT_NAME}-cli" "$REMOTE_TEMP_DIR/renamed-cli"
  chmod +x "$REMOTE_TEMP_DIR/renamed-cli"
  "$REMOTE_TEMP_DIR/renamed-cli" --help 2>&1 | grep -q renamed-cli && pass "renamed CLI reports its name" || fail "renamed CLI name is missing"
fi

echo "=== Tor overlay with tor installed ==="
check "tor binary exists" command -v tor
check "Tor is enabled" test "$(overlay tor enabled)" = true
case "$(overlay tor status)" in disabled|starting|healthy|unavailable) pass "Tor status is a contract word" ;; *) fail "Tor status is outside the contract" ;; esac
for i in $(seq 1 "$TOR_WAIT"); do [[ "$(overlay tor status)" == healthy ]] && break; sleep 1; done
if [[ "$(overlay tor status)" == healthy ]]; then
  [[ "$(overlay tor hostname)" =~ ^[a-z2-7]{56}\.onion$ ]] && pass "Tor has a v3 hostname" || fail "Tor hostname is not v3"
  check "Tor reports running" test "$(overlay tor running)" = true
  check "Tor health check is ok" test "$(healthz | jq -r '.checks.tor')" = ok
  check "Tor hostname persisted" test -s "$DATA_ROOT/tor/site/hostname"
  for dir in "$CONF_ROOT/tor" "$DATA_ROOT/tor" "$DATA_ROOT/tor/site"; do check "$dir has mode 700" test "$(stat -c %a "$dir")" = 700; done
elif [[ "$REQUIRE_OVERLAY" == 1 ]]; then
  fail "Tor did not become healthy within ${TOR_WAIT}s"
else
  echo "SKIP: Tor bootstrap did not finish within ${TOR_WAIT}s"
  case "$(overlay tor status)" in
    starting) check "starting Tor has an ok health check" test "$(healthz | jq -r '.checks.tor')" = ok ;;
    unavailable) check "unavailable Tor has a degraded health check" test "$(healthz | jq -r '.checks.tor')" = degraded ;;
    *) fail "Tor did not reach a valid bootstrap state" ;;
  esac
fi
tor_bin="$(readlink -f "$(command -v tor)")"
mv "$tor_bin" "${tor_bin}.hidden"
"${PROJECT_NAME}" --service restart
check "health responds after Tor binary removal" wait_for 60 healthz_up
check "Tor is disabled without binary" test "$(overlay tor enabled)" = false
check "Tor status is disabled without binary" test "$(overlay tor status)" = disabled
check "Tor hostname clears without binary" test -z "$(overlay tor hostname)"
check "Tor check is omitted without binary" test "$(healthz | jq '.checks | has("tor")')" = false
mv "${tor_bin}.hidden" "$tor_bin"
"${PROJECT_NAME}" --service restart
check "health responds after restoring Tor" wait_for 60 healthz_up

echo "=== I2P system-container default and enable/opt-out/provider checks ==="
check "i2pd binary exists" command -v i2pd
check "server config has mode 600" test "$(stat -c %a "$CONF_ROOT/server.yml")" = 600
check "I2P is off by default in system containers" test "$(overlay i2p enabled)" = false
check "I2P default status is disabled" test "$(overlay i2p status)" = disabled
check "I2P default lifecycle state is disabled" test "$(overlay i2p state)" = disabled
check "I2P default health check is omitted" test "$(healthz | jq '.checks | has("i2p")')" = false
mkdir -p "$CONF_ROOT/yaml.d"
printf 'server:\n  i2p:\n    enabled: true\n' > "$CONF_ROOT/yaml.d/50-i2p.yml"
chown "$INTERNAL_NAME:$INTERNAL_NAME" "$CONF_ROOT/yaml.d" "$CONF_ROOT/yaml.d/50-i2p.yml"
chmod 0700 "$CONF_ROOT/yaml.d"
chmod 0600 "$CONF_ROOT/yaml.d/50-i2p.yml"
"${PROJECT_NAME}" --service restart
check "health responds with I2P enabled" wait_for 60 healthz_up
check "I2P becomes enabled" test "$(overlay i2p enabled)" = true
check "I2P provider is i2pd" test "$(overlay i2p provider)" = i2pd
case "$(overlay i2p status)" in starting|healthy|unavailable) pass "I2P status is a contract word" ;; *) fail "enabled I2P status is outside the contract" ;; esac
for i in $(seq 1 "$I2P_WAIT"); do [[ "$(overlay i2p status)" == healthy ]] && break; sleep 1; done
if [[ "$(overlay i2p status)" == healthy ]]; then
  [[ "$(overlay i2p hostname)" =~ ^[a-z2-7]{52}\.b32\.i2p$ ]] && pass "I2P has a b32 hostname" || fail "I2P hostname is invalid"
  check "I2P reports running" test "$(overlay i2p running)" = true
  check "I2P health check is ok" test "$(healthz | jq -r '.checks.i2p')" = ok
  check "I2P destination key persisted" sh -c "find '$DATA_ROOT/i2p/site' -type f -size +0 | grep -q ."
elif [[ "$REQUIRE_OVERLAY" == 1 ]]; then
  fail "I2P did not become healthy within ${I2P_WAIT}s"
else
  echo "SKIP: I2P bootstrap did not finish within ${I2P_WAIT}s"
fi
printf 'server:\n  i2p:\n    enabled: false\n' > "$CONF_ROOT/yaml.d/50-i2p.yml"
"${PROJECT_NAME}" --service restart
check "health responds after I2P opt-out" wait_for 60 healthz_up
check "I2P is disabled after opt-out" test "$(overlay i2p enabled)" = false
check "I2P status is disabled after opt-out" test "$(overlay i2p status)" = disabled
check "I2P check is omitted after opt-out" test "$(healthz | jq '.checks | has("i2p")')" = false
printf 'server:\n  i2p:\n    enabled: true\n' > "$CONF_ROOT/yaml.d/50-i2p.yml"
"${PROJECT_NAME}" --service restart
check "health responds before provider removal" wait_for 60 healthz_up
i2pd_bin="$(readlink -f "$(command -v i2pd)")"
mv "$i2pd_bin" "${i2pd_bin}.hidden"
"${PROJECT_NAME}" --service restart
check "health responds without I2P provider" wait_for 60 healthz_up
check "I2P disables without provider" test "$(overlay i2p enabled)" = false
check "I2P status is disabled without provider" test "$(overlay i2p status)" = disabled
check "I2P lifecycle state identifies the missing provider" test "$(overlay i2p state)" = no_provider
check "I2P provider is none without provider" test "$(overlay i2p provider)" = none
check "I2P check is omitted without provider" test "$(healthz | jq '.checks | has("i2p")')" = false
mv "${i2pd_bin}.hidden" "$i2pd_bin"
rm -f "$CONF_ROOT/yaml.d/50-i2p.yml"
"${PROJECT_NAME}" --service restart
check "health responds after restoring I2P provider" wait_for 60 healthz_up
if healthz | grep -Eq '/(etc|var|home|config|data)/'; then fail "health response exposes filesystem path"; else pass "health response has no filesystem path"; fi

[[ "$FAILURES" -eq 0 ]] || { printf '%s check(s) failed\n' "$FAILURES" >&2; exit 1; }
REMOTE_TESTS
}

__run_mode() {
  local init="$1" image instance
  local suffix
  suffix="$(random_suffix)"
  instance="${PROJECT_NAME}-${suffix}"
  INSTANCES+=("$instance")
  if [[ "$init" == systemd ]]; then image="$INCUS_IMAGE_SYSTEMD"; else image="$INCUS_IMAGE_OPENRC"; fi
  printf '=== [%s] Launching %s from %s ===\n' "$init" "$instance" "$image"
  incus launch "$image" "$instance" --network "$TEST_NETWORK" \
    -c limits.cpu="$INCUS_CPUS" -c limits.memory="$INCUS_MEM" -c limits.cpu.priority="$INCUS_CPU_PRIORITY"
  __wait_boot "$instance" "$init"
  if [[ "$init" == systemd ]]; then
    incus exec "$instance" -- bash -c 'apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq curl jq file tor i2pd' >/dev/null
    incus exec "$instance" -- systemctl disable --now tor.service tor@default.service i2pd.service >/dev/null 2>&1 || true
  else
    incus exec "$instance" -- sh -c 'apk add --no-cache bash curl jq file tor i2pd tzdata ca-certificates shadow' >/dev/null
    incus exec "$instance" -- sh -c 'rc-service tor stop; rc-service i2pd stop; rc-update del tor; rc-update del i2pd' >/dev/null 2>&1 || true
  fi
  incus file push "$TEMP_DIR/$PROJECT_NAME" "$instance/usr/local/bin/$PROJECT_NAME" --mode=0755
  incus file push "$TEMP_DIR/$PROJECT_NAME-cli" "$instance/usr/local/bin/$PROJECT_NAME-cli" --mode=0755
  __remote "$instance" "$init" initial
  incus restart "$instance"
  __wait_boot "$instance" "$init"
  __remote "$instance" "$init" reboot
}

__require incus
__require docker
if ! incus info >/dev/null 2>&1; then
  printf 'ERROR: Incus daemon is not reachable\n' >&2
  exit 1
fi

mkdir -p "${TMPDIR:-/tmp}/${PROJECT_ORG}"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT_ORG}/${INTERNAL_NAME}-XXXXXX")"
chmod 0700 "$TEMP_DIR"
mkdir -p "$GO_CACHE" "$GO_BUILD"
docker_suffix="$(random_suffix)"
printf '=== Building Linux binaries ===\n'
timeout -k 30 "$DOCKER_TIMEOUT" docker run --rm --no-healthcheck --name "${PROJECT_NAME}-${docker_suffix}" \
  --memory="$DOCKER_MEM" --cpus="$DOCKER_CPUS" --cpu-shares="$DOCKER_CPU_SHARES" \
  -v "$REPO_ROOT:/build:ro" -v "$TEMP_DIR:/out" \
  -v "$GO_CACHE:/usr/local/share/go/pkg/mod" -v "$GO_BUILD:/usr/local/share/go/cache" \
  -w /build -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 -e GOFLAGS=-buildvcs=false \
  casjaysdev/go:latest sh -c \
  "go build -buildvcs=false -trimpath -ldflags '-s -w' -o /out/${PROJECT_NAME} ./src && go build -buildvcs=false -trimpath -ldflags '-s -w' -o /out/${PROJECT_NAME}-cli ./src/client"

network_suffix="$(random_suffix)"
TEST_NETWORK="${PROJECT_NAME:0:8}-${network_suffix:0:6}"
incus network create "$TEST_NETWORK"
for init in "${MODES[@]}"; do
  __run_mode "$init"
done

printf 'Incus tests completed successfully (%s)\n' "${MODES[*]}"
# ex: ts=2 sw=2 et filetype=sh

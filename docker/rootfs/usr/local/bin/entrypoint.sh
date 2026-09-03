#!/usr/bin/env bash
set -eo pipefail

# =============================================================================
# Container Entrypoint Script - MINIMAL
# Only: set env, start binary, handle signals
# Binary handles: directories, permissions, user/group, Tor, etc.
# =============================================================================

APP_NAME="airports"
APP_BIN="/usr/local/bin/${APP_NAME}"

# Export environment defaults (binary reads these)
export TZ="${TZ:-America/New_York}"
export CONFIG_DIR="${CONFIG_DIR:-/config/${APP_NAME}}"
export DATA_DIR="${DATA_DIR:-/data/${APP_NAME}}"

log() { echo "[entrypoint] $(date '+%Y-%m-%dT%H:%M:%S%z') $*"; }

# =============================================================================
# Start main application
# =============================================================================
log "Starting ${APP_NAME}..."

# Build flags from environment
FLAGS="--address ${ADDRESS:-0.0.0.0} --port ${PORT:-80}"
[ "${DEBUG:-false}" = "true" ] && FLAGS="$FLAGS --debug"

# Start binary (binary handles ALL setup: dirs, perms, user/group, Tor, etc.)
# exec replaces this shell so the app becomes PID 1 and receives signals
# directly from tini (-p SIGTERM) without any intermediary.
exec $APP_BIN $FLAGS "$@"

#!/usr/bin/env bash
set -e

# =============================================================================
# Container Entrypoint Script
# Handles service startup, signal handling, and graceful shutdown
# =============================================================================

APP_NAME="airports"
APP_BIN="/usr/local/bin/${APP_NAME}"
TOR_ENABLED="${ENABLE_TOR:-true}"
TOR_DATA_DIR="/data/tor"

# Array to track background PIDs
declare -a PIDS=()

# -----------------------------------------------------------------------------
# Logging
# -----------------------------------------------------------------------------
log() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

log_error() {
    echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') ERROR: $*" >&2
}

# -----------------------------------------------------------------------------
# Signal handling
# -----------------------------------------------------------------------------
cleanup() {
    log "Received shutdown signal, stopping services..."

    # Stop services in reverse order
    for ((i=${#PIDS[@]}-1; i>=0; i--)); do
        pid="${PIDS[i]}"
        if kill -0 "$pid" 2>/dev/null; then
            log "Stopping PID $pid..."
            kill -TERM "$pid" 2>/dev/null || true
        fi
    done

    # Wait for processes to exit (max 30 seconds)
    local timeout=30
    while [ $timeout -gt 0 ]; do
        local running=0
        for pid in "${PIDS[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                running=1
                break
            fi
        done
        [ $running -eq 0 ] && break
        sleep 1
        ((timeout--))
    done

    # Force kill any remaining
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            log "Force killing PID $pid..."
            kill -9 "$pid" 2>/dev/null || true
        fi
    done

    log "Shutdown complete"
    exit 0
}

# Trap signals for graceful shutdown
# SIGRTMIN+3 (37) is the Docker STOPSIGNAL
# SIGTERM is propagated by tini -p SIGTERM
trap cleanup SIGTERM SIGINT SIGQUIT
trap cleanup SIGRTMIN+3 2>/dev/null || trap cleanup 37

# -----------------------------------------------------------------------------
# Directory setup
# -----------------------------------------------------------------------------
setup_directories() {
    log "Setting up directories..."
    mkdir -p /config /data/db /data/logs /data/tor /data/geoip /data/backup

    # Fix permissions for Tor (runs as tor user)
    if [ "$TOR_ENABLED" = "true" ]; then
        chown -R tor:tor "$TOR_DATA_DIR"
        chmod 700 "$TOR_DATA_DIR"
    fi
}

# -----------------------------------------------------------------------------
# Start Tor (if enabled)
# -----------------------------------------------------------------------------
start_tor() {
    if [ "$TOR_ENABLED" != "true" ]; then
        return 0
    fi

    log "Starting Tor hidden service..."

    # Create torrc if not exists
    if [ ! -f /config/torrc ]; then
        cat > /config/torrc <<EOF
DataDirectory ${TOR_DATA_DIR}
HiddenServiceDir ${TOR_DATA_DIR}/hidden_service
HiddenServicePort 80 127.0.0.1:80
Log notice file /data/logs/tor.log
EOF
    fi

    # Start Tor in background
    tor -f /config/torrc &
    PIDS+=($!)
    log "Tor started (PID: ${PIDS[-1]})"

    # Wait for .onion address
    local timeout=60
    while [ $timeout -gt 0 ]; do
        if [ -f "${TOR_DATA_DIR}/hidden_service/hostname" ]; then
            local onion_addr
            onion_addr=$(cat "${TOR_DATA_DIR}/hidden_service/hostname")
            log "Tor hidden service: ${onion_addr}"
            break
        fi
        sleep 1
        ((timeout--))
    done
}

# -----------------------------------------------------------------------------
# Start main application
# -----------------------------------------------------------------------------
start_app() {
    log "Starting ${APP_NAME}..."

    # Run the main application on port 80 (container internal port)
    # Pass through any additional arguments from CMD
    "$APP_BIN" --port 80 "$@" &
    PIDS+=($!)
    log "${APP_NAME} started (PID: ${PIDS[-1]})"
}

# -----------------------------------------------------------------------------
# Wait for services
# -----------------------------------------------------------------------------
wait_for_services() {
    log "All services started, waiting..."

    # Wait for any process to exit
    while true; do
        for pid in "${PIDS[@]}"; do
            if ! kill -0 "$pid" 2>/dev/null; then
                log_error "Process $pid exited unexpectedly"
                cleanup
            fi
        done
        sleep 5
    done
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------
main() {
    log "Container starting..."
    log "MODE: ${MODE:-development}"
    log "TZ: ${TZ:-UTC}"
    log "ENABLE_TOR: ${TOR_ENABLED}"

    setup_directories
    start_tor
    start_app "$@"
    wait_for_services
}

main "$@"

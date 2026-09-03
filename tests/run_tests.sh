#!/usr/bin/env bash
#
# tests/run_tests.sh
# Auto-detect available runtime (Incus preferred, Docker fallback) and run
# the Airports integration test suite.
#
# Usage:
#   ./tests/run_tests.sh [--force docker|incus]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

FORCE_RUNTIME=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --force)
            FORCE_RUNTIME="${2:-}"
            shift 2
            ;;
        -h|--help)
            sed -n '2,9p' "$0"
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            exit 2
            ;;
    esac
done

__has() { command -v "$1" >/dev/null 2>&1; }

__detect_runtime() {
    if [[ -n "$FORCE_RUNTIME" ]]; then
        echo "$FORCE_RUNTIME"
        return
    fi
    if __has incus && incus info >/dev/null 2>&1; then
        echo "incus"
    elif __has docker && docker info >/dev/null 2>&1; then
        echo "docker"
    else
        echo ""
    fi
}

RUNTIME="$(__detect_runtime)"

case "$RUNTIME" in
    incus)
        echo ">> Using Incus runtime"
        exec "$SCRIPT_DIR/incus.sh"
        ;;
    docker)
        echo ">> Using Docker runtime"
        exec "$SCRIPT_DIR/docker.sh"
        ;;
    *)
        echo "ERROR: neither incus nor docker is available." >&2
        echo "Install one of them or pass --force docker|incus." >&2
        exit 1
        ;;
esac

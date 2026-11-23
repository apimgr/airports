#!/usr/bin/env bash
#
# tests/run_tests.sh
# Run the Docker-based basic test tier; full-app and browser tests are explicit.
#
# Usage:
#   ./tests/run_tests.sh
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if command -v docker >/dev/null 2>&1; then
    echo "Docker detected - running the basic tests..."
    exec "$SCRIPT_DIR/docker.sh"
fi

echo "ERROR: docker not found: every test script builds with Docker" >&2
echo "Install Docker: https://docker.com/" >&2
exit 69

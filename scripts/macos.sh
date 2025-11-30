#!/bin/bash
# Airports API Server - macOS Installation Script
# Organization: apimgr
# Project: airports

set -e

# Configuration
PROJECT="airports"
ORG="apimgr"
REPO="https://github.com/${ORG}/${PROJECT}"
BINARY_NAME="${PROJECT}"
SERVICE_ID="com.${ORG}.${PROJECT}"

# Directories
CONFIG_DIR="${HOME}/.config/${ORG}/${PROJECT}"
DATA_DIR="${HOME}/.local/share/${ORG}/${PROJECT}"
LOGS_DIR="${HOME}/.local/share/${ORG}/${PROJECT}/logs"
INSTALL_DIR="/usr/local/bin"
LAUNCHD_DIR="${HOME}/Library/LaunchAgents"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Detect architecture
detect_arch() {
    case $(uname -m) in
        x86_64) echo "amd64" ;;
        arm64)  echo "arm64" ;;
        *)      log_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
}

# Download latest release
download_binary() {
    local arch=$(detect_arch)
    local url="${REPO}/releases/latest/download/${PROJECT}-macos-${arch}"

    log_info "Downloading ${PROJECT} for macos/${arch}..."
    curl -fsSL -o "/tmp/${BINARY_NAME}" "${url}"
    chmod +x "/tmp/${BINARY_NAME}"
}

# Install binary
install_binary() {
    log_info "Installing ${BINARY_NAME} to ${INSTALL_DIR}..."

    if [ -w "${INSTALL_DIR}" ]; then
        mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi
}

# Create directories
create_directories() {
    log_info "Creating directories..."
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}" "${LOGS_DIR}" "${LAUNCHD_DIR}"
}

# Install launchd service
install_launchd() {
    log_info "Installing launchd service..."

    cat > "${LAUNCHD_DIR}/${SERVICE_ID}.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${SERVICE_ID}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>CONFIG_DIR</key>
        <string>${CONFIG_DIR}</string>
        <key>DATA_DIR</key>
        <string>${DATA_DIR}</string>
        <key>LOGS_DIR</key>
        <string>${LOGS_DIR}</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>${LOGS_DIR}/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>${LOGS_DIR}/stderr.log</string>
</dict>
</plist>
EOF

    log_info "Service installed."
    log_info "Load with: launchctl load ${LAUNCHD_DIR}/${SERVICE_ID}.plist"
    log_info "Start with: launchctl start ${SERVICE_ID}"
}

# Main installation
main() {
    log_info "Installing ${PROJECT}..."

    download_binary
    install_binary
    create_directories
    install_launchd

    log_info "Installation complete!"
    log_info "Configuration: ${CONFIG_DIR}/server.yaml"
    log_info "Data: ${DATA_DIR}"
    log_info "Logs: ${LOGS_DIR}"

    # Show version
    "${INSTALL_DIR}/${BINARY_NAME}" --version
}

# Uninstall
uninstall() {
    log_info "Uninstalling ${PROJECT}..."

    launchctl stop "${SERVICE_ID}" 2>/dev/null || true
    launchctl unload "${LAUNCHD_DIR}/${SERVICE_ID}.plist" 2>/dev/null || true
    rm -f "${LAUNCHD_DIR}/${SERVICE_ID}.plist"

    if [ -w "${INSTALL_DIR}" ]; then
        rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    log_info "Binary and service removed."
    log_info "Config/data directories preserved: ${CONFIG_DIR}, ${DATA_DIR}"
}

# Handle arguments
case "${1:-install}" in
    install)   main ;;
    uninstall) uninstall ;;
    *)         echo "Usage: $0 [install|uninstall]"; exit 1 ;;
esac

#!/bin/bash
# Airports API Server - Linux Installation Script
# Organization: apimgr
# Project: airports

set -e

# Configuration
PROJECT="airports"
ORG="apimgr"
REPO="https://github.com/${ORG}/${PROJECT}"
BINARY_NAME="${PROJECT}"
SERVICE_NAME="${PROJECT}"

# Directories (following BASE.md spec)
CONFIG_DIR="/etc/${ORG}/${PROJECT}"
DATA_DIR="/var/lib/${ORG}/${PROJECT}"
LOGS_DIR="/var/log/${ORG}/${PROJECT}"
INSTALL_DIR="/usr/local/bin"

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
        x86_64)  echo "amd64" ;;
        aarch64) echo "arm64" ;;
        armv7l)  echo "arm" ;;
        *)       log_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
}

# Detect init system
detect_init() {
    if command -v systemctl &> /dev/null && [ -d /run/systemd/system ]; then
        echo "systemd"
    elif command -v sv &> /dev/null; then
        echo "runit"
    elif command -v rc-service &> /dev/null; then
        echo "openrc"
    else
        echo "unknown"
    fi
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Download latest release
download_binary() {
    local arch=$(detect_arch)
    local url="${REPO}/releases/latest/download/${PROJECT}-linux-${arch}"

    log_info "Downloading ${PROJECT} for linux/${arch}..."

    if command -v curl &> /dev/null; then
        curl -fsSL -o "/tmp/${BINARY_NAME}" "${url}"
    elif command -v wget &> /dev/null; then
        wget -q -O "/tmp/${BINARY_NAME}" "${url}"
    else
        log_error "Neither curl nor wget found"
        exit 1
    fi

    chmod +x "/tmp/${BINARY_NAME}"
}

# Install binary
install_binary() {
    log_info "Installing ${BINARY_NAME} to ${INSTALL_DIR}..."
    mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod 755 "${INSTALL_DIR}/${BINARY_NAME}"
}

# Create directories
create_directories() {
    log_info "Creating directories..."
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}" "${LOGS_DIR}"
    chmod 755 "${CONFIG_DIR}" "${DATA_DIR}" "${LOGS_DIR}"
}

# Create user
create_user() {
    if ! id "${PROJECT}" &>/dev/null; then
        log_info "Creating system user ${PROJECT}..."
        useradd -r -s /usr/sbin/nologin -d "${DATA_DIR}" "${PROJECT}"
    fi
    chown -R "${PROJECT}:${PROJECT}" "${CONFIG_DIR}" "${DATA_DIR}" "${LOGS_DIR}"
}

# Install systemd service
install_systemd() {
    log_info "Installing systemd service..."

    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Airports API Server
Documentation=${REPO}
After=network.target

[Service]
Type=simple
User=${PROJECT}
Group=${PROJECT}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
Restart=always
RestartSec=5
Environment="CONFIG_DIR=${CONFIG_DIR}"
Environment="DATA_DIR=${DATA_DIR}"
Environment="LOGS_DIR=${LOGS_DIR}"

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=${CONFIG_DIR} ${DATA_DIR} ${LOGS_DIR}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"
    log_info "Service installed. Start with: systemctl start ${SERVICE_NAME}"
}

# Install runit service
install_runit() {
    log_info "Installing runit service..."

    local sv_dir="/etc/sv/${SERVICE_NAME}"
    mkdir -p "${sv_dir}/log"

    cat > "${sv_dir}/run" << EOF
#!/bin/sh
exec chpst -u ${PROJECT} ${INSTALL_DIR}/${BINARY_NAME}
EOF
    chmod +x "${sv_dir}/run"

    cat > "${sv_dir}/log/run" << EOF
#!/bin/sh
exec svlogd -tt ${LOGS_DIR}
EOF
    chmod +x "${sv_dir}/log/run"

    ln -sf "${sv_dir}" /etc/service/
    log_info "Service installed. Check with: sv status ${SERVICE_NAME}"
}

# Install OpenRC service
install_openrc() {
    log_info "Installing OpenRC service..."

    cat > "/etc/init.d/${SERVICE_NAME}" << EOF
#!/sbin/openrc-run

name="${SERVICE_NAME}"
description="Airports API Server"
command="${INSTALL_DIR}/${BINARY_NAME}"
command_user="${PROJECT}"
command_background="yes"
pidfile="/run/${SERVICE_NAME}.pid"

depend() {
    need net
}
EOF
    chmod +x "/etc/init.d/${SERVICE_NAME}"
    rc-update add "${SERVICE_NAME}" default
    log_info "Service installed. Start with: rc-service ${SERVICE_NAME} start"
}

# Main installation
main() {
    check_root

    log_info "Installing ${PROJECT}..."

    download_binary
    install_binary
    create_directories
    create_user

    local init=$(detect_init)
    case "${init}" in
        systemd) install_systemd ;;
        runit)   install_runit ;;
        openrc)  install_openrc ;;
        *)       log_warn "Unknown init system. Service not installed." ;;
    esac

    log_info "Installation complete!"
    log_info "Configuration: ${CONFIG_DIR}/server.yaml"
    log_info "Data: ${DATA_DIR}"
    log_info "Logs: ${LOGS_DIR}"

    # Show version
    "${INSTALL_DIR}/${BINARY_NAME}" --version
}

# Uninstall
uninstall() {
    check_root
    log_info "Uninstalling ${PROJECT}..."

    local init=$(detect_init)
    case "${init}" in
        systemd)
            systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
            systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
            rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
            systemctl daemon-reload
            ;;
        runit)
            sv stop "${SERVICE_NAME}" 2>/dev/null || true
            rm -rf "/etc/sv/${SERVICE_NAME}" "/etc/service/${SERVICE_NAME}"
            ;;
        openrc)
            rc-service "${SERVICE_NAME}" stop 2>/dev/null || true
            rc-update del "${SERVICE_NAME}" 2>/dev/null || true
            rm -f "/etc/init.d/${SERVICE_NAME}"
            ;;
    esac

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    userdel "${PROJECT}" 2>/dev/null || true

    log_info "Binary and service removed."
    log_info "Config/data directories preserved: ${CONFIG_DIR}, ${DATA_DIR}"
}

# Handle arguments
case "${1:-install}" in
    install)   main ;;
    uninstall) uninstall ;;
    *)         echo "Usage: $0 [install|uninstall]"; exit 1 ;;
esac

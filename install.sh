#!/usr/bin/env bash
# Installation script - builds Spotumn from source and sets up binary and config.
set -euo pipefail

log_info()    { echo -e "\033[34m==>\033[0m \033[1m$1\033[0m"; }
log_success() { echo -e "\033[32m✔\033[0m $1"; }
log_warn()    { echo -e "\033[33m▲\033[0m $1"; }
log_error()   { echo -e "\033[31m✖\033[0m $1" >&2; }

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "\033[34m\033[1m"
cat << 'EOF'
  ___ _ __   ___ | |_ _   _ _ __ ___  _ __  
 / __| '_ \ / _ \| __| | | | '_ ` _ \| '_ \ 
 \__ \ |_) | (_) | |_| |_| | | | | | | | | |
 |___/ .__/ \___/ \__|\__,_|_| |_| |_|_| |_|
     |_|                                    
EOF
echo -e "\033[0m\033[2m   Spotify TUI Client (Compiling from Source)\033[0m\n"

if ! command -v go >/dev/null 2>&1; then
    log_error "Go compiler not found. Please install Go (>= 1.20)."
    exit 1
fi
log_success "Found Go compiler ($(go version | awk '{print $3}'))"

if command -v chafa >/dev/null 2>&1; then
    log_success "Found chafa image renderer"
else
    log_warn "chafa not found (ANSI half-blocks will be used automatically)"
fi

if [ -n "${PREFIX:-}" ]; then
    INSTALL_DIR="${PREFIX}/bin"
elif [ "$(id -u)" -eq 0 ] || [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="${HOME}/.local/bin"
fi
mkdir -p "$INSTALL_DIR"

BUILD_TMP="$(mktemp -d -t spotumn-build-XXXXXX)"
trap 'rm -rf "$BUILD_TMP"' EXIT

log_info "Compiling spotumn..."
go build -trimpath -ldflags="-s -w" -o "${BUILD_TMP}/spotumn" ./cmd/spotumn
log_success "Compilation completed"

log_info "Installing binary to ${INSTALL_DIR}..."
install -m 0755 "${BUILD_TMP}/spotumn" "${INSTALL_DIR}/spotumn"
log_success "Installed binary at ${INSTALL_DIR}/spotumn"

CONFIG_DIR="${HOME}/.config/spotumn"
mkdir -p "${CONFIG_DIR}/themes"
chmod 0700 "$CONFIG_DIR" "${CONFIG_DIR}/themes"

CONFIG_FILE="${CONFIG_DIR}/config.yml"
if [ -f "$CONFIG_FILE" ]; then
    log_success "Existing configuration preserved at ${CONFIG_FILE}"
else
    printf "port: 8989\nart_renderer: auto\ntheme: spotify\n" > "$CONFIG_FILE"
    chmod 0600 "$CONFIG_FILE"
    log_success "Created configuration template at ${CONFIG_FILE}"
fi

echo -e "\n\033[32m\033[1mSpotumn successfully installed!\033[0m"
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    log_warn "${INSTALL_DIR} is not in your \$PATH."
    echo -e "  Add to your shell configuration: \033[34mexport PATH=\"${INSTALL_DIR}:\$PATH\"\033[0m"
fi
echo -e "Run \033[34m\033[1mspotumn\033[0m to start.\n"

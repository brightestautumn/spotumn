#!/usr/bin/env bash
# Installation script - builds Spotumn from source and sets up binaries, desktop entries, and dependencies.
set -euo pipefail

CLR_RESET="\033[0m"
CLR_BOLD="\033[1m"
CLR_BLUE="\033[34m"
CLR_GREEN="\033[32m"
CLR_YELLOW="\033[33m"
CLR_RED="\033[31m"
CLR_DIM="\033[2m"

log_info() {
    echo -e "${CLR_BLUE}==>${CLR_RESET} ${CLR_BOLD}$1${CLR_RESET}"
}

log_success() {
    echo -e "${CLR_GREEN}✔${CLR_RESET} $1"
}

log_warn() {
    echo -e "${CLR_YELLOW}▲${CLR_RESET} $1"
}

log_error() {
    echo -e "${CLR_RED}✖${CLR_RESET} $1" >&2
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo -e "${CLR_BLUE}${CLR_BOLD}"
cat << 'EOF'
  ___ _ __   ___ | |_ _   _ _ __ ___  _ __  
 / __| '_ \ / _ \| __| | | | '_ ` _ \| '_ \ 
 \__ \ |_) | (_) | |_| |_| | | | | | | | | |
 |___/ .__/ \___/ \__|\__,_|_| |_| |_|_| |_|
     |_|                                    
EOF
echo -e "${CLR_RESET}${CLR_DIM}   Spotify TUI Client (Compiling from Source)${CLR_RESET}\n"

BUILD_EDITION=""
NON_INTERACTIVE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --full|-1)
            BUILD_EDITION="full"
            shift
            ;;
        --minimal|-2)
            BUILD_EDITION="minimal"
            shift
            ;;
        -y|--yes)
            NON_INTERACTIVE=true
            shift
            ;;
        -h|--help)
            echo "Usage: ./install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --full, -1         Full edition (Chafa graphics + ANSI fallback)"
            echo "  --minimal, -2      Minimal edition (ANSI only, zero external tools)"
            echo "  -y, --yes          Non-interactive mode (defaults to full)"
            echo "  -h, --help         Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Run './install.sh --help' for usage."
            exit 1
            ;;
    esac
done

if [ -z "$BUILD_EDITION" ]; then
    if [ -t 0 ] && [ "$NON_INTERACTIVE" = false ]; then
        echo -e "${CLR_BOLD}Select edition:${CLR_RESET}"
        echo -e "  ${CLR_BLUE}1)${CLR_RESET} ${CLR_BOLD}Full${CLR_RESET}    ${CLR_DIM}(Chafa artwork + ANSI fallback - Default)${CLR_RESET}"
        echo -e "  ${CLR_BLUE}2)${CLR_RESET} ${CLR_BOLD}Minimal${CLR_RESET} ${CLR_DIM}(Pure ANSI half-blocks, standalone)${CLR_RESET}"
        echo ""
        read -r -p "Enter choice [1/2] (default: 1): " choice
        case "$choice" in
            2|"minimal"|"Minimal")
                BUILD_EDITION="minimal"
                ;;
            *)
                BUILD_EDITION="full"
                ;;
        esac
    else
        BUILD_EDITION="full"
    fi
fi

if ! command -v go >/dev/null 2>&1; then
    log_error "Go compiler not found. Please install Go (>= 1.20)."
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
log_success "Found Go compiler (${GO_VERSION})"

if [ "$BUILD_EDITION" = "full" ]; then
    if command -v chafa >/dev/null 2>&1; then
        log_success "Found chafa image renderer"
    else
        log_warn "chafa not found (ANSI half-blocks will be used automatically)"
    fi
fi

PREFIX="${PREFIX:-}"
if [ -n "$PREFIX" ]; then
    INSTALL_DIR="${PREFIX}/bin"
elif [ "$(id -u)" -eq 0 ] || [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="${HOME}/.local/bin"
fi

mkdir -p "$INSTALL_DIR"

BUILD_TMP="$(mktemp -d -t spotumn-build-XXXXXX)"
cleanup() {
    rm -rf "$BUILD_TMP"
}
trap cleanup EXIT

log_info "Compiling spotumn (${BUILD_EDITION})..."
TARGET_BIN="${BUILD_TMP}/spotumn"
if [ "$BUILD_EDITION" = "minimal" ]; then
    go build -tags minimal -trimpath -ldflags="-s -w" -o "$TARGET_BIN" ./cmd/spotumn
else
    go build -trimpath -ldflags="-s -w" -o "$TARGET_BIN" ./cmd/spotumn
fi
log_success "Compilation completed"

log_info "Installing binary to ${INSTALL_DIR}..."
install -m 0755 "$TARGET_BIN" "${INSTALL_DIR}/spotumn"
log_success "Installed binary at ${INSTALL_DIR}/spotumn"

CONFIG_DIR="${HOME}/.config/spotumn"
mkdir -p "$CONFIG_DIR"
chmod 0700 "$CONFIG_DIR"

CONFIG_FILE="${CONFIG_DIR}/config.yml"
if [ -f "$CONFIG_FILE" ]; then
    log_success "Existing configuration preserved at ${CONFIG_FILE}"
else
    cat > "$CONFIG_FILE" << 'EOF'
port: 8989
art_renderer: auto
theme: spotify
EOF
    chmod 0600 "$CONFIG_FILE"
    log_success "Created configuration template at ${CONFIG_FILE}"
fi

THEMES_DIR="${CONFIG_DIR}/themes"
mkdir -p "$THEMES_DIR"
chmod 0700 "$THEMES_DIR"


echo ""
echo -e "${CLR_GREEN}${CLR_BOLD}Spotumn successfully installed!${CLR_RESET}"

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    log_warn "${INSTALL_DIR} is not in your \$PATH."
    echo -e "  Add to your shell configuration: ${CLR_BLUE}export PATH=\"${INSTALL_DIR}:\$PATH\"${CLR_RESET}"
fi

echo -e "Run ${CLR_BLUE}${CLR_BOLD}spotumn${CLR_RESET} to start.\n"

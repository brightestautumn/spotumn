#!/usr/bin/env bash
# Uninstallation script - removes Spotumn binaries, configuration, and cache.
set -euo pipefail

log_info()    { echo -e "\033[34m==>\033[0m \033[1m$1\033[0m"; }
log_success() { echo -e "\033[32m✔\033[0m $1"; }
log_error()   { echo -e "\033[31m✖\033[0m $1" >&2; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_SCRIPT_DIR="$(realpath "$SCRIPT_DIR" 2>/dev/null || echo "$SCRIPT_DIR")"

echo -e "\033[34m\033[1m"
cat << 'EOF'
  ___ _ __   ___ | |_ _   _ _ __ ___  _ __  
 / __| '_ \ / _ \| __| | | | '_ ` _ \| '_ \ 
 \__ \ |_) | (_) | |_| |_| | | | | | | | | |
 |___/ .__/ \___/ \__|\__,_|_| |_| |_|_| |_|
     |_|                                    
EOF
echo -e "\033[0m\033[2m   Spotify TUI Client (Uninstaller)\033[0m\n"

AUTO_CONFIRM=false
DELETE_CONFIG=""
DELETE_CACHE=""

for arg in "$@"; do
    case "$arg" in
        -y|--yes|-f|--force) AUTO_CONFIRM=true ;;
        --keep-config)      DELETE_CONFIG=false; [ -z "$DELETE_CACHE" ] && DELETE_CACHE=true ;;
        --keep-cache)       DELETE_CACHE=false; [ -z "$DELETE_CONFIG" ] && DELETE_CONFIG=true ;;
        --keep-both)        DELETE_CONFIG=false; DELETE_CACHE=false ;;
        --all|--purge)      DELETE_CONFIG=true; DELETE_CACHE=true ;;
        -h|--help)
            echo "Usage: ./uninstall.sh [OPTIONS]"
            echo -e "\nOptions:"
            echo "  --keep-config            Keep configuration & credentials (~/.config/spotumn)"
            echo "  --keep-cache             Keep cache directory (~/.cache/spotumn)"
            echo "  --keep-both              Keep both configuration and cache"
            echo "  --all, --purge           Remove binary, configuration, and cache"
            echo "  -y, --yes, -f, --force   Skip confirmation prompt"
            echo "  -h, --help               Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown argument: $arg"
            echo "Run './uninstall.sh --help' for usage."
            exit 1
            ;;
    esac
done

if [ -z "$DELETE_CONFIG" ] || [ -z "$DELETE_CACHE" ]; then
    if [ -t 0 ] && [ "$AUTO_CONFIRM" = false ]; then
        echo -e "\033[1mSelect uninstallation scope:\033[0m"
        echo -e "  \033[34m1)\033[0m \033[1mKeep settings & cache\033[0m  \033[2m(Remove binary only - Default)\033[0m"
        echo -e "  \033[34m2)\033[0m \033[1mKeep settings only\033[0m      \033[2m(Remove binary and cache)\033[0m"
        echo -e "  \033[34m3)\033[0m \033[1mComplete purge\033[0m          \033[2m(Remove binary, settings, and cache)\033[0m\n"
        read -r -p "Enter choice [1/2/3] (default: 1): " choice
        case "$choice" in
            2) DELETE_CONFIG=false; DELETE_CACHE=true ;;
            3) DELETE_CONFIG=true;  DELETE_CACHE=true ;;
            *) DELETE_CONFIG=false; DELETE_CACHE=false ;;
        esac
    else
        DELETE_CONFIG=false; DELETE_CACHE=false
    fi
fi

if pgrep -x "spotumn" >/dev/null 2>&1; then
    log_info "Stopping active spotumn process..."
    pkill -x "spotumn" 2>/dev/null || true
    sleep 0.5
fi

log_info "Removing spotumn binary..."
REMOVED_ANY_BIN=false
CANDIDATES=("${HOME}/.local/bin/spotumn" "/usr/local/bin/spotumn")
[ -n "${PREFIX:-}" ] && CANDIDATES+=("${PREFIX}/bin/spotumn")
PATH_BIN="$(command -v spotumn 2>/dev/null || true)"
[ -n "$PATH_BIN" ] && CANDIDATES+=("$PATH_BIN")

SEEN=()
for bin in "${CANDIDATES[@]}"; do
    [ -e "$bin" ] || [ -L "$bin" ] || continue
    real="$(realpath "$bin" 2>/dev/null || echo "$bin")"
    [[ "$real" == "${REAL_SCRIPT_DIR}"/* ]] || [ "$real" = "$REAL_SCRIPT_DIR" ] && continue
    [[ " ${SEEN[*]} " =~ " ${real} " ]] && continue
    SEEN+=("$real")

    if rm -f "$bin" 2>/dev/null || sudo rm -f "$bin" 2>/dev/null; then
        log_success "Removed binary: $bin"
        REMOVED_ANY_BIN=true
    else
        log_error "Permission denied: unable to remove $bin"
    fi
done
[ "$REMOVED_ANY_BIN" = false ] && log_info "No installed binary found in system locations"

CONFIG_DIR="${HOME}/.config/spotumn"
if [ "$DELETE_CONFIG" = true ]; then
    [ -d "$CONFIG_DIR" ] && rm -rf "$CONFIG_DIR" && log_success "Removed configuration: ${CONFIG_DIR}"
    rm -rf "${HOME}/.spotumn"
elif [ -d "$CONFIG_DIR" ]; then
    log_info "Preserved configuration: ${CONFIG_DIR}"
fi

CACHE_DIR="${HOME}/.cache/spotumn"
if [ "$DELETE_CACHE" = true ]; then
    [ -d "$CACHE_DIR" ] && rm -rf "$CACHE_DIR" && log_success "Removed cache: ${CACHE_DIR}"
elif [ -d "$CACHE_DIR" ]; then
    log_info "Preserved cache: ${CACHE_DIR}"
fi

rm -rf /tmp/spotumn* /tmp/art-* "${HOME}/.cache/spotumn.log" 2>/dev/null || true

echo -e "\n\033[32m\033[1mSpotumn uninstallation complete!\033[0m\n"

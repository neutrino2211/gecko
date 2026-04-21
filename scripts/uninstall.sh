#!/bin/bash
set -e

INSTALL_DIR="${GECKO_INSTALL_DIR:-$HOME/.gecko}"
BIN_DIR="${GECKO_BIN_DIR:-$HOME/.local/bin}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo ""
echo "  Gecko Uninstaller"
echo "  ================="
echo ""

# Remove symlink
if [ -L "$BIN_DIR/gecko" ]; then
    rm "$BIN_DIR/gecko"
    success "Removed symlink: $BIN_DIR/gecko"
fi

# Remove installation directory
if [ -d "$INSTALL_DIR" ]; then
    rm -rf "$INSTALL_DIR"
    success "Removed installation directory: $INSTALL_DIR"
else
    warn "Installation directory not found: $INSTALL_DIR"
fi

echo ""
success "Gecko has been uninstalled."
echo ""
info "Remember to remove these from your shell profile if added:"
echo "  - export PATH=\"...:$BIN_DIR\""
echo "  - export GECKO_HOME=\"$INSTALL_DIR\""
echo ""

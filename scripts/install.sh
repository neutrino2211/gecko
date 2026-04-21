#!/bin/bash
set -e

REPO="neutrino2211/gecko"
INSTALL_DIR="${GECKO_INSTALL_DIR:-$HOME/.gecko}"
BIN_DIR="${GECKO_BIN_DIR:-$HOME/.local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)     OS="linux" ;;
        Darwin*)    OS="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) OS="windows" ;;
        *)          error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)   ARCH="amd64" ;;
        arm64|aarch64)  ARCH="arm64" ;;
        *)              error "Unsupported architecture: $ARCH" ;;
    esac

    echo "${OS}-${ARCH}"
}

# Get latest release tag
get_latest_release() {
    # Use /releases/latest endpoint - GitHub determines "latest" correctly
    curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install
install_gecko() {
    PLATFORM=$(detect_platform)
    info "Detected platform: $PLATFORM"

    # Get latest release
    info "Fetching latest release..."
    TAG=$(get_latest_release)
    if [ -z "$TAG" ]; then
        error "Failed to fetch latest release. Check your internet connection."
    fi
    info "Latest release: $TAG"

    # Determine download URL and extension
    if [ "$OS" = "windows" ]; then
        EXT="zip"
    else
        EXT="tar.gz"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/gecko-${PLATFORM}.${EXT}"
    info "Downloading from: $DOWNLOAD_URL"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download
    if command -v curl &> /dev/null; then
        curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/gecko.${EXT}"
    elif command -v wget &> /dev/null; then
        wget -q "$DOWNLOAD_URL" -O "$TMP_DIR/gecko.${EXT}"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi

    # Extract
    info "Extracting..."
    mkdir -p "$INSTALL_DIR"

    if [ "$EXT" = "zip" ]; then
        unzip -q "$TMP_DIR/gecko.zip" -d "$TMP_DIR/extracted"
    else
        tar -xzf "$TMP_DIR/gecko.tar.gz" -C "$TMP_DIR"
        mkdir -p "$TMP_DIR/extracted"
        mv "$TMP_DIR/gecko" "$TMP_DIR/stdlib" "$TMP_DIR/extracted/" 2>/dev/null || true
    fi

    # Install
    info "Installing to $INSTALL_DIR..."

    # Copy binary
    if [ -f "$TMP_DIR/extracted/gecko" ]; then
        cp "$TMP_DIR/extracted/gecko" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/gecko"
    elif [ -f "$TMP_DIR/gecko" ]; then
        cp "$TMP_DIR/gecko" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/gecko"
    else
        error "Binary not found in archive"
    fi

    # Copy stdlib
    if [ -d "$TMP_DIR/extracted/stdlib" ]; then
        rm -rf "$INSTALL_DIR/stdlib"
        cp -r "$TMP_DIR/extracted/stdlib" "$INSTALL_DIR/"
    elif [ -d "$TMP_DIR/stdlib" ]; then
        rm -rf "$INSTALL_DIR/stdlib"
        cp -r "$TMP_DIR/stdlib" "$INSTALL_DIR/"
    else
        warn "stdlib not found in archive"
    fi

    # Create symlink in bin directory
    mkdir -p "$BIN_DIR"
    ln -sf "$INSTALL_DIR/gecko" "$BIN_DIR/gecko"
    success "Binary installed to $BIN_DIR/gecko"

    # Check if bin directory is in PATH
    if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
        warn "$BIN_DIR is not in your PATH"
        echo ""
        echo "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo ""
        echo "  export PATH=\"\$PATH:$BIN_DIR\""
        echo "  export GECKO_HOME=\"$INSTALL_DIR\""
        echo ""
    fi

    # Set GECKO_HOME hint
    if [ -z "$GECKO_HOME" ]; then
        echo ""
        info "Set GECKO_HOME to enable stdlib imports:"
        echo ""
        echo "  export GECKO_HOME=\"$INSTALL_DIR\""
        echo ""
    fi

    success "Gecko installed successfully!"
    echo ""
    info "Version: $TAG"
    info "Binary: $INSTALL_DIR/gecko"
    info "Stdlib: $INSTALL_DIR/stdlib"
    echo ""
    echo "Run 'gecko --help' to get started."
}

# Run installation
install_gecko

#!/bin/bash
# bundle-macos.sh - Create a macOS .app bundle from a Gecko executable
#
# Usage: bundle-macos.sh <binary> <app-name> [icon.icns]
#
# Example:
#   bundle-macos.sh myapp "My App" icon.icns
#
# In gecko.toml:
#   [build.scripts]
#   post_build = ["scripts/bundle-macos.sh $OUTPUT \"My App\""]

set -e

BINARY="$1"
APP_NAME="$2"
ICON="$3"
BUNDLE_ID="${BUNDLE_ID:-com.example.$(echo "$APP_NAME" | tr ' ' '-' | tr '[:upper:]' '[:lower:]')}"
VERSION="${PACKAGE_VERSION:-1.0.0}"

if [ -z "$BINARY" ] || [ -z "$APP_NAME" ]; then
    echo "Usage: $0 <binary> <app-name> [icon.icns]"
    exit 1
fi

APP_DIR="${APP_NAME}.app"
CONTENTS_DIR="${APP_DIR}/Contents"
MACOS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"

echo "Creating ${APP_DIR}..."

# Create directory structure
rm -rf "$APP_DIR"
mkdir -p "$MACOS_DIR"
mkdir -p "$RESOURCES_DIR"

# Copy binary
cp "$BINARY" "${MACOS_DIR}/${APP_NAME}"
chmod +x "${MACOS_DIR}/${APP_NAME}"

# Copy icon if provided
if [ -n "$ICON" ] && [ -f "$ICON" ]; then
    cp "$ICON" "${RESOURCES_DIR}/AppIcon.icns"
    ICON_FILE="AppIcon"
else
    ICON_FILE=""
fi

# Create Info.plist
cat > "${CONTENTS_DIR}/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>NSHighResolutionCapable</key>
    <true/>
EOF

if [ -n "$ICON_FILE" ]; then
    cat >> "${CONTENTS_DIR}/Info.plist" << EOF
    <key>CFBundleIconFile</key>
    <string>${ICON_FILE}</string>
EOF
fi

cat >> "${CONTENTS_DIR}/Info.plist" << EOF
</dict>
</plist>
EOF

# Create PkgInfo
echo -n "APPL????" > "${CONTENTS_DIR}/PkgInfo"

echo "Created ${APP_DIR}"
echo "  Bundle ID: ${BUNDLE_ID}"
echo "  Version: ${VERSION}"

# Optional: Sign the app if codesign is available and identity is set
if [ -n "$CODESIGN_IDENTITY" ]; then
    echo "Signing with identity: ${CODESIGN_IDENTITY}"
    codesign --force --deep --sign "$CODESIGN_IDENTITY" "$APP_DIR"
fi

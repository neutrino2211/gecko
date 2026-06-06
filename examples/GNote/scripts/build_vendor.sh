#!/usr/bin/env sh
set -eu

PROJECT_ROOT="${PROJECT_ROOT:-$(pwd)}"
BUILD_DIR="$PROJECT_ROOT/.gecko_build/vendor"
SQLITE_DIR="$PROJECT_ROOT/vendor/sqlite"
SQLITE_BRIDGE_C="$PROJECT_ROOT/scripts/sqlite_bridge.c"
WEBVIEW_DIR="$PROJECT_ROOT/vendor/webview"

mkdir -p "$BUILD_DIR"

if [ ! -d "$SQLITE_DIR" ]; then
    echo "Missing vendor/sqlite directory." >&2
    exit 1
fi

if [ ! -f "$WEBVIEW_DIR/core/src/webview.cc" ]; then
    echo "Missing webview source file: vendor/webview/core/src/webview.cc" >&2
    exit 1
fi

if [ ! -f "$SQLITE_DIR/sqlite3.c" ] || [ ! -f "$SQLITE_DIR/sqlite3.h" ]; then
    if [ -f "$SQLITE_DIR/configure" ] && command -v make >/dev/null 2>&1; then
        echo "Generating SQLite amalgamation in vendor/sqlite ..."
        if ! (
            cd "$SQLITE_DIR"
            sh ./configure >/dev/null
            make sqlite3.c >/dev/null
        ); then
            echo "Automatic SQLite amalgamation generation failed." >&2
        fi
    fi
fi

if [ ! -f "$SQLITE_DIR/sqlite3.c" ] || [ ! -f "$SQLITE_DIR/sqlite3.h" ]; then
    echo "Could not find sqlite3.c/sqlite3.h in vendor/sqlite." >&2
    echo "Provide SQLite amalgamation files or generate them manually in vendor/sqlite with:" >&2
    echo "  sh ./configure && make sqlite3.c" >&2
    exit 1
fi

"${CC:-cc}" -O2 -fPIC -I"$SQLITE_DIR" -c "$SQLITE_DIR/sqlite3.c" -o "$BUILD_DIR/sqlite3.o"
"${CC:-cc}" -O2 -fPIC -I"$SQLITE_DIR" -c "$SQLITE_BRIDGE_C" -o "$BUILD_DIR/sqlite_bridge.o"

"${CXX:-c++}" -O2 -fPIC -std=c++11 -DWEBVIEW_STATIC \
    -I"$WEBVIEW_DIR/core/include" \
    -c "$WEBVIEW_DIR/core/src/webview.cc" \
    -o "$BUILD_DIR/webview.o"

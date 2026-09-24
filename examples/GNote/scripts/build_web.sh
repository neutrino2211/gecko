#!/usr/bin/env sh
set -eu

WEB_DIR="$(cd "$(dirname "$0")/../web" && pwd)"
OUTPUT=$(mktemp "$WEB_DIR/notes.html.XXXXXX")
trap 'rm -f "$OUTPUT"' EXIT HUP INT TERM

awk -v styles="$WEB_DIR/notes.css" -v script="$WEB_DIR/notes.js" '
function include_asset(path, line, status) {
    while ((status = getline line < path) > 0) {
        print (line == "" ? "" : "    " line)
    }
    if (status < 0) {
        print "Cannot read asset: " path > "/dev/stderr"
        exit 1
    }
    close(path)
}
$0 == "{{STYLES}}" { include_asset(styles); next }
$0 == "{{SCRIPT}}" { include_asset(script); next }
{ print }
' "$WEB_DIR/notes.template.html" > "$OUTPUT"

if ! cmp -s "$OUTPUT" "$WEB_DIR/notes.html"; then
    mv "$OUTPUT" "$WEB_DIR/notes.html"
fi

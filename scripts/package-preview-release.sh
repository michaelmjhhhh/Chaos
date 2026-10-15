#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "$#" -gt 1 ]; then
    echo "Usage: ./scripts/package-preview-release.sh [version matching mygo.json]" >&2
    exit 1
fi

VERSION="$(python3 - <<'PY'
import json
import re

with open("mygo.json", encoding="utf-8") as source:
    version = json.load(source).get("version")
if not isinstance(version, str) or not re.fullmatch(
    r"[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", version
):
    raise SystemExit("mygo.json must contain a filename-safe application version")
print(version)
PY
)"
if [ "$#" -eq 1 ] && [ "$1" != "$VERSION" ]; then
    echo "Preview version must match mygo.json: $VERSION (received $1)" >&2
    exit 1
fi

TARGET_OS="$(go env GOOS)"
ARCH="$(go env GOARCH)"
if [ "$TARGET_OS" != "darwin" ]; then
    echo "Preview packaging requires a darwin target (received $TARGET_OS)" >&2
    exit 1
fi
case "$ARCH" in
    arm64|amd64) ;;
    *) echo "Unsupported macOS target architecture: $ARCH" >&2; exit 1 ;;
esac

DIST_DIR="dist"
ARCHIVE="$DIST_DIR/Chaos-v$VERSION-$ARCH.zip"

./build-app.sh
codesign --verify --deep --strict --verbose=2 ".build/Chaos.app"

mkdir -p "$DIST_DIR"
rm -f "$ARCHIVE"
ditto -c -k --keepParent ".build/Chaos.app" "$ARCHIVE"

echo "Built: $ARCHIVE"
shasum -a 256 "$ARCHIVE"

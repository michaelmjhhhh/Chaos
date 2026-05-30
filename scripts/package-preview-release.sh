#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:?usage: ./scripts/package-preview-release.sh <version>}"
ARCH="$(uname -m)"
DIST_DIR="dist"
ARCHIVE="$DIST_DIR/Chaos-v$VERSION-$ARCH.zip"

./build-app.sh
codesign --verify --deep --strict --verbose=2 ".build/Chaos.app"

mkdir -p "$DIST_DIR"
rm -f "$ARCHIVE"
ditto -c -k --keepParent ".build/Chaos.app" "$ARCHIVE"

echo "Built: $ARCHIVE"
shasum -a 256 "$ARCHIVE"

#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ $# -gt 1 ]]; then
  echo "usage: ./scripts/package-preview-release.sh [version]" >&2
  exit 2
fi
VERSION="$(python3 -c 'import json; print(json.load(open("mygo.json"))["version"])')"
if [[ $# -eq 1 && "$1" != "$VERSION" ]]; then
  echo "Preview version must match mygo.json: $VERSION" >&2
  exit 1
fi
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

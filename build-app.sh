#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"
swift build

APP_DIR=".build/Chaos.app/Contents"
mkdir -p "$APP_DIR/MacOS"
mkdir -p "$APP_DIR/Resources"
cp .build/debug/Chaos "$APP_DIR/MacOS/Chaos"
cp Chaos/Info.plist "$APP_DIR/Info.plist"

ICONSET_DIR=".build/Chaos.iconset"
rm -rf "$ICONSET_DIR"
cp -R Chaos/Resources/Assets.xcassets/AppIcon.appiconset "$ICONSET_DIR"
rm "$ICONSET_DIR/Contents.json"
iconutil -c icns "$ICONSET_DIR" -o "$APP_DIR/Resources/Chaos.icns"

if [ -d ".build/debug/Chaos_Chaos.bundle" ]; then
    cp -R .build/debug/Chaos_Chaos.bundle "$APP_DIR/Resources/"
fi

echo "Built: .build/Chaos.app"
echo "Run:   open .build/Chaos.app"

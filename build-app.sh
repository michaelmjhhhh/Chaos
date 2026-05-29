#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"
swift build

APP_DIR=".build/VibeShot.app/Contents"
mkdir -p "$APP_DIR/MacOS"
cp .build/debug/VibeShot "$APP_DIR/MacOS/VibeShot"
cp VibeShot/Info.plist "$APP_DIR/Info.plist"

echo "Built: .build/VibeShot.app"
echo "Run:   open .build/VibeShot.app"

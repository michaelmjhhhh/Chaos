#!/bin/bash
# Build a drag-to-install Chaos.dmg.
#
# Chaos is distributed WITHOUT Apple notarization (no paid Developer account), the
# same way many open-source Mac apps ship. The app is ad-hoc signed by build-app.sh,
# which avoids the "is damaged" error; first-time users still approve it once under
# System Settings → Privacy & Security → "Open Anyway". See README "Install".
#
# If you later get a Developer ID certificate, set CODESIGN_IDENTITY (e.g.
# "Developer ID Application: Your Name (TEAMID)") and this script will sign with it;
# add `xcrun notarytool` + `xcrun stapler staple` afterward for a clean first launch.
set -euo pipefail

cd "$(dirname "$0")"

APP=".build/Chaos.app"
STAGING=".build/dmg-staging"
DMG=".build/Chaos.dmg"
VOLNAME="Chaos"

# 1. Build (and ad-hoc sign) the .app bundle.
./build-app.sh

# 2. Optional: re-sign with a real Developer ID if one is provided.
if [ -n "${CODESIGN_IDENTITY:-}" ]; then
    echo "Signing with: $CODESIGN_IDENTITY"
    codesign --force --deep --options runtime --sign "$CODESIGN_IDENTITY" "$APP"
fi

# 3. Stage the app next to an /Applications shortcut for drag-to-install.
rm -rf "$STAGING" "$DMG"
mkdir -p "$STAGING"
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"

# 4. Build a compressed, read-only DMG.
hdiutil create \
    -volname "$VOLNAME" \
    -srcfolder "$STAGING" \
    -fs HFS+ \
    -format UDZO \
    -ov \
    "$DMG" >/dev/null

rm -rf "$STAGING"

SIZE=$(du -h "$DMG" | cut -f1)
echo "Built: $DMG ($SIZE)"
echo "Open:  open $DMG"

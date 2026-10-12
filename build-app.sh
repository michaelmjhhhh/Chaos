#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo build -skip-dmg -o .build/mygo -sign "${CODESIGN_IDENTITY:--}"
BUILT_APP=".build/mygo/darwin-$(go env GOARCH)/Chaos.app"
if [ ! -d "$BUILT_APP" ]; then
    echo "MyGo did not produce the expected bundle: $BUILT_APP" >&2
    exit 1
fi
rm -rf ".build/Chaos.app"
cp -R "$BUILT_APP" ".build/Chaos.app"
codesign --verify --deep --strict ".build/Chaos.app"

echo "Built: .build/Chaos.app"
echo "Run:   open .build/Chaos.app"

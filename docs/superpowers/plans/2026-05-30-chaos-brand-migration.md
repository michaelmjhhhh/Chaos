# Chaos Brand Migration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the native macOS app from VibeShot to Chaos, migrate legacy settings safely, and ship a generated Editorial Shutter app icon.

**Architecture:** Rename the SwiftPM package and filesystem layout as one mechanical identity change. Keep migration behavior isolated in `ConfigService`, and keep icon generation isolated in the asset catalog plus bundle assembly script.

**Tech Stack:** Swift 6 package manifest, SwiftUI, XCTest, macOS app bundle shell script, asset catalogs, generated PNG icon source.

---

## Chunk 1: Identity And Migration

### Task 1: Add Config Migration Tests

**Files:**
- Create: `Tests/ChaosTests/ConfigServiceTests.swift`
- Modify: `Chaos/Services/ConfigService.swift`

- [ ] Write tests proving a legacy config is copied only when the Chaos config is absent.
- [ ] Run `swift test` and confirm the new tests fail before implementation.
- [ ] Add injectable config URLs to `ConfigService` and implement copy-forward migration.
- [ ] Run `swift test` and confirm the migration tests pass.

### Task 2: Rename Product Identity

**Files:**
- Move: `VibeShot/` to `Chaos/`
- Move: `Tests/VibeShotTests/` to `Tests/ChaosTests/`
- Modify: `Package.swift`
- Modify: `Chaos/Info.plist`
- Modify: `Chaos/ChaosApp.swift`
- Modify: `Chaos/Views/MenuBarView.swift`
- Modify: `Chaos/Views/Editorial/Masthead.swift`
- Modify: `Chaos/Models/SessionMeta.swift`
- Modify: `Chaos/Services/DirectoryWatcher.swift`
- Modify: `Chaos/AppState.swift`
- Modify: `README.md`

- [ ] Rename directories and app entry file.
- [ ] Replace active code and README identity references with `Chaos` or `chaos`.
- [ ] Keep historical dashboard plan/spec files unchanged.
- [ ] Run `swift test`.

## Chunk 2: Editorial Shutter Icon

### Task 3: Generate And Wire Icon

**Files:**
- Create: `Chaos/Resources/Assets.xcassets/Contents.json`
- Create: `Chaos/Resources/Assets.xcassets/AppIcon.appiconset/Contents.json`
- Create: `Chaos/Resources/Assets.xcassets/AppIcon.appiconset/*.png`
- Modify: `Chaos/Info.plist`
- Modify: `build-app.sh`

- [ ] Generate the 1024px Editorial Shutter bitmap with built-in image generation.
- [ ] Inspect the generated image and copy it into the project.
- [ ] Derive standard macOS icon sizes.
- [ ] Add asset catalog metadata and icon plist metadata.
- [ ] Update `build-app.sh` to assemble `.build/Chaos.app`, copy the executable, plist, and resource bundle.

## Chunk 3: Verification

### Task 4: Verify And Launch

- [ ] Run `swift test`.
- [ ] Run `git diff --check`.
- [ ] Run `./build-app.sh`.
- [ ] Verify `.build/Chaos.app/Contents/MacOS/Chaos`, resources, and icon metadata.
- [ ] Launch `.build/Chaos.app`.

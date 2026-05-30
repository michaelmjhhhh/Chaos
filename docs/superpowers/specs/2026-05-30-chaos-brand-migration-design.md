# Chaos Brand Migration Design

**Date:** 2026-05-30
**Status:** Approved
**Project:** Chaos macOS screenshot organizer

## Goal

Rename VibeShot to Chaos across the entire project and ship a new application icon that matches the editorial-paper interface and screenshot-organizing purpose.

## Product Identity

The application name becomes `Chaos` everywhere: Swift package, executable target, source directory, test target, bundle, bundle identifier, visible labels, masthead, docs, internal defaults keys, queue labels, and default output directory.

The bundle identifier becomes `com.chaos.app`. The built app bundle becomes `.build/Chaos.app`, containing the `Chaos` executable.

## Config Migration

Chaos stores settings at:

```text
~/Library/Application Support/chaos/config.json
```

On load, when the Chaos config does not exist and the legacy config exists at:

```text
~/Library/Application Support/vibe-shot/config.json
```

Chaos copies the legacy config to the new location and loads the copy. It never deletes or overwrites the legacy file. Once the Chaos config exists, the two applications are independent.

## Application Icon

The icon direction is **Editorial Shutter**:

- macOS rounded-square application icon
- warm cream paper background
- refined coral aperture blades with subtle irregularity
- dark ink outlines and restrained paper texture
- no text, gradients kept subtle, ample negative space
- legible in Dock, Finder, and small app listings

Generate a 1024px source bitmap, then derive the standard macOS icon PNG sizes into `Chaos/Resources/Assets.xcassets/AppIcon.appiconset`. Wire the icon name through `Info.plist` and copy resources into the app bundle.

## Compatibility

Legacy config import is the only intentional compatibility bridge. New settings, default output paths, `UserDefaults` keys, queue labels, and bundle identity use `chaos`.

Historical dashboard redesign specs remain historical records and are not rewritten.

## Verification

- Add tests for config migration and Chaos config precedence.
- Rename imports and test paths so `swift test` compiles the renamed module.
- Run `swift test`.
- Run `./build-app.sh`.
- Verify `.build/Chaos.app`, its executable, bundled resources, and icon metadata exist.
- Launch `.build/Chaos.app`.

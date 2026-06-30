# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Chaos is a native macOS (15+, Apple Silicon, Swift 6) menu-bar app that watches a folder for
new screenshots, asks an OpenAI-compatible vision model what each one shows, and renames/files
it under a meaningful name. Built with Swift Package Manager — **there is no `.xcodeproj`**.

## Commands

```bash
swift build                      # compile (debug)
swift test                       # run all tests (XCTest, target: ChaosTests)
swift test --filter InsightsRepositoryTests          # one test class
swift test --filter InsightsRepositoryTests/testPeakHour   # one test method

./build-app.sh                   # build + assemble + ad-hoc sign .build/Chaos.app
open .build/Chaos.app            # run the assembled app
./package-dmg.sh                 # build the drag-to-install Chaos.dmg

swiftformat --lint .             # format check (CI fails if anything is unformatted)
swiftformat .                    # apply formatting
swiftlint lint                   # lint (CI runs this non-strict; warnings are advisory)
```

CI (`.github/workflows/ci.yml`, macOS-15) runs `swift build`, `swift test`, **`swiftformat
--lint .`**, and `swiftlint lint` on every push to `main` and on PRs. The format check runs
across the **whole repo** — run `swiftformat .` before committing or CI's lint job will fail
even when your own files are clean. Tagging `v*` triggers the release workflow, which builds
and attaches the DMG.

Note: tools are Swift 6 but both targets compile in **Swift 5 language mode**
(`.swiftLanguageMode(.v5)` in `Package.swift`), so strict-concurrency violations surface as
warnings, not errors.

## Architecture

**`AppState` (`Chaos/AppState.swift`) is the hub.** It's an `@Observable @MainActor` class
created once in `ChaosApp` and injected via `.environment(appState)` into every scene (main
window, `MenuBarExtra`, `Settings`). All views read it with `@Environment(AppState.self)`. It
owns watcher lifecycle, the current processing stage, in-memory metrics, the loaded config, and
the recent-files working set. Services are deliberately stateless/standalone; AppState wires
them together. When changing behavior, the change usually lands here plus a focused service.

**Three surfaces, one state:** `ContentView` hosts a `TabView` with three tabs —
`DashboardView` (live status + editorial layout), `PipelineView` (searchable history with
retry/revert/rename), and `InsightsView` (all-time analytics). `MenuBarView` and `SettingsView`
are separate scenes. To add a tab, extend `ContentView.AppTab` and add a `Tab(...)`.

**Processing pipeline** (`Chaos/Services/FileProcessor.swift`, an `actor`): a
`DirectoryWatcher` fires on new files → `ScreenshotGuard` rejects anything that isn't a fresh
macOS screenshot (drag-dropped images bypass this guard but use the same pipeline) →
`VisionImage` prepares base64 → `VisionAPIClient` calls the model → `SlugSanitizer` makes the
result filesystem-safe → `NamingPolicy` applies the `{slug}/{date}/{time}` template and subfolder
rule → `FileRenamer` moves the file, avoiding collisions. AppState records the outcome and
publishes `ProcessingStage` updates for the UI. Batch/drag-drop and "retry all" funnel through
`AppState.processBatch`, which guards against concurrent runs.

**Data & persistence — two stores, distinct roles:**
- **`HistoryDatabase` (GRDB/SQLite) is the source of truth** for processed-image history, at
  `~/Library/Application Support/chaos/history.sqlite`. It keeps the full lifetime of records
  (no cap). `AppState` writes through it on `record`/`revert`/`rename` and loads the most-recent
  500 into `recentFiles` for Dashboard/Pipeline. On first open it imports any legacy
  `history.json` via `HistoryStore` and archives that file as `history.json.migrated`.
  `HistoryStore` (JSON) now exists **only** for that one-time import.
- **`InsightsRepository`** runs read-only `GROUP BY` aggregations over the same DB queue and
  returns one `InsightsSnapshot` (all stats computed in a single read). `InsightsView` recomputes
  when `AppState.historyRevision` bumps. SQLite date functions use the `'localtime'` modifier so
  heatmap days / peak hour match the user's wall clock (GRDB stores dates as UTC text).
- **`ConfigService`** persists `AppConfig` to `~/Library/Application Support/chaos/config.json`
  (snake_case keys). AppState resolves config into effective values via its many `resolved*`
  computed properties — prefer those over reading `config` fields directly.

**Provider abstraction:** `Provider` (`Chaos/Models/Provider.swift`) enumerates the supported
OpenAI-compatible vision backends (OpenAI, DeepSeek, OpenRouter, SiliconRouter, Ollama, custom,
and a bundled "Chaos Hosted" free-trial proxy). `VisionAPIClient` speaks the
OpenAI chat-completions format to all of them.

**Domain model split:** `RecentFile` is the storage-agnostic domain type the UI uses;
`ImageRecord` is its GRDB row mapping (with flattened `isError`/`errorMessage` and a cached `ext`
column so analytics can aggregate in SQL without decoding the result enum). Keep the UI depending
on `RecentFile`, not GRDB.

## Design system

All UI pulls from `Chaos/Views/Theme.swift` — an editorial/almanac aesthetic with adaptive
light/dark color pairs (resolved via dynamic `NSColor`, so switching system appearance updates
live), serif display faces, a coral brand accent, and spacing/radii scales. Reusable pieces live
in `Chaos/Views/Editorial/` (`Masthead`, `EditorialRule`, `MetricFigure`, `Sparkline`, `Heatmap`,
`BarBreakdown`, the `.card()` / `.sectionHead()` / `.smallCaps()` modifiers). Build new UI from
these tokens and components rather than introducing ad-hoc colors, fonts, or one-off chrome.

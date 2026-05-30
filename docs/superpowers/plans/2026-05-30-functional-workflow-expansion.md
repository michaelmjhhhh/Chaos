# Chaos Functional Workflow Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add durable bounded history, explicit retry, drag-and-drop image intake, filename templates, and simple dated subfolder organization to Chaos.

**Architecture:** Keep `AppState` as the workflow coordinator while extracting pure, testable helpers for persistence, image acceptance, and naming policy. Feed every intake source into one processing method and keep UI changes restrained to existing editorial surfaces.

**Tech Stack:** Swift 6 package, SwiftUI, AppKit, UniformTypeIdentifiers, XCTest, JSON persistence.

**Spec:** [docs/superpowers/specs/2026-05-30-functional-workflow-expansion-design.md](../specs/2026-05-30-functional-workflow-expansion-design.md)

---

## Task 1: Persist Recent History

**Files:**
- Modify: `Chaos/Models/RecentFile.swift`
- Create: `Chaos/Services/HistoryStore.swift`
- Create: `Tests/ChaosTests/HistoryStoreTests.swift`

- [ ] **Step 1: Write failing tests**

Add tests that construct successful and failed `RecentFile` values, save them
through `HistoryStore`, and assert round-trip equality. Add a test that saves
501 records and asserts only the newest 500 load. Add a malformed JSON test
that asserts `load()` returns `[]`.

- [ ] **Step 2: Run tests to verify RED**

Run: `swift test --filter HistoryStoreTests`

Expected: FAIL because `HistoryStore` does not exist and `RecentFile` is not
codable.

- [ ] **Step 3: Implement minimal persistence**

Make `RecentFile`, `RecentFile.Result`, and their stored fields `Codable` and
`Equatable`, preserving a decoded stable UUID. Add `sourcePath`. Create
`HistoryStore` with injectable URL, `limit = 500`, tolerant `load()`, and
atomic sorted JSON `save(_:)`.

- [ ] **Step 4: Run tests to verify GREEN**

Run: `swift test --filter HistoryStoreTests`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Chaos/Models/RecentFile.swift Chaos/Services/HistoryStore.swift Tests/ChaosTests/HistoryStoreTests.swift
git commit -m "feat: persist bounded processing history"
```

## Task 2: Load History and Add Retry

**Files:**
- Modify: `Chaos/AppState.swift`
- Modify: `Chaos/Views/PipelineView.swift`
- Modify: `Chaos/Views/Editorial/FiledColumn.swift`

- [ ] **Step 1: Refactor `AppState` history insertion**

Add a `HistoryStore` dependency, load history in `loadConfig()`, and route
attempt inserts through `record(_:)`. Keep session metrics separate from loaded
history.

- [ ] **Step 2: Extract shared processing**

Keep watcher guard checks in `handleNewFile`, then call a shared
`processInput(url:)`. On failure, store the original `sourcePath`.

- [ ] **Step 3: Add explicit retry**

Add `retry(_:)` to `AppState`. It calls `processInput(url:)` for an existing
source path and records a new failure when the source file is unavailable.
Thread a retry closure through `PipelineView` into `FiledColumn`. Add `Retry`
to the context menu only for failed cards.

- [ ] **Step 4: Compile-check**

Run: `swift test`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Chaos/AppState.swift Chaos/Views/PipelineView.swift Chaos/Views/Editorial/FiledColumn.swift
git commit -m "feat: restore history and retry failed images"
```

## Task 3: Add Drag-and-Drop Intake

**Files:**
- Create: `Chaos/Models/ImageIntake.swift`
- Create: `Tests/ChaosTests/ImageIntakeTests.swift`
- Modify: `Chaos/AppState.swift`
- Modify: `Chaos/Views/DashboardView.swift`
- Modify: `Chaos/Views/Editorial/HeroCard.swift`

- [ ] **Step 1: Write failing extension tests**

Assert PNG, JPG, JPEG, HEIC, and WebP are accepted case-insensitively and a PDF
is rejected.

- [ ] **Step 2: Run tests to verify RED**

Run: `swift test --filter ImageIntakeTests`

Expected: FAIL because `ImageIntake` does not exist.

- [ ] **Step 3: Implement pure intake validation**

Create `ImageIntake.accepts(url:)`. Add `processDroppedURLs(_:)` to `AppState`
and send accepted files to `processInput(url:)`.

- [ ] **Step 4: Wire the editorial UI**

Add `.dropDestination(for: URL.self)` to the Dashboard hero card. Add the quiet
idle helper line `Drop an image to file it.` to `HeroCard`.

- [ ] **Step 5: Verify GREEN and commit**

Run: `swift test`

Expected: PASS.

```bash
git add Chaos/Models/ImageIntake.swift Tests/ChaosTests/ImageIntakeTests.swift Chaos/AppState.swift Chaos/Views/DashboardView.swift Chaos/Views/Editorial/HeroCard.swift
git commit -m "feat: process dropped image files"
```

## Task 4: Add Naming Policy and Dated Subfolders

**Files:**
- Modify: `Chaos/Models/AppConfig.swift`
- Create: `Chaos/Models/NamingPolicy.swift`
- Create: `Tests/ChaosTests/NamingPolicyTests.swift`
- Modify: `Chaos/Services/FileProcessor.swift`
- Modify: `Chaos/Services/FileRenamer.swift`
- Modify: `Tests/ChaosTests/FileProcessorTests.swift`
- Modify: `Chaos/AppState.swift`
- Modify: `Chaos/Views/SettingsView.swift`

- [ ] **Step 1: Write failing naming tests**

Assert default rendering, `{date}` and `{time}` rendering with a fixed date,
empty-template fallback, day and month output directories, extension
preservation, and collision suffixes.

- [ ] **Step 2: Run tests to verify RED**

Run: `swift test --filter NamingPolicyTests`

Expected: FAIL because `NamingPolicy` does not exist.

- [ ] **Step 3: Implement naming policy**

Add optional `filename_template` and `subfolder_rule` config keys. Create
`NamingPolicy`, `SubfolderRule`, and a deterministic renderer. Extend
`FileRenamer.moveScreenshot` to accept the rendered base name and preserve the
input extension.

- [ ] **Step 4: Integrate processing**

Pass policy from `AppState` to `FileProcessor`, resolve the dated destination
directory during processing, and update processor tests.

- [ ] **Step 5: Add restrained settings UI**

Add an `Organization` section with a filename-template field, supported-token
caption, and subfolder picker.

- [ ] **Step 6: Verify GREEN and commit**

Run: `swift test`

Expected: PASS.

```bash
git add Chaos/Models/AppConfig.swift Chaos/Models/NamingPolicy.swift Tests/ChaosTests/NamingPolicyTests.swift Chaos/Services/FileProcessor.swift Chaos/Services/FileRenamer.swift Tests/ChaosTests/FileProcessorTests.swift Chaos/AppState.swift Chaos/Views/SettingsView.swift
git commit -m "feat: add filename templates and dated folders"
```

## Task 5: Verify the Full Workflow

- [ ] Run `swift test`.
- [ ] Run `git diff --check`.
- [ ] Run `./build-app.sh`.
- [ ] Launch `.build/Chaos.app`.
- [ ] Inspect the Dashboard drop affordance, Settings organization section,
  Pipeline retry action, and persisted history behavior.
- [ ] Review `git status --short` and leave unrelated `DESIGN.md` untouched.


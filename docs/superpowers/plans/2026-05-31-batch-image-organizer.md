# Batch Image Organizer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Dashboard action that lets users select multiple existing screenshots and process them sequentially through Chaos's existing rename-and-move pipeline.

**Architecture:** Add an order-preserving URL filter to `ImageIntake`, then expose a UI-neutral manual-intake method in `AppState` for both drag-and-drop and picker selection. Add a secondary Dashboard action that presents a native macOS `NSOpenPanel` configured for multiple supported image files and forwards confirmed URLs to the shared method.

**Tech Stack:** Swift 6 package, SwiftUI, AppKit `NSOpenPanel`, Uniform Type Identifiers, XCTest.

---

## File Structure

- Modify `Chaos/Models/ImageIntake.swift`: centralize order-preserving filtering of supported image URLs.
- Modify `Tests/ChaosTests/ImageIntakeTests.swift`: cover mixed URL filtering and order preservation.
- Modify `Chaos/AppState.swift`: expose a UI-neutral sequential manual-intake method.
- Modify `Chaos/Views/DashboardView.swift`: add the organizer action and native multi-file picker.

### Task 1: Shared manual-intake filter

**Files:**
- Modify: `Tests/ChaosTests/ImageIntakeTests.swift`
- Modify: `Chaos/Models/ImageIntake.swift`

- [ ] **Step 1: Write the failing order-preserving filter test**

Add to `ImageIntakeTests`:

```swift
func testAcceptedURLsFiltersUnsupportedFilesAndPreservesSelectionOrder() {
    let urls = [
        URL(fileURLWithPath: "first.PNG"),
        URL(fileURLWithPath: "notes.pdf"),
        URL(fileURLWithPath: "second.heic"),
        URL(fileURLWithPath: "third.webp"),
    ]

    XCTAssertEqual(
        ImageIntake.acceptedURLs(from: urls).map(\.lastPathComponent),
        ["first.PNG", "second.heic", "third.webp"]
    )
}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run:

```bash
swift test --filter ImageIntakeTests
```

Expected: FAIL because `ImageIntake.acceptedURLs(from:)` does not exist.

- [ ] **Step 3: Add the shared filter**

Update `Chaos/Models/ImageIntake.swift`:

```swift
enum ImageIntake {
    private static let supportedExtensions: Set<String> = [
        "png", "jpg", "jpeg", "heic", "webp",
    ]

    static func accepts(url: URL) -> Bool {
        supportedExtensions.contains(url.pathExtension.lowercased())
    }

    static func acceptedURLs(from urls: [URL]) -> [URL] {
        urls.filter(accepts)
    }
}
```

- [ ] **Step 4: Run the focused tests to verify they pass**

Run:

```bash
swift test --filter ImageIntakeTests
```

Expected: PASS with 3 tests and 0 failures.

- [ ] **Step 5: Commit**

```bash
git add Chaos/Models/ImageIntake.swift Tests/ChaosTests/ImageIntakeTests.swift
git commit -m "feat: centralize manual image intake filtering"
```

### Task 2: UI-neutral sequential manual intake

**Files:**
- Modify: `Chaos/AppState.swift`
- Modify: `Chaos/Views/DashboardView.swift`

- [ ] **Step 1: Rename the app-state entry point**

Replace `processDroppedURLs(_:)` in `Chaos/AppState.swift` with:

```swift
func processManualURLs(_ urls: [URL]) {
    let accepted = ImageIntake.acceptedURLs(from: urls)
    guard !accepted.isEmpty else { return }

    Task {
        for url in accepted {
            await processInput(url: url)
        }
    }
}
```

This retains sequential processing and makes the method appropriate for both
drag-and-drop and picker-backed selection.

- [ ] **Step 2: Route drag-and-drop through shared manual intake**

Update the Dashboard hero card drop destination:

```swift
.dropDestination(for: URL.self) { urls, _ in
    appState.processManualURLs(urls)
    return !ImageIntake.acceptedURLs(from: urls).isEmpty
}
```

- [ ] **Step 3: Build to verify the rename and wiring**

Run:

```bash
swift build
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add Chaos/AppState.swift Chaos/Views/DashboardView.swift
git commit -m "refactor: share sequential manual image intake"
```

### Task 3: Dashboard multi-image organizer action

**Files:**
- Modify: `Chaos/Views/DashboardView.swift`

- [ ] **Step 1: Import the native picker dependencies**

At the top of `Chaos/Views/DashboardView.swift`, use:

```swift
import SwiftUI
import AppKit
import UniformTypeIdentifiers
```

- [ ] **Step 2: Add the organizer action below the Dashboard hero card**

Replace `heroColumn` with:

```swift
@ViewBuilder
private var heroColumn: some View {
    VStack(alignment: .leading, spacing: Theme.sMed) {
        HeroCard(
            stage: appState.currentStage,
            currentFile: appState.currentFile,
            thumbnailPath: latestThumbnailPath,
            proposedSlug: proposedSlug,
            elapsedSeconds: heroElapsed,
            includesClipboard: appState.resolvedCopyToClipboard
        )
        .dropDestination(for: URL.self) { urls, _ in
            appState.processManualURLs(urls)
            return !ImageIntake.acceptedURLs(from: urls).isEmpty
        }

        Button(action: organizeExistingScreenshots) {
            Label("Organize Existing Screenshots", systemImage: "photo.on.rectangle.angled")
                .font(Theme.button)
        }
        .buttonStyle(.bordered)
        .tint(Theme.coral)
    }
}
```

- [ ] **Step 3: Present a multi-file image picker**

Add to `DashboardView`:

```swift
private func organizeExistingScreenshots() {
    let panel = NSOpenPanel()
    panel.title = "Organize Existing Screenshots"
    panel.message = "Choose images to rename and move into your Chaos output folder."
    panel.prompt = "Organize"
    panel.allowsMultipleSelection = true
    panel.canChooseFiles = true
    panel.canChooseDirectories = false
    panel.allowedContentTypes = [.png, .jpeg, .heic, .webP]

    guard panel.runModal() == .OK else { return }
    appState.processManualURLs(panel.urls)
}
```

- [ ] **Step 4: Build and run the complete test suite**

Run:

```bash
swift test
git diff --check
./build-app.sh
```

Expected: all tests pass, no whitespace errors, and `.build/Chaos.app` builds.

- [ ] **Step 5: Launch and inspect the app**

Run:

```bash
open .build/Chaos.app
```

Inspect the Dashboard:

- `Organize Existing Screenshots` appears below the existing manual intake card.
- The action opens a native file picker.
- The picker allows selecting multiple image files.
- The picker does not allow selecting directories.
- Cancel closes the picker without starting processing.
- Existing drag-and-drop still accepts supported images.

- [ ] **Step 6: Commit**

```bash
git add Chaos/Views/DashboardView.swift
git commit -m "feat: add batch screenshot organizer picker"
```

### Task 4: Completion audit

**Files:**
- Review: `docs/superpowers/specs/2026-05-31-batch-image-organizer-design.md`
- Review: `Chaos/Models/ImageIntake.swift`
- Review: `Chaos/AppState.swift`
- Review: `Chaos/Views/DashboardView.swift`
- Review: `Tests/ChaosTests/ImageIntakeTests.swift`

- [ ] **Step 1: Confirm the feature diff is scoped**

Run:

```bash
git status --short --branch
git diff main...HEAD --stat
git diff main...HEAD -- Chaos/Models/ImageIntake.swift Chaos/AppState.swift Chaos/Views/DashboardView.swift Tests/ChaosTests/ImageIntakeTests.swift
```

Expected: only the spec, plan, shared intake filter, app-state rename, Dashboard
picker, and focused tests changed.

- [ ] **Step 2: Confirm no stale drop-only intake references remain**

Run:

```bash
rg -n "processDroppedURLs|processManualURLs|acceptedURLs" Chaos Tests
```

Expected: no `processDroppedURLs` references; picker and drop destination both
route through `processManualURLs`; accepted URL filtering is centralized.

- [ ] **Step 3: Run final verification**

Run:

```bash
swift test
git diff --check main...HEAD
./build-app.sh
```

Expected: all tests pass, no whitespace errors, and the app bundle builds.

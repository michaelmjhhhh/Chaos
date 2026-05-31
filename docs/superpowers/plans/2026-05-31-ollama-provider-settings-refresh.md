# Ollama Provider and Settings Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Ollama as a first-class optional local provider and replace the confusing Settings form with a clearer editorial card layout.

**Architecture:** Extend `Provider` as the single source of preset metadata, keep provider-switch semantics in `AppState`, and preserve the existing OpenAI-compatible network path. Extract stateless Settings presentation components so `SettingsView` remains focused on bindings and actions.

**Tech Stack:** Swift 6 package, SwiftUI for macOS 15+, XCTest, URLSession.

---

## File Structure

- Create `Tests/ChaosTests/ProviderTests.swift`: provider preset and AppState resolution tests.
- Create `Chaos/Views/SettingsComponents.swift`: stateless editorial Settings card, header, badge, and connection-result views.
- Modify `Chaos/Models/Provider.swift`: Ollama case and provider presentation metadata.
- Modify `Chaos/AppState.swift`: resolved API key, startup validation, and provider-switch reset behavior.
- Modify `Chaos/Services/VisionAPIClient.swift`: reject malformed base URLs without force-unwrapping.
- Modify `Chaos/Views/SettingsView.swift`: compose the refreshed Settings screen.
- Modify `README.md`: document Ollama setup and the expanded provider list.

### Task 1: Provider preset metadata

**Files:**
- Create: `Tests/ChaosTests/ProviderTests.swift`
- Modify: `Chaos/Models/Provider.swift`

- [ ] **Step 1: Write failing Ollama provider tests**

```swift
import XCTest
@testable import Chaos

final class ProviderTests: XCTestCase {
    func testOllamaPresetUsesLocalVisionDefaultsWithoutAPIKey() {
        let provider = Provider.ollama
        XCTAssertEqual(provider.defaultBaseURL, "http://localhost:11434/v1")
        XCTAssertEqual(provider.defaultModel, "qwen3-vl:2b")
        XCTAssertFalse(provider.requiresAPIKey)
        XCTAssertFalse(provider.allowsCustomBaseURL)
        XCTAssertEqual(provider.connectionKind, "Local")
    }

    func testOpenAICompatibleRequiresEditableBaseURL() {
        XCTAssertTrue(Provider.openaiCompatible.requiresAPIKey)
        XCTAssertTrue(Provider.openaiCompatible.allowsCustomBaseURL)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `swift test --filter ProviderTests`

Expected: FAIL because `Provider.ollama`, `requiresAPIKey`,
`allowsCustomBaseURL`, and `connectionKind` do not exist.

- [ ] **Step 3: Implement provider metadata**

Add `.ollama` and switch branches for:

```swift
var requiresAPIKey: Bool { self != .ollama }
var allowsCustomBaseURL: Bool { self == .openaiCompatible }
var connectionKind: String { self == .ollama ? "Local" : "Remote" }
var summary: String { /* concise provider-specific copy */ }
var connectionFailureHint: String { /* Ollama pull hint or remote hint */ }
```

Use `http://localhost:11434/v1` and `qwen3-vl:2b` for Ollama defaults.

- [ ] **Step 4: Run tests to verify they pass**

Run: `swift test --filter ProviderTests`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Chaos/Models/Provider.swift Tests/ChaosTests/ProviderTests.swift
git commit -m "feat: add ollama provider preset"
```

### Task 2: AppState provider switching and API-key rules

**Files:**
- Modify: `Tests/ChaosTests/ProviderTests.swift`
- Modify: `Chaos/AppState.swift`

- [ ] **Step 1: Add failing state-resolution tests**

```swift
@MainActor
func testSelectingProviderClearsStaleOverridesAndPreservesAPIKey() {
    let state = AppState()
    state.config = AppConfig(
        provider: "openai",
        apiKey: "saved-key",
        model: "old-model",
        baseURL: "https://old.example/v1"
    )
    state.selectProvider(.ollama)
    XCTAssertEqual(state.config.provider, "ollama")
    XCTAssertNil(state.config.model)
    XCTAssertNil(state.config.baseURL)
    XCTAssertEqual(state.config.apiKey, "saved-key")
    XCTAssertEqual(state.resolvedModel, "qwen3-vl:2b")
    XCTAssertEqual(state.resolvedBaseURL, "http://localhost:11434/v1")
}

@MainActor
func testOllamaDoesNotRequireAPIKeyToStart() {
    let state = AppState()
    state.config = AppConfig(provider: "ollama")
    XCTAssertEqual(state.resolvedAPIKey, "")
    XCTAssertNil(state.startupValidationError)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `swift test --filter ProviderTests`

Expected: FAIL because `selectProvider`, `resolvedAPIKey`, and
`startupValidationError` do not exist.

- [ ] **Step 3: Implement state behavior**

Add:

```swift
var resolvedAPIKey: String {
    resolvedProvider.requiresAPIKey ? (config.apiKey ?? "") : ""
}

var startupValidationError: String? {
    if resolvedProvider.requiresAPIKey &&
        resolvedAPIKey.trimmingCharacters(in: .whitespaces).isEmpty {
        return "API key not configured"
    }
    return nil
}

func selectProvider(_ provider: Provider) {
    guard provider != resolvedProvider else { return }
    config.provider = provider.rawValue
    config.model = nil
    config.baseURL = nil
}
```

Use `startupValidationError` in `start()` and pass `resolvedAPIKey` through
health checks and processing.

- [ ] **Step 4: Run tests to verify they pass**

Run: `swift test --filter ProviderTests`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Chaos/AppState.swift Tests/ChaosTests/ProviderTests.swift
git commit -m "feat: resolve provider-specific connection settings"
```

### Task 3: Invalid endpoint guard

**Files:**
- Modify: `Tests/ChaosTests/FileProcessorTests.swift`
- Modify: `Chaos/Services/VisionAPIClient.swift`

- [ ] **Step 1: Add failing invalid-base-URL test**

```swift
func testHealthCheckRejectsEmptyBaseURL() async {
    let processor = FileProcessor()
    let healthy = await processor.checkAPIHealth(
        baseURL: "",
        apiKey: "test-key",
        model: "test-model"
    )
    XCTAssertFalse(healthy)
}
```

- [ ] **Step 2: Run test to verify the crash or failure**

Run: `swift test --filter FileProcessorTests/testHealthCheckRejectsEmptyBaseURL`

Expected: FAIL because the network client force-unwraps an invalid URL.

- [ ] **Step 3: Guard endpoint construction**

Replace force-unwrapped URL construction in both network methods with:

```swift
guard let url = URL(string: "\(baseURL.trimmingSuffix("/"))/chat/completions"),
      let scheme = url.scheme,
      ["http", "https"].contains(scheme),
      url.host != nil else {
    throw ChaosError.apiError("Invalid provider base URL")
}
```

- [ ] **Step 4: Run focused tests**

Run: `swift test --filter FileProcessorTests`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add Chaos/Services/VisionAPIClient.swift Tests/ChaosTests/FileProcessorTests.swift
git commit -m "fix: reject invalid provider endpoints"
```

### Task 4: Editorial Settings refresh

**Files:**
- Create: `Chaos/Views/SettingsComponents.swift`
- Modify: `Chaos/Views/SettingsView.swift`

- [ ] **Step 1: Create stateless presentation components**

Implement:

```swift
struct SettingsCard<Content: View>: View {
    let title: String
    let subtitle: String?
    @ViewBuilder let content: Content
}

struct SettingsCardHeader: View {
    let title: String
    let subtitle: String?
}

struct SettingsBadge: View {
    let text: String
    let systemImage: String
    let tint: Color
}

struct SettingsConnectionResult: View {
    let status: String
    let failureHint: String
}
```

Use `Theme.canvas`, `Theme.surfaceCard`, `Theme.border`, `Theme.coral`,
semantic colors, system icons, and readable labels.

- [ ] **Step 2: Recompose SettingsView**

Replace the grouped form with a `ScrollView` and card stack. Use:

```swift
Picker("Provider", selection: providerBinding) { ... }
if appState.resolvedProvider.requiresAPIKey { SecureField(...) }
if appState.resolvedProvider.allowsCustomBaseURL { TextField(...) }
else { Text(appState.resolvedBaseURL) }
```

Keep existing directory, output, behavior, save-on-change, and reveal-config
functionality. Route the provider picker setter through
`appState.selectProvider(_:)`.

- [ ] **Step 3: Build to verify SwiftUI composition**

Run: `swift build`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add Chaos/Views/SettingsComponents.swift Chaos/Views/SettingsView.swift
git commit -m "refactor(settings): clarify provider configuration"
```

### Task 5: Documentation and complete verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document Ollama setup**

Add Ollama to the provider table and include:

```bash
ollama pull qwen3-vl:2b
```

Explain that users install and run Ollama separately, then select `Ollama` in
Chaos Settings. No API key is required.

- [ ] **Step 2: Run verification**

Run:

```bash
swift test
git diff --check
./build-app.sh
```

Expected: all tests pass, no whitespace errors, and the app bundle builds.

- [ ] **Step 3: Launch and visually inspect Settings**

Run:

```bash
open .build/Chaos.app
```

Inspect provider switching, API-key visibility, model defaults, custom URL
visibility, and connection-result readability.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: explain ollama provider setup"
```

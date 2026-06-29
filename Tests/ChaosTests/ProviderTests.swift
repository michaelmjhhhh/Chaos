import XCTest
@testable import Chaos

final class ProviderTests: XCTestCase {
    func testOllamaPresetUsesLocalVisionDefaultsWithoutAPIKey() {
        let provider = Provider.ollama
        XCTAssertEqual(provider.displayName, "Ollama")
        XCTAssertEqual(provider.defaultBaseURL, "http://localhost:11434/v1")
        XCTAssertEqual(provider.defaultModel, "qwen3-vl:2b")
        XCTAssertFalse(provider.requiresAPIKey)
        XCTAssertFalse(provider.allowsCustomBaseURL)
        XCTAssertEqual(provider.connectionKind, "Local")
        XCTAssertEqual(provider.summary, "Local vision model served by Ollama.")
        XCTAssertEqual(provider.connectionFailureHint, "Start Ollama, then run: ollama pull qwen3-vl:2b")
    }

    func testOpenAICompatibleRequiresEditableBaseURL() {
        XCTAssertTrue(Provider.openaiCompatible.requiresAPIKey)
        XCTAssertTrue(Provider.openaiCompatible.allowsCustomBaseURL)
        XCTAssertEqual(Provider.openaiCompatible.summary, "Custom OpenAI-compatible endpoint.")
    }

    func testRemoteProviderUsesRemoteConnectionMetadata() {
        let provider = Provider.openai
        XCTAssertEqual(provider.connectionKind, "Remote")
        XCTAssertEqual(provider.summary, "OpenAI hosted API.")
        XCTAssertEqual(provider.connectionFailureHint, "Check your API key and network connection.")
    }

    func testOllamaRawValueResolvesPreset() {
        XCTAssertEqual(Provider.from("ollama"), .ollama)
    }

    func testProviderConnectionRulesRemainScopedToExpectedPresets() {
        let keyless: Set<Provider> = [.ollama, .chaosHosted]
        for provider in Provider.allCases {
            XCTAssertEqual(provider.requiresAPIKey, !keyless.contains(provider), "\(provider)")
            XCTAssertEqual(provider.allowsCustomBaseURL, provider == .openaiCompatible, "\(provider)")
            let expectedKind = switch provider {
            case .ollama: "Local"
            case .chaosHosted: "Built-in"
            default: "Remote"
            }
            XCTAssertEqual(provider.connectionKind, expectedKind, "\(provider)")
        }
    }

    func testChaosHostedProviderNeedsNoKeyAndOffersNoSignup() {
        let provider = Provider.chaosHosted
        XCTAssertFalse(provider.requiresAPIKey)
        XCTAssertFalse(provider.allowsCustomBaseURL)
        XCTAssertNil(provider.signupURL)
        XCTAssertEqual(provider.connectionKind, "Built-in")
    }

    func testEveryKeyedProviderExposesASignupLink() {
        for provider in Provider.allCases where provider.requiresAPIKey && provider != .openaiCompatible {
            XCTAssertNotNil(provider.signupURL, "\(provider) should offer a way to get a key")
        }
    }

    @MainActor
    func testSelectingProviderClearsStaleOverridesAndPreservesAPIKey() {
        let state = AppState()
        state.config = AppConfig(provider: "openai", apiKey: "saved-key", model: "old-model", baseURL: "https://old.example/v1")
        state.selectProvider(.ollama)
        XCTAssertEqual(state.config.provider, "ollama")
        XCTAssertNil(state.config.model)
        XCTAssertNil(state.config.baseURL)
        XCTAssertEqual(state.config.apiKey, "saved-key")
        XCTAssertEqual(state.resolvedModel, "qwen3-vl:2b")
        XCTAssertEqual(state.resolvedBaseURL, "http://localhost:11434/v1")
    }

    @MainActor
    func testSelectingCurrentProviderPreservesOverrides() {
        let state = AppState()
        state.config = AppConfig(provider: "openai", apiKey: "saved-key", model: "custom-model", baseURL: "https://custom.example/v1")
        state.selectProvider(.openai)
        XCTAssertEqual(state.config.provider, "openai")
        XCTAssertEqual(state.config.model, "custom-model")
        XCTAssertEqual(state.config.baseURL, "https://custom.example/v1")
        XCTAssertEqual(state.config.apiKey, "saved-key")
    }

    @MainActor
    func testPresetProviderIgnoresLegacyCustomBaseURL() {
        let state = AppState()
        state.config = AppConfig(provider: "openai", baseURL: "https://legacy-proxy.example/v1")
        XCTAssertEqual(state.resolvedBaseURL, "https://api.openai.com/v1")
    }

    @MainActor
    func testOpenAICompatibleUsesCustomBaseURL() {
        let state = AppState()
        state.config = AppConfig(provider: "openai-compatible", baseURL: "https://custom.example/v1")
        XCTAssertEqual(state.resolvedBaseURL, "https://custom.example/v1")
    }

    @MainActor
    func testOllamaDoesNotRequireAPIKeyToStart() throws {
        let watchURL = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: watchURL, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: watchURL) }

        let state = AppState()
        state.config = AppConfig(provider: "ollama", watchDir: watchURL.path)
        XCTAssertEqual(state.resolvedAPIKey, "")
        XCTAssertNil(state.startupValidationError)

        state.start()
        XCTAssertEqual(state.watcherStatus, .running)
        state.stop()
    }

    @MainActor
    func testRemoteProviderRequiresNonWhitespaceAPIKeyToStart() {
        let state = AppState()
        state.config = AppConfig(provider: "openai", apiKey: " \t ")
        XCTAssertEqual(state.resolvedAPIKey, " \t ")
        XCTAssertEqual(state.startupValidationError, "API key not configured")

        state.start()
        XCTAssertEqual(state.watcherStatus, .error("API key not configured"))
    }
}

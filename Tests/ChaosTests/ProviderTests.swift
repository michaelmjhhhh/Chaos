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
}

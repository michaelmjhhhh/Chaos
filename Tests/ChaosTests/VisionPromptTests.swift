import XCTest
@testable import Chaos

final class VisionPromptTests: XCTestCase {
    func testResolveSystemPromptUsesDefaultWhenCustomIsNil() {
        let resolved = VisionAPIClient.resolveSystemPrompt(language: .en, customSystemPrompt: nil)
        XCTAssertEqual(resolved, VisionAPIClient.defaultSystemPrompt(language: .en))
    }

    func testResolveSystemPromptUsesDefaultWhenCustomIsBlank() {
        let resolved = VisionAPIClient.resolveSystemPrompt(language: .en, customSystemPrompt: "   \n  ")
        XCTAssertEqual(resolved, VisionAPIClient.defaultSystemPrompt(language: .en))
    }

    func testResolveSystemPromptUsesCustomVerbatimWhenProvided() {
        let custom = "Name screenshots after the dominant UI element and the visible date."
        let resolved = VisionAPIClient.resolveSystemPrompt(language: .en, customSystemPrompt: custom)
        XCTAssertEqual(resolved, custom)
    }

    func testCustomPromptOverridesLanguageDefault() {
        // A custom prompt wins regardless of the language setting.
        let custom = "always name files in pirate-speak"
        XCTAssertEqual(
            VisionAPIClient.resolveSystemPrompt(language: .zh, customSystemPrompt: custom),
            custom
        )
    }

    func testDefaultSystemPromptDiffersByLanguage() {
        XCTAssertNotEqual(
            VisionAPIClient.defaultSystemPrompt(language: .en),
            VisionAPIClient.defaultSystemPrompt(language: .zh)
        )
    }

    // MARK: - Request payload assembly

    private func systemContent(_ messages: [[String: Any]]) -> String? {
        messages.first { $0["role"] as? String == "system" }?["content"] as? String
    }

    private func userText(_ messages: [[String: Any]]) -> String? {
        guard let user = messages.first(where: { $0["role"] as? String == "user" }),
              let parts = user["content"] as? [[String: Any]],
              let textPart = parts.first(where: { $0["type"] as? String == "text" })
        else { return nil }
        return textPart["text"] as? String
    }

    func testChatMessagesPutsCustomPromptInSystemTurn() {
        let custom = "name files after the visible app icon"
        let messages = VisionAPIClient.chatMessages(
            imageBase64: "abc", mimeType: "image/png", language: .en, customSystemPrompt: custom
        )
        XCTAssertEqual(systemContent(messages), custom)
    }

    func testChatMessagesUsesDefaultSystemTurnWhenCustomNil() {
        let messages = VisionAPIClient.chatMessages(
            imageBase64: "abc", mimeType: "image/png", language: .en, customSystemPrompt: nil
        )
        XCTAssertEqual(systemContent(messages), VisionAPIClient.defaultSystemPrompt(language: .en))
    }

    func testChatMessagesUsesLanguageNeutralUserTurnWhenCustomActive() {
        // A custom prompt overrides the language, so the user-turn nudge must not force Chinese.
        let messages = VisionAPIClient.chatMessages(
            imageBase64: "abc", mimeType: "image/png", language: .zh, customSystemPrompt: "pirate names only"
        )
        XCTAssertEqual(userText(messages), "Return only the filename slug.")
    }

    func testChatMessagesEmbedsImageAsDataURL() {
        let messages = VisionAPIClient.chatMessages(
            imageBase64: "BASE64DATA", mimeType: "image/png", language: .en, customSystemPrompt: nil
        )
        let user = messages.first { $0["role"] as? String == "user" }
        let parts = user?["content"] as? [[String: Any]]
        let imageURL = parts?.first { $0["type"] as? String == "image_url" }?["image_url"] as? [String: String]
        XCTAssertEqual(imageURL?["url"], "data:image/png;base64,BASE64DATA")
    }
}

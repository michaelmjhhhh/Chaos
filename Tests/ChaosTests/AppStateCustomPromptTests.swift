import XCTest
@testable import Chaos

@MainActor
final class AppStateCustomPromptTests: XCTestCase {
    func testResolvedCustomPromptNilWhenToggleOff() {
        let state = AppState()
        state.config = AppConfig(useCustomPrompt: false, customPrompt: "name by app")
        XCTAssertNil(state.resolvedCustomPrompt)
    }

    func testResolvedCustomPromptNilWhenToggleUnset() {
        let state = AppState()
        state.config = AppConfig(customPrompt: "name by app")
        XCTAssertNil(state.resolvedCustomPrompt)
    }

    func testResolvedCustomPromptNilWhenTextMissing() {
        let state = AppState()
        state.config = AppConfig(useCustomPrompt: true, customPrompt: nil)
        XCTAssertNil(state.resolvedCustomPrompt)
    }

    func testResolvedCustomPromptNilWhenTextBlank() {
        let state = AppState()
        state.config = AppConfig(useCustomPrompt: true, customPrompt: "   \n\t ")
        XCTAssertNil(state.resolvedCustomPrompt)
    }

    func testResolvedCustomPromptReturnsTrimmedTextWhenEnabled() {
        let state = AppState()
        state.config = AppConfig(useCustomPrompt: true, customPrompt: "  name files by window title  ")
        XCTAssertEqual(state.resolvedCustomPrompt, "name files by window title")
    }
}

import XCTest
@testable import Chaos

final class FriendlyErrorTests: XCTestCase {
    func testUnauthorizedSuggestsOpeningSettings() {
        let friendly = FriendlyError(ChaosError.httpStatus(401), provider: .openai)
        XCTAssertEqual(friendly.action, .openSettings)
        XCTAssertTrue(friendly.message.localizedCaseInsensitiveContains("key"))
        XCTAssertFalse(friendly.message.contains("401"))
    }

    func testRateLimitSuggestsRetry() {
        let friendly = FriendlyError(ChaosError.httpStatus(429), provider: .openai)
        XCTAssertEqual(friendly.action, .retry)
    }

    func testServerErrorSuggestsRetry() {
        let friendly = FriendlyError(ChaosError.httpStatus(503), provider: .openai)
        XCTAssertEqual(friendly.action, .retry)
    }

    func testFileNotStableIsRetryable() {
        let friendly = FriendlyError(ChaosError.fileNotStable("shot.png"), provider: .openai)
        XCTAssertEqual(friendly.action, .retry)
        XCTAssertFalse(friendly.message.contains("shot.png"))
    }

    func testOfflineUrlErrorAsksToCheckInternet() {
        let friendly = FriendlyError(URLError(.notConnectedToInternet), provider: .openai)
        XCTAssertEqual(friendly.action, .checkInternet)
    }

    func testOllamaConnectionFailureMentionsOllama() {
        let friendly = FriendlyError(URLError(.cannotConnectToHost), provider: .ollama)
        XCTAssertTrue(friendly.message.localizedCaseInsensitiveContains("ollama"))
    }

    func testRecoveryActionLabels() {
        XCTAssertEqual(RecoveryAction.retry.label, "Try Again")
        XCTAssertEqual(RecoveryAction.openSettings.label, "Open Settings")
        XCTAssertNil(RecoveryAction.none.label)
    }
}

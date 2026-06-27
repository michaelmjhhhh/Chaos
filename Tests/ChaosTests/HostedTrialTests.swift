import XCTest
@testable import Chaos

final class HostedTrialTests: XCTestCase {
    func testDeviceHashIsStableHex() {
        let a = DeviceIdentity.hash
        let b = DeviceIdentity.hash
        XCTAssertEqual(a, b, "device hash must be stable within a run")
        XCTAssertEqual(a.count, 64, "SHA-256 hex is 64 chars")
        XCTAssertTrue(a.allSatisfy { $0.isHexDigit && ($0.isNumber || $0.isLowercase) },
                      "hash should be lowercase hex")
    }

    @MainActor
    func testHostedBearerCombinesAppTokenAndDeviceHash() {
        let state = AppState()
        state.config = AppConfig(provider: "chaos-hosted")
        XCTAssertEqual(state.resolvedProvider, .chaosHosted)

        let bearer = state.resolvedAPIKey
        XCTAssertTrue(bearer.hasPrefix("\(HostedProvider.bundledCredential):"),
                      "bearer should start with the app token and a colon")
        XCTAssertTrue(bearer.hasSuffix(DeviceIdentity.hash),
                      "bearer should end with this device's hash")
    }

    func testChaosHostedTrialExhaustionMessageIsFriendly() {
        let friendly = FriendlyError(ChaosError.httpStatus(402), provider: .chaosHosted)
        XCTAssertEqual(friendly.action, .openSettings)
        XCTAssertTrue(friendly.message.localizedCaseInsensitiveContains("free"))
        XCTAssertFalse(friendly.message.contains("402"))
    }

    func testNonHostedPaymentRequiredKeepsGenericMessage() {
        let friendly = FriendlyError(ChaosError.httpStatus(402), provider: .openai)
        XCTAssertTrue(friendly.message.localizedCaseInsensitiveContains("credit"))
    }
}

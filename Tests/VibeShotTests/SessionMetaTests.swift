import XCTest
@testable import VibeShot

final class SessionMetaTests: XCTestCase {
    private let defaultsKey = "vibeshot.test.sessionNumber"

    override func setUp() {
        super.setUp()
        UserDefaults.standard.removeObject(forKey: defaultsKey)
    }

    override func tearDown() {
        UserDefaults.standard.removeObject(forKey: defaultsKey)
        super.tearDown()
    }

    func testFirstLaunchReturnsOne() {
        let meta = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(meta.sessionNumber, 1)
    }

    func testSubsequentLaunchesIncrement() {
        _ = SessionMeta(defaultsKey: defaultsKey)
        let second = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(second.sessionNumber, 2)

        let third = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertEqual(third.sessionNumber, 3)
    }

    func testStartedAtIsRecent() {
        let meta = SessionMeta(defaultsKey: defaultsKey)
        XCTAssertLessThan(abs(meta.startedAt.timeIntervalSinceNow), 2.0)
    }
}

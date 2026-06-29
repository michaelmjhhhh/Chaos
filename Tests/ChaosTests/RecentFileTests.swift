import XCTest
@testable import Chaos

final class RecentFileTests: XCTestCase {
    func testSearchKeyLowercasesNamesForSuccess() {
        let file = RecentFile(
            originalName: "Screenshot 2026.PNG",
            newName: "Terminal_GitLog.png",
            path: "/out/Terminal_GitLog.png",
            timestamp: Date(),
            duration: 1,
            result: .success
        )

        XCTAssertEqual(file.searchKey, "terminal_gitlog.png screenshot 2026.png")
        // Success files do not expose the "ok" result text to search.
        XCTAssertFalse(file.searchKey.contains("ok"))
    }

    func testSearchKeyIncludesErrorMessageForErrors() {
        let file = RecentFile(
            originalName: "Dropped.jpg",
            newName: "",
            path: "",
            timestamp: Date(),
            duration: 0,
            result: .error("API Timeout")
        )

        XCTAssertTrue(file.searchKey.contains("api timeout"))
        XCTAssertTrue(file.searchKey.contains("dropped.jpg"))
    }
}

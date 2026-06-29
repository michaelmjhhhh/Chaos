import XCTest
@testable import Chaos

final class HistoryStoreTests: XCTestCase {
    func testSaveAndLoadRoundTripsSuccessAndErrorRecords() throws {
        let paths = TemporaryHistoryPaths()
        defer { paths.remove() }
        let success = RecentFile(
            originalName: "Screenshot.png",
            newName: "terminal_120000.png",
            path: "/output/terminal_120000.png",
            sourcePath: "/input/Screenshot.png",
            timestamp: Date(timeIntervalSince1970: 100),
            duration: 1.25,
            result: .success
        )
        let failure = RecentFile(
            originalName: "Dropped.jpg",
            newName: "",
            path: "",
            sourcePath: "/input/Dropped.jpg",
            timestamp: Date(timeIntervalSince1970: 200),
            duration: 0,
            result: .error("API timeout")
        )

        let store = HistoryStore(historyURL: paths.historyURL)
        try store.save([failure, success])

        XCTAssertEqual(store.load(), [failure, success])
    }

    func testSaveKeepsNewestFiveHundredRecords() throws {
        let paths = TemporaryHistoryPaths()
        defer { paths.remove() }
        let files = (0 ..< 501).map { index in
            RecentFile(
                originalName: "Screenshot-\(index).png",
                newName: "file-\(index).png",
                path: "/output/file-\(index).png",
                sourcePath: "/input/Screenshot-\(index).png",
                timestamp: Date(timeIntervalSince1970: TimeInterval(index)),
                duration: 1,
                result: .success
            )
        }

        let store = HistoryStore(historyURL: paths.historyURL)
        try store.save(files)

        let loaded = store.load()
        XCTAssertEqual(loaded.count, 500)
        XCTAssertEqual(loaded.first?.originalName, "Screenshot-0.png")
        XCTAssertEqual(loaded.last?.originalName, "Screenshot-499.png")
    }

    func testLoadMalformedJSONReturnsEmptyHistory() throws {
        let paths = TemporaryHistoryPaths()
        defer { paths.remove() }
        try FileManager.default.createDirectory(
            at: paths.historyURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try Data("not json".utf8).write(to: paths.historyURL)

        XCTAssertEqual(HistoryStore(historyURL: paths.historyURL).load(), [])
    }
}

private struct TemporaryHistoryPaths {
    let directory = FileManager.default.temporaryDirectory
        .appendingPathComponent(UUID().uuidString)

    var historyURL: URL {
        directory.appendingPathComponent("chaos/history.json")
    }

    func remove() {
        try? FileManager.default.removeItem(at: directory)
    }
}

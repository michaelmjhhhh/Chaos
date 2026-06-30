import XCTest
@testable import Chaos

@MainActor
final class AppStateRetryTests: XCTestCase {
    private var tmp: URL!

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    private func makeFile(_ name: String) throws -> String {
        let url = tmp.appendingPathComponent(name)
        try Data("x".utf8).write(to: url)
        return url.path
    }

    private func failure(source: String) -> RecentFile {
        RecentFile(
            originalName: "Screenshot.png", newName: "", path: "",
            sourcePath: source, timestamp: Date(), duration: 0, result: .error("API timeout")
        )
    }

    func testRetryableFailuresFiltersDedupesAndChecksDisk() throws {
        let existing = try makeFile("a.png")
        let missing = tmp.appendingPathComponent("gone.png").path

        let state = AppState()
        state.recentFiles = [
            failure(source: existing), // retryable
            failure(source: existing), // duplicate source → deduped out
            failure(source: missing), // source no longer on disk → excluded
            failure(source: ""), // no source path → excluded
            RecentFile( // success → excluded
                originalName: "ok.png", newName: "named.png", path: existing,
                sourcePath: existing, timestamp: Date(), duration: 1, result: .success
            )
        ]

        let retryable = state.retryableFailures
        XCTAssertEqual(retryable.count, 1)
        XCTAssertEqual(retryable.first?.sourcePath, existing)
    }
}

import XCTest
@testable import VibeShot

final class DirectoryWatcherTests: XCTestCase {
    func testStartReturnsFalseWhenDirectoryCannotBeOpened() {
        let missingDirectory = FileManager.default.temporaryDirectory
            .appendingPathComponent(UUID().uuidString)
        let watcher = DirectoryWatcher(directory: missingDirectory)

        XCTAssertFalse(watcher.start { _ in })
    }
}

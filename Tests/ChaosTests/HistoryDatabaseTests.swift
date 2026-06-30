import XCTest
@testable import Chaos

@MainActor
final class HistoryDatabaseTests: XCTestCase {
    private var tmp: URL!

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    private func makeDatabase() throws -> HistoryDatabase {
        try HistoryDatabase(url: tmp.appendingPathComponent("history.sqlite"), importingLegacyJSONFrom: nil)
    }

    private func success(_ name: String, at date: Date, duration: Double = 1) -> RecentFile {
        RecentFile(
            originalName: "Screenshot.png",
            newName: name,
            path: "/out/\(name)",
            sourcePath: "/in/Screenshot.png",
            timestamp: date,
            duration: duration,
            result: .success
        )
    }

    func testInsertAndRecentFilesRoundTripsNewestFirst() async throws {
        let db = try makeDatabase()
        let older = success("a.png", at: Date(timeIntervalSince1970: 100))
        let newer = success("b.png", at: Date(timeIntervalSince1970: 200))

        try await db.insert(older)
        try await db.insert(newer)

        let loaded = try await db.recentFiles(limit: 10)
        XCTAssertEqual(loaded.map(\.newName), ["b.png", "a.png"])
        XCTAssertEqual(loaded.first?.result, .success)
    }

    func testDeleteRemovesRow() async throws {
        let db = try makeDatabase()
        let file = success("a.png", at: Date(timeIntervalSince1970: 100))
        try await db.insert(file)

        try await db.delete(id: file.id)

        let loaded = try await db.recentFiles(limit: 10)
        XCTAssertTrue(loaded.isEmpty)
    }

    func testUpdateChangesNameAndExtension() async throws {
        let db = try makeDatabase()
        let file = success("a.png", at: Date(timeIntervalSince1970: 100))
        try await db.insert(file)

        try await db.update(id: file.id, newName: "renamed.jpg", path: "/out/renamed.jpg")

        let loaded = try await db.recentFiles(limit: 10)
        XCTAssertEqual(loaded.first?.newName, "renamed.jpg")
        XCTAssertEqual(loaded.first?.path, "/out/renamed.jpg")
    }

    func testErrorRecordPreservesMessage() async throws {
        let db = try makeDatabase()
        let failure = RecentFile(
            originalName: "Dropped.jpg",
            newName: "",
            path: "",
            sourcePath: "/in/Dropped.jpg",
            timestamp: Date(timeIntervalSince1970: 100),
            duration: 0,
            result: .error("API timeout")
        )
        try await db.insert(failure)

        let loaded = try await db.recentFiles(limit: 10)
        XCTAssertEqual(loaded.first?.result, .error("API timeout"))
    }

    func testImportsLegacyJSONOnceAndArchivesIt() async throws {
        let legacyURL = tmp.appendingPathComponent("history.json")
        let legacyStore = HistoryStore(historyURL: legacyURL)
        try legacyStore.save([success("legacy.png", at: Date(timeIntervalSince1970: 100))])

        let db = try HistoryDatabase(url: tmp.appendingPathComponent("history.sqlite"), importingLegacyJSONFrom: legacyURL)

        let loaded = try await db.recentFiles(limit: 10)
        XCTAssertEqual(loaded.map(\.newName), ["legacy.png"])
        XCTAssertFalse(FileManager.default.fileExists(atPath: legacyURL.path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: legacyURL.appendingPathExtension("migrated").path))
    }
}

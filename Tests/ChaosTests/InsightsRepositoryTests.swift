import XCTest
@testable import Chaos

@MainActor
final class InsightsRepositoryTests: XCTestCase {
    private var tmp: URL!
    private var db: HistoryDatabase!
    private var repository: InsightsRepository!
    private let calendar = Calendar.current

    override func setUpWithError() throws {
        tmp = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
        db = try HistoryDatabase(url: tmp.appendingPathComponent("history.sqlite"), importingLegacyJSONFrom: nil)
        repository = InsightsRepository(dbQueue: db.dbQueue)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: tmp)
    }

    // MARK: - Helpers

    /// A local timestamp `daysAgo` days back at the given local hour, so SQLite's `'localtime'`
    /// grouping lands on the day/hour the test intends.
    private func date(daysAgo: Int, hour: Int = 12) -> Date {
        let startOfToday = calendar.startOfDay(for: Date())
        let day = calendar.date(byAdding: .day, value: -daysAgo, to: startOfToday)!
        return calendar.date(byAdding: .hour, value: hour, to: day)!
    }

    private func insertSuccess(_ name: String, daysAgo: Int, hour: Int = 12, duration: Double = 1) async throws {
        try await db.insert(RecentFile(
            originalName: "Screenshot.png",
            newName: name,
            path: "/out/\(name)",
            sourcePath: "/in/Screenshot.png",
            timestamp: date(daysAgo: daysAgo, hour: hour),
            duration: duration,
            result: .success
        ))
    }

    private func insertError(daysAgo: Int) async throws {
        try await db.insert(RecentFile(
            originalName: "Dropped.png",
            newName: "",
            path: "",
            sourcePath: "/in/Dropped.png",
            timestamp: date(daysAgo: daysAgo),
            duration: 0,
            result: .error("boom")
        ))
    }

    // MARK: - Tests

    func testEmptyDatabaseReturnsEmptySnapshot() async throws {
        let snapshot = try await repository.snapshot()
        XCTAssertFalse(snapshot.hasData)
        XCTAssertEqual(snapshot.totalProcessed, 0)
    }

    func testTotalsAndSuccessRate() async throws {
        try await insertSuccess("a.png", daysAgo: 0)
        try await insertSuccess("b.png", daysAgo: 0)
        try await insertSuccess("c.png", daysAgo: 0)
        try await insertError(daysAgo: 0)

        let snapshot = try await repository.snapshot()
        XCTAssertEqual(snapshot.totalProcessed, 4)
        XCTAssertEqual(snapshot.successes, 3)
        XCTAssertEqual(snapshot.errors, 1)
        XCTAssertEqual(snapshot.successRate, 0.75, accuracy: 0.0001)
    }

    func testDailyCountsAndCurrentStreak() async throws {
        // Three consecutive active days ending today.
        try await insertSuccess("today.png", daysAgo: 0)
        try await insertSuccess("yesterday.png", daysAgo: 1)
        try await insertSuccess("twoDaysAgo.png", daysAgo: 2)
        // A gap, then an older active day that should not extend the current streak.
        try await insertSuccess("old.png", daysAgo: 5)

        let snapshot = try await repository.snapshot()
        XCTAssertEqual(snapshot.daily.count, 4)
        XCTAssertEqual(snapshot.currentStreak, 3)
        XCTAssertEqual(snapshot.longestStreak, 3)
    }

    func testPeakHour() async throws {
        try await insertSuccess("a.png", daysAgo: 0, hour: 14)
        try await insertSuccess("b.png", daysAgo: 1, hour: 14)
        try await insertSuccess("c.png", daysAgo: 2, hour: 9)

        let snapshot = try await repository.snapshot()
        XCTAssertEqual(snapshot.peakHour, 14)
        XCTAssertEqual(snapshot.hourly[14], 2)
        XCTAssertEqual(snapshot.hourly[9], 1)
    }

    func testTopCategoriesFromSlugs() async throws {
        try await insertSuccess("terminal-log.png", daysAgo: 0)
        try await insertSuccess("terminal-git.png", daysAgo: 0)
        try await insertSuccess("browser-tab.png", daysAgo: 0)

        let snapshot = try await repository.snapshot()
        XCTAssertEqual(snapshot.topCategories.first?.label, "terminal")
        XCTAssertEqual(snapshot.topCategories.first?.count, 2)
    }

    func testFileTypeBreakdown() async throws {
        try await insertSuccess("a.png", daysAgo: 0)
        try await insertSuccess("b.png", daysAgo: 0)
        try await insertSuccess("c.jpg", daysAgo: 0)
        try await insertError(daysAgo: 0) // errors excluded from the breakdown

        let snapshot = try await repository.snapshot()
        let counts = Dictionary(uniqueKeysWithValues: snapshot.fileTypes.map { ($0.label, $0.count) })
        XCTAssertEqual(counts["png"], 2)
        XCTAssertEqual(counts["jpg"], 1)
    }

    func testPerformanceStats() async throws {
        try await insertSuccess("fast.png", daysAgo: 0, duration: 1)
        try await insertSuccess("mid.png", daysAgo: 0, duration: 2)
        try await insertSuccess("slow.png", daysAgo: 0, duration: 3)

        let snapshot = try await repository.snapshot()
        XCTAssertEqual(snapshot.avgDuration, 2, accuracy: 0.0001)
        XCTAssertEqual(snapshot.fastest?.name, "fast.png")
        XCTAssertEqual(snapshot.slowest?.name, "slow.png")
    }
}

import Foundation
import GRDB

/// Read-only analytics over the full image history. Every statistic is one SQL query (or a
/// small in-Swift reduction over one column), assembled into a single `InsightsSnapshot`. The
/// whole snapshot is computed inside one database read so the page never mixes numbers from
/// different points in time.
///
/// Time math uses SQLite's `'localtime'` modifier so the heatmap days and peak hour reflect the
/// user's wall clock, not UTC (GRDB stores dates as UTC text).
struct InsightsRepository: Sendable {
    let dbQueue: DatabaseQueue

    /// Rough manual effort each successfully filed screenshot saves — eyeballing it, typing a
    /// sensible name, and dragging it into a folder. One tunable constant behind the
    /// "Time saved" headline.
    static let secondsSavedPerImage: TimeInterval = 30

    func snapshot(categoryLimit: Int = 6, typeLimit: Int = 5) async throws -> InsightsSnapshot {
        try await dbQueue.read { db in
            try Self.build(db, categoryLimit: categoryLimit, typeLimit: typeLimit)
        }
    }

    private static func build(_ db: Database, categoryLimit: Int, typeLimit: Int) throws -> InsightsSnapshot {
        let total = try ImageRecord.fetchCount(db)
        guard total > 0 else { return .empty }

        let successes = try ImageRecord
            .filter(ImageRecord.Columns.isError == false)
            .fetchCount(db)
        let errors = total - successes

        let (thisMonth, lastMonth) = try monthCounts(db)
        let daily = try dailyCounts(db)
        let hourly = try hourlyCounts(db)
        let topCategories = try categories(db, limit: categoryLimit)
        let fileTypes = try fileTypes(db, limit: typeLimit)
        let performance = try performance(db)

        return InsightsSnapshot(
            totalProcessed: total,
            successes: successes,
            errors: errors,
            thisMonth: thisMonth,
            lastMonth: lastMonth,
            daily: daily,
            hourly: hourly,
            topCategories: topCategories,
            fileTypes: fileTypes,
            avgDuration: performance.avg,
            p95Duration: performance.p95,
            fastest: performance.fastest,
            slowest: performance.slowest
        )
    }

    // MARK: - Month over month

    private static func monthCounts(_ db: Database) throws -> (thisMonth: Int, lastMonth: Int) {
        let calendar = Calendar.current
        let now = Date()
        let startOfThisMonth = calendar.date(from: calendar.dateComponents([.year, .month], from: now)) ?? now
        let startOfLastMonth = calendar.date(byAdding: .month, value: -1, to: startOfThisMonth) ?? startOfThisMonth

        let thisMonth = try ImageRecord
            .filter(ImageRecord.Columns.timestamp >= startOfThisMonth)
            .fetchCount(db)
        let lastMonth = try ImageRecord
            .filter(ImageRecord.Columns.timestamp >= startOfLastMonth)
            .filter(ImageRecord.Columns.timestamp < startOfThisMonth)
            .fetchCount(db)
        return (thisMonth, lastMonth)
    }

    // MARK: - Time series

    private static func dailyCounts(_ db: Database) throws -> [Date: Int] {
        let rows = try Row.fetchAll(db, sql: """
        SELECT date(timestamp, 'localtime') AS day, COUNT(*) AS n
        FROM \(ImageRecord.databaseTableName)
        GROUP BY day
        """)

        let calendar = Calendar.current
        let parser = DateFormatter()
        parser.calendar = calendar
        parser.locale = Locale(identifier: "en_US_POSIX")
        parser.timeZone = calendar.timeZone
        parser.dateFormat = "yyyy-MM-dd"

        var daily: [Date: Int] = [:]
        for row in rows {
            guard let dayString: String = row["day"], let date = parser.date(from: dayString) else { continue }
            daily[calendar.startOfDay(for: date)] = row["n"]
        }
        return daily
    }

    private static func hourlyCounts(_ db: Database) throws -> [Int] {
        let rows = try Row.fetchAll(db, sql: """
        SELECT CAST(strftime('%H', timestamp, 'localtime') AS INTEGER) AS hour, COUNT(*) AS n
        FROM \(ImageRecord.databaseTableName)
        GROUP BY hour
        """)

        var hourly = Array(repeating: 0, count: 24)
        for row in rows {
            guard let hour: Int = row["hour"], (0 ..< 24).contains(hour) else { continue }
            hourly[hour] = row["n"]
        }
        return hourly
    }

    // MARK: - Content

    private static func categories(_ db: Database, limit: Int) throws -> [InsightsSnapshot.Labeled] {
        // Tokenize the AI-generated slugs of successful files, reusing the same logic as the
        // Dashboard's "vocabulary" so categories stay consistent across the app.
        let slugs = try String.fetchAll(db, sql: """
        SELECT newName FROM \(ImageRecord.databaseTableName) WHERE isError = 0
        """)
        return Tokenizer.topNounCounts(from: slugs, limit: limit)
            .map { InsightsSnapshot.Labeled(label: $0.token, count: $0.count) }
    }

    private static func fileTypes(_ db: Database, limit: Int) throws -> [InsightsSnapshot.Labeled] {
        let rows = try Row.fetchAll(db, sql: """
        SELECT ext, COUNT(*) AS n
        FROM \(ImageRecord.databaseTableName)
        WHERE isError = 0 AND ext <> ''
        GROUP BY ext
        ORDER BY n DESC, ext ASC
        LIMIT ?
        """, arguments: [limit])
        return rows.compactMap { row in
            guard let ext: String = row["ext"] else { return nil }
            return InsightsSnapshot.Labeled(label: ext, count: row["n"])
        }
    }

    // MARK: - Performance

    private struct Performance {
        var avg: TimeInterval = 0
        var p95: TimeInterval = 0
        var fastest: InsightsSnapshot.DurationRecord?
        var slowest: InsightsSnapshot.DurationRecord?
    }

    private static func performance(_ db: Database) throws -> Performance {
        // Only successes have a meaningful duration; errors record 0.
        let durations = try Double.fetchAll(db, sql: """
        SELECT duration FROM \(ImageRecord.databaseTableName)
        WHERE isError = 0 AND duration > 0
        """)
        guard !durations.isEmpty else { return Performance() }

        let avg = durations.reduce(0, +) / Double(durations.count)

        let sorted = durations.sorted()
        let index = Int(Double(sorted.count - 1) * 0.95 + 0.5)
        let p95 = sorted[min(index, sorted.count - 1)]

        return try Performance(
            avg: avg,
            p95: p95,
            fastest: durationRecord(db, ascending: true),
            slowest: durationRecord(db, ascending: false)
        )
    }

    private static func durationRecord(_ db: Database, ascending: Bool) throws -> InsightsSnapshot.DurationRecord? {
        let order = ascending ? "ASC" : "DESC"
        let row = try Row.fetchOne(db, sql: """
        SELECT newName, duration FROM \(ImageRecord.databaseTableName)
        WHERE isError = 0 AND duration > 0
        ORDER BY duration \(order)
        LIMIT 1
        """)
        guard let row, let name: String = row["newName"] else { return nil }
        return InsightsSnapshot.DurationRecord(name: name, duration: row["duration"])
    }
}

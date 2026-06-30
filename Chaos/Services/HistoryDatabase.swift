import Foundation
import GRDB

/// Durable, full-lifetime store for every processed image, backed by SQLite via GRDB. This
/// replaces the capped `history.json`: nothing is ever evicted, so the Insights page can show
/// true all-time totals and a complete activity heatmap.
///
/// One database queue serves both roles — the writes on the processing hot path (`AppState`)
/// and the read-only aggregations behind Insights (`InsightsRepository`). GRDB's queue is safe
/// to share across threads, which is why this type is `Sendable`.
final class HistoryDatabase: Sendable {
    let dbQueue: DatabaseQueue

    static let defaultURL: URL = {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport
            .appendingPathComponent("chaos")
            .appendingPathComponent("history.sqlite")
    }()

    /// Opens (creating if needed) the database at `url`, runs migrations, and performs the
    /// one-time import of any legacy `history.json`. Pass `importingLegacyJSONFrom: nil` in
    /// tests to skip the import entirely.
    init(
        url: URL = defaultURL,
        importingLegacyJSONFrom legacyURL: URL? = HistoryStore.defaultHistoryURL
    ) throws {
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        dbQueue = try DatabaseQueue(path: url.path)
        try Self.migrator.migrate(dbQueue)
        if let legacyURL {
            try importLegacyJSONIfNeeded(from: legacyURL)
        }
    }

    // MARK: - Schema

    private static let migrator: DatabaseMigrator = {
        var migrator = DatabaseMigrator()
        migrator.registerMigration("v1_createProcessedImages") { db in
            try db.create(table: ImageRecord.databaseTableName) { t in
                t.primaryKey("id", .text)
                t.column("originalName", .text).notNull()
                t.column("newName", .text).notNull()
                t.column("path", .text).notNull()
                t.column("sourcePath", .text).notNull()
                t.column("timestamp", .datetime).notNull()
                t.column("duration", .double).notNull()
                t.column("isError", .boolean).notNull()
                t.column("errorMessage", .text)
                t.column("ext", .text).notNull()
            }
            // The heatmap, streaks, trend, and month-over-month all filter/group on time.
            try db.create(
                index: "idx_processed_images_timestamp",
                on: ImageRecord.databaseTableName,
                columns: ["timestamp"]
            )
        }
        return migrator
    }()

    // MARK: - Writes

    func insert(_ file: RecentFile) async throws {
        let record = ImageRecord(file)
        try await dbQueue.write { db in
            try record.insert(db)
        }
    }

    func delete(id: UUID) async throws {
        let key = id.uuidString
        _ = try await dbQueue.write { db in
            try ImageRecord.deleteOne(db, key: key)
        }
    }

    /// Reflect an in-place rename: update the stored name, path, and cached extension.
    func update(id: UUID, newName: String, path: String) async throws {
        let key = id.uuidString
        try await dbQueue.write { db in
            guard var record = try ImageRecord.fetchOne(db, key: key) else { return }
            record.newName = newName
            record.path = path
            record.ext = ImageRecord.fileExtension(newName: newName, originalName: record.originalName)
            try record.update(db)
        }
    }

    // MARK: - Reads

    /// The most recent `limit` records, newest first — the working set the Dashboard and
    /// Pipeline render. Insights queries the full table directly instead.
    func recentFiles(limit: Int) async throws -> [RecentFile] {
        try await dbQueue.read { db in
            try ImageRecord
                .order(ImageRecord.Columns.timestamp.desc)
                .limit(limit)
                .fetchAll(db)
                .map(\.recentFile)
        }
    }

    // MARK: - One-time migration

    /// Imports a legacy `history.json` into an empty database, then archives the JSON file to
    /// `<name>.migrated` so it is never re-imported and remains as a manual backup. Idempotent:
    /// once the table has rows (or the JSON is gone), this does nothing.
    private func importLegacyJSONIfNeeded(from legacyURL: URL) throws {
        guard FileManager.default.fileExists(atPath: legacyURL.path) else { return }

        let alreadyHasRows = try dbQueue.read { db in
            try ImageRecord.fetchCount(db) > 0
        }
        guard !alreadyHasRows else {
            try? archiveLegacyFile(at: legacyURL)
            return
        }

        let files = HistoryStore(historyURL: legacyURL).load()
        if !files.isEmpty {
            try dbQueue.write { db in
                for file in files {
                    try ImageRecord(file).insert(db)
                }
            }
        }
        try? archiveLegacyFile(at: legacyURL)
    }

    private func archiveLegacyFile(at legacyURL: URL) throws {
        let destination = legacyURL.appendingPathExtension("migrated")
        try? FileManager.default.removeItem(at: destination)
        try FileManager.default.moveItem(at: legacyURL, to: destination)
    }
}

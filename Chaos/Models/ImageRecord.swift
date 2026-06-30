import Foundation
import GRDB

/// The SQLite row for one processed image. This is the *storage* shape; `RecentFile` stays
/// the storage-agnostic domain model the rest of the app works with, so the UI never depends
/// on GRDB.
///
/// Two fields differ from `RecentFile` on purpose, so the Insights page can aggregate with
/// plain SQL instead of decoding the `RecentFile.Result` enum row by row:
/// - `isError` / `errorMessage` flatten the result.
/// - `ext` caches the lowercased file extension for the file-type breakdown.
struct ImageRecord: Codable, FetchableRecord, PersistableRecord, Equatable {
    static let databaseTableName = "processed_images"

    var id: String
    var originalName: String
    var newName: String
    var path: String
    var sourcePath: String
    var timestamp: Date
    var duration: Double
    var isError: Bool
    var errorMessage: String?
    var ext: String

    enum Columns {
        static let id = Column(CodingKeys.id)
        static let timestamp = Column(CodingKeys.timestamp)
        static let duration = Column(CodingKeys.duration)
        static let isError = Column(CodingKeys.isError)
        static let newName = Column(CodingKeys.newName)
        static let ext = Column(CodingKeys.ext)
    }
}

extension ImageRecord {
    init(_ file: RecentFile) {
        id = file.id.uuidString
        originalName = file.originalName
        newName = file.newName
        path = file.path
        sourcePath = file.sourcePath
        timestamp = file.timestamp
        duration = file.duration
        switch file.result {
        case .success:
            isError = false
            errorMessage = nil
        case .error(let message):
            isError = true
            errorMessage = message
        }
        ext = Self.fileExtension(newName: file.newName, originalName: file.originalName)
    }

    /// Back to the domain model. A malformed stored id is replaced with a fresh UUID rather
    /// than dropping the row.
    var recentFile: RecentFile {
        RecentFile(
            id: UUID(uuidString: id) ?? UUID(),
            originalName: originalName,
            newName: newName,
            path: path,
            sourcePath: sourcePath,
            timestamp: timestamp,
            duration: duration,
            result: isError ? .error(errorMessage ?? "Unknown error") : .success
        )
    }

    /// Lowercased file extension, preferring the renamed file and falling back to the original
    /// (error rows have no `newName`). Empty when neither carries an extension.
    static func fileExtension(newName: String, originalName: String) -> String {
        let renamed = (newName as NSString).pathExtension.lowercased()
        if !renamed.isEmpty { return renamed }
        return (originalName as NSString).pathExtension.lowercased()
    }
}

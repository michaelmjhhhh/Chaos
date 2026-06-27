import Foundation

struct HistoryStore: Sendable {
    static let defaultHistoryURL: URL = {
        let appSupport = FileManager.default.urls(
            for: .applicationSupportDirectory,
            in: .userDomainMask
        ).first!
        return appSupport
            .appendingPathComponent("chaos")
            .appendingPathComponent("history.json")
    }()

    let historyURL: URL
    let limit: Int

    init(historyURL: URL = Self.defaultHistoryURL, limit: Int = 500) {
        self.historyURL = historyURL
        self.limit = limit
    }

    func load() -> [RecentFile] {
        guard let data = try? Data(contentsOf: historyURL),
              let files = try? JSONDecoder().decode([RecentFile].self, from: data) else {
            return []
        }
        return Array(files.prefix(limit))
    }

    func save(_ files: [RecentFile]) throws {
        try FileManager.default.createDirectory(
            at: historyURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        // Compact JSON: this is a machine-read cache, not a file people edit, and the
        // write happens after every filed screenshot.
        let data = try JSONEncoder().encode(Array(files.prefix(limit)))
        try data.write(to: historyURL, options: .atomic)
    }
}

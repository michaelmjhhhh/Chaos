import Foundation

struct HistoryStore {
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
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(Array(files.prefix(limit)))
        try data.write(to: historyURL, options: .atomic)
    }
}

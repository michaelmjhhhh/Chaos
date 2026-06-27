import Foundation

struct RecentFile: Identifiable, Codable, Equatable, Sendable {
    let id: UUID
    let originalName: String
    let newName: String
    let path: String
    let sourcePath: String
    let timestamp: Date
    let duration: TimeInterval
    let result: Result

    enum Result: Codable, Equatable, Sendable {
        case success
        case error(String)
    }

    init(
        id: UUID = UUID(),
        originalName: String,
        newName: String,
        path: String,
        sourcePath: String = "",
        timestamp: Date,
        duration: TimeInterval,
        result: Result
    ) {
        self.id = id
        self.originalName = originalName
        self.newName = newName
        self.path = path
        self.sourcePath = sourcePath
        self.timestamp = timestamp
        self.duration = duration
        self.result = result
    }

    var isError: Bool {
        if case .error = result { return true }
        return false
    }

    var resultText: String {
        switch result {
        case .success: "ok"
        case .error(let msg): msg
        }
    }
}

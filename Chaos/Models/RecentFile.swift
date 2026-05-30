import Foundation

struct RecentFile: Identifiable {
    let id = UUID()
    let originalName: String
    let newName: String
    let path: String
    let timestamp: Date
    let duration: TimeInterval
    let result: Result

    enum Result {
        case success
        case error(String)
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

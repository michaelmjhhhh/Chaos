import Foundation

enum ProcessingStage: Equatable {
    case analyzing
    case renaming
    case clipboard
    case success(String)
    case error(String)
}

enum WatcherStatus: Equatable {
    case stopped
    case starting
    case running
    case error(String)
}

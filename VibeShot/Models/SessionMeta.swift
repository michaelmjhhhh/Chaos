import Foundation

struct SessionMeta {
    let sessionNumber: Int
    let startedAt: Date

    init(defaultsKey: String = "vibeshot.sessionNumber",
         defaults: UserDefaults = .standard,
         now: Date = Date()) {
        let previous = defaults.integer(forKey: defaultsKey)
        let next = previous + 1
        defaults.set(next, forKey: defaultsKey)
        self.sessionNumber = next
        self.startedAt = now
    }
}

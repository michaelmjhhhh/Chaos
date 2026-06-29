import Foundation

struct SessionMeta {
    let sessionNumber: Int
    let startedAt: Date

    init(defaultsKey: String = "chaos.sessionNumber",
         defaults: UserDefaults = .standard,
         now: Date = Date())
    {
        let previous = defaults.integer(forKey: defaultsKey)
        let next = previous + 1
        defaults.set(next, forKey: defaultsKey)
        sessionNumber = next
        startedAt = now
    }
}

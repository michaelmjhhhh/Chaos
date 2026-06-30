import Foundation

/// An immutable, computed-all-at-once view of every statistic the Insights page shows.
/// Built off the main actor by `InsightsRepository`, then handed to the view to render. Keeping
/// it a single value type means the page can swap the whole snapshot atomically and never shows
/// a half-updated set of numbers.
struct InsightsSnapshot: Equatable, Sendable {
    var totalProcessed: Int
    var successes: Int
    var errors: Int
    var thisMonth: Int
    var lastMonth: Int

    /// Local start-of-day → images processed that day. Powers the heatmap, streaks, and trend.
    var daily: [Date: Int]
    /// 24 buckets indexed by local hour-of-day.
    var hourly: [Int]

    var topCategories: [Labeled]
    var fileTypes: [Labeled]

    var avgDuration: TimeInterval
    var p95Duration: TimeInterval
    var fastest: DurationRecord?
    var slowest: DurationRecord?

    struct Labeled: Equatable, Sendable, Identifiable {
        var label: String
        var count: Int
        var id: String {
            label
        }
    }

    struct DurationRecord: Equatable, Sendable {
        var name: String
        var duration: TimeInterval
    }

    static let empty = InsightsSnapshot(
        totalProcessed: 0,
        successes: 0,
        errors: 0,
        thisMonth: 0,
        lastMonth: 0,
        daily: [:],
        hourly: Array(repeating: 0, count: 24),
        topCategories: [],
        fileTypes: [],
        avgDuration: 0,
        p95Duration: 0,
        fastest: nil,
        slowest: nil
    )

    var hasData: Bool {
        totalProcessed > 0
    }

    var successRate: Double {
        guard totalProcessed > 0 else { return 0 }
        return Double(successes) / Double(totalProcessed)
    }

    /// Month-over-month change as a fraction (0.41 ⇒ +41%). Nil when last month had no
    /// activity, since there's no meaningful baseline to grow from.
    var monthOverMonth: Double? {
        guard lastMonth > 0 else { return nil }
        return (Double(thisMonth) - Double(lastMonth)) / Double(lastMonth)
    }

    /// Estimated manual effort saved across all successfully filed images.
    func timeSaved(perImage seconds: TimeInterval) -> TimeInterval {
        Double(successes) * seconds
    }

    /// Busiest local hour-of-day, or nil when there's no activity.
    var peakHour: Int? {
        guard let peak = hourly.max(), peak > 0 else { return nil }
        return hourly.firstIndex(of: peak)
    }

    /// Consecutive active days ending today (or yesterday, so a not-yet-active today doesn't
    /// reset a live streak).
    var currentStreak: Int {
        let calendar = Calendar.current
        let activeDays = Set(daily.keys)
        guard !activeDays.isEmpty else { return 0 }

        var day = calendar.startOfDay(for: Date())
        if !activeDays.contains(day) {
            guard let yesterday = calendar.date(byAdding: .day, value: -1, to: day) else { return 0 }
            day = yesterday
        }

        var streak = 0
        while activeDays.contains(day) {
            streak += 1
            guard let previous = calendar.date(byAdding: .day, value: -1, to: day) else { break }
            day = previous
        }
        return streak
    }

    /// Longest run of consecutive active days anywhere in the history.
    var longestStreak: Int {
        let calendar = Calendar.current
        let days = daily.keys.sorted()
        guard let first = days.first else { return 0 }

        var longest = 1
        var run = 1
        var previous = first
        for day in days.dropFirst() {
            if calendar.date(byAdding: .day, value: 1, to: previous) == day {
                run += 1
            } else {
                run = 1
            }
            longest = max(longest, run)
            previous = day
        }
        return longest
    }
}

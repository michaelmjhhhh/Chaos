import SwiftUI

/// The Insights page's signature element: an almanac-style activity calendar. Each square is a
/// day, columns are weeks (oldest on the left, this week on the right), rows are weekdays.
/// Intensity is a stepped coral ramp keyed to each day's volume against the busiest day in view.
struct Heatmap: View {
    /// Local start-of-day → images processed that day.
    let daily: [Date: Int]
    var weeks: Int = 26
    var cell: CGFloat = 13
    var spacing: CGFloat = 3

    private let calendar = Calendar.current

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            grid
            legend
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Activity heatmap")
        .accessibilityValue(accessibilitySummary)
    }

    // MARK: - Grid

    private var grid: some View {
        HStack(alignment: .top, spacing: spacing) {
            weekdayLabels
            VStack(alignment: .leading, spacing: spacing) {
                monthLabels
                HStack(spacing: spacing) {
                    ForEach(0 ..< weeks, id: \.self) { column in
                        VStack(spacing: spacing) {
                            ForEach(0 ..< 7, id: \.self) { row in
                                square(for: date(column: column, row: row))
                            }
                        }
                    }
                }
            }
        }
    }

    private var weekdayLabels: some View {
        VStack(alignment: .trailing, spacing: spacing) {
            // Spacer matching the month-label row so weekdays line up with their squares.
            Text(" ").font(Theme.captionSm).frame(height: 11)
            ForEach(0 ..< 7, id: \.self) { row in
                Text(weekdayLabel(row))
                    .font(.system(size: 9, weight: .medium))
                    .foregroundStyle(Theme.textSoft)
                    .frame(width: 24, height: cell, alignment: .trailing)
            }
        }
    }

    private var monthLabels: some View {
        HStack(spacing: spacing) {
            ForEach(0 ..< weeks, id: \.self) { column in
                Text(monthLabel(column))
                    .font(.system(size: 9, weight: .medium))
                    .foregroundStyle(Theme.textSoft)
                    .frame(width: cell, height: 11, alignment: .leading)
                    .fixedSize()
            }
        }
    }

    private func square(for day: Date?) -> some View {
        RoundedRectangle(cornerRadius: 2.5)
            .fill(fill(for: day))
            .frame(width: cell, height: cell)
            .overlay(
                RoundedRectangle(cornerRadius: 2.5)
                    .stroke(Theme.ink.opacity(day == nil ? 0 : 0.04), lineWidth: 0.5)
            )
    }

    // MARK: - Color ramp

    private func fill(for day: Date?) -> Color {
        guard let day, day <= today else { return .clear }
        let count = daily[day] ?? 0
        guard count > 0 else { return Theme.rule.opacity(0.20) }
        let peak = maxCount
        guard peak > 0 else { return Theme.coral.opacity(0.35) }

        switch Double(count) / Double(peak) {
        case ..<0.25: return Theme.coral.opacity(0.30)
        case ..<0.50: return Theme.coral.opacity(0.50)
        case ..<0.75: return Theme.coral.opacity(0.72)
        default: return Theme.coral
        }
    }

    private var legend: some View {
        HStack(spacing: 5) {
            Text("Less").font(.system(size: 9, weight: .medium)).foregroundStyle(Theme.textSoft)
            ForEach([0.20, 0.30, 0.50, 0.72, 1.0], id: \.self) { level in
                RoundedRectangle(cornerRadius: 2.5)
                    .fill(level == 0.20 ? Theme.rule.opacity(0.20) : Theme.coral.opacity(level))
                    .frame(width: 10, height: 10)
            }
            Text("More").font(.system(size: 9, weight: .medium)).foregroundStyle(Theme.textSoft)
        }
        .accessibilityHidden(true)
    }

    // MARK: - Date math

    private var today: Date {
        calendar.startOfDay(for: Date())
    }

    private var maxCount: Int {
        daily.values.max() ?? 0
    }

    /// Start-of-week containing the first (leftmost) column, aligned to the calendar's first
    /// weekday so rows are consistent.
    private var firstColumnStart: Date {
        let offset = (calendar.component(.weekday, from: today) - calendar.firstWeekday + 7) % 7
        let startOfThisWeek = calendar.date(byAdding: .day, value: -offset, to: today) ?? today
        return calendar.date(byAdding: .day, value: -7 * (weeks - 1), to: startOfThisWeek) ?? startOfThisWeek
    }

    private func date(column: Int, row: Int) -> Date? {
        calendar.date(byAdding: .day, value: column * 7 + row, to: firstColumnStart)
    }

    private func weekdayLabel(_ row: Int) -> String {
        // Sparse labels (Mon / Wed / Fri) to keep the grid uncluttered.
        let weekday = ((calendar.firstWeekday - 1 + row) % 7)
        let symbols = calendar.shortWeekdaySymbols // index 0 == Sunday
        switch weekday {
        case 1, 3, 5: return symbols[weekday]
        default: return ""
        }
    }

    /// Month abbreviation shown only on the first column of each month.
    private func monthLabel(_ column: Int) -> String {
        guard let columnStart = date(column: column, row: 0) else { return "" }
        let month = calendar.component(.month, from: columnStart)
        let previous = column > 0 ? date(column: column - 1, row: 0).map { calendar.component(.month, from: $0) } : nil
        guard month != previous else { return "" }
        return calendar.shortMonthSymbols[month - 1]
    }

    private var accessibilitySummary: String {
        let active = daily.values.count(where: { $0 > 0 })
        guard active > 0 else { return "No activity yet." }
        return "\(active) active days, busiest day \(maxCount) images."
    }
}

#Preview {
    let calendar = Calendar.current
    let today = calendar.startOfDay(for: Date())
    var daily: [Date: Int] = [:]
    for offset in 0 ..< 120 where offset % 3 != 0 {
        if let day = calendar.date(byAdding: .day, value: -offset, to: today) {
            daily[day] = Int.random(in: 1 ... 12)
        }
    }
    return Heatmap(daily: daily)
        .padding(40)
        .background(Theme.canvas)
}

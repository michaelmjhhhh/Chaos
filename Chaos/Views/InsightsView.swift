import SwiftUI

/// The "almanac" page of Chaos: an at-a-glance, all-time view of everything the app has
/// processed. It reads a single `InsightsSnapshot` computed off the main actor and recomputes
/// whenever the history changes (`AppState.historyRevision`).
struct InsightsView: View {
    @Environment(AppState.self) private var appState

    @State private var snapshot: InsightsSnapshot = .empty
    @State private var hasLoaded = false

    var body: some View {
        ZStack {
            PaperBackground()

            VStack(spacing: 0) {
                header

                if snapshot.hasData {
                    ScrollView {
                        VStack(alignment: .leading, spacing: Theme.sSec) {
                            heroLede
                            tierOne
                            activitySection
                            captureSection
                            performanceSection
                        }
                        .padding(.horizontal, Theme.sSec)
                        .padding(.vertical, Theme.sLg)
                    }
                } else {
                    emptyState
                }
            }
        }
        .frame(minWidth: 520, minHeight: 540)
        .task(id: appState.historyRevision) {
            await reload()
        }
    }

    // MARK: - Loading

    private func reload() async {
        guard let repository = appState.insightsRepository else {
            hasLoaded = true
            return
        }
        snapshot = await (try? repository.snapshot()) ?? .empty
        hasLoaded = true
    }

    // MARK: - Header

    private var header: some View {
        VStack(spacing: 0) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.sSmall) {
                Text("Insights")
                    .font(Theme.displaySm)
                    .foregroundStyle(Theme.warmInk)
                Spacer()
                if let since = sinceText {
                    Text(since)
                        .font(Theme.captionSm)
                        .tracking(0.4)
                        .foregroundStyle(Theme.textSoft)
                }
            }
            .padding(.horizontal, Theme.sLg)
            .padding(.vertical, Theme.sSmall)
            .frame(height: 32)

            EditorialRule()
        }
    }

    private var sinceText: String? {
        guard let first = snapshot.daily.keys.min() else { return nil }
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM yyyy"
        return "Lifetime · since \(formatter.string(from: first))"
    }

    // MARK: - Hero lede

    private var heroLede: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.sMed) {
                Text(snapshot.totalProcessed.formatted())
                    .font(.system(size: 64, weight: .regular, design: .serif))
                    .foregroundStyle(Theme.ink)
                    .monospacedDigit()
                    .tracking(-1)
                monthOverMonthBadge
            }
            Text("Images processed")
                .smallCaps()
                .foregroundStyle(Theme.textMuted)
        }
    }

    @ViewBuilder
    private var monthOverMonthBadge: some View {
        if let change = snapshot.monthOverMonth {
            let isUp = change >= 0
            let percent = Int((abs(change) * 100).rounded())
            HStack(spacing: 3) {
                Image(systemName: isUp ? "arrow.up.right" : "arrow.down.right")
                    .font(.system(size: 9, weight: .bold))
                Text("\(percent)% this month")
                    .font(Theme.captionSm)
            }
            .foregroundStyle(isUp ? Theme.teal : Theme.textSoft)
            .padding(.horizontal, Theme.sSmall)
            .padding(.vertical, 4)
            .background((isUp ? Theme.teal : Theme.textSoft).opacity(0.12))
            .clipShape(.rect(cornerRadius: Theme.r6))
        }
    }

    // MARK: - Tier one figures

    private var tierOne: some View {
        HStack(alignment: .top, spacing: Theme.sBreak) {
            MetricFigure(
                value: "\(Int((snapshot.successRate * 100).rounded()))%",
                label: "Success rate"
            )
            MetricFigure(
                value: Format.timeSaved(snapshot.timeSaved(perImage: InsightsRepository.secondsSavedPerImage)),
                label: "Time saved"
            )
            if snapshot.errors > 0 {
                MetricFigure(
                    value: snapshot.errors.formatted(),
                    label: "Errors caught",
                    accent: Theme.error
                )
            }
        }
    }

    // MARK: - Activity

    private var activitySection: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("Activity").sectionHead()
            EditorialRule()

            Heatmap(daily: snapshot.daily)

            HStack(spacing: Theme.sLg) {
                streakStat("\(snapshot.currentStreak)", unit: dayUnit(snapshot.currentStreak), label: "Current streak")
                streakStat("\(snapshot.longestStreak)", unit: dayUnit(snapshot.longestStreak), label: "Longest streak")
                if let peak = snapshot.peakHour {
                    streakStat(Format.hour(peak), unit: "", label: "Peak hour")
                }
            }

            if trendValues.count >= 2 {
                Sparkline(
                    values: trendValues,
                    caption: "Fig. 1 — Volume, last \(trendValues.count) weeks",
                    lastValueText: "\(Int(trendValues.last ?? 0))"
                )
            }
        }
    }

    private func streakStat(_ value: String, unit: String, label: String) -> some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline, spacing: 3) {
                Text(value)
                    .font(.system(size: 22, weight: .regular, design: .serif))
                    .foregroundStyle(Theme.ink)
                    .monospacedDigit()
                if !unit.isEmpty {
                    Text(unit).font(Theme.bodySm).foregroundStyle(Theme.textSoft)
                }
            }
            Text(label).smallCaps().foregroundStyle(Theme.textSoft)
        }
    }

    /// Weekly totals for the trailing weeks, oldest first — the trend sparkline's input.
    private var trendValues: [Double] {
        let calendar = Calendar.current
        let today = calendar.startOfDay(for: Date())
        let weeks = 12
        var totals = Array(repeating: 0.0, count: weeks)
        for (day, count) in snapshot.daily {
            let daysAgo = calendar.dateComponents([.day], from: day, to: today).day ?? 0
            guard daysAgo >= 0 else { continue }
            let weekIndex = weeks - 1 - (daysAgo / 7)
            guard weekIndex >= 0 else { continue }
            totals[weekIndex] += Double(count)
        }
        return totals
    }

    // MARK: - What you capture

    private var captureSection: some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            Text("What you capture").sectionHead()
            EditorialRule()

            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: Theme.sBreak) {
                    categoriesColumn.frame(maxWidth: .infinity, alignment: .leading)
                    fileTypesColumn.frame(width: 240)
                }
                VStack(alignment: .leading, spacing: Theme.sLg) {
                    categoriesColumn
                    fileTypesColumn
                }
            }
        }
    }

    private var categoriesColumn: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("Top categories").marginalia()
            if snapshot.topCategories.isEmpty {
                Text("Not enough named files yet.")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)
            } else {
                BarBreakdown(items: snapshot.topCategories)
            }
        }
    }

    private var fileTypesColumn: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text("File types").marginalia()
            if snapshot.fileTypes.isEmpty {
                Text("No filed images yet.")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)
            } else {
                BarBreakdown(items: snapshot.fileTypes, showsPercentage: true, labelWidth: 48)
            }
        }
    }

    // MARK: - Performance

    @ViewBuilder
    private var performanceSection: some View {
        if snapshot.avgDuration > 0 {
            VStack(alignment: .leading, spacing: Theme.sMed) {
                Text("Performance").sectionHead()
                EditorialRule()

                HStack(spacing: Theme.sBreak) {
                    MetricFigure(value: Format.duration(snapshot.avgDuration), label: "Avg per image")
                    MetricFigure(value: Format.duration(snapshot.p95Duration), label: "95th percentile")
                }

                VStack(alignment: .leading, spacing: Theme.sMicro) {
                    if let fastest = snapshot.fastest {
                        recordLine(symbol: "hare", label: "Fastest", record: fastest)
                    }
                    if let slowest = snapshot.slowest {
                        recordLine(symbol: "tortoise", label: "Slowest", record: slowest)
                    }
                }
            }
        }
    }

    private func recordLine(symbol: String, label: String, record: InsightsSnapshot.DurationRecord) -> some View {
        HStack(spacing: Theme.sSmall) {
            Image(systemName: symbol)
                .font(.system(size: 11))
                .foregroundStyle(Theme.textSoft)
                .frame(width: 16)
            Text(label).smallCaps().foregroundStyle(Theme.textSoft).frame(width: 56, alignment: .leading)
            Text(Format.duration(record.duration))
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textBody)
                .monospacedDigit()
            Text(record.name)
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textMuted)
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(label) \(Format.duration(record.duration)), \(record.name)")
    }

    // MARK: - Empty state

    private var emptyState: some View {
        VStack(spacing: Theme.sMed) {
            Spacer()
            Image(systemName: "chart.bar.xaxis")
                .font(.system(size: 32, weight: .light))
                .foregroundStyle(Theme.textSoft)
            Text("No images filed yet")
                .font(Theme.displaySm)
                .foregroundStyle(Theme.warmInk)
            Text("Insights fills in as Chaos renames and files your screenshots.")
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textMuted)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 280)
            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(Theme.sSec)
    }

    private func dayUnit(_ count: Int) -> String {
        count == 1 ? "day" : "days"
    }
}

/// Small formatting helpers local to the Insights page.
private enum Format {
    static func duration(_ seconds: TimeInterval) -> String {
        if seconds <= 0 { return "—" }
        if seconds < 1 { return String(format: "%.0fms", seconds * 1000) }
        return String(format: "%.1fs", seconds)
    }

    static func timeSaved(_ seconds: TimeInterval) -> String {
        if seconds < 60 { return "\(Int(seconds))s" }
        let minutes = seconds / 60
        if minutes < 60 { return "\(Int(minutes.rounded())) min" }
        let hours = minutes / 60
        if hours < 10 { return String(format: "%.1f hrs", hours) }
        return "\(Int(hours.rounded())) hrs"
    }

    static func hour(_ hour: Int) -> String {
        var components = DateComponents()
        components.hour = hour
        let date = Calendar.current.date(from: components) ?? Date()
        let formatter = DateFormatter()
        formatter.dateFormat = "h a"
        return formatter.string(from: date)
    }
}

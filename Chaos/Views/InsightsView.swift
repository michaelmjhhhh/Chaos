import SwiftUI

/// The "almanac" page of Chaos: an at-a-glance, all-time view of everything the app has
/// processed. Laid out as a tidy card grid — a 3-up row of headline figures, a full-width
/// activity card, then paired analytic panels — mirroring the reference dashboard. It reads a
/// single `InsightsSnapshot` computed off the main actor and recomputes whenever the history
/// changes (`AppState.historyRevision`).
struct InsightsView: View {
    @Environment(AppState.self) private var appState

    @State private var snapshot: InsightsSnapshot = .empty
    @State private var hasLoaded = false

    private let gap = Theme.sMed
    /// Cap the dashboard width and center it, so cards stay a comfortable size on very wide
    /// windows instead of stretching edge to edge.
    private let maxContentWidth: CGFloat = 1040

    var body: some View {
        ZStack {
            PaperBackground()

            VStack(spacing: 0) {
                header

                if snapshot.hasData {
                    ScrollView {
                        VStack(spacing: gap) {
                            statRow
                            activityCard
                            analyticsRow
                        }
                        .padding(Theme.sLg)
                        .frame(maxWidth: maxContentWidth)
                        .frame(maxWidth: .infinity)
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

    // MARK: - Row 1 · headline figures

    private var statRow: some View {
        HStack(alignment: .top, spacing: gap) {
            totalCard
            successCard
            timeSavedCard
        }
        .fixedSize(horizontal: false, vertical: true)
    }

    private var totalCard: some View {
        statCard {
            HStack(alignment: .top) {
                bigNumber(snapshot.totalProcessed.formatted())
                Spacer(minLength: Theme.sSmall)
                monthOverMonthBadge
            }
            Text("Images processed").cardLabel()
        }
    }

    private var successCard: some View {
        statCard {
            bigNumber("\(Int((snapshot.successRate * 100).rounded()))%")
            Text("Success rate").cardLabel()
            cardDivider
            VStack(alignment: .leading, spacing: 5) {
                subStat(snapshot.successes.formatted(), "filed cleanly")
                subStat(snapshot.errors.formatted(), "errors caught", accent: snapshot.errors > 0 ? Theme.error : nil)
            }
        }
    }

    private var timeSavedCard: some View {
        statCard {
            bigNumber(Format.timeSaved(snapshot.timeSaved(perImage: InsightsRepository.secondsSavedPerImage)))
            Text("Time saved").cardLabel()
            cardDivider
            Text("≈ \(Int(InsightsRepository.secondsSavedPerImage))s of manual filing saved per image")
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textSoft)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    // MARK: - Row 2 · activity

    private var activityCard: some View {
        card {
            HStack(alignment: .firstTextBaseline) {
                Text(streakTitle)
                    .font(Theme.displayMd)
                    .foregroundStyle(Theme.warmInk)
                Spacer()
                Text("Longest · \(snapshot.longestStreak) \(dayUnit(snapshot.longestStreak))")
                    .cardLabel()
            }

            ScrollView(.horizontal, showsIndicators: false) {
                Heatmap(daily: snapshot.daily)
            }

            if let peak = snapshot.peakHour {
                Text("Most active around \(Format.hour(peak))")
                    .font(Theme.bodySm)
                    .foregroundStyle(Theme.textSoft)
            }
        }
    }

    private var streakTitle: String {
        let streak = snapshot.currentStreak
        guard streak > 0 else { return "Activity" }
        return "\(streak) \(dayUnit(streak)) streak"
    }

    // MARK: - Row 3 · analytic panels

    private var analyticsRow: some View {
        HStack(alignment: .top, spacing: gap) {
            captureCard
            performanceCard
        }
        .fixedSize(horizontal: false, vertical: true)
    }

    private var captureCard: some View {
        card(fill: true) {
            Text("What you capture")
                .font(Theme.displayMd)
                .foregroundStyle(Theme.warmInk)

            Text("Top categories").cardLabel()
            if snapshot.topCategories.isEmpty {
                placeholder("Not enough named files yet.")
            } else {
                BarBreakdown(items: snapshot.topCategories)
            }

            cardDivider

            Text("File types").cardLabel()
            if snapshot.fileTypes.isEmpty {
                placeholder("No filed images yet.")
            } else {
                BarBreakdown(items: snapshot.fileTypes, showsPercentage: true, labelWidth: 48)
            }
        }
    }

    private var performanceCard: some View {
        card(fill: true) {
            Text("Performance")
                .font(Theme.displayMd)
                .foregroundStyle(Theme.warmInk)

            if snapshot.avgDuration > 0 {
                HStack(alignment: .top, spacing: Theme.sLg) {
                    MetricFigure(value: Format.duration(snapshot.avgDuration), label: "Avg per image")
                    MetricFigure(value: Format.duration(snapshot.p95Duration), label: "95th percentile")
                }

                cardDivider

                VStack(alignment: .leading, spacing: Theme.sSmall) {
                    if let fastest = snapshot.fastest {
                        recordLine(symbol: "hare", label: "Fastest", record: fastest)
                    }
                    if let slowest = snapshot.slowest {
                        recordLine(symbol: "tortoise", label: "Slowest", record: slowest)
                    }
                }
            } else {
                placeholder("No timing data yet.")
            }

            if trendValues.count >= 2 {
                cardDivider
                Sparkline(
                    values: trendValues,
                    caption: "Volume · last \(trendValues.count) weeks",
                    lastValueText: "\(Int(trendValues.last ?? 0))"
                )
            }
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

    // MARK: - Building blocks

    /// A headline-figure card. `fill: true` lets it stretch to the tallest card in its row, so a
    /// row of cards shares one consistent height (the row bounds the height with
    /// `.fixedSize(vertical:)`).
    private func statCard(@ViewBuilder _ content: () -> some View) -> some View {
        card(fill: true, content: content)
    }

    private func card(
        fill: Bool = false,
        @ViewBuilder content: () -> some View
    ) -> some View {
        VStack(alignment: .leading, spacing: Theme.sMed) {
            content()
        }
        .frame(maxWidth: .infinity, maxHeight: fill ? .infinity : nil, alignment: .topLeading)
        .padding(Theme.sLg)
        .background(Theme.surfaceCard)
        .clipShape(RoundedRectangle(cornerRadius: Theme.r12))
        .overlay(
            RoundedRectangle(cornerRadius: Theme.r12)
                .stroke(Theme.border, lineWidth: 0.5)
        )
        .shadow(color: Theme.shadowCard, radius: 5, y: 2)
    }

    private func bigNumber(_ text: String, accent: Color? = nil) -> some View {
        Text(text)
            .font(.system(size: 40, weight: .regular, design: .serif))
            .foregroundStyle(accent ?? Theme.ink)
            .monospacedDigit()
            .tracking(-0.5)
            .lineLimit(1)
            .minimumScaleFactor(0.6)
    }

    private func subStat(_ value: String, _ label: String, accent: Color? = nil) -> some View {
        HStack(spacing: 6) {
            Text(value)
                .font(Theme.titleSm)
                .foregroundStyle(accent ?? Theme.ink)
                .monospacedDigit()
            Text(label)
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textMuted)
            Spacer(minLength: 0)
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
            .fixedSize()
        }
    }

    private var cardDivider: some View {
        Rectangle()
            .fill(Theme.divider)
            .frame(height: 0.5)
    }

    private func placeholder(_ text: String) -> some View {
        Text(text)
            .font(Theme.bodySm)
            .foregroundStyle(Theme.textSoft)
    }

    private func recordLine(symbol: String, label: String, record: InsightsSnapshot.DurationRecord) -> some View {
        HStack(spacing: Theme.sSmall) {
            Image(systemName: symbol)
                .font(.system(size: 11))
                .foregroundStyle(Theme.textSoft)
                .frame(width: 16)
            Text(label)
                .smallCaps()
                .foregroundStyle(Theme.textSoft)
                .lineLimit(1)
                .fixedSize()
                .frame(width: 64, alignment: .leading)
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

/// Card section label — the small uppercase gray caption used throughout the reference's cards.
private extension View {
    func cardLabel() -> some View {
        font(Theme.captionSm)
            .textCase(.uppercase)
            .tracking(0.8)
            .foregroundStyle(Theme.textMuted)
    }
}

import SwiftUI

/// Ranked horizontal bars for a small set of labeled counts — used for "Top categories" and the
/// file-type split. Coral fills on paper with the value at the end: the editorial reading of the
/// reference's "usage" chart.
struct BarBreakdown: View {
    let items: [InsightsSnapshot.Labeled]
    /// Show each bar's share of the total as a percentage instead of the raw count.
    var showsPercentage = false
    var labelWidth: CGFloat = 84

    private var maxCount: Int {
        items.map(\.count).max() ?? 0
    }

    private var total: Int {
        items.reduce(0) { $0 + $1.count }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            ForEach(items) { item in
                row(item)
            }
        }
    }

    private func row(_ item: InsightsSnapshot.Labeled) -> some View {
        HStack(spacing: Theme.sSmall) {
            Text(item.label)
                .font(Theme.bodySm)
                .foregroundStyle(Theme.textBody)
                .lineLimit(1)
                .truncationMode(.tail)
                .frame(width: labelWidth, alignment: .leading)

            GeometryReader { geo in
                let fraction = maxCount > 0 ? Double(item.count) / Double(maxCount) : 0
                ZStack(alignment: .leading) {
                    RoundedRectangle(cornerRadius: Theme.r4)
                        .fill(Theme.rule.opacity(0.18))
                    RoundedRectangle(cornerRadius: Theme.r4)
                        .fill(Theme.coral.opacity(0.85))
                        .frame(width: max(2, geo.size.width * fraction))
                }
            }
            .frame(height: 14)

            Text(valueText(item))
                .font(Theme.codeSm)
                .foregroundStyle(Theme.textMuted)
                .monospacedDigit()
                .frame(width: 46, alignment: .trailing)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(item.label), \(valueText(item))")
    }

    private func valueText(_ item: InsightsSnapshot.Labeled) -> String {
        if showsPercentage, total > 0 {
            let percent = Int((Double(item.count) / Double(total) * 100).rounded())
            return "\(percent)%"
        }
        return item.count.formatted()
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 24) {
        BarBreakdown(items: [
            .init(label: "terminal", count: 412),
            .init(label: "browser", count: 288),
            .init(label: "design", count: 140),
            .init(label: "error", count: 60)
        ])
        BarBreakdown(items: [
            .init(label: "png", count: 710),
            .init(label: "jpg", count: 220),
            .init(label: "heic", count: 70)
        ], showsPercentage: true)
    }
    .padding(40)
    .frame(width: 360)
    .background(Theme.canvas)
}

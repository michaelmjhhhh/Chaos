import SwiftUI

struct Masthead: View {
    var status: WatcherStatus? = nil
    let date: Date

    private var dateText: String {
        let f = DateFormatter()
        f.dateFormat = "EEE, d MMM"
        return f.string(from: date)
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.sSmall) {
                Text("Chaos")
                    .font(Theme.displaySm)
                    .foregroundStyle(Theme.warmInk)

                if let status {
                    statusPill(status)
                }

                Spacer()

                Text(dateText)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.textSoft)
            }
            .padding(.horizontal, Theme.sLg)
            .padding(.vertical, Theme.sSmall)
            .frame(height: 32)

            EditorialRule()
        }
    }

    @ViewBuilder
    private func statusPill(_ status: WatcherStatus) -> some View {
        HStack(spacing: 5) {
            Circle()
                .fill(dotColor(status))
                .frame(width: 5, height: 5)
            Text(label(status))
                .font(Theme.captionSm)
                .tracking(0.6)
                .foregroundStyle(Theme.textMuted)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel("Status: \(label(status))")
    }

    private func dotColor(_ status: WatcherStatus) -> Color {
        switch status {
        case .running: Theme.success
        case .starting: Theme.warning
        case .error: Theme.error
        case .stopped: Theme.textSoft.opacity(0.5)
        }
    }

    private func label(_ status: WatcherStatus) -> String {
        switch status {
        case .running: "Watching"
        case .starting: "Starting"
        case .error: "Needs attention"
        case .stopped: "Paused"
        }
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 0) {
        Masthead(status: .running, date: Date())
        Spacer()
    }
    .frame(width: 760, height: 540)
    .background(Theme.canvas)
}

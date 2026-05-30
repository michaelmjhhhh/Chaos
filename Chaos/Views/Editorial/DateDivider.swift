import SwiftUI

struct DateDivider: View {
    let label: String

    var body: some View {
        HStack(spacing: Theme.sSmall) {
            Text(label)
                .smallCaps()
                .foregroundStyle(Theme.textMuted)
            Rectangle()
                .fill(Theme.rule)
                .frame(height: 0.5)
        }
        .padding(.vertical, Theme.sSmall)
    }
}

#Preview {
    VStack(spacing: 8) {
        DateDivider(label: "TODAY · 30 MAY")
        DateDivider(label: "YESTERDAY · 29 MAY")
        DateDivider(label: "EARLIER · 28 MAY AND BEFORE")
    }
    .padding(40)
    .background(Theme.canvas)
}

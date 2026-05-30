import SwiftUI

struct Masthead: View {
    let sessionNumber: Int
    let date: Date

    private var dateText: String {
        let f = DateFormatter()
        f.dateFormat = "EEE, d MMM"
        return f.string(from: date)
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                segment("CHAOS")
                separator
                segment("DAILY EDITION")
                separator
                segment(dateText.uppercased())
                separator
                segment("NO. \(sessionNumber)")
                Spacer()
            }
            .padding(.horizontal, 24)
            .padding(.vertical, 8)
            .frame(height: 28)

            EditorialRule()
        }
    }

    @ViewBuilder
    private func segment(_ text: String) -> some View {
        Text(text)
            .font(Theme.smallCapsSm)
            .tracking(1.2)
            .foregroundStyle(Theme.ink)
    }

    @ViewBuilder
    private var separator: some View {
        Text(" · ")
            .font(Theme.smallCapsSm)
            .foregroundStyle(Theme.borderLight)
            .padding(.horizontal, 8)
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 0) {
        Masthead(sessionNumber: 14, date: Date())
        Spacer()
    }
    .frame(width: 760, height: 540)
    .background(Theme.canvas)
}

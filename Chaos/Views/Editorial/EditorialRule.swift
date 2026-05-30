import SwiftUI

struct EditorialRule: View {
    enum Ornament: String {
        case none = ""
        case section = "§"
        case asterism = "⁂"
        case dot = "·"
    }

    var ornament: Ornament = .none
    var color: Color = Theme.rule

    var body: some View {
        HStack(spacing: Theme.sSmall) {
            Rectangle()
                .fill(color)
                .frame(height: 0.5)
            if ornament != .none {
                Text(ornament.rawValue)
                    .font(.system(size: 11, design: .serif))
                    .foregroundStyle(color)
                Rectangle()
                    .fill(color)
                    .frame(height: 0.5)
            }
        }
    }
}

#Preview {
    VStack(spacing: 24) {
        EditorialRule()
        EditorialRule(ornament: .dot)
        EditorialRule(ornament: .section)
        EditorialRule(ornament: .asterism)
    }
    .padding(40)
    .background(Theme.canvas)
}

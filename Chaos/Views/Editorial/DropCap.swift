import SwiftUI

struct DropCap: View {
    let text: String
    var baseFont: Font = Theme.body
    var capSize: CGFloat = 32

    private var firstChar: String {
        guard let c = text.first else { return "" }
        return String(c)
    }

    private var rest: String {
        guard !text.isEmpty else { return "" }
        return String(text.dropFirst())
    }

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 2) {
            Text(firstChar)
                .font(.system(size: capSize, weight: .regular, design: .serif))
                .foregroundStyle(Theme.ink)
                .baselineOffset(-2)
            Text(rest)
                .font(baseFont)
                .foregroundStyle(Theme.ink)
        }
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 20) {
        DropCap(text: "TODAY'S READING")
        DropCap(text: "NUMBERS")
        DropCap(text: "DIRECTORIES")
    }
    .padding(40)
    .background(Theme.canvas)
}

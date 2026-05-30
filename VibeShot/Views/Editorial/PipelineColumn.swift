import SwiftUI

struct PipelineColumn<Content: View>: View {
    let title: String
    let isActive: Bool
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            content()
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        }
    }

    @ViewBuilder
    private var header: some View {
        VStack(alignment: .leading, spacing: Theme.sSmall) {
            Text(title)
                .smallCaps()
                .foregroundStyle(Theme.ink)
            Rectangle()
                .fill(isActive ? Theme.coral : Theme.rule)
                .frame(height: 0.5)
                .animation(.easeInOut(duration: 0.2), value: isActive)
        }
        .padding(.bottom, Theme.sSmall)
    }
}

struct EmptyColumnDash: View {
    var body: some View {
        Text("—")
            .font(.system(size: 18, design: .serif))
            .foregroundStyle(Theme.ink.opacity(0.3))
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

#Preview {
    HStack(alignment: .top, spacing: 16) {
        PipelineColumn(title: "CAUGHT", isActive: false) { EmptyColumnDash() }
        PipelineColumn(title: "READING", isActive: true) { EmptyColumnDash() }
        PipelineColumn(title: "SETTING", isActive: false) { EmptyColumnDash() }
    }
    .frame(width: 460, height: 240)
    .padding(20)
    .background(Theme.canvas)
}

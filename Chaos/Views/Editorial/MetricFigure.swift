import SwiftUI

struct MetricFigure: View {
    let value: String
    let label: String
    var accent: Color?

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            Text(value)
                .font(.system(size: 22, weight: .regular, design: .serif))
                .foregroundStyle(accent ?? Theme.ink)
                .monospacedDigit()
                .tracking(-0.3)
            Text(label)
                .smallCaps()
                .foregroundStyle(Theme.textSoft)
        }
    }
}

#Preview {
    HStack(spacing: 32) {
        MetricFigure(value: "47", label: "Processed")
        MetricFigure(value: "45", label: "Successful")
        MetricFigure(value: "2", label: "Errors", accent: Theme.error)
    }
    .padding(40)
    .background(Theme.canvas)
}

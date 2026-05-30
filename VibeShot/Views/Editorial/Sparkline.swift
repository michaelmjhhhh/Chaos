import SwiftUI

struct Sparkline: View {
    let values: [Double]
    let caption: String
    var lastValueText: String? = nil
    var height: CGFloat = 28

    private var minValue: Double { values.min() ?? 0 }
    private var maxValue: Double { values.max() ?? 1 }
    private var range: Double {
        let r = maxValue - minValue
        return r > 0 ? r : 1
    }

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.sMicro) {
            HStack(alignment: .firstTextBaseline) {
                Text(caption)
                    .font(Theme.serifItalicSm)
                    .foregroundStyle(Theme.textSoft)
                Spacer(minLength: Theme.sMed)
                if let last = lastValueText {
                    Text(last)
                        .font(Theme.codeSm)
                        .foregroundStyle(Theme.textMuted)
                }
            }

            chart
                .frame(height: height)
        }
    }

    @ViewBuilder
    private var chart: some View {
        Canvas { ctx, size in
            let allSame = maxValue == minValue
            guard values.count >= 2, !allSame else {
                drawFlat(ctx: ctx, size: size)
                return
            }

            let stepX = size.width / CGFloat(values.count - 1)
            let pointFor: (Int) -> CGPoint = { i in
                let normalized = (values[i] - minValue) / range
                let x = CGFloat(i) * stepX
                let y = size.height - CGFloat(normalized) * size.height
                return CGPoint(x: x, y: y)
            }

            var fill = Path()
            fill.move(to: CGPoint(x: 0, y: size.height))
            for i in 0..<values.count {
                fill.addLine(to: pointFor(i))
            }
            fill.addLine(to: CGPoint(x: size.width, y: size.height))
            fill.closeSubpath()
            ctx.fill(fill, with: .color(Theme.ink.opacity(0.05)))

            var line = Path()
            line.move(to: pointFor(0))
            for i in 1..<values.count {
                line.addLine(to: pointFor(i))
            }
            ctx.stroke(line, with: .color(Theme.ink), lineWidth: 1)
        }
    }

    private func drawFlat(ctx: GraphicsContext, size: CGSize) {
        var p = Path()
        p.move(to: CGPoint(x: 0, y: size.height / 2))
        p.addLine(to: CGPoint(x: size.width, y: size.height / 2))
        ctx.stroke(p, with: .color(Theme.textSoft.opacity(0.4)), style: StrokeStyle(lineWidth: 0.5, dash: [2, 3]))
    }
}

#Preview {
    VStack(alignment: .leading, spacing: 16) {
        Sparkline(values: [1.2, 1.8, 1.4, 2.1, 2.6, 1.9, 1.7, 2.2, 2.8, 1.5],
                  caption: "Fig. 1 — Latency, last 10 captures",
                  lastValueText: "1.5s")
        Sparkline(values: [3, 4, 5, 7, 6, 9, 11, 8, 6, 5, 4],
                  caption: "Fig. 2 — Throughput, hourly",
                  lastValueText: "4")
        Sparkline(values: [], caption: "Fig. 3 — No data yet")
    }
    .padding(40)
    .background(Theme.canvas)
    .frame(width: 380)
}

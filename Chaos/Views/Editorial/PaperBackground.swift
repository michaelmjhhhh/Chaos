import SwiftUI

struct PaperBackground: View {
    var grainOpacity: Double = 0.025

    var body: some View {
        ZStack {
            Theme.canvas
            Canvas { context, size in
                let cellSize: CGFloat = 3
                let columns = Int(size.width / cellSize) + 1
                let rows = Int(size.height / cellSize) + 1
                var rng = SeededRandom(seed: 0xA1B2C3)

                for x in 0..<columns {
                    for y in 0..<rows {
                        let n = rng.nextDouble()
                        guard n > 0.88 else { continue }
                        let opacity = (n - 0.88) * 0.5
                        let rect = CGRect(
                            x: CGFloat(x) * cellSize,
                            y: CGFloat(y) * cellSize,
                            width: cellSize,
                            height: cellSize
                        )
                        context.fill(
                            Path(rect),
                            with: .color(Theme.ink.opacity(opacity * grainOpacity * 3))
                        )
                    }
                }
            }
            .allowsHitTesting(false)
        }
        .ignoresSafeArea()
    }
}

private struct SeededRandom {
    private var state: UInt64

    init(seed: UInt64) {
        self.state = seed
    }

    mutating func nextDouble() -> Double {
        state = state &* 6364136223846793005 &+ 1442695040888963407
        return Double(state >> 11) / Double(UInt64(1) << 53)
    }
}

#Preview {
    PaperBackground()
        .frame(width: 400, height: 300)
}

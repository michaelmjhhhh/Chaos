import SwiftUI

enum EditorialIcon {
    struct Eye: View {
        var size: CGFloat = 14
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                var outer = Path()
                outer.move(to: CGPoint(x: 0, y: h / 2))
                outer.addQuadCurve(to: CGPoint(x: w, y: h / 2), control: CGPoint(x: w / 2, y: 0))
                outer.addQuadCurve(to: CGPoint(x: 0, y: h / 2), control: CGPoint(x: w / 2, y: h))
                ctx.stroke(outer, with: .color(color), lineWidth: 0.5)

                let pupilRadius: CGFloat = h * 0.18
                let pupil = Path(ellipseIn: CGRect(
                    x: w / 2 - pupilRadius,
                    y: h / 2 - pupilRadius,
                    width: pupilRadius * 2,
                    height: pupilRadius * 2
                ))
                ctx.fill(pupil, with: .color(color))
            }
            .frame(width: size * 1.4, height: size * 0.8)
        }
    }

    struct TrayArrow: View {
        var size: CGFloat = 14
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                var arrow = Path()
                arrow.move(to: CGPoint(x: w / 2, y: 0))
                arrow.addLine(to: CGPoint(x: w / 2, y: h * 0.65))
                arrow.move(to: CGPoint(x: w * 0.3, y: h * 0.45))
                arrow.addLine(to: CGPoint(x: w / 2, y: h * 0.65))
                arrow.addLine(to: CGPoint(x: w * 0.7, y: h * 0.45))
                ctx.stroke(arrow, with: .color(color), lineWidth: 0.5)

                var tray = Path()
                tray.move(to: CGPoint(x: 0, y: h * 0.8))
                tray.addLine(to: CGPoint(x: w, y: h * 0.8))
                ctx.stroke(tray, with: .color(color), lineWidth: 0.5)
            }
            .frame(width: size, height: size)
        }
    }

    struct Shutter: View {
        var size: CGFloat = 80
        var color: Color = Theme.textSoft

        var body: some View {
            Canvas { ctx, canvasSize in
                let w = canvasSize.width, h = canvasSize.height
                let cx = w / 2, cy = h / 2
                let radius = min(w, h) / 2 - 2
                let sides = 8

                var outer = Path()
                for i in 0 ..< sides {
                    let angle = (Double(i) / Double(sides)) * 2 * .pi - .pi / 2
                    let pt = CGPoint(
                        x: cx + radius * CGFloat(cos(angle)),
                        y: cy + radius * CGFloat(sin(angle))
                    )
                    if i == 0 { outer.move(to: pt) } else { outer.addLine(to: pt) }
                }
                outer.closeSubpath()
                ctx.stroke(outer, with: .color(color), lineWidth: 0.5)

                for i in 0 ..< sides {
                    let angle = (Double(i) / Double(sides)) * 2 * .pi - .pi / 2
                    let outerPt = CGPoint(
                        x: cx + radius * CGFloat(cos(angle)),
                        y: cy + radius * CGFloat(sin(angle))
                    )
                    let inner = CGPoint(x: cx, y: cy)
                    var blade = Path()
                    blade.move(to: outerPt)
                    blade.addLine(to: inner)
                    ctx.stroke(blade, with: .color(color.opacity(0.35)), lineWidth: 0.5)
                }
            }
            .frame(width: size, height: size)
        }
    }
}

#Preview {
    HStack(spacing: 40) {
        EditorialIcon.Eye(size: 18)
        EditorialIcon.TrayArrow(size: 18)
        EditorialIcon.Shutter(size: 80)
    }
    .padding(40)
    .background(Theme.canvas)
}

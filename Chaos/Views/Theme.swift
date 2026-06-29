import SwiftUI
import AppKit

enum Theme {
    // Colors are defined as light/dark pairs so the whole app adapts to the
    // system appearance automatically — call sites stay unchanged.

    // MARK: - Brand
    static let coral = Color(light: 0xCC785C, dark: 0xD98E6B)
    static let coralHover = Color(light: 0xB8694F, dark: 0xE49C79)
    static let coralActive = Color(light: 0xA9583E, dark: 0xF0A985)
    /// Text/glyphs that sit on top of the coral brand fill (both appearances).
    static let onBrand = Color(light: 0xFFFFFF, dark: 0xFFFFFF)

    // MARK: - Surfaces
    static let canvas = Color(light: 0xFAF9F5, dark: 0x1A1918)
    static let surfaceCard = Color(light: 0xFFFFFF, dark: 0x262523)
    static let surfaceMuted = Color(light: 0xF5F0E8, dark: 0x201F1D)
    static let surfaceDark = Color(light: 0x1A1918, dark: 0x121110)
    static let surfaceDarkElevated = Color(light: 0x262523, dark: 0x2E2C2A)

    // MARK: - Text
    static let ink = Color(light: 0x141413, dark: 0xF2F0EA)
    static let textBody = Color(light: 0x3D3D3A, dark: 0xCFCBC2)
    static let textMuted = Color(light: 0x6C6A64, dark: 0x9C9890)
    static let textSoft = Color(light: 0x8E8B82, dark: 0x7C7972)
    static let textOnDark = Color(light: 0xFAF9F5, dark: 0xFAF9F5)

    // MARK: - Semantic
    static let success = Color(light: 0x4DA664, dark: 0x5FBE79)
    static let warning = Color(light: 0xD4A017, dark: 0xE0B53A)
    static let error = Color(light: 0xC64545, dark: 0xE06B6B)
    static let teal = Color(light: 0x5DB8A6, dark: 0x6FCDBA)

    // MARK: - Borders & Lines
    static let border = Color(light: 0xE2DDD4, dark: 0x3A3833)
    static let borderLight = Color(light: 0xEBE6DF, dark: 0x322F2B)
    static let divider = Color(light: 0xE6DFD8, dark: 0x35332E)

    // MARK: - Display (Serif — weight 400, negative tracking)
    static let displayXL = Font.system(size: 32, weight: .regular, design: .serif)
    static let displayLg = Font.system(size: 24, weight: .regular, design: .serif)
    static let displayMd = Font.system(size: 18, weight: .regular, design: .serif)
    static let displaySm = Font.system(size: 15, weight: .regular, design: .serif)

    // MARK: - Body (Sans)
    static let titleMd = Font.system(size: 14, weight: .medium)
    static let titleSm = Font.system(size: 13, weight: .medium)
    static let body = Font.system(size: 13, weight: .regular)
    static let bodySm = Font.system(size: 12, weight: .regular)
    static let caption = Font.system(size: 11, weight: .medium)
    static let captionSm = Font.system(size: 10, weight: .medium)
    static let button = Font.system(size: 13, weight: .medium)

    // MARK: - Mono
    static let code = Font.system(size: 12, weight: .regular, design: .monospaced)
    static let codeSm = Font.system(size: 11, weight: .regular, design: .monospaced)

    // MARK: - Radii
    static let r4: CGFloat = 4
    static let r6: CGFloat = 6
    static let r8: CGFloat = 8
    static let r10: CGFloat = 10
    static let r12: CGFloat = 12

    // MARK: - Shadows
    // Shadows need more weight on dark surfaces to remain visible.
    static let shadowCard = Color(lightBlack: 0.04, darkBlack: 0.45)
    static let shadowMd = Color(lightBlack: 0.06, darkBlack: 0.55)

    // MARK: - Editorial additions

    static let warmInk = Color(light: 0x1F1E1B, dark: 0xEDEAE3)
    static let rule = Color(light: 0xD5CFC4, dark: 0x3F3C36)
    /// Subtle zone tint on cards: darkens slightly in light mode, lightens in dark.
    static let paperTint = Color(lightOverlay: (white: 0, alpha: 0.06),
                                 darkOverlay: (white: 1, alpha: 0.05))

    // MARK: - Editorial type

    static let displayHero = Font.system(size: 32, weight: .regular, design: .serif)
    static let serifItalicLg = Font.system(size: 24, weight: .regular, design: .serif).italic()
    static let serifItalicSm = Font.system(size: 11, weight: .regular, design: .serif).italic()
    static let smallCapsSm = Font.system(size: 10, weight: .medium)

    // MARK: - Editorial spacing

    static let sMicro: CGFloat = 4
    static let sSmall: CGFloat = 8
    static let sMed: CGFloat = 16
    static let sLg: CGFloat = 24
    static let sSec: CGFloat = 32
    static let sBreak: CGFloat = 48
}

// MARK: - Card modifier with border + subtle shadow

struct CardModifier: ViewModifier {
    var padding: CGFloat = 16

    func body(content: Content) -> some View {
        content
            .padding(padding)
            .background(Theme.surfaceCard)
            .clipShape(.rect(cornerRadius: Theme.r10))
            .overlay(
                RoundedRectangle(cornerRadius: Theme.r10)
                    .stroke(Theme.border, lineWidth: 0.5)
            )
            .shadow(color: Theme.shadowCard, radius: 4, y: 2)
    }
}

struct SectionHeader: ViewModifier {
    func body(content: Content) -> some View {
        content
            .font(Theme.captionSm)
            .foregroundStyle(Theme.textSoft)
            .textCase(.uppercase)
            .tracking(1.2)
    }
}

struct SmallCaps: ViewModifier {
    func body(content: Content) -> some View {
        content
            .font(Theme.smallCapsSm)
            .tracking(1.2)
            .textCase(.uppercase)
    }
}

struct ClippingCard: ViewModifier {
    var padding: CGFloat = 10

    func body(content: Content) -> some View {
        VStack(spacing: 0) {
            Rectangle()
                .fill(Theme.rule)
                .frame(height: 0.5)
            content
                .padding(padding)
        }
        .background(Theme.surfaceCard)
    }
}

struct Marginalia: ViewModifier {
    func body(content: Content) -> some View {
        content
            .font(Theme.serifItalicSm)
            .foregroundStyle(Theme.textSoft)
    }
}

extension View {
    func card(padding: CGFloat = 16) -> some View {
        modifier(CardModifier(padding: padding))
    }
    func sectionHead() -> some View {
        modifier(SectionHeader())
    }
    func smallCaps() -> some View {
        modifier(SmallCaps())
    }
    func clipping(padding: CGFloat = 10) -> some View {
        modifier(ClippingCard(padding: padding))
    }
    func marginalia() -> some View {
        modifier(Marginalia())
    }
}

extension Color {
    init(hex: UInt32) {
        self.init(
            red: Double((hex >> 16) & 0xFF) / 255.0,
            green: Double((hex >> 8) & 0xFF) / 255.0,
            blue: Double(hex & 0xFF) / 255.0
        )
    }

    /// A color that resolves to `light` in light appearance and `dark` in dark
    /// appearance. Backed by a dynamic NSColor so it updates live when the user
    /// switches the system theme, without threading `colorScheme` through views.
    init(light: UInt32, dark: UInt32) {
        self.init(nsColor: NSColor(name: nil) { appearance in
            appearance.isDark ? NSColor(hex: dark) : NSColor(hex: light)
        })
    }

    /// Adaptive black overlay (e.g. shadows) with per-appearance opacity.
    init(lightBlack: Double, darkBlack: Double) {
        self.init(nsColor: NSColor(name: nil) { appearance in
            NSColor(white: 0, alpha: appearance.isDark ? darkBlack : lightBlack)
        })
    }

    /// Adaptive grayscale overlay (white + alpha) that differs by appearance.
    init(lightOverlay: (white: CGFloat, alpha: CGFloat),
         darkOverlay: (white: CGFloat, alpha: CGFloat)) {
        self.init(nsColor: NSColor(name: nil) { appearance in
            let o = appearance.isDark ? darkOverlay : lightOverlay
            return NSColor(white: o.white, alpha: o.alpha)
        })
    }
}

extension NSColor {
    convenience init(hex: UInt32) {
        self.init(
            srgbRed: Double((hex >> 16) & 0xFF) / 255.0,
            green: Double((hex >> 8) & 0xFF) / 255.0,
            blue: Double(hex & 0xFF) / 255.0,
            alpha: 1
        )
    }
}

extension NSAppearance {
    /// Whether this appearance is one of the dark (Dark Aqua) variants.
    var isDark: Bool {
        bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
    }
}

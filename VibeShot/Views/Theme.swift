import SwiftUI

enum Theme {
    // MARK: - Brand
    static let coral = Color(hex: 0xCC785C)
    static let coralHover = Color(hex: 0xB8694F)
    static let coralActive = Color(hex: 0xA9583E)

    // MARK: - Surfaces
    static let canvas = Color(hex: 0xFAF9F5)
    static let surfaceCard = Color(hex: 0xFFFFFF)
    static let surfaceMuted = Color(hex: 0xF5F0E8)
    static let surfaceDark = Color(hex: 0x1A1918)
    static let surfaceDarkElevated = Color(hex: 0x262523)

    // MARK: - Text
    static let ink = Color(hex: 0x141413)
    static let textBody = Color(hex: 0x3D3D3A)
    static let textMuted = Color(hex: 0x6C6A64)
    static let textSoft = Color(hex: 0x8E8B82)
    static let textOnDark = Color(hex: 0xFAF9F5)

    // MARK: - Semantic
    static let success = Color(hex: 0x4DA664)
    static let warning = Color(hex: 0xD4A017)
    static let error = Color(hex: 0xC64545)
    static let teal = Color(hex: 0x5DB8A6)

    // MARK: - Borders & Lines
    static let border = Color(hex: 0xE2DDD4)
    static let borderLight = Color(hex: 0xEBE6DF)
    static let divider = Color(hex: 0xE6DFD8)

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
    static let shadowCard = Color.black.opacity(0.04)
    static let shadowMd = Color.black.opacity(0.06)

    // MARK: - Editorial additions

    static let warmInk = Color(hex: 0x1F1E1B)
    static let rule = Color(hex: 0xD5CFC4)
    static let paperTint = Color.black.opacity(0.06)

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

extension View {
    func card(padding: CGFloat = 16) -> some View {
        modifier(CardModifier(padding: padding))
    }
    func sectionHead() -> some View {
        modifier(SectionHeader())
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
}

import Foundation

/// How Chaos chooses its light/dark appearance. `.system` follows the Mac's
/// setting; `.light` / `.dark` pin the app regardless of the system.
enum AppearancePreference: String, CaseIterable, Identifiable {
    case system
    case light
    case dark

    var id: String { rawValue }

    var label: String {
        switch self {
        case .system: "System"
        case .light: "Light"
        case .dark: "Dark"
        }
    }

    static func from(_ raw: String?) -> AppearancePreference {
        guard let raw, let value = AppearancePreference(rawValue: raw) else { return .system }
        return value
    }
}

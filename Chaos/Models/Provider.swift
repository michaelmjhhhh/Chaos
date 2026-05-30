import Foundation

enum Provider: String, CaseIterable, Identifiable {
    case siliconrouter
    case openai
    case deepseek
    case openrouter
    case openaiCompatible = "openai-compatible"

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .siliconrouter: "SiliconRouter"
        case .openai: "OpenAI"
        case .deepseek: "DeepSeek"
        case .openrouter: "OpenRouter"
        case .openaiCompatible: "OpenAI-Compatible"
        }
    }

    var defaultBaseURL: String? {
        switch self {
        case .siliconrouter: "https://api.siliconrouter.com/v1"
        case .openai: "https://api.openai.com/v1"
        case .deepseek: "https://api.deepseek.com"
        case .openrouter: "https://openrouter.ai/api/v1"
        case .openaiCompatible: nil
        }
    }

    var defaultModel: String {
        switch self {
        case .siliconrouter: "gemini-3-flash-preview"
        case .openai: "gpt-4o-mini"
        case .deepseek: "deepseek-v4-flash"
        case .openrouter: "openai/gpt-4o-mini"
        case .openaiCompatible: "gpt-4o-mini"
        }
    }

    var requiresBaseURL: Bool {
        self == .openaiCompatible
    }

    static func from(_ raw: String?) -> Provider {
        guard let raw, !raw.isEmpty else { return .siliconrouter }
        let normalized = raw.lowercased().trimmingCharacters(in: .whitespaces)
        return Provider(rawValue: normalized) ?? .siliconrouter
    }
}

enum SlugLanguage: String, CaseIterable, Identifiable {
    case en
    case zh

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .en: "English"
        case .zh: "Chinese"
        }
    }

    static func from(_ raw: String?) -> SlugLanguage {
        guard let raw, !raw.isEmpty else { return .en }
        return SlugLanguage(rawValue: raw.lowercased().trimmingCharacters(in: .whitespaces)) ?? .en
    }
}

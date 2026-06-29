import Foundation

/// Configuration seam for the bundled "Chaos" hosted provider.
///
/// The goal is zero-config naming for non-technical users: on first launch the app
/// should name screenshots without anyone pasting an API key. That requires a hosted
/// proxy (base URL + a managed credential) that lives server-side and is rate-limited
/// per install — it is intentionally NOT shipped in plaintext here.
///
/// Until that backend is provisioned, `baseURL` stays empty and `isConfigured` is
/// `false`. The app detects this and gracefully falls back to the guided "bring your
/// own key" path. Once the proxy exists, fill in `baseURL` (and wire the credential in
/// `VisionAPIClient`) and flip the default in `Provider.from(_:)` to light up
/// zero-config automatically.
enum HostedProvider {
    /// Hosted proxy endpoint (Vercel). The app appends `/chat/completions`.
    static let baseURL = "https://chaos-snowy-ten.vercel.app/api"

    /// Model the proxy serves. The proxy pins this server-side; the value here is for
    /// display/consistency.
    static let model = "agnes-2.0-flash"

    /// Shared app token the proxy validates (first half of the Bearer; the second half is
    /// this device's hash, added in AppState.resolvedAPIKey). This is only a weak gate —
    /// real protection is the proxy's per-device + global limits — so shipping it is fine.
    /// Rotate by changing both this value and the Vercel APP_TOKEN env together.
    static let bundledCredential = "chaostrial2026alpha"

    /// Whether the hosted provider is ready for real use. When false, onboarding and
    /// settings steer the user to the guided key flow instead.
    static var isConfigured: Bool {
        !baseURL.isEmpty
    }
}

enum Provider: String, CaseIterable, Identifiable {
    case chaosHosted = "chaos-hosted"
    case siliconrouter
    case openai
    case deepseek
    case openrouter
    case openaiCompatible = "openai-compatible"
    case ollama

    var id: String {
        rawValue
    }

    var displayName: String {
        switch self {
        case .chaosHosted: "Chaos (recommended)"
        case .siliconrouter: "SiliconRouter"
        case .openai: "OpenAI"
        case .deepseek: "DeepSeek"
        case .openrouter: "OpenRouter"
        case .openaiCompatible: "OpenAI-Compatible"
        case .ollama: "Ollama"
        }
    }

    var defaultBaseURL: String? {
        switch self {
        case .chaosHosted: HostedProvider.baseURL.isEmpty ? nil : HostedProvider.baseURL
        case .siliconrouter: "https://api.siliconrouter.com/v1"
        case .openai: "https://api.openai.com/v1"
        case .deepseek: "https://api.deepseek.com"
        case .openrouter: "https://openrouter.ai/api/v1"
        case .openaiCompatible: nil
        case .ollama: "http://localhost:11434/v1"
        }
    }

    var defaultModel: String {
        switch self {
        case .chaosHosted: HostedProvider.model
        case .siliconrouter: "gemini-3-flash-preview"
        case .openai: "gpt-4o-mini"
        case .deepseek: "deepseek-v4-flash"
        case .openrouter: "openai/gpt-4o-mini"
        case .openaiCompatible: "gpt-4o-mini"
        case .ollama: "qwen3-vl:2b"
        }
    }

    /// Providers that need the user to supply their own API key. The bundled hosted
    /// provider and local Ollama do not.
    var requiresAPIKey: Bool {
        self != .ollama && self != .chaosHosted
    }

    var allowsCustomBaseURL: Bool {
        self == .openaiCompatible
    }

    var connectionKind: String {
        switch self {
        case .ollama: "Local"
        case .chaosHosted: "Built-in"
        default: "Remote"
        }
    }

    var summary: String {
        switch self {
        case .chaosHosted: "Built-in naming, no setup required."
        case .siliconrouter: "SiliconRouter hosted API."
        case .openai: "OpenAI hosted API."
        case .deepseek: "DeepSeek hosted API."
        case .openrouter: "OpenRouter hosted API."
        case .openaiCompatible: "Custom OpenAI-compatible endpoint."
        case .ollama: "Local vision model served by Ollama."
        }
    }

    /// One-line, jargon-free explanation of who each option is for. Shown in the
    /// provider picker so a non-technical user can choose with confidence.
    var plainDescription: String {
        switch self {
        case .chaosHosted:
            "Works instantly — Chaos names your screenshots for you. No account or key needed."
        case .siliconrouter:
            "Use a SiliconRouter account. Good value across many models."
        case .openai:
            "Use your own OpenAI account (the makers of ChatGPT)."
        case .deepseek:
            "Use your own DeepSeek account. Often lower cost."
        case .openrouter:
            "Use OpenRouter to reach many AI providers with one key."
        case .openaiCompatible:
            "Connect any service that speaks the OpenAI format. For advanced setups."
        case .ollama:
            "Run naming privately on your own Mac. Free, but you install Ollama first."
        }
    }

    /// Where to sign up / get a key for providers that need one. `nil` when no signup
    /// is required (Chaos hosted, Ollama).
    var signupURL: URL? {
        switch self {
        case .chaosHosted, .ollama: nil
        case .siliconrouter: URL(string: "https://siliconrouter.com")
        case .openai: URL(string: "https://platform.openai.com/api-keys")
        case .deepseek: URL(string: "https://platform.deepseek.com/api_keys")
        case .openrouter: URL(string: "https://openrouter.ai/keys")
        case .openaiCompatible: nil
        }
    }

    var connectionFailureHint: String {
        switch self {
        case .ollama: "Start Ollama, then run: ollama pull qwen3-vl:2b"
        case .chaosHosted: "The built-in naming service is unreachable. Check your internet, or connect your own provider in Settings."
        default: "Check your API key and network connection."
        }
    }

    var requiresBaseURL: Bool {
        self == .openaiCompatible
    }

    static func from(_ raw: String?) -> Provider {
        guard let raw, !raw.isEmpty else { return defaultProvider }
        let normalized = raw.lowercased().trimmingCharacters(in: .whitespaces)
        return Provider(rawValue: normalized) ?? defaultProvider
    }

    /// The provider a brand-new install starts on. Prefers the zero-config hosted
    /// provider once it is provisioned; otherwise falls back to SiliconRouter so the
    /// product keeps working for existing/technical users in the meantime.
    static var defaultProvider: Provider {
        HostedProvider.isConfigured ? .chaosHosted : .siliconrouter
    }
}

enum SlugLanguage: String, CaseIterable, Identifiable {
    case en
    case zh

    var id: String {
        rawValue
    }

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

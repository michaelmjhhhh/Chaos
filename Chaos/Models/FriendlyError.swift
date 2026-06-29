import Foundation

/// The next step we can offer a user after a failure. Surfaces as a hint and, where a
/// view supports it, a button.
enum RecoveryAction: Equatable {
    /// Re-run the same image through processing.
    case retry
    /// Open Settings (e.g. to fix a key or provider).
    case openSettings
    /// Check the network connection.
    case checkInternet
    /// Nothing actionable beyond reading the message.
    case none

    var label: String? {
        switch self {
        case .retry: "Try Again"
        case .openSettings: "Open Settings"
        case .checkInternet: "Try Again"
        case .none: nil
        }
    }
}

/// Translates technical failures into plain-language messages a non-programmer can act
/// on. Replaces raw strings like "API error: HTTP 401" with "Your key doesn't look
/// right — open Settings to re-enter it."
struct FriendlyError: Equatable {
    let message: String
    let action: RecoveryAction

    init(message: String, action: RecoveryAction = .none) {
        self.message = message
        self.action = action
    }

    init(_ error: Error, provider: Provider) {
        if let chaos = error as? ChaosError {
            self = Self.map(chaos, provider: provider)
        } else if let urlError = error as? URLError {
            self = Self.map(urlError, provider: provider)
        } else {
            self = FriendlyError(
                message: "Something went wrong while naming that image. Try again.",
                action: .retry
            )
        }
    }

    private static func map(_ error: ChaosError, provider: Provider) -> FriendlyError {
        switch error {
        case .httpStatus(let code):
            switch code {
            case 401, 403:
                return FriendlyError(
                    message: "Your API key doesn't look right. Open Settings to re-enter it.",
                    action: .openSettings
                )
            case 402:
                if provider == .chaosHosted {
                    return FriendlyError(
                        message: "You've used all your free names. Add your own provider key in Settings to keep going.",
                        action: .openSettings
                    )
                }
                return FriendlyError(
                    message: "Your provider account is out of credit. Add funds, or switch provider in Settings.",
                    action: .openSettings
                )
            case 404:
                return FriendlyError(
                    message: "That model wasn't found. Check the model name in Settings.",
                    action: .openSettings
                )
            case 429:
                return FriendlyError(
                    message: "The naming service is busy right now. Wait a moment and try again.",
                    action: .retry
                )
            case 500 ... 599:
                return FriendlyError(
                    message: "The naming service had a temporary problem. Try again in a moment.",
                    action: .retry
                )
            default:
                return FriendlyError(
                    message: "The naming service refused the request (error \(code)). \(provider.connectionFailureHint)",
                    action: .openSettings
                )
            }
        case .apiError(let raw):
            if raw.localizedCaseInsensitiveContains("base url") || raw.localizedCaseInsensitiveContains("endpoint") {
                return FriendlyError(
                    message: "The provider address looks wrong. Check the Base URL in Settings.",
                    action: .openSettings
                )
            }
            if raw.localizedCaseInsensitiveContains("empty response") {
                return FriendlyError(
                    message: "The AI didn't return a name for that image. Try again.",
                    action: .retry
                )
            }
            return FriendlyError(
                message: "The naming service reported a problem. Try again, or check Settings.",
                action: .retry
            )
        case .fileNotStable:
            return FriendlyError(
                message: "That screenshot was still being saved. Try again in a second.",
                action: .retry
            )
        case .renameCollision:
            return FriendlyError(
                message: "Couldn't find a free filename for that image. Try again.",
                action: .retry
            )
        case .configError(let raw):
            return FriendlyError(
                message: "Something's off with the setup: \(raw). Check Settings.",
                action: .openSettings
            )
        }
    }

    private static func map(_ error: URLError, provider: Provider) -> FriendlyError {
        switch error.code {
        case .notConnectedToInternet, .networkConnectionLost, .dataNotAllowed:
            FriendlyError(
                message: "You appear to be offline. Check your internet, then try again.",
                action: .checkInternet
            )
        case .timedOut:
            FriendlyError(
                message: "The naming service took too long to respond. Try again.",
                action: .retry
            )
        case .cannotFindHost, .cannotConnectToHost, .dnsLookupFailed:
            FriendlyError(
                message: provider == .ollama
                    ? "Couldn't reach Ollama on this Mac. Make sure Ollama is running."
                    : "Couldn't reach the naming service. Check your internet, then try again.",
                action: provider == .ollama ? .openSettings : .checkInternet
            )
        default:
            FriendlyError(
                message: "A network problem stopped naming that image. Try again.",
                action: .retry
            )
        }
    }
}

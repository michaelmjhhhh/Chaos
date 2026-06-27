import Foundation
import UserNotifications

/// Thin wrapper around user notifications so Chaos can speak up when it files (or fails
/// to file) a screenshot while running in the background. Deliberately defensive: the
/// notification APIs require a real, signed `.app` bundle, so every entry point first
/// checks `isAvailable` and silently no-ops otherwise (e.g. under `swift test`).
enum NotificationService {
    /// Only touch UNUserNotificationCenter from inside a genuine app bundle. Calling it
    /// from a bare executable or test host can trap at the system level.
    private static var isAvailable: Bool {
        Bundle.main.bundleURL.pathExtension == "app" && Bundle.main.bundleIdentifier != nil
    }

    /// Ask permission once, up front, when the user opts into notifications.
    static func requestAuthorization() {
        guard isAvailable else { return }
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound]) { _, _ in }
    }

    static func notifySuccess(originalName: String, newName: String) {
        post(
            title: "Screenshot filed",
            body: "“\(originalName)” → \(newName)"
        )
    }

    static func notifyError(originalName: String, message: String) {
        post(
            title: "Couldn't name a screenshot",
            body: "\(originalName): \(message)"
        )
    }

    private static func post(title: String, body: String) {
        guard isAvailable else { return }
        let content = UNMutableNotificationContent()
        content.title = title
        content.body = body
        let request = UNNotificationRequest(
            identifier: UUID().uuidString,
            content: content,
            trigger: nil
        )
        UNUserNotificationCenter.current().add(request)
    }
}

import Foundation
import IOKit
import CryptoKit
import Security

/// A stable, privacy-preserving identifier for this Mac.
///
/// The hosted free trial counts names per device, and we don't want a reinstall to reset
/// the count — so the identity is derived from the Mac's hardware UUID rather than from
/// app-local storage. The raw hardware UUID never leaves the machine: only a salted
/// SHA-256 hash is sent, as the second half of the hosted Bearer token.
///
/// If the hardware UUID can't be read, we fall back to a random UUID persisted in the
/// login Keychain (which also survives an app reinstall), so the app keeps working.
enum DeviceIdentity {
    /// App-specific salt so the value isn't a bare hardware-UUID hash that could be
    /// correlated across other software.
    private static let salt = "chaos.device.v1"

    /// Cached so we only touch IOKit/Keychain once per launch.
    private static let cached: String = computeHash()

    /// Lowercase hex SHA-256 used to identify this device to the hosted proxy.
    static var hash: String {
        cached
    }

    private static func computeHash() -> String {
        let seed = hardwareUUID() ?? keychainFallbackUUID()
        let digest = SHA256.hash(data: Data((salt + seed).utf8))
        return digest.map { String(format: "%02x", $0) }.joined()
    }

    /// The Mac's `IOPlatformUUID` — stable across app reinstalls and OS user accounts.
    private static func hardwareUUID() -> String? {
        let service = IOServiceGetMatchingService(
            kIOMainPortDefault,
            IOServiceMatching("IOPlatformExpertDevice")
        )
        guard service != 0 else { return nil }
        defer { IOObjectRelease(service) }

        guard let property = IORegistryEntryCreateCFProperty(
            service,
            kIOPlatformUUIDKey as CFString,
            kCFAllocatorDefault,
            0
        )?.takeRetainedValue() as? String, !property.isEmpty else {
            return nil
        }
        return property
    }

    // MARK: - Keychain fallback

    private static let keychainAccount = "device-seed"
    private static let keychainService = "com.chaos.app.device"

    /// A random UUID stored in the login Keychain. Keychain items aren't removed when an
    /// app is deleted, so this still resists an app-only reinstall.
    private static func keychainFallbackUUID() -> String {
        if let existing = readKeychain() { return existing }
        let fresh = UUID().uuidString
        writeKeychain(fresh)
        return fresh
    }

    private static func keychainQuery() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: keychainService,
            kSecAttrAccount as String: keychainAccount
        ]
    }

    private static func readKeychain() -> String? {
        var query = keychainQuery()
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne

        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let value = String(data: data, encoding: .utf8)
        else {
            return nil
        }
        return value
    }

    private static func writeKeychain(_ value: String) {
        var attributes = keychainQuery()
        attributes[kSecValueData as String] = Data(value.utf8)
        SecItemDelete(keychainQuery() as CFDictionary)
        SecItemAdd(attributes as CFDictionary, nil)
    }
}

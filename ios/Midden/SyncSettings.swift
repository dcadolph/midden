import Foundation
import Security

/// SyncSettings stores where the journal syncs and the credential it uses.
///
/// The remote URL and branch are ordinary preferences, but the access token is
/// a secret and lives in the Keychain rather than in UserDefaults or the vault.
enum SyncSettings {
    /// remoteKey is the defaults key holding the git remote URL.
    private static let remoteKey = "midden.sync.remote"
    /// branchKey is the defaults key holding the branch name.
    private static let branchKey = "midden.sync.branch"
    /// tokenService is the Keychain service the access token is filed under.
    private static let tokenService = "com.dcadolph.midden.sync"
    /// tokenAccount names the single token entry within that service.
    private static let tokenAccount = "access-token"

    /// remoteURL is the https git URL the vault syncs with.
    static var remoteURL: String {
        get { UserDefaults.standard.string(forKey: remoteKey) ?? "" }
        set { UserDefaults.standard.set(newValue.trimmingCharacters(in: .whitespaces), forKey: remoteKey) }
    }

    /// branch is the branch synced, defaulting to main.
    static var branch: String {
        get {
            let stored = UserDefaults.standard.string(forKey: branchKey) ?? ""
            return stored.isEmpty ? "main" : stored
        }
        set { UserDefaults.standard.set(newValue.trimmingCharacters(in: .whitespaces), forKey: branchKey) }
    }

    /// isConfigured reports whether a sync has everything it needs.
    static var isConfigured: Bool {
        !remoteURL.isEmpty && !token.isEmpty
    }

    /// token is the git access token, read from and written to the Keychain.
    /// Setting an empty value removes the stored token.
    static var token: String {
        get { readToken() ?? "" }
        set { writeToken(newValue) }
    }

    /// readToken returns the stored token, or nil when none is saved.
    private static func readToken() -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: tokenService,
            kSecAttrAccount as String: tokenAccount,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let value = String(data: data, encoding: .utf8) else {
            return nil
        }
        return value
    }

    /// writeToken saves the token, replacing any existing one.
    /// The item is marked available only after first unlock so a background
    /// sync can still reach it while the device is locked.
    private static func writeToken(_ value: String) {
        let base: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: tokenService,
            kSecAttrAccount as String: tokenAccount,
        ]
        SecItemDelete(base as CFDictionary)
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty, let data = trimmed.data(using: .utf8) else {
            return
        }
        var item = base
        item[kSecValueData as String] = data
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        SecItemAdd(item as CFDictionary, nil)
    }
}

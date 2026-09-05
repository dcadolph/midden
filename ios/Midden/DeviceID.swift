import Foundation

/// DeviceID is this device's stable inbox name.
///
/// Entries captured here are written to `inbox/<id>/` rather than to the
/// canonical day files, so this device and the desktop never append to the same
/// file on the same day and their journals merge without conflict. The name has
/// to stay stable across launches or a device would strand entries in an inbox
/// nothing drains.
enum DeviceID {
    /// defaultsKey is where the generated identifier is persisted.
    private static let defaultsKey = "midden.device.id"

    /// current returns the identifier, generating and storing one on first use.
    static var current: String {
        if let saved = UserDefaults.standard.string(forKey: defaultsKey), !saved.isEmpty {
            return saved
        }
        let generated = "ios-" + String(UUID().uuidString.prefix(8)).lowercased()
        UserDefaults.standard.set(generated, forKey: defaultsKey)
        return generated
    }
}

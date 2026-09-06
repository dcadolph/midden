import Foundation
import MiddenCore

/// VaultStore owns the Go vault handle and publishes what the screens display.
///
/// Every read and write crosses into the shared Go core, so the journal format,
/// encryption, and date handling match the command line exactly.
@MainActor
final class VaultStore: ObservableObject {
    /// today holds the entries written on the current date, newest last.
    @Published private(set) var today: [Entry] = []
    /// week holds the trailing seven days of entries, newest last.
    @Published private(set) var week: [Entry] = []
    /// streak is the run of consecutive days ending today that have entries.
    @Published private(set) var streak: Int = 0
    /// errorMessage carries the most recent failure for display.
    @Published var errorMessage: String?
    /// showingError drives the alert presentation.
    @Published var showingError = false
    /// syncStatus describes the outcome of the most recent sync attempt.
    @Published private(set) var syncStatus = ""
    /// isSyncing reports whether a sync is currently running.
    @Published private(set) var isSyncing = false

    /// vault is the Go handle, nil when the vault could not be opened.
    private var vault: MobileVault?

    init() {
        openVault()
        refresh()
    }

    /// vaultDirectory returns the on-device journal location.
    /// Documents keeps the vault visible to the Files app and included in
    /// device backups. Git sync will clone into this same directory.
    /// It is not actor-isolated so the App Intent can reach the vault without
    /// hopping to the main actor.
    nonisolated static var vaultDirectory: URL {
        let base = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
        return base.appendingPathComponent("midden", isDirectory: true)
    }

    /// openVault creates the journal directory if needed and opens the core handle.
    private func openVault() {
        let dir = VaultStore.vaultDirectory
        do {
            try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        } catch {
            report("Could not create the journal directory: \(error.localizedDescription)")
            return
        }
        var err: NSError?
        guard let handle = MobileOpenDevice(dir.path, "", DeviceID.current, &err) else {
            report("Could not open the vault: \(err?.localizedDescription ?? "unknown error")")
            return
        }
        vault = handle
    }

    /// append writes a new entry stamped with the device clock and refreshes the views.
    /// tags is a comma-separated list and may be empty.
    func append(tags: String, body: String) {
        guard let vault else {
            report("The vault is not open.")
            return
        }
        do {
            try vault.append(at: Entry.timestampFormatter.string(from: Date()), tags: tags, body: body)
            refresh()
        } catch {
            report("Could not save the entry: \(error.localizedDescription)")
            return
        }
        // The entry is already safe on disk, so a sync failure here is worth
        // reporting but never worth losing the capture over.
        Task { await sync() }
    }

    /// sync hands this device's captures to the remote and takes back whatever
    /// the other devices have written.
    ///
    /// It runs off the main actor because the transfer is network-bound, and it
    /// is a no-op when sync has not been configured, so capture works fully
    /// offline and unconfigured.
    func sync() async {
        guard let vault, SyncSettings.isConfigured, !isSyncing else { return }
        let remote = SyncSettings.remoteURL
        let token = SyncSettings.token
        let branch = SyncSettings.branch
        isSyncing = true
        defer { isSyncing = false }
        do {
            let summary = try await Task.detached {
                try bridged { vault.sync(remote, token: token, branch: branch, error: $0) }
            }.value
            syncStatus = summary
            refresh()
        } catch {
            syncStatus = "Sync failed: \(error.localizedDescription)"
        }
    }

    /// refresh reloads today, the trailing week, and the streak from the core.
    func refresh() {
        guard let vault else { return }
        let formatter = VaultStore.dateFormatter
        let now = Date()
        let todayKey = formatter.string(from: now)
        let weekStart = Calendar.current.date(byAdding: .day, value: -6, to: now) ?? now
        do {
            today = try Entry.decodeList(bridged { vault.dayJSON(todayKey, error: $0) })
            week = try Entry.decodeList(bridged {
                vault.rangeJSON(formatter.string(from: weekStart), to: todayKey, error: $0)
            })
            var count: Int = 0
            try vault.streak(&count)
            streak = count
        } catch {
            report("Could not read the journal: \(error.localizedDescription)")
        }
    }

    /// report surfaces a failure to the user without throwing out of a view action.
    private func report(_ message: String) {
        errorMessage = message
        showingError = true
    }

    /// dateFormatter renders the YYYY-MM-DD keys the core expects.
    static let dateFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd"
        return f
    }()
}

/// bridged calls a core method that reports failure through an error pointer and
/// rethrows it as a Swift error.
///
/// Methods returning a non-optional string cannot be imported as throwing, so the
/// pointer is threaded here rather than at every call site. It is a free function
/// so work running off the main actor can use it too.
func bridged(_ call: (NSErrorPointer) -> String) throws -> String {
    var err: NSError?
    let result = call(&err)
    if let err {
        throw err
    }
    return result
}

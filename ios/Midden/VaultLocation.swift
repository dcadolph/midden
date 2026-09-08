import Foundation

/// VaultLocation decides where the journal lives on this device.
///
/// iCloud Drive is the default so the journal is backed up and shared between
/// devices without the person setting anything up. When iCloud is unavailable,
/// because nobody is signed in or the person turned it off for this app, the
/// journal falls back to local storage and keeps working; the entries are still
/// captured, they simply are not backed up until iCloud returns.
enum VaultLocation {
    /// folderName is the journal directory inside whichever container is used.
    private static let folderName = "midden"

    /// containerID is the app's iCloud container.
    private static let containerID = "iCloud.com.dcadolph.middenjournal"

    /// Place is a resolved journal directory and whether it is backed up.
    struct Place {
        /// url is the journal directory.
        let url: URL
        /// isCloud reports whether the directory lives in iCloud Drive.
        let isCloud: Bool
    }

    /// resolve returns the journal directory, preferring iCloud Drive.
    ///
    /// The lookup runs off the main actor because asking for the container can
    /// block the first time it is called on a device, and a journal app that
    /// freezes on launch is worse than one that takes a moment to settle.
    /// Entries already written locally are moved into iCloud the first time it
    /// becomes available, so turning on iCloud never appears to erase history.
    static func resolve() async -> Place {
        await Task.detached(priority: .userInitiated) {
            guard let container = FileManager.default.url(forUbiquityContainerIdentifier: containerID) else {
                return Place(url: localDirectory(), isCloud: false)
            }
            let cloud = container.appendingPathComponent("Documents", isDirectory: true)
                .appendingPathComponent(folderName, isDirectory: true)
            do {
                try FileManager.default.createDirectory(at: cloud, withIntermediateDirectories: true)
            } catch {
                return Place(url: localDirectory(), isCloud: false)
            }
            migrateLocalEntries(into: cloud)
            return Place(url: cloud, isCloud: true)
        }.value
    }

    /// localDirectory returns the on-device journal directory, creating it if needed.
    static func localDirectory() -> URL {
        let base = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
        let dir = base.appendingPathComponent(folderName, isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    /// migrateLocalEntries moves a previously local journal into iCloud once.
    ///
    /// Only entries the cloud does not already hold are moved, and the local
    /// copy is left in place rather than deleted, so a failed migration cannot
    /// lose anything. A local directory with nothing in it is ignored.
    private static func migrateLocalEntries(into cloud: URL) {
        let manager = FileManager.default
        let local = localDirectory()
        guard let walker = manager.enumerator(at: local,
                                              includingPropertiesForKeys: [.isRegularFileKey],
                                              options: [.skipsHiddenFiles]) else {
            return
        }
        for case let source as URL in walker {
            guard (try? source.resourceValues(forKeys: [.isRegularFileKey]))?.isRegularFile == true else {
                continue
            }
            let relative = source.path.replacingOccurrences(of: local.path + "/", with: "")
            let destination = cloud.appendingPathComponent(relative)
            if manager.fileExists(atPath: destination.path) {
                continue
            }
            try? manager.createDirectory(at: destination.deletingLastPathComponent(),
                                         withIntermediateDirectories: true)
            try? manager.copyItem(at: source, to: destination)
        }
    }

    /// warmUp asks iCloud to pull down any journal files it is holding as
    /// placeholders, so a read of an evicted day file finds real content.
    ///
    /// iCloud can evict the contents of files it considers cold, leaving a stub
    /// behind. The Go core reads files directly and would see an empty or
    /// missing file, so the download is requested up front.
    static func warmUp(_ dir: URL) {
        let manager = FileManager.default
        guard let walker = manager.enumerator(at: dir,
                                              includingPropertiesForKeys: [.isUbiquitousItemKey,
                                                                           .ubiquitousItemDownloadingStatusKey],
                                              options: [.skipsHiddenFiles]) else {
            return
        }
        for case let url as URL in walker {
            let values = try? url.resourceValues(forKeys: [.ubiquitousItemDownloadingStatusKey])
            if values?.ubiquitousItemDownloadingStatus == .current {
                continue
            }
            try? manager.startDownloadingUbiquitousItem(at: url)
        }
    }
}

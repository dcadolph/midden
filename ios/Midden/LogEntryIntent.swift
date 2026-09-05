import AppIntents
import Foundation
import MiddenCore

/// LogEntryIntent appends a spoken or typed entry without opening the app.
///
/// Siri, the Shortcuts app, and the Action button all reach the journal through
/// this intent, which is the phone counterpart to the "Hey Siri, journal"
/// shortcut on the desktop.
struct LogEntryIntent: AppIntent {
    static var title: LocalizedStringResource = "Log a journal entry"
    static var description = IntentDescription("Append an entry to today's journal.")
    static var openAppWhenRun = false

    /// text is the entry body. Siri asks for it when the shortcut supplies none.
    @Parameter(title: "Entry", requestValueDialog: "What should I write down?")
    var text: String

    /// tags is an optional comma-separated tag list.
    @Parameter(title: "Tags", default: "")
    var tags: String

    static var parameterSummary: some ParameterSummary {
        Summary("Journal \(\.$text)")
    }

    /// perform appends the entry through the shared core.
    func perform() async throws -> some IntentResult & ProvidesDialog {
        let body = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !body.isEmpty else {
            throw AppIntentError.entryWasEmpty
        }
        let dir = VaultStore.vaultDirectory
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        var err: NSError?
        guard let vault = MobileOpen(dir.path, "", &err) else {
            throw AppIntentError.vaultUnavailable(err?.localizedDescription ?? "unknown error")
        }
        try vault.append(at: Entry.timestampFormatter.string(from: Date()), tags: tags, body: body)
        return .result(dialog: "Added to your journal.")
    }
}

/// AppIntentError describes why an intent could not append an entry.
enum AppIntentError: Error, CustomLocalizedStringResourceConvertible {
    /// entryWasEmpty means no text was dictated or supplied.
    case entryWasEmpty
    /// vaultUnavailable means the journal directory could not be opened.
    case vaultUnavailable(String)

    var localizedStringResource: LocalizedStringResource {
        switch self {
        case .entryWasEmpty:
            return "There was nothing to write down."
        case .vaultUnavailable(let reason):
            return "The journal could not be opened: \(reason)"
        }
    }
}

/// MiddenShortcuts registers the phrases that trigger the journal intent.
struct MiddenShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: LogEntryIntent(),
            phrases: [
                "Journal with \(.applicationName)",
                "Add to my \(.applicationName)",
                "Log an entry in \(.applicationName)"
            ],
            shortTitle: "Journal",
            systemImageName: "square.and.pencil"
        )
    }
}

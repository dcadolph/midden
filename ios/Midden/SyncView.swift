import SwiftUI

/// SyncView configures the git remote and runs a sync on demand.
struct SyncView: View {
    /// store performs the sync and reports its outcome.
    @EnvironmentObject private var store: VaultStore
    /// remote is the edited git remote URL.
    @State private var remote = SyncSettings.remoteURL
    /// branch is the edited branch name.
    @State private var branch = SyncSettings.branch
    /// token is the edited access token.
    @State private var token = SyncSettings.token
    /// saved marks that the settings were written.
    @State private var saved = false

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("https://github.com/you/vault.git", text: $remote)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                        .keyboardType(.URL)
                    TextField("Branch", text: $branch)
                        .autocorrectionDisabled()
                        .textInputAutocapitalization(.never)
                    SecureField("Access token", text: $token)
                        .textContentType(.password)
                } header: {
                    Text("Remote")
                } footer: {
                    Text("The token is kept in the device keychain, never in the journal. "
                         + "Entries are captured on this device whether or not sync is set up.")
                }

                Section {
                    Button("Save settings", action: save)
                    Button {
                        Task { await store.sync() }
                    } label: {
                        HStack {
                            Text("Sync now")
                            if store.isSyncing {
                                Spacer()
                                ProgressView()
                            }
                        }
                    }
                    .disabled(store.isSyncing || remote.trimmingCharacters(in: .whitespaces).isEmpty)
                }

                if saved {
                    Section { Text("Settings saved.").foregroundStyle(.secondary) }
                }
                if !store.syncStatus.isEmpty {
                    Section("Last sync") {
                        Text(store.syncStatus).font(.footnote).foregroundStyle(.secondary)
                    }
                }
                Section("This device") {
                    Text(DeviceID.current).font(.footnote.monospaced()).foregroundStyle(.secondary)
                }
            }
            .navigationTitle("Sync")
        }
    }

    /// save writes the edited settings back to preferences and the keychain.
    private func save() {
        SyncSettings.remoteURL = remote
        SyncSettings.branch = branch
        SyncSettings.token = token
        saved = true
    }
}

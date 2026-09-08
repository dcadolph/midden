import SwiftUI

/// CaptureView is the speak-or-type screen for writing a new entry.
struct CaptureView: View {
    /// store receives the finished entry.
    @EnvironmentObject private var store: VaultStore
    /// dictation supplies the spoken text.
    @StateObject private var dictation = Dictation()
    /// draft is the entry body, editable whether spoken or typed.
    @State private var draft = ""
    /// tags is the comma-separated tag list applied to the entry.
    @State private var tags = ""
    /// savedAt marks the last successful save so the view can confirm it.
    @State private var savedAt: Date?
    /// suggestions are the tags used most often, offered as one-tap chips.
    @State private var suggestions: [TagCount] = []
    /// showingSync presents the sync settings sheet.
    @State private var showingSync = false

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                TextEditor(text: $draft)
                    .frame(minHeight: 180)
                    .padding(8)
                    .overlay(RoundedRectangle(cornerRadius: 10).stroke(.secondary.opacity(0.3)))
                    .overlay(alignment: .topLeading) {
                        if draft.isEmpty {
                            Text("Speak or type an entry.")
                                .foregroundStyle(.secondary)
                                .padding(16)
                                .allowsHitTesting(false)
                        }
                    }

                TextField("Tags, comma separated", text: $tags)
                    .textFieldStyle(.roundedBorder)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)

                recentTags

                recordButton

                if let message = dictation.errorMessage {
                    Text(message)
                        .font(.footnote)
                        .foregroundStyle(.red)
                        .multilineTextAlignment(.center)
                }

                Button("Save entry", action: save)
                    .buttonStyle(.borderedProminent)
                    .disabled(draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

                if let savedAt {
                    Text("Saved at \(savedAt.formatted(date: .omitted, time: .shortened))")
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                }

                Spacer()
            }
            .padding()
            .navigationTitle("Capture")
            .toolbar {
                // Sync is configuration rather than a place to visit, so it
                // sits behind a control instead of spending a tab.
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        showingSync = true
                    } label: {
                        Label("Sync", systemImage: "arrow.triangle.2.circlepath")
                    }
                }
            }
            .sheet(isPresented: $showingSync) {
                SyncView()
            }
            .onChange(of: dictation.transcript) { _, transcript in
                if !transcript.isEmpty {
                    draft = transcript
                }
            }
            .onAppear { suggestions = store.tagCounts(limit: 12) }
        }
    }

    /// recentTags offers the most used tags as chips.
    ///
    /// Tagging at capture time is what makes a question like "the vacation
    /// entries" answerable later by plain retrieval, so it has to cost one tap
    /// rather than a spelling.
    private var recentTags: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 8) {
                ForEach(suggestions) { suggestion in
                    TagChip(tag: suggestion.tag,
                            isSelected: selected.contains(suggestion.tag)) {
                        toggle(tag: suggestion.tag)
                    }
                }
            }
        }
        .frame(height: suggestions.isEmpty ? 0 : 48)
    }

    /// selected is the set of tags currently in the tag field.
    private var selected: Set<String> {
        Set(tags.split(separator: ",")
            .map { $0.trimmingCharacters(in: .whitespaces).lowercased() }
            .filter { !$0.isEmpty })
    }

    /// toggle adds or removes a tag from the comma-separated field.
    private func toggle(tag: String) {
        var current = tags.split(separator: ",")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        if let index = current.firstIndex(where: { $0.caseInsensitiveCompare(tag) == .orderedSame }) {
            current.remove(at: index)
        } else {
            current.append(tag)
        }
        tags = current.joined(separator: ", ")
    }

    /// recordButton starts and stops dictation.
    private var recordButton: some View {
        Button {
            if dictation.isRecording {
                dictation.stop()
            } else {
                dictation.start()
            }
        } label: {
            Label(dictation.isRecording ? "Stop" : "Speak",
                  systemImage: dictation.isRecording ? "stop.circle.fill" : "mic.circle.fill")
                .font(.title2)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
        }
        .buttonStyle(.bordered)
        .tint(dictation.isRecording ? .red : .accentColor)
    }

    /// save appends the draft and clears the screen for the next entry.
    private func save() {
        dictation.stop()
        let body = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !body.isEmpty else { return }
        store.append(tags: tags, body: body)
        draft = ""
        dictation.reset()
        savedAt = Date()
    }
}

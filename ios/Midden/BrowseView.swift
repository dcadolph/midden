import SwiftUI

/// TagCount pairs a tag with how many entries carry it.
struct TagCount: Codable, Identifiable, Hashable {
    /// tag is the label without a leading hash character.
    let tag: String
    /// count is the number of entries carrying the label.
    let count: Int

    /// id distinguishes tags within a list.
    var id: String { tag }
}

/// BrowseView finds past entries by text, by tag, or by this day in other years.
///
/// None of this needs a model. The journal already knows its own words, so
/// retrieval is exact, instant, and works with no network.
struct BrowseView: View {
    /// store answers the queries.
    @EnvironmentObject private var store: VaultStore
    /// query is the current search text.
    @State private var query = ""
    /// results holds the entries matching the active query or tag.
    @State private var results: [Entry] = []
    /// tags are the labels in use, most used first.
    @State private var tags: [TagCount] = []
    /// activeTag is the tag being browsed, empty when searching by text.
    @State private var activeTag = ""
    /// showingFlashback switches the result list to this day in other years.
    @State private var showingFlashback = false

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // The chips sit outside the List because a List row treats its
                // contents as one tap target, which swallows taps meant for the
                // individual chips.
                if !tags.isEmpty {
                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack(spacing: 8) {
                            ForEach(tags) { tag in
                                TagChip(tag: tag.tag,
                                        count: tag.count,
                                        isSelected: activeTag == tag.tag) {
                                    select(tag: tag.tag)
                                }
                            }
                        }
                        .padding(.horizontal)
                        .padding(.vertical, 8)
                    }
                }

                List {
                    Section {
                        Button(action: showFlashback) {
                            Label("On this day in past years", systemImage: "clock.arrow.circlepath")
                        }
                    }

                    if !results.isEmpty {
                        Section(resultTitle) {
                            ForEach(results) { entry in
                                // Results span many days, so each row needs its
                                // date. EntryRow already carries time and tags.
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(entry.time.formatted(date: .abbreviated, time: .omitted))
                                        .font(.caption.weight(.medium))
                                        .foregroundStyle(.secondary)
                                    EntryRow(entry: entry)
                                }
                            }
                        }
                    }
                }
                .overlay {
                    if results.isEmpty && !query.isEmpty {
                        ContentUnavailableView.search(text: query)
                    }
                }
            }
            .navigationTitle("Browse")
            .searchable(text: $query, prompt: "Search your entries")
            .onSubmit(of: .search, runSearch)
            .onChange(of: query) { _, text in
                if text.isEmpty {
                    clearResults()
                }
            }
            .onAppear { tags = store.tagCounts(limit: 40) }
        }
    }

    /// resultTitle describes what the current result list is showing.
    private var resultTitle: String {
        if showingFlashback {
            return "On this day"
        }
        if !activeTag.isEmpty {
            return "#\(activeTag)"
        }
        return "Results"
    }

    /// runSearch performs a text search over every entry.
    private func runSearch() {
        activeTag = ""
        showingFlashback = false
        results = store.search(query)
    }

    /// select shows every entry carrying the given tag, or clears it when tapped again.
    private func select(tag: String) {
        if activeTag == tag {
            clearResults()
            return
        }
        query = ""
        showingFlashback = false
        activeTag = tag
        results = store.entries(tagged: tag)
    }

    /// showFlashback lists entries from earlier years on today's calendar date.
    private func showFlashback() {
        query = ""
        activeTag = ""
        showingFlashback = true
        results = store.flashback()
    }

    /// clearResults empties the result list and the active selections.
    private func clearResults() {
        activeTag = ""
        showingFlashback = false
        results = []
    }
}

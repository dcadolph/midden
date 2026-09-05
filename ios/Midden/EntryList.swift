import SwiftUI

/// EntryRow renders one journal entry with its time and tags.
struct EntryRow: View {
    /// entry is the record to display.
    let entry: Entry

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                Text(entry.time.formatted(date: .omitted, time: .shortened))
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
                ForEach(entry.tags, id: \.self) { tag in
                    Text("#\(tag)")
                        .font(.caption2)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(.secondary.opacity(0.15), in: Capsule())
                }
            }
            Text(entry.body)
                .font(.body)
        }
        .padding(.vertical, 4)
    }
}

/// TodayView lists the entries written today alongside the current streak.
struct TodayView: View {
    /// store supplies today's entries.
    @EnvironmentObject private var store: VaultStore

    var body: some View {
        NavigationStack {
            List {
                if store.today.isEmpty {
                    ContentUnavailableView("Nothing yet today",
                                           systemImage: "square.and.pencil",
                                           description: Text("Entries you capture will appear here."))
                } else {
                    ForEach(store.today) { entry in
                        EntryRow(entry: entry)
                    }
                }
            }
            .navigationTitle("Today")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Label("\(store.streak)", systemImage: "flame")
                        .labelStyle(.titleAndIcon)
                        .font(.subheadline)
                }
            }
            .refreshable { store.refresh() }
            .onAppear { store.refresh() }
        }
    }
}

/// WeekView groups the trailing seven days of entries by day.
struct WeekView: View {
    /// store supplies the week's entries.
    @EnvironmentObject private var store: VaultStore

    var body: some View {
        NavigationStack {
            List {
                ForEach(groupedDays, id: \.key) { group in
                    Section(group.key.formatted(date: .abbreviated, time: .omitted)) {
                        ForEach(group.value) { entry in
                            EntryRow(entry: entry)
                        }
                    }
                }
            }
            .navigationTitle("This week")
            .overlay {
                if store.week.isEmpty {
                    ContentUnavailableView("No entries this week",
                                           systemImage: "calendar",
                                           description: Text("The last seven days are empty."))
                }
            }
            .refreshable { store.refresh() }
            .onAppear { store.refresh() }
        }
    }

    /// groupedDays buckets the week's entries by calendar day, newest day first.
    private var groupedDays: [(key: Date, value: [Entry])] {
        let calendar = Calendar.current
        let groups = Dictionary(grouping: store.week) { calendar.startOfDay(for: $0.time) }
        return groups.sorted { $0.key > $1.key }
    }
}

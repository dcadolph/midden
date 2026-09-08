import SwiftUI

/// LifeView shows the whole record at once: how much of it there is, when it
/// was written, the chapters it falls into, what recurs, when it went quiet,
/// and who is in it.
struct LifeView: View {
    /// store computes the analysis.
    @EnvironmentObject private var store: VaultStore
    /// insights is the loaded analysis.
    @State private var insights = Insights.empty
    /// loading marks the first computation, which walks every day file.
    @State private var loading = true

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 28) {
                    totals
                    heatmap
                    section("Chapters", empty: insights.eras.isEmpty) {
                        ForEach(insights.eras) { era in
                            barRow(title: era.span,
                                   detail: "\(era.entries) entries, \(era.per_month) a month",
                                   width: era.width)
                        }
                    }
                    section("Still going", empty: insights.ongoing.isEmpty) {
                        ForEach(insights.ongoing) { thread in
                            textRow(thread.label, "\(thread.span). \(thread.detail)")
                        }
                    }
                    section("Quiet stretches", empty: insights.silences.isEmpty) {
                        ForEach(insights.silences) { silence in
                            textRow(silence.span, "\(silence.months) months. \(silence.detail)")
                        }
                    }
                    section("People", empty: insights.people.isEmpty) {
                        ForEach(insights.people) { person in
                            textRow(person.name, personDetail(person))
                        }
                    }
                    section("Faded out", empty: insights.dormant.isEmpty) {
                        ForEach(insights.dormant) { thread in
                            textRow(thread.label, thread.detail)
                        }
                    }
                }
                .padding()
                // The tab bar floats over the content, so the last rows need
                // room or they sit underneath it.
                .padding(.bottom, 60)
            }
            .navigationTitle("Your record")
            .overlay {
                if loading {
                    ProgressView("Reading the whole record")
                } else if insights.stats.entries == 0 {
                    ContentUnavailableView("Nothing to map yet",
                                           systemImage: "chart.bar.doc.horizontal",
                                           description: Text("Write for a while and the shape of it appears here."))
                }
            }
            .refreshable { await load() }
            .task { await load() }
        }
    }

    /// totals is the headline count of what the record holds.
    private var totals: some View {
        HStack(alignment: .top) {
            stat("\(insights.stats.entries)", "entries")
            stat("\(insights.stats.days)", "days")
            stat(wordLabel, "words")
            stat("\(insights.streak)", "day streak")
        }
    }

    /// wordLabel abbreviates the word count so four numbers fit on one line.
    private var wordLabel: String {
        let words = insights.stats.words
        return words >= 1000 ? "\(words / 1000)k" : "\(words)"
    }

    /// stat renders one headline number with its label.
    private func stat(_ value: String, _ label: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(value).font(.title2.bold().monospacedDigit())
            Text(label).font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// heatmap draws one small square per day, columns running week by week, so
    /// a year of writing habit reads at a glance.
    private var heatmap: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("When you wrote").font(.headline)
            ScrollView(.horizontal, showsIndicators: false) {
                let columns = Array(repeating: GridItem(.fixed(11), spacing: 3), count: 7)
                LazyHGrid(rows: columns, spacing: 3) {
                    ForEach(insights.calendar) { cell in
                        RoundedRectangle(cornerRadius: 2)
                            .fill(shade(for: cell.count))
                            .frame(width: 11, height: 11)
                    }
                }
                .frame(height: 7 * 11 + 6 * 3)
            }
        }
    }

    /// shade maps a day's entry count onto the heatmap ramp.
    private func shade(for count: Int) -> Color {
        switch count {
        case 0: return Color.secondary.opacity(0.12)
        case 1: return Color.accentColor.opacity(0.35)
        case 2...3: return Color.accentColor.opacity(0.6)
        case 4...6: return Color.accentColor.opacity(0.8)
        default: return Color.accentColor
        }
    }

    /// section wraps a titled group, hiding itself when it has nothing to show.
    @ViewBuilder
    private func section<Content: View>(_ title: String,
                                        empty: Bool,
                                        @ViewBuilder content: () -> Content) -> some View {
        if !empty {
            VStack(alignment: .leading, spacing: 10) {
                Text(title).font(.headline)
                content()
            }
        }
    }

    /// barRow renders a labelled row with a proportional bar behind it.
    private func barRow(title: String, detail: String, width: Int) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.subheadline.weight(.medium))
            GeometryReader { geo in
                RoundedRectangle(cornerRadius: 3)
                    .fill(Color.accentColor.opacity(0.35))
                    .frame(width: max(2, geo.size.width * CGFloat(width) / 100), height: 6)
            }
            .frame(height: 6)
            Text(detail).font(.caption).foregroundStyle(.secondary)
        }
    }

    /// textRow renders a labelled row with a supporting line.
    private func textRow(_ title: String, _ detail: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(title).font(.subheadline.weight(.medium))
            Text(detail).font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// personDetail describes how long someone has been in the record.
    private func personDetail(_ person: Insights.Person) -> String {
        let mentions = "\(person.mentions) mentions, \(person.span)"
        return person.last_seen.isEmpty ? mentions : mentions + ". Last seen \(person.last_seen)"
    }

    /// load computes the analysis off the main actor, since it reads every day file.
    private func load() async {
        loading = insights.stats.entries == 0
        insights = await store.insights()
        loading = false
    }
}

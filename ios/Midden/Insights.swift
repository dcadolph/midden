import Foundation

/// Insights is the computed shape of the whole record, as the Go core describes it.
struct Insights: Codable {
    /// stats are the record's totals.
    let stats: Stats
    /// streak is the run of consecutive days ending today.
    let streak: Int
    /// calendar is one cell per day in the heatmap window.
    let calendar: [CalendarCell]
    /// tags is the tag histogram, most used first.
    let tags: [TagBar]
    /// eras are the chapters the record falls into.
    let eras: [Era]
    /// ongoing, dormant, and ended are recurring threads by status.
    let ongoing: [Thread]
    let dormant: [Thread]
    let ended: [Thread]
    /// silences are the stretches where the record went quiet.
    let silences: [Silence]
    /// people are the names the record keeps mentioning.
    let people: [Person]

    /// empty is the placeholder shown before the first load.
    static let empty = Insights(stats: Stats(days: 0, entries: 0, words: 0, tags: 0),
                                streak: 0, calendar: [], tags: [], eras: [],
                                ongoing: [], dormant: [], ended: [], silences: [], people: [])

    /// Decoding treats a missing or null list as an empty one.
    ///
    /// The core marshals an empty Go slice as null rather than [], and a record
    /// too short to have eras or finished threads legitimately has none. Failing
    /// the whole decode over that would blank the screen for a real journal.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        stats = try c.decode(Stats.self, forKey: .stats)
        streak = try c.decodeIfPresent(Int.self, forKey: .streak) ?? 0
        calendar = try c.decodeIfPresent([CalendarCell].self, forKey: .calendar) ?? []
        tags = try c.decodeIfPresent([TagBar].self, forKey: .tags) ?? []
        eras = try c.decodeIfPresent([Era].self, forKey: .eras) ?? []
        ongoing = try c.decodeIfPresent([Thread].self, forKey: .ongoing) ?? []
        dormant = try c.decodeIfPresent([Thread].self, forKey: .dormant) ?? []
        ended = try c.decodeIfPresent([Thread].self, forKey: .ended) ?? []
        silences = try c.decodeIfPresent([Silence].self, forKey: .silences) ?? []
        people = try c.decodeIfPresent([Person].self, forKey: .people) ?? []
    }

    /// Memberwise construction for the placeholder value.
    init(stats: Stats, streak: Int, calendar: [CalendarCell], tags: [TagBar], eras: [Era],
         ongoing: [Thread], dormant: [Thread], ended: [Thread],
         silences: [Silence], people: [Person]) {
        self.stats = stats
        self.streak = streak
        self.calendar = calendar
        self.tags = tags
        self.eras = eras
        self.ongoing = ongoing
        self.dormant = dormant
        self.ended = ended
        self.silences = silences
        self.people = people
    }

    /// Stats holds the record's totals.
    struct Stats: Codable {
        let days: Int
        let entries: Int
        let words: Int
        let tags: Int
    }

    /// CalendarCell is one day in the heatmap.
    struct CalendarCell: Codable, Identifiable {
        let date: String
        let count: Int
        var id: String { date }
    }

    /// TagBar is one row of the tag histogram.
    struct TagBar: Codable, Identifiable {
        let tag: String
        let count: Int
        let width: Int
        var id: String { tag }
    }

    /// Era is one chapter of the record.
    struct Era: Codable, Identifiable {
        let span: String
        let months: Int
        let entries: Int
        let per_month: String
        let width: Int
        var id: String { span }
    }

    /// Thread is one recurring commitment.
    struct Thread: Codable, Identifiable {
        let label: String
        let count: Int
        let span: String
        let detail: String
        var id: String { label + span }
    }

    /// Silence is one stretch where the record went quiet.
    struct Silence: Codable, Identifiable {
        let span: String
        let months: Int
        let scope: String
        let detail: String
        var id: String { span + scope }
    }

    /// Person is one name the record keeps mentioning.
    struct Person: Codable, Identifiable {
        let name: String
        let mentions: Int
        let span: String
        let last_seen: String
        var id: String { name }
    }
}

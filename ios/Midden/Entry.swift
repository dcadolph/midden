import Foundation

/// Entry is one journal record as the Go core serializes it.
struct Entry: Codable, Identifiable, Hashable {
    /// time is the entry timestamp.
    let time: Date
    /// tags are the entry tags without leading hash characters.
    let tags: [String]
    /// body is the free-form entry text.
    let body: String

    /// id distinguishes entries within a list.
    var id: String { "\(time.timeIntervalSince1970)-\(body.hashValue)" }

    private enum CodingKeys: String, CodingKey {
        case time, tags, body
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let raw = try container.decode(String.self, forKey: .time)
        guard let parsed = Entry.timestampFormatter.date(from: raw) else {
            throw DecodingError.dataCorruptedError(forKey: .time, in: container,
                                                   debugDescription: "unparsable timestamp \(raw)")
        }
        time = parsed
        tags = try container.decodeIfPresent([String].self, forKey: .tags) ?? []
        body = try container.decode(String.self, forKey: .body)
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(Entry.timestampFormatter.string(from: time), forKey: .time)
        try container.encode(tags, forKey: .tags)
        try container.encode(body, forKey: .body)
    }

    /// timestampFormatter reads and writes the core's wire format.
    ///
    /// Day files record local wall clock time with no zone, so the timestamp is
    /// interpreted in the device's current time zone rather than converted.
    static let timestampFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "yyyy-MM-dd'T'HH:mm:ss"
        f.locale = Locale(identifier: "en_US_POSIX")
        f.timeZone = .current
        return f
    }()

    /// decodeList parses the JSON array the core returns for a read.
    static func decodeList(_ json: String) throws -> [Entry] {
        guard let data = json.data(using: .utf8) else { return [] }
        return try JSONDecoder().decode([Entry].self, from: data)
    }
}

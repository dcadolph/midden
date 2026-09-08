// Package mobile exposes a narrow midden core surface for gomobile bind.
//
// The API is deliberately flat because gomobile restricts the types that
// cross the language boundary: strings and ints in, JSON strings out. Dates
// cross as "2006-01-02" and timestamps as "2006-01-02T15:04:05". Entry lists
// are JSON arrays of {time, tags, body} objects.
//
// Timestamps carry no zone offset because day files record local wall clock
// time and nothing else. A bound framework cannot rely on time.Local either,
// since the host process reports UTC on iOS, so the caller supplies the wall
// clock it wants recorded and reads it back unchanged.
package mobile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/util"
	"github.com/dcadolph/midden/internal/vault"
)

// layoutDate is the wire format for calendar dates.
const layoutDate = "2006-01-02"

// layoutTimestamp is the wire format for entry timestamps: local wall clock
// with no zone offset, matching what a day file records.
const layoutTimestamp = "2006-01-02T15:04:05"

// Vault is a handle to a journal directory usable from Swift.
type Vault struct {
	// v is the canonical journal the desktop owns.
	v *vault.Vault
	// inbox is this device's own append target, nil when the handle writes
	// straight to the canonical day files.
	inbox *vault.Vault
}

// Open returns a Vault that appends straight to the canonical day files.
// passphrase may be empty for an unencrypted vault; for an encrypted vault it
// is verified before the handle is returned so a wrong passphrase fails here
// rather than on first read.
func Open(dir, passphrase string) (*Vault, error) {
	v, err := openCanonical(dir, passphrase)
	if err != nil {
		return nil, err
	}
	return &Vault{v: v}, nil
}

// OpenDevice returns a Vault that appends to this device's inbox rather than to
// the canonical day files, which is what a synced device must do so two devices
// never write the same file on the same day. Reads still cover both, so the
// device sees its own unsynced entries alongside everything else.
func OpenDevice(dir, passphrase, device string) (*Vault, error) {
	v, err := openCanonical(dir, passphrase)
	if err != nil {
		return nil, err
	}
	box, err := v.Inbox(device)
	if err != nil {
		return nil, fmt.Errorf("open inbox: %w", err)
	}
	return &Vault{v: v, inbox: box}, nil
}

// openCanonical opens the journal root and unlocks it when it is encrypted.
func openCanonical(dir, passphrase string) (*vault.Vault, error) {
	v, err := vault.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	if passphrase != "" {
		v = v.WithPassphrase(passphrase)
	}
	if v.IsEncrypted() {
		if err := v.VerifyPassphrase(); err != nil {
			return nil, fmt.Errorf("verify passphrase: %w", err)
		}
	}
	return v, nil
}

// writeTarget returns the vault new entries are appended to.
func (m *Vault) writeTarget() *vault.Vault {
	if m.inbox != nil {
		return m.inbox
	}
	return m.v
}

// Dir returns the absolute vault directory path.
func (m *Vault) Dir() string {
	return m.v.Dir
}

// IsEncrypted reports whether the vault encrypts day files at rest.
func (m *Vault) IsEncrypted() bool {
	return m.v.IsEncrypted()
}

// AppendAt writes a new entry stamped with the given local wall clock time,
// formatted as "2006-01-02T15:04:05".
//
// The caller supplies the timestamp rather than the core reading a clock, both
// so the host's time zone is authoritative and so a capture recorded offline
// keeps the time it was spoken rather than the time it was synced.
func (m *Vault) AppendAt(timestamp, tags, body string) error {
	when, err := time.ParseInLocation(layoutTimestamp, timestamp, time.Local)
	if err != nil {
		return fmt.Errorf("parse timestamp %q: want %s: %w", timestamp, layoutTimestamp, err)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("entry body is empty")
	}
	entry := vault.Entry{Time: when, Tags: util.NormalizeTags(strings.Split(tags, ",")), Body: body}
	if err := m.writeTarget().Append(entry); err != nil {
		return fmt.Errorf("append entry: %w", err)
	}
	return nil
}

// DayJSON returns the entries for one date as a JSON array.
func (m *Vault) DayJSON(date string) (string, error) {
	day, err := parseDate(date)
	if err != nil {
		return "", err
	}
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.ReadDay(day) })
	if err != nil {
		return "", fmt.Errorf("read day %s: %w", date, err)
	}
	return entriesJSON(sortByTime(entries))
}

// RangeJSON returns the entries between two dates inclusive as a JSON array.
func (m *Vault) RangeJSON(from, to string) (string, error) {
	start, err := parseDate(from)
	if err != nil {
		return "", err
	}
	end, err := parseDate(to)
	if err != nil {
		return "", err
	}
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.ReadRange(start, end) })
	if err != nil {
		return "", fmt.Errorf("read range %s to %s: %w", from, to, err)
	}
	return entriesJSON(sortByTime(entries))
}

// RecentJSON returns the n most recent entries as a JSON array, newest first.
func (m *Vault) RecentJSON(n int) (string, error) {
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.Recent(n) })
	if err != nil {
		return "", fmt.Errorf("read recent: %w", err)
	}
	// Each source returned its own newest n; keep the newest n of the union.
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Time.After(entries[j].Time) })
	if n >= 0 && len(entries) > n {
		entries = entries[:n]
	}
	return entriesJSON(entries)
}

// SearchJSON returns the entries whose text matches query as a JSON array.
func (m *Vault) SearchJSON(query string) (string, error) {
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.Search(query) })
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	return entriesJSON(sortByTime(entries))
}

// TagsJSON returns the tags in use with their entry counts as a JSON array of
// {tag, count}, ordered by descending count then label. A non-positive limit
// returns every tag.
//
// Counts cover this device's unsynced captures as well, so a tag that so far
// exists only in a pending entry still offers itself for browsing.
func (m *Vault) TagsJSON(limit int) (string, error) {
	totals := map[string]int{}
	sources := []*vault.Vault{m.v}
	if m.inbox != nil {
		sources = append(sources, m.inbox)
	}
	for _, src := range sources {
		counts, err := src.TagCounts(0)
		if err != nil {
			return "", fmt.Errorf("tag counts: %w", err)
		}
		for _, c := range counts {
			totals[c.Tag] += c.Count
		}
	}
	data, err := json.Marshal(util.SortedCounts(totals, limit))
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	return string(data), nil
}

// TaggedJSON returns every entry carrying the given tag as a JSON array.
func (m *Vault) TaggedJSON(tag string) (string, error) {
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.WithTag(tag) })
	if err != nil {
		return "", fmt.Errorf("read tag %s: %w", tag, err)
	}
	return entriesJSON(sortByTime(entries))
}

// FlashbackJSON returns entries from past years that fall on the given month
// and day, as a JSON array. month is 1 through 12.
func (m *Vault) FlashbackJSON(month, day int) (string, error) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return "", fmt.Errorf("invalid month %d or day %d", month, day)
	}
	entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) {
		return v.Flashback(time.Month(month), day)
	})
	if err != nil {
		return "", fmt.Errorf("flashback: %w", err)
	}
	return entriesJSON(sortByTime(entries))
}

// merged runs read against the canonical vault and this device's inbox and
// returns the combined entries, so unsynced local captures are never missing
// from what the device displays.
func (m *Vault) merged(read func(*vault.Vault) ([]vault.Entry, error)) ([]vault.Entry, error) {
	entries, err := read(m.v)
	if err != nil {
		return nil, err
	}
	if m.inbox == nil {
		return entries, nil
	}
	pending, err := read(m.inbox)
	if err != nil {
		return nil, err
	}
	return append(entries, pending...), nil
}

// sortByTime orders entries ascending by timestamp, keeping arrival order
// within a timestamp so entries written in the same second stay stable.
func sortByTime(entries []vault.Entry) []vault.Entry {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Time.Before(entries[j].Time) })
	return entries
}

// Streak returns the number of consecutive days ending today with at least one
// entry, counting this device's unsynced captures so a day journaled only on
// the phone still holds the streak.
func (m *Vault) Streak() (int, error) {
	count := 0
	// A day file can exist while holding no entries, so the day has to be read
	// rather than merely listed, matching how the vault counts a streak.
	for day := time.Now(); ; day = day.AddDate(0, 0, -1) {
		entries, err := m.merged(func(v *vault.Vault) ([]vault.Entry, error) { return v.ReadDay(day) })
		if err != nil {
			return 0, fmt.Errorf("streak: %w", err)
		}
		if len(entries) == 0 {
			return count, nil
		}
		count++
	}
}

// jsonEntry is the wire shape of one journal entry.
type jsonEntry struct {
	// Time is the entry timestamp as local wall clock without a zone offset.
	Time string `json:"time"`
	// Tags are the entry tags without leading hash characters.
	Tags []string `json:"tags,omitempty"`
	// Body is the free-form entry text.
	Body string `json:"body"`
}

// entriesJSON renders vault entries as the wire JSON array.
func entriesJSON(entries []vault.Entry) (string, error) {
	out := make([]jsonEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, jsonEntry{Time: e.Time.Format(layoutTimestamp), Tags: e.Tags, Body: e.Body})
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshal entries: %w", err)
	}
	return string(data), nil
}

// parseDate parses a wire-format calendar date in the local time zone.
func parseDate(date string) (time.Time, error) {
	day, err := time.ParseInLocation(layoutDate, date, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date %q: want YYYY-MM-DD: %w", date, err)
	}
	return day, nil
}

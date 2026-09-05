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
	v *vault.Vault
}

// Open returns a Vault rooted at dir.
// passphrase may be empty for an unencrypted vault; for an encrypted vault it
// is verified before the handle is returned so a wrong passphrase fails here
// rather than on first read.
func Open(dir, passphrase string) (*Vault, error) {
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
	return &Vault{v: v}, nil
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
	if err := m.v.Append(entry); err != nil {
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
	entries, err := m.v.ReadDay(day)
	if err != nil {
		return "", fmt.Errorf("read day %s: %w", date, err)
	}
	return entriesJSON(entries)
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
	entries, err := m.v.ReadRange(start, end)
	if err != nil {
		return "", fmt.Errorf("read range %s to %s: %w", from, to, err)
	}
	return entriesJSON(entries)
}

// RecentJSON returns the n most recent entries as a JSON array.
func (m *Vault) RecentJSON(n int) (string, error) {
	entries, err := m.v.Recent(n)
	if err != nil {
		return "", fmt.Errorf("read recent: %w", err)
	}
	return entriesJSON(entries)
}

// SearchJSON returns the entries whose text matches query as a JSON array.
func (m *Vault) SearchJSON(query string) (string, error) {
	entries, err := m.v.Search(query)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}
	return entriesJSON(entries)
}

// Streak returns the number of consecutive days ending today with at least one entry.
func (m *Vault) Streak() (int, error) {
	n, err := m.v.Streak(time.Now(), func(vault.Entry) bool { return true })
	if err != nil {
		return 0, fmt.Errorf("streak: %w", err)
	}
	return n, nil
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

// Package mobile exposes a narrow midden core surface for gomobile bind.
//
// The API is deliberately flat because gomobile restricts the types that
// cross the language boundary: strings and ints in, JSON strings out. Dates
// cross as "2006-01-02" and timestamps as RFC 3339. Entry lists are JSON
// arrays of {time, tags, body} objects.
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

// Append writes a new entry timestamped now.
// tags is comma-separated and may be empty.
func (m *Vault) Append(tags, body string) error {
	return m.AppendAt(time.Now().Format(time.RFC3339), tags, body)
}

// AppendAt writes a new entry with an explicit RFC 3339 timestamp.
// Capture flows that record offline and append later use this to keep the
// spoken time rather than the sync time.
func (m *Vault) AppendAt(timestamp, tags, body string) error {
	when, err := time.ParseInLocation(time.RFC3339, timestamp, time.Local)
	if err != nil {
		return fmt.Errorf("parse timestamp %q: %w", timestamp, err)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("entry body is empty")
	}
	entry := vault.Entry{Time: when.In(time.Local), Tags: util.NormalizeTags(strings.Split(tags, ",")), Body: body}
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
	// Time is the entry timestamp in RFC 3339.
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
		out = append(out, jsonEntry{Time: e.Time.Format(time.RFC3339), Tags: e.Tags, Body: e.Body})
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

package vault

import (
	"fmt"
	"strings"
	"time"
)

// ReadRange returns every entry across day files whose date is in the inclusive range from..to.
// Both bounds are inclusive and the slice is ordered ascending by timestamp.
func (v *Vault) ReadRange(from, to time.Time) ([]Entry, error) {
	from = dayStart(from)
	to = dayStart(to)
	if to.Before(from) {
		from, to = to, from
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range days {
		if d.Before(from) || d.After(to) {
			continue
		}
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	return out, nil
}

// Recent returns the most recent n entries across all day files.
// The slice is ordered newest first.
func (v *Vault) Recent(n int) ([]Entry, error) {
	if n <= 0 {
		return nil, fmt.Errorf("recent count must be positive")
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for i := len(days) - 1; i >= 0 && len(out) < n; i-- {
		entries, err := v.ReadDay(days[i])
		if err != nil {
			return nil, err
		}
		for j := len(entries) - 1; j >= 0 && len(out) < n; j-- {
			out = append(out, entries[j])
		}
	}
	return out, nil
}

// Search returns every entry whose body or tag list contains the given substring, case-insensitive.
// The slice is ordered ascending by timestamp.
func (v *Vault) Search(query string) ([]Entry, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, fmt.Errorf("search query is empty")
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if matches(e, q) {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// WithTag returns every entry tagged with the given label, case-insensitive.
// The slice is ordered ascending by timestamp.
func (v *Vault) WithTag(tag string) ([]Entry, error) {
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
	if tag == "" {
		return nil, fmt.Errorf("tag is empty")
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.HasTag(tag) {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// matches reports whether the entry body or tag list contains the lowercase query.
func matches(e Entry, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(e.Body), lowerQuery) {
		return true
	}
	for _, t := range e.Tags {
		if strings.Contains(strings.ToLower(t), lowerQuery) {
			return true
		}
	}
	return false
}

// dayStart truncates the timestamp to the local-midnight start of its day.
func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

package vault

import (
	"fmt"
	"sort"
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
// The slice is ordered newest first. Entries inside a day file are sorted by
// timestamp before selection because imports can append out of order; an
// entry's timestamp always falls on its file's date, so day order holds.
func (v *Vault) Recent(n int) ([]Entry, error) {
	return v.RecentBefore(n, time.Time{})
}

// RecentBefore returns the most recent n entries at or before the cutoff,
// newest first. A zero cutoff includes everything.
//
// The cutoff exists because a vault holding an imported calendar contains
// appointments that have not happened yet. Without it "the most recent entry"
// means the furthest one in the future, so a vault backfilled in August reports
// next March's dentist appointment as the latest thing in the record.
func (v *Vault) RecentBefore(n int, cutoff time.Time) ([]Entry, error) {
	if n <= 0 {
		return nil, fmt.Errorf("recent count must be positive")
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for i := len(days) - 1; i >= 0 && len(out) < n; i-- {
		if !cutoff.IsZero() && days[i].After(cutoff) {
			continue
		}
		entries, err := v.ReadDay(days[i])
		if err != nil {
			return nil, err
		}
		sort.SliceStable(entries, func(a, b int) bool { return entries[a].Time.Before(entries[b].Time) })
		for j := len(entries) - 1; j >= 0 && len(out) < n; j-- {
			if !cutoff.IsZero() && entries[j].Time.After(cutoff) {
				continue
			}
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

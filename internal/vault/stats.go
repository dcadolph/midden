package vault

import (
	"fmt"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/util"
)

// Stats is a summary of the vault contents.
type Stats struct {
	// Days is the number of day files that contain at least one entry.
	Days int `json:"days"`
	// Entries is the number of entries across the vault.
	Entries int `json:"entries"`
	// Words is the total whitespace-separated word count across entry bodies.
	Words int `json:"words"`
	// Tags is the number of distinct tag labels in use.
	Tags int `json:"tags"`
	// FirstEntry is the timestamp of the earliest entry, or the zero time when the vault is empty.
	FirstEntry time.Time `json:"first_entry"`
	// LastEntry is the timestamp of the latest entry, or the zero time when the vault is empty.
	LastEntry time.Time `json:"last_entry"`
	// TopTags is the tag histogram ordered by descending count then label.
	TopTags []util.TagCount `json:"top_tags,omitempty"`
	// Scheduled is the number of entries dated after the observation time. An
	// imported calendar carries appointments that have not happened yet, and
	// counting them among what the record holds overstates it.
	Scheduled int `json:"scheduled"`
	// LastPast is the latest entry at or before the observation time, which is
	// what a person means by the most recent thing in the record.
	LastPast time.Time `json:"last_past"`
}

// ComputeStats walks every entry once and assembles the summary.
// TopTags is capped at the given limit; pass a non-positive limit to include
// every tag. Entries dated after now are counted separately as scheduled rather
// than folded into the record's span, so an imported calendar does not make the
// vault appear to run into next year.
func (v *Vault) ComputeStats(topTagLimit int, now time.Time) (Stats, error) {
	var s Stats
	days, err := v.ListDays()
	if err != nil {
		return Stats{}, err
	}
	tagCounts := map[string]int{}
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return Stats{}, err
		}
		if len(entries) == 0 {
			continue
		}
		s.Days++
		for _, e := range entries {
			s.Entries++
			s.Words += countWords(e.Body)
			if s.FirstEntry.IsZero() || e.Time.Before(s.FirstEntry) {
				s.FirstEntry = e.Time
			}
			if e.Time.After(s.LastEntry) {
				s.LastEntry = e.Time
			}
			switch {
			case !now.IsZero() && e.Time.After(now):
				s.Scheduled++
			case e.Time.After(s.LastPast):
				s.LastPast = e.Time
			}
			for _, t := range e.Tags {
				tagCounts[strings.ToLower(t)]++
			}
		}
	}
	s.Tags = len(tagCounts)
	s.TopTags = util.SortedCounts(tagCounts, topTagLimit)
	return s, nil
}

// TagCounts returns every distinct tag with its entry count, ordered by descending count then label.
// Pass a non-positive limit to include every tag.
func (v *Vault) TagCounts(limit int) ([]util.TagCount, error) {
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	tagCounts := map[string]int{}
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			for _, t := range e.Tags {
				tagCounts[strings.ToLower(t)]++
			}
		}
	}
	return util.SortedCounts(tagCounts, limit), nil
}

// Streak returns the number of consecutive days ending today on which at least one entry was written.
// A day with no entry breaks the streak, including today.
func (v *Vault) Streak(today time.Time) (int, error) {
	today = dayStart(today)
	days, err := v.ListDays()
	if err != nil {
		return 0, err
	}
	have := map[string]bool{}
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return 0, err
		}
		if len(entries) > 0 {
			have[dayKey(d)] = true
		}
	}
	streak := 0
	cursor := today
	for have[dayKey(cursor)] {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak, nil
}

// Flashback returns every entry across past years whose month and day match the given calendar date.
// The slice is ordered ascending by timestamp.
func (v *Vault) Flashback(month time.Month, day int) ([]Entry, error) {
	if month < time.January || month > time.December || day < 1 || day > 31 {
		return nil, fmt.Errorf("invalid month or day")
	}
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range days {
		if d.Month() != month || d.Day() != day {
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

// countWords returns the number of whitespace-separated tokens in s.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// dayKey formats a date as YYYY-MM-DD for use as a map key.
func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

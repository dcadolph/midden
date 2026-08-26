package weave

import (
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// monthLayout renders a calendar month as YYYY-MM.
const monthLayout = "2006-01"

// Period is one calendar month with a count per source.
type Period struct {
	// Month is the calendar month in YYYY-MM form.
	Month string
	// Counts holds entries per source tag inside the month.
	Counts map[string]int
	// Total is the number of entries in the month across every source.
	Total int
}

// Rhythm returns the record month by month, counted per source. A person knows
// what they did in a given month but not how the balance between the parts of
// their life shifted across years, because that comparison spans more time than
// memory holds at once.
func Rhythm(entries []vault.Entry, sources []string, now time.Time) []Period {
	byMonth := map[string]map[string]int{}
	for _, e := range entries {
		if !now.IsZero() && e.Time.After(now) {
			continue
		}
		src := sourceOf(e, sources)
		if src == "" {
			continue
		}
		m := e.Time.Format(monthLayout)
		if byMonth[m] == nil {
			byMonth[m] = map[string]int{}
		}
		byMonth[m][src]++
	}
	out := make([]Period, 0, len(byMonth))
	for m, counts := range byMonth {
		total := 0
		for _, n := range counts {
			total += n
		}
		out = append(out, Period{Month: m, Counts: counts, Total: total})
	}
	// The layout is zero-padded, so lexical order is calendar order.
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	return out
}

// sourceOf returns which of the given source tags an entry carries, or empty
// when it carries none. Sources are checked in order, so the caller controls
// precedence when an entry carries more than one.
func sourceOf(e vault.Entry, sources []string) string {
	for _, want := range sources {
		for _, t := range e.Tags {
			if strings.EqualFold(t, want) {
				return want
			}
		}
	}
	return ""
}

// Overlap is a day on which more than one source recorded something. A single
// source describes one part of a life; the days where two of them meet are the
// only place the record can say something neither source knows alone.
type Overlap struct {
	// Day is the calendar date.
	Day time.Time
	// Counts holds entries per source on that day.
	Counts map[string]int
	// Headlines holds one representative entry headline per source.
	Headlines map[string]string
}

// Overlaps returns the days where every one of the given sources recorded
// something, ordered by how much was recorded, heaviest first.
func Overlaps(entries []vault.Entry, sources []string, now time.Time) []Overlap {
	type bucket struct {
		counts    map[string]int
		headlines map[string]string
		day       time.Time
	}
	byDay := map[string]*bucket{}
	for _, e := range entries {
		if !now.IsZero() && e.Time.After(now) {
			continue
		}
		src := sourceOf(e, sources)
		if src == "" {
			continue
		}
		key := e.Time.Format("2006-01-02")
		b := byDay[key]
		if b == nil {
			b = &bucket{
				counts:    map[string]int{},
				headlines: map[string]string{},
				day:       time.Date(e.Time.Year(), e.Time.Month(), e.Time.Day(), 0, 0, 0, 0, e.Time.Location()),
			}
			byDay[key] = b
		}
		b.counts[src]++
		if _, ok := b.headlines[src]; !ok {
			b.headlines[src] = headline(e.Body)
		}
	}
	var out []Overlap
	for _, b := range byDay {
		complete := true
		for _, s := range sources {
			if b.counts[s] == 0 {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		out = append(out, Overlap{Day: b.day, Counts: b.counts, Headlines: b.headlines})
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := 0, 0
		for _, n := range out[i].Counts {
			ti += n
		}
		for _, n := range out[j].Counts {
			tj += n
		}
		if ti != tj {
			return ti > tj
		}
		return out[i].Day.Before(out[j].Day)
	})
	return out
}

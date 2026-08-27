package weave

import (
	"sort"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// Gap is a stretch where the record went quiet relative to the activity around
// it. A thread ending is one commitment stopping; a gap is the record itself
// falling silent, which is a different and usually larger thing. It is also
// invisible from the inside: a person notices that a class ended, but never
// that a whole span of years went unrecorded.
type Gap struct {
	// Source is the source tag whose record fell silent, or empty when the
	// silence spans the whole record. The distinction matters because one loud
	// source can flood the months where another went quiet: commits pouring in
	// during years the calendar recorded nothing would otherwise hide exactly
	// the silence worth asking about.
	Source string
	// From is the first day of the first quiet month.
	From time.Time
	// To is the last day of the last quiet month.
	To time.Time
	// Months is how long the silence ran.
	Months int
	// Entries is how many entries fall inside it.
	Entries int
	// Before and After are entries per month in the active stretches on either
	// side, which is what makes the quiet legible as a departure.
	Before float64
	After  float64
}

// GapOptions tune silence detection.
type GapOptions struct {
	// Now is the observation date. Months after it are ignored, since a record
	// cannot be silent about a future that has not happened.
	Now time.Time
	// MinMonths is the shortest silence worth reporting.
	MinMonths int
	// QuietFraction is the share of the record's usual monthly volume at or
	// below which a month counts as quiet.
	QuietFraction float64
}

// DefaultGapOptions returns silence settings suited to a personal record.
func DefaultGapOptions(now time.Time) GapOptions {
	return GapOptions{Now: now, MinMonths: 6, QuietFraction: 0.15}
}

// GapsBySource finds interior silences in the whole record and inside each
// source separately, longest first. A silence found in the whole record is not
// repeated per source.
func GapsBySource(entries []vault.Entry, sources []string, opts GapOptions) []Gap {
	out := Gaps(entries, opts)
	covered := func(g Gap) bool {
		for _, w := range out {
			if w.Source == "" && !g.From.Before(w.From) && !g.To.After(w.To) {
				return true
			}
		}
		return false
	}
	for _, src := range sources {
		var subset []vault.Entry
		for _, e := range entries {
			if sourceOf(e, []string{src}) != "" {
				subset = append(subset, e)
			}
		}
		for _, g := range Gaps(subset, opts) {
			g.Source = src
			if !covered(g) {
				out = append(out, g)
			}
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Months != out[b].Months {
			return out[a].Months > out[b].Months
		}
		return out[a].From.Before(out[b].From)
	})
	return out
}

// Gaps finds the interior silences in a record, longest first.
//
// Only interior silences count. A record is quiet before it starts and after it
// ends by definition, and reporting those as holes would say nothing about the
// life, so a gap is required to have activity on both sides.
func Gaps(entries []vault.Entry, opts GapOptions) []Gap {
	if opts.MinMonths < 1 {
		opts.MinMonths = 1
	}
	months, first, last := monthCounts(entries, opts.Now)
	if len(months) == 0 || !first.Before(last) {
		return nil
	}
	series := monthSeries(first, last)
	if len(series) < opts.MinMonths+2 {
		return nil
	}
	threshold := quietThreshold(months, series, opts.QuietFraction)

	var out []Gap
	i := 0
	for i < len(series) {
		if months[key(series[i])] > threshold {
			i++
			continue
		}
		j := i
		for j < len(series) && months[key(series[j])] <= threshold {
			j++
		}
		// Interior only: something has to come before and after the silence.
		if i > 0 && j < len(series) && j-i >= opts.MinMonths {
			out = append(out, buildGap(series, months, i, j))
		}
		i = j
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Months != out[b].Months {
			return out[a].Months > out[b].Months
		}
		return out[a].From.Before(out[b].From)
	})
	return out
}

// buildGap assembles the gap spanning series[i:j].
func buildGap(series []time.Time, months map[string]int, i, j int) Gap {
	start := series[i]
	end := series[j-1].AddDate(0, 1, -1)
	inside := 0
	for k := i; k < j; k++ {
		inside += months[key(series[k])]
	}
	return Gap{
		From:    start,
		To:      end,
		Months:  j - i,
		Entries: inside,
		Before:  rate(series, months, 0, i),
		After:   rate(series, months, j, len(series)),
	}
}

// rate returns entries per month across series[from:to].
func rate(series []time.Time, months map[string]int, from, to int) float64 {
	if to <= from {
		return 0
	}
	total := 0
	for k := from; k < to; k++ {
		total += months[key(series[k])]
	}
	return float64(total) / float64(to-from)
}

// quietThreshold returns the monthly count at or below which a month reads as
// quiet. It is a fraction of the median active month rather than of the mean,
// because a single explosive month would otherwise drag the bar high enough to
// call ordinary months silent.
func quietThreshold(months map[string]int, series []time.Time, fraction float64) int {
	var active []int
	for _, m := range series {
		if n := months[key(m)]; n > 0 {
			active = append(active, n)
		}
	}
	if len(active) == 0 {
		return 0
	}
	sort.Ints(active)
	median := active[len(active)/2]
	if fraction <= 0 {
		return 0
	}
	t := int(float64(median) * fraction)
	if t < 1 {
		t = 1
	}
	return t
}

// monthCounts buckets entries by month and returns the bounding months.
func monthCounts(entries []vault.Entry, now time.Time) (map[string]int, time.Time, time.Time) {
	out := map[string]int{}
	var first, last time.Time
	for _, e := range entries {
		if !now.IsZero() && e.Time.After(now) {
			continue
		}
		m := monthOf(e.Time)
		out[key(m)]++
		if first.IsZero() || m.Before(first) {
			first = m
		}
		if m.After(last) {
			last = m
		}
	}
	return out, first, last
}

// monthSeries returns every month from first to last inclusive, including the
// empty ones, which are the whole point.
func monthSeries(first, last time.Time) []time.Time {
	var out []time.Time
	for m := first; !m.After(last); m = m.AddDate(0, 1, 0) {
		out = append(out, m)
	}
	return out
}

// monthOf truncates a timestamp to the first day of its month.
func monthOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// key renders a month as its bucket key.
func key(m time.Time) string {
	return m.Format(monthLayout)
}

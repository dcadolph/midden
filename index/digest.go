package index

import (
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/util"
)

// monthLayout renders a calendar month as YYYY-MM.
const monthLayout = "2006-01"

// Digest is a whole-corpus summary computed from counts over every indexed
// entry. Nearest-neighbor retrieval answers questions about a specific memory
// but cannot answer questions about the record as a whole, so aggregate shape
// is measured here and supplied alongside whatever entries were retrieved.
type Digest struct {
	// Entries is the number of indexed entries inside the range.
	Entries int `json:"entries"`
	// First is the earliest entry timestamp in range, zero when the range is empty.
	First time.Time `json:"first,omitempty"`
	// Last is the latest entry timestamp in range, zero when the range is empty.
	Last time.Time `json:"last,omitempty"`
	// TopTags is the tag histogram ordered by descending count then label.
	TopTags []util.TagCount `json:"top_tags,omitempty"`
	// Months is the per-month entry histogram ordered by ascending month.
	Months []MonthCount `json:"months,omitempty"`
}

// MonthCount pairs a calendar month with the number of entries it holds.
type MonthCount struct {
	// Month is the calendar month in YYYY-MM form.
	Month string `json:"month"`
	// Count is the number of entries timestamped inside the month.
	Count int `json:"count"`
}

// Digest summarizes every indexed entry inside the range. A zero from or to
// leaves that end of the range open. TopTags is capped at topTags; a
// non-positive value includes every tag.
func (i *Index) Digest(from, to time.Time, topTags int) Digest {
	var d Digest
	tags := map[string]int{}
	months := map[string]int{}
	for _, e := range i.Entries {
		if !inRange(e.Time, from, to) {
			continue
		}
		d.Entries++
		if d.First.IsZero() || e.Time.Before(d.First) {
			d.First = e.Time
		}
		if e.Time.After(d.Last) {
			d.Last = e.Time
		}
		for _, t := range e.Tags {
			tags[strings.ToLower(t)]++
		}
		months[e.Time.Format(monthLayout)]++
	}
	d.TopTags = util.SortedCounts(tags, topTags)
	d.Months = sortedMonths(months)
	return d
}

// sortedMonths renders the month histogram in calendar order. The YYYY-MM
// layout is zero-padded, so ascending lexical order is ascending calendar order.
func sortedMonths(counts map[string]int) []MonthCount {
	out := make([]MonthCount, 0, len(counts))
	for month, n := range counts {
		out = append(out, MonthCount{Month: month, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	return out
}

package cmd

import (
	"fmt"
	"time"

	"github.com/dcadolph/midden/dateutil"
)

// dateRange bounds a query to a span of days. A zero bound is open.
type dateRange struct {
	// From is the inclusive start of the range at local midnight.
	From time.Time
	// To is the inclusive end of the range at the last instant of that day.
	To time.Time
}

// Bounded reports whether either end of the range is set.
func (r dateRange) Bounded() bool {
	return !r.From.IsZero() || !r.To.IsZero()
}

// Label renders the range for human-readable output.
func (r dateRange) Label() string {
	switch {
	case !r.Bounded():
		return "whole vault"
	case r.From.IsZero():
		return "through " + r.To.Format(layoutDate)
	case r.To.IsZero():
		return "since " + r.From.Format(layoutDate)
	}
	return r.From.Format(layoutDate) + " to " + r.To.Format(layoutDate)
}

// resolveDateRange parses the since and until flag values. Both accept every
// form dateutil understands, so "2026-08-01", "30-days-ago", and "monday" are
// equivalent kinds of input. The until bound extends to the end of its day so a
// single-day range still covers that day's entries.
func resolveDateRange(since, until string) (dateRange, error) {
	var r dateRange
	if since != "" {
		from, err := dateutil.Parse(since)
		if err != nil {
			return dateRange{}, fmt.Errorf("parse --since %q: %w", since, err)
		}
		r.From = from
	}
	if until != "" {
		to, err := dateutil.Parse(until)
		if err != nil {
			return dateRange{}, fmt.Errorf("parse --until %q: %w", until, err)
		}
		r.To = to.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	if !r.From.IsZero() && !r.To.IsZero() && r.To.Before(r.From) {
		return dateRange{}, fmt.Errorf("--until %q is before --since %q", until, since)
	}
	return r, nil
}

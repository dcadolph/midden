package ics

import (
	"sort"
	"time"
)

// ExpandReport records what expansion could not do faithfully, so a caller can
// say so rather than presenting a partial calendar as a complete one.
type ExpandReport struct {
	// Occurrences is the number of events produced from recurring series.
	Occurrences int
	// Unexpanded is the number of recurring events whose rule midden does not
	// expand, each of which contributes only its first occurrence.
	Unexpanded int
	// Truncated is the number of series whose expansion hit the period cap and
	// may therefore be missing later occurrences.
	Truncated int
	// Excluded is the number of generated occurrences dropped by EXDATE.
	Excluded int
	// Overridden is the number of generated occurrences replaced by an explicit
	// override event carrying the same recurrence identifier.
	Overridden int
}

// Expand turns parsed events into the concrete occurrences falling inside the
// window, ordered by start time. A recurring event becomes one event per
// occurrence with its start and end shifted and its UID preserved, so a series
// contributes every time it actually happened rather than only the first. A
// zero from or to leaves that end of the window open, though a rule bounded by
// neither the window nor its own COUNT or UNTIL is reported as truncated rather
// than walked forever.
func Expand(events []Event, from, to time.Time) ([]Event, ExpandReport) {
	var report ExpandReport
	overrides := overrideIndex(events)
	var out []Event
	for _, e := range events {
		// An override is already a standalone event; it is emitted on its own
		// terms and suppresses the occurrence it replaces.
		if !e.RecurrenceID.IsZero() || !e.Recurs() {
			if within(e.Start, from, to) {
				out = append(out, e)
			}
			continue
		}
		if e.Rule == nil {
			report.Unexpanded++
			if within(e.Start, from, to) {
				out = append(out, e)
			}
			continue
		}
		starts, truncated := e.Rule.Occurrences(e.Start, from, to)
		if truncated {
			report.Truncated++
		}
		for _, start := range starts {
			if excluded(e.ExDates, start) {
				report.Excluded++
				continue
			}
			if overrides[e.UID][start.UnixNano()] {
				report.Overridden++
				continue
			}
			report.Occurrences++
			out = append(out, occurrence(e, start))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, report
}

// occurrence returns a copy of the series event moved to the given start,
// holding its duration. The rule is cleared because the copy is one concrete
// instant, not a series that could be expanded again.
func occurrence(e Event, start time.Time) Event {
	o := e
	o.RawRule = ""
	o.Rule = nil
	o.ExDates = nil
	o.RecurrenceID = e.Start
	o.Start = start
	if d := e.Duration(); d > 0 {
		o.End = start.Add(d)
	} else {
		o.End = time.Time{}
	}
	return o
}

// overrideIndex maps each series UID to the occurrence starts that an explicit
// override event replaces, keyed by instant so an occurrence is matched however
// its zone was written.
func overrideIndex(events []Event) map[string]map[int64]bool {
	out := map[string]map[int64]bool{}
	for _, e := range events {
		if e.RecurrenceID.IsZero() || e.UID == "" {
			continue
		}
		if out[e.UID] == nil {
			out[e.UID] = map[int64]bool{}
		}
		out[e.UID][e.RecurrenceID.UnixNano()] = true
	}
	return out
}

// excluded reports whether the start matches an EXDATE value.
func excluded(exDates []time.Time, start time.Time) bool {
	for _, x := range exDates {
		if x.Equal(start) {
			return true
		}
	}
	return false
}

// within reports whether t falls inside the closed window, treating a zero
// bound as open.
func within(t, from, to time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
}

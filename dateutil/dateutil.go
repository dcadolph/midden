// Package dateutil parses the relative and absolute date strings accepted by the midden CLI.
package dateutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// agoRe matches the "<n>-<unit>-ago" pattern such as "3-days-ago" and "1-week-ago".
var agoRe = regexp.MustCompile(`^(\d+)-(day|days|week|weeks|month|months|year|years)-ago$`)

// Parse converts the given string into a local-midnight time.Time.
// It accepts canonical YYYY-MM-DD, the literals "today" and "yesterday",
// weekday names ("monday", "last-monday"), and "<n>-<unit>-ago" patterns.
// All values are anchored to the current local day.
func Parse(s string) (time.Time, error) {
	return parseAt(s, time.Now())
}

// parseAt is Parse with the reference time injected for deterministic tests.
func parseAt(s string, ref time.Time) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return time.Time{}, fmt.Errorf("date is empty")
	}
	today := startOfDay(ref)
	switch s {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}
	if d, ok := tryAbsolute(s); ok {
		return d, nil
	}
	if d, ok := tryWeekday(s, today); ok {
		return d, nil
	}
	if d, ok := tryAgo(s, today); ok {
		return d, nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD, today, yesterday, weekday name, or N-units-ago", s)
}

// startOfDay returns the local-midnight start of the day containing t.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// tryAbsolute parses a YYYY-MM-DD date.
func tryAbsolute(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// tryWeekday parses a weekday name with optional "last-" prefix, anchored to the reference date.
func tryWeekday(s string, today time.Time) (time.Time, bool) {
	last := false
	if strings.HasPrefix(s, "last-") {
		last = true
		s = strings.TrimPrefix(s, "last-")
	}
	wd, ok := weekday(s)
	if !ok {
		return time.Time{}, false
	}
	delta := int(today.Weekday()) - int(wd)
	if last {
		if delta <= 0 {
			delta += 7
		}
	} else if delta < 0 {
		delta += 7
	}
	return today.AddDate(0, 0, -delta), true
}

// tryAgo parses the "<n>-<unit>-ago" pattern.
func tryAgo(s string, today time.Time) (time.Time, bool) {
	m := agoRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 {
		return time.Time{}, false
	}
	switch m[2] {
	case "day", "days":
		return today.AddDate(0, 0, -n), true
	case "week", "weeks":
		return today.AddDate(0, 0, -7*n), true
	case "month", "months":
		return addMonthsClamped(today, -n), true
	case "year", "years":
		return addMonthsClamped(today, -12*n), true
	}
	return time.Time{}, false
}

// addMonthsClamped shifts t by delta months, clamping the day of month to the
// last day of the target month so overflow never spills into the following
// month (Mar 31 minus one month yields Feb 28, not Mar 3).
func addMonthsClamped(t time.Time, delta int) time.Time {
	first := time.Date(t.Year(), time.Month(int(t.Month())+delta), 1, 0, 0, 0, 0, t.Location())
	day := t.Day()
	if last := daysInMonth(first); day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, t.Location())
}

// daysInMonth returns the number of days in the month containing t.
func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// weekday converts a lowercase day name into a time.Weekday.
func weekday(name string) (time.Weekday, bool) {
	switch name {
	case "sunday", "sun":
		return time.Sunday, true
	case "monday", "mon":
		return time.Monday, true
	case "tuesday", "tue", "tues":
		return time.Tuesday, true
	case "wednesday", "wed":
		return time.Wednesday, true
	case "thursday", "thu", "thur", "thurs":
		return time.Thursday, true
	case "friday", "fri":
		return time.Friday, true
	case "saturday", "sat":
		return time.Saturday, true
	}
	return 0, false
}

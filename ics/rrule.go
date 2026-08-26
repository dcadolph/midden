package ics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxPeriods bounds how many periods a single rule is walked before the
// expansion gives up. A rule with no COUNT or UNTIL is bounded only by the
// requested window, so a pathological one would otherwise walk forever.
const maxPeriods = 100000

// Frequency is the RRULE FREQ value. Only the frequencies personal calendars
// actually use are expanded; anything else is left unexpanded rather than
// guessed at.
type Frequency string

// The expanded frequencies.
const (
	Daily   Frequency = "DAILY"
	Weekly  Frequency = "WEEKLY"
	Monthly Frequency = "MONTHLY"
	Yearly  Frequency = "YEARLY"
)

// WeekDayNum is one BYDAY entry: a weekday with an optional ordinal that
// selects, for example, the second Tuesday or the last Friday of a month.
type WeekDayNum struct {
	// Day is the weekday the entry selects.
	Day time.Weekday
	// Ordinal positions the weekday within the period, counting from the end
	// when negative. Zero means every matching weekday.
	Ordinal int
}

// Recurrence is the RRULE subset midden expands. BYSETPOS, BYYEARDAY, and
// BYWEEKNO are not supported; a rule using them still expands on its remaining
// parts rather than being dropped, so the result may hold more occurrences than
// the calendar shows.
type Recurrence struct {
	// Freq is the base frequency of the rule.
	Freq Frequency
	// Interval is the number of periods between occurrences, at least one.
	Interval int
	// Count caps the number of occurrences generated from DTSTART, or zero for
	// no cap.
	Count int
	// Until is the inclusive last instant an occurrence may fall on, or the
	// zero time for no bound.
	Until time.Time
	// ByDay restricts or selects weekdays within each period.
	ByDay []WeekDayNum
	// ByMonthDay restricts days of the month, counting from the end when negative.
	ByMonthDay []int
	// ByMonth restricts which calendar months may hold occurrences.
	ByMonth []time.Month
	// WeekStart is the first day of the week, which decides week boundaries for
	// a weekly rule with an interval above one.
	WeekStart time.Weekday
}

// ParseRRULE parses an RRULE property value. An unrecognized or absent FREQ is
// an error, because expanding a rule whose base frequency is unknown would
// invent occurrences rather than omit them.
func ParseRRULE(value string) (Recurrence, error) {
	r := Recurrence{Interval: 1, WeekStart: time.Monday}
	for part := range strings.SplitSeq(value, ";") {
		key, val, ok := strings.Cut(part, "=")
		if !ok || val == "" {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(key)) {
		case "FREQ":
			r.Freq = Frequency(strings.ToUpper(val))
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return Recurrence{}, fmt.Errorf("rrule interval %q: not a positive number", val)
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return Recurrence{}, fmt.Errorf("rrule count %q: not a positive number", val)
			}
			r.Count = n
		case "UNTIL":
			t, _, ok := parseTime(val, "")
			if !ok {
				return Recurrence{}, fmt.Errorf("rrule until %q: unparseable time", val)
			}
			r.Until = t
		case "BYDAY":
			days, err := parseByDay(val)
			if err != nil {
				return Recurrence{}, err
			}
			r.ByDay = days
		case "BYMONTHDAY":
			days, err := parseInts(val, "bymonthday")
			if err != nil {
				return Recurrence{}, err
			}
			r.ByMonthDay = days
		case "BYMONTH":
			months, err := parseInts(val, "bymonth")
			if err != nil {
				return Recurrence{}, err
			}
			for _, m := range months {
				if m < 1 || m > 12 {
					return Recurrence{}, fmt.Errorf("rrule bymonth %d: out of range", m)
				}
				r.ByMonth = append(r.ByMonth, time.Month(m))
			}
		case "WKST":
			d, ok := parseWeekday(val)
			if !ok {
				return Recurrence{}, fmt.Errorf("rrule wkst %q: not a weekday", val)
			}
			r.WeekStart = d
		}
	}
	switch r.Freq {
	case Daily, Weekly, Monthly, Yearly:
		return r, nil
	case "":
		return Recurrence{}, fmt.Errorf("rrule has no freq")
	}
	return Recurrence{}, fmt.Errorf("rrule freq %q: not expanded", r.Freq)
}

// Occurrences returns every start time the rule generates for dtstart that
// falls inside the window, ordered ascending, and reports whether the walk was
// cut short by the period cap. Generation always begins at dtstart so COUNT is
// applied to the real series rather than to the part of it inside the window. A
// zero to leaves the window unbounded on the far side, which only terminates
// when the rule itself does.
func (r Recurrence) Occurrences(dtstart, from, to time.Time) ([]time.Time, bool) {
	interval := r.Interval
	if interval < 1 {
		interval = 1
	}
	end := to
	if !r.Until.IsZero() && (end.IsZero() || r.Until.Before(end)) {
		end = r.Until
	}
	if end.IsZero() && r.Count == 0 {
		// Nothing bounds the walk, so refuse rather than spin.
		return nil, true
	}

	var out []time.Time
	generated := 0
	period := r.periodAnchor(dtstart)
	for guard := 0; ; guard++ {
		if guard >= maxPeriods {
			return out, true
		}
		if !end.IsZero() && period.After(end) {
			break
		}
		for _, c := range r.candidates(period, dtstart) {
			if c.Before(dtstart) {
				continue
			}
			if !end.IsZero() && c.After(end) {
				continue
			}
			generated++
			if r.Count > 0 && generated > r.Count {
				return out, false
			}
			if from.IsZero() || !c.Before(from) {
				out = append(out, c)
			}
		}
		if r.Count > 0 && generated >= r.Count {
			break
		}
		period = r.advance(period, interval)
	}
	return out, false
}

// periodAnchor normalizes dtstart to the start of the period containing it, so
// stepping by interval never has to clamp a day of month that the next period
// does not have.
func (r Recurrence) periodAnchor(dtstart time.Time) time.Time {
	y, m, d := dtstart.Date()
	loc := dtstart.Location()
	switch r.Freq {
	case Weekly:
		day := time.Date(y, m, d, 0, 0, 0, 0, loc)
		back := (int(day.Weekday()) - int(r.WeekStart) + 7) % 7
		return day.AddDate(0, 0, -back)
	case Monthly:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)
	case Yearly:
		return time.Date(y, time.January, 1, 0, 0, 0, 0, loc)
	default:
		return time.Date(y, m, d, 0, 0, 0, 0, loc)
	}
}

// advance steps the period anchor forward by interval periods.
func (r Recurrence) advance(period time.Time, interval int) time.Time {
	switch r.Freq {
	case Weekly:
		return period.AddDate(0, 0, 7*interval)
	case Monthly:
		return addMonths(period, interval)
	case Yearly:
		return time.Date(period.Year()+interval, time.January, 1, 0, 0, 0, 0, period.Location())
	default:
		return period.AddDate(0, 0, interval)
	}
}

// candidates returns the occurrence times the rule produces inside the period
// beginning at the anchor, ordered ascending and carrying dtstart's clock time.
func (r Recurrence) candidates(period, dtstart time.Time) []time.Time {
	var days []time.Time
	switch r.Freq {
	case Daily:
		days = []time.Time{period}
	case Weekly:
		days = r.weeklyDays(period, dtstart)
	case Monthly:
		days = r.monthDays(period.Year(), period.Month(), dtstart)
	case Yearly:
		days = r.yearlyDays(period.Year(), dtstart)
	}
	out := make([]time.Time, 0, len(days))
	for _, d := range days {
		if !r.allows(d) {
			continue
		}
		out = append(out, withClock(d, dtstart))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

// weeklyDays returns the days a weekly rule selects inside the week beginning
// at the anchor, defaulting to dtstart's own weekday when BYDAY is absent.
func (r Recurrence) weeklyDays(period, dtstart time.Time) []time.Time {
	wanted := map[time.Weekday]bool{}
	if len(r.ByDay) == 0 {
		wanted[dtstart.Weekday()] = true
	}
	for _, wd := range r.ByDay {
		wanted[wd.Day] = true
	}
	var out []time.Time
	for i := range 7 {
		d := period.AddDate(0, 0, i)
		if wanted[d.Weekday()] {
			out = append(out, d)
		}
	}
	return out
}

// monthDays returns the days a monthly or yearly rule selects inside one month.
// BYDAY ordinals count from the end of the month when negative. Giving both
// BYDAY and BYMONTHDAY narrows to the days satisfying both, which is what makes
// FREQ=MONTHLY;BYDAY=FR;BYMONTHDAY=13 mean Friday the thirteenth rather than
// every Friday plus every thirteenth. With neither part the rule falls back to
// dtstart's day of month, which months too short to hold it simply skip.
func (r Recurrence) monthDays(year int, month time.Month, dtstart time.Time) []time.Time {
	loc := dtstart.Location()
	last := daysInMonth(year, month)
	var selected map[int]bool
	switch {
	case len(r.ByMonthDay) > 0 && len(r.ByDay) > 0:
		selected = intersectDays(r.monthDayNumbers(last), r.weekdayNumbers(year, month, loc, last))
	case len(r.ByMonthDay) > 0:
		selected = r.monthDayNumbers(last)
	case len(r.ByDay) > 0:
		selected = r.weekdayNumbers(year, month, loc, last)
	default:
		selected = map[int]bool{dtstart.Day(): true}
	}
	out := make([]time.Time, 0, len(selected))
	for day := 1; day <= last; day++ {
		if selected[day] {
			out = append(out, time.Date(year, month, day, 0, 0, 0, 0, loc))
		}
	}
	return out
}

// monthDayNumbers returns the days of month BYMONTHDAY selects, resolving
// negative values against the length of the month.
func (r Recurrence) monthDayNumbers(last int) map[int]bool {
	out := map[int]bool{}
	for _, d := range r.ByMonthDay {
		if d < 0 {
			d = last + 1 + d
		}
		if d >= 1 && d <= last {
			out[d] = true
		}
	}
	return out
}

// weekdayNumbers returns the days of month BYDAY selects, resolving ordinals
// against the weekdays present in that month.
func (r Recurrence) weekdayNumbers(year int, month time.Month, loc *time.Location, last int) map[int]bool {
	out := map[int]bool{}
	for _, wd := range r.ByDay {
		if wd.Ordinal != 0 {
			if day := nthWeekday(year, month, wd.Day, wd.Ordinal, loc); day != 0 {
				out[day] = true
			}
			continue
		}
		for day := 1; day <= last; day++ {
			if time.Date(year, month, day, 0, 0, 0, 0, loc).Weekday() == wd.Day {
				out[day] = true
			}
		}
	}
	return out
}

// intersectDays returns the days present in both sets.
func intersectDays(a, b map[int]bool) map[int]bool {
	out := map[int]bool{}
	for day := range a {
		if b[day] {
			out[day] = true
		}
	}
	return out
}

// yearlyDays returns the days a yearly rule selects inside one year, defaulting
// to dtstart's own month when BYMONTH is absent.
func (r Recurrence) yearlyDays(year int, dtstart time.Time) []time.Time {
	months := r.ByMonth
	if len(months) == 0 {
		months = []time.Month{dtstart.Month()}
	}
	var out []time.Time
	for _, m := range months {
		out = append(out, r.monthDays(year, m, dtstart)...)
	}
	return out
}

// allows applies the BY parts that act as filters rather than as generators for
// the rule's frequency. A weekly rule already generated its weekdays from
// BYDAY, and a monthly or yearly rule already generated its days, so applying
// those parts again here would be a no-op at best.
func (r Recurrence) allows(day time.Time) bool {
	if len(r.ByMonth) > 0 && !containsMonth(r.ByMonth, day.Month()) {
		return false
	}
	if r.Freq != Daily {
		return true
	}
	if len(r.ByDay) > 0 && !containsWeekday(r.ByDay, day.Weekday()) {
		return false
	}
	if len(r.ByMonthDay) > 0 {
		last := daysInMonth(day.Year(), day.Month())
		if !containsInt(r.ByMonthDay, day.Day()) && !containsInt(r.ByMonthDay, day.Day()-last-1) {
			return false
		}
	}
	return true
}

// withClock returns the day carrying dtstart's wall-clock time. Wall clock is
// preserved rather than elapsed time so an event stays at the hour the calendar
// shows it across a daylight-saving shift.
func withClock(day, dtstart time.Time) time.Time {
	h, m, s := dtstart.Clock()
	y, mo, d := day.Date()
	return time.Date(y, mo, d, h, m, s, 0, dtstart.Location())
}

// nthWeekday returns the day of month of the ordinal-th given weekday, counting
// from the end of the month when the ordinal is negative, or zero when the
// month has no such day.
func nthWeekday(year int, month time.Month, want time.Weekday, ordinal int, loc *time.Location) int {
	last := daysInMonth(year, month)
	var days []int
	for day := 1; day <= last; day++ {
		if time.Date(year, month, day, 0, 0, 0, 0, loc).Weekday() == want {
			days = append(days, day)
		}
	}
	if ordinal > 0 && ordinal <= len(days) {
		return days[ordinal-1]
	}
	if ordinal < 0 && -ordinal <= len(days) {
		return days[len(days)+ordinal]
	}
	return 0
}

// addMonths shifts a first-of-month anchor forward by delta months.
func addMonths(t time.Time, delta int) time.Time {
	total := int(t.Month()) - 1 + delta
	year := t.Year() + total/12
	month := time.Month(total%12 + 1)
	return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
}

// daysInMonth returns the number of days in the given month.
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// parseByDay parses a BYDAY value such as "MO,WE,FR" or "-1FR,2TU".
func parseByDay(value string) ([]WeekDayNum, error) {
	var out []WeekDayNum
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if len(part) < 2 {
			return nil, fmt.Errorf("rrule byday %q: too short", part)
		}
		name := part[len(part)-2:]
		day, ok := parseWeekday(name)
		if !ok {
			return nil, fmt.Errorf("rrule byday %q: not a weekday", part)
		}
		entry := WeekDayNum{Day: day}
		if prefix := part[:len(part)-2]; prefix != "" {
			n, err := strconv.Atoi(prefix)
			if err != nil || n == 0 {
				return nil, fmt.Errorf("rrule byday %q: bad ordinal", part)
			}
			entry.Ordinal = n
		}
		out = append(out, entry)
	}
	return out, nil
}

// parseInts parses a comma-separated list of signed integers.
func parseInts(value, label string) ([]int, error) {
	var out []int
	for part := range strings.SplitSeq(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("rrule %s %q: not a number", label, part)
		}
		out = append(out, n)
	}
	return out, nil
}

// parseWeekday converts a two-letter iCalendar weekday abbreviation.
func parseWeekday(s string) (time.Weekday, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "SU":
		return time.Sunday, true
	case "MO":
		return time.Monday, true
	case "TU":
		return time.Tuesday, true
	case "WE":
		return time.Wednesday, true
	case "TH":
		return time.Thursday, true
	case "FR":
		return time.Friday, true
	case "SA":
		return time.Saturday, true
	}
	return 0, false
}

// containsMonth reports whether the month appears in the list.
func containsMonth(months []time.Month, m time.Month) bool {
	for _, x := range months {
		if x == m {
			return true
		}
	}
	return false
}

// containsWeekday reports whether the weekday appears in the BYDAY list.
func containsWeekday(days []WeekDayNum, d time.Weekday) bool {
	for _, x := range days {
		if x.Day == d {
			return true
		}
	}
	return false
}

// containsInt reports whether n appears in the list.
func containsInt(list []int, n int) bool {
	for _, x := range list {
		if x == n {
			return true
		}
	}
	return false
}

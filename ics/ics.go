// Package ics parses the small iCalendar subset midden ingests from local .ics exports.
//
// The parser handles line folding, VEVENT records, and the SUMMARY, DTSTART,
// DTEND, LOCATION, DESCRIPTION, UID, and RRULE properties. Components nested
// inside a VEVENT (VALARM and friends) are skipped so their properties never
// touch the parent event. TZID parameters resolve through time.LoadLocation
// with a fallback to the machine's local zone; the UTC suffix Z is honored.
// Parsed times are anchored to the local zone. Events whose DTSTART is
// missing or unparseable are dropped and counted rather than returned.
package ics

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// Event is a single calendar entry.
type Event struct {
	// UID is the calendar-assigned unique identifier, or empty when absent.
	UID string
	// Summary is the event title.
	Summary string
	// Start is the local-anchored start time.
	Start time.Time
	// End is the local-anchored end time, or the zero time when missing.
	End time.Time
	// Location is the optional event location.
	Location string
	// Description is the optional event body.
	Description string
	// AllDay reports whether DTSTART carried a date-only value.
	AllDay bool
	// Recurs reports whether the event carries an RRULE. Recurrences are not expanded.
	Recurs bool
}

// Parse reads an iCalendar stream and returns every VEVENT it contains plus
// the number of events dropped because DTSTART was missing or unparseable.
func Parse(r io.Reader) ([]Event, int, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, 0, err
	}
	var events []Event
	var current *Event
	depth := 0
	skipped := 0
	for _, line := range lines {
		upper := strings.ToUpper(line)
		if current == nil {
			if upper == "BEGIN:VEVENT" {
				current = &Event{}
				depth = 0
			}
			continue
		}
		switch {
		case strings.HasPrefix(upper, "BEGIN:"):
			depth++
		case upper == "END:VEVENT" && depth == 0:
			if current.Start.IsZero() {
				skipped++
			} else {
				events = append(events, *current)
			}
			current = nil
		case strings.HasPrefix(upper, "END:"):
			if depth > 0 {
				depth--
			}
		case depth == 0:
			applyProperty(current, line)
		}
	}
	return events, skipped, nil
}

// unfold reads the stream and merges folded continuation lines back into single logical lines.
func unfold(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lines []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			lines = append(lines, current.String())
			current.Reset()
		}
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			current.WriteString(line[1:])
			continue
		}
		flush()
		current.WriteString(line)
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ics: %w", err)
	}
	return lines, nil
}

// applyProperty matches one iCalendar property line and stores the value on the event.
func applyProperty(e *Event, line string) {
	name, params, value, ok := splitProperty(line)
	if !ok {
		return
	}
	switch strings.ToUpper(name) {
	case "SUMMARY":
		e.Summary = unescape(value)
	case "LOCATION":
		e.Location = unescape(value)
	case "DESCRIPTION":
		e.Description = unescape(value)
	case "UID":
		e.UID = unescape(value)
	case "RRULE":
		e.Recurs = true
	case "DTSTART":
		if t, allDay, ok := parseTime(value, params); ok {
			e.Start = t
			e.AllDay = allDay
		}
	case "DTEND":
		if t, _, ok := parseTime(value, params); ok {
			e.End = t
		}
	}
}

// splitProperty splits a logical line into its name, parameters, and value.
func splitProperty(line string) (string, string, string, bool) {
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", "", false
	}
	head := before
	value := after
	if before, after, ok := strings.Cut(head, ";"); ok {
		return before, after, value, true
	}
	return head, "", value, true
}

// parseTime accepts the date-time and date forms midden cares about. It
// returns the parsed time, whether the value was date-only, and success.
func parseTime(value, params string) (time.Time, bool, bool) {
	if isDateOnly(params, value) {
		t, err := time.ParseInLocation("20060102", value, time.Local)
		if err != nil {
			return time.Time{}, false, false
		}
		return t, true, true
	}
	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return time.Time{}, false, false
		}
		return t.Local(), false, true
	}
	t, err := time.ParseInLocation("20060102T150405", value, location(params))
	if err != nil {
		return time.Time{}, false, false
	}
	return t.In(time.Local), false, true
}

// location resolves the TZID parameter to a time.Location, falling back to the
// machine's local zone when the parameter is absent or the zone is unknown.
func location(params string) *time.Location {
	tzid := paramValue(params, "TZID")
	if tzid == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		return time.Local
	}
	return loc
}

// isDateOnly reports whether the property is date-only. An explicit VALUE
// parameter is trusted; a bare eight-character value is the fallback.
func isDateOnly(params, value string) bool {
	if v := paramValue(params, "VALUE"); v != "" {
		return strings.EqualFold(v, "DATE")
	}
	return len(value) == 8
}

// paramValue returns the named parameter's value from the raw parameter
// string with surrounding quotes removed, or empty when the parameter is absent.
func paramValue(params, name string) string {
	for p := range strings.SplitSeq(params, ";") {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// unescape rewrites the iCalendar text escapes in a single pass so an escaped
// backslash never re-forms an escape sequence with the character after it.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case '\\', ';', ',':
			b.WriteByte(s[i])
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// Package ics parses the small iCalendar subset midden ingests from local .ics exports.
//
// The parser handles line folding, plain VEVENT records, and SUMMARY, DTSTART,
// DTEND, LOCATION, and DESCRIPTION properties. TZID parameters are ignored;
// floating times are treated as local. UTC suffix Z is honored.
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
}

// Parse reads an iCalendar stream and returns every VEVENT it contains.
func Parse(r io.Reader) ([]Event, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}
	var events []Event
	var current *Event
	for _, line := range lines {
		switch {
		case line == "BEGIN:VEVENT":
			current = &Event{}
		case line == "END:VEVENT":
			if current != nil {
				events = append(events, *current)
				current = nil
			}
		case current != nil:
			applyProperty(current, line)
		}
	}
	return events, nil
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
	case "DTSTART":
		if t, ok := parseTime(value, params); ok {
			e.Start = t
		}
	case "DTEND":
		if t, ok := parseTime(value, params); ok {
			e.End = t
		}
	}
}

// splitProperty splits a logical line into its name, parameters, and value.
func splitProperty(line string) (string, string, string, bool) {
	colon := strings.Index(line, ":")
	if colon < 0 {
		return "", "", "", false
	}
	head := line[:colon]
	value := line[colon+1:]
	if semi := strings.Index(head, ";"); semi >= 0 {
		return head[:semi], head[semi+1:], value, true
	}
	return head, "", value, true
}

// parseTime accepts the date-time and date forms midden cares about.
func parseTime(value, params string) (time.Time, bool) {
	if isDateOnly(params, value) {
		t, err := time.ParseInLocation("20060102", value, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		if err != nil {
			return time.Time{}, false
		}
		return t.Local(), true
	}
	t, err := time.ParseInLocation("20060102T150405", value, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// isDateOnly reports whether the property uses VALUE=DATE or a bare YYYYMMDD value.
func isDateOnly(params, value string) bool {
	if strings.Contains(strings.ToUpper(params), "VALUE=DATE") {
		return true
	}
	return len(value) == 8
}

// unescape rewrites the small set of iCalendar text escapes.
func unescape(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\N`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

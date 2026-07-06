package ics

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// errRead is the failure returned by errReader.
var errRead = errors.New("boom")

// errReader always fails so tests can exercise the Parse read-error path.
type errReader struct{}

// Read implements io.Reader and always returns errRead.
func (errReader) Read([]byte) (int, error) { return 0, errRead }

// TestParse covers the full property, nesting, escaping, and skip behavior of Parse.
func TestParse(t *testing.T) {
	t.Parallel()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	tests := []struct {
		In          io.Reader
		Want        error
		WantEvents  []Event
		WantSkipped int
	}{{ // Test 0: Basic event with UTC times, UID capture, and an escaped comma.
		In: strings.NewReader(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:evt-1@example.com
SUMMARY:Coffee with Alex
DTSTART:20260616T140000Z
DTEND:20260616T150000Z
LOCATION:Texas Coffee
DESCRIPTION:Catch up on the proposal\, send notes after.
END:VEVENT
END:VCALENDAR
`),
		WantEvents: []Event{{
			UID:         "evt-1@example.com",
			Summary:     "Coffee with Alex",
			Start:       time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
			End:         time.Date(2026, 6, 16, 15, 0, 0, 0, time.UTC),
			Location:    "Texas Coffee",
			Description: "Catch up on the proposal, send notes after.",
		}},
	}, { // Test 1: Folded continuation lines are joined into one logical line.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:First line of summary
  that continues here.
DTSTART:20260616T140000Z
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "First line of summary that continues here.",
			Start:   time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
		}},
	}, { // Test 2: A nested VALARM must not overwrite the parent event's properties.
		In: strings.NewReader(`BEGIN:VEVENT
UID:alarm-evt
SUMMARY:Dentist
DTSTART:20260620T090000Z
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Reminder
TRIGGER:-PT15M
END:VALARM
LOCATION:Clinic
END:VEVENT
`),
		WantEvents: []Event{{
			UID:      "alarm-evt",
			Summary:  "Dentist",
			Start:    time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC),
			Location: "Clinic",
		}},
	}, { // Test 3: TZID resolves to the named zone; top-level VTIMEZONE is ignored.
		In: strings.NewReader(`BEGIN:VCALENDAR
BEGIN:VTIMEZONE
TZID:America/New_York
BEGIN:STANDARD
DTSTART:19701101T020000
END:STANDARD
END:VTIMEZONE
BEGIN:VEVENT
SUMMARY:East coast call
DTSTART;TZID=America/New_York:20260101T090000
DTEND;TZID=America/New_York:20260101T093000
END:VEVENT
END:VCALENDAR
`),
		WantEvents: []Event{{
			Summary: "East coast call",
			Start:   time.Date(2026, 1, 1, 9, 0, 0, 0, ny),
			End:     time.Date(2026, 1, 1, 9, 30, 0, 0, ny),
		}},
	}, { // Test 4: An unknown TZID falls back to the machine's local zone.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Mystery zone
DTSTART;TZID=Not/AZone:20260101T090000
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Mystery zone",
			Start:   time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local),
		}},
	}, { // Test 5: Escapes decode in one pass so \\n stays a backslash and an n.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Escapes
DTSTART:20260616T140000Z
DESCRIPTION:a\nb\Nc\\nd\;e\,f\\g
END:VEVENT
`),
		WantEvents: []Event{{
			Summary:     "Escapes",
			Start:       time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
			Description: "a\nb\nc\\nd;e,f\\g",
		}},
	}, { // Test 6: An explicit VALUE=DATE parameter marks the event all-day.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Field trip
DTSTART;VALUE=DATE:20260616
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Field trip",
			Start:   time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
			AllDay:  true,
		}},
	}, { // Test 7: A bare eight-digit value still falls back to all-day.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Bare date
DTSTART:20260616
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Bare date",
			Start:   time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
			AllDay:  true,
		}},
	}, { // Test 8: An explicit VALUE=DATE-TIME parameter is not treated as all-day.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Timed
DTSTART;VALUE=DATE-TIME:20260616T140000Z
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Timed",
			Start:   time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
		}},
	}, { // Test 9: An RRULE property sets Recurs without expanding occurrences.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Standup
DTSTART:20260616T140000Z
RRULE:FREQ=WEEKLY;BYDAY=MO
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Standup",
			Start:   time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
			Recurs:  true,
		}},
	}, { // Test 10: A malformed DTSTART drops the event and counts it as skipped.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:Broken
DTSTART:not-a-date
END:VEVENT
BEGIN:VEVENT
SUMMARY:Fine
DTSTART:20260616T140000Z
END:VEVENT
`),
		WantEvents: []Event{{
			Summary: "Fine",
			Start:   time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC),
		}},
		WantSkipped: 1,
	}, { // Test 11: A missing DTSTART also drops the event and counts it as skipped.
		In: strings.NewReader(`BEGIN:VEVENT
SUMMARY:No start
END:VEVENT
`),
		WantSkipped: 1,
	}, { // Test 12: A failing reader surfaces the wrapped read error.
		In:   errReader{},
		Want: errRead,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			events, skipped, err := Parse(test.In)
			if !errors.Is(err, test.Want) {
				t.Fatalf("error mismatch: want %v, got %v", test.Want, err)
			}
			if diff := cmp.Diff(test.WantSkipped, skipped); diff != "" {
				t.Errorf("skipped mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantEvents, events, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("events mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

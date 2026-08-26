package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/ics"
)

func TestIngestWindowDefaultsToTheWholeFile(t *testing.T) {
	t.Parallel()
	events := []ics.Event{
		{Start: time.Date(2018, time.April, 9, 10, 0, 0, 0, time.Local)},
		{Start: time.Date(2021, time.July, 2, 8, 0, 0, 0, time.Local)},
	}
	span, err := ingestWindow(events, "", "")
	if err != nil {
		t.Fatalf("ingestWindow: %v", err)
	}
	// Backfilling a decade of calendar is the point of the command, so an
	// unnarrowed run must not collapse to a single day.
	want := time.Date(2018, time.April, 9, 0, 0, 0, 0, time.Local)
	if !span.From.Equal(want) {
		t.Errorf("want the range to open at the earliest event %s, got %s", want, span.From)
	}
	if span.To.Before(time.Now()) {
		t.Errorf("want the range to reach at least today, got %s", span.To)
	}
}

func TestIngestWindowReachesPastTheLastEventForOpenSeries(t *testing.T) {
	t.Parallel()
	// Everything in the file is historical, but an unbounded weekly series still
	// runs to today, so the window has to as well.
	events := []ics.Event{{Start: time.Date(2019, time.January, 7, 9, 0, 0, 0, time.Local)}}
	span, err := ingestWindow(events, "", "")
	if err != nil {
		t.Fatalf("ingestWindow: %v", err)
	}
	if span.To.Before(dayStart(time.Now())) {
		t.Errorf("want the range to reach today, got %s", span.To)
	}
}

func TestIngestWindowHonorsExplicitBounds(t *testing.T) {
	t.Parallel()
	events := []ics.Event{{Start: time.Date(2018, time.April, 9, 10, 0, 0, 0, time.Local)}}
	span, err := ingestWindow(events, "2020-01-01", "2020-12-31")
	if err != nil {
		t.Fatalf("ingestWindow: %v", err)
	}
	if diff := cmp.Diff("2020-01-01 to 2020-12-31", span.Label()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestIngestWindowRejectsBadBounds(t *testing.T) {
	t.Parallel()
	if _, err := ingestWindow(nil, "not-a-date", ""); err == nil {
		t.Error("want an error for an unparseable --from")
	}
}

func TestOccurrenceKeySeparatesOccurrencesOfOneSeries(t *testing.T) {
	t.Parallel()
	uid := "standup@example"
	first := occurrenceKey(uid, "Standup", time.Date(2024, time.March, 4, 9, 0, 0, 0, time.Local))
	second := occurrenceKey(uid, "Standup", time.Date(2024, time.March, 11, 9, 0, 0, 0, time.Local))
	// Every occurrence of a series carries the same UID, so a key built from the
	// UID alone would collapse a weekly meeting into one entry.
	if first == second {
		t.Errorf("want distinct keys for two occurrences, both were %q", first)
	}
	repeat := occurrenceKey(uid, "Standup", time.Date(2024, time.March, 4, 9, 0, 0, 0, time.Local))
	if first != repeat {
		t.Errorf("want a stable key for the same occurrence, got %q then %q", first, repeat)
	}
}

func TestOccurrenceKeyFallsBackToTheHeadline(t *testing.T) {
	t.Parallel()
	start := time.Date(2024, time.March, 4, 9, 0, 0, 0, time.Local)
	a := occurrenceKey("", "Lunch with Sam", start)
	b := occurrenceKey("", "Dentist", start)
	if a == b {
		t.Error("want different keys for different untitled events at the same time")
	}
	if a != occurrenceKey("", "Lunch with Sam", start) {
		t.Error("want a stable key for the same untitled event")
	}
}

func TestBodyUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Body string
		Want string
	}{{ // Test 0: The UID line is read out of a formatted calendar entry.
		Body: "Standup (09:00 to 09:30)\nICS-UID: abc123@google.com",
		Want: "abc123@google.com",
	}, { // Test 1: A UID after a description is still found.
		Body: "Trip\nLocation: Denver\n\nPacking list\nICS-UID: xyz",
		Want: "xyz",
	}, { // Test 2: An entry written by hand has no UID.
		Body: "Thought about the roadmap today.",
		Want: "",
	}, { // Test 3: A body mentioning the prefix mid-line does not produce a UID.
		Body: "Wrote about the ICS-UID: format in my notes today.",
		Want: "",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, bodyUID(test.Body)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatEventRoundTripsThroughBodyUID(t *testing.T) {
	t.Parallel()
	e := ics.Event{
		UID:      "standup@example",
		Summary:  "Standup",
		Start:    time.Date(2024, time.March, 4, 9, 0, 0, 0, time.Local),
		End:      time.Date(2024, time.March, 4, 9, 30, 0, 0, time.Local),
		Location: "Zoom",
	}
	body := formatEvent(e)
	// The key an ingest writes and the key a later ingest reads back have to
	// agree, or every re-ingest duplicates the whole calendar.
	written := occurrenceKey(e.UID, firstLine(body), e.Start)
	read := occurrenceKey(bodyUID(body), firstLine(body), e.Start)
	if diff := cmp.Diff(written, read); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

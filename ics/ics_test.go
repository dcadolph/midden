package ics

import (
	"strings"
	"testing"
	"time"
)

func TestParseSingleEvent(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
SUMMARY:Coffee with Alex
DTSTART:20260616T140000Z
DTEND:20260616T150000Z
LOCATION:Texas Coffee
DESCRIPTION:Catch up on the proposal\, send notes after.
END:VEVENT
END:VCALENDAR
`)
	events, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	got := events[0]
	if got.Summary != "Coffee with Alex" {
		t.Errorf("summary mismatch: %q", got.Summary)
	}
	want := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC).Local()
	if !got.Start.Equal(want) {
		t.Errorf("start mismatch: want %v, got %v", want, got.Start)
	}
	if got.Description != "Catch up on the proposal, send notes after." {
		t.Errorf("description mismatch: %q", got.Description)
	}
}

func TestParseFoldedLines(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`BEGIN:VEVENT
SUMMARY:First line of summary
 that continues here.
DTSTART:20260616T140000Z
END:VEVENT
`)
	events, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if !strings.Contains(events[0].Summary, "continues here") {
		t.Errorf("folded summary not joined: %q", events[0].Summary)
	}
}

func TestParseDateOnly(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`BEGIN:VEVENT
SUMMARY:All day event
DTSTART;VALUE=DATE:20260616
END:VEVENT
`)
	events, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	want := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	if !events[0].Start.Equal(want) {
		t.Errorf("date-only start mismatch: want %v, got %v", want, events[0].Start)
	}
}

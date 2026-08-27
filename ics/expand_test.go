package ics

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// weekly builds a recurring master event from an RRULE value.
func weekly(t *testing.T, uid, summary, rule string, start time.Time, d time.Duration) Event {
	t.Helper()
	r, err := ParseRRULE(rule)
	if err != nil {
		t.Fatalf("ParseRRULE(%q): %v", rule, err)
	}
	e := Event{UID: uid, Summary: summary, Start: start, RawRule: rule, Rule: &r}
	if d > 0 {
		e.End = start.Add(d)
	}
	return e
}

// startStrings renders the start times of expanded events.
func startStrings(events []Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Start.Format("2006-01-02 15:04")
	}
	return out
}

func TestExpandSeriesProducesEveryOccurrence(t *testing.T) {
	t.Parallel()
	master := weekly(t, "standup@example", "Standup", "FREQ=WEEKLY;BYDAY=MO,WE", at(2024, time.March, 4, 9), 30*time.Minute)
	got, report := Expand([]Event{master}, at(2024, time.March, 1, 0), at(2024, time.March, 15, 23))
	want := []string{
		"2024-03-04 09:00", "2024-03-06 09:00",
		"2024-03-11 09:00", "2024-03-13 09:00",
	}
	if diff := cmp.Diff(want, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
	if report.Occurrences != 4 {
		t.Errorf("want 4 occurrences reported, got %d", report.Occurrences)
	}
	for _, e := range got {
		if e.Recurs() {
			t.Errorf("occurrence %s still carries a rule", e.Start)
		}
		if e.UID != master.UID {
			t.Errorf("occurrence %s lost the series UID", e.Start)
		}
		if e.End.Sub(e.Start) != 30*time.Minute {
			t.Errorf("occurrence %s lost the series duration", e.Start)
		}
	}
}

func TestExpandSeriesStartingBeforeTheWindow(t *testing.T) {
	t.Parallel()
	// A weekly meeting running for years contributes its in-window occurrences
	// even though the master event predates the window by a decade.
	master := weekly(t, "oneone@example", "1:1", "FREQ=WEEKLY;BYDAY=TH", at(2014, time.January, 2, 15), time.Hour)
	got, _ := Expand([]Event{master}, at(2024, time.March, 1, 0), at(2024, time.March, 31, 23))
	want := []string{"2024-03-07 15:00", "2024-03-14 15:00", "2024-03-21 15:00", "2024-03-28 15:00"}
	if diff := cmp.Diff(want, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandHonorsExDate(t *testing.T) {
	t.Parallel()
	master := weekly(t, "standup@example", "Standup", "FREQ=WEEKLY;BYDAY=MO", at(2024, time.March, 4, 9), 0)
	master.ExDates = []time.Time{at(2024, time.March, 11, 9)}
	got, report := Expand([]Event{master}, at(2024, time.March, 1, 0), at(2024, time.March, 20, 23))
	want := []string{"2024-03-04 09:00", "2024-03-18 09:00"}
	if diff := cmp.Diff(want, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
	if report.Excluded != 1 {
		t.Errorf("want 1 exclusion reported, got %d", report.Excluded)
	}
}

func TestExpandOverrideReplacesGeneratedOccurrence(t *testing.T) {
	t.Parallel()
	master := weekly(t, "standup@example", "Standup", "FREQ=WEEKLY;BYDAY=MO", at(2024, time.March, 4, 9), 0)
	// The calendar moved the second occurrence to the afternoon. Both the
	// generated 09:00 and the moved 14:00 would otherwise land in the vault.
	override := Event{
		UID:          "standup@example",
		Summary:      "Standup (moved)",
		Start:        at(2024, time.March, 11, 14),
		RecurrenceID: at(2024, time.March, 11, 9),
	}
	got, report := Expand([]Event{master, override}, at(2024, time.March, 1, 0), at(2024, time.March, 20, 23))
	want := []string{"2024-03-04 09:00", "2024-03-11 14:00", "2024-03-18 09:00"}
	if diff := cmp.Diff(want, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
	if report.Overridden != 1 {
		t.Errorf("want 1 override reported, got %d", report.Overridden)
	}
}

func TestExpandUnexpandableRuleKeepsFirstOccurrence(t *testing.T) {
	t.Parallel()
	// A frequency midden does not expand still contributes the event as written,
	// and is counted so the caller can say the series is incomplete.
	master := Event{UID: "odd@example", Summary: "Hourly", Start: at(2024, time.March, 4, 9), RawRule: "FREQ=HOURLY"}
	got, report := Expand([]Event{master}, at(2024, time.March, 1, 0), at(2024, time.March, 20, 23))
	if diff := cmp.Diff([]string{"2024-03-04 09:00"}, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
	if report.Unexpanded != 1 {
		t.Errorf("want 1 unexpanded series reported, got %d", report.Unexpanded)
	}
}

func TestExpandFiltersPlainEventsToWindow(t *testing.T) {
	t.Parallel()
	events := []Event{
		{UID: "a", Summary: "before", Start: at(2024, time.February, 1, 9)},
		{UID: "b", Summary: "inside", Start: at(2024, time.March, 5, 9)},
		{UID: "c", Summary: "after", Start: at(2024, time.April, 1, 9)},
	}
	got, report := Expand(events, at(2024, time.March, 1, 0), at(2024, time.March, 31, 23))
	if diff := cmp.Diff([]string{"2024-03-05 09:00"}, startStrings(got)); diff != "" {
		t.Errorf("occurrences mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(ExpandReport{}, report, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("want an empty report for plain events (-want +got):\n%s", diff)
	}
}

func TestExpandOrdersMixedSourcesByStart(t *testing.T) {
	t.Parallel()
	events := []Event{
		{UID: "plain", Summary: "one off", Start: at(2024, time.March, 12, 8)},
		weekly(t, "s", "Standup", "FREQ=WEEKLY;BYDAY=MO", at(2024, time.March, 4, 9), 0),
	}
	got, _ := Expand(events, at(2024, time.March, 1, 0), at(2024, time.March, 20, 23))
	want := []string{"2024-03-04 09:00", "2024-03-11 09:00", "2024-03-12 08:00", "2024-03-18 09:00"}
	if diff := cmp.Diff(want, startStrings(got)); diff != "" {
		t.Errorf("ordering mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandReportsTruncatedUnboundedSeries(t *testing.T) {
	t.Parallel()
	master := weekly(t, "forever@example", "Forever", "FREQ=DAILY", at(2024, time.March, 4, 9), 0)
	// With no window end and no COUNT or UNTIL there is nothing to stop the
	// walk, so the series is reported rather than expanded.
	got, report := Expand([]Event{master}, time.Time{}, time.Time{})
	if len(got) != 0 {
		t.Errorf("want no occurrences from an unbounded walk, got %d", len(got))
	}
	if report.Truncated != 1 {
		t.Errorf("want 1 truncated series reported, got %d", report.Truncated)
	}
}

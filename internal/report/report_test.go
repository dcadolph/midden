package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/vault"
)

// TestCellClass verifies the mapping from entry counts to CSS shading classes.
func TestCellClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantClass string
		In        int
	}{{ // Test 0: Zero entries gets no class.
		In: 0, WantClass: "",
	}, { // Test 1: One entry gets the lightest shade.
		In: 1, WantClass: "l1",
	}, { // Test 2: Two entries gets the middle shade.
		In: 2, WantClass: "l2",
	}, { // Test 3: Three entries stays in the middle shade.
		In: 3, WantClass: "l2",
	}, { // Test 4: Four entries gets the darkest shade.
		In: 4, WantClass: "l3",
	}, { // Test 5: Large counts stay in the darkest shade.
		In: 42, WantClass: "l3",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := cellClass(test.In)
			if diff := cmp.Diff(test.WantClass, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStartOfWeek verifies alignment to midnight on the Sunday at or before t.
func TestStartOfWeek(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In       time.Time
		WantDate time.Time
	}{{ // Test 0: A Thursday maps to the preceding Sunday.
		In:       time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		WantDate: time.Date(1969, 12, 28, 0, 0, 0, 0, time.UTC),
	}, { // Test 1: A Saturday maps to the preceding Sunday.
		In:       time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		WantDate: time.Date(1999, 12, 26, 0, 0, 0, 0, time.UTC),
	}, { // Test 2: A Sunday maps to itself.
		In:       time.Date(1999, 12, 26, 0, 0, 0, 0, time.UTC),
		WantDate: time.Date(1999, 12, 26, 0, 0, 0, 0, time.UTC),
	}, { // Test 3: The time of day is truncated to midnight.
		In:       time.Date(2000, 1, 1, 15, 30, 45, 1, time.UTC),
		WantDate: time.Date(1999, 12, 26, 0, 0, 0, 0, time.UTC),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := startOfWeek(test.In)
			if got.Weekday() != time.Sunday {
				t.Errorf("weekday = %v, want %v", got.Weekday(), time.Sunday)
			}
			if diff := cmp.Diff(test.WantDate, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBuildCalendar verifies the heatmap window shape and cell placement.
func TestBuildCalendar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Counts    map[string]dayMetric
		Now       time.Time
		EntryDate string
		WantCount int
		WantClass string
	}{{ // Test 0: A mid-window entry lands on its own date with the middle shade.
		Counts:    map[string]dayMetric{"2026-01-15": {Entries: 2, Words: 20}},
		Now:       time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		EntryDate: "2026-01-15",
		WantCount: 2,
		WantClass: "l2",
	}, { // Test 1: A reference date on a Sunday keeps the window week aligned.
		Counts:    map[string]dayMetric{"2026-07-05": {Entries: 1, Words: 5}},
		Now:       time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
		EntryDate: "2026-07-05",
		WantCount: 1,
		WantClass: "l1",
	}, { // Test 2: A date with no entries renders an empty cell.
		Counts:    map[string]dayMetric{},
		Now:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EntryDate: "2026-03-10",
		WantCount: 0,
		WantClass: "",
	}, { // Test 3: A busy day gets the darkest shade.
		Counts:    map[string]dayMetric{"2025-12-31": {Entries: 9, Words: 90}},
		Now:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EntryDate: "2025-12-31",
		WantCount: 9,
		WantClass: "l3",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := buildCalendar(test.Counts, test.Now)
			if len(got) == 0 || len(got)%7 != 0 {
				t.Fatalf("cell count = %d, want a positive multiple of 7", len(got))
			}
			first, err := time.Parse("2006-01-02", got[0].Date)
			if err != nil {
				t.Fatalf("parse first cell date %q: %v", got[0].Date, err)
			}
			if first.Weekday() != time.Sunday {
				t.Errorf("first cell weekday = %v, want %v", first.Weekday(), time.Sunday)
			}
			entry, err := time.Parse("2006-01-02", test.EntryDate)
			if err != nil {
				t.Fatalf("parse entry date %q: %v", test.EntryDate, err)
			}
			idx := int(entry.Sub(first).Hours() / 24)
			if idx < 0 || idx >= len(got) {
				t.Fatalf("entry date %s outside window of %d cells", test.EntryDate, len(got))
			}
			want := CalendarCell{Date: test.EntryDate, Count: test.WantCount, Class: test.WantClass}
			if diff := cmp.Diff(want, got[idx]); diff != "" {
				t.Errorf("cell %d mismatch (-want +got):\n%s", idx, diff)
			}
			if idx%7 != int(entry.Weekday()) {
				t.Errorf("row = %d, want weekday row %d", idx%7, int(entry.Weekday()))
			}
		})
	}
}

// TestBuildDayRows verifies ordering and the skipping of empty days.
func TestBuildDayRows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Counts   map[string]dayMetric
		Days     []time.Time
		WantRows []DayRow
	}{{ // Test 0: Rows come back newest first and empty days are skipped.
		Days: []time.Time{
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		},
		Counts: map[string]dayMetric{
			"2026-01-01": {Entries: 2, Words: 10},
			"2026-01-02": {},
			"2026-01-03": {Entries: 1, Words: 5},
		},
		WantRows: []DayRow{
			{Date: "2026-01-03", Entries: 1, Words: 5},
			{Date: "2026-01-01", Entries: 2, Words: 10},
		},
	}, { // Test 1: No days yields no rows.
		Days:     nil,
		Counts:   map[string]dayMetric{},
		WantRows: nil,
	}, { // Test 2: A day missing from counts is skipped.
		Days:     []time.Time{time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		Counts:   map[string]dayMetric{},
		WantRows: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := buildDayRows(test.Days, test.Counts)
			if diff := cmp.Diff(test.WantRows, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestBuildTagBars verifies bar scaling against the top tag.
func TestBuildTagBars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In       []vault.TagCount
		WantBars []TagBar
	}{{ // Test 0: The top tag gets width 100 and tiny tags get the minimum width 4.
		In: []vault.TagCount{{Tag: "work", Count: 50}, {Tag: "home", Count: 25}, {Tag: "gym", Count: 1}},
		WantBars: []TagBar{
			{Tag: "work", Count: 50, Width: 100},
			{Tag: "home", Count: 25, Width: 50},
			{Tag: "gym", Count: 1, Width: 4},
		},
	}, { // Test 1: A single tag spans the full width.
		In:       []vault.TagCount{{Tag: "solo", Count: 3}},
		WantBars: []TagBar{{Tag: "solo", Count: 3, Width: 100}},
	}, { // Test 2: No tags yields no bars.
		In:       nil,
		WantBars: nil,
	}, { // Test 3: A zero top count yields zero widths.
		In:       []vault.TagCount{{Tag: "ghost", Count: 0}},
		WantBars: []TagBar{{Tag: "ghost", Count: 0, Width: 0}},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := buildTagBars(test.In)
			if diff := cmp.Diff(test.WantBars, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRenderSmoke verifies that a small Data renders without a template error.
func TestRenderSmoke(t *testing.T) {
	t.Parallel()
	data := Data{
		Title:  "Midden smoke report",
		Stats:  vault.Stats{Days: 1, Entries: 2, Words: 3, Tags: 1},
		Streak: 1,
		Calendar: []CalendarCell{
			{Date: "2026-06-28", Count: 0, Class: ""},
			{Date: "2026-06-29", Count: 2, Class: "l2"},
		},
		Tags: []TagBar{{Tag: "work", Count: 2, Width: 100}},
		Days: []DayRow{{Date: "2026-06-29", Entries: 2, Words: 3}},
	}
	var buf bytes.Buffer
	if err := Render(&buf, data); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), "Midden smoke report") {
		t.Errorf("output does not contain the report title")
	}
}

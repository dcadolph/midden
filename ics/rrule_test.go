package ics

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// at builds a local timestamp for the given calendar date and hour.
func at(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.Local)
}

// occurrenceStrings renders occurrence times for comparison.
func occurrenceStrings(times []time.Time) []string {
	out := make([]string, len(times))
	for i, t := range times {
		out[i] = t.Format("2006-01-02 15:04")
	}
	return out
}

func TestParseRRULE(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Rule     string
		WantRule Recurrence
		Want     bool
	}{{ // Test 0: A plain weekly rule takes the default interval and week start.
		Rule:     "FREQ=WEEKLY",
		WantRule: Recurrence{Freq: Weekly, Interval: 1, WeekStart: time.Monday},
	}, { // Test 1: Every documented part is parsed.
		Rule: "FREQ=MONTHLY;INTERVAL=2;COUNT=5;BYDAY=-1FR,TU;BYMONTHDAY=13,-1;BYMONTH=3,6;WKST=SU",
		WantRule: Recurrence{
			Freq: Monthly, Interval: 2, Count: 5,
			ByDay:      []WeekDayNum{{Day: time.Friday, Ordinal: -1}, {Day: time.Tuesday}},
			ByMonthDay: []int{13, -1},
			ByMonth:    []time.Month{time.March, time.June},
			WeekStart:  time.Sunday,
		},
	}, { // Test 2: UNTIL in UTC resolves to a local instant.
		Rule:     "FREQ=DAILY;UNTIL=20240310T235959Z",
		WantRule: Recurrence{Freq: Daily, Interval: 1, WeekStart: time.Monday, Until: time.Date(2024, time.March, 10, 23, 59, 59, 0, time.UTC).Local()},
	}, { // Test 3: A missing FREQ is rejected rather than defaulted.
		Rule: "INTERVAL=2",
		Want: true,
	}, { // Test 4: An unsupported FREQ is rejected rather than silently expanded wrong.
		Rule: "FREQ=SECONDLY",
		Want: true,
	}, { // Test 5: A zero interval is rejected.
		Rule: "FREQ=DAILY;INTERVAL=0",
		Want: true,
	}, { // Test 6: A bad weekday is rejected.
		Rule: "FREQ=WEEKLY;BYDAY=XX",
		Want: true,
	}, { // Test 7: A zero BYDAY ordinal is rejected.
		Rule: "FREQ=MONTHLY;BYDAY=0FR",
		Want: true,
	}, { // Test 8: An out-of-range BYMONTH is rejected.
		Rule: "FREQ=YEARLY;BYMONTH=13",
		Want: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := ParseRRULE(test.Rule)
			if test.Want {
				if err == nil {
					t.Fatalf("want error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRRULE: %v", err)
			}
			if diff := cmp.Diff(test.WantRule, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOccurrences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		DTStart time.Time
		From    time.Time
		To      time.Time
		Rule    string
		Want    []string
	}{{ // Test 0: A weekly standup lands on each listed weekday.
		Rule:    "FREQ=WEEKLY;BYDAY=MO,WE,FR",
		DTStart: at(2024, time.March, 4, 9), // a Monday
		To:      at(2024, time.March, 15, 23),
		Want: []string{
			"2024-03-04 09:00", "2024-03-06 09:00", "2024-03-08 09:00",
			"2024-03-11 09:00", "2024-03-13 09:00", "2024-03-15 09:00",
		},
	}, { // Test 1: A weekly rule with no BYDAY repeats on dtstart's own weekday.
		Rule:    "FREQ=WEEKLY",
		DTStart: at(2024, time.March, 5, 14), // a Tuesday
		To:      at(2024, time.March, 26, 23),
		Want:    []string{"2024-03-05 14:00", "2024-03-12 14:00", "2024-03-19 14:00", "2024-03-26 14:00"},
	}, { // Test 2: A fortnightly one-to-one skips the intervening week.
		Rule:    "FREQ=WEEKLY;INTERVAL=2;BYDAY=TH",
		DTStart: at(2024, time.March, 7, 15), // a Thursday
		To:      at(2024, time.April, 18, 23),
		Want:    []string{"2024-03-07 15:00", "2024-03-21 15:00", "2024-04-04 15:00", "2024-04-18 15:00"},
	}, { // Test 3: A monthly rule repeats on dtstart's day of month.
		Rule:    "FREQ=MONTHLY",
		DTStart: at(2024, time.January, 15, 10),
		To:      at(2024, time.April, 30, 23),
		Want:    []string{"2024-01-15 10:00", "2024-02-15 10:00", "2024-03-15 10:00", "2024-04-15 10:00"},
	}, { // Test 4: A monthly rule on the 31st skips months too short to hold it.
		Rule:    "FREQ=MONTHLY;BYMONTHDAY=31",
		DTStart: at(2024, time.January, 31, 8),
		To:      at(2024, time.June, 30, 23),
		Want:    []string{"2024-01-31 08:00", "2024-03-31 08:00", "2024-05-31 08:00"},
	}, { // Test 5: An ordinal BYDAY selects the second Tuesday of each month.
		Rule:    "FREQ=MONTHLY;BYDAY=2TU",
		DTStart: at(2024, time.January, 9, 18),
		To:      at(2024, time.April, 30, 23),
		Want:    []string{"2024-01-09 18:00", "2024-02-13 18:00", "2024-03-12 18:00", "2024-04-09 18:00"},
	}, { // Test 6: A negative ordinal selects the last Friday of each month.
		Rule:    "FREQ=MONTHLY;BYDAY=-1FR",
		DTStart: at(2024, time.January, 26, 17),
		To:      at(2024, time.March, 31, 23),
		Want:    []string{"2024-01-26 17:00", "2024-02-23 17:00", "2024-03-29 17:00"},
	}, { // Test 7: BYDAY and BYMONTHDAY together narrow to Friday the thirteenth.
		Rule:    "FREQ=MONTHLY;BYDAY=FR;BYMONTHDAY=13",
		DTStart: at(2024, time.September, 13, 12),
		To:      at(2025, time.December, 31, 23),
		Want:    []string{"2024-09-13 12:00", "2024-12-13 12:00", "2025-06-13 12:00"},
	}, { // Test 8: A yearly rule is the birthday case.
		Rule:    "FREQ=YEARLY",
		DTStart: at(2020, time.July, 4, 0),
		To:      at(2023, time.December, 31, 23),
		Want:    []string{"2020-07-04 00:00", "2021-07-04 00:00", "2022-07-04 00:00", "2023-07-04 00:00"},
	}, { // Test 9: A yearly rule with BYMONTH and an ordinal BYDAY tracks a moving holiday.
		Rule:    "FREQ=YEARLY;BYMONTH=5;BYDAY=2SU",
		DTStart: at(2024, time.May, 12, 11),
		To:      at(2026, time.December, 31, 23),
		Want:    []string{"2024-05-12 11:00", "2025-05-11 11:00", "2026-05-10 11:00"},
	}, { // Test 10: COUNT caps the series regardless of how far the window reaches.
		Rule:    "FREQ=DAILY;COUNT=3",
		DTStart: at(2024, time.March, 1, 7),
		To:      at(2024, time.December, 31, 23),
		Want:    []string{"2024-03-01 07:00", "2024-03-02 07:00", "2024-03-03 07:00"},
	}, { // Test 11: UNTIL ends the series before the window does.
		Rule:    "FREQ=DAILY;UNTIL=20240304T000000",
		DTStart: at(2024, time.March, 1, 0),
		To:      at(2024, time.December, 31, 23),
		Want:    []string{"2024-03-01 00:00", "2024-03-02 00:00", "2024-03-03 00:00", "2024-03-04 00:00"},
	}, { // Test 12: An interval above one on a daily rule skips days.
		Rule:    "FREQ=DAILY;INTERVAL=3",
		DTStart: at(2024, time.March, 1, 6),
		To:      at(2024, time.March, 10, 23),
		Want:    []string{"2024-03-01 06:00", "2024-03-04 06:00", "2024-03-07 06:00", "2024-03-10 06:00"},
	}, { // Test 13: COUNT is measured from dtstart, not from the start of the window.
		Rule:    "FREQ=DAILY;COUNT=5",
		DTStart: at(2024, time.March, 1, 9),
		From:    at(2024, time.March, 4, 0),
		To:      at(2024, time.December, 31, 23),
		Want:    []string{"2024-03-04 09:00", "2024-03-05 09:00"},
	}, { // Test 14: A window opening after the series ends yields nothing.
		Rule:    "FREQ=WEEKLY;COUNT=2",
		DTStart: at(2024, time.March, 4, 9),
		From:    at(2025, time.January, 1, 0),
		To:      at(2025, time.December, 31, 23),
		Want:    nil,
	}, { // Test 15: A daily rule filtered by BYDAY keeps only weekdays.
		Rule:    "FREQ=DAILY;BYDAY=SA,SU",
		DTStart: at(2024, time.March, 1, 8), // a Friday
		To:      at(2024, time.March, 11, 23),
		Want:    []string{"2024-03-02 08:00", "2024-03-03 08:00", "2024-03-09 08:00", "2024-03-10 08:00"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			rule, err := ParseRRULE(test.Rule)
			if err != nil {
				t.Fatalf("ParseRRULE(%q): %v", test.Rule, err)
			}
			got, truncated := rule.Occurrences(test.DTStart, test.From, test.To)
			if truncated {
				t.Fatalf("expansion was truncated")
			}
			if diff := cmp.Diff(test.Want, occurrenceStrings(got), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestOccurrencesRefusesUnboundedWalk(t *testing.T) {
	t.Parallel()
	rule, err := ParseRRULE("FREQ=DAILY")
	if err != nil {
		t.Fatalf("ParseRRULE: %v", err)
	}
	// No COUNT, no UNTIL, and no window end leaves nothing to stop the walk.
	got, truncated := rule.Occurrences(at(2024, time.March, 1, 9), time.Time{}, time.Time{})
	if !truncated {
		t.Errorf("want the walk refused, got %d occurrences", len(got))
	}
}

func TestOccurrencesPreservesWallClockAcrossDST(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	rule, err := ParseRRULE("FREQ=WEEKLY;BYDAY=SU")
	if err != nil {
		t.Fatalf("ParseRRULE: %v", err)
	}
	// US daylight saving began on 2024-03-10. A weekly 09:00 meeting stays at
	// 09:00 local rather than sliding to 08:00 or 10:00.
	start := time.Date(2024, time.March, 3, 9, 0, 0, 0, loc)
	got, truncated := rule.Occurrences(start, time.Time{}, time.Date(2024, time.March, 17, 23, 0, 0, 0, loc))
	if truncated {
		t.Fatal("expansion was truncated")
	}
	for _, o := range got {
		if h, m, _ := o.In(loc).Clock(); h != 9 || m != 0 {
			t.Errorf("occurrence %s drifted off the 09:00 wall clock", o.In(loc))
		}
	}
	if len(got) != 3 {
		t.Errorf("want 3 occurrences, got %d", len(got))
	}
}

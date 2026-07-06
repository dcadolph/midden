package cmd

import (
	"fmt"
	"testing"
	"time"
)

func TestImportTimestamp(t *testing.T) {
	t.Parallel()
	yesterday := time.Now().AddDate(0, 0, -1)
	tests := []struct {
		WantTime time.Time
		In       string
	}{{ // Test 0: Absolute dates resolve to noon local.
		In:       "2026-01-15",
		WantTime: time.Date(2026, 1, 15, 12, 0, 0, 0, time.Local),
	}, { // Test 1: Relative dates other than today resolve to noon local.
		In:       "yesterday",
		WantTime: time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 12, 0, 0, 0, time.Local),
	}, { // Test 2: Leap day resolves to noon on the exact date.
		In:       "2024-02-29",
		WantTime: time.Date(2024, 2, 29, 12, 0, 0, 0, time.Local),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := importTimestamp(test.In)
			if err != nil {
				t.Fatalf("importTimestamp(%q): %v", test.In, err)
			}
			if !got.Equal(test.WantTime) {
				t.Errorf("importTimestamp(%q) = %v, want %v", test.In, got, test.WantTime)
			}
		})
	}
}

func TestImportTimestampTodayKeepsClockTime(t *testing.T) {
	t.Parallel()
	before := time.Now()
	got, err := importTimestamp("today")
	if err != nil {
		t.Fatalf("importTimestamp(today): %v", err)
	}
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("importTimestamp(today) = %v, want between %v and %v", got, before, after)
	}
}

func TestImportTimestampRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "not-a-date", "2026/01/15"} {
		t.Run(fmt.Sprintf("input %q", in), func(t *testing.T) {
			t.Parallel()
			if _, err := importTimestamp(in); err == nil {
				t.Errorf("importTimestamp(%q) returned no error", in)
			}
		})
	}
}

package dateutil

import (
	"fmt"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	t.Parallel()
	ref := time.Date(2026, 6, 16, 14, 0, 0, 0, time.Local) // Tuesday
	tests := []struct {
		Want time.Time
		In   string
	}{{ // Test 0: today resolves to midnight on the reference day.
		In: "today", Want: time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
	}, { // Test 1: yesterday backs up one day.
		In: "yesterday", Want: time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local),
	}, { // Test 2: tomorrow advances one day.
		In: "tomorrow", Want: time.Date(2026, 6, 17, 0, 0, 0, 0, time.Local),
	}, { // Test 3: Absolute YYYY-MM-DD.
		In: "2026-01-01", Want: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
	}, { // Test 4: Past-week weekday resolves to most recent past instance.
		In: "monday", Want: time.Date(2026, 6, 15, 0, 0, 0, 0, time.Local),
	}, { // Test 5: Weekday matching reference day stays on same day without last- prefix.
		In: "tuesday", Want: time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
	}, { // Test 6: last-tuesday backs up a full week from the reference day.
		In: "last-tuesday", Want: time.Date(2026, 6, 9, 0, 0, 0, 0, time.Local),
	}, { // Test 7: N-days-ago.
		In: "3-days-ago", Want: time.Date(2026, 6, 13, 0, 0, 0, 0, time.Local),
	}, { // Test 8: N-weeks-ago.
		In: "2-weeks-ago", Want: time.Date(2026, 6, 2, 0, 0, 0, 0, time.Local),
	}, { // Test 9: N-months-ago.
		In: "1-month-ago", Want: time.Date(2026, 5, 16, 0, 0, 0, 0, time.Local),
	}, { // Test 10: N-years-ago.
		In: "1-year-ago", Want: time.Date(2025, 6, 16, 0, 0, 0, 0, time.Local),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := parseAt(test.In, ref)
			if err != nil {
				t.Fatalf("Parse(%q): %v", test.In, err)
			}
			if !got.Equal(test.Want) {
				t.Errorf("Parse(%q) = %v, want %v", test.In, got, test.Want)
			}
		})
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	t.Parallel()
	ref := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	cases := []string{"", "yesteryear", "2026/06/16", "march", "-1-days-ago", "0-zorps-ago"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if _, err := parseAt(in, ref); err == nil {
				t.Errorf("Parse(%q) returned no error", in)
			}
		})
	}
}

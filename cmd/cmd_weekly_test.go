package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/vault"
)

func TestFirstLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult string
		In         string
	}{{ // Test 0: Single line returns unchanged.
		In: "hello", WantResult: "hello",
	}, { // Test 1: First line of a multi-line body wins.
		In: "first\nsecond\nthird", WantResult: "first",
	}, { // Test 2: Leading blank lines are skipped.
		In: "\n\n  \nbody", WantResult: "body",
	}, { // Test 3: Surrounding whitespace is trimmed.
		In: "  padded  \nrest", WantResult: "padded",
	}, { // Test 4: All-blank input returns empty.
		In: " \n\t\n", WantResult: "",
	}, { // Test 5: Empty input returns empty.
		In: "", WantResult: "",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := firstLine(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEntriesOnDay(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	morning := vault.Entry{Time: day.Add(9 * time.Hour), Body: "morning"}
	night := vault.Entry{Time: day.Add(23*time.Hour + 59*time.Minute), Body: "night"}
	nextDay := vault.Entry{Time: day.AddDate(0, 0, 1), Body: "next day"}
	tests := []struct {
		WantResult []vault.Entry
		Entries    []vault.Entry
		Day        time.Time
	}{{ // Test 0: Empty input yields empty output.
		Entries: nil, Day: day, WantResult: nil,
	}, { // Test 1: Only entries on the target day are kept.
		Entries: []vault.Entry{morning, night, nextDay}, Day: day,
		WantResult: []vault.Entry{morning, night},
	}, { // Test 2: A time of day on the day argument does not change the result.
		Entries: []vault.Entry{morning, nextDay}, Day: day.Add(13 * time.Hour),
		WantResult: []vault.Entry{morning},
	}, { // Test 3: No entries on the day yields empty output.
		Entries: []vault.Entry{nextDay}, Day: day, WantResult: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := entriesOnDay(test.Entries, test.Day)
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDayStart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult time.Time
		In         time.Time
	}{{ // Test 0: Midday truncates to local midnight.
		In:         time.Date(2026, 6, 16, 15, 4, 5, 123, time.Local),
		WantResult: time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
	}, { // Test 1: Midnight stays unchanged.
		In:         time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
		WantResult: time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
	}, { // Test 2: One nanosecond before midnight stays on its own day.
		In:         time.Date(2026, 6, 16, 23, 59, 59, 999999999, time.Local),
		WantResult: time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := dayStart(test.In)
			if !got.Equal(test.WantResult) {
				t.Errorf("dayStart(%v) = %v, want %v", test.In, got, test.WantResult)
			}
		})
	}
}

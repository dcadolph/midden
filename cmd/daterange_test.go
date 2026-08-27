package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestResolveDateRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Since     string
		Until     string
		WantLabel string
		WantFrom  string
		WantTo    string
		Want      bool
		WantBound bool
	}{{ // Test 0: Both ends empty leaves the range open.
		WantLabel: "whole vault",
		WantBound: false,
	}, { // Test 1: A since bound alone leaves the far end open.
		Since:     "2024-03-01",
		WantFrom:  "2024-03-01 00:00:00",
		WantLabel: "since 2024-03-01",
		WantBound: true,
	}, { // Test 2: An until bound extends to the last instant of its day.
		Until:     "2024-03-31",
		WantTo:    "2024-03-31 23:59:59",
		WantLabel: "through 2024-03-31",
		WantBound: true,
	}, { // Test 3: Both bounds render as a span.
		Since:     "2024-03-01",
		Until:     "2024-03-31",
		WantFrom:  "2024-03-01 00:00:00",
		WantTo:    "2024-03-31 23:59:59",
		WantLabel: "2024-03-01 to 2024-03-31",
		WantBound: true,
	}, { // Test 4: A single day covers that whole day.
		Since:     "2024-03-15",
		Until:     "2024-03-15",
		WantFrom:  "2024-03-15 00:00:00",
		WantTo:    "2024-03-15 23:59:59",
		WantLabel: "2024-03-15 to 2024-03-15",
		WantBound: true,
	}, { // Test 5: An inverted range is rejected rather than silently returning nothing.
		Since: "2024-03-31",
		Until: "2024-03-01",
		Want:  true,
	}, { // Test 6: An unparseable since value is rejected.
		Since: "not-a-date",
		Want:  true,
	}, { // Test 7: An unparseable until value is rejected.
		Until: "someday",
		Want:  true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got, err := resolveDateRange(test.Since, test.Until)
			if test.Want {
				if err == nil {
					t.Fatalf("want error, got range %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveDateRange: %v", err)
			}
			if diff := cmp.Diff(test.WantBound, got.Bounded()); diff != "" {
				t.Errorf("Bounded mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantLabel, got.Label()); diff != "" {
				t.Errorf("Label mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantFrom, formatBound(got.From)); diff != "" {
				t.Errorf("From mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(test.WantTo, formatBound(got.To)); diff != "" {
				t.Errorf("To mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveDateRangeUntilCoversTrailingSecond(t *testing.T) {
	t.Parallel()
	got, err := resolveDateRange("", "2024-03-31")
	if err != nil {
		t.Fatalf("resolveDateRange: %v", err)
	}
	last := time.Date(2024, time.March, 31, 23, 59, 59, 999999999, time.Local)
	if got.To.Before(last) {
		t.Errorf("until bound %s excludes the final instant %s of its day", got.To, last)
	}
	if !got.To.Before(time.Date(2024, time.April, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("until bound %s spills into the next day", got.To)
	}
}

// formatBound renders a range bound for comparison, using an empty string for
// the zero time so an open end is distinguishable from a real timestamp.
func formatBound(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layoutDateTime)
}

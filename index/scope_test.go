package index

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSearchRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		From       time.Time
		To         time.Time
		WantBodies []string
		K          int
	}{{ // Test 0: An open range behaves like an unscoped search.
		K:          4,
		WantBodies: []string{"alpha", "gamma", "beta", "delta"},
	}, { // Test 1: Scoping excludes a closer match that falls outside the range.
		From:       day(2024, time.May, 1),
		K:          4,
		WantBodies: []string{"gamma", "delta"},
	}, { // Test 2: A closed range keeps only entries inside both bounds.
		From:       day(2024, time.March, 1),
		To:         day(2024, time.March, 31),
		K:          4,
		WantBodies: []string{"alpha", "beta"},
	}, { // Test 3: k truncates the ranked head after scoping.
		K:          2,
		WantBodies: []string{"alpha", "gamma"},
	}, { // Test 4: A non-positive k returns every candidate in range.
		From:       day(2024, time.March, 1),
		To:         day(2024, time.March, 31),
		WantBodies: []string{"alpha", "beta"},
	}, { // Test 5: A range holding no entries returns nothing.
		From:       day(2030, time.January, 1),
		K:          4,
		WantBodies: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			matches := digestFixture().SearchRange([]float32{1, 0}, test.K, test.From, test.To)
			got := make([]string, len(matches))
			for i, m := range matches {
				got[i] = m.Entry.Body
			}
			if diff := cmp.Diff(test.WantBodies, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		From       time.Time
		To         time.Time
		WantBodies []string
	}{{ // Test 0: An open range returns the whole corpus chronologically.
		WantBodies: []string{"alpha", "beta", "gamma", "delta"},
	}, { // Test 1: Bounds are inclusive at both ends.
		From:       day(2024, time.March, 3),
		To:         day(2024, time.May, 9),
		WantBodies: []string{"alpha", "beta", "gamma"},
	}, { // Test 2: An open end bounds only the near side.
		From:       day(2025, time.January, 1),
		WantBodies: []string{"delta"},
	}, { // Test 3: A range holding no entries returns nothing.
		To:         day(2000, time.January, 1),
		WantBodies: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			entries := digestFixture().InRange(test.From, test.To)
			got := make([]string, len(entries))
			for i, e := range entries {
				got[i] = e.Body
			}
			if diff := cmp.Diff(test.WantBodies, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInRangeOrdersUnsortedEntries(t *testing.T) {
	t.Parallel()
	idx := &Index{Entries: []Entry{
		{Time: day(2025, time.June, 1), Body: "late"},
		{Time: day(2020, time.June, 1), Body: "early"},
		{Time: day(2023, time.June, 1), Body: "middle"},
	}}
	got := make([]string, 0, 3)
	for _, e := range idx.InRange(time.Time{}, time.Time{}) {
		got = append(got, e.Body)
	}
	if diff := cmp.Diff([]string{"early", "middle", "late"}, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

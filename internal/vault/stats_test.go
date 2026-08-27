package vault

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/internal/util"
)

func TestComputeStats(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)
	got, err := v.ComputeStats(0, time.Time{})
	if err != nil {
		t.Fatalf("ComputeStats: %v", err)
	}
	want := Stats{
		Days:       2,
		Entries:    3,
		Words:      3,
		Tags:       2,
		FirstEntry: time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local),
		LastEntry:  time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local),
		// With no observation time nothing counts as scheduled, so the last past
		// entry is simply the last entry.
		LastPast: time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local),
		// The fixture's entries are all hand-written, so every one is authored.
		Authored:     3,
		LastAuthored: time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local),
		TopTags: []util.TagCount{
			{Tag: "life", Count: 1},
			{Tag: "project", Count: 1},
		},
	}
	if diff := cmp.Diff(want, got, cmp.Comparer(equalTimes)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTagCountsLimit(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)
	got, err := v.TagCounts(1)
	if err != nil {
		t.Fatalf("TagCounts: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 tag returned, got %d", len(got))
	}
}

func TestStreakCountsConsecutiveDays(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	today := time.Date(2026, 6, 16, 12, 0, 0, 0, time.Local)
	entries := []Entry{
		{Time: today, Body: "today"},
		{Time: today.AddDate(0, 0, -1), Body: "yesterday"},
		{Time: today.AddDate(0, 0, -2), Body: "two ago"},
		{Time: today.AddDate(0, 0, -4), Body: "broken streak before this"},
	}
	for _, e := range entries {
		if err := v.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	streak, err := v.Streak(today, nil)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if streak != 3 {
		t.Errorf("want streak 3, got %d", streak)
	}
}

func TestStreakIsZeroWhenTodayMissing(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	today := time.Date(2026, 6, 16, 12, 0, 0, 0, time.Local)
	if err := v.Append(Entry{Time: today.AddDate(0, 0, -1), Body: "yesterday"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	streak, err := v.Streak(today, nil)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if streak != 0 {
		t.Errorf("want streak 0, got %d", streak)
	}
}

func TestFlashbackMatchesMonthAndDay(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries := []Entry{
		{Time: time.Date(2024, 6, 16, 9, 0, 0, 0, time.Local), Body: "two years ago"},
		{Time: time.Date(2025, 6, 16, 9, 0, 0, 0, time.Local), Body: "one year ago"},
		{Time: time.Date(2025, 6, 17, 9, 0, 0, 0, time.Local), Body: "different day"},
		{Time: time.Date(2026, 6, 16, 9, 0, 0, 0, time.Local), Body: "today"},
	}
	for _, e := range entries {
		if err := v.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := v.Flashback(time.June, 16)
	if err != nil {
		t.Fatalf("Flashback: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 matching entries, got %d", len(got))
	}
}

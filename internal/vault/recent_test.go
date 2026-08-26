package vault

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// scheduledFixture returns a vault holding past entries plus a calendar
// appointment dated well into the future, which is what an imported calendar
// puts in a vault.
func scheduledFixture(t *testing.T) (*Vault, time.Time) {
	t.Helper()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	now := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.Local)
	entries := []Entry{
		{Time: now.AddDate(0, 0, -30), Body: "older"},
		{Time: now.AddDate(0, 0, -1), Body: "yesterday"},
		{Time: now.Add(-2 * time.Hour), Body: "this morning"},
		{Time: now.AddDate(0, 7, 0), Body: "dentist next spring"},
	}
	if err := v.AppendAll(entries); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	return v, now
}

func TestRecentBeforeExcludesScheduledEntries(t *testing.T) {
	t.Parallel()
	v, now := scheduledFixture(t)
	got, err := v.RecentBefore(2, now)
	if err != nil {
		t.Fatalf("RecentBefore: %v", err)
	}
	bodies := make([]string, len(got))
	for i, e := range got {
		bodies[i] = e.Body
	}
	// Without a cutoff the newest entry is an appointment that has not happened,
	// so a vault backfilled in August reports next spring as the latest thing.
	if diff := cmp.Diff([]string{"this morning", "yesterday"}, bodies); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestRecentBeforeZeroCutoffIncludesEverything(t *testing.T) {
	t.Parallel()
	v, _ := scheduledFixture(t)
	got, err := v.RecentBefore(1, time.Time{})
	if err != nil {
		t.Fatalf("RecentBefore: %v", err)
	}
	if len(got) != 1 || got[0].Body != "dentist next spring" {
		t.Errorf("want an open cutoff to reach the scheduled entry, got %v", got)
	}
}

func TestRecentKeepsOpenBehavior(t *testing.T) {
	t.Parallel()
	v, _ := scheduledFixture(t)
	got, err := v.Recent(1)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 1 || got[0].Body != "dentist next spring" {
		t.Errorf("want Recent to stay unbounded, got %v", got)
	}
}

func TestComputeStatsSeparatesScheduledFromPast(t *testing.T) {
	t.Parallel()
	v, now := scheduledFixture(t)
	s, err := v.ComputeStats(0, now)
	if err != nil {
		t.Fatalf("ComputeStats: %v", err)
	}
	if diff := cmp.Diff(1, s.Scheduled); diff != "" {
		t.Errorf("scheduled count mismatch (-want +got):\n%s", diff)
	}
	// The span a person means is what has happened, not what is booked.
	if !s.LastPast.Before(now) && !s.LastPast.Equal(now) {
		t.Errorf("want the last past entry at or before now, got %s", s.LastPast)
	}
	if !s.LastEntry.After(now) {
		t.Errorf("want the overall last entry to still reach the scheduled one, got %s", s.LastEntry)
	}
	if diff := cmp.Diff(4, s.Entries); diff != "" {
		t.Errorf("want every entry counted (-want +got):\n%s", diff)
	}
}

func TestComputeStatsZeroNowCountsNothingScheduled(t *testing.T) {
	t.Parallel()
	v, _ := scheduledFixture(t)
	s, err := v.ComputeStats(0, time.Time{})
	if err != nil {
		t.Fatalf("ComputeStats: %v", err)
	}
	if s.Scheduled != 0 {
		t.Errorf("want nothing scheduled without an observation time, got %d", s.Scheduled)
	}
}

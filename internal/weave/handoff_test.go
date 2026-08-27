package weave

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/internal/vault"
)

// tagged builds an entry carrying a source tag.
func tagged(year int, month time.Month, day int, tag, body string) vault.Entry {
	return vault.Entry{
		Time: time.Date(year, month, day, 12, 0, 0, 0, time.Local),
		Tags: []string{tag},
		Body: body,
	}
}

func TestHandoffsPairSuccessionsWithinOneSubject(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	// One child drops an activity in October and picks up another that December.
	entries = append(entries, weekly(2024, time.January, 6, 40, "William- martial arts")...)
	entries = append(entries, weekly(2024, time.December, 1, 10, "William- baseball practice")...)
	// A sibling starts something in the same window, which must not be paired.
	entries = append(entries, weekly(2024, time.December, 2, 10, "Hannah- choir")...)

	threads := Threads(entries, DefaultOptions(observed))
	got := Handoffs(threads, DefaultHandoffOptions())
	if len(got) == 0 {
		t.Fatal("want at least one succession")
	}
	for _, h := range got {
		if h.From.Label == "William- martial arts" && h.To.Label == "Hannah- choir" {
			t.Error("want siblings kept apart: one child stopping has nothing to do with another starting")
		}
	}
	found := false
	for _, h := range got {
		if h.From.Label == "William- martial arts" && h.To.Label == "William- baseball practice" {
			found = true
			if h.GapDays <= 0 {
				t.Errorf("want a positive gap between the ending and the beginning, got %d", h.GapDays)
			}
		}
	}
	if !found {
		t.Error("want the succession within one child's activities")
	}
}

func TestHandoffsRespectTheWindow(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, weekly(2023, time.January, 5, 20, "William- piano")...)
	// Begins years later, far outside any plausible succession.
	entries = append(entries, weekly(2026, time.June, 4, 10, "William- baseball")...)
	threads := Threads(entries, DefaultOptions(observed))
	if got := Handoffs(threads, HandoffOptions{Window: 60, SameSubject: SharesSubject}); len(got) != 0 {
		t.Errorf("want nothing paired across a multi-year gap, got %d", len(got))
	}
}

func TestSharesSubject(t *testing.T) {
	t.Parallel()
	a := Thread{Key: "arts martial william"}
	b := Thread{Key: "baseball william"}
	c := Thread{Key: "choir hannah"}
	if !SharesSubject(a, b) {
		t.Error("want threads naming the same child to share a subject")
	}
	if SharesSubject(a, c) {
		t.Error("want threads naming different children to be unrelated")
	}
}

func TestMilestonesAreChronological(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, weekly(2024, time.January, 6, 30, "William- martial arts")...)
	entries = append(entries, weekly(2026, time.July, 1, 8, "William- baseball")...)
	got := Milestones(Threads(entries, DefaultOptions(observed)))
	if len(got) < 2 {
		t.Fatalf("want an ending and a beginning, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].When.Before(got[i-1].When) {
			t.Errorf("milestones out of order at %d: %s before %s", i, got[i].When, got[i-1].When)
		}
	}
}

func TestRhythmCountsBySourcePerMonth(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		tagged(2026, time.July, 1, "calendar", "Dentist"),
		tagged(2026, time.July, 2, "calendar", "Piano"),
		tagged(2026, time.July, 3, "git", "Fix a bug"),
		tagged(2026, time.August, 1, "git", "Ship it"),
	}
	got := Rhythm(entries, []string{"calendar", "git"}, observed)
	if len(got) != 2 {
		t.Fatalf("want two months, got %d", len(got))
	}
	if diff := cmp.Diff("2026-07", got[0].Month); diff != "" {
		t.Errorf("months out of order (-want +got):\n%s", diff)
	}
	if got[0].Counts["calendar"] != 2 || got[0].Counts["git"] != 1 {
		t.Errorf("July counts wrong: %v", got[0].Counts)
	}
	if got[0].Total != 3 {
		t.Errorf("want a July total of 3, got %d", got[0].Total)
	}
}

func TestOverlapsFindDaysWhereSourcesMeet(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		tagged(2026, time.July, 30, "calendar", "Dad- off work/ vacation"),
		tagged(2026, time.July, 30, "git", "Trim projects"),
		tagged(2026, time.July, 30, "git", "Alphabetize projects"),
		// A day carrying only one source is not a crossing.
		tagged(2026, time.July, 31, "git", "Ship it"),
	}
	got := Overlaps(entries, []string{"calendar", "git"}, observed)
	if len(got) != 1 {
		t.Fatalf("want exactly the one day both sources recorded, got %d", len(got))
	}
	if got[0].Counts["git"] != 2 || got[0].Counts["calendar"] != 1 {
		t.Errorf("counts wrong: %v", got[0].Counts)
	}
	// The headline is what makes a crossing legible: neither source alone says
	// the day was spent working through a vacation.
	if got[0].Headlines["calendar"] != "Dad- off work/ vacation" {
		t.Errorf("want the calendar headline preserved, got %q", got[0].Headlines["calendar"])
	}
}

func TestOverlapsIgnoreFutureDays(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		tagged(2027, time.March, 12, "calendar", "Future appointment"),
		tagged(2027, time.March, 12, "git", "Future commit"),
	}
	if got := Overlaps(entries, []string{"calendar", "git"}, observed); len(got) != 0 {
		t.Errorf("want future days excluded, got %d", len(got))
	}
}

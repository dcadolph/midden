package weave

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/vault"
)

// on builds an entry with the given body on the given date.
func on(year int, month time.Month, day int, body string) vault.Entry {
	return vault.Entry{Time: time.Date(year, month, day, 12, 0, 0, 0, time.Local), Body: body}
}

// weekly builds n entries seven days apart starting at the given date.
func weekly(year int, month time.Month, day, n int, body string) []vault.Entry {
	out := make([]vault.Entry, 0, n)
	start := time.Date(year, month, day, 12, 0, 0, 0, time.Local)
	for i := range n {
		d := start.AddDate(0, 0, 7*i)
		out = append(out, vault.Entry{Time: d, Body: body})
	}
	return out
}

// observed is a fixed observation date, so classification never depends on when
// the suite runs.
var observed = time.Date(2026, time.August, 26, 12, 0, 0, 0, time.Local)

// find returns the thread whose label matches, or fails the test.
func find(t *testing.T, threads []Thread, label string) Thread {
	t.Helper()
	for _, th := range threads {
		if th.Label == label {
			return th
		}
	}
	t.Fatalf("no thread labeled %q in %d threads", label, len(threads))
	return Thread{}
}

func TestNormalizeTitleMergesPhrasingVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   []string
		Same bool
	}{{ // Test 0: A handwritten commitment survives separator and preposition drift.
		Name: "sleepover variants",
		In: []string{
			"Hannah- sleepover with Kayla",
			"Hannah- sleepover @ Kayla's",
			"Hannah - sleepover at Kayla’s  (18:00 to 13:00)",
			"Hannah- sleepover w/ Kayla",
		},
		Same: true,
	}, { // Test 1: Case and emoji do not split a thread.
		Name: "case and emoji",
		In:   []string{"William- Martial arts", "William- martial arts ⚔️", "WILLIAM - MARTIAL ARTS"},
		Same: true,
	}, { // Test 2: A trailing field or grade number does not split a thread.
		Name: "trailing number",
		In:   []string{"William- baseball practice (field 3)", "William- baseball practice field 4"},
		Same: true,
	}, { // Test 3: Genuinely different activities stay apart.
		Name: "distinct activities",
		In:   []string{"Hannah- hip hop", "Hannah- acro"},
		Same: false,
	}, { // Test 4: The same activity for different people stays apart.
		Name: "different subjects",
		In:   []string{"William- piano", "Haylie- piano"},
		Same: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			first := NormalizeTitle(test.In[0])
			for _, other := range test.In[1:] {
				got := NormalizeTitle(other)
				if test.Same && got != first {
					t.Errorf("want %q and %q to share a key, got %q vs %q", test.In[0], other, first, got)
				}
				if !test.Same && got == first {
					t.Errorf("want %q and %q to stay separate, both keyed %q", test.In[0], other, got)
				}
			}
		})
	}
}

func TestThreadsClassifiesByOwnCadence(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	// A weekly class that stopped a year ago.
	entries = append(entries, weekly(2024, time.January, 6, 40, "William- martial arts")...)
	// A weekly class still running up to the observation date.
	entries = append(entries, weekly(2026, time.June, 3, 12, "William- baseball practice")...)
	// A yearly tradition, which must not read as ended merely because a year passed.
	for y := 2022; y <= 2026; y++ {
		entries = append(entries, on(y, time.March, 4, "Dad birthday"))
	}
	opts := DefaultOptions(observed)
	threads := Threads(entries, opts)

	if got := find(t, threads, "William- martial arts"); got.Status != Ended {
		t.Errorf("want a long-silent weekly class to read as ended, got %s (silent %d days)", got.Status, got.SilentDays)
	}
	if got := find(t, threads, "William- baseball practice"); got.Status == Ended {
		t.Errorf("want a current weekly class to read as live, got %s", got.Status)
	}
	// A yearly gap is normal for a yearly thread, which is the whole reason
	// silence is measured against cadence rather than a fixed window.
	if got := find(t, threads, "Dad birthday"); got.Status == Ended {
		t.Errorf("want a yearly tradition to survive a yearly gap, got ended (gap %d days)", got.MedianGap)
	}
}

func TestThreadsIgnoresFutureEntries(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, weekly(2026, time.August, 5, 3, "Dentist")...)
	// Calendars hold future appointments; counting them would report activity
	// that has not happened.
	entries = append(entries, on(2027, time.March, 12, "Dentist"))
	threads := Threads(entries, Options{Now: observed, MinCount: 2, SilenceFactor: 4, MinSilenceDays: 90})
	got := find(t, threads, "Dentist")
	if got.Count != 3 {
		t.Errorf("want 3 past occurrences, got %d", got.Count)
	}
	if got.Last.After(observed) {
		t.Errorf("want the last occurrence on or before the observation date, got %s", got.Last)
	}
}

func TestThreadsHonorsMinCount(t *testing.T) {
	t.Parallel()
	entries := append(weekly(2026, time.July, 1, 6, "Recurring"), on(2026, time.July, 2, "One off"))
	threads := Threads(entries, DefaultOptions(observed))
	for _, th := range threads {
		if th.Label == "One off" {
			t.Error("want a single occurrence excluded from threads")
		}
	}
	find(t, threads, "Recurring")
}

func TestMergeSimilarFoldsExtraWords(t *testing.T) {
	t.Parallel()
	// The same standing arrangement, recorded once with an extra name attached.
	entries := []vault.Entry{
		on(2025, time.January, 4, "Hannah- sleepover with Kayla"),
		on(2025, time.February, 8, "Hannah- sleepover @ Kayla's"),
		on(2025, time.March, 15, "Hannah- sleepover w/ Kayla"),
		on(2025, time.April, 19, "Hannah- sleepover with Kayla"),
		on(2025, time.May, 24, "Hannah- Kayla sleepover"),
	}
	threads := Threads(entries, Options{
		Now: observed, MinCount: 5, SilenceFactor: 4, MinSilenceDays: 90, MergeSimilarity: 0.6,
	})
	if len(threads) != 1 {
		t.Fatalf("want one merged thread, got %d: %v", len(threads), labelsOf(threads))
	}
	if threads[0].Count != 5 {
		t.Errorf("want all 5 occurrences merged, got %d", threads[0].Count)
	}
}

func TestMergeSimilarKeepsDistinctThreadsApart(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, weekly(2026, time.January, 6, 6, "Hannah- hip hop")...)
	entries = append(entries, weekly(2026, time.January, 7, 6, "William- baseball practice")...)
	threads := Threads(entries, DefaultOptions(observed))
	if len(threads) != 2 {
		t.Errorf("want two distinct threads, got %d: %v", len(threads), labelsOf(threads))
	}
}

func TestMedianGapDays(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	times := []time.Time{base, base.AddDate(0, 0, 7), base.AddDate(0, 0, 14), base.AddDate(0, 0, 21)}
	if diff := cmp.Diff(7, medianGapDays(times)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(0, medianGapDays(times[:1])); diff != "" {
		t.Errorf("single occurrence should have no gap (-want +got):\n%s", diff)
	}
}

func TestWeightPrefersLongRunningThreads(t *testing.T) {
	t.Parallel()
	long := Thread{Count: 50, SpanDays: 700}
	burst := Thread{Count: 50, SpanDays: 20}
	// Two threads of equal volume are not equal in a life: the one held for two
	// years mattered more than the one that filled three weeks.
	if long.Weight() <= burst.Weight() {
		t.Errorf("want the sustained thread to outrank the burst, got %d vs %d", long.Weight(), burst.Weight())
	}
}

func TestThreadsEmptyInput(t *testing.T) {
	t.Parallel()
	if diff := cmp.Diff([]Thread(nil), Threads(nil, DefaultOptions(observed)), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// labelsOf returns thread labels for failure messages.
func labelsOf(threads []Thread) []string {
	out := make([]string, len(threads))
	for i, t := range threads {
		out[i] = t.Label
	}
	return out
}

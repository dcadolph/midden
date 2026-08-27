package weave

import (
	"testing"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// monthly builds n entries spread through the given month.
func monthly(year int, month time.Month, n int) []vault.Entry {
	out := make([]vault.Entry, 0, n)
	for i := range n {
		day := 1 + i%27
		out = append(out, vault.Entry{
			Time: time.Date(year, month, day, 12, 0, 0, 0, time.Local),
			Body: "entry",
		})
	}
	return out
}

// span builds entries across every month from one date to another.
func span(fromYear int, fromMonth time.Month, months, perMonth int) []vault.Entry {
	var out []vault.Entry
	cur := time.Date(fromYear, fromMonth, 1, 0, 0, 0, 0, time.Local)
	for range months {
		out = append(out, monthly(cur.Year(), cur.Month(), perMonth)...)
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func TestGapsFindAnInteriorSilence(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2015, time.January, 12, 20)...) // active
	// 2016 through 2017 silent
	entries = append(entries, span(2018, time.January, 12, 20)...) // active again
	got := Gaps(entries, DefaultGapOptions(time.Date(2019, time.January, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 1 {
		t.Fatalf("want one silence, got %d: %+v", len(got), got)
	}
	if got[0].Months != 24 {
		t.Errorf("want a 24 month silence, got %d", got[0].Months)
	}
	if got[0].Entries != 0 {
		t.Errorf("want nothing inside the silence, got %d", got[0].Entries)
	}
	if got[0].Before <= 0 || got[0].After <= 0 {
		t.Errorf("want activity measured on both sides, got %.1f and %.1f", got[0].Before, got[0].After)
	}
}

func TestGapsIgnoreTheEdgesOfARecord(t *testing.T) {
	t.Parallel()
	// A record is quiet before it starts and after it ends by definition.
	// Reporting those as holes would say nothing about the life.
	entries := span(2020, time.January, 12, 20)
	got := Gaps(entries, DefaultGapOptions(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 0 {
		t.Errorf("want no silence for a record that simply ended, got %+v", got)
	}
}

func TestGapsHonorMinMonths(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2020, time.January, 6, 20)...)
	// A three month lull, shorter than the floor.
	entries = append(entries, span(2020, time.October, 6, 20)...)
	opts := DefaultGapOptions(time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local))
	if got := Gaps(entries, opts); len(got) != 0 {
		t.Errorf("want a short lull ignored, got %+v", got)
	}
	opts.MinMonths = 3
	if got := Gaps(entries, opts); len(got) != 1 {
		t.Errorf("want the lull found at a lower floor, got %d", len(got))
	}
}

func TestGapsToleratesSparseMonthsInsideASilence(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2015, time.January, 12, 40)...)
	// Two stray entries during the quiet stretch, which is what a real silence
	// looks like: not empty, just nearly so.
	entries = append(entries, monthly(2016, time.June, 1)...)
	entries = append(entries, monthly(2017, time.March, 1)...)
	entries = append(entries, span(2018, time.January, 12, 40)...)
	got := Gaps(entries, DefaultGapOptions(time.Date(2019, time.January, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 1 {
		t.Fatalf("want the near-silence still found, got %d: %+v", len(got), got)
	}
	if got[0].Entries != 2 {
		t.Errorf("want the stray entries counted inside it, got %d", got[0].Entries)
	}
}

func TestGapsIgnoreFutureMonths(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2025, time.January, 6, 20)...)
	// A scheduled appointment far ahead must not open a silence between now and it.
	entries = append(entries, monthly(2028, time.March, 1)...)
	got := Gaps(entries, DefaultGapOptions(time.Date(2026, time.August, 26, 0, 0, 0, 0, time.Local)))
	if len(got) != 0 {
		t.Errorf("want future months excluded, got %+v", got)
	}
}

func TestGapsEmptyRecord(t *testing.T) {
	t.Parallel()
	if got := Gaps(nil, DefaultGapOptions(time.Now())); len(got) != 0 {
		t.Errorf("want nothing from an empty record, got %+v", got)
	}
}

func TestGapsOrderLongestFirst(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2010, time.January, 6, 20)...)
	entries = append(entries, span(2011, time.July, 6, 20)...) // after a 12 month gap
	entries = append(entries, span(2014, time.July, 6, 20)...) // after a 30 month gap
	got := Gaps(entries, DefaultGapOptions(time.Date(2015, time.June, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 2 {
		t.Fatalf("want two silences, got %d", len(got))
	}
	if got[0].Months < got[1].Months {
		t.Errorf("want the longer silence first, got %d then %d", got[0].Months, got[1].Months)
	}
}

func TestGapsBySourceFindsASilenceOneSourceMasks(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	// The calendar goes quiet for two years while commits flood the same months.
	for _, e := range span(2022, time.January, 12, 8) {
		e.Tags = []string{"calendar"}
		entries = append(entries, e)
	}
	for _, e := range span(2025, time.January, 12, 8) {
		e.Tags = []string{"calendar"}
		entries = append(entries, e)
	}
	for _, e := range span(2022, time.January, 48, 30) {
		e.Tags = []string{"git"}
		entries = append(entries, e)
	}
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	// The whole record never goes quiet, which is exactly how one loud source
	// hides the silence in another.
	if got := Gaps(entries, DefaultGapOptions(now)); len(got) != 0 {
		t.Fatalf("precondition: want no whole-record silence, got %+v", got)
	}
	got := GapsBySource(entries, []string{"calendar", "git"}, DefaultGapOptions(now))
	if len(got) != 1 {
		t.Fatalf("want the masked calendar silence found, got %d: %+v", len(got), got)
	}
	if got[0].Source != "calendar" {
		t.Errorf("want the silence attributed to the calendar, got %q", got[0].Source)
	}
	if got[0].Months < 20 {
		t.Errorf("want roughly two years of silence, got %d months", got[0].Months)
	}
}

func TestGapsBySourceDoesNotRepeatAWholeRecordSilence(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	// Every source is quiet over the same stretch, so the whole-record gap
	// already covers it and per-source copies would ask the same question twice.
	for _, e := range span(2020, time.January, 12, 10) {
		e.Tags = []string{"calendar"}
		entries = append(entries, e)
	}
	for _, e := range span(2023, time.January, 12, 10) {
		e.Tags = []string{"calendar"}
		entries = append(entries, e)
	}
	now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.Local)
	got := GapsBySource(entries, []string{"calendar"}, DefaultGapOptions(now))
	whole, perSource := 0, 0
	for _, g := range got {
		if g.Source == "" {
			whole++
		} else {
			perSource++
		}
	}
	if whole != 1 || perSource != 0 {
		t.Errorf("want one whole-record silence and no per-source copy, got %d and %d", whole, perSource)
	}
}

func TestErasFindTheChapters(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	entries = append(entries, span(2013, time.January, 48, 4)...)  // sparse years
	entries = append(entries, span(2017, time.January, 48, 0)...)  // silence
	entries = append(entries, span(2021, time.January, 48, 40)...) // dense years
	got := Eras(entries, DefaultEraOptions(time.Date(2025, time.January, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 3 {
		t.Fatalf("want three eras, got %d: %+v", len(got), got)
	}
	// The middle era is the silence and its boundaries have to land on the year
	// marks, give or take a month of segmentation slack.
	mid := got[1]
	if mid.PerMonth > 1 {
		t.Errorf("want the middle era near zero volume, got %.1f/month", mid.PerMonth)
	}
	if mid.From.Year() != 2017 || mid.To.Year() != 2020 {
		t.Errorf("want the silence spanning 2017-2020, got %s to %s", mid.From, mid.To)
	}
	if got[0].PerMonth >= got[2].PerMonth {
		t.Errorf("want the last era denser than the first, got %.1f vs %.1f", got[0].PerMonth, got[2].PerMonth)
	}
}

func TestErasDoNotShatterAFlatRecord(t *testing.T) {
	t.Parallel()
	// A steady record has one chapter, and a greedy splitter with no brake
	// would cut it into MaxEras pieces anyway.
	entries := span(2020, time.January, 48, 20)
	got := Eras(entries, DefaultEraOptions(time.Date(2024, time.February, 1, 0, 0, 0, 0, time.Local)))
	if len(got) != 1 {
		t.Errorf("want one era for a flat record, got %d: %+v", len(got), got)
	}
}

func TestErasEmptyAndTinyRecords(t *testing.T) {
	t.Parallel()
	if got := Eras(nil, DefaultEraOptions(time.Now())); got != nil {
		t.Errorf("want nil for an empty record, got %+v", got)
	}
	if got := Eras(monthly(2026, time.March, 5), DefaultEraOptions(time.Now())); got != nil {
		t.Errorf("want nil for a record shorter than two eras, got %+v", got)
	}
}

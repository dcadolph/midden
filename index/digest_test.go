package index

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/util"
)

// day returns a local timestamp for the given calendar date at noon.
func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 12, 0, 0, 0, time.Local)
}

// digestFixture is a small corpus spanning three months across two years.
func digestFixture() *Index {
	return &Index{
		Provider: "test",
		Dim:      2,
		Entries: []Entry{
			{Time: day(2024, time.March, 3), Tags: []string{"work"}, Body: "alpha", Embedding: []float32{1, 0}},
			{Time: day(2024, time.March, 20), Tags: []string{"Work", "travel"}, Body: "beta", Embedding: []float32{0, 1}},
			{Time: day(2024, time.May, 9), Tags: []string{"home"}, Body: "gamma", Embedding: []float32{1, 1}},
			{Time: day(2025, time.January, 15), Tags: []string{"work"}, Body: "delta", Embedding: []float32{-1, 0}},
		},
	}
}

func TestDigest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		From     time.Time
		To       time.Time
		WantSpan string
		Want     Digest
		TopTags  int
	}{{ // Test 0: Open range covers the whole corpus and folds tag case.
		TopTags: 0,
		Want: Digest{
			Entries: 4,
			First:   day(2024, time.March, 3),
			Last:    day(2025, time.January, 15),
			TopTags: []util.TagCount{{Tag: "work", Count: 3}, {Tag: "home", Count: 1}, {Tag: "travel", Count: 1}},
			Months: []MonthCount{
				{Month: "2024-03", Count: 2},
				{Month: "2024-05", Count: 1},
				{Month: "2025-01", Count: 1},
			},
		},
	}, { // Test 1: A bounded range counts only the entries inside it.
		From:    day(2024, time.March, 1),
		To:      day(2024, time.March, 31),
		TopTags: 0,
		Want: Digest{
			Entries: 2,
			First:   day(2024, time.March, 3),
			Last:    day(2024, time.March, 20),
			TopTags: []util.TagCount{{Tag: "work", Count: 2}, {Tag: "travel", Count: 1}},
			Months:  []MonthCount{{Month: "2024-03", Count: 2}},
		},
	}, { // Test 2: An open start bounds only the far end.
		To:      day(2024, time.March, 31),
		TopTags: 1,
		Want: Digest{
			Entries: 2,
			First:   day(2024, time.March, 3),
			Last:    day(2024, time.March, 20),
			TopTags: []util.TagCount{{Tag: "work", Count: 2}},
			Months:  []MonthCount{{Month: "2024-03", Count: 2}},
		},
	}, { // Test 3: A range holding no entries reports zero rather than the whole corpus.
		From:    day(2030, time.January, 1),
		TopTags: 0,
		Want:    Digest{},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := digestFixture().Digest(test.From, test.To, test.TopTags)
			if diff := cmp.Diff(test.Want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDigestEmptyIndex(t *testing.T) {
	t.Parallel()
	got := (&Index{}).Digest(time.Time{}, time.Time{}, 10)
	if diff := cmp.Diff(Digest{}, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

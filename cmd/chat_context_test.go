package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/internal/util"
)

// chunkEntry builds an indexed entry dated d days after 2024-01-01 with a body
// of the given length.
func chunkEntry(d, size int) index.Entry {
	return index.Entry{
		Time: time.Date(2024, time.January, 1, 12, 0, 0, 0, time.Local).AddDate(0, 0, d),
		Body: strings.Repeat("x", size),
	}
}

func TestChunkEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantSizes []int
		In        []index.Entry
		Budget    int
	}{{ // Test 0: Entries that fit the budget stay in one chunk.
		In:        []index.Entry{chunkEntry(0, 10), chunkEntry(1, 10)},
		Budget:    100,
		WantSizes: []int{2},
	}, { // Test 1: The chunk breaks before the entry that would exceed the budget.
		In:        []index.Entry{chunkEntry(0, 40), chunkEntry(1, 40), chunkEntry(2, 40)},
		Budget:    100,
		WantSizes: []int{2, 1},
	}, { // Test 2: An entry larger than the budget becomes a chunk of one rather than being dropped.
		In:        []index.Entry{chunkEntry(0, 10), chunkEntry(1, 500), chunkEntry(2, 10)},
		Budget:    100,
		WantSizes: []int{1, 1, 1},
	}, { // Test 3: No entries produce no chunks.
		In:        nil,
		Budget:    100,
		WantSizes: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			chunks := chunkEntries(test.In, test.Budget)
			got := make([]int, len(chunks))
			total := 0
			for i, c := range chunks {
				got[i] = len(c)
				total += len(c)
			}
			if diff := cmp.Diff(test.WantSizes, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("chunk sizes mismatch (-want +got):\n%s", diff)
			}
			if total != len(test.In) {
				t.Errorf("chunking dropped entries: want %d total, got %d", len(test.In), total)
			}
		})
	}
}

func TestChunkLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Want string
		In   []index.Entry
	}{{ // Test 0: A multi-day chunk renders as a span.
		In:   []index.Entry{chunkEntry(0, 1), chunkEntry(10, 1)},
		Want: "2024-01-01 to 2024-01-11",
	}, { // Test 1: A single-day chunk renders as one date.
		In:   []index.Entry{chunkEntry(0, 1)},
		Want: "2024-01-01",
	}, { // Test 2: An empty chunk is labeled rather than panicking.
		In:   nil,
		Want: "empty range",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(test.Want, chunkLabel(test.In)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderDigestCarriesWholeCorpusCounts(t *testing.T) {
	t.Parallel()
	d := index.Digest{
		Entries: 4213,
		First:   time.Date(2015, time.June, 2, 9, 0, 0, 0, time.Local),
		Last:    time.Date(2026, time.August, 24, 18, 0, 0, 0, time.Local),
		TopTags: []util.TagCount{{Tag: "calendar", Count: 3900}, {Tag: "work", Count: 210}},
		Months:  []index.MonthCount{{Month: "2015-06", Count: 12}, {Month: "2026-08", Count: 40}},
	}
	got := renderDigest(d, "whole vault")
	for _, want := range []string{
		"every indexed entry in scope (whole vault)",
		"Entries: 4213",
		"Span: 2015-06-02 to 2026-08-24",
		"calendar (3900)",
		"2026-08 (40)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("digest missing %q:\n%s", want, got)
		}
	}
}

func TestRenderDigestEmptyRange(t *testing.T) {
	t.Parallel()
	got := renderDigest(index.Digest{}, "since 2030-01-01")
	if !strings.Contains(got, "Entries: 0") {
		t.Errorf("want a zero count, got:\n%s", got)
	}
	if strings.Contains(got, "Span:") {
		t.Errorf("want no span for an empty range, got:\n%s", got)
	}
}

func TestEntriesSize(t *testing.T) {
	t.Parallel()
	got := entriesSize([]index.Entry{chunkEntry(0, 10), chunkEntry(1, 25)})
	if diff := cmp.Diff(35, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestRenderEntriesIncludesDateAndTags(t *testing.T) {
	t.Parallel()
	e := index.Entry{
		Time: time.Date(2024, time.March, 3, 14, 30, 0, 0, time.Local),
		Tags: []string{"work", "travel"},
		Body: "flew to Denver",
	}
	got := renderEntries([]index.Entry{e})
	for _, want := range []string{"2024-03-03 14:30:00", "[work, travel]", "flew to Denver"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered entry missing %q:\n%s", want, got)
		}
	}
}

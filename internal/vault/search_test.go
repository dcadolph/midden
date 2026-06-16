package vault

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRecentReturnsNewestFirst(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)

	got, err := v.Recent(2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	want := []Entry{
		{Time: time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local), Body: "third"},
		{Time: time.Date(2026, 6, 16, 14, 32, 1, 0, time.Local), Body: "second", Tags: []string{"life"}},
	}
	if diff := cmp.Diff(want, got, cmp.Comparer(equalTimes)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestSearchFindsBodyAndTags(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)

	got, err := v.Search("LIFE")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 hit, got %d", len(got))
	}
	if got[0].Body != "second" {
		t.Errorf("want body 'second', got %q", got[0].Body)
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := v.Search("   "); err == nil {
		t.Fatal("Search with empty query returned no error")
	}
}

func TestWithTagFiltersExactTag(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)

	got, err := v.WithTag("project")
	if err != nil {
		t.Fatalf("WithTag: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 hit, got %d", len(got))
	}
	if got[0].Body != "first" {
		t.Errorf("want body 'first', got %q", got[0].Body)
	}
}

func TestReadRangeIsInclusive(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)
	from := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)

	got, err := v.ReadRange(from, to)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 entries on 2026-06-16, got %d", len(got))
	}
}

func TestReadRangeSwapsReversedBounds(t *testing.T) {
	t.Parallel()
	v := seedFixture(t)
	from := time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)

	got, err := v.ReadRange(from, to)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want 3 entries across full range, got %d", len(got))
	}
}

// seedFixture builds a vault with three entries across two days and returns the vault handle.
func seedFixture(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries := []Entry{
		{Time: time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local), Tags: []string{"project"}, Body: "first"},
		{Time: time.Date(2026, 6, 16, 14, 32, 1, 0, time.Local), Tags: []string{"life"}, Body: "second"},
		{Time: time.Date(2026, 6, 18, 10, 0, 0, 0, time.Local), Body: "third"},
	}
	for _, e := range entries {
		if err := v.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	return v
}

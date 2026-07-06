package vault

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSerializeParseRoundTrip(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	tests := []struct {
		Body string
		Tags []string
	}{
		// Test 0: Plain body.
		{Body: "walked the dog"},
		// Test 1: Body line that looks like an entry header.
		{Body: "notes from standup\n## 10:00:00 fake header line\nmore text"},
		// Test 2: Body line that looks like a day header.
		{Body: "# 2026-07-01\nthat date line above is body text"},
		// Test 3: Markdown heading that is not a header pattern survives untouched.
		{Body: "# Notes\n- one\n- two"},
		// Test 4: Tagged entry with blank interior line.
		{Body: "first paragraph\n\nsecond paragraph", Tags: []string{"work", "deep"}},
	}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			entry := Entry{
				Time: time.Date(2026, 7, 1, 9, 30, 0, 0, time.Local),
				Tags: test.Tags,
				Body: test.Body,
			}
			data := []byte("# 2026-07-01\n\n" + entry.Serialize())
			got, err := parseDayBytes(day, "test.md", data)
			if err != nil {
				t.Fatalf("parseDayBytes: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("want exactly 1 entry back, got %d: %+v", len(got), got)
			}
			if diff := cmp.Diff(entry.Tags, got[0].Tags, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("tags mismatch (-want +got):\n%s", diff)
			}
			if !got[0].Time.Equal(entry.Time) {
				t.Errorf("time mismatch: want %v got %v", entry.Time, got[0].Time)
			}
		})
	}
}

func TestParseDayBytesToleratesBadEntryTime(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	data := []byte("# 2026-07-01\n\n## 09:00:00\nreal entry\n\n## 25:99:99 #broken\nnot a valid time\n")
	got, err := parseDayBytes(day, "test.md", data)
	if err != nil {
		t.Fatalf("parseDayBytes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if !strings.Contains(got[0].Body, "real entry") || !strings.Contains(got[0].Body, "25:99:99") {
		t.Errorf("malformed header must fold into the previous body, got %q", got[0].Body)
	}
}

func TestRecentSortsWithinDay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := &Vault{Dir: dir}
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	// Append out of chronological order, as a calendar import does.
	for _, hour := range []int{15, 9, 12} {
		entry := Entry{Time: day.Add(time.Duration(hour) * time.Hour), Body: fmt.Sprintf("h%d", hour)}
		if err := v.Append(entry); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := v.Recent(2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	want := []string{"h15", "h12"}
	bodies := []string{got[0].Body, got[1].Body}
	if diff := cmp.Diff(want, bodies); diff != "" {
		t.Errorf("recent order mismatch (-want +got):\n%s", diff)
	}
}

package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestReadDayParsesEntries(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	day := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	path, err := v.EnsureDayFile(day)
	if err != nil {
		t.Fatalf("EnsureDayFile: %v", err)
	}
	contents := "# 2026-06-16\n\n" +
		"## 09:14:23 #project #idea\n" +
		"First entry.\nWith two lines.\n\n" +
		"## 14:32:01\n" +
		"Second entry.\n\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := v.ReadDay(day)
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	want := []Entry{
		{
			Time: time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local),
			Tags: []string{"project", "idea"},
			Body: "First entry.\nWith two lines.",
		},
		{
			Time: time.Date(2026, 6, 16, 14, 32, 1, 0, time.Local),
			Body: "Second entry.",
		},
	}
	if diff := cmp.Diff(want, got, cmp.Comparer(equalTimes)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestReadDayMissingFile(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := v.ReadDay(time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty entries, got %d", len(got))
	}
}

func TestListDaysSortsAscending(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	days := []time.Time{
		time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local),
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local),
	}
	for _, d := range days {
		if _, err := v.EnsureDayFile(d); err != nil {
			t.Fatalf("EnsureDayFile %v: %v", d, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(v.Dir, "noise"), 0o755); err != nil {
		t.Fatalf("mkdir noise: %v", err)
	}

	got, err := v.ListDays()
	if err != nil {
		t.Fatalf("ListDays: %v", err)
	}
	want := []time.Time{
		time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local),
		time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local),
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local),
	}
	if diff := cmp.Diff(want, got, cmp.Comparer(equalTimes)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestExtractTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult []string
		In         string
	}{{ // Test 0: No trailing text returns nil.
		In:         "",
		WantResult: nil,
	}, { // Test 1: Single tag.
		In:         " #project",
		WantResult: []string{"project"},
	}, { // Test 2: Multiple tags including dashes and digits.
		In:         "  #project-x #idea2 #life",
		WantResult: []string{"project-x", "idea2", "life"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := extractTags(test.In)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// equalTimes compares two time.Time values by their absolute instant, ignoring monotonic clock fields.
func equalTimes(a, b time.Time) bool {
	return a.Equal(b)
}

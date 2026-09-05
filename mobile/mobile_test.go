package mobile

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// openTestVault returns a Vault rooted in a fresh temp directory.
func openTestVault(t *testing.T) *Vault {
	t.Helper()
	v, err := Open(t.TempDir(), "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return v
}

// decodeEntries parses the wire JSON array produced by the Vault methods.
func decodeEntries(t *testing.T, data string) []jsonEntry {
	t.Helper()
	var entries []jsonEntry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		t.Fatalf("unmarshal %q: %v", data, err)
	}
	return entries
}

func TestAppendAtRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Timestamp   string
		Tags        string
		Body        string
		WantDate    string
		WantEntries []jsonEntry
		Want        error
	}{{ // Test 0: A plain entry round-trips through DayJSON.
		Timestamp: "2026-09-01T08:15:00",
		Body:      "Walked the trail before work.",
		WantDate:  "2026-09-01",
		WantEntries: []jsonEntry{{
			Time: "2026-09-01T08:15:00", Body: "Walked the trail before work.",
		}},
	}, { // Test 1: Comma-separated tags are normalized.
		Timestamp: "2026-09-02T19:00:00",
		Tags:      " #voice , garden ,, ",
		Body:      "Planted the fall garlic.",
		WantDate:  "2026-09-02",
		WantEntries: []jsonEntry{{
			Time: "2026-09-02T19:00:00", Tags: []string{"voice", "garden"}, Body: "Planted the fall garlic.",
		}},
	}, { // Test 2: A bad timestamp is rejected.
		Timestamp: "yesterday-ish", Body: "x", Want: errBadInput,
	}, { // Test 3: A timestamp carrying a zone offset is rejected.
		Timestamp: "2026-09-03T10:00:00-05:00", Body: "x", Want: errBadInput,
	}, { // Test 4: An empty body is rejected.
		Timestamp: "2026-09-03T10:00:00", Body: "  ", Want: errBadInput,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			v := openTestVault(t)
			err := v.AppendAt(test.Timestamp, test.Tags, test.Body)
			if (err != nil) != (test.Want != nil) {
				t.Fatalf("AppendAt error = %v, want error %t", err, test.Want != nil)
			}
			if test.Want != nil {
				return
			}
			day, err := v.DayJSON(test.WantDate)
			if err != nil {
				t.Fatalf("DayJSON: %v", err)
			}
			got := decodeEntries(t, day)
			if diff := cmp.Diff(test.WantEntries, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// errBadInput marks table rows that expect a rejected append.
var errBadInput = fmt.Errorf("bad input")

func TestRangeRecentSearchStreak(t *testing.T) {
	t.Parallel()
	v := openTestVault(t)
	seed := []struct {
		Timestamp string
		Tags      string
		Body      string
	}{
		{"2026-09-01T08:00:00", "run", "Morning run."},
		{"2026-09-02T09:00:00", "", "Fixed the gate latch."},
		{"2026-09-03T10:00:00", "garden", "Planted garlic."},
	}
	for _, s := range seed {
		if err := v.AppendAt(s.Timestamp, s.Tags, s.Body); err != nil {
			t.Fatalf("AppendAt(%s): %v", s.Timestamp, err)
		}
	}

	rangeJSON, err := v.RangeJSON("2026-09-01", "2026-09-02")
	if err != nil {
		t.Fatalf("RangeJSON: %v", err)
	}
	if got := len(decodeEntries(t, rangeJSON)); got != 2 {
		t.Errorf("RangeJSON returned %d entries, want 2", got)
	}

	if _, err := v.RangeJSON("nope", "2026-09-02"); err == nil {
		t.Error("RangeJSON accepted a bad from date")
	}

	recentJSON, err := v.RecentJSON(2)
	if err != nil {
		t.Fatalf("RecentJSON: %v", err)
	}
	recent := decodeEntries(t, recentJSON)
	if len(recent) != 2 || recent[0].Body != "Planted garlic." {
		t.Errorf("RecentJSON = %+v, want the two newest entries starting with the garlic entry", recent)
	}

	searchJSON, err := v.SearchJSON("gate latch")
	if err != nil {
		t.Fatalf("SearchJSON: %v", err)
	}
	if got := len(decodeEntries(t, searchJSON)); got != 1 {
		t.Errorf("SearchJSON returned %d entries, want 1", got)
	}

	// Streak counts from today; the seeded past days do not reach it.
	if _, err := v.Streak(); err != nil {
		t.Errorf("Streak: %v", err)
	}
}

func TestOpenDeviceKeepsDevicesOffTheSameFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	desktop, err := Open(dir, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	phone, err := OpenDevice(dir, "", "iphone")
	if err != nil {
		t.Fatalf("OpenDevice: %v", err)
	}
	if err := desktop.AppendAt("2026-09-05T08:00:00", "desk", "Written on the desktop."); err != nil {
		t.Fatalf("desktop append: %v", err)
	}
	if err := phone.AppendAt("2026-09-05T09:00:00", "phone", "Captured on the phone."); err != nil {
		t.Fatalf("phone append: %v", err)
	}

	// The phone sees both its own pending capture and the canonical entry.
	got := decodeEntries(t, mustDay(t, phone, "2026-09-05"))
	if len(got) != 2 || got[0].Body != "Written on the desktop." || got[1].Body != "Captured on the phone." {
		t.Errorf("phone day = %+v, want both entries in time order", got)
	}

	// The desktop does not see the phone's entry until it is folded in.
	deskGot := decodeEntries(t, mustDay(t, desktop, "2026-09-05"))
	if len(deskGot) != 1 || deskGot[0].Body != "Written on the desktop." {
		t.Errorf("desktop day = %+v, want only the desktop entry", deskGot)
	}
}

func TestDeviceReadsMergeAcrossSurfaces(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	desktop, err := Open(dir, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	phone, err := OpenDevice(dir, "", "iphone")
	if err != nil {
		t.Fatalf("OpenDevice: %v", err)
	}
	if err := desktop.AppendAt("2026-09-04T08:00:00", "", "Older desktop entry."); err != nil {
		t.Fatalf("desktop append: %v", err)
	}
	if err := phone.AppendAt("2026-09-05T09:00:00", "", "Newer phone entry."); err != nil {
		t.Fatalf("phone append: %v", err)
	}

	inRange := decodeEntries(t, mustRange(t, phone, "2026-09-04", "2026-09-05"))
	if len(inRange) != 2 {
		t.Errorf("range returned %d entries, want 2", len(inRange))
	}

	recent, err := phone.RecentJSON(1)
	if err != nil {
		t.Fatalf("RecentJSON: %v", err)
	}
	newest := decodeEntries(t, recent)
	if len(newest) != 1 || newest[0].Body != "Newer phone entry." {
		t.Errorf("recent = %+v, want the phone entry as newest", newest)
	}

	found, err := phone.SearchJSON("phone entry")
	if err != nil {
		t.Fatalf("SearchJSON: %v", err)
	}
	if got := len(decodeEntries(t, found)); got != 1 {
		t.Errorf("search returned %d entries, want 1", got)
	}
}

// mustDay returns the day JSON or fails the test.
func mustDay(t *testing.T, v *Vault, date string) string {
	t.Helper()
	data, err := v.DayJSON(date)
	if err != nil {
		t.Fatalf("DayJSON(%s): %v", date, err)
	}
	return data
}

// mustRange returns the range JSON or fails the test.
func mustRange(t *testing.T, v *Vault, from, to string) string {
	t.Helper()
	data, err := v.RangeJSON(from, to)
	if err != nil {
		t.Fatalf("RangeJSON(%s,%s): %v", from, to, err)
	}
	return data
}

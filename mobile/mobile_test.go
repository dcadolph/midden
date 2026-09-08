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

func TestRetrievalCoversInboxAndCanonical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	desk, err := Open(dir, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	phone, err := OpenDevice(dir, "", "iphone")
	if err != nil {
		t.Fatalf("OpenDevice: %v", err)
	}
	if err := desk.AppendAt("2024-07-04T09:00:00", "vacation,greece", "Ferry to the island."); err != nil {
		t.Fatalf("desk append: %v", err)
	}
	if err := desk.AppendAt("2026-07-04T09:00:00", "vacation", "Same date, later year."); err != nil {
		t.Fatalf("desk append: %v", err)
	}
	if err := phone.AppendAt("2026-09-05T09:00:00", "vacation", "Pending capture, still tagged."); err != nil {
		t.Fatalf("phone append: %v", err)
	}

	// Tag counts must sum across the canonical vault and the pending inbox.
	tagsJSON, err := phone.TagsJSON(0)
	if err != nil {
		t.Fatalf("TagsJSON: %v", err)
	}
	var tags []struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		t.Fatalf("unmarshal tags: %v", err)
	}
	got := map[string]int{}
	for _, tc := range tags {
		got[tc.Tag] = tc.Count
	}
	if got["vacation"] != 3 {
		t.Errorf("vacation count = %d, want 3 (two canonical plus one pending): %s", got["vacation"], tagsJSON)
	}
	if got["greece"] != 1 {
		t.Errorf("greece count = %d, want 1", got["greece"])
	}
	if len(tags) > 0 && tags[0].Tag != "vacation" {
		t.Errorf("tags[0] = %q, want the most used tag first", tags[0].Tag)
	}

	// Browsing a tag must reach the pending capture too.
	tagged := decodeEntries(t, mustCall(t, func() (string, error) { return phone.TaggedJSON("vacation") }))
	if len(tagged) != 3 {
		t.Errorf("tagged returned %d entries, want 3", len(tagged))
	}
	// A leading hash is how tags are written in a day file, so accept it.
	if hashed := decodeEntries(t, mustCall(t, func() (string, error) { return phone.TaggedJSON("#vacation") })); len(hashed) != 3 {
		t.Errorf("tagged with a leading hash returned %d entries, want 3", len(hashed))
	}

	// Flashback finds the same calendar day in an earlier year.
	back := decodeEntries(t, mustCall(t, func() (string, error) { return phone.FlashbackJSON(7, 4) }))
	if len(back) != 2 {
		t.Errorf("flashback returned %d entries, want both July 4ths", len(back))
	}
	if _, err := phone.FlashbackJSON(13, 1); err == nil {
		t.Error("FlashbackJSON accepted month 13")
	}
	if _, err := phone.FlashbackJSON(7, 0); err == nil {
		t.Error("FlashbackJSON accepted day 0")
	}
}

// mustCall runs a binding that returns JSON and fails the test on error.
func mustCall(t *testing.T, call func() (string, error)) string {
	t.Helper()
	data, err := call()
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	return data
}

func TestInsightsJSONDescribesTheRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v, err := Open(dir, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// A short run of days so the heatmap, totals, and tags have something real.
	for day := 1; day <= 5; day++ {
		stamp := fmt.Sprintf("2026-09-%02dT09:00:00", day)
		if err := v.AppendAt(stamp, "garden", "Worked in the garden."); err != nil {
			t.Fatalf("append %s: %v", stamp, err)
		}
	}
	raw, err := v.InsightsJSON(10)
	if err != nil {
		t.Fatalf("InsightsJSON: %v", err)
	}
	var got struct {
		Stats struct {
			Entries int `json:"entries"`
			Days    int `json:"days"`
		} `json:"stats"`
		Calendar []struct {
			Date  string `json:"date"`
			Count int    `json:"count"`
		} `json:"calendar"`
		Tags []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tags"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal insights: %v\n%s", err, raw)
	}
	if got.Stats.Entries != 5 || got.Stats.Days != 5 {
		t.Errorf("stats = %d entries over %d days, want 5 and 5", got.Stats.Entries, got.Stats.Days)
	}
	if len(got.Calendar) == 0 {
		t.Error("calendar is empty; the heatmap would render blank")
	}
	if len(got.Tags) != 1 || got.Tags[0].Tag != "garden" || got.Tags[0].Count != 5 {
		t.Errorf("tags = %+v, want garden counted five times", got.Tags)
	}
}

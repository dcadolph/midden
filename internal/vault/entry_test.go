package vault

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestEntrySerialize(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local)
	tests := []struct {
		WantResult string
		In         Entry
	}{{ // Test 0: Header time only, no tags, single-line body.
		In: Entry{
			Time: stamp,
			Body: "Single line entry.",
		},
		WantResult: "## 09:14:23\nSingle line entry.\n\n",
	}, { // Test 1: Header carries tags and body keeps internal newlines.
		In: Entry{
			Time: stamp,
			Tags: []string{"project", "idea"},
			Body: "First line.\nSecond line.",
		},
		WantResult: "## 09:14:23 #project #idea\nFirst line.\nSecond line.\n\n",
	}, { // Test 2: Trailing whitespace and newlines are trimmed on the body.
		In: Entry{
			Time: stamp,
			Body: "Body with trailing\n\n  ",
		},
		WantResult: "## 09:14:23\nBody with trailing\n\n",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := test.In.Serialize()
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEntryHasTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult bool
		In         Entry
		Probe      string
	}{{ // Test 0: Exact match.
		In:         Entry{Tags: []string{"project", "idea"}},
		Probe:      "project",
		WantResult: true,
	}, { // Test 1: Case-insensitive match.
		In:         Entry{Tags: []string{"Project"}},
		Probe:      "PROJECT",
		WantResult: true,
	}, { // Test 2: Leading hash on probe is tolerated.
		In:         Entry{Tags: []string{"project"}},
		Probe:      "#project",
		WantResult: true,
	}, { // Test 3: Missing tag returns false.
		In:         Entry{Tags: []string{"project"}},
		Probe:      "idea",
		WantResult: false,
	}, { // Test 4: No tags on entry returns false.
		In:         Entry{},
		Probe:      "anything",
		WantResult: false,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := test.In.HasTag(test.Probe)
			if got != test.WantResult {
				t.Errorf("want %t, got %t", test.WantResult, got)
			}
		})
	}
}

func TestAuthored(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		Body string
		Want bool
	}{{ // Test 0: A plain written entry is authored.
		Name: "handwritten", Body: "Thought about the roadmap today.", Want: true,
	}, { // Test 1: A calendar import is not.
		Name: "calendar", Body: "Standup (09:00 to 09:15)\nICS-UID: abc@example.com", Want: false,
	}, { // Test 2: A commit import is not.
		Name: "git", Body: "Fix the bug\nRepo: midden\nGIT-COMMIT: deadbeef", Want: false,
	}, { // Test 3: Prose mentioning a marker mid-line stays authored.
		Name: "prose mention", Body: "Wrote about how the ICS-UID: format works.", Want: true,
	}, { // Test 4: An answer to an ask question is authored.
		Name: "answer", Body: "He switched to baseball.\n\nIn answer to: something\nMIDDEN-ASKED: ended-abc", Want: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			if got := (Entry{Body: test.Body}).Authored(); got != test.Want {
				t.Errorf("want %v, got %v", test.Want, got)
			}
		})
	}
}

func TestStreakCountsOnlyKeptEntries(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	today := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.Local)
	entries := []Entry{
		// Imported events on today and yesterday, a real entry only yesterday.
		{Time: today, Body: "Standup\nICS-UID: a@b"},
		{Time: today.AddDate(0, 0, -1), Body: "Standup\nICS-UID: c@d"},
		{Time: today.AddDate(0, 0, -1), Body: "I wrote this myself."},
	}
	if err := v.AppendAll(entries); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	all, err := v.Streak(today, nil)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if all != 2 {
		t.Errorf("want an unfiltered streak of 2, got %d", all)
	}
	authored, err := v.Streak(today, Entry.Authored)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	// Today holds only an imported event, so the authored streak is broken at
	// today: appointments attended are not writing.
	if authored != 0 {
		t.Errorf("want an authored streak of 0, got %d", authored)
	}
}

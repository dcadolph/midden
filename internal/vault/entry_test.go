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

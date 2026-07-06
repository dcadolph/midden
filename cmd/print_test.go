package cmd

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/vault"
)

func TestEntriesToJSON(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 6, 16, 9, 30, 0, 0, time.Local)
	tests := []struct {
		WantResult []entryJSON
		In         []vault.Entry
	}{{ // Test 0: Nil input yields an empty slice so JSON output is [] not null.
		In: nil, WantResult: []entryJSON{},
	}, { // Test 1: Fields map one to one with an RFC 3339 timestamp.
		In: []vault.Entry{{Time: when, Tags: []string{"work", "meeting"}, Body: "standup"}},
		WantResult: []entryJSON{{
			Time: when.Format(time.RFC3339), Tags: []string{"work", "meeting"}, Body: "standup",
		}},
	}, { // Test 2: Entries without tags keep an empty tag list.
		In:         []vault.Entry{{Time: when, Body: "note"}},
		WantResult: []entryJSON{{Time: when.Format(time.RFC3339), Body: "note"}},
	}, { // Test 3: Order is preserved.
		In: []vault.Entry{{Time: when, Body: "first"}, {Time: when.Add(time.Hour), Body: "second"}},
		WantResult: []entryJSON{
			{Time: when.Format(time.RFC3339), Body: "first"},
			{Time: when.Add(time.Hour).Format(time.RFC3339), Body: "second"},
		},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := entriesToJSON(test.In)
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteEntryText(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 6, 16, 9, 30, 0, 0, time.Local)
	tests := []struct {
		WantResult string
		Entry      vault.Entry
		Color      bool
	}{{ // Test 0: Plain entry with tags.
		Entry:      vault.Entry{Time: when, Tags: []string{"work", "meeting"}, Body: "standup notes"},
		WantResult: "2026-06-16 09:30:00  [work, meeting]\nstandup notes\n\n",
	}, { // Test 1: Plain entry without tags.
		Entry:      vault.Entry{Time: when, Body: "just a note"},
		WantResult: "2026-06-16 09:30:00\njust a note\n\n",
	}, { // Test 2: Multi-line body prints verbatim.
		Entry:      vault.Entry{Time: when, Body: "line one\nline two"},
		WantResult: "2026-06-16 09:30:00\nline one\nline two\n\n",
	}, { // Test 3: Color wraps the timestamp and tag list in ANSI escapes.
		Color:      true,
		Entry:      vault.Entry{Time: when, Tags: []string{"work"}, Body: "standup"},
		WantResult: "\x1b[36m2026-06-16 09:30:00\x1b[0m\x1b[33m  [work]\x1b[0m\nstandup\n\n",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			writeEntryText(&buf, test.Entry, test.Color)
			if diff := cmp.Diff(test.WantResult, buf.String()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

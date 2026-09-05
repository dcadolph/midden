package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/dcadolph/midden/internal/vault"
)

func TestVoiceEntry(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 9, 5, 14, 30, 0, 0, time.Local)
	tests := []struct {
		Transcript string
		AudioPath  string
		Tags       []string
		WantResult vault.Entry
	}{{ // Test 0: Transcript alone becomes the body.
		Transcript: "Planted the fall garlic.",
		Tags:       []string{"voice"},
		WantResult: vault.Entry{Time: when, Tags: []string{"voice"}, Body: "Planted the fall garlic."},
	}, { // Test 1: A kept recording is linked below the transcript.
		Transcript: "Planted the fall garlic.",
		AudioPath:  "/vault/audio/2026/09/05/14-30-00.wav",
		Tags:       []string{"voice", "garden"},
		WantResult: vault.Entry{
			Time: when,
			Tags: []string{"voice", "garden"},
			Body: "Planted the fall garlic.\n\nAudio: `/vault/audio/2026/09/05/14-30-00.wav`",
		},
	}, { // Test 2: Nil tags pass through untouched.
		Transcript: "Quick note.",
		WantResult: vault.Entry{Time: when, Body: "Quick note."},
	}, { // Test 3: Multiline transcripts keep their structure.
		Transcript: "First thought.\n\nSecond thought.",
		Tags:       []string{"voice"},
		WantResult: vault.Entry{Time: when, Tags: []string{"voice"}, Body: "First thought.\n\nSecond thought."},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := voiceEntry(when, test.Tags, test.Transcript, test.AudioPath)
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

package cmd

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRecorderArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name       string
		Out        string
		Mono16k    bool
		WantResult []string
	}{{ // Test 0: Sox records the default device to the output file.
		Name: "sox", Out: "a.wav",
		WantResult: []string{"-d", "a.wav"},
	}, { // Test 1: Sox downsamples through effects after the output file.
		Name: "sox", Out: "a.wav", Mono16k: true,
		WantResult: []string{"-d", "a.wav", "rate", "16000", "channels", "1"},
	}, { // Test 2: Rec takes only the output file.
		Name: "rec", Out: "a.wav",
		WantResult: []string{"a.wav"},
	}, { // Test 3: Rec downsamples through effects after the output file.
		Name: "rec", Out: "a.wav", Mono16k: true,
		WantResult: []string{"a.wav", "rate", "16000", "channels", "1"},
	}, { // Test 4: Ffmpeg ends with overwrite and the output file.
		Name: "ffmpeg", Out: "a.wav", Mono16k: true,
		WantResult: []string{"-ar", "16000", "-ac", "1", "-y", "a.wav"},
	}, { // Test 5: Unknown recorders yield no args.
		Name: "arecord", Out: "a.wav",
		WantResult: nil,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := recorderArgs(test.Name, test.Out, test.Mono16k)
			if test.Name == "ffmpeg" && len(got) >= 5 {
				// The per-OS input args vary by platform; assert only the tail.
				got = got[len(got)-len(test.WantResult):]
			}
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

package cmd

import (
	"fmt"
	"testing"
)

func TestTrimTranscript(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		WantResult string
	}{{ // Test 0: Plain text is trimmed.
		In: "  Fed the sourdough starter.  \n", WantResult: "Fed the sourdough starter.",
	}, { // Test 1: Empty input stays empty.
		In: "", WantResult: "",
	}, { // Test 2: Blank-audio markers are dropped.
		In: "[BLANK_AUDIO]\nWent for a run.\n[BLANK_AUDIO]", WantResult: "Went for a run.",
	}, { // Test 3: Parenthesized noise markers are dropped.
		In: "(silence)\nCalled the vet about Biscuit.", WantResult: "Called the vet about Biscuit.",
	}, { // Test 4: Interior line structure is preserved.
		In: "First thought.\n\nSecond thought.", WantResult: "First thought.\n\nSecond thought.",
	}, { // Test 5: Noise-only input trims to empty.
		In: "[BLANK_AUDIO]\n(silence)\n", WantResult: "",
	}, { // Test 6: Brackets inside a sentence survive.
		In: "Ordered the part [rev B] today.", WantResult: "Ordered the part [rev B] today.",
	}, { // Test 7: Trailing per-line whitespace is trimmed.
		In: "Line one.  \nLine two.\t", WantResult: "Line one.\nLine two.",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := trimTranscript(test.In); got != test.WantResult {
				t.Errorf("trimTranscript(%q) = %q, want %q", test.In, got, test.WantResult)
			}
		})
	}
}

func TestIsModelPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         string
		WantResult bool
	}{{ // Test 0: Empty means the default model name.
		In: "", WantResult: false,
	}, { // Test 1: Bare model names are not paths.
		In: "base.en", WantResult: false,
	}, { // Test 2: Sized model names are not paths.
		In: "small.en", WantResult: false,
	}, { // Test 3: Absolute paths are paths.
		In: "/models/ggml-base.en.bin", WantResult: true,
	}, { // Test 4: Home-relative paths are paths.
		In: "~/models/custom.bin", WantResult: true,
	}, { // Test 5: A bare .bin filename is a path.
		In: "custom.bin", WantResult: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := isModelPath(test.In); got != test.WantResult {
				t.Errorf("isModelPath(%q) = %t, want %t", test.In, got, test.WantResult)
			}
		})
	}
}

func TestSizeLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult string
		In         int64
	}{{ // Test 0: Negative counts are unknown.
		In: -1, WantResult: "unknown size",
	}, { // Test 1: Zero renders as zero megabytes.
		In: 0, WantResult: "0 MB",
	}, { // Test 2: Whole megabytes round cleanly.
		In: 148 << 20, WantResult: "148 MB",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			if got := sizeLabel(test.In); got != test.WantResult {
				t.Errorf("sizeLabel(%d) = %q, want %q", test.In, got, test.WantResult)
			}
		})
	}
}

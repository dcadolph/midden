package util

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult string
		In         string
		Max        int
	}{{ // Test 0: Short string untouched.
		In: "abc", Max: 10, WantResult: "abc",
	}, { // Test 1: Truncates and appends ellipsis.
		In: "abcdef", Max: 4, WantResult: "abc…",
	}, { // Test 2: Non-positive max returns input.
		In: "hello", Max: 0, WantResult: "hello",
	}, { // Test 3: Multibyte runes counted correctly.
		In: "café au lait", Max: 5, WantResult: "café…",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := TruncateRunes(test.In, test.Max)
			if diff := cmp.Diff(test.WantResult, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Haystack   string
		Needle     string
		WantResult bool
	}{{ // Test 0: Exact match.
		Haystack: "midden", Needle: "midden", WantResult: true,
	}, { // Test 1: Case-insensitive match.
		Haystack: "Midden Vault", Needle: "vault", WantResult: true,
	}, { // Test 2: No match returns false.
		Haystack: "midden", Needle: "stele", WantResult: false,
	}, { // Test 3: Empty needle is treated as match.
		Haystack: "midden", Needle: "", WantResult: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := ContainsFold(test.Haystack, test.Needle)
			if got != test.WantResult {
				t.Errorf("want %t, got %t", test.WantResult, got)
			}
		})
	}
}

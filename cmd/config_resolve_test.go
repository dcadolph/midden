package cmd

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMergeTags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		WantResult []string
		Defaults   []string
		Flags      []string
	}{{ // Test 0: Nil inputs yield nil.
		Defaults: nil, Flags: nil, WantResult: nil,
	}, { // Test 1: Defaults only.
		Defaults: []string{"work"}, Flags: nil, WantResult: []string{"work"},
	}, { // Test 2: Flags only.
		Defaults: nil, Flags: []string{"health"}, WantResult: []string{"health"},
	}, { // Test 3: Defaults precede flags.
		Defaults: []string{"work"}, Flags: []string{"meeting"}, WantResult: []string{"work", "meeting"},
	}, { // Test 4: Case-insensitive duplicate keeps first-seen case.
		Defaults: []string{"Work"}, Flags: []string{"work"}, WantResult: []string{"Work"},
	}, { // Test 5: Hash prefixes and whitespace are normalized before merging.
		Defaults: []string{" #work "}, Flags: []string{"work", "  "}, WantResult: []string{"work"},
	}, { // Test 6: Duplicates within one source collapse.
		Defaults: []string{"a", "A", "b"}, Flags: []string{"B", "c"}, WantResult: []string{"a", "b", "c"},
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			got := mergeTags(test.Defaults, test.Flags)
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

package jsonutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// failWriter always errors to exercise the encode failure path.
type failWriter struct{}

// Write implements io.Writer by failing unconditionally.
func (failWriter) Write([]byte) (int, error) { return 0, errors.New("sink closed") }

func TestEncode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In         any
		WantResult string
		Pretty     bool
	}{{ // Test 0: Compact object ends with one newline.
		In: map[string]int{"a": 1}, Pretty: false, WantResult: "{\"a\":1}\n",
	}, { // Test 1: Pretty output is indented.
		In: map[string]int{"a": 1}, Pretty: true, WantResult: "{\n  \"a\": 1\n}\n",
	}, { // Test 2: HTML characters stay unescaped.
		In: map[string]string{"s": "<b>&</b>"}, Pretty: false, WantResult: "{\"s\":\"<b>&</b>\"}\n",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			if err := Encode(&b, test.In, test.Pretty); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if diff := cmp.Diff(test.WantResult, b.String()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEncodeWriterError(t *testing.T) {
	t.Parallel()
	if err := Encode(failWriter{}, map[string]int{"a": 1}, false); err == nil {
		t.Fatal("want error from failing writer, got nil")
	}
}

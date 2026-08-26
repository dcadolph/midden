package index

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestVectorRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   Vector
	}{
		{Name: "typical", In: Vector{0.1, -0.25, 1, -1, 0}},
		{Name: "empty", In: Vector{}},
		{Name: "extremes", In: Vector{math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32}},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(test.In)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var got Vector
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			// The blob is exact rather than approximate: the bits go out and come
			// back unchanged, so search scores do not drift with the file format.
			if diff := cmp.Diff(test.In, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestVectorNilRoundTrip(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Vector(nil))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("want null for an entry with no embedding yet, got %s", data)
	}
	var got Vector
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestVectorRejectsBadInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name string
		In   string
	}{
		{Name: "json array from an older index", In: "[0.1,0.2,0.3]"},
		{Name: "not base64", In: `"not base64!!"`},
		{Name: "truncated float", In: `"AAAAAAA="`},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			var got Vector
			if err := json.Unmarshal([]byte(test.In), &got); err == nil {
				t.Errorf("want an error for %s, got %v", test.Name, got)
			}
		})
	}
}

func TestIndexRoundTripsThroughBlobVectors(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Provider: "test",
		Dim:      3,
		Entries: []Entry{
			{Body: "alpha", Embedding: Vector{1, 0, 0}},
			{Body: "beta", Embedding: Vector{0, 0.5, -0.5}},
			{Body: "not embedded yet"},
		},
	}
	data, err := idx.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if diff := cmp.Diff(idx.Entries, got.Entries); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

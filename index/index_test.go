package index

import (
	"testing"
	"time"
)

func TestSearchRanksByCosine(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Provider: "test",
		Dim:      3,
		Entries: []Entry{
			{Time: time.Now(), Body: "alpha", Embedding: []float32{1, 0, 0}},
			{Time: time.Now(), Body: "beta", Embedding: []float32{0.7071, 0.7071, 0}},
			{Time: time.Now(), Body: "gamma", Embedding: []float32{0, 1, 0}},
		},
	}
	got := idx.Search([]float32{1, 0, 0}, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 matches, got %d", len(got))
	}
	if got[0].Entry.Body != "alpha" || got[1].Entry.Body != "beta" {
		t.Errorf("unexpected order: %q then %q", got[0].Entry.Body, got[1].Entry.Body)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	idx := &Index{
		Provider: "stub:dim3",
		Dim:      3,
		BuiltAt:  time.Unix(1_700_000_000, 0).UTC(),
		Entries: []Entry{
			{Time: time.Unix(1_700_000_500, 0).UTC(), Body: "hello", Embedding: []float32{1, 2, 3}},
		},
	}
	data, err := idx.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	loaded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if loaded.Provider != idx.Provider || loaded.Dim != idx.Dim || len(loaded.Entries) != 1 {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestDecodeEmpty(t *testing.T) {
	t.Parallel()
	idx, err := Decode(nil)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("empty index should have no entries, got %d", len(idx.Entries))
	}
}

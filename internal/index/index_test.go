package index

import (
	"path/filepath"
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

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), Filename)
	idx := &Index{
		Provider: "stub:dim3",
		Dim:      3,
		BuiltAt:  time.Unix(1_700_000_000, 0).UTC(),
		Entries: []Entry{
			{Time: time.Unix(1_700_000_500, 0).UTC(), Body: "hello", Embedding: []float32{1, 2, 3}},
		},
	}
	if err := idx.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Provider != idx.Provider || loaded.Dim != idx.Dim || len(loaded.Entries) != 1 {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	idx, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("missing index should be empty, got %d entries", len(idx.Entries))
	}
}

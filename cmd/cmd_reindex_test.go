package cmd

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/internal/vault"
)

// mockEmbedder is an Embedder whose vectors are produced by a configurable
// function, recording how many texts it was asked to embed.
type mockEmbedder struct {
	// EmbedFunc produces the vectors for one call.
	EmbedFunc func(texts []string) ([][]float32, error)
	// embedded counts the texts passed across every call.
	embedded atomic.Int64
	// calls counts how many times Embed was invoked.
	calls atomic.Int64
}

// Embed delegates to EmbedFunc, recording the volume it was asked for.
func (m *mockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	m.calls.Add(1)
	m.embedded.Add(int64(len(texts)))
	return m.EmbedFunc(texts)
}

// Dim reports the fixed width of the mock's vectors.
func (m *mockEmbedder) Dim() int { return 2 }

// Name identifies the mock provider.
func (m *mockEmbedder) Name() string { return "mock:embed" }

// unitVectors returns one distinct two-dimensional vector per input.
func unitVectors(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = []float32{float32(len(t)), 1}
	}
	return out, nil
}

// testReindexConfig returns a rebuild config with a generous per-call deadline,
// so a slow machine never turns a unit test into a timeout failure.
func testReindexConfig(batch int) reindexConfig {
	return reindexConfig{Batch: batch, Timeout: time.Minute}
}

// reindexVault returns a temp vault and a command with output discarded.
func reindexVault(t *testing.T) (*vault.Vault, *cobra.Command) {
	t.Helper()
	v, err := vault.Open(t.TempDir())
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	c := &cobra.Command{}
	c.SetOut(new(strings.Builder))
	c.SetErr(new(strings.Builder))
	return v, c
}

// buildIndex assembles an index over the given bodies with no embeddings yet,
// plus the positions needing one.
func buildIndex(bodies ...string) (*index.Index, []int) {
	idx := &index.Index{Provider: "mock:embed"}
	pending := make([]int, 0, len(bodies))
	for i, b := range bodies {
		idx.Entries = append(idx.Entries, index.Entry{Body: b})
		pending = append(pending, i)
	}
	return idx, pending
}

func TestEmbedPendingFillsEveryPendingEntry(t *testing.T) {
	t.Parallel()
	v, cmd := reindexVault(t)
	idx, pending := buildIndex("alpha", "beta", "gamma", "delta", "epsilon")
	emb := &mockEmbedder{EmbedFunc: unitVectors}
	if err := embedPending(cmd, v, emb, idx, pending, testReindexConfig(2)); err != nil {
		t.Fatalf("embedPending: %v", err)
	}
	for i, e := range idx.Entries {
		if len(e.Embedding) == 0 {
			t.Errorf("entry %d (%q) was left without an embedding", i, e.Body)
		}
	}
	if got := emb.embedded.Load(); got != 5 {
		t.Errorf("want 5 texts embedded, got %d", got)
	}
	// Five entries at a batch size of two is three calls, which is what keeps a
	// large backfill inside a per-call deadline instead of one run-long one.
	if got := emb.calls.Load(); got != 3 {
		t.Errorf("want 3 batched calls, got %d", got)
	}
}

func TestEmbedPendingCheckpointsBeforeReturningAFailure(t *testing.T) {
	t.Parallel()
	v, cmd := reindexVault(t)
	idx, pending := buildIndex("alpha", "beta", "gamma", "delta")
	emb := &mockEmbedder{EmbedFunc: func(texts []string) ([][]float32, error) {
		if strings.Contains(strings.Join(texts, ","), "gamma") {
			return nil, errors.New("provider exploded")
		}
		return unitVectors(texts)
	}}
	err := embedPending(cmd, v, emb, idx, pending, testReindexConfig(2))
	if err == nil {
		t.Fatal("want an error when a batch fails")
	}
	// The first batch was paid for, so it has to survive the failure or the next
	// run buys the same embeddings again.
	saved, err := loadIndex(v)
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if len(saved.Entries) != 2 {
		t.Errorf("want the 2 completed entries checkpointed, got %d", len(saved.Entries))
	}
	if saved.Dim != 2 {
		t.Errorf("want the checkpoint to record the embedding width, got %d", saved.Dim)
	}
}

func TestEmbedPendingRejectsAShortProviderResponse(t *testing.T) {
	t.Parallel()
	v, cmd := reindexVault(t)
	idx, pending := buildIndex("alpha", "beta", "gamma")
	emb := &mockEmbedder{EmbedFunc: func(texts []string) ([][]float32, error) {
		vecs, _ := unitVectors(texts)
		return vecs[:len(vecs)-1], nil
	}}
	// Silently accepting fewer vectors than texts would shift every embedding
	// onto the wrong entry.
	if err := embedPending(cmd, v, emb, idx, pending, testReindexConfig(4)); err == nil {
		t.Fatal("want an error when the provider returns fewer vectors than texts")
	}
}

func TestCachedEmbeddingsReuseAndInvalidation(t *testing.T) {
	t.Parallel()
	v, _ := reindexVault(t)
	stored := &index.Index{
		Provider: "mock:embed",
		Dim:      2,
		Entries: []index.Entry{
			{Body: "alpha", Embedding: []float32{1, 0}},
			{Body: "beta", Embedding: []float32{0, 1}},
		},
	}
	if err := saveIndex(v, stored); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}

	cached, err := cachedEmbeddings(v, "mock:embed", testReindexConfig(4))
	if err != nil {
		t.Fatalf("cachedEmbeddings: %v", err)
	}
	if len(cached) != 2 {
		t.Errorf("want 2 reusable vectors, got %d", len(cached))
	}
	if _, ok := cached[index.ContentHash("alpha")]; !ok {
		t.Error("want the vector for unchanged text to be reusable")
	}

	// A different provider means a different geometry, so nothing may carry over.
	other, err := cachedEmbeddings(v, "mock:other", testReindexConfig(4))
	if err != nil {
		t.Fatalf("cachedEmbeddings: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("want no reuse across providers, got %d vectors", len(other))
	}

	fullCfg := testReindexConfig(4)
	fullCfg.Full = true
	full, err := cachedEmbeddings(v, "mock:embed", fullCfg)
	if err != nil {
		t.Fatalf("cachedEmbeddings: %v", err)
	}
	if len(full) != 0 {
		t.Errorf("want --full to ignore the cache, got %d vectors", len(full))
	}
}

func TestSetDimSkipsEntriesWithoutVectors(t *testing.T) {
	t.Parallel()
	idx := &index.Index{Entries: []index.Entry{
		{Body: "no vector yet"},
		{Body: "embedded", Embedding: []float32{1, 2, 3}},
	}}
	setDim(idx)
	if idx.Dim != 3 {
		t.Errorf("want dim 3, got %d", idx.Dim)
	}
}

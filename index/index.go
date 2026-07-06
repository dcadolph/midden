// Package index stores per-entry embeddings on disk for fast semantic recall.
package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// Filename is the JSON file midden writes index data to inside the vault root.
const Filename = ".midden.index.json"

// Entry is one indexed journal entry with its embedding.
type Entry struct {
	// Time is the local timestamp of the entry.
	Time time.Time `json:"time"`
	// Tags are the entry tags.
	Tags []string `json:"tags,omitempty"`
	// Body is the entry text. Stored verbatim so recall can quote it back.
	Body string `json:"body"`
	// Embedding is the vector representation of Body.
	Embedding []float32 `json:"embedding"`
}

// Index is the persistent vector store kept inside the vault root.
type Index struct {
	// Provider is the embedder identifier (model name) the entries were embedded with.
	Provider string `json:"provider"`
	// Dim is the embedding dimension expected by every Entry.
	Dim int `json:"dim"`
	// BuiltAt is the timestamp the index was last fully rebuilt.
	BuiltAt time.Time `json:"built_at"`
	// Entries hold every indexed entry.
	Entries []Entry `json:"entries"`
}

// Match pairs an Entry with its similarity score.
type Match struct {
	// Entry is the matched entry.
	Entry Entry
	// Score is the cosine similarity between query and entry embedding.
	Score float32
}

// Decode parses index bytes produced by Encode.
// Empty input returns a zero Index and a nil error.
func Decode(data []byte) (*Index, error) {
	if len(data) == 0 {
		return &Index{}, nil
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	return &idx, nil
}

// Encode renders the index as JSON bytes. Callers persist the bytes through
// the vault so encrypted vaults keep the index sealed at rest.
func (i *Index) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(i); err != nil {
		return nil, fmt.Errorf("encode index: %w", err)
	}
	return buf.Bytes(), nil
}

// Search returns the top-k entries by cosine similarity to the query vector.
// A non-positive k returns every entry ranked.
func (i *Index) Search(query []float32, k int) []Match {
	if len(i.Entries) == 0 {
		return nil
	}
	matches := make([]Match, 0, len(i.Entries))
	for _, e := range i.Entries {
		matches = append(matches, Match{Entry: e, Score: cosine(query, e.Embedding)})
	}
	sort.Slice(matches, func(a, b int) bool { return matches[a].Score > matches[b].Score })
	if k > 0 && len(matches) > k {
		matches = matches[:k]
	}
	return matches
}

// cosine returns the cosine similarity between two equal-length vectors.
// Vectors of different length or zero magnitude return zero.
func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

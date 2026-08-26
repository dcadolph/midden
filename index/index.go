// Package index stores per-entry embeddings on disk for fast semantic recall.
package index

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	Embedding Vector `json:"embedding"`
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

// ContentHash returns the key that matches an entry body to a cached
// embedding. Only the body is ever embedded, so two entries with the same text
// can share a vector no matter how their timestamps or tags differ.
func ContentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// EmbeddingsByContent maps each indexed body's content hash to its embedding so
// a rebuild can reuse the vectors for text that has not changed. Rebuilding a
// vault holding years of backfilled history costs one provider call per batch
// of new entries this way, rather than re-embedding the whole corpus every time
// a single day is added.
func (i *Index) EmbeddingsByContent() map[string][]float32 {
	out := make(map[string][]float32, len(i.Entries))
	for _, e := range i.Entries {
		if len(e.Embedding) == 0 {
			continue
		}
		out[ContentHash(e.Body)] = e.Embedding
	}
	return out
}

// Search returns the top-k entries by cosine similarity to the query vector.
// A non-positive k returns every entry ranked.
func (i *Index) Search(query []float32, k int) []Match {
	return i.SearchRange(query, k, time.Time{}, time.Time{})
}

// SearchRange returns the top-k entries by cosine similarity to the query
// vector, considering only entries timestamped inside the range. A zero from or
// to leaves that end of the range open, and a non-positive k returns every
// candidate ranked. Scoping by time before ranking keeps a question about one
// period from matching a semantically similar entry years away from it.
func (i *Index) SearchRange(query []float32, k int, from, to time.Time) []Match {
	if len(i.Entries) == 0 {
		return nil
	}
	matches := make([]Match, 0, len(i.Entries))
	for _, e := range i.Entries {
		if !inRange(e.Time, from, to) {
			continue
		}
		matches = append(matches, Match{Entry: e, Score: cosine(query, e.Embedding)})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(a, b int) bool { return matches[a].Score > matches[b].Score })
	if k > 0 && len(matches) > k {
		matches = matches[:k]
	}
	return matches
}

// InRange returns every entry timestamped inside the range ordered by ascending
// timestamp. A zero from or to leaves that end of the range open. Questions
// about a period are answered from the whole period rather than from its
// nearest neighbors, so callers take the full slice instead of a ranked head.
func (i *Index) InRange(from, to time.Time) []Entry {
	out := make([]Entry, 0, len(i.Entries))
	for _, e := range i.Entries {
		if inRange(e.Time, from, to) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Time.Before(out[b].Time) })
	return out
}

// inRange reports whether t falls inside the closed range, treating a zero
// bound as open.
func inRange(t, from, to time.Time) bool {
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
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

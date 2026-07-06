// Package llm wraps the external LLM providers midden talks to for embeddings and chat.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
)

// Embedder produces a vector embedding for each input string.
// Implementations are safe to call from multiple goroutines.
type Embedder interface {
	// Embed returns one vector per input string.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim returns the embedding dimension, or zero when unknown until first call.
	Dim() int
	// Name identifies the provider for index headers and logs.
	Name() string
}

// EmbedderFromEnv picks an embedder using documented environment precedence.
// VOYAGE_API_KEY → Voyage AI, OPENAI_API_KEY → OpenAI, then a configured or
// reachable Ollama daemon. MIDDEN_EMBED_PROVIDER short-circuits autodetection.
func EmbedderFromEnv() (Embedder, error) {
	provider := os.Getenv("MIDDEN_EMBED_PROVIDER")
	switch provider {
	case "voyage":
		return newVoyage()
	case "openai":
		return newOpenAIEmbed()
	case "ollama":
		return newOllamaEmbed()
	case "":
	default:
		return nil, fmt.Errorf("unknown MIDDEN_EMBED_PROVIDER %q", provider)
	}
	if os.Getenv("VOYAGE_API_KEY") != "" {
		return newVoyage()
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return newOpenAIEmbed()
	}
	if os.Getenv("OLLAMA_HOST") != "" || isOllamaReachable() {
		return newOllamaEmbed()
	}
	return nil, errors.New("no embedding provider configured: set VOYAGE_API_KEY, OPENAI_API_KEY, or run ollama locally")
}

// dimTracker records the embedding dimension observed on the first result.
// Embed may run from multiple goroutines, so the value is atomic.
type dimTracker struct {
	// dim is the observed embedding dimension, zero until known.
	dim atomic.Int64
}

// Dim returns the recorded embedding dimension, or zero when unknown.
func (d *dimTracker) Dim() int { return int(d.dim.Load()) }

// record stores the dimension of the first vector in vecs, when present.
func (d *dimTracker) record(vecs [][]float32) {
	if len(vecs) > 0 && len(vecs[0]) > 0 {
		d.dim.Store(int64(len(vecs[0])))
	}
}

// dataEmbeddings decodes the {"data":[{"embedding":[...]}]} shape shared by
// OpenAI and Voyage embedding responses.
func dataEmbeddings(data []byte) ([][]float32, error) {
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

// openAIEmbed implements Embedder against the OpenAI embeddings endpoint.
type openAIEmbed struct {
	dimTracker
	// apiKey authenticates requests; never logged or serialized.
	apiKey string
	// model is the model ID sent with every request.
	model string
}

func newOpenAIEmbed() (*openAIEmbed, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}
	model := os.Getenv("OPENAI_EMBED_MODEL")
	if model == "" {
		model = defaultOpenAIEmbedModel
	}
	return &openAIEmbed{apiKey: key, model: model}, nil
}

func (e *openAIEmbed) Name() string { return "openai:" + e.model }

func (e *openAIEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	data, err := postJSON(ctx, "openai embeddings", "https://api.openai.com/v1/embeddings",
		map[string]string{"Authorization": "Bearer " + e.apiKey},
		map[string]any{"input": texts, "model": e.model})
	if err != nil {
		return nil, err
	}
	out, err := dataEmbeddings(data)
	if err != nil {
		return nil, err
	}
	e.record(out)
	return out, nil
}

// voyageEmbed implements Embedder against the Voyage AI embeddings endpoint.
type voyageEmbed struct {
	dimTracker
	// apiKey authenticates requests; never logged or serialized.
	apiKey string
	// model is the model ID sent with every request.
	model string
}

func newVoyage() (*voyageEmbed, error) {
	key := os.Getenv("VOYAGE_API_KEY")
	if key == "" {
		return nil, errors.New("VOYAGE_API_KEY is not set")
	}
	model := os.Getenv("VOYAGE_EMBED_MODEL")
	if model == "" {
		model = defaultVoyageEmbedModel
	}
	return &voyageEmbed{apiKey: key, model: model}, nil
}

func (e *voyageEmbed) Name() string { return "voyage:" + e.model }

func (e *voyageEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	data, err := postJSON(ctx, "voyage embeddings", "https://api.voyageai.com/v1/embeddings",
		map[string]string{"Authorization": "Bearer " + e.apiKey},
		map[string]any{"input": texts, "model": e.model, "input_type": "document"})
	if err != nil {
		return nil, err
	}
	out, err := dataEmbeddings(data)
	if err != nil {
		return nil, err
	}
	e.record(out)
	return out, nil
}

// ollamaEmbed implements Embedder against a local Ollama daemon.
type ollamaEmbed struct {
	dimTracker
	// host is the Ollama base URL.
	host string
	// model is the model name sent with every request.
	model string
}

func newOllamaEmbed() (*ollamaEmbed, error) {
	model := os.Getenv("OLLAMA_EMBED_MODEL")
	if model == "" {
		model = defaultOllamaEmbedModel
	}
	return &ollamaEmbed{host: ollamaHost(), model: model}, nil
}

func (e *ollamaEmbed) Name() string { return "ollama:" + e.model }

func (e *ollamaEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	data, err := postJSON(ctx, "ollama embeddings", e.host+"/api/embed", nil,
		map[string]any{"model": e.model, "input": texts})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embeddings: got %d vectors for %d inputs", len(parsed.Embeddings), len(texts))
	}
	e.record(parsed.Embeddings)
	return parsed.Embeddings, nil
}
